# Perceptron SRE Core v0.1

This directory contains the first single-host Go binary. It implements a deliberately narrow **observe → reason → explain** path. The reviewed architecture and implementation roadmap remain the source documents; this README records the executable's provisional choices and current limits without changing those documents.

## Requirements

- Go 1.13 or newer.
- Linux for host collection. The binary also builds on other platforms, but reports Linux-only measurements as unavailable and the mission as `UNKNOWN`.

## Build and run

From this directory:

```powershell
go test ./...
go build -o perceptron.exe ./cmd/perceptron
.\perceptron.exe --format text
```

On Linux, omit the `.exe` suffix. Without flags the program collects one snapshot and prints a compact JSON report to stdout; structured JSON logs go to stderr. Use `--watch --interval 15s` to collect repeated one-line JSON reports until interrupted. Use `--print-default-config` to print the provisional settings.

### Local dashboard

Run a continuously refreshed, read-only dashboard bound only to IPv4 loopback:

```bash
./perceptron-linux-amd64 --web
```

Open `http://127.0.0.1:8080` on the same machine. Dashboard mode implies repeated collection, using the default 15-second cycle interval; the page refreshes its report every 3 seconds. Change the cycle interval with `--interval 30s` and the local-only port with `--web-port 8081`. The dashboard never listens on a public or LAN interface. If the selected port is already in use, startup fails explicitly.

The dashboard uses a top-down decision pyramid: mission state and recommended next step at the top; evidence-based root-cause assessment, reasoning engines, and component health below; then raw observations, log-stream guidance, and a chronological event/provenance record. The diagnosis is labeled as leading evidence, not a causal claim beyond what the measurements support. It keeps engine execution status separate from component health and shows unknown/unavailable evidence explicitly. Operations and Engine Store are separate pages; the Store lists bundled and locally declared engines, their categories, event contracts, policy declarations, and enablement state. Collector settings remain core configuration; thresholds and settings are grouped under the engine that owns them. The top recommendation control is **Run read-only refresh**: it queues one fresh host observation cycle; it does not run shell commands or make host changes. Runtime logs are emitted to stderr (typically collected by the system journal under systemd) and are not embedded in the dashboard.

The latest complete assessment is available as JSON from `/api/report` and `/api/findings`; Prometheus-compatible current-cycle metrics are exposed at `/metrics` for existing scrapers such as Prometheus/Grafana. The metrics endpoint is a scrape projection, not a remote-write exporter, alert router, or durable history. The dashboard still binds to loopback only.

The timeline currently contains only events from the latest in-memory assessment cycle. It does not yet retain the 60 seconds before an incident, record a mitigation event, or capture the five-minute post-mitigation recovery window, so it cannot yet produce the requested time-bounded incident report. This prototype has no remediation tools, confidence-based execution, or action authorization; future action tools must be introduced behind the policy and approval boundary rather than by enabling automatic execution in this dashboard.

When the binary is running on a remote Oracle Linux host, use an SSH local port forward from your client:

```bash
ssh -L 8080:127.0.0.1:8080 perceptron-oracle
```

Then open `http://127.0.0.1:8080` in the client browser. The SSH connection remains open while forwarding is active. The dashboard has no authentication of its own; loopback binding and the SSH tunnel are its access boundary, so do not expose the listener through a proxy or change it to a non-loopback bind without adding an appropriate access-control design.

The app reads `perceptron.json` from the current directory by default and creates it with initial values when it does not exist. Use `--config path/to/config.json` to select another file. The file is checked before each collection cycle; a valid external edit is applied on the next cycle without restarting. Invalid edits are logged and the last-known-good configuration remains active. Dashboard edits are validated, atomically written to that same file, and applied on the next assessment cycle.

The JSON configuration controls the host identifier, absolute Linux filesystem paths, CPU sample interval, maximum observation age, warning/critical thresholds, optional DNS probe target/timeout, and engine enablement. Unknown fields and invalid threshold ranges are rejected. The dashboard exposes current CPU, memory, filesystem, and per-CPU load warning/critical values, collector inputs, and engine enable/order controls. Example:

```json
{
  "host_id": "host-a",
  "filesystem_paths": ["/", "/var"],
  "dns_probe_name": "service.example.test",
  "dns_probe_timeout": "2s",
  "cpu_sample_interval": "250ms",
  "max_observation_age": "30s",
  "thresholds": {
    "cpu_percent": { "warning": 85, "critical": 95 },
    "memory_percent": { "warning": 85, "critical": 95 },
    "filesystem_percent": { "warning": 85, "critical": 95 },
    "load_per_cpu": { "warning": 1.5, "critical": 3 }
  },
  "engines": ["host-pressure", "host-health"]
}
```

Set `"engines": []` to run the collection core with no domain reasoning engines. The core still emits observations and an `UNKNOWN` mission assessment with missing coverage; mission aggregation is a core service, not a plugin.

### Engine extensions

The core has an explicit engine allow-list and can run with no engines. Built-in engines can be selected and ordered in the dashboard. Additional engines use the versioned, language-neutral subprocess protocol documented in [ENGINE-PLUGIN-PROTOCOL.md](./ENGINE-PLUGIN-PROTOCOL.md): declare a trusted executable, version, category, role, input/output event types, timeout, engine-owned JSON settings, and policy metadata in the config file, then enable its exact name. The core verifies output schema, cycle/configuration provenance, declared output types, and direct derivation lineage. Local package guidance lives under `external`.

Policy declarations (`reasoning_only`, `human_escalation`, `self_mitigation`, `read_only`, tool/script lists) are descriptive inputs, not security enforcement. Plugins are arbitrary executable code running with the Perceptron process user's privileges; they are not sandboxed and can perform any operation allowed by the OS identity. There is no automatic marketplace installation or signature verification. Only install trusted code and use least-privilege service accounts. The current runtime has no tool/script broker; self-mitigation and writable plugins are rejected for enablement.

The DNS probe is disabled unless `dns_probe_name` is explicitly configured. When enabled, the collector sends a name-resolution query through the operating system's configured resolver; it records only success/failure, latency, and the configured name, not returned addresses.

## v0.1 implementation choices

- Events use an in-memory append-only store for each assessment cycle. Events have generated IDs, a cycle correlation ID, occurrence/emission times, source and producer version, a SHA-256 identity for the normalized runtime configuration, type/schema version, host/entity references, reasoning/safety classification, payload, and derivation links.
- Bundled engines are currently compiled into the binary and run in-process; configured external engines use the versioned JSON process protocol. This is a transitional mismatch with the target that all domain engines use a common package boundary. An empty engine list is valid. The runtime validates output schemas, engine identity, event types, configuration/cycle provenance, and derivation lineage. Process boundaries do not sandbox plugins.
- Linux collectors read CPU/load/memory/network/process data from `/proc` and filesystem utilization from `statfs`. Memory utilization requires an explicit `MemAvailable` field; older kernels without it report memory as unavailable rather than assuming zero available memory. Network values are cumulative byte counters, not rates. Process count is not process-level health.
- An explicitly configured DNS probe is evaluated as a required mission component; its failure is a negative observation, while a missing/stale probe result remains unknown.
- Pressure thresholds are prototype defaults and are configurable through JSON or the dashboard; edits are applied on the next collection cycle. They are not universal SLOs or policy. The load threshold is normalized by the Go runtime's detected CPU count.
- Mission scope is only the configured host's resource headroom for continued operation. Required v0.1 components are CPU, load, memory, and at least one configured filesystem; a configured DNS probe adds a required DNS component. `GREEN`, `YELLOW`, `RED`, and `UNKNOWN`, along with their deterministic mappings, are provisional labels. A high pressure assessment yields `YELLOW`; critical yields `RED`; required unavailable/stale evidence prevents a green result. A known warning/critical status remains visible alongside any unknown input, and the explanation warns that risk may be higher. Coverage completeness means each required component assessment is represented; it does not mean that the underlying evidence is available or fresh. Missing assessments, evidence availability, and freshness are reported separately.
- No confidence value is emitted. Confidence semantics are unresolved; the binary does not manufacture a confidence label from coverage or thresholds.
- Reports include the immutable event chain, assessment coverage/freshness, and basic collection/runtime counts, timings, goroutine count, and Go heap allocation. Non-GREEN cycles are logged at warning level. Structured JSON logs are written to stderr; the application does not create a log file. In a systemd deployment, stderr is normally collected by the system journal; for an interactive session, redirect stderr to a file if persistent logs are needed. `/api/findings` returns the complete latest cycle and `/metrics` exposes bounded Prometheus text output; neither retains historical cycles.
- The program is observe-only. It has no policy engine, production action adapter, investigation/knowledge service, DNS integration, twin, database, or federation.

## Output and safety behavior

Unavailable measurements are represented with an explicit reason and no numeric value. Stale or future-dated evidence is not evaluated as normal. If required evidence is missing, the mission assessment is `UNKNOWN` or remains visibly degraded/critical with incomplete coverage; absence is never interpreted as a healthy measurement. Engine and output-validation failures are surfaced as errors and cause a nonzero exit rather than a success-shaped assessment.

This is a development prototype, not a production-ready monitoring or remediation agent. Thresholds, event schema, status definitions, and runtime contracts need review before being treated as stable interfaces.
