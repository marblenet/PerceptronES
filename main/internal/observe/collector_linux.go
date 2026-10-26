package observe

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"perceptronsre/internal/config"
	"perceptronsre/internal/model"
)

func collectHost(ctx context.Context, cfg config.Config) ([]model.Observation, error) {
	sampleDuration, err := cfg.CPUSampleDuration()
	if err != nil {
		return nil, err
	}
	observations := make([]model.Observation, 0, 7+len(cfg.FilesystemPaths))
	firstCPU, firstErr := readCPUTicks()
	firstExtended := takeExtendedSnapshot()
	if firstErr == nil {
		timer := time.NewTimer(sampleDuration)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	secondCPU, secondErr := readCPUTicks()
	now := time.Now().UTC()
	if firstErr != nil {
		observations = append(observations, observation("cpu.utilization_percent", "cpu", "percent", nil, firstErr.Error(), now))
	} else if secondErr != nil {
		observations = append(observations, observation("cpu.utilization_percent", "cpu", "percent", nil, secondErr.Error(), now))
	} else {
		value, cpuErr := cpuUtilization(firstCPU, secondCPU)
		if cpuErr != nil {
			observations = append(observations, observation("cpu.utilization_percent", "cpu", "percent", nil, cpuErr.Error(), now))
		} else {
			observations = append(observations, observation("cpu.utilization_percent", "cpu", "percent", number(value), "", now))
		}
	}

	load, loadErr := readLoadAverage()
	if loadErr != nil {
		observations = append(observations, observation("system.load_1m", "load", "load", nil, loadErr.Error(), now))
	} else {
		observations = append(observations, observation("system.load_1m", "load", "load", number(load), "", now))
	}

	memory, memoryErr := readMemoryUsedPercent()
	if memoryErr != nil {
		observations = append(observations, observation("memory.used_percent", "memory", "percent", nil, memoryErr.Error(), now))
	} else {
		observations = append(observations, observation("memory.used_percent", "memory", "percent", number(memory), "", now))
	}

	receive, transmit, networkErr := readNetworkBytes()
	if networkErr != nil {
		observations = append(observations,
			observation("network.rx_bytes", "network", "bytes", nil, networkErr.Error(), now),
			observation("network.tx_bytes", "network", "bytes", nil, networkErr.Error(), now))
	} else {
		observations = append(observations,
			observation("network.rx_bytes", "network", "bytes", number(float64(receive)), "", now),
			observation("network.tx_bytes", "network", "bytes", number(float64(transmit)), "", now))
	}

	processCount, processErr := readProcessCount()
	if processErr != nil {
		observations = append(observations, observation("process.count", "process", "processes", nil, processErr.Error(), now))
	} else {
		observations = append(observations, observation("process.count", "process", "processes", number(float64(processCount)), "", now))
	}

	for _, path := range cfg.FilesystemPaths {
		value, fsErr := readFilesystemUsedPercent(path)
		if fsErr != nil {
			item := observation("filesystem.used_percent", "filesystem", "percent", nil, fmt.Sprintf("%s: %v", path, fsErr), now)
			item.ScopeID = filesystemEntityID(path)
			observations = append(observations, item)
		} else {
			item := observation("filesystem.used_percent", "filesystem", "percent", number(value), "", now)
			item.ScopeID = filesystemEntityID(path)
			observations = append(observations, item)
		}
	}
	if cfg.DNSProbeName != "" {
		probe, err := probeDNS(ctx, cfg)
		if err != nil {
			return nil, err
		}
		observations = append(observations, probe)
	}
	observations = append(observations, collectExtended(cfg, firstExtended, takeExtendedSnapshot(), now)...)
	return observations, nil
}

type cpuTicks struct {
	total uint64
	idle  uint64
}

func readCPUTicks() (cpuTicks, error) {
	content, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return cpuTicks{}, fmt.Errorf("read /proc/stat: %w", err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	if !scanner.Scan() {
		return cpuTicks{}, errors.New("CPU aggregate line is missing from /proc/stat")
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuTicks{}, errors.New("CPU aggregate line in /proc/stat is malformed")
	}
	var result cpuTicks
	for i, field := range fields[1:] {
		if i >= 8 {
			break
		}
		value, parseErr := strconv.ParseUint(field, 10, 64)
		if parseErr != nil {
			return cpuTicks{}, fmt.Errorf("parse CPU tick %q: %w", field, parseErr)
		}
		result.total += value
		if i == 3 || i == 4 {
			result.idle += value
		}
	}
	return result, nil
}

func cpuUtilization(start, end cpuTicks) (float64, error) {
	if end.total <= start.total || end.idle < start.idle {
		return 0, errors.New("CPU counters did not advance during the sample interval")
	}
	totalDelta := end.total - start.total
	idleDelta := end.idle - start.idle
	if totalDelta == 0 || idleDelta > totalDelta {
		return 0, errors.New("CPU counters produced an invalid sample")
	}
	return (1 - float64(idleDelta)/float64(totalDelta)) * 100, nil
}

func readLoadAverage() (float64, error) {
	content, err := ioutil.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, fmt.Errorf("read /proc/loadavg: %w", err)
	}
	fields := strings.Fields(string(content))
	if len(fields) < 1 {
		return 0, errors.New("/proc/loadavg is empty")
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("parse 1-minute load average: %w", err)
	}
	return value, nil
}

func readMemoryUsedPercent() (float64, error) {
	content, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, fmt.Errorf("read /proc/meminfo: %w", err)
	}
	return parseMemoryUsedPercent(content)
}

func readNetworkBytes() (uint64, uint64, error) {
	content, err := ioutil.ReadFile("/proc/net/dev")
	if err != nil {
		return 0, 0, fmt.Errorf("read /proc/net/dev: %w", err)
	}
	var receive, transmit uint64
	found := false
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := scanner.Text()
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		fields := strings.Fields(line[colon+1:])
		if len(fields) < 9 {
			continue
		}
		rx, parseErr := strconv.ParseUint(fields[0], 10, 64)
		if parseErr != nil {
			return 0, 0, fmt.Errorf("parse receive bytes for interface %q: %w", strings.TrimSpace(line[:colon]), parseErr)
		}
		tx, parseErr := strconv.ParseUint(fields[8], 10, 64)
		if parseErr != nil {
			return 0, 0, fmt.Errorf("parse transmit bytes for interface %q: %w", strings.TrimSpace(line[:colon]), parseErr)
		}
		receive += rx
		transmit += tx
		found = true
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, fmt.Errorf("scan /proc/net/dev: %w", err)
	}
	if !found {
		return 0, 0, errors.New("no network interfaces were found in /proc/net/dev")
	}
	return receive, transmit, nil
}

func readProcessCount() (int, error) {
	entries, err := ioutil.ReadDir("/proc")
	if err != nil {
		return 0, fmt.Errorf("read /proc process list: %w", err)
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			if _, err := strconv.Atoi(entry.Name()); err == nil {
				count++
			}
		}
	}
	return count, nil
}

func readFilesystemUsedPercent(path string) (float64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, fmt.Errorf("stat filesystem %q: %w", path, err)
	}
	if stat.Blocks == 0 || stat.Bfree > stat.Blocks {
		return 0, fmt.Errorf("filesystem %q returned invalid block counts", path)
	}
	used := stat.Blocks - stat.Bfree
	return float64(used) / float64(stat.Blocks) * 100, nil
}

func filesystemEntityID(path string) string {
	return filepath.Clean(path)
}
