package simulation

import (
	"context"
	"encoding/json"
	"testing"

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
