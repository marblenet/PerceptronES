package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DomainDir is the directory, next to the core configuration file, that holds
// per-domain configuration (host.json, security.json, ...).
const DomainDir = "config.d"

// domainKeys lists the configuration keys each domain file may set. Platform
// settings (dashboard, policy, engines, plugins, knowledge) stay in the core
// file so a domain change cannot alter platform behavior.
var domainKeys = map[string][]string{
	"host":        {"host_id", "filesystem_paths", "cpu_sample_interval", "max_observation_age", "thresholds", "metric_thresholds"},
	"network":     {"metric_thresholds"},
	"security":    {"security_required_checks", "security_evidence"},
	"dns":         {"dns_probe_name", "dns_probe_timeout"},
	"application": {"metric_thresholds"},
}

// applyDomainDir overlays <dir of path>/config.d/<domain>.json onto cfg in
// lexical order. A missing directory is not an error.
func applyDomainDir(cfg Config, path string) (Config, error) {
	dir := filepath.Join(filepath.Dir(path), DomainDir)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read domain config directory %q: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		domain := strings.TrimSuffix(name, ".json")
		allowed, ok := domainKeys[domain]
		if !ok {
			return Config{}, fmt.Errorf("domain config %q: unknown domain %q", name, domain)
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return Config{}, fmt.Errorf("read domain config %q: %w", name, err)
		}
		var keys map[string]json.RawMessage
		if err := json.Unmarshal(data, &keys); err != nil {
			return Config{}, fmt.Errorf("domain config %q: %w", name, err)
		}
		for key := range keys {
			if !contains(allowed, key) {
				return Config{}, fmt.Errorf("domain config %q: key %q is not allowed in the %s domain", name, key, domain)
			}
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&cfg); err != nil {
			return Config{}, fmt.Errorf("domain config %q: %w", name, err)
		}
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("domain config in %q: %w", dir, err)
	}
	return cfg, nil
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
