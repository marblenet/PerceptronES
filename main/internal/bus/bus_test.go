package bus

import (
	"context"
	"testing"

	"perceptronsre/internal/model"
)

func TestPublishAppendsAndRejectsDuplicateIDs(t *testing.T) {
	eventBus := New()
	event, err := testEvent()
	if err != nil {
		t.Fatal(err)
	}
	if err := eventBus.Publish(context.Background(), event); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}
	if err := eventBus.Publish(context.Background(), event); err != ErrDuplicateEvent {
		t.Fatalf("expected duplicate-event error, got %v", err)
	}
	snapshot := eventBus.Snapshot("cycle-1")
	if len(snapshot) != 1 || snapshot[0].ID != event.ID {
		t.Fatalf("unexpected event snapshot: %#v", snapshot)
	}
}

func TestPublishRejectsInvalidEventWithoutStoring(t *testing.T) {
	eventBus := New()
	event, err := testEvent()
	if err != nil {
		t.Fatal(err)
	}
	event.Payload = []byte("null")
	if err := eventBus.Publish(context.Background(), event); err == nil {
		t.Fatal("expected invalid event to be rejected")
	}
	if got := len(eventBus.Snapshot("")); got != 0 {
		t.Fatalf("invalid event was stored: %d events", got)
	}
}

func TestPublishHonorsCancellation(t *testing.T) {
	eventBus := New()
	event, err := testEvent()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := eventBus.Publish(ctx, event); err == nil {
		t.Fatal("expected canceled publish to fail")
	}
}

func TestSnapshotDoesNotExposeStoredPayload(t *testing.T) {
	eventBus := New()
	event, err := testEvent()
	if err != nil {
		t.Fatal(err)
	}
	if err := eventBus.Publish(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	snapshot := eventBus.Snapshot("cycle-1")
	snapshot[0].Payload[0] = 'x'
	again := eventBus.Snapshot("cycle-1")
	if again[0].Payload[0] != '{' {
		t.Fatalf("caller mutated stored event payload: %s", again[0].Payload)
	}
}

func testEvent() (model.Event, error) {
	return model.NewEvent("cycle-1", "test", model.TypeObservation,
		[]model.EntityRef{{Type: "host", ID: "host-1"}}, nil, model.ReasoningObserved, model.SafetyInformational,
		map[string]string{"metric": "cpu"})
}
