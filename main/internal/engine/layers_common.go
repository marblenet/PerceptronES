package engine

import (
	"encoding/json"
	"fmt"

	"perceptronsre/internal/model"
)

// Files in this package are named by cascade layer (layer<N>_<name>.go):
// Layer-1 Information, Layer-2 Determinations, Layer-3 Assessment,
// Layer-4 Recommendation, Layer-5 Policy, Layer-6 Action, Layer-7 Outcome,
// Layer-8 Learning. Layer-0 Input lives in internal/observe. Nothing in the
// layer 4-8 engines executes a command, and no recommendation is authorization.

func deriveEvent(source string, parents []model.Event, eventType string, payload interface{}) (model.Event, error) {
	if len(parents) == 0 {
		return model.Event{}, fmt.Errorf("%s output has no parent events", source)
	}
	var derives []string
	var entities []model.EntityRef
	seenParent := make(map[string]bool)
	seenEntity := make(map[model.EntityRef]bool)
	for _, parent := range parents {
		if seenParent[parent.ID] {
			continue
		}
		seenParent[parent.ID] = true
		derives = append(derives, parent.ID)
		for _, entity := range parent.EntityRefs {
			if !seenEntity[entity] {
				seenEntity[entity] = true
				entities = append(entities, entity)
			}
		}
	}
	first := parents[0]
	output, err := model.NewEvent(first.CorrelationID, source, eventType, entities, derives, model.ReasoningCalculated, model.SafetyReasoning, payload)
	if err != nil {
		return model.Event{}, err
	}
	output.ProducerVersion = model.CoreVersion
	output.ConfigurationID = first.ConfigurationID
	output.CausationID = first.ID
	return output, output.Validate()
}

func decodePayload(event model.Event, target interface{}) error {
	if err := json.Unmarshal(event.Payload, target); err != nil {
		return fmt.Errorf("decode %s %s: %w", event.Type, event.ID, err)
	}
	return nil
}
