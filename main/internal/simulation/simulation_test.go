package simulation

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"perceptronsre/internal/config"
	"perceptronsre/internal/model"
)

func TestRunScenarioKeepsSyntheticResultSeparateAndReportsExpectedState(t *testing.T) {
	cfg := config.Default()
	result, err := RunScenario(context.Background(), cfg, "cpu-critical")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Synthetic || result.Report.Runtime.Mission == nil || result.Report.Runtime.Mission.Status != model.MissionRed {
		t.Fatalf("critical CPU simulation did not produce a synthetic RED assessment: %#v", result)
	}
	if result.Report.Runtime.CycleID[:11] != "simulation-" {
		t.Fatalf("simulation cycle is not labeled as synthetic: %q", result.Report.Runtime.CycleID)
	}
	for _, event := range result.Report.Runtime.Events {
		if event.Origin != model.OriginSimulation {
			t.Fatalf("synthetic event lost simulation origin: %#v", event)
		}
	}
}

func TestRunSnapshotForksObservationsAsSimulation(t *testing.T) {
	source, err := RunScenario(context.Background(), config.Default(), "normal")
	if err != nil {
		t.Fatal(err)
	}
	var snapshot []model.Observation
	for _, event := range source.Report.Runtime.Events {
		if event.Type == model.TypeObservation {
			var observation model.Observation
			if err := json.Unmarshal(event.Payload, &observation); err != nil {
				t.Fatal(err)
			}
			snapshot = append(snapshot, observation)
		}
	}
	critical := 99.0
	result, err := RunSnapshot(context.Background(), config.Default(), "live-cycle-1", snapshot, Inputs{CPUPercent: &critical})
	if err != nil {
		t.Fatal(err)
	}
	if result.SnapshotOf != "live-cycle-1" || result.Report.Runtime.Mission == nil || result.Report.Runtime.Mission.Status != model.MissionRed {
		t.Fatalf("snapshot fork did not apply override on captured observations: %#v", result.SnapshotOf)
	}
	if *snapshot[0].Value == critical {
		t.Fatal("snapshot input was mutated by the fork")
	}
	for _, event := range result.Report.Runtime.Events {
		if event.Origin != model.OriginSimulation {
			t.Fatalf("snapshot event lost simulation origin: %#v", event)
		}
	}
	if _, err := RunSnapshot(context.Background(), config.Default(), "x", nil, Inputs{}); err == nil {
		t.Fatal("empty snapshot must be rejected")
	}
}

func TestRunSnapshotKeepsUnavailableObservationsUnknown(t *testing.T) {
	cpu, memory := 20.0, 40.0
	snapshot := []model.Observation{
		{Metric: "cpu.utilization_percent", Component: "cpu", Value: &cpu, Unit: "percent", Available: true, ObservedAt: time.Now().UTC()},
		{Metric: "memory.used_percent", Component: "memory", Unit: "percent", Available: false, Reason: "collector failed", ObservedAt: time.Now().UTC()},
	}
	override := 30.0
	result, err := RunSnapshot(context.Background(), config.Default(), "live-2", snapshot, Inputs{CPUPercent: &override, MemoryPercent: &memory})
	if err != nil {
		t.Fatalf("unavailable live observation must not fail the fork: %v", err)
	}
	if result.Report.Runtime.Mission == nil || result.Report.Runtime.Mission.Status == model.MissionGreen {
		t.Fatalf("missing memory evidence must not produce GREEN: %#v", result.Report.Runtime.Mission)
	}
}

func TestBuildLiveIncidentLabelsChainsLiveAndKeepsUnknowns(t *testing.T) {
	incident := BuildLiveIncident("c1", nil)
	for _, link := range append(append([]ChainLink{}, incident.WhyChain...), incident.HowChain...) {
		if link.Origin != model.OriginLive {
			t.Fatalf("live chain link has wrong origin: %#v", link)
		}
	}
	if incident.WhyChain[len(incident.WhyChain)-1].Status != "unknown" {
		t.Fatal("root cause must remain unknown")
	}
}

func TestRunSecurityScenarioProducesSyntheticSecurityFinding(t *testing.T) {
	result, err := RunScenario(context.Background(), config.Default(), "security-critical")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range result.Report.Runtime.Events {
		if event.Type != model.TypeSecurityFinding {
			continue
		}
		found = true
		if event.Source != "security-posture" {
			t.Fatalf("security finding was not produced by the configured engine: %#v", event)
		}
	}
	if !found {
		t.Fatal("security scenario did not produce a finding")
	}
	if result.Report.Runtime.Mission.Status != model.MissionRed ||
		!result.Report.Runtime.Mission.Coverage.Complete {
		t.Fatalf("critical synthetic security evidence did not affect mission status: %#v", result.Report.Runtime.Mission)
	}
	if !contains(result.Report.Runtime.Mission.Coverage.RequiredComponents, "security:sample-security-control") {
		t.Fatalf("simulation did not declare the synthetic security check scope: %#v", result.Report.Runtime.Mission.Coverage)
	}
}

func TestRunScenarioReportsUnknownForMissingAndStaleEvidence(t *testing.T) {
	for _, name := range []string{"missing-memory", "stale-memory"} {
		result, err := RunScenario(context.Background(), config.Default(), name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if result.Report.Runtime.Mission == nil || result.Report.Runtime.Mission.Status != model.MissionUnknown {
			t.Errorf("%s scenario did not remain UNKNOWN: %#v", name, result.Report.Runtime.Mission)
		}
	}
}

func TestRunScenarioRejectsUnknownName(t *testing.T) {
	if _, err := RunScenario(context.Background(), config.Default(), "not-a-scenario"); err == nil {
		t.Fatal("unknown scenario must fail explicitly")
	}
}

func TestRunScenarioAppliesSyntheticMeasurementAndThresholdInputs(t *testing.T) {
	cpu := 79.0
	thresholds := config.Default().Thresholds
	thresholds.CPUPercent.Warning = 75
	thresholds.CPUPercent.Critical = 90
	result, err := RunScenarioWithInputs(context.Background(), config.Default(), "normal", Inputs{
		CPUPercent: &cpu,
		Thresholds: &thresholds,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Report.Runtime.Mission.Status != model.MissionYellow {
		t.Fatalf("custom threshold did not produce the expected simulated warning: %#v", result.Report.Runtime.Mission)
	}
	if result.IncidentReport.Status != string(model.MissionYellow) || len(result.IncidentReport.ContributingFactors) == 0 {
		t.Fatalf("simulation did not produce a useful incident summary: %#v", result.IncidentReport)
	}
	for _, link := range append(append([]ChainLink{}, result.IncidentReport.WhyChain...), result.IncidentReport.HowChain...) {
		if link.Origin != model.OriginSimulation || link.Status == "" {
			t.Fatalf("chain link not labeled as simulation: %#v", link)
		}
	}
	whyLast := result.IncidentReport.WhyChain[len(result.IncidentReport.WhyChain)-1]
	if whyLast.Status != "unknown" {
		t.Fatalf("root cause must remain unknown, got %#v", whyLast)
	}
	for _, event := range result.Report.Runtime.Events {
		if event.Type != model.TypeObservation {
			continue
		}
		var observation model.Observation
		if err := json.Unmarshal(event.Payload, &observation); err != nil {
			t.Fatal(err)
		}
		if observation.Component == "cpu" && (observation.Value == nil || *observation.Value != cpu) {
			t.Fatalf("synthetic CPU input was not applied: %#v", observation)
		}
	}
}

func TestRunScenarioRejectsOutOfRangeSyntheticInputs(t *testing.T) {
	cpu := 101.0
	if _, err := RunScenarioWithInputs(context.Background(), config.Default(), "normal", Inputs{CPUPercent: &cpu}); err == nil {
		t.Fatal("out-of-range synthetic input must be rejected")
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
