package engine

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"perceptronsre/internal/config"
	"perceptronsre/internal/model"
)

func eventsOfType(events []model.Event, eventType string) []model.Event {
	var result []model.Event
	for _, event := range events {
		if event.Type == eventType {
			result = append(result, event)
		}
	}
	return result
}

func TestCascadeRecommendationPolicyActionOutcomeLearning(t *testing.T) {
	cfg := config.Default()
	cfg.Policy.Rules = map[string]string{"domain:storage": "denied"}
	runtime, err := NewRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	full := []model.Observation{
		observationAt("cpu.utilization_percent", "cpu", 20, "percent", now),
		observationAt("system.load_1m", "load", 0.5, "load", now),
		observationAt("memory.used_percent", "memory", 40, "percent", now),
		observationAt("filesystem.used_percent", "filesystem", 97, "percent", now),
	}
	first, err := runtime.Run(context.Background(), "cycle-1", full)
	if err != nil {
		t.Fatal(err)
	}
	if len(eventsOfType(first.Events, model.TypeDetermination)) == 0 {
		t.Fatal("expected a disk determination")
	}
	recommendations := eventsOfType(first.Events, model.TypeRecommendation)
	if len(recommendations) == 0 {
		t.Fatal("expected a recommendation")
	}
	var recommendation model.Recommendation
	if err := json.Unmarshal(recommendations[0].Payload, &recommendation); err != nil || !recommendation.Advisory || recommendation.ActionKind != "escalate_human" {
		t.Fatalf("unexpected recommendation %#v %v", recommendation, err)
	}
	var decision model.PolicyDecision
	policies := eventsOfType(first.Events, model.TypePolicyDecision)
	if len(policies) == 0 {
		t.Fatal("expected a policy decision")
	}
	if err := json.Unmarshal(policies[0].Payload, &decision); err != nil || decision.Decision != "denied" {
		t.Fatalf("domain rule should deny: %#v %v", decision, err)
	}
	var action model.ActionRecord
	actions := eventsOfType(first.Events, model.TypeAction)
	if len(actions) == 0 {
		t.Fatal("expected an action record")
	}
	if err := json.Unmarshal(actions[0].Payload, &action); err != nil || action.Executed || action.Status != "blocked" {
		t.Fatalf("unexpected action record %#v %v", action, err)
	}
	if len(eventsOfType(first.Events, model.TypeOutcome)) != 0 {
		t.Fatal("first cycle has no previous determinations, so no outcomes")
	}

	full[3] = observationAt("filesystem.used_percent", "filesystem", 50, "percent", time.Now().UTC())
	second, err := runtime.Run(context.Background(), "cycle-2", full)
	if err != nil {
		t.Fatal(err)
	}
	outcomes := eventsOfType(second.Events, model.TypeOutcome)
	if len(outcomes) == 0 {
		t.Fatal("expected a resolved outcome")
	}
	var outcome model.Outcome
	if err := json.Unmarshal(outcomes[0].Payload, &outcome); err != nil || outcome.Status != "resolved" || outcome.ActionAttributed {
		t.Fatalf("unexpected outcome %#v %v", outcome, err)
	}
	learning := eventsOfType(second.Events, model.TypeLearning)
	if len(learning) == 0 {
		t.Fatal("expected a learning candidate")
	}
	var candidate model.LearningCandidate
	if err := json.Unmarshal(learning[0].Payload, &candidate); err != nil || candidate.Applied || candidate.Kind != "regression_test" {
		t.Fatalf("unexpected learning candidate %#v %v", candidate, err)
	}
}

func TestOutcomeIsUnknownWhenEvidenceIsMissing(t *testing.T) {
	runtime, err := NewRuntime(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	bad := []model.Observation{
		observationAt("cpu.utilization_percent", "cpu", 20, "percent", now),
		observationAt("system.load_1m", "load", 0.5, "load", now),
		observationAt("memory.used_percent", "memory", 40, "percent", now),
		observationAt("filesystem.used_percent", "filesystem", 97, "percent", now),
	}
	if _, err := runtime.Run(context.Background(), "c1", bad); err != nil {
		t.Fatal(err)
	}
	bad[3] = unavailableObservation("filesystem.used_percent", "filesystem")
	second, err := runtime.Run(context.Background(), "c2", bad)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range eventsOfType(second.Events, model.TypeOutcome) {
		var outcome model.Outcome
		if err := json.Unmarshal(event.Payload, &outcome); err != nil {
			t.Fatal(err)
		}
		if outcome.Status == "resolved" {
			t.Fatalf("resolved must not be claimed without fresh evidence: %#v", outcome)
		}
	}
	if len(eventsOfType(second.Events, model.TypeOutcome)) == 0 {
		t.Fatal("expected an unknown outcome")
	}
}
