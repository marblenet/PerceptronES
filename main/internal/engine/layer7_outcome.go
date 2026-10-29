package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"perceptronsre/internal/config"
	"perceptronsre/internal/knowledge"
	"perceptronsre/internal/model"
)

type previousDetermination struct {
	severity string
	title    string
	domain   string
}

// Layer-7 Outcome compares determinations across live cycles. It keeps the
// last live cycle's determinations in memory, so the first cycle after start
// has no outcomes. "Resolved" requires fresh evidence for every metric the
// pattern uses; otherwise the outcome is unknown.
type outcomeEngine struct {
	patterns map[string]knowledge.Pattern
	mu       sync.Mutex
	previous map[string]previousDetermination
}

func newOutcomeEngine(k config.KnowledgeConfig) (*outcomeEngine, error) {
	resolved, err := knowledge.ResolveWeighted(k.LocalPatterns, k.CommunityPatterns, k.DisabledPatterns, k.Weights)
	if err != nil {
		return nil, err
	}
	patterns := make(map[string]knowledge.Pattern, len(resolved))
	for _, pattern := range resolved {
		patterns[pattern.ID] = pattern
	}
	return &outcomeEngine{patterns: patterns, previous: make(map[string]previousDetermination)}, nil
}

func (*outcomeEngine) Name() string    { return "host-outcome" }
func (*outcomeEngine) Version() string { return model.CoreVersion }
func (*outcomeEngine) Role() string    { return "outcome" }
func (*outcomeEngine) Consumes() []string {
	return []string{model.TypePressure, model.TypeDetermination}
}
func (*outcomeEngine) Produces() []string { return []string{model.TypeOutcome} }

func severityRank(severity string) int {
	switch severity {
	case knowledge.SeverityCritical:
		return 3
	case knowledge.SeverityWarning:
		return 2
	}
	return 1
}

func (engine *outcomeEngine) Process(ctx context.Context, inputs []model.Event) ([]model.Event, error) {
	facts := make(map[string][]knowledge.Fact)
	pressureByMetric := make(map[string]model.Event)
	current := make(map[string]model.Event)
	currentPayload := make(map[string]model.Determination)
	live := true
	for _, input := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if input.Origin != model.OriginLive {
			live = false
		}
		switch input.Type {
		case model.TypePressure:
			pressure, err := decodePressure(input)
			if err != nil {
				return nil, fmt.Errorf("decode pressure %s: %w", input.ID, err)
			}
			pressureByMetric[pressure.Metric] = input
			facts[pressure.Metric] = append(facts[pressure.Metric], knowledge.Fact{Metric: pressure.Metric, Value: pressure.Value, Fresh: pressure.Freshness == "fresh"})
		case model.TypeDetermination:
			var determination model.Determination
			if err := decodePayload(input, &determination); err != nil {
				return nil, err
			}
			current[determination.PatternID] = input
			currentPayload[determination.PatternID] = determination
		}
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	var outputs []model.Event
	now := time.Now().UTC()
	emit := func(id string, previous previousDetermination, status, currentSeverity, explanation string, parents []model.Event) error {
		if len(parents) == 0 {
			parents = inputs[:1]
		}
		output, err := deriveEvent("host-outcome", parents, model.TypeOutcome, model.Outcome{
			PatternID: id, Title: previous.title, Domain: previous.domain, Status: status,
			PreviousSeverity: previous.severity, CurrentSeverity: currentSeverity,
			ActionAttributed: false, Explanation: explanation + " No action was executed, so this change is not attributed to any action.", EvaluatedAt: now,
		})
		if err != nil {
			return err
		}
		outputs = append(outputs, output)
		return nil
	}
	next := make(map[string]previousDetermination, len(current))
	for id, previous := range engine.previous {
		if event, ok := current[id]; ok {
			severity := currentPayload[id].Severity
			status, text := "unchanged", "The condition is still present at the same severity."
			switch {
			case severityRank(severity) > severityRank(previous.severity):
				status, text = "worse", "The condition is still present and its severity rose."
			case severityRank(severity) < severityRank(previous.severity):
				status, text = "improved", "The condition is still present but its severity fell."
			}
			if err := emit(id, previous, status, severity, text, []model.Event{event}); err != nil {
				return nil, err
			}
			continue
		}
		pattern, known := engine.patterns[id]
		var parents []model.Event
		allFresh := known
		if known {
			for _, metric := range pattern.Metrics() {
				fresh := false
				for _, fact := range facts[metric] {
					fresh = fresh || fact.Fresh
				}
				if !fresh {
					allFresh = false
					continue
				}
				parents = append(parents, pressureByMetric[metric])
			}
		}
		if allFresh {
			if err := emit(id, previous, "resolved", "", "The pattern no longer matches on fresh evidence for every metric it uses.", parents); err != nil {
				return nil, err
			}
			continue
		}
		if err := emit(id, previous, "unknown", "", "The pattern no longer matches, but evidence is missing or stale, so resolution cannot be concluded.", nil); err != nil {
			return nil, err
		}
		next[id] = previous
	}
	for id, payload := range currentPayload {
		next[id] = previousDetermination{severity: payload.Severity, title: payload.Title, domain: payload.Domain}
	}
	if live {
		engine.previous = next
	}
	return outputs, nil
}
