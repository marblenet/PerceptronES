package observe

import (
	"context"
	"runtime"
	"testing"

	"perceptronsre/internal/config"
)

func TestHostCollectorEmitsEveryConfiguredSignal(t *testing.T) {
	cfg := config.Default()
	cfg.CPUSampleInterval = "50ms"
	collector, err := NewHostCollector(cfg)
	if err != nil {
		t.Fatal(err)
	}
	observations, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	required := map[string]bool{
		"cpu.utilization_percent": false,
		"system.load_1m":          false,
		"memory.used_percent":     false,
		"network.rx_bytes":        false,
		"network.tx_bytes":        false,
		"process.count":           false,
		"filesystem.used_percent": false,
	}
	for _, observation := range observations {
		required[observation.Metric] = true
		if !observation.Available && observation.Reason == "" {
			t.Errorf("%s is unavailable without a reason", observation.Metric)
		}
	}
	for metric, found := range required {
		if !found {
			t.Errorf("collector did not emit %s", metric)
		}
	}
	if runtime.GOOS != "linux" {
		for _, observation := range observations {
			if observation.Available {
				t.Fatalf("unsupported platform reported %s as available", observation.Metric)
			}
		}
	}
}

func TestHostCollectorHonorsCanceledContext(t *testing.T) {
	collector, err := NewHostCollector(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := collector.Collect(ctx); err == nil {
		t.Fatal("expected canceled collection to fail")
	}
}

func TestMemoryParserRequiresMemAvailable(t *testing.T) {
	content := []byte("MemTotal: 1000 kB\nMemFree: 500 kB\nBuffers: 20 kB\nCached: 100 kB\n")
	if _, err := parseMemoryUsedPercent(content); err == nil {
		t.Fatal("missing MemAvailable must not be interpreted as zero available memory")
	}
}

func TestMemoryParserUsesExplicitAvailableMemory(t *testing.T) {
	content := []byte("MemTotal: 1000 kB\nMemAvailable: 250 kB\n")
	value, err := parseMemoryUsedPercent(content)
	if err != nil {
		t.Fatal(err)
	}
	if value != 75 {
		t.Fatalf("expected 75%% memory utilization, got %.2f%%", value)
	}
}
