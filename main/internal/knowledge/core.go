package knowledge

func num(value float64) *float64 { return &value }

func elevated(metric string) Condition { return Condition{Metric: metric, Op: OpElevated} }
func normal(metric string) Condition   { return Condition{Metric: metric, Op: OpNormal} }
func atLeast(metric string, value float64) Condition {
	return Condition{Metric: metric, Op: OpAtLeast, Value: num(value)}
}
func below(metric string, value float64) Condition {
	return Condition{Metric: metric, Op: OpBelow, Value: num(value)}
}

// Core returns the knowledge shipped with the binary. These are common,
// conservative host patterns. Value comparisons use fixed defaults that an
// operator may replace with a local pattern of the same ID. Patterns are
// descriptions of what a combination may indicate, not diagnoses or actions.
func Core() []Pattern {
	return []Pattern{
		{ID: "core.cpu.saturation", Domain: "cpu", Title: "CPU saturation", Severity: SeverityWarning,
			Interpretation: "CPU is busy and work is queueing, so runnable work is waiting for CPU time.",
			All:            []Condition{elevated("cpu.utilization_percent")},
			Any:            []Condition{elevated("system.load_1m"), elevated("pressure.cpu_some_avg10")}},
		{ID: "core.cpu.busy_without_queueing", Domain: "cpu", Title: "High CPU without queueing", Severity: SeverityInfo,
			Interpretation: "CPU is busy but load is normal and CPU stall pressure is low; this is consistent with expected processing load, though it does not prove it.",
			All:            []Condition{elevated("cpu.utilization_percent"), normal("system.load_1m"), below("pressure.cpu_some_avg10", 10)}},
		{ID: "core.cpu.hypervisor_contention", Domain: "cpu", Title: "Hypervisor CPU contention", Severity: SeverityWarning,
			Interpretation: "The hypervisor is withholding CPU time from this host; local processes cannot fix this.",
			All:            []Condition{elevated("cpu.steal_percent")}},
		{ID: "core.memory.pressure", Domain: "memory", Title: "Memory pressure", Severity: SeverityWarning,
			Interpretation: "Memory use is high and the kernel reports stalls or paging activity, so applications may be slowed by memory reclaim.",
			All:            []Condition{elevated("memory.used_percent")},
			Any:            []Condition{elevated("pressure.memory_some_avg10"), atLeast("memory.swap_out_pages_per_sec", 1), atLeast("memory.major_faults_per_sec", 100)}},
		{ID: "core.memory.swapping", Domain: "memory", Title: "Active swapping", Severity: SeverityWarning,
			Interpretation: "Pages are moving to or from swap right now, which usually slows applications.",
			All:            nil,
			Any:            []Condition{atLeast("memory.swap_in_pages_per_sec", 10), atLeast("memory.swap_out_pages_per_sec", 10)}},
		{ID: "core.memory.oom_history", Domain: "memory", Title: "Out-of-memory kills since boot", Severity: SeverityWarning,
			Interpretation: "The kernel has killed at least one process for lack of memory since boot. The counter is historical and does not say when it happened.",
			All:            []Condition{atLeast("memory.oom_kills_total", 1)}},
		{ID: "core.memory.overcommit", Domain: "memory", Title: "Memory overcommit near limit", Severity: SeverityWarning,
			Interpretation: "Committed memory is near or above the kernel commit limit; new allocations may fail.",
			All:            []Condition{elevated("memory.commit_percent")}},
		{ID: "core.storage.disk_full", Domain: "storage", Title: "Filesystem capacity low", Severity: SeverityWarning,
			Interpretation: "A monitored filesystem is nearly full; writes may soon fail.",
			All:            []Condition{elevated("filesystem.used_percent")}},
		{ID: "core.storage.inode_exhaustion", Domain: "storage", Title: "Inode exhaustion", Severity: SeverityWarning,
			Interpretation: "A filesystem is running out of inodes; files cannot be created even if free space remains.",
			All:            []Condition{elevated("filesystem.inodes_used_percent")}},
		{ID: "core.storage.read_only", Domain: "storage", Title: "Filesystem mounted read-only", Severity: SeverityCritical,
			Interpretation: "A monitored path is on a read-only mount; writes will fail. This can follow a filesystem error or be intentional.",
			All:            []Condition{atLeast("filesystem.read_only", 1)}},
		{ID: "core.storage.disk_saturation", Domain: "storage", Title: "Disk saturation", Severity: SeverityWarning,
			Interpretation: "A physical disk is busy for most of the sample interval; I/O requests are likely queueing.",
			All:            []Condition{elevated("disk.io_utilization_percent")}},
		{ID: "core.storage.bottleneck", Domain: "storage", Title: "Possible storage bottleneck", Severity: SeverityWarning,
			Interpretation: "CPU is mostly idle while tasks wait on I/O; the host appears limited by storage rather than compute.",
			All:            []Condition{below("cpu.utilization_percent", 50)},
			Any:            []Condition{elevated("cpu.iowait_percent"), elevated("pressure.io_some_avg10")}},
		{ID: "core.network.connection_table_exhaustion", Domain: "network", Title: "Connection-tracking table filling", Severity: SeverityWarning,
			Interpretation: "The conntrack table is near its limit; new connections may be dropped, which is typical of a connection storm.",
			All:            []Condition{elevated("network.conntrack_used_percent")}},
		{ID: "core.network.no_interface_up", Domain: "network", Title: "No non-loopback interface is up", Severity: SeverityCritical,
			Interpretation: "No network interface other than loopback reports operstate up; the host may be unreachable.",
			All:            []Condition{below("network.interfaces_up", 1)}},
		{ID: "core.system.file_descriptor_exhaustion", Domain: "system", Title: "File descriptor exhaustion", Severity: SeverityWarning,
			Interpretation: "System-wide file handles are nearly exhausted; programs may fail to open files or sockets.",
			All:            []Condition{elevated("system.file_descriptors_used_percent")}},
		{ID: "core.system.pid_exhaustion", Domain: "system", Title: "Process ID exhaustion", Severity: SeverityWarning,
			Interpretation: "The process count is near kernel.pid_max; new processes may fail to start.",
			All:            []Condition{elevated("process.pid_used_percent")}},
		{ID: "core.system.zombie_accumulation", Domain: "system", Title: "Zombie process accumulation", Severity: SeverityWarning,
			Interpretation: "Many exited processes are not being reaped by their parents, which may indicate a faulty parent process.",
			All:            []Condition{elevated("process.zombies")}},
		{ID: "core.system.kernel_tainted", Domain: "system", Title: "Kernel is tainted", Severity: SeverityInfo,
			Interpretation: "The kernel taint mask is non-zero (for example an out-of-tree module or a prior kernel warning). This is informational and may be long-standing.",
			All:            []Condition{atLeast("system.kernel_tainted", 1)}},
	}
}
