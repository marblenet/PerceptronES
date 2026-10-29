package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"perceptronsre/internal/model"
)

type securityEngine struct{}

const securityEvidenceMaxAge = 24 * time.Hour

func (securityEngine) Name() string    { return "security-posture" }
func (securityEngine) Version() string { return model.CoreVersion }
func (securityEngine) Role() string    { return "security assessment" }
func (securityEngine) Consumes() []string {
	return []string{model.TypeSecurityObservation}
}
func (securityEngine) Produces() []string {
	return []string{model.TypeSecurityFinding}
}

func (securityEngine) Process(ctx context.Context, inputs []model.Event) ([]model.Event, error) {
	outputs := make([]model.Event, 0, len(inputs))
	for _, input := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var observation model.SecurityObservation
		if err := json.Unmarshal(input.Payload, &observation); err != nil {
			return nil, fmt.Errorf("decode security observation %s: %w", input.ID, err)
		}
		if observation.Check == "" || observation.Source == "" || observation.Summary == "" || observation.ObservedAt.IsZero() {
			return nil, fmt.Errorf("security observation %s is incomplete", input.ID)
		}
		finding, err := assessSecurity(observation, time.Now().UTC())
		if err != nil {
			return nil, fmt.Errorf("assess security observation %s: %w", input.ID, err)
		}
		output, err := derivedEvent(input, (securityEngine{}).Name(), (securityEngine{}).Version(), model.TypeSecurityFinding, finding)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}

func assessSecurity(observation model.SecurityObservation, evaluatedAt time.Time) (model.SecurityFinding, error) {
	finding := model.SecurityFinding{
		Check:       observation.Check,
		State:       observation.State,
		Source:      observation.Source,
		Summary:     observation.Summary,
		Freshness:   "fresh",
		ObservedAt:  observation.ObservedAt.UTC(),
		EvaluatedAt: evaluatedAt.UTC(),
		Explanation: observation.Summary,
	}
	switch observation.State {
	case "HEALTHY", "WARNING", "CRITICAL", "UNKNOWN":
	default:
		return model.SecurityFinding{}, fmt.Errorf("unsupported state %q", observation.State)
	}
	age := evaluatedAt.Sub(observation.ObservedAt)
	switch {
	case age < -time.Minute:
		finding.State = "UNKNOWN"
		finding.Freshness = "future-dated"
		finding.Explanation += " Evidence timestamp is more than one minute in the future."
	case age > securityEvidenceMaxAge:
		finding.State = "UNKNOWN"
		finding.Freshness = "stale"
		finding.Explanation += fmt.Sprintf(" Evidence is stale: age %s exceeds the %s security-evidence limit.", age.Round(time.Minute), securityEvidenceMaxAge)
	}
	return finding, nil
}
