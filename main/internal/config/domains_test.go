package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDomainFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "perceptron.json"), []byte(`{"host_id":"base"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, DomainDir), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, DomainDir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "perceptron.json")
}

func TestDomainConfigOverlaysCoreConfiguration(t *testing.T) {
	path := writeDomainFixture(t, map[string]string{
		"host.json": `{"host_id":"from-domain","thresholds":{"cpu_percent":{"warning":70,"critical":90}}}`,
		"dns.json":  `{"dns_probe_name":"example.com"}`,
	})
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.HostID != "from-domain" || cfg.Thresholds.CPUPercent.Warning != 70 || cfg.DNSProbeName != "example.com" {
		t.Fatalf("domain overlay not applied: %#v", cfg)
	}
	if cfg.Thresholds.MemoryPercent.Warning != Default().Thresholds.MemoryPercent.Warning {
		t.Fatal("partial threshold overlay changed an unrelated threshold")
	}
}

func TestDomainConfigRejectsPlatformKeysAndUnknownDomains(t *testing.T) {
	cases := map[string]string{
		"host.json":    `{"dashboard":{"port":9}}`,
		"weather.json": `{}`,
		"dns.json":     `{"unknown":1}`,
	}
	for name, body := range cases {
		path := writeDomainFixture(t, map[string]string{name: body})
		if _, err := Load(path); err == nil || !strings.Contains(err.Error(), name) {
			t.Fatalf("%s: expected rejection naming the file, got %v", name, err)
		}
	}
}
