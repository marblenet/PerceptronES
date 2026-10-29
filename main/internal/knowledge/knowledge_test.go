package knowledge

import "testing"

func fact(metric string, value float64, elevated, critical, normal bool) Fact {
	return Fact{Metric: metric, Value: &value, Unit: "x", Fresh: true, Elevated: elevated, Critical: critical, Normal: normal, EventID: "e-" + metric}
}

func facts(items ...Fact) map[string][]Fact {
	result := make(map[string][]Fact)
	for _, item := range items {
		result[item.Metric] = append(result[item.Metric], item)
	}
	return result
}

func TestCorePatternsAreValidAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, pattern := range Core() {
		if err := pattern.Validate(); err != nil {
			t.Fatalf("core pattern invalid: %v", err)
		}
		if seen[pattern.ID] {
			t.Fatalf("duplicate core pattern %s", pattern.ID)
		}
		seen[pattern.ID] = true
	}
}

func find(t *testing.T, id string) Pattern {
	t.Helper()
	for _, pattern := range Core() {
		if pattern.ID == id {
			return pattern
		}
	}
	t.Fatalf("missing pattern %s", id)
	return Pattern{}
}

func TestStorageBottleneckNeedsLowCPUAndWaiting(t *testing.T) {
	pattern := find(t, "core.storage.bottleneck")
	if _, ok := pattern.Evaluate(facts(fact("cpu.utilization_percent", 10, false, false, true), fact("cpu.iowait_percent", 35, true, false, false))); !ok {
		t.Fatal("expected a match")
	}
	if _, ok := pattern.Evaluate(facts(fact("cpu.utilization_percent", 90, true, false, false), fact("cpu.iowait_percent", 35, true, false, false))); ok {
		t.Fatal("busy CPU must not match a storage bottleneck")
	}
}

func TestUnavailableOrStaleFactsNeverMatch(t *testing.T) {
	pattern := find(t, "core.storage.bottleneck")
	stale := fact("cpu.iowait_percent", 35, true, false, false)
	stale.Fresh = false
	if _, ok := pattern.Evaluate(facts(fact("cpu.utilization_percent", 10, false, false, true), stale)); ok {
		t.Fatal("stale evidence matched")
	}
	missing := Fact{Metric: "cpu.utilization_percent", Fresh: true}
	if _, ok := pattern.Evaluate(facts(missing, fact("cpu.iowait_percent", 35, true, false, false))); ok {
		t.Fatal("a fact without a value satisfied a value comparison")
	}
	if _, ok := pattern.Evaluate(facts()); ok {
		t.Fatal("empty facts matched")
	}
}

func TestCriticalEvidenceRaisesWarningToCritical(t *testing.T) {
	match, ok := find(t, "core.storage.disk_full").Evaluate(facts(fact("filesystem.used_percent", 99, true, true, false)))
	if !ok || match.Severity != SeverityCritical {
		t.Fatalf("unexpected match %#v ok=%v", match, ok)
	}
	match, ok = find(t, "core.storage.disk_full").Evaluate(facts(fact("filesystem.used_percent", 90, true, false, false)))
	if !ok || match.Severity != SeverityWarning {
		t.Fatalf("unexpected match %#v ok=%v", match, ok)
	}
}

func TestWeightsDecideBetweenSourcesWithTheSameID(t *testing.T) {
	community := Pattern{ID: "core.storage.disk_full", Domain: "storage", Title: "Community", Interpretation: "c", Severity: SeverityInfo,
		All: []Condition{{Metric: "filesystem.used_percent", Op: OpElevated}}}
	titleOf := func(weights Weights) string {
		patterns, err := ResolveWeighted(nil, []Pattern{community}, nil, weights)
		if err != nil {
			t.Fatal(err)
		}
		for _, pattern := range patterns {
			if pattern.ID == community.ID {
				return pattern.Title + "/" + pattern.Source
			}
		}
		return ""
	}
	if got := titleOf(Weights{}); got == "Community/community" {
		t.Fatalf("default weights must prefer core over community, got %s", got)
	}
	if got := titleOf(Weights{Core: 10, Community: 90}); got != "Community/community" {
		t.Fatalf("heavier community weight must win, got %s", got)
	}
	if got := titleOf(Weights{Core: 100, Community: 0}); got == "Community/community" {
		t.Fatalf("zero-weight source must be ignored, got %s", got)
	}
	if err := (Weights{Core: -1, Local: 5}).Validate(); err == nil {
		t.Fatal("negative weight must be rejected")
	}
}

func TestResolveLocalOverridesAndDisables(t *testing.T) {
	override := Pattern{ID: "core.storage.disk_full", Domain: "storage", Title: "Custom", Interpretation: "Local policy", Severity: SeverityCritical,
		All: []Condition{{Metric: "filesystem.used_percent", Op: OpAtLeast, Value: num(70)}}}
	patterns, err := Resolve([]Pattern{override}, []string{"core.cpu.saturation"})
	if err != nil {
		t.Fatal(err)
	}
	var overridden, cpu bool
	for _, pattern := range patterns {
		if pattern.ID == "core.storage.disk_full" {
			overridden = pattern.Source == SourceLocal && pattern.Title == "Custom"
		}
		if pattern.ID == "core.cpu.saturation" {
			cpu = true
		}
	}
	if !overridden || cpu {
		t.Fatalf("override=%v cpuStillPresent=%v", overridden, cpu)
	}
	if _, err := Resolve(nil, []string{"typo"}); err == nil {
		t.Fatal("unknown disabled pattern must be rejected")
	}
	bad := Pattern{ID: "x", Domain: "d", Title: "t", Interpretation: "i", Severity: SeverityInfo, All: []Condition{{Metric: "nope.metric", Op: OpElevated}}}
	if _, err := Resolve([]Pattern{bad}, nil); err == nil {
		t.Fatal("uncataloged metric must be rejected")
	}
	noThreshold := Pattern{ID: "y", Domain: "d", Title: "t", Interpretation: "i", Severity: SeverityInfo, All: []Condition{{Metric: "disk.read_iops", Op: OpElevated}}}
	if err := noThreshold.Validate(); err == nil {
		t.Fatal("elevated on a metric without a threshold must be rejected")
	}
}
