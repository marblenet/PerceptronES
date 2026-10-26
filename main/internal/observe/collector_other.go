//go:build !linux
// +build !linux

package observe

import (
	"context"
	pathpkg "path"
	"runtime"
	"time"

	"perceptronsre/internal/config"
	"perceptronsre/internal/metrics"
	"perceptronsre/internal/model"
)

func collectHost(ctx context.Context, cfg config.Config) ([]model.Observation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	reason := "Linux /proc and filesystem collectors are unsupported on " + runtime.GOOS
	observations := []model.Observation{
		observation("cpu.utilization_percent", "cpu", "percent", nil, reason, now),
		observation("system.load_1m", "load", "load", nil, reason, now),
		observation("memory.used_percent", "memory", "percent", nil, reason, now),
		observation("network.rx_bytes", "network", "bytes", nil, reason, now),
		observation("network.tx_bytes", "network", "bytes", nil, reason, now),
		observation("process.count", "process", "processes", nil, reason, now),
	}
	for _, filesystemPath := range cfg.FilesystemPaths {
		item := observation("filesystem.used_percent", "filesystem", "percent", nil, reason, now)
		item.ScopeID = pathpkg.Clean(filesystemPath)
		observations = append(observations, item)
	}
	for _, def := range metrics.Extended() {
		if def.Scoped {
			for _, filesystemPath := range cfg.FilesystemPaths {
				item := observation(def.Name, def.Component, def.Unit, nil, reason, now)
				item.ScopeID = pathpkg.Clean(filesystemPath)
				observations = append(observations, item)
			}
			continue
		}
		observations = append(observations, observation(def.Name, def.Component, def.Unit, nil, reason, now))
	}
	if cfg.DNSProbeName != "" {
		probe, err := probeDNS(ctx, cfg)
		if err != nil {
			return nil, err
		}
		observations = append(observations, probe)
	}
	return observations, nil
}
