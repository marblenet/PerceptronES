package observe

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	"perceptronsre/internal/config"
	"perceptronsre/internal/model"
)

type Collector interface {
	Collect(context.Context) ([]model.Observation, error)
}

type HostCollector struct {
	config config.Config
}

func NewHostCollector(cfg config.Config) (*HostCollector, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid collector configuration: %w", err)
	}
	return &HostCollector{config: cfg}, nil
}

func (collector *HostCollector) Collect(ctx context.Context) ([]model.Observation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return collectHost(ctx, collector.config)
}

func observation(metric, component, unit string, value *float64, reason string, observedAt time.Time) model.Observation {
	result := model.Observation{
		Metric:     metric,
		Component:  component,
		Value:      value,
		Unit:       unit,
		Available:  value != nil,
		Reason:     reason,
		ObservedAt: observedAt.UTC(),
	}
	if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0)) {
		result.Value = nil
		result.Available = false
		result.Reason = "collector returned a non-finite measurement"
	}
	if !result.Available && result.Reason == "" {
		result.Reason = "measurement unavailable"
	}
	return result
}

func number(value float64) *float64 {
	return &value
}

func parseMemoryUsedPercent(content []byte) (float64, error) {
	var total, available uint64
	hasTotal, hasAvailable := false, false
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse %s in /proc/meminfo: %w", fields[0], err)
		}
		switch key {
		case "MemTotal":
			total = value
			hasTotal = true
		case "MemAvailable":
			available = value
			hasAvailable = true
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("scan /proc/meminfo: %w", err)
	}
	if !hasTotal || total == 0 {
		return 0, fmt.Errorf("MemTotal is missing or invalid in /proc/meminfo")
	}
	if !hasAvailable {
		return 0, fmt.Errorf("MemAvailable is missing from /proc/meminfo; memory utilization cannot be assessed")
	}
	if available > total {
		return 0, fmt.Errorf("MemAvailable exceeds MemTotal in /proc/meminfo")
	}
	return float64(total-available) / float64(total) * 100, nil
}

func probeDNS(ctx context.Context, cfg config.Config) (model.Observation, error) {
	timeout, err := time.ParseDuration(cfg.DNSProbeTimeout)
	if err != nil {
		return model.Observation{}, fmt.Errorf("parse DNS probe timeout: %w", err)
	}
	probeContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	startedAt := time.Now()
	_, lookupErr := net.DefaultResolver.LookupHost(probeContext, cfg.DNSProbeName)
	completedAt := time.Now().UTC()
	if ctx.Err() != nil {
		return model.Observation{}, ctx.Err()
	}
	value := 1.0
	reason := ""
	if lookupErr != nil {
		value = 0
		reason = fmt.Sprintf("resolution failed for %q: %v", cfg.DNSProbeName, lookupErr)
	}
	latency := float64(completedAt.Sub(startedAt).Microseconds()) / 1000
	return model.Observation{
		Metric:        "dns.resolution_success",
		Component:     "dns",
		Target:        cfg.DNSProbeName,
		Value:         number(value),
		LatencyMillis: number(latency),
		Unit:          "boolean",
		Available:     true,
		Reason:        reason,
		ObservedAt:    completedAt,
	}, nil
}
