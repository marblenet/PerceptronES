package engine

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"perceptronsre/internal/config"
	"perceptronsre/internal/model"
)

func TestRuntimeProducesTraceableGreenAssessment(t *testing.T) {
	cfg := config.Default()
	runtime, err := NewRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.Run(context.Background(), "cycle-green", healthyObservations())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Mission.Status != model.MissionGreen {
		t.Fatalf("expected GREEN, got %s", result.Mission.Status)
	}
	if !result.Mission.Coverage.Complete {
		t.Fatalf("expected complete coverage, got %#v", result.Mission.Coverage)
	}
	if got := len(result.Events); got != 14 {
		t.Fatalf("expected 14 cycle, observation, pressure, health and mission events; got %d", got)
	}
	if len(result.Events[len(result.Events)-1].DerivesFrom) != 5 {
		t.Fatalf("mission event should derive from cycle context and four health events: %#v", result.Events[len(result.Events)-1].DerivesFrom)
	}
	if result.Metrics.Engines["host-pressure"].Runs != 1 || result.Metrics.Engines["host-health"].Runs != 1 {
		t.Fatalf("engine invocation metrics are incomplete: %#v", result.Metrics.Engines)
	}
	if result.Metrics.CoreServices["perceptron-mission"].Runs != 1 {
		t.Fatalf("core mission service metrics are incomplete: %#v", result.Metrics.CoreServices)
	}
	if result.Metrics.Goroutines <= 0 || result.Metrics.HeapAllocBytes == 0 {
		t.Fatalf("platform self-observation metrics are missing: %#v", result.Metrics)
	}
	for _, event := range result.Events {
		if event.ProducerVersion == "" || event.ConfigurationID == "" {
			t.Fatalf("event is missing producer/configuration provenance: %#v", event)
		}
	}
	knownIDs := make(map[string]bool, len(result.Events))
	for _, event := range result.Events {
		for _, parentID := range event.DerivesFrom {
			if !knownIDs[parentID] {
				t.Fatalf("event %s references missing or later derivation input %s", event.ID, parentID)
			}
		}
		knownIDs[event.ID] = true
	}
}

func TestConfigurationFingerprintChangesWithRules(t *testing.T) {
	first := config.Default()
	second := config.Default()
	second.Thresholds.CPUPercent.Warning++
	firstID, err := fingerprintConfig(first)
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := fingerprintConfig(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstID == secondID {
		t.Fatal("configuration fingerprint did not change when a threshold changed")
	}
}

func TestRuntimeSupportsNoEnabledEngines(t *testing.T) {
	cfg := config.Default()
	cfg.Engines = []string{}
	runtime, err := NewRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.Run(context.Background(), "cycle-no-engines", healthyObservations())
	if err != nil {
		t.Fatalf("engine-free collection cycle failed: %v", err)
	}
	if result.Mission == nil || result.Mission.Status != model.MissionUnknown {
		t.Fatalf("engine-free cycle should still produce an UNKNOWN core mission assessment: %#v", result.Mission)
	}
	if len(result.Metrics.Engines) != 0 {
		t.Fatalf("engine-free cycle unexpectedly ran engines: %#v", result.Metrics.Engines)
	}
	if got := len(result.Events); got != 2+len(healthyObservations()) {
		t.Fatalf("engine-free cycle should retain cycle, observation, and core mission events; got %d", got)
	}
}

func TestProcessEngineUsesVersionedJSONProtocol(t *testing.T) {
	previous := os.Getenv("PERCEPTRON_ENGINE_HELPER")
	if err := os.Setenv("PERCEPTRON_ENGINE_HELPER", "1"); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv("PERCEPTRON_ENGINE_HELPER", previous)

	plugin, err := newProcessEngine(config.Plugin{
		Name:     "test-plugin",
		Version:  "1.0.0",
		Command:  os.Args[0],
		Args:     []string{"-test.run=TestProcessEngineHelper"},
		Category: "test",
		Role:     "test",
		Consumes: []string{model.TypeObservation},
		Produces: []string{"test.output.v1"},
		Timeout:  "5s",
		Policy:   config.EnginePolicy{Mode: "reasoning_only", ReadOnly: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := plugin.Process(context.Background(), []model.Event{})
	if err != nil {
		t.Fatalf("plugin process call failed: %v", err)
	}
	if outputs == nil || len(outputs) != 0 {
		t.Fatalf("unexpected plugin outputs: %#v", outputs)
	}
}

func TestProcessEngineHelper(t *testing.T) {
	if os.Getenv("PERCEPTRON_ENGINE_HELPER") != "1" {
		return
	}
	var request PluginRequest
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		os.Exit(2)
	}
	if request.ProtocolVersion != PluginProtocolVersion || request.Name != "test-plugin" {
		os.Exit(3)
	}
	response := PluginResponse{ProtocolVersion: PluginProtocolVersion, Outputs: []model.Event{}}
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		os.Exit(4)
	}
	os.Exit(0)
}

func TestRuntimeDoesNotTreatMissingInputsAsHealthy(t *testing.T) {
	cfg := config.Default()
	runtime, err := NewRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	observations := healthyObservations()
	observations[2] = unavailableObservation("memory.used_percent", "memory")
	result, err := runtime.Run(context.Background(), "cycle-missing", observations)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Mission.Status != model.MissionUnknown {
		t.Fatalf("expected UNKNOWN for missing required memory evidence, got %s", result.Mission.Status)
	}
	if !result.Mission.Coverage.Complete {
		t.Fatal("the explicit unavailable-memory assessment should represent the required scope")
	}
	assertHealthState(t, result.Events, "memory", model.HealthUnknown)
	if healthCoverageFor(t, result.Events, "memory") != "unavailable" {
		t.Fatal("unavailable evidence was not distinguished from scope coverage")
	}
}

func TestKnownPressureDoesNotHideUnknownRequiredEvidence(t *testing.T) {
	runtime, err := NewRuntime(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	observations := healthyObservations()
	observations[0] = availableObservation("cpu.utilization_percent", "cpu", 90, "percent")
	observations[2] = unavailableObservation("memory.used_percent", "memory")
	result, err := runtime.Run(context.Background(), "cycle-warning-unknown", observations)
	if err != nil {
		t.Fatal(err)
	}
	if result.Mission.Status != model.MissionYellow || result.Mission.Freshness != "unknown" || !strings.Contains(result.Mission.Explanation, "risk may be higher") {
		t.Fatalf("known warning hid uncertainty in another required input: %#v", result.Mission)
	}
}

func TestRuntimeReportsAbsentComponentAsCoverageGap(t *testing.T) {
	runtime, err := NewRuntime(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	observations := healthyObservations()
	observations = append(observations[:2], observations[3:]...)
	result, err := runtime.Run(context.Background(), "cycle-coverage-gap", observations)
	if err != nil {
		t.Fatal(err)
	}
	if result.Mission.Status != model.MissionUnknown || result.Mission.Coverage.Complete {
		t.Fatalf("absent memory assessment did not create an explicit coverage gap: %#v", result.Mission)
	}
	if len(result.Mission.Coverage.MissingComponents) != 1 || result.Mission.Coverage.MissingComponents[0] != "memory" {
		t.Fatalf("missing component was not identified: %#v", result.Mission.Coverage)
	}
}

func TestRuntimeReportsCriticalPressureWithoutActionAuthority(t *testing.T) {
	cfg := config.Default()
	runtime, err := NewRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	observations := healthyObservations()
	observations[0] = availableObservation("cpu.utilization_percent", "cpu", 99, "percent")
	result, err := runtime.Run(context.Background(), "cycle-critical", observations)
	if err != nil {
		t.Fatal(err)
	}
	if result.Mission.Status != model.MissionRed || result.Mission.Disposition != "escalate" {
		t.Fatalf("expected RED/escalate, got %s/%s", result.Mission.Status, result.Mission.Disposition)
	}
	if result.Mission.Explanation == "" {
		t.Fatal("mission explanation is required")
	}
}

func TestRuntimeReportsWarningAndKeepsUnassessedSignalsOutOfMissionCoverage(t *testing.T) {
	runtime, err := NewRuntime(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	observations := healthyObservations()
	observations[0] = availableObservation("cpu.utilization_percent", "cpu", 90, "percent")
	observations = append(observations,
		unavailableObservation("network.rx_bytes", "network"),
		unavailableObservation("process.count", "process"))
	result, err := runtime.Run(context.Background(), "cycle-warning", observations)
	if err != nil {
		t.Fatal(err)
	}
	if result.Mission.Status != model.MissionYellow {
		t.Fatalf("expected YELLOW for warning pressure, got %s", result.Mission.Status)
	}
	if !result.Mission.Coverage.Complete || len(result.Mission.Coverage.AssessedComponents) != 4 {
		t.Fatalf("unassessed network/process signals affected mission coverage: %#v", result.Mission.Coverage)
	}
}

func TestConfiguredDNSProbeFailureAffectsMissionWithProvenance(t *testing.T) {
	cfg := config.Default()
	cfg.DNSProbeName = "service.example.test"
	runtime, err := NewRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	failed := 0.0
	latency := 25.0
	dnsObservation := model.Observation{
		Metric:        "dns.resolution_success",
		Component:     "dns",
		Target:        cfg.DNSProbeName,
		Value:         &failed,
		LatencyMillis: &latency,
		Unit:          "boolean",
		Available:     true,
		Reason:        "resolution failed for configured probe",
		ObservedAt:    time.Now().UTC(),
	}
	observations := append(healthyObservations(), dnsObservation)
	result, err := runtime.Run(context.Background(), "cycle-dns-failed", observations)
	if err != nil {
		t.Fatal(err)
	}
	if result.Mission.Status != model.MissionRed || result.Mission.Coverage.Complete == false {
		t.Fatalf("configured DNS failure was not included in mission: %#v", result.Mission)
	}
	if result.Mission.Objective == "" || result.Mission.Components[len(result.Mission.Components)-1].ScopeID != cfg.DNSProbeName {
		t.Fatalf("DNS target scope was not preserved: %#v", result.Mission)
	}
}

func TestConfiguredDNSSuccessCanSupportGreenMission(t *testing.T) {
	cfg := config.Default()
	cfg.DNSProbeName = "service.example.test"
	runtime, err := NewRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	success := 1.0
	latency := 4.5
	dnsObservation := model.Observation{
		Metric:        "dns.resolution_success",
		Component:     "dns",
		Target:        cfg.DNSProbeName,
		Value:         &success,
		LatencyMillis: &latency,
		Unit:          "boolean",
		Available:     true,
		ObservedAt:    time.Now().UTC(),
	}
	result, err := runtime.Run(context.Background(), "cycle-dns-success", append(healthyObservations(), dnsObservation))
	if err != nil {
		t.Fatal(err)
	}
	if result.Mission.Status != model.MissionGreen || !result.Mission.Coverage.Complete {
		t.Fatalf("successful DNS probe did not contribute to a green mission: %#v", result.Mission)
	}
}

func TestConfiguredDNSProbeMissingPreventsGreenMission(t *testing.T) {
	cfg := config.Default()
	cfg.DNSProbeName = "service.example.test"
	runtime, err := NewRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.Run(context.Background(), "cycle-dns-missing", healthyObservations())
	if err != nil {
		t.Fatal(err)
	}
	if result.Mission.Status != model.MissionUnknown || result.Mission.Coverage.Complete || result.Mission.Freshness != "fresh" {
		t.Fatalf("missing configured DNS probe was not reported as unknown: %#v", result.Mission)
	}
}

func TestRuntimeWithNoObservationsEmitsUnknownMission(t *testing.T) {
	runtime, err := NewRuntime(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.Run(context.Background(), "cycle-empty", nil)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Mission.Status != model.MissionUnknown || result.Mission.Coverage.Complete {
		t.Fatalf("empty cycle was not represented as unknown/incomplete: %#v", result.Mission)
	}
}

func TestRuntimeRejectsObservationWithMismatchedUnit(t *testing.T) {
	runtime, err := NewRuntime(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	observations := healthyObservations()
	observations[0].Unit = "load"
	if _, err := runtime.Run(context.Background(), "cycle-bad-unit", observations); err == nil {
		t.Fatal("expected a CPU observation with the wrong unit to be rejected")
	}
}

func TestRuntimeMarksStaleObservationUnknown(t *testing.T) {
	runtime, err := NewRuntime(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	observations := healthyObservations()
	stale := time.Now().Add(-time.Minute)
	observations[0].ObservedAt = stale
	result, err := runtime.Run(context.Background(), "cycle-stale", observations)
	if err != nil {
		t.Fatal(err)
	}
	if result.Mission.Status != model.MissionUnknown || result.Mission.Freshness != "stale" || !result.Mission.Coverage.Complete {
		t.Fatalf("stale evidence did not make assessment unknown: %#v", result.Mission)
	}
}

func healthCoverageFor(t *testing.T, events []model.Event, component string) string {
	t.Helper()
	for _, event := range events {
		if event.Type != model.TypeHealth {
			continue
		}
		var assessment model.HealthAssessment
		if err := json.Unmarshal(event.Payload, &assessment); err != nil {
			t.Fatal(err)
		}
		if assessment.Component == component {
			return assessment.Coverage
		}
	}
	t.Fatalf("did not find %s component assessment", component)
	return ""
}

func assertHealthState(t *testing.T, events []model.Event, component string, expected model.HealthState) {
	t.Helper()
	for _, event := range events {
		if event.Type != model.TypeHealth {
			continue
		}
		var assessment model.HealthAssessment
		if err := json.Unmarshal(event.Payload, &assessment); err != nil {
			t.Fatal(err)
		}
		if assessment.Component == component && assessment.State == expected {
			return
		}
	}
	t.Fatalf("did not find %s component in %s state", component, expected)
}

func healthyObservations() []model.Observation {
	now := time.Now().UTC()
	return []model.Observation{
		observationAt("cpu.utilization_percent", "cpu", 20, "percent", now),
		observationAt("system.load_1m", "load", 0.5, "load", now),
		observationAt("memory.used_percent", "memory", 40, "percent", now),
		observationAt("filesystem.used_percent", "filesystem", 50, "percent", now),
	}
}

func availableObservation(metric, component string, value float64, unit string) model.Observation {
	return observationAt(metric, component, value, unit, time.Now().UTC())
}

func unavailableObservation(metric, component string) model.Observation {
	unit := "percent"
	switch metric {
	case "network.rx_bytes", "network.tx_bytes":
		unit = "bytes"
	case "process.count":
		unit = "processes"
	case "system.load_1m":
		unit = "load"
	}
	return model.Observation{
		Metric:     metric,
		Component:  component,
		Unit:       unit,
		Available:  false,
		Reason:     "synthetic missing measurement",
		ObservedAt: time.Now().UTC(),
	}
}

func observationAt(metric, component string, value float64, unit string, observedAt time.Time) model.Observation {
	return model.Observation{
		Metric:     metric,
		Component:  component,
		Value:      &value,
		Unit:       unit,
		Available:  true,
		ObservedAt: observedAt,
	}
}
