package engine

import (
	"context"
	"strings"
	"time"

	"perceptronsre/internal/knowledge"
	"perceptronsre/internal/model"
)

// Layer-4 Recommendation: determinations become advisory recommendations.
type recommendationEngine struct{}

func (recommendationEngine) Name() string       { return "host-recommendation" }
func (recommendationEngine) Version() string    { return model.CoreVersion }
func (recommendationEngine) Role() string       { return "recommendation" }
func (recommendationEngine) Consumes() []string { return []string{model.TypeDetermination} }
func (recommendationEngine) Produces() []string { return []string{model.TypeRecommendation} }

var domainGuidance = map[string]string{
	"cpu":     "Identify the top CPU consumers and any recent deployment or load change; reduce or reschedule work if the condition persists.",
	"memory":  "Identify the largest memory consumers and check for growth or leaks; free memory or add capacity before the kernel kills processes.",
	"storage": "Identify what is consuming or waiting on storage; free space or inodes, or move heavy I/O, and check device health.",
	"network": "Check connection counts, interface state and errors; identify the peers or services driving the load.",
	"system":  "Check the limit that is being approached (descriptors, PIDs, zombies, kernel state) and the process responsible.",
}

func recommendationFor(determination model.Determination) string {
	if strings.TrimSpace(determination.Recommendation) != "" {
		return determination.Recommendation
	}
	if text, ok := domainGuidance[determination.Domain]; ok {
		return text
	}
	return "Investigate the cited evidence; no domain guidance is defined for this pattern."
}

func (recommendationEngine) Process(ctx context.Context, inputs []model.Event) ([]model.Event, error) {
	var outputs []model.Event
	now := time.Now().UTC()
	for _, input := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var determination model.Determination
		if err := decodePayload(input, &determination); err != nil {
			return nil, err
		}
		priority, action := 3, "investigate"
		switch determination.Severity {
		case knowledge.SeverityCritical:
			priority, action = 1, "escalate_human"
		case knowledge.SeverityWarning:
			priority = 2
		}
		output, err := deriveEvent("host-recommendation", []model.Event{input}, model.TypeRecommendation, model.Recommendation{
			PatternID: determination.PatternID, Domain: determination.Domain, Title: determination.Title,
			Severity: determination.Severity, Priority: priority, ActionKind: action,
			Recommendation: recommendationFor(determination), Advisory: true, EvaluatedAt: now,
		})
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}
