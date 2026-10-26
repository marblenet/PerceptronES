package metrics

import "testing"

func TestCatalogHasFiftyUniqueWellFormedMetrics(t *testing.T) {
	all := All()
	if len(all) != 50 {
		t.Fatalf("expected 50 metrics, got %d", len(all))
	}
	seen := map[string]bool{}
	core := 0
	for _, def := range all {
		if seen[def.Name] {
			t.Fatalf("duplicate metric %s", def.Name)
		}
		seen[def.Name] = true
		if def.Component == "" || def.Unit == "" || def.Category == "" || def.Description == "" {
			t.Fatalf("incomplete definition %#v", def)
		}
		if def.Core {
			core++
		}
		if def.Thresholded && (def.Warn < 0 || def.Crit < def.Warn || (def.Percent && def.Crit > 100)) {
			t.Fatalf("invalid default threshold for %s", def.Name)
		}
		if def.Percent && def.Boolean {
			t.Fatalf("%s cannot be both percent and boolean", def.Name)
		}
	}
	if core != 7 {
		t.Fatalf("expected the 7 original core metrics, got %d", core)
	}
}

func TestAdvisoryComponentsNeverIncludeCoreMetrics(t *testing.T) {
	for _, component := range []string{"cpu", "memory", "load", "filesystem", "network", "process", "dns", "unknown-plugin"} {
		if IsAdvisoryComponent(component) {
			t.Fatalf("%s must not be advisory", component)
		}
	}
	for _, component := range []string{"cpu_detail", "swap", "pressure", "disk", "system"} {
		if !IsAdvisoryComponent(component) {
			t.Fatalf("%s should be advisory", component)
		}
	}
}
