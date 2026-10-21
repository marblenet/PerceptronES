package bus

import (
	"context"
	"errors"
	"sync"

	"perceptronsre/internal/model"
)

var ErrDuplicateEvent = errors.New("event ID has already been published")

type Bus struct {
	mu     sync.RWMutex
	events []model.Event
	ids    map[string]struct{}
}

func New() *Bus {
	return &Bus{ids: make(map[string]struct{})}
}

func (b *Bus) Publish(ctx context.Context, event model.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := event.Validate(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.ids[event.ID]; exists {
		return ErrDuplicateEvent
	}
	event.EntityRefs = append([]model.EntityRef(nil), event.EntityRefs...)
	event.DerivesFrom = append([]string(nil), event.DerivesFrom...)
	event.Payload = append(event.Payload[:0:0], event.Payload...)
	b.events = append(b.events, event)
	b.ids[event.ID] = struct{}{}
	return nil
}

func (b *Bus) Snapshot(correlationID string) []model.Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]model.Event, 0, len(b.events))
	for _, event := range b.events {
		if correlationID == "" || event.CorrelationID == correlationID {
			result = append(result, cloneEvent(event))
		}
	}
	return result
}

func (b *Bus) Select(correlationID string, eventTypes []string) []model.Event {
	typeSet := make(map[string]struct{}, len(eventTypes))
	for _, eventType := range eventTypes {
		typeSet[eventType] = struct{}{}
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]model.Event, 0)
	for _, event := range b.events {
		if event.CorrelationID != correlationID {
			continue
		}
		if _, exists := typeSet[event.Type]; exists {
			result = append(result, cloneEvent(event))
		}
	}
	return result
}

func cloneEvent(event model.Event) model.Event {
	event.EntityRefs = append([]model.EntityRef(nil), event.EntityRefs...)
	event.DerivesFrom = append([]string(nil), event.DerivesFrom...)
	event.Payload = append(event.Payload[:0:0], event.Payload...)
	return event
}
