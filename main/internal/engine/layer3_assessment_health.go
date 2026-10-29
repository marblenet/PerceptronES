package engine

import (
	"context"
	"fmt"
	"time"

	"perceptronsre/internal/model"
)

type healthEngine struct{}

func (healthEngine) Name() string    { return "host-health" }
func (healthEngine) Version() string { return model.CoreVersion }
func (healthEngine) Role() string    { return "health" }
func (healthEngine) Consumes() []string {
	return []string{model.TypePressure}
}
func (healthEngine) Produces() []string {
	return []string{model.TypeHealth}
}

func (healthEngine) Process(ctx context.Context, inputs []model.Event) ([]model.Event, error) {
	outputs := make([]model.Event, 0, len(inputs))
	for _, input := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pressure, err := decodePressure(input)
		if err != nil {
			return nil, fmt.Errorf("decode pressure %s: %w", input.ID, err)
		}
		if pressure.State == model.PressureNotEvaluated {
			continue
		}
		health := model.HealthAssessment{
			Component:   pressure.Component,
			State:       healthState(pressure.State),
			Metric:      pressure.Metric,
			ScopeID:     scopeID(input, pressure.Component),
			ObservedAt:  pressure.ObservedAt,
			EvaluatedAt: time.Now().UTC(),
			Freshness:   pressure.Freshness,
			Coverage:    healthCoverage(pressure),
			Explanation: pressure.Explanation,
		}
		output, err := derivedEvent(input, (healthEngine{}).Name(), (healthEngine{}).Version(), model.TypeHealth, health)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}

func healthCoverage(pressure model.PressureAssessment) string {
	if pressure.State == model.PressureUnknown && pressure.Freshness == "unavailable" {
		return "unavailable"
	}
	return "covered"
}

func healthState(state model.PressureState) model.HealthState {
	switch state {
	case model.PressureNormal:
		return model.HealthHealthy
	case model.PressureHigh:
		return model.HealthDegraded
	case model.PressureCritical:
		return model.HealthCritical
	default:
		return model.HealthUnknown
	}
}

func scopeID(event model.Event, component string) string {
	for _, entity := range event.EntityRefs {
		if entity.Type == component || (component == "dns" && entity.Type == "dns-target") {
			return entity.ID
		}
	}
	for _, entity := range event.EntityRefs {
		if entity.Type == "host" {
			return entity.ID
		}
	}
	return ""
}
