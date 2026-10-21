package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"perceptronsre/internal/model"
)

type missionEngine struct {
	hostID       string
	dnsProbeName string
}

func (missionEngine) Name() string    { return "perceptron-mission" }
func (missionEngine) Version() string { return model.CoreVersion }
func (missionEngine) Role() string    { return "mission" }
func (missionEngine) Consumes() []string {
	return []string{model.TypeHealth, model.TypeCycle}
}
func (missionEngine) Produces() []string {
	return []string{model.TypeMission}
}

func (engine missionEngine) Process(ctx context.Context, inputs []model.Event) ([]model.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	components := make([]model.HealthAssessment, 0, len(inputs))
	references := make([]string, 0, len(inputs))
	entities := []model.EntityRef{{Type: "host", ID: engine.hostID}}
	correlationID := ""
	for _, input := range inputs {
		if correlationID == "" {
			correlationID = input.CorrelationID
		}
		if input.Type == model.TypeCycle {
			references = append(references, input.ID)
			continue
		}
		if input.Type != model.TypeHealth {
			continue
		}
		var assessment model.HealthAssessment
		if err := json.Unmarshal(input.Payload, &assessment); err != nil {
			return nil, fmt.Errorf("decode health assessment %s: %w", input.ID, err)
		}
		if assessment.Component == "" || assessment.State == "" || assessment.ScopeID == "" {
			return nil, fmt.Errorf("health assessment %s is incomplete", input.ID)
		}
		components = append(components, assessment)
		references = append(references, input.ID)
	}
	mission := assessMission(engine.hostID, engine.dnsProbeName, components, time.Now().UTC())
	if correlationID == "" {
		return nil, fmt.Errorf("mission engine received no cycle context")
	}
	output, err := model.NewEvent(correlationID, (missionEngine{}).Name(), model.TypeMission, entities, references, model.ReasoningCalculated, model.SafetyReasoning, mission)
	if err != nil {
		return nil, fmt.Errorf("create mission assessment: %w", err)
	}
	output.ProducerVersion = (missionEngine{}).Version()
	for _, input := range inputs {
		if input.Type == model.TypeCycle {
			output.ConfigurationID = input.ConfigurationID
			break
		}
	}
	if len(references) > 0 {
		output.CausationID = references[0]
	}
	return []model.Event{output}, nil
}

func assessMission(hostID, dnsProbeName string, components []model.HealthAssessment, evaluatedAt time.Time) model.MissionAssessment {
	required := []string{"cpu", "memory", "load", "filesystem"}
	objective := "host resource headroom for continued operation"
	if dnsProbeName != "" {
		required = append(required, "dns")
		objective += " including DNS resolution for " + dnsProbeName
	}
	assessed := make([]string, 0, len(components))
	present := make(map[string]bool)
	status := model.MissionGreen
	freshness := "not_assessed"
	hasUnknown := false
	for _, component := range components {
		assessed = append(assessed, component.Component)
		present[component.Component] = true
		freshness = lessFresh(freshness, missionFreshness(component.Freshness))
		switch component.State {
		case model.HealthCritical:
			status = model.MissionRed
		case model.HealthDegraded:
			if status != model.MissionRed {
				status = model.MissionYellow
			}
		case model.HealthUnknown:
			hasUnknown = true
			if status != model.MissionRed && status != model.MissionYellow {
				status = model.MissionUnknown
			}
		}
	}
	assessed = sortedUnique(assessed)
	missing := make([]string, 0)
	for _, component := range required {
		if !present[component] {
			missing = append(missing, component)
		}
	}
	if len(missing) > 0 && status != model.MissionRed && status != model.MissionYellow {
		status = model.MissionUnknown
	}
	complete := len(missing) == 0
	coverage := model.Coverage{
		RequiredComponents: required,
		AssessedComponents: assessed,
		MissingComponents:  missing,
		Complete:           complete,
	}

	disposition := "continue"
	explanation := "All required host pressure inputs are covered and below v0.1 warning thresholds."
	switch status {
	case model.MissionYellow:
		disposition = "observe"
		explanation = "One or more required host components have elevated pressure; inspect the component assessments."
		if hasUnknown {
			explanation += " Other required evidence is unavailable or uncertain, so the overall risk may be higher."
		}
	case model.MissionRed:
		disposition = "escalate"
		explanation = "One or more required host components have critical pressure; this assessment does not authorize an action."
		if hasUnknown {
			explanation += " Other required evidence is unavailable or uncertain."
		}
	case model.MissionUnknown:
		disposition = "unable_to_assess"
		explanation = "The host mission cannot be assessed as healthy because required evidence is missing, stale, unavailable, or unknown."
	}
	return model.MissionAssessment{
		Scope:       model.EntityRef{Type: "host", ID: hostID},
		Objective:   objective,
		Status:      status,
		Disposition: disposition,
		Coverage:    coverage,
		Freshness:   freshness,
		EvaluatedAt: evaluatedAt.UTC(),
		Explanation: explanation,
		Components:  components,
	}
}

func lessFresh(current, candidate string) string {
	rank := map[string]int{
		"not_assessed": 0,
		"fresh":        1,
		"stale":        2,
		"future-dated": 3,
		"unknown":      4,
	}
	if rank[candidate] > rank[current] {
		return candidate
	}
	return current
}

func missionFreshness(value string) string {
	switch value {
	case "fresh", "stale", "future-dated":
		return value
	default:
		return "unknown"
	}
}
