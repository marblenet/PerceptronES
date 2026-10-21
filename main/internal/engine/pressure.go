package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"time"

	"perceptronsre/internal/config"
	"perceptronsre/internal/model"
)

type pressureEngine struct {
	config config.Config
	maxAge time.Duration
}

func newPressureEngine(cfg config.Config) (*pressureEngine, error) {
	maxAge, err := cfg.MaxAgeDuration()
	if err != nil {
		return nil, fmt.Errorf("parse max observation age: %w", err)
	}
	return &pressureEngine{config: cfg, maxAge: maxAge}, nil
}

func (pressureEngine) Name() string    { return "host-pressure" }
func (pressureEngine) Version() string { return model.CoreVersion }
func (pressureEngine) Role() string    { return "pressure" }
func (pressureEngine) Consumes() []string {
	return []string{model.TypeObservation}
}
func (pressureEngine) Produces() []string {
	return []string{model.TypePressure}
}

func (engine *pressureEngine) Process(ctx context.Context, inputs []model.Event) ([]model.Event, error) {
	outputs := make([]model.Event, 0, len(inputs))
	for _, input := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var observation model.Observation
		if err := json.Unmarshal(input.Payload, &observation); err != nil {
			return nil, fmt.Errorf("decode observation %s: %w", input.ID, err)
		}
		if err := validateObservation(observation); err != nil {
			return nil, fmt.Errorf("validate observation %s: %w", input.ID, err)
		}
		assessment := engine.assess(observation, time.Now().UTC())
		output, err := derivedEvent(input, engine.Name(), engine.Version(), model.TypePressure, assessment)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}

func (engine *pressureEngine) assess(observation model.Observation, now time.Time) model.PressureAssessment {
	result := model.PressureAssessment{
		Metric:      observation.Metric,
		Component:   observation.Component,
		State:       model.PressureNotEvaluated,
		Value:       observation.Value,
		Unit:        observation.Unit,
		Freshness:   "fresh",
		ObservedAt:  observation.ObservedAt.UTC(),
		EvaluatedAt: now,
		Explanation: "No v0.1 pressure threshold is defined for this observed counter.",
	}
	isDNSProbe := observation.Metric == "dns.resolution_success"
	threshold, recognized := engine.thresholdFor(observation.Metric)
	recognized = recognized || isDNSProbe
	if !recognized {
		if !observation.Available {
			result.Freshness = "unavailable"
		}
		return result
	}
	if !observation.Available {
		result.State = model.PressureUnknown
		result.Freshness = "unavailable"
		result.Explanation = "The observation is unavailable: " + observation.Reason
		return result
	}
	age := now.Sub(observation.ObservedAt)
	if age < -time.Minute {
		result.State = model.PressureUnknown
		result.Freshness = "future-dated"
		result.Explanation = "The observation timestamp is more than one minute in the future."
		return result
	}
	if age > engine.maxAge {
		result.State = model.PressureUnknown
		result.Freshness = "stale"
		result.Explanation = fmt.Sprintf("The observation is stale: age %s exceeds maximum %s.", age.Round(time.Second), engine.maxAge)
		return result
	}
	if isDNSProbe {
		if *observation.Value == 1 {
			result.State = model.PressureNormal
			result.Explanation = fmt.Sprintf("DNS resolution for %q succeeded in %.3f ms.", observation.Target, valueOrZero(observation.LatencyMillis))
		} else {
			result.State = model.PressureCritical
			result.Explanation = fmt.Sprintf("DNS resolution for %q failed: %s", observation.Target, observation.Reason)
		}
		return result
	}
	result.WarningAt = &threshold.Warning
	result.CriticalAt = &threshold.Critical
	switch {
	case *observation.Value >= threshold.Critical:
		result.State = model.PressureCritical
		result.Explanation = fmt.Sprintf("%s is %.2f %s, at or above the critical threshold %.2f.", observation.Metric, *observation.Value, observation.Unit, threshold.Critical)
	case *observation.Value >= threshold.Warning:
		result.State = model.PressureHigh
		result.Explanation = fmt.Sprintf("%s is %.2f %s, at or above the warning threshold %.2f.", observation.Metric, *observation.Value, observation.Unit, threshold.Warning)
	default:
		result.State = model.PressureNormal
		result.Explanation = fmt.Sprintf("%s is %.2f %s, below the warning threshold %.2f.", observation.Metric, *observation.Value, observation.Unit, threshold.Warning)
	}

	return result
}

func valueOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func (engine *pressureEngine) thresholdFor(metric string) (config.Threshold, bool) {
	switch metric {
	case "cpu.utilization_percent":
		return engine.config.Thresholds.CPUPercent, true
	case "memory.used_percent":
		return engine.config.Thresholds.MemoryPercent, true
	case "filesystem.used_percent":
		return engine.config.Thresholds.FilesystemPercent, true
	case "system.load_1m":
		cpuCount := float64(runtime.NumCPU())
		if cpuCount <= 0 {
			return config.Threshold{}, false
		}
		return config.Threshold{
			Warning:  engine.config.Thresholds.LoadPerCPU.Warning * cpuCount,
			Critical: engine.config.Thresholds.LoadPerCPU.Critical * cpuCount,
		}, true
	default:
		return config.Threshold{}, false
	}
}

func derivedEvent(input model.Event, source, producerVersion, eventType string, payload interface{}) (model.Event, error) {
	event, err := model.NewEvent(input.CorrelationID, source, eventType, input.EntityRefs, []string{input.ID}, model.ReasoningCalculated, model.SafetyReasoning, payload)
	if err != nil {
		return model.Event{}, fmt.Errorf("create %s event: %w", eventType, err)
	}
	event.ProducerVersion = producerVersion
	event.ConfigurationID = input.ConfigurationID
	event.CausationID = input.ID
	if err := event.Validate(); err != nil {
		return model.Event{}, err
	}
	return event, nil
}

func decodePressure(event model.Event) (model.PressureAssessment, error) {
	var pressure model.PressureAssessment
	if err := json.Unmarshal(event.Payload, &pressure); err != nil {
		return model.PressureAssessment{}, err
	}
	if pressure.Metric == "" || pressure.Component == "" || pressure.Explanation == "" {
		return model.PressureAssessment{}, errors.New("pressure assessment is incomplete")
	}
	return pressure, nil
}
