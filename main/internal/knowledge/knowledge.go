// Package knowledge holds determination patterns: named interpretations of
// combinations of host information (Layer-1). Patterns are data, not code, so
// operators can add or disable them without changing the core.
//
// A pattern matches only when every required fact is present, fresh, and
// satisfies its condition. A missing or unavailable metric never matches, so
// the absence of a determination is not evidence that a condition is absent.
package knowledge

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"perceptronsre/internal/metrics"
)

const (
	OpAtLeast  = ">="
	OpAbove    = ">"
	OpBelow    = "<"
	OpElevated = "elevated" // pressure state HIGH or CRITICAL
	OpNormal   = "normal"   // pressure state NORMAL

	SeverityInfo     = "INFO"
	SeverityWarning  = "WARNING"
	SeverityCritical = "CRITICAL"

	SourceCore      = "core"
	SourceLocal     = "local"
	SourceCommunity = "community"
)

type Condition struct {
	Metric string   `json:"metric"`
	Op     string   `json:"op"`
	Value  *float64 `json:"value,omitempty"`
}

// Pattern matches when every All condition holds and, if Any is non-empty, at
// least one Any condition holds.
type Pattern struct {
	ID             string      `json:"id"`
	Domain         string      `json:"domain"`
	Title          string      `json:"title"`
	Interpretation string      `json:"interpretation"`
	Severity       string      `json:"severity"`
	All            []Condition `json:"all"`
	Any            []Condition `json:"any,omitempty"`
	// Recommendation is optional operator guidance; a domain default is used when empty.
	Recommendation string `json:"recommendation,omitempty"`
	// Source and Weight are set by the loader and ignored in configuration input.
	Source string  `json:"-"`
	Weight float64 `json:"-"`
}

// Fact is one piece of Layer-1 information about a metric.
type Fact struct {
	Metric   string
	Value    *float64
	Unit     string
	Fresh    bool
	Elevated bool
	Critical bool
	Normal   bool
	EventID  string
}

type Evidence struct {
	Metric  string   `json:"metric"`
	Value   *float64 `json:"value,omitempty"`
	Unit    string   `json:"unit"`
	State   string   `json:"state"`
	EventID string   `json:"event_id"`
}

type Match struct {
	Pattern  Pattern
	Severity string
	Evidence []Evidence
}

func (condition Condition) evaluate(facts map[string][]Fact) (Evidence, bool, bool) {
	for _, fact := range facts[condition.Metric] {
		if !fact.Fresh {
			continue
		}
		holds := false
		switch condition.Op {
		case OpElevated:
			holds = fact.Elevated
		case OpNormal:
			holds = fact.Normal
		case OpAtLeast, OpAbove, OpBelow:
			if fact.Value == nil || condition.Value == nil {
				continue
			}
			switch condition.Op {
			case OpAtLeast:
				holds = *fact.Value >= *condition.Value
			case OpAbove:
				holds = *fact.Value > *condition.Value
			case OpBelow:
				holds = *fact.Value < *condition.Value
			}
		}
		if holds {
			state := "observed"
			switch {
			case fact.Critical:
				state = "CRITICAL"
			case fact.Elevated:
				state = "HIGH"
			case fact.Normal:
				state = "NORMAL"
			}
			return Evidence{Metric: fact.Metric, Value: fact.Value, Unit: fact.Unit, State: state, EventID: fact.EventID}, fact.Critical && condition.Op == OpElevated, true
		}
	}
	return Evidence{}, false, false
}

// Evaluate returns a match only when the pattern's conditions all hold.
func (pattern Pattern) Evaluate(facts map[string][]Fact) (Match, bool) {
	match := Match{Pattern: pattern, Severity: pattern.Severity}
	critical := false
	for _, condition := range pattern.All {
		evidence, isCritical, ok := condition.evaluate(facts)
		if !ok {
			return Match{}, false
		}
		critical = critical || isCritical
		match.Evidence = append(match.Evidence, evidence)
	}
	if len(pattern.Any) > 0 {
		anyHolds := false
		for _, condition := range pattern.Any {
			if evidence, isCritical, ok := condition.evaluate(facts); ok {
				anyHolds = true
				critical = critical || isCritical
				match.Evidence = append(match.Evidence, evidence)
			}
		}
		if !anyHolds {
			return Match{}, false
		}
	}
	if critical && pattern.Severity == SeverityWarning {
		match.Severity = SeverityCritical
	}
	return match, true
}

// hasPressureState reports whether the pressure engine evaluates the metric.
func hasPressureState(name string) bool {
	switch name {
	case "cpu.utilization_percent", "memory.used_percent", "filesystem.used_percent", "system.load_1m":
		return true
	}
	def, ok := metrics.Lookup(name)
	return ok && def.Thresholded
}

func validateCondition(condition Condition) error {
	if _, ok := metrics.Lookup(condition.Metric); !ok {
		return fmt.Errorf("metric %q is not in the host metric catalog", condition.Metric)
	}
	switch condition.Op {
	case OpElevated, OpNormal:
		if !hasPressureState(condition.Metric) {
			return fmt.Errorf("metric %q has no pressure threshold, so %q cannot be evaluated; use a value comparison", condition.Metric, condition.Op)
		}
		if condition.Value != nil {
			return fmt.Errorf("%q must not carry a value", condition.Op)
		}
	case OpAtLeast, OpAbove, OpBelow:
		if condition.Value == nil {
			return fmt.Errorf("operator %q requires a value", condition.Op)
		}
	default:
		return fmt.Errorf("unsupported operator %q", condition.Op)
	}
	return nil
}

// Validate checks a pattern's shape and that every metric is cataloged.
func (pattern Pattern) Validate() error {
	if strings.TrimSpace(pattern.ID) == "" || pattern.ID != strings.TrimSpace(pattern.ID) {
		return errors.New("pattern id must be non-empty and trimmed")
	}
	if strings.TrimSpace(pattern.Domain) == "" || strings.TrimSpace(pattern.Title) == "" || strings.TrimSpace(pattern.Interpretation) == "" {
		return fmt.Errorf("pattern %q requires domain, title, and interpretation", pattern.ID)
	}
	switch pattern.Severity {
	case SeverityInfo, SeverityWarning, SeverityCritical:
	default:
		return fmt.Errorf("pattern %q severity must be INFO, WARNING, or CRITICAL", pattern.ID)
	}
	if len(pattern.All)+len(pattern.Any) == 0 {
		return fmt.Errorf("pattern %q requires at least one condition", pattern.ID)
	}
	if len(pattern.All)+len(pattern.Any) > 8 {
		return fmt.Errorf("pattern %q has more than 8 conditions", pattern.ID)
	}
	for _, condition := range append(append([]Condition{}, pattern.All...), pattern.Any...) {
		if err := validateCondition(condition); err != nil {
			return fmt.Errorf("pattern %q: %w", pattern.ID, err)
		}
	}
	return nil
}

// Resolve merges core and local patterns with the default weights.
func Resolve(local []Pattern, disabled []string) ([]Pattern, error) {
	return ResolveWeighted(local, nil, disabled, Weights{})
}

// Weights express operator preference between knowledge sources. They are
// relative (normalized to percentages); a zero weight turns a source off.
// An all-zero value selects DefaultWeights.
type Weights struct {
	Core      float64 `json:"core"`
	Local     float64 `json:"local"`
	Community float64 `json:"community"`
}

func DefaultWeights() Weights { return Weights{Core: 30, Local: 60, Community: 10} }

func (weights Weights) effective() Weights {
	if weights == (Weights{}) {
		return DefaultWeights()
	}
	return weights
}

func (weights Weights) Validate() error {
	w := weights.effective()
	if w.Core < 0 || w.Local < 0 || w.Community < 0 {
		return errors.New("knowledge weights must not be negative")
	}
	if w.Core+w.Local+w.Community <= 0 {
		return errors.New("knowledge weights must not all be zero")
	}
	return nil
}

// Percent returns the source's share of the total weight, 0-100.
func (weights Weights) Percent(source string) float64 {
	w := weights.effective()
	total := w.Core + w.Local + w.Community
	if total <= 0 {
		return 0
	}
	switch source {
	case SourceCore:
		return w.Core / total * 100
	case SourceLocal:
		return w.Local / total * 100
	case SourceCommunity:
		return w.Community / total * 100
	}
	return 0
}

// ResolveWeighted merges core, local and community patterns. When several
// sources define the same ID, the source with the highest weight wins (ties:
// local, then core, then community). Sources with zero weight are ignored.
// Disabled IDs are removed. Output is sorted by ID.
func ResolveWeighted(local, community []Pattern, disabled []string, weights Weights) ([]Pattern, error) {
	if err := weights.Validate(); err != nil {
		return nil, err
	}
	rank := map[string]int{SourceLocal: 3, SourceCore: 2, SourceCommunity: 1}
	byID := make(map[string]Pattern)
	add := func(pattern Pattern, source string) {
		pattern.Source = source
		pattern.Weight = weights.Percent(source)
		if pattern.Weight <= 0 {
			return
		}
		current, exists := byID[pattern.ID]
		if !exists || pattern.Weight > current.Weight || (pattern.Weight == current.Weight && rank[source] > rank[current.Source]) {
			byID[pattern.ID] = pattern
		}
	}
	for _, pattern := range Core() {
		add(pattern, SourceCore)
	}
	for _, group := range []struct {
		source   string
		patterns []Pattern
	}{{SourceLocal, local}, {SourceCommunity, community}} {
		seen := make(map[string]bool)
		for _, pattern := range group.patterns {
			if err := pattern.Validate(); err != nil {
				return nil, err
			}
			if seen[pattern.ID] {
				return nil, fmt.Errorf("%s pattern %q is defined more than once", group.source, pattern.ID)
			}
			seen[pattern.ID] = true
			add(pattern, group.source)
		}
	}
	for _, id := range disabled {
		if _, ok := byID[id]; !ok {
			return nil, fmt.Errorf("disabled pattern %q is not a known active pattern", id)
		}
		delete(byID, id)
	}
	result := make([]Pattern, 0, len(byID))
	for _, pattern := range byID {
		result = append(result, pattern)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

// Metrics lists every metric the pattern depends on.
func (pattern Pattern) Metrics() []string {
	var names []string
	for _, condition := range append(append([]Condition{}, pattern.All...), pattern.Any...) {
		names = append(names, condition.Metric)
	}
	return names
}
