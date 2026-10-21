package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfigIsValidAndPrintable(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default configuration is invalid: %v", err)
	}
	var output strings.Builder
	if err := WriteDefault(&output); err != nil {
		t.Fatalf("WriteDefault returned error: %v", err)
	}
	if !strings.Contains(output.String(), `"host_id": "localhost"`) {
		t.Fatalf("printed configuration does not include host ID: %s", output.String())
	}
}

func TestValidationRejectsInvalidThresholdAndPath(t *testing.T) {
	cfg := Default()
	cfg.Thresholds.CPUPercent = Threshold{Warning: 95, Critical: 85}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected inverted CPU thresholds to be rejected")
	}
	cfg = Default()
	cfg.FilesystemPaths = []string{"relative"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected relative filesystem path to be rejected")
	}
	cfg = Default()
	cfg.DNSProbeName = "name with whitespace"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected DNS probe name containing whitespace to be rejected")
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := temporaryConfigPath(t)
	defer os.Remove(path)
	if err := ioutil.WriteFile(path, []byte(`{"unknown":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unknown configuration field to be rejected")
	}
}

func TestLoadUsesDefaultsForOmittedFields(t *testing.T) {
	path := temporaryConfigPath(t)
	defer os.Remove(path)
	if err := ioutil.WriteFile(path, []byte(`{"host_id":"test-host"}`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.HostID != "test-host" || cfg.CPUSampleInterval != Default().CPUSampleInterval {
		t.Fatalf("unexpected merged configuration: %#v", cfg)
	}
}

func TestEmptyEngineSetIsValid(t *testing.T) {
	cfg := Default()
	cfg.Engines = []string{}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("empty engine configuration should be valid: %v", err)
	}
}

func TestLoadOrCreateCreatesConfigOnlyWhenMissing(t *testing.T) {
	path := temporaryConfigPath(t)
	defer os.Remove(path)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("create default config: %v", err)
	}
	if cfg.HostID != Default().HostID || len(cfg.Engines) != len(Default().Engines) {
		t.Fatalf("unexpected created config: %#v", cfg)
	}
	cfg.Thresholds.CPUPercent.Warning = 72
	if err := writeFileAtomically(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("load existing config: %v", err)
	}
	if loaded.Thresholds.CPUPercent.Warning != 72 {
		t.Fatal("existing file was overwritten by default initialization")
	}
}

func TestPluginConfigurationValidation(t *testing.T) {
	cfg := Default()
	cfg.Plugins = []Plugin{{
		Name:     "example-analyzer",
		Version:  "1.0.0",
		Command:  "example-analyzer",
		Category: "host health",
		Role:     "analysis",
		Consumes: []string{"observation.host.v1"},
		Produces: []string{"analysis.example.v1"},
		Timeout:  "5s",
		Policy:   EnginePolicy{Mode: "reasoning_only", ReadOnly: true},
	}}
	cfg.Engines = append(cfg.Engines, "example-analyzer")
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid plugin config rejected: %v", err)
	}
	cfg.Plugins[0].Timeout = "0s"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected non-positive plugin timeout to be rejected")
	}
	cfg.Plugins[0].Timeout = "5s"
	cfg.Plugins[0].Settings = []byte("{invalid")
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid plugin settings JSON to be rejected")
	}
	cfg.Plugins[0].Settings = nil
	cfg.Plugins[0].Policy.ReadOnly = false
	if err := cfg.Validate(); err == nil {
		t.Fatal("enabled write-capable plugin must be rejected until a policy-enforced tool broker exists")
	}
	cfg.Engines = nil
	if err := cfg.Validate(); err != nil {
		t.Fatalf("disabled write-capable catalog entry should be allowed: %v", err)
	}
}

func TestStoreUpdatesAndReloadsConfiguration(t *testing.T) {
	path := temporaryConfigPath(t)
	defer os.Remove(path)
	initial := Default()
	if err := writeFileAtomically(path, initial); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(path, initial)
	if err != nil {
		t.Fatal(err)
	}
	updated := initial
	updated.Thresholds.CPUPercent = Threshold{Warning: 70, Critical: 90}
	if err := store.Update(updated); err != nil {
		t.Fatalf("persist configuration update: %v", err)
	}
	current := store.Current()
	if current.Thresholds.CPUPercent.Warning != 70 || current.Thresholds.CPUPercent.Critical != 90 {
		t.Fatalf("store did not publish updated values: %#v", current.Thresholds.CPUPercent)
	}
	disk, err := Load(path)
	if err != nil {
		t.Fatalf("load persisted update: %v", err)
	}
	if disk.Thresholds.CPUPercent.Warning != 70 {
		t.Fatalf("configuration update was not persisted: %#v", disk.Thresholds.CPUPercent)
	}

	external := disk
	external.Thresholds.MemoryPercent.Warning = 75
	if err := writeFileAtomically(path, external); err != nil {
		t.Fatal(err)
	}
	changed, err := store.Reload()
	if err != nil || !changed {
		t.Fatalf("reload did not apply external edit: changed=%t err=%v", changed, err)
	}
	if store.Current().Thresholds.MemoryPercent.Warning != 75 {
		t.Fatal("external edit was not reflected in current configuration")
	}
	changed, err = store.Reload()
	if err != nil || changed {
		t.Fatalf("unchanged config reported as changed: changed=%t err=%v", changed, err)
	}
}

func TestStoreKeepsLastValidConfigurationWhenExternalEditIsInvalid(t *testing.T) {
	path := temporaryConfigPath(t)
	defer os.Remove(path)
	initial := Default()
	if err := writeFileAtomically(path, initial); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(path, initial)
	if err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(path, []byte(`{"thresholds":{"cpu_percent":{"warning":99,"critical":50}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Reload(); err == nil {
		t.Fatal("expected invalid external configuration to return an error")
	}
	if store.Current().Thresholds.CPUPercent.Warning != initial.Thresholds.CPUPercent.Warning {
		t.Fatal("invalid external edit replaced last-known-good configuration")
	}
}

func temporaryConfigPath(t *testing.T) string {
	t.Helper()
	file, err := ioutil.TempFile("", "perceptronsre-config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(path)
}
