package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"
)

type Threshold struct {
	Warning  float64 `json:"warning"`
	Critical float64 `json:"critical"`
}

type Thresholds struct {
	CPUPercent        Threshold `json:"cpu_percent"`
	MemoryPercent     Threshold `json:"memory_percent"`
	FilesystemPercent Threshold `json:"filesystem_percent"`
	LoadPerCPU        Threshold `json:"load_per_cpu"`
}

type Config struct {
	HostID            string     `json:"host_id"`
	FilesystemPaths   []string   `json:"filesystem_paths"`
	DNSProbeName      string     `json:"dns_probe_name,omitempty"`
	DNSProbeTimeout   string     `json:"dns_probe_timeout"`
	CPUSampleInterval string     `json:"cpu_sample_interval"`
	MaxObservationAge string     `json:"max_observation_age"`
	Thresholds        Thresholds `json:"thresholds"`
	Engines           []string   `json:"engines"`
	Plugins           []Plugin   `json:"plugins,omitempty"`
}

type Plugin struct {
	Name     string          `json:"name"`
	Version  string          `json:"version"`
	Command  string          `json:"command"`
	Args     []string        `json:"args,omitempty"`
	Category string          `json:"category"`
	Role     string          `json:"role"`
	Consumes []string        `json:"consumes"`
	Produces []string        `json:"produces"`
	Timeout  string          `json:"timeout"`
	Policy   EnginePolicy    `json:"policy"`
	Settings json.RawMessage `json:"settings,omitempty"`
}

type EnginePolicy struct {
	Mode           string   `json:"mode"`
	ReadOnly       bool     `json:"read_only"`
	AllowedTools   []string `json:"allowed_tools,omitempty"`
	AllowedScripts []string `json:"allowed_scripts,omitempty"`
}

func Default() Config {
	return Config{
		HostID:            "localhost",
		FilesystemPaths:   []string{"/"},
		DNSProbeTimeout:   "2s",
		CPUSampleInterval: "250ms",
		MaxObservationAge: "30s",
		Thresholds: Thresholds{
			CPUPercent:        Threshold{Warning: 85, Critical: 95},
			MemoryPercent:     Threshold{Warning: 85, Critical: 95},
			FilesystemPercent: Threshold{Warning: 85, Critical: 95},
			LoadPerCPU:        Threshold{Warning: 1.5, Critical: 3},
		},
		Engines: []string{"host-pressure", "host-health"},
	}
}

func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		cfg := Default()
		return cfg, cfg.Validate()
	}
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config %q: %w", path, err)
	}
	defer file.Close()
	cfg, err := LoadFrom(file)
	if err != nil {
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	}
	return cfg, nil
}

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.HostID) == "" {
		return errors.New("host_id must not be empty")
	}
	if len(cfg.FilesystemPaths) == 0 {
		return errors.New("filesystem_paths must include at least one path")
	}
	if strings.ContainsAny(cfg.DNSProbeName, " \t\r\n") {
		return errors.New("dns_probe_name must not contain whitespace")
	}
	seenPaths := make(map[string]bool)
	for _, path := range cfg.FilesystemPaths {
		if !strings.HasPrefix(path, "/") {
			return fmt.Errorf("filesystem path %q must be an absolute Linux path", path)
		}
		if seenPaths[path] {
			return fmt.Errorf("filesystem path %q is duplicated", path)
		}
		seenPaths[path] = true
	}
	sampleInterval, err := time.ParseDuration(cfg.CPUSampleInterval)
	if err != nil {
		return fmt.Errorf("cpu_sample_interval: %w", err)
	}
	if sampleInterval < 50*time.Millisecond || sampleInterval > time.Minute {
		return errors.New("cpu_sample_interval must be between 50ms and 1m")
	}
	maxAge, err := time.ParseDuration(cfg.MaxObservationAge)
	if err != nil {
		return fmt.Errorf("max_observation_age: %w", err)
	}
	if maxAge <= 0 {
		return errors.New("max_observation_age must be positive")
	}
	dnsTimeout, err := time.ParseDuration(cfg.DNSProbeTimeout)
	if err != nil {
		return fmt.Errorf("dns_probe_timeout: %w", err)
	}
	if dnsTimeout <= 0 || dnsTimeout > 30*time.Second {
		return errors.New("dns_probe_timeout must be greater than 0 and at most 30s")
	}
	for name, threshold := range map[string]Threshold{
		"cpu_percent":        cfg.Thresholds.CPUPercent,
		"memory_percent":     cfg.Thresholds.MemoryPercent,
		"filesystem_percent": cfg.Thresholds.FilesystemPercent,
	} {
		if math.IsNaN(threshold.Warning) || math.IsInf(threshold.Warning, 0) ||
			math.IsNaN(threshold.Critical) || math.IsInf(threshold.Critical, 0) ||
			threshold.Warning < 0 || threshold.Critical > 100 || threshold.Warning >= threshold.Critical {
			return fmt.Errorf("%s threshold must satisfy 0 <= warning < critical <= 100", name)
		}
	}
	load := cfg.Thresholds.LoadPerCPU
	if math.IsNaN(load.Warning) || math.IsInf(load.Warning, 0) ||
		math.IsNaN(load.Critical) || math.IsInf(load.Critical, 0) ||
		load.Warning <= 0 || load.Critical <= load.Warning {
		return errors.New("load_per_cpu threshold must satisfy 0 < warning < critical")
	}
	knownEngines := map[string]bool{
		"host-pressure": true,
		"host-health":   true,
		"host-mission":  true,
	}
	enabledEngines := make(map[string]bool)
	for index, name := range cfg.Engines {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("engines[%d] must not be empty", index)
		}
		if enabledEngines[name] {
			return fmt.Errorf("engine %q is enabled more than once", name)
		}
		enabledEngines[name] = true
	}
	for index, plugin := range cfg.Plugins {
		if strings.TrimSpace(plugin.Name) == "" || strings.TrimSpace(plugin.Version) == "" ||
			strings.TrimSpace(plugin.Command) == "" || strings.TrimSpace(plugin.Role) == "" ||
			strings.TrimSpace(plugin.Category) == "" {
			return fmt.Errorf("plugins[%d] requires name, version, command, category, and role", index)
		}
		if plugin.Name != strings.TrimSpace(plugin.Name) || plugin.Version != strings.TrimSpace(plugin.Version) ||
			plugin.Command != strings.TrimSpace(plugin.Command) || plugin.Role != strings.TrimSpace(plugin.Role) ||
			plugin.Category != strings.TrimSpace(plugin.Category) {
			return fmt.Errorf("plugins[%d] name, version, command, category, and role must not have surrounding whitespace", index)
		}
		if knownEngines[plugin.Name] {
			return fmt.Errorf("engine name %q is duplicated", plugin.Name)
		}
		knownEngines[plugin.Name] = true
		if len(plugin.Consumes) == 0 || len(plugin.Produces) == 0 {
			return fmt.Errorf("plugin %q must declare consumed and produced event types", plugin.Name)
		}
		if len(plugin.Settings) > 0 && !json.Valid(plugin.Settings) {
			return fmt.Errorf("plugin %q settings must be valid JSON", plugin.Name)
		}
		switch plugin.Policy.Mode {
		case "reasoning_only", "human_escalation", "self_mitigation":
		default:
			return fmt.Errorf("plugin %q policy.mode must be reasoning_only, human_escalation, or self_mitigation", plugin.Name)
		}
		timeout, err := time.ParseDuration(plugin.Timeout)
		if err != nil || timeout <= 0 || timeout > 5*time.Minute {
			return fmt.Errorf("plugin %q timeout must be greater than 0 and at most 5m", plugin.Name)
		}
		for _, eventType := range append(append([]string(nil), plugin.Consumes...), plugin.Produces...) {
			if strings.TrimSpace(eventType) == "" {
				return fmt.Errorf("plugin %q event types must not be empty", plugin.Name)
			}
		}
	}
	for _, name := range cfg.Engines {
		if !knownEngines[name] {
			return fmt.Errorf("enabled engine %q is not a registered built-in or configured plugin", name)
		}
	}
	for _, plugin := range cfg.Plugins {
		if !enabledEngines[plugin.Name] {
			continue
		}
		if plugin.Policy.Mode == "self_mitigation" || !plugin.Policy.ReadOnly ||
			len(plugin.Policy.AllowedTools) > 0 || len(plugin.Policy.AllowedScripts) > 0 {
			return fmt.Errorf("plugin %q cannot be enabled in the observe-only configuration: it must declare read_only=true and request no tools or scripts; self_mitigation is unavailable", plugin.Name)
		}
	}
	return nil
}

func (cfg Config) CPUSampleDuration() (time.Duration, error) {
	return time.ParseDuration(cfg.CPUSampleInterval)
}

func (cfg Config) MaxAgeDuration() (time.Duration, error) {
	return time.ParseDuration(cfg.MaxObservationAge)
}

func WriteDefault(writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(Default())
}
