package model

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const EventSchemaVersion = "v2"
const CoreVersion = "0.1.0"

const (
	ReasoningObserved   = "observed"
	ReasoningCalculated = "calculated"
	SafetyInformational = "informational"
	SafetyReasoning     = "reasoning"
	OriginLive          = "live"
	OriginSimulation    = "simulation"
	OriginReplay        = "replay"
	OriginHumanAction   = "human_action"
)

type EntityRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type Classification struct {
	ReasoningKind string `json:"reasoning_kind"`
	SafetyClass   string `json:"safety_class"`
}

type Event struct {
	ID              string          `json:"event_id"`
	CorrelationID   string          `json:"correlation_id"`
	CausationID     string          `json:"causation_id,omitempty"`
	DerivesFrom     []string        `json:"derives_from,omitempty"`
	Source          string          `json:"source"`
	ProducerVersion string          `json:"producer_version,omitempty"`
	ConfigurationID string          `json:"configuration_id,omitempty"`
	Type            string          `json:"type"`
	SchemaVersion   string          `json:"schema_version"`
	Origin          string          `json:"origin"`
	OccurredAt      time.Time       `json:"occurred_at"`
	EmittedAt       time.Time       `json:"emitted_at"`
	EntityRefs      []EntityRef     `json:"entity_refs"`
	Classification  Classification  `json:"classification"`
	Payload         json.RawMessage `json:"payload"`
}

func NewEvent(correlationID, source, eventType string, entityRefs []EntityRef, derivesFrom []string, reasoningKind, safetyClass string, payload interface{}) (Event, error) {
	now := time.Now().UTC()
	return NewEventAt(now, correlationID, source, eventType, entityRefs, derivesFrom, reasoningKind, safetyClass, payload)
}

func NewEventAt(now time.Time, correlationID, source, eventType string, entityRefs []EntityRef, derivesFrom []string, reasoningKind, safetyClass string, payload interface{}) (Event, error) {
	return NewEventWithTimes(now, now, correlationID, source, eventType, entityRefs, derivesFrom, reasoningKind, safetyClass, payload)
}

func NewEventWithTimes(occurredAt, emittedAt time.Time, correlationID, source, eventType string, entityRefs []EntityRef, derivesFrom []string, reasoningKind, safetyClass string, payload interface{}) (Event, error) {
	return NewEventWithTimesAndOrigin(OriginLive, occurredAt, emittedAt, correlationID, source, eventType, entityRefs, derivesFrom, reasoningKind, safetyClass, payload)
}

func NewEventWithTimesAndOrigin(origin string, occurredAt, emittedAt time.Time, correlationID, source, eventType string, entityRefs []EntityRef, derivesFrom []string, reasoningKind, safetyClass string, payload interface{}) (Event, error) {
	id, err := NewID()
	if err != nil {
		return Event{}, fmt.Errorf("generate event ID: %w", err)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("encode event payload: %w", err)
	}
	event := Event{
		ID:            id,
		CorrelationID: correlationID,
		Source:        source,
		Type:          eventType,
		SchemaVersion: EventSchemaVersion,
		Origin:        origin,
		OccurredAt:    occurredAt.UTC(),
		EmittedAt:     emittedAt.UTC(),
		EntityRefs:    append([]EntityRef(nil), entityRefs...),
		DerivesFrom:   append([]string(nil), derivesFrom...),
		Classification: Classification{
			ReasoningKind: reasoningKind,
			SafetyClass:   safetyClass,
		},
		Payload: encoded,
	}
	if err := event.Validate(); err != nil {
		return Event{}, err
	}
	return event, nil
}

func (event Event) Validate() error {
	required := []struct {
		name  string
		value string
	}{
		{"event_id", event.ID},
		{"correlation_id", event.CorrelationID},
		{"source", event.Source},
		{"type", event.Type},
		{"schema_version", event.SchemaVersion},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required", field.name)
		}
	}
	if event.SchemaVersion != EventSchemaVersion {
		return fmt.Errorf("unsupported event schema version %q", event.SchemaVersion)
	}
	switch event.Origin {
	case OriginLive, OriginSimulation, OriginReplay, OriginHumanAction:
	default:
		return fmt.Errorf("unsupported or missing event origin %q", event.Origin)
	}
	if event.ProducerVersion != "" && strings.TrimSpace(event.ProducerVersion) == "" {
		return errors.New("producer_version must not be blank")
	}
	if event.ConfigurationID != "" && strings.TrimSpace(event.ConfigurationID) == "" {
		return errors.New("configuration_id must not be blank")
	}
	if event.OccurredAt.IsZero() || event.EmittedAt.IsZero() {
		return errors.New("occurred_at and emitted_at are required")
	}
	if len(event.EntityRefs) == 0 {
		return errors.New("at least one entity reference is required")
	}
	for i, entity := range event.EntityRefs {
		if strings.TrimSpace(entity.Type) == "" || strings.TrimSpace(entity.ID) == "" {
			return fmt.Errorf("entity_refs[%d] requires type and id", i)
		}
	}
	for i, eventID := range event.DerivesFrom {
		if strings.TrimSpace(eventID) == "" {
			return fmt.Errorf("derives_from[%d] must not be empty", i)
		}
	}
	switch event.Classification.ReasoningKind {
	case ReasoningObserved, ReasoningCalculated:
	default:
		return fmt.Errorf("unsupported reasoning_kind %q", event.Classification.ReasoningKind)
	}
	switch event.Classification.SafetyClass {
	case SafetyInformational, SafetyReasoning:
	default:
		return fmt.Errorf("unsupported safety_class %q", event.Classification.SafetyClass)
	}
	if len(event.Payload) == 0 || !json.Valid(event.Payload) {
		return errors.New("payload must contain valid JSON")
	}
	payload := strings.TrimSpace(string(event.Payload))
	if len(payload) == 0 || payload[0] != '{' {
		return errors.New("payload must be a JSON object")
	}
	return nil
}

func NewID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}
