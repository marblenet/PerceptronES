package engine

import (
	"context"
	"fmt"
	"time"

	"perceptronsre/internal/config"
	"perceptronsre/internal/knowledge"
	"perceptronsre/internal/model"
)

// determinationEngine (Layer-2 Determinations) applies knowledge patterns to
// Layer-1 information (pressure events) and emits a determination for each
// pattern that matches.
type determinationEngine struct {
	patterns []knowledge.Pattern
}

func newDeterminationEngine(k config.KnowledgeConfig) (*determinationEngine, error) {
	patterns, err := knowledge.ResolveWeighted(k.LocalPatterns, k.CommunityPatterns, k.DisabledPatterns, k.Weights)
	if err != nil {
		return nil, err
	}
	return &determinationEngine{patterns: patterns}, nil
}

func (determinationEngine) Name() string    { return "host-determination" }
func (determinationEngine) Version() string { return model.CoreVersion }
func (determinationEngine) Role() string    { return "determination" }
func (determinationEngine) Consumes() []string {
	return []string{model.TypePressure}
}
func (determinationEngine) Produces() []string {
	return []string{model.TypeDetermination}
}

func (engine *determinationEngine) Process(ctx context.Context, inputs []model.Event) ([]model.Event, error) {
	facts := make(map[string][]knowledge.Fact)
	byID := make(map[string]model.Event, len(inputs))
	for _, input := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pressure, err := decodePressure(input)
		if err != nil {
			return nil, fmt.Errorf("decode pressure %s: %w", input.ID, err)
		}
		byID[input.ID] = input
		facts[pressure.Metric] = append(facts[pressure.Metric], knowledge.Fact{
			Metric:   pressure.Metric,
			Value:    pressure.Value,
			Unit:     pressure.Unit,
			Fresh:    pressure.Freshness == "fresh",
			Elevated: pressure.State == model.PressureHigh || pressure.State == model.PressureCritical,
			Critical: pressure.State == model.PressureCritical,
			Normal:   pressure.State == model.PressureNormal,
			EventID:  input.ID,
		})
	}
	var outputs []model.Event
	now := time.Now().UTC()
	for _, pattern := range engine.patterns {
		match, ok := pattern.Evaluate(facts)
		if !ok {
			continue
		}
		determination := model.Determination{
			PatternID:       pattern.ID,
			KnowledgeSource: pattern.Source,
			KnowledgeWeight: pattern.Weight,
			Domain:          pattern.Domain,
			Title:           pattern.Title,
			Severity:        match.Severity,
			Interpretation:  pattern.Interpretation,
			Recommendation:  pattern.Recommendation,
			EvaluatedAt:     now,
		}
		var derives []string
		var entities []model.EntityRef
		seenInput := make(map[string]bool)
		seenEntity := make(map[model.EntityRef]bool)
		for _, evidence := range match.Evidence {
			determination.Evidence = append(determination.Evidence, model.PatternEvidence{
				Metric: evidence.Metric, Value: evidence.Value, Unit: evidence.Unit, State: evidence.State, EventID: evidence.EventID,
			})
			if seenInput[evidence.EventID] {
				continue
			}
			seenInput[evidence.EventID] = true
			derives = append(derives, evidence.EventID)
			for _, entity := range byID[evidence.EventID].EntityRefs {
				if !seenEntity[entity] {
					seenEntity[entity] = true
					entities = append(entities, entity)
				}
			}
		}
		first := byID[derives[0]]
		output, err := model.NewEvent(first.CorrelationID, engine.Name(), model.TypeDetermination, entities, derives, model.ReasoningCalculated, model.SafetyReasoning, determination)
		if err != nil {
			return nil, fmt.Errorf("create determination %s: %w", pattern.ID, err)
		}
		output.ProducerVersion = engine.Version()
		output.ConfigurationID = first.ConfigurationID
		output.CausationID = first.ID
		if err := output.Validate(); err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}
