package observe

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"perceptronsre/internal/config"
	"perceptronsre/internal/metrics"
	"perceptronsre/internal/model"
)

// extendedSnapshot holds the counters needed for rate metrics. Each source
// records its own error so one missing file only makes its own metrics unknown.
type extendedSnapshot struct {
	at      time.Time
	stat    statSnapshot
	statErr error
	vm      map[string]uint64
	vmErr   error
	disk    diskCounters
	perDisk map[string]uint64
	diskErr error
	net     netTotals
	netErr  error
	tcp     map[string]uint64
	tcpErr  error
}

func readText(path string) (string, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(content), nil
}

func wholeDisks() map[string]bool {
	result := make(map[string]bool)
	entries, err := ioutil.ReadDir("/sys/block")
	if err != nil {
		return result
	}
	for _, entry := range entries {
		name := entry.Name()
		skip := false
		for _, prefix := range []string{"loop", "ram", "zram", "dm-", "md", "sr", "fd"} {
			if strings.HasPrefix(name, prefix) {
				skip = true
			}
		}
		if !skip {
			result[name] = true
		}
	}
	return result
}

func takeExtendedSnapshot() extendedSnapshot {
	snap := extendedSnapshot{at: time.Now()}
	if text, err := readText("/proc/stat"); err != nil {
		snap.statErr = err
	} else {
		snap.stat, snap.statErr = parseStat(text)
	}
	if text, err := readText("/proc/vmstat"); err != nil {
		snap.vmErr = err
	} else {
		snap.vm = parseKeyValueCounters(text)
	}
	if text, err := readText("/proc/diskstats"); err != nil {
		snap.diskErr = err
	} else {
		snap.disk, snap.perDisk, snap.diskErr = parseDiskstats(text, wholeDisks())
	}
	if text, err := readText("/proc/net/dev"); err != nil {
		snap.netErr = err
	} else {
		snap.net, snap.netErr = parseNetDevExternal(text)
	}
	if text, err := readText("/proc/net/snmp"); err != nil {
		snap.tcpErr = err
	} else {
		snap.tcp, snap.tcpErr = parseSNMPTcp(text)
	}
	return snap
}

type extendedEmitter struct {
	now          time.Time
	observations []model.Observation
}

func (e *extendedEmitter) emit(name, scope string, value float64, err error) {
	def, ok := metrics.Lookup(name)
	if !ok {
		return
	}
	var item model.Observation
	if err != nil {
		item = observation(name, def.Component, def.Unit, nil, err.Error(), e.now)
	} else {
		item = observation(name, def.Component, def.Unit, number(value), "", e.now)
	}
	item.ScopeID = scope
	e.observations = append(e.observations, item)
}

func counterRate(first, second uint64, seconds float64) (float64, error) {
	if seconds <= 0 {
		return 0, errors.New("sample interval was too short to compute a rate")
	}
	if second < first {
		return 0, errors.New("counter decreased between samples")
	}
	return float64(second-first) / seconds, nil
}

func percentOf(part, whole uint64) (float64, error) {
	if whole == 0 {
		return 0, errors.New("denominator is zero")
	}
	return float64(part) / float64(whole) * 100, nil
}

func collectExtended(cfg config.Config, first, second extendedSnapshot, now time.Time) []model.Observation {
	e := &extendedEmitter{now: now}
	seconds := second.at.Sub(first.at).Seconds()

	// CPU detail and run queue.
	if err := firstErr(first.statErr, second.statErr); err != nil {
		for _, name := range []string{"cpu.iowait_percent", "cpu.steal_percent", "cpu.system_percent", "cpu.count", "cpu.context_switches_per_sec", "process.running", "process.blocked"} {
			e.emit(name, "", 0, err)
		}
	} else {
		delta := second.stat.total - first.stat.total
		if second.stat.total < first.stat.total || delta == 0 {
			err := errors.New("CPU counters did not advance during the sample interval")
			for _, name := range []string{"cpu.iowait_percent", "cpu.steal_percent", "cpu.system_percent"} {
				e.emit(name, "", 0, err)
			}
		} else {
			e.emit("cpu.iowait_percent", "", clampPercent(float64(second.stat.iowait-first.stat.iowait)/float64(delta)*100), nil)
			e.emit("cpu.steal_percent", "", clampPercent(float64(second.stat.steal-first.stat.steal)/float64(delta)*100), nil)
			e.emit("cpu.system_percent", "", clampPercent(float64(second.stat.system-first.stat.system)/float64(delta)*100), nil)
		}
		e.emit("cpu.count", "", float64(second.stat.cpus), nil)
		rate, err := counterRate(first.stat.contextSwitches, second.stat.contextSwitches, seconds)
		e.emit("cpu.context_switches_per_sec", "", rate, err)
		e.emit("process.running", "", float64(second.stat.running), nil)
		e.emit("process.blocked", "", float64(second.stat.blocked), nil)
	}

	// Load averages.
	if text, err := readText("/proc/loadavg"); err != nil {
		e.emit("system.load_5m", "", 0, err)
		e.emit("system.load_15m", "", 0, err)
	} else {
		fields := strings.Fields(text)
		for index, name := range map[int]string{1: "system.load_5m", 2: "system.load_15m"} {
			if len(fields) <= index {
				e.emit(name, "", 0, errors.New("/proc/loadavg is malformed"))
				continue
			}
			value, parseErr := strconv.ParseFloat(fields[index], 64)
			e.emit(name, "", value, parseErr)
		}
	}

	collectMemory(e, first, second, seconds)
	collectPressure(e)
	collectDisk(e, first, second, seconds)
	collectNetwork(e, first, second, seconds)
	collectSystem(e)
	for _, path := range cfg.FilesystemPaths {
		collectFilesystemDetail(e, path)
	}
	return e.observations
}

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func collectMemory(e *extendedEmitter, first, second extendedSnapshot, seconds float64) {
	memNames := []string{"memory.total_bytes", "memory.available_bytes", "memory.dirty_bytes", "memory.commit_percent", "memory.swap_used_percent"}
	text, err := readText("/proc/meminfo")
	if err != nil {
		for _, name := range memNames {
			e.emit(name, "", 0, err)
		}
	} else {
		info := parseMeminfoBytes(text)
		require := func(name, key string) {
			if value, ok := info[key]; ok {
				e.emit(name, "", float64(value), nil)
			} else {
				e.emit(name, "", 0, fmt.Errorf("%s missing from /proc/meminfo", key))
			}
		}
		require("memory.total_bytes", "MemTotal")
		require("memory.available_bytes", "MemAvailable")
		require("memory.dirty_bytes", "Dirty")
		commit, commitErr := percentOf(info["Committed_AS"], info["CommitLimit"])
		e.emit("memory.commit_percent", "", commit, commitErr)
		if swapTotal, ok := info["SwapTotal"]; !ok {
			e.emit("memory.swap_used_percent", "", 0, errors.New("SwapTotal missing from /proc/meminfo"))
		} else if swapTotal == 0 {
			e.emit("memory.swap_used_percent", "", 0, nil)
		} else {
			swap, swapErr := percentOf(swapTotal-info["SwapFree"], swapTotal)
			e.emit("memory.swap_used_percent", "", clampPercent(swap), swapErr)
		}
	}
	vmNames := []string{"memory.swap_in_pages_per_sec", "memory.swap_out_pages_per_sec", "memory.major_faults_per_sec"}
	if err := firstErr(first.vmErr, second.vmErr); err != nil {
		for _, name := range vmNames {
			e.emit(name, "", 0, err)
		}
		e.emit("memory.oom_kills_total", "", 0, err)
		return
	}
	for name, key := range map[string]string{"memory.swap_in_pages_per_sec": "pswpin", "memory.swap_out_pages_per_sec": "pswpout", "memory.major_faults_per_sec": "pgmajfault"} {
		a, aok := first.vm[key]
		b, bok := second.vm[key]
		if !aok || !bok {
			e.emit(name, "", 0, fmt.Errorf("%s missing from /proc/vmstat", key))
			continue
		}
		rate, rateErr := counterRate(a, b, seconds)
		e.emit(name, "", rate, rateErr)
	}
	if oom, ok := second.vm["oom_kill"]; ok {
		e.emit("memory.oom_kills_total", "", float64(oom), nil)
	} else {
		e.emit("memory.oom_kills_total", "", 0, errors.New("oom_kill is not exposed by this kernel's /proc/vmstat"))
	}
}

func collectPressure(e *extendedEmitter) {
	for name, file := range map[string]string{
		"pressure.cpu_some_avg10":    "/proc/pressure/cpu",
		"pressure.memory_some_avg10": "/proc/pressure/memory",
		"pressure.io_some_avg10":     "/proc/pressure/io",
	} {
		text, err := readText(file)
		if err != nil {
			e.emit(name, "", 0, fmt.Errorf("PSI unavailable: %w", err))
			continue
		}
		value, parseErr := parsePSIAvg10(text)
		e.emit(name, "", clampPercent(value), parseErr)
	}
}

func collectDisk(e *extendedEmitter, first, second extendedSnapshot, seconds float64) {
	names := []string{"disk.read_bytes_per_sec", "disk.write_bytes_per_sec", "disk.read_iops", "disk.write_iops", "disk.io_utilization_percent"}
	if err := firstErr(first.diskErr, second.diskErr); err != nil {
		for _, name := range names {
			e.emit(name, "", 0, err)
		}
		return
	}
	a, b := first.disk, second.disk
	readBytes, err1 := counterRate(a.readSectors*512, b.readSectors*512, seconds)
	e.emit("disk.read_bytes_per_sec", "", readBytes, err1)
	writeBytes, err2 := counterRate(a.writeSectors*512, b.writeSectors*512, seconds)
	e.emit("disk.write_bytes_per_sec", "", writeBytes, err2)
	readOps, err3 := counterRate(a.reads, b.reads, seconds)
	e.emit("disk.read_iops", "", readOps, err3)
	writeOps, err4 := counterRate(a.writes, b.writes, seconds)
	e.emit("disk.write_iops", "", writeOps, err4)
	busiest := 0.0
	var utilErr error
	for disk, start := range first.perDisk {
		end, ok := second.perDisk[disk]
		if !ok || end < start {
			utilErr = errors.New("disk I/O counters changed between samples")
			break
		}
		if seconds <= 0 {
			utilErr = errors.New("sample interval was too short to compute a rate")
			break
		}
		if busy := float64(end-start) / (seconds * 1000) * 100; busy > busiest {
			busiest = busy
		}
	}
	e.emit("disk.io_utilization_percent", "", clampPercent(busiest), utilErr)
}

func collectNetwork(e *extendedEmitter, first, second extendedSnapshot, seconds float64) {
	if err := firstErr(first.netErr, second.netErr); err != nil {
		for _, name := range []string{"network.errors_total", "network.dropped_total", "network.rx_bytes_per_sec", "network.tx_bytes_per_sec"} {
			e.emit(name, "", 0, err)
		}
	} else {
		e.emit("network.errors_total", "", float64(second.net.errors), nil)
		e.emit("network.dropped_total", "", float64(second.net.dropped), nil)
		rx, rxErr := counterRate(first.net.rxBytes, second.net.rxBytes, seconds)
		e.emit("network.rx_bytes_per_sec", "", rx, rxErr)
		tx, txErr := counterRate(first.net.txBytes, second.net.txBytes, seconds)
		e.emit("network.tx_bytes_per_sec", "", tx, txErr)
	}
	if err := firstErr(first.tcpErr, second.tcpErr); err != nil {
		e.emit("network.tcp_established", "", 0, err)
		e.emit("network.tcp_retransmits_per_sec", "", 0, err)
	} else {
		if value, ok := second.tcp["CurrEstab"]; ok {
			e.emit("network.tcp_established", "", float64(value), nil)
		} else {
			e.emit("network.tcp_established", "", 0, errors.New("CurrEstab missing from /proc/net/snmp"))
		}
		a, aok := first.tcp["RetransSegs"]
		b, bok := second.tcp["RetransSegs"]
		if !aok || !bok {
			e.emit("network.tcp_retransmits_per_sec", "", 0, errors.New("RetransSegs missing from /proc/net/snmp"))
		} else {
			rate, rateErr := counterRate(a, b, seconds)
			e.emit("network.tcp_retransmits_per_sec", "", rate, rateErr)
		}
	}
	entries, err := ioutil.ReadDir("/sys/class/net")
	if err != nil {
		e.emit("network.interfaces_up", "", 0, err)
	} else {
		up := 0
		for _, entry := range entries {
			if entry.Name() == "lo" {
				continue
			}
			if text, readErr := readText(filepath.Join("/sys/class/net", entry.Name(), "operstate")); readErr == nil && strings.TrimSpace(text) == "up" {
				up++
			}
		}
		e.emit("network.interfaces_up", "", float64(up), nil)
	}
	count, countErr := readText("/proc/sys/net/netfilter/nf_conntrack_count")
	limit, limitErr := readText("/proc/sys/net/netfilter/nf_conntrack_max")
	if err := firstErr(countErr, limitErr); err != nil {
		e.emit("network.conntrack_used_percent", "", 0, fmt.Errorf("conntrack unavailable: %w", err))
		return
	}
	used, usedErr := parseSingleUint(count)
	max, maxErr := parseSingleUint(limit)
	if err := firstErr(usedErr, maxErr); err != nil {
		e.emit("network.conntrack_used_percent", "", 0, err)
		return
	}
	percent, percentErr := percentOf(used, max)
	e.emit("network.conntrack_used_percent", "", clampPercent(percent), percentErr)
}

func collectSystem(e *extendedEmitter) {
	procNames := []string{"process.zombies", "process.threads_total", "process.pid_used_percent"}
	entries, err := ioutil.ReadDir("/proc")
	if err != nil {
		for _, name := range procNames {
			e.emit(name, "", 0, err)
		}
	} else {
		var zombies, threads, processes uint64
		for _, entry := range entries {
			if _, convErr := strconv.Atoi(entry.Name()); convErr != nil || !entry.IsDir() {
				continue
			}
			text, readErr := readText(filepath.Join("/proc", entry.Name(), "stat"))
			if readErr != nil {
				continue // the process exited while scanning
			}
			state, count, parseErr := parseProcStat(text)
			if parseErr != nil {
				continue
			}
			processes++
			threads += count
			if state == "Z" {
				zombies++
			}
		}
		e.emit("process.zombies", "", float64(zombies), nil)
		e.emit("process.threads_total", "", float64(threads), nil)
		if text, readErr := readText("/proc/sys/kernel/pid_max"); readErr != nil {
			e.emit("process.pid_used_percent", "", 0, readErr)
		} else if max, parseErr := parseSingleUint(text); parseErr != nil {
			e.emit("process.pid_used_percent", "", 0, parseErr)
		} else {
			percent, percentErr := percentOf(processes, max)
			e.emit("process.pid_used_percent", "", clampPercent(percent), percentErr)
		}
	}
	if text, readErr := readText("/proc/sys/fs/file-nr"); readErr != nil {
		e.emit("system.file_descriptors_used_percent", "", 0, readErr)
	} else if allocated, max, parseErr := parseFileNr(text); parseErr != nil {
		e.emit("system.file_descriptors_used_percent", "", 0, parseErr)
	} else {
		percent, percentErr := percentOf(allocated, max)
		e.emit("system.file_descriptors_used_percent", "", clampPercent(percent), percentErr)
	}
	if text, readErr := readText("/proc/uptime"); readErr != nil {
		e.emit("system.uptime_seconds", "", 0, readErr)
	} else {
		value, parseErr := parseUptimeSeconds(text)
		e.emit("system.uptime_seconds", "", value, parseErr)
	}
	if text, readErr := readText("/proc/sys/kernel/tainted"); readErr != nil {
		e.emit("system.kernel_tainted", "", 0, readErr)
	} else if mask, parseErr := parseSingleUint(text); parseErr != nil {
		e.emit("system.kernel_tainted", "", 0, parseErr)
	} else if mask != 0 {
		e.emit("system.kernel_tainted", "", 1, nil)
	} else {
		e.emit("system.kernel_tainted", "", 0, nil)
	}
}

func collectFilesystemDetail(e *extendedEmitter, path string) {
	scope := filesystemEntityID(path)
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		err = fmt.Errorf("stat filesystem %q: %w", path, err)
		for _, name := range []string{"filesystem.inodes_used_percent", "filesystem.available_bytes", "filesystem.read_only"} {
			e.emit(name, scope, 0, err)
		}
		return
	}
	if stat.Files == 0 || stat.Ffree > stat.Files {
		e.emit("filesystem.inodes_used_percent", scope, 0, fmt.Errorf("filesystem %q does not report inode counts", path))
	} else {
		percent, _ := percentOf(stat.Files-stat.Ffree, stat.Files)
		e.emit("filesystem.inodes_used_percent", scope, percent, nil)
	}
	e.emit("filesystem.available_bytes", scope, float64(stat.Bavail)*float64(stat.Bsize), nil)
	mounts, err := readText("/proc/mounts")
	if err != nil {
		e.emit("filesystem.read_only", scope, 0, err)
		return
	}
	readOnly, err := mountIsReadOnly(mounts, filepath.Clean(path))
	if err != nil {
		e.emit("filesystem.read_only", scope, 0, err)
		return
	}
	if readOnly {
		e.emit("filesystem.read_only", scope, 1, nil)
	} else {
		e.emit("filesystem.read_only", scope, 0, nil)
	}
}
