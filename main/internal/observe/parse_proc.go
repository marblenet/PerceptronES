package observe

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Parsers in this file are platform-neutral so they can be tested anywhere.

type statSnapshot struct {
	total, system, iowait, steal uint64
	contextSwitches              uint64
	running, blocked             uint64
	cpus                         uint64
}

func parseStat(content string) (statSnapshot, error) {
	var snap statSnapshot
	sawAggregate := false
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch {
		case fields[0] == "cpu":
			if len(fields) < 5 {
				return snap, errors.New("aggregate cpu line is malformed")
			}
			for i, field := range fields[1:] {
				if i >= 8 {
					break
				}
				value, err := strconv.ParseUint(field, 10, 64)
				if err != nil {
					return snap, fmt.Errorf("parse cpu tick %q: %w", field, err)
				}
				snap.total += value
				switch i {
				case 2, 5, 6: // system, irq, softirq
					snap.system += value
				case 4:
					snap.iowait = value
				case 7:
					snap.steal = value
				}
			}
			sawAggregate = true
		case strings.HasPrefix(fields[0], "cpu"):
			snap.cpus++
		default:
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				continue
			}
			switch fields[0] {
			case "ctxt":
				snap.contextSwitches = value
			case "procs_running":
				snap.running = value
			case "procs_blocked":
				snap.blocked = value
			}
		}
	}
	if !sawAggregate {
		return snap, errors.New("aggregate cpu line is missing")
	}
	return snap, nil
}

func parseKeyValueCounters(content string) map[string]uint64 {
	result := make(map[string]uint64)
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if value, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
			result[fields[0]] = value
		}
	}
	return result
}

// parseMeminfoKB returns /proc/meminfo values in bytes.
func parseMeminfoBytes(content string) map[string]uint64 {
	result := make(map[string]uint64)
	for _, line := range strings.Split(content, "\n") {
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		fields := strings.Fields(line[colon+1:])
		if len(fields) == 0 {
			continue
		}
		value, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		if len(fields) > 1 && strings.EqualFold(fields[1], "kB") {
			value *= 1024
		}
		result[line[:colon]] = value
	}
	return result
}

type diskCounters struct {
	reads, readSectors, writes, writeSectors, ioMillis uint64
}

// parseDiskstats sums the listed whole devices and returns the per-device I/O
// milliseconds so utilization can use the busiest disk.
func parseDiskstats(content string, wholeDisks map[string]bool) (diskCounters, map[string]uint64, error) {
	var total diskCounters
	perDisk := make(map[string]uint64)
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 13 || !wholeDisks[fields[2]] {
			continue
		}
		values := make([]uint64, 13)
		for i := 3; i <= 12; i++ {
			value, err := strconv.ParseUint(fields[i], 10, 64)
			if err != nil {
				return total, nil, fmt.Errorf("parse diskstats field for %s: %w", fields[2], err)
			}
			values[i] = value
		}
		total.reads += values[3]
		total.readSectors += values[5]
		total.writes += values[7]
		total.writeSectors += values[9]
		total.ioMillis += values[12]
		perDisk[fields[2]] = values[12]
	}
	if len(perDisk) == 0 {
		return total, nil, errors.New("no physical disks found in /proc/diskstats")
	}
	return total, perDisk, nil
}

type netTotals struct {
	rxBytes, txBytes, errors, dropped uint64
}

// parseNetDevExternal sums every non-loopback interface.
func parseNetDevExternal(content string) (netTotals, error) {
	var total netTotals
	found := false
	for _, line := range strings.Split(content, "\n") {
		colon := strings.IndexByte(line, ':')
		if colon < 0 || strings.TrimSpace(line[:colon]) == "lo" {
			continue
		}
		fields := strings.Fields(line[colon+1:])
		if len(fields) < 12 {
			continue
		}
		values := make([]uint64, 12)
		for i := 0; i < 12; i++ {
			value, err := strconv.ParseUint(fields[i], 10, 64)
			if err != nil {
				return total, fmt.Errorf("parse /proc/net/dev for %s: %w", strings.TrimSpace(line[:colon]), err)
			}
			values[i] = value
		}
		total.rxBytes += values[0]
		total.errors += values[2] + values[10]
		total.dropped += values[3] + values[11]
		total.txBytes += values[8]
		found = true
	}
	if !found {
		return total, errors.New("no non-loopback interfaces in /proc/net/dev")
	}
	return total, nil
}

func parseSNMPTcp(content string) (map[string]uint64, error) {
	lines := strings.Split(content, "\n")
	for i := 0; i+1 < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "Tcp:") || !strings.HasPrefix(lines[i+1], "Tcp:") {
			continue
		}
		names := strings.Fields(lines[i])[1:]
		values := strings.Fields(lines[i+1])[1:]
		if len(names) != len(values) {
			return nil, errors.New("Tcp header and value columns differ in /proc/net/snmp")
		}
		result := make(map[string]uint64)
		for j, name := range names {
			if value, err := strconv.ParseUint(values[j], 10, 64); err == nil {
				result[name] = value
			}
		}
		return result, nil
	}
	return nil, errors.New("Tcp section missing from /proc/net/snmp")
}

// parsePSIAvg10 reads the "some" avg10 value from a /proc/pressure/* file.
func parsePSIAvg10(content string) (float64, error) {
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "some" {
			continue
		}
		for _, field := range fields[1:] {
			if strings.HasPrefix(field, "avg10=") {
				return strconv.ParseFloat(strings.TrimPrefix(field, "avg10="), 64)
			}
		}
	}
	return 0, errors.New("pressure file has no some avg10 value")
}

func parseFileNr(content string) (allocated, max uint64, err error) {
	fields := strings.Fields(content)
	if len(fields) < 3 {
		return 0, 0, errors.New("file-nr is malformed")
	}
	if allocated, err = strconv.ParseUint(fields[0], 10, 64); err != nil {
		return 0, 0, err
	}
	if max, err = strconv.ParseUint(fields[2], 10, 64); err != nil {
		return 0, 0, err
	}
	if max == 0 {
		return 0, 0, errors.New("file-nr maximum is zero")
	}
	return allocated, max, nil
}

func parseSingleUint(content string) (uint64, error) {
	return strconv.ParseUint(strings.TrimSpace(content), 10, 64)
}

func parseUptimeSeconds(content string) (float64, error) {
	fields := strings.Fields(content)
	if len(fields) < 1 {
		return 0, errors.New("uptime is empty")
	}
	return strconv.ParseFloat(fields[0], 64)
}

// parseProcStat extracts the state and thread count from a /proc/<pid>/stat line.
func parseProcStat(content string) (state string, threads uint64, err error) {
	end := strings.LastIndexByte(content, ')')
	if end < 0 {
		return "", 0, errors.New("process stat is malformed")
	}
	fields := strings.Fields(content[end+1:])
	if len(fields) < 18 {
		return "", 0, errors.New("process stat has too few fields")
	}
	threads, err = strconv.ParseUint(fields[17], 10, 64)
	return fields[0], threads, err
}

func mountIsReadOnly(mounts, path string) (bool, error) {
	best := ""
	readOnly := false
	for _, line := range strings.Split(mounts, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		mountPoint := strings.ReplaceAll(fields[1], "\\040", " ")
		if path != mountPoint && !strings.HasPrefix(path, strings.TrimSuffix(mountPoint, "/")+"/") {
			continue
		}
		if len(mountPoint) >= len(best) {
			best = mountPoint
			readOnly = false
			for _, option := range strings.Split(fields[3], ",") {
				if option == "ro" {
					readOnly = true
				}
			}
		}
	}
	if best == "" {
		return false, fmt.Errorf("no mount found for %q", path)
	}
	return readOnly, nil
}
