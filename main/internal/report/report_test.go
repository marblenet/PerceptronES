package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"perceptronsre/internal/engine"
	"perceptronsre/internal/model"
)

func TestWriteJSONProducesMachineReadableAssessment(t *testing.T) {
	report := CycleReport{
		HostID: "host-1",
		Runtime: engine.CycleResult{
			CycleID: "cycle-1",
			Mission: &model.MissionAssessment{
				Scope:       model.EntityRef{Type: "host", ID: "host-1"},
				Objective:   "single-host operating condition",
				Status:      model.MissionUnknown,
				Disposition: "unable_to_assess",
				EvaluatedAt: time.Now().UTC(),
			},
		},
	}
	var output bytes.Buffer
	if err := WriteJSON(&output, report); err != nil {
		t.Fatal(err)
	}
	var decoded CycleReport
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("report is not valid JSON: %v", err)
	}
	if decoded.Runtime.Mission == nil || decoded.Runtime.Mission.Status != model.MissionUnknown {
		t.Fatalf("assessment was lost in JSON report: %#v", decoded)
	}
}

func TestWriteTextIncludesUnknownStatusAndReason(t *testing.T) {
	cycle := CycleReport{
		HostID:                  "host-1",
		UnavailableObservations: 1,
		Runtime: engine.CycleResult{
			CycleID: "cycle-1",
			Mission: &model.MissionAssessment{
				Status:      model.MissionUnknown,
				Disposition: "unable_to_assess",
				Objective:   "single-host operating condition",
				Coverage: model.Coverage{
					RequiredComponents: []string{"cpu"},
				},
				Freshness:   "unknown",
				Explanation: "Required evidence is unavailable.",
				Components: []model.HealthAssessment{{
					Component:   "cpu",
					State:       model.HealthUnknown,
					ScopeID:     "host-1",
					Coverage:    "unavailable",
					Freshness:   "unknown",
					Explanation: "collector failure is explicit",
				}},
			},
			Events: []model.Event{
				textObservationEvent(t),
			},
		},
	}
	var output bytes.Buffer
	if err := WriteText(&output, cycle); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Mission: UNKNOWN") ||
		!strings.Contains(output.String(), "test collector failed") ||
		!strings.Contains(output.String(), "collector failure is explicit") {
		t.Fatalf("text report did not explain unknown status: %s", output.String())
	}
}

func TestWritePrometheusExportsCurrentEngineFindings(t *testing.T) {
	value := 92.5
	observationEvent, err := model.NewEvent("cycle-1", "host-observer", model.TypeObservation,
		[]model.EntityRef{{Type: "host", ID: "host-1"}}, nil, model.ReasoningObserved, model.SafetyInformational,
		model.Observation{Metric: "cpu.utilization_percent", Component: "cpu", ScopeID: "host-1", Value: &value, Unit: "percent", Available: true, ObservedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	healthEvent, err := model.NewEvent("cycle-1", "host-health", model.TypeHealth,
		[]model.EntityRef{{Type: "host", ID: "host-1"}}, []string{observationEvent.ID}, model.ReasoningCalculated, model.SafetyReasoning,
		model.HealthAssessment{Component: "cpu", State: model.HealthDegraded, ScopeID: "host-1"})
	if err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	cycle := CycleReport{HostID: "host-1", Runtime: engine.CycleResult{Events: []model.Event{observationEvent, healthEvent}}}
	if err := WritePrometheus(&output, cycle); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"perceptron_observation_value", "metric=\"cpu.utilization_percent\"", "perceptron_component_health", "state=\"DEGRADED\"", "perceptron_engine_finding"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("Prometheus output missing %q: %s", expected, output.String())
		}
	}
}

func textObservationEvent(t *testing.T) model.Event {
	t.Helper()
	observation := model.Observation{
		Metric:     "cpu.utilization_percent",
		Component:  "cpu",
		Unit:       "percent",
		Available:  false,
		Reason:     "test collector failed",
		ObservedAt: time.Now().UTC(),
	}
	event, err := model.NewEvent("cycle-1", "test", model.TypeObservation,
		[]model.EntityRef{{Type: "host", ID: "host-1"}}, nil, model.ReasoningObserved, model.SafetyInformational, observation)
	if err != nil {
		t.Fatal(err)
	}
	return event
}
