package engine

import (
	"context"
	"time"

	"perceptronsre/internal/model"
)

// Layer-6 Action: records what happened to each policy decision. No executor
// exists, so Executed is always false.
type actionEngine struct{}

func (actionEngine) Name() string       { return "host-action" }
func (actionEngine) Version() string    { return model.CoreVersion }
func (actionEngine) Role() string       { return "action" }
func (actionEngine) Consumes() []string { return []string{model.TypePolicyDecision} }
func (actionEngine) Produces() []string { return []string{model.TypeAction} }

func (actionEngine) Process(ctx context.Context, inputs []model.Event) ([]model.Event, error) {
	var outputs []model.Event
	now := time.Now().UTC()
	for _, input := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var decision model.PolicyDecision
		if err := decodePayload(input, &decision); err != nil {
			return nil, err
		}
		status, reason := "not_executed", "Authorized, but this build has no action executor; nothing was run."
		switch decision.Decision {
		case "denied":
			status, reason = "blocked", "Policy denied this action."
		case "simulation_only":
			status, reason = "simulation_only", "Policy allows simulation only; nothing was run."
		case "require_approval":
			status, reason = "awaiting_approval", "Human approval is required; no approval workflow exists in this build, so nothing was run."
		}
		output, err := deriveEvent("host-action", []model.Event{input}, model.TypeAction, model.ActionRecord{
			PatternID: decision.PatternID, ActionKind: decision.ActionKind, Decision: decision.Decision,
			Status: status, Executed: false, Reason: reason, EvaluatedAt: now,
		})
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}
