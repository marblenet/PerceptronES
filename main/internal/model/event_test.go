package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewEventCreatesValidImmutableEnvelope(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	event, err := NewEventAt(now, "cycle-1", "collector", TypeObservation,
		[]EntityRef{{Type: "host", ID: "host-1"}}, nil, ReasoningObserved, SafetyInformational,
		map[string]interface{}{"metric": "cpu.utilization_percent"})
	if err != nil {
		t.Fatalf("NewEventAt returned error: %v", err)
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("event is invalid: %v", err)
	}
	if event.ID == "" || event.OccurredAt != now || event.EmittedAt != now {
		t.Fatalf("unexpected generated envelope: %#v", event)
	}
	if !json.Valid(event.Payload) {
		t.Fatalf("event payload is not valid JSON: %s", event.Payload)
	}
}

func TestEventRejectsUnsupportedVersionAndMissingScope(t *testing.T) {
	event, err := NewEvent("cycle-1", "collector", TypeObservation,
		[]EntityRef{{Type: "host", ID: "host-1"}}, nil, ReasoningObserved, SafetyInformational, map[string]int{"x": 1})
	if err != nil {
		t.Fatal(err)
	}
	event.SchemaVersion = "v999"
	if err := event.Validate(); err == nil {
		t.Fatal("expected unsupported schema version to be rejected")
	}
	event.SchemaVersion = EventSchemaVersion
	event.EntityRefs = nil
	if err := event.Validate(); err == nil {
		t.Fatal("expected missing entity scope to be rejected")
	}
}

func TestNewIDIsUniqueAndUUIDShaped(t *testing.T) {
	first, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	if first == second || len(first) != 36 {
		t.Fatalf("expected unique UUID-shaped IDs, got %q and %q", first, second)
	}
}

func TestEventSeparatesOccurrenceAndEmissionTimes(t *testing.T) {
	occurred := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	emitted := occurred.Add(250 * time.Millisecond)
	event, err := NewEventWithTimes(occurred, emitted, "cycle-1", "collector", TypeObservation,
		[]EntityRef{{Type: "host", ID: "host-1"}}, nil, ReasoningObserved, SafetyInformational, map[string]string{"metric": "cpu"})
	if err != nil {
		t.Fatal(err)
	}
	if !event.OccurredAt.Equal(occurred) || !event.EmittedAt.Equal(emitted) {
		t.Fatalf("event occurrence and emission timestamps were conflated: %#v", event)
	}
}

func TestEventOriginIsRequiredAndVersioned(t *testing.T) {
	event, err := NewEventWithTimesAndOrigin(OriginSimulation, time.Now().UTC(), time.Now().UTC(),
		"cycle-sim", "simulator", TypeObservation, []EntityRef{{Type: "host", ID: "host-1"}},
		nil, ReasoningObserved, SafetyInformational, map[string]string{"metric": "cpu"})
	if err != nil {
		t.Fatal(err)
	}
	if event.Origin != OriginSimulation || event.SchemaVersion != EventSchemaVersion {
		t.Fatalf("event origin/schema was not recorded: %#v", event)
	}
	event.Origin = ""
	if err := event.Validate(); err == nil {
		t.Fatal("missing event origin must be rejected")
	}
}
