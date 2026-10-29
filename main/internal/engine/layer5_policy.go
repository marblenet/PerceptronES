package engine

import (
	"context"
	"time"

	"perceptronsre/internal/model"
)

// Layer-5 Policy: can we do this? Decisions are keyed by pattern ID, then
// "domain:<name>", then the configured default.
type policyEngine struct {
	defaultDecision string
	rules           map[string]string
}

func (policyEngine) Name() string       { return "host-policy" }
func (policyEngine) Version() string    { return model.CoreVersion }
func (policyEngine) Role() string       { return "policy" }
func (policyEngine) Consumes() []string { return []string{model.TypeRecommendation} }
func (policyEngine) Produces() []string { return []string{model.TypePolicyDecision} }

func (engine policyEngine) decide(recommendation model.Recommendation) (decision, rule string) {
	if value, ok := engine.rules[recommendation.PatternID]; ok {
		return value, "rule:" + recommendation.PatternID
	}
	if value, ok := engine.rules["domain:"+recommendation.Domain]; ok {
		return value, "rule:domain:" + recommendation.Domain
	}
	if engine.defaultDecision != "" {
		return engine.defaultDecision, "default"
	}
	return "require_approval", "built-in default"
}

func (engine policyEngine) Process(ctx context.Context, inputs []model.Event) ([]model.Event, error) {
	var outputs []model.Event
	now := time.Now().UTC()
	for _, input := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var recommendation model.Recommendation
		if err := decodePayload(input, &recommendation); err != nil {
			return nil, err
		}
		decision, rule := engine.decide(recommendation)
		output, err := deriveEvent("host-policy", []model.Event{input}, model.TypePolicyDecision, model.PolicyDecision{
			PatternID: recommendation.PatternID, ActionKind: recommendation.ActionKind, Decision: decision, Rule: rule,
			Explanation: "Policy decision " + decision + " from " + rule + ". Policy answers whether an action may be taken; it does not perform it.",
			EvaluatedAt: now,
		})
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}
