package engine

import (
	"context"
	"time"

	"perceptronsre/internal/model"
)

// Layer-8 Learning: outcomes become knowledge candidates. Candidates are
// proposals only and are never applied automatically.
type learningEngine struct{}

func (learningEngine) Name() string       { return "host-learning" }
func (learningEngine) Version() string    { return model.CoreVersion }
func (learningEngine) Role() string       { return "learning" }
func (learningEngine) Consumes() []string { return []string{model.TypeOutcome} }
func (learningEngine) Produces() []string { return []string{model.TypeLearning} }

func (learningEngine) Process(ctx context.Context, inputs []model.Event) ([]model.Event, error) {
	var outputs []model.Event
	now := time.Now().UTC()
	for _, input := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var outcome model.Outcome
		if err := decodePayload(input, &outcome); err != nil {
			return nil, err
		}
		var kind, proposal string
		switch outcome.Status {
		case "resolved":
			kind = "regression_test"
			proposal = "Add a replay scenario in which \"" + outcome.Title + "\" clears, to verify the pattern stops matching when its evidence returns to normal."
		case "worse":
			kind = "knowledge_review"
			proposal = "\"" + outcome.Title + "\" worsened after being determined. Review the pattern severity and thresholds, and record the Why/How chain for this incident."
		default:
			continue
		}
		output, err := deriveEvent("host-learning", []model.Event{input}, model.TypeLearning, model.LearningCandidate{
			PatternID: outcome.PatternID, Kind: kind, Basis: outcome.Status, Proposal: proposal,
			Status: "candidate", Applied: false, EvaluatedAt: now,
		})
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}
