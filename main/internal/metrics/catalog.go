// Package metrics defines the catalog of host metrics the core understands.
// The catalog is the single source of truth for names, components, units,
// value ranges, and default thresholds. Collectors, validation, pressure
// assessment, and the dashboard all read from it.
package metrics

import "sort"

type Def struct {
	Name        string
	Component   string
	Unit        string
	Category    string
	Description string
	// Core metrics predate the catalog; their component names are required by the
	// mission and their thresholds come from config.Thresholds.
	Core bool
	// Percent values must be within 0..100; Boolean values must be 0 or 1.
	Percent bool
	Boolean bool
	// Scoped metrics are emitted once per configured filesystem path.
	Scoped bool
	// Warn/Crit are catalog default thresholds, used only when Thresholded.
	Thresholded bool
	Warn, Crit  float64
}

// Extended components are advisory: a degraded or critical extended finding can
// raise the mission status, but an unavailable extended metric alone does not
// make the mission UNKNOWN (kernel features such as PSI or conntrack may be
// legitimately absent). Unavailability is still reported on the metric.
var catalog = []Def{
	{Name: "cpu.utilization_percent", Component: "cpu", Unit: "percent", Category: "CPU", Description: "Aggregate CPU busy time over the sample interval", Core: true, Percent: true},
	{Name: "cpu.iowait_percent", Component: "cpu_detail", Unit: "percent", Category: "CPU", Description: "CPU time waiting on I/O", Percent: true, Thresholded: true, Warn: 20, Crit: 40},
	{Name: "cpu.steal_percent", Component: "cpu_detail", Unit: "percent", Category: "CPU", Description: "CPU time stolen by the hypervisor", Percent: true, Thresholded: true, Warn: 10, Crit: 25},
	{Name: "cpu.system_percent", Component: "cpu_detail", Unit: "percent", Category: "CPU", Description: "CPU time in kernel mode", Percent: true},
	{Name: "cpu.count", Component: "cpu_detail", Unit: "cpus", Category: "CPU", Description: "Online CPUs"},
	{Name: "cpu.context_switches_per_sec", Component: "cpu_detail", Unit: "per_second", Category: "CPU", Description: "Context switches per second"},
	{Name: "process.running", Component: "process_detail", Unit: "processes", Category: "CPU", Description: "Runnable processes"},
	{Name: "system.load_1m", Component: "load", Unit: "load", Category: "Load", Description: "1-minute load average", Core: true},
	{Name: "system.load_5m", Component: "load_detail", Unit: "load", Category: "Load", Description: "5-minute load average"},
	{Name: "system.load_15m", Component: "load_detail", Unit: "load", Category: "Load", Description: "15-minute load average"},
	{Name: "memory.used_percent", Component: "memory", Unit: "percent", Category: "Memory", Description: "Memory not available to applications", Core: true, Percent: true},
	{Name: "memory.total_bytes", Component: "memory_detail", Unit: "bytes", Category: "Memory", Description: "Installed memory"},
	{Name: "memory.available_bytes", Component: "memory_detail", Unit: "bytes", Category: "Memory", Description: "Memory available without swapping"},
	{Name: "memory.dirty_bytes", Component: "memory_detail", Unit: "bytes", Category: "Memory", Description: "Memory waiting to be written to disk"},
	{Name: "memory.commit_percent", Component: "memory_detail", Unit: "percent", Category: "Memory", Description: "Committed address space versus commit limit (can exceed 100)", Thresholded: true, Warn: 90, Crit: 100},
	{Name: "memory.swap_used_percent", Component: "swap", Unit: "percent", Category: "Memory", Description: "Swap in use (0 when no swap is configured)", Percent: true, Thresholded: true, Warn: 50, Crit: 80},
	{Name: "memory.swap_in_pages_per_sec", Component: "swap", Unit: "per_second", Category: "Memory", Description: "Pages swapped in per second"},
	{Name: "memory.swap_out_pages_per_sec", Component: "swap", Unit: "per_second", Category: "Memory", Description: "Pages swapped out per second"},
	{Name: "memory.major_faults_per_sec", Component: "memory_detail", Unit: "per_second", Category: "Memory", Description: "Major page faults per second"},
	{Name: "memory.oom_kills_total", Component: "memory_detail", Unit: "events", Category: "Memory", Description: "Out-of-memory kills since boot"},
	{Name: "pressure.cpu_some_avg10", Component: "pressure", Unit: "percent", Category: "Pressure (PSI)", Description: "Share of last 10s some tasks stalled on CPU", Percent: true, Thresholded: true, Warn: 20, Crit: 50},
	{Name: "pressure.memory_some_avg10", Component: "pressure", Unit: "percent", Category: "Pressure (PSI)", Description: "Share of last 10s some tasks stalled on memory", Percent: true, Thresholded: true, Warn: 10, Crit: 30},
	{Name: "pressure.io_some_avg10", Component: "pressure", Unit: "percent", Category: "Pressure (PSI)", Description: "Share of last 10s some tasks stalled on I/O", Percent: true, Thresholded: true, Warn: 20, Crit: 50},
	{Name: "filesystem.used_percent", Component: "filesystem", Unit: "percent", Category: "Filesystem", Description: "Block usage per configured path", Core: true, Percent: true, Scoped: true},
	{Name: "filesystem.inodes_used_percent", Component: "filesystem_detail", Unit: "percent", Category: "Filesystem", Description: "Inode usage per configured path", Percent: true, Scoped: true, Thresholded: true, Warn: 85, Crit: 95},
	{Name: "filesystem.available_bytes", Component: "filesystem_detail", Unit: "bytes", Category: "Filesystem", Description: "Bytes available to unprivileged users per path", Scoped: true},
	{Name: "filesystem.read_only", Component: "filesystem_detail", Unit: "boolean", Category: "Filesystem", Description: "1 when the path is on a read-only mount", Boolean: true, Scoped: true, Thresholded: true, Warn: 1, Crit: 1},
	{Name: "disk.read_bytes_per_sec", Component: "disk", Unit: "bytes_per_second", Category: "Disk I/O", Description: "Bytes read from physical disks per second"},
	{Name: "disk.write_bytes_per_sec", Component: "disk", Unit: "bytes_per_second", Category: "Disk I/O", Description: "Bytes written to physical disks per second"},
	{Name: "disk.read_iops", Component: "disk", Unit: "per_second", Category: "Disk I/O", Description: "Read operations per second"},
	{Name: "disk.write_iops", Component: "disk", Unit: "per_second", Category: "Disk I/O", Description: "Write operations per second"},
	{Name: "disk.io_utilization_percent", Component: "disk", Unit: "percent", Category: "Disk I/O", Description: "Busiest physical disk, share of interval with I/O in flight", Percent: true, Thresholded: true, Warn: 80, Crit: 95},
	{Name: "network.rx_bytes", Component: "network", Unit: "bytes", Category: "Network", Description: "Received bytes counter, all interfaces", Core: true},
	{Name: "network.tx_bytes", Component: "network", Unit: "bytes", Category: "Network", Description: "Transmitted bytes counter, all interfaces", Core: true},
	{Name: "network.errors_total", Component: "network_detail", Unit: "packets", Category: "Network", Description: "Receive and transmit errors since boot, non-loopback"},
	{Name: "network.dropped_total", Component: "network_detail", Unit: "packets", Category: "Network", Description: "Receive and transmit drops since boot, non-loopback"},
	{Name: "network.rx_bytes_per_sec", Component: "network_detail", Unit: "bytes_per_second", Category: "Network", Description: "Receive rate, non-loopback"},
	{Name: "network.tx_bytes_per_sec", Component: "network_detail", Unit: "bytes_per_second", Category: "Network", Description: "Transmit rate, non-loopback"},
	{Name: "network.interfaces_up", Component: "network_detail", Unit: "interfaces", Category: "Network", Description: "Non-loopback interfaces with operstate up"},
	{Name: "network.tcp_established", Component: "network_detail", Unit: "connections", Category: "Network", Description: "Established TCP connections"},
	{Name: "network.tcp_retransmits_per_sec", Component: "network_detail", Unit: "per_second", Category: "Network", Description: "TCP segments retransmitted per second"},
	{Name: "network.conntrack_used_percent", Component: "network_detail", Unit: "percent", Category: "Network", Description: "Connection-tracking table usage", Percent: true, Thresholded: true, Warn: 80, Crit: 95},
	{Name: "process.count", Component: "process", Unit: "processes", Category: "System", Description: "Processes in /proc", Core: true},
	{Name: "process.zombies", Component: "process_detail", Unit: "processes", Category: "System", Description: "Zombie processes", Thresholded: true, Warn: 10, Crit: 100},
	{Name: "process.blocked", Component: "process_detail", Unit: "processes", Category: "System", Description: "Processes blocked on I/O"},
	{Name: "process.threads_total", Component: "process_detail", Unit: "threads", Category: "System", Description: "Kernel scheduling entities"},
	{Name: "system.file_descriptors_used_percent", Component: "system", Unit: "percent", Category: "System", Description: "Allocated file handles versus system maximum", Percent: true, Thresholded: true, Warn: 80, Crit: 95},
	{Name: "process.pid_used_percent", Component: "system", Unit: "percent", Category: "System", Description: "Process count versus kernel.pid_max", Percent: true, Thresholded: true, Warn: 80, Crit: 95},
	{Name: "system.uptime_seconds", Component: "system", Unit: "seconds", Category: "System", Description: "Seconds since boot"},
	{Name: "system.kernel_tainted", Component: "system", Unit: "boolean", Category: "System", Description: "1 when the kernel taint mask is non-zero", Boolean: true},
}

var byName = func() map[string]Def {
	result := make(map[string]Def, len(catalog))
	for _, def := range catalog {
		result[def.Name] = def
	}
	return result
}()

// All returns the catalog in display order.
func All() []Def { return append([]Def(nil), catalog...) }

func Lookup(name string) (Def, bool) {
	def, ok := byName[name]
	return def, ok
}

// Extended returns the non-core definitions.
func Extended() []Def {
	result := make([]Def, 0, len(catalog))
	for _, def := range catalog {
		if !def.Core {
			result = append(result, def)
		}
	}
	return result
}

// IsAdvisoryComponent reports whether a component holds only extended metrics.
func IsAdvisoryComponent(component string) bool {
	found := false
	for _, def := range catalog {
		if def.Component == component {
			if def.Core {
				return false
			}
			found = true
		}
	}
	return found
}

// ThresholdKeys lists metric names whose thresholds may be configured.
func ThresholdKeys() []string {
	keys := make([]string, 0, len(catalog))
	for _, def := range catalog {
		if def.Thresholded {
			keys = append(keys, def.Name)
		}
	}
	sort.Strings(keys)
	return keys
}
