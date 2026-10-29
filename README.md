# Perceptron SRE -- Version 0.0.1

**Explainable operational reasoning for the systems you already run.**

Perceptron SRE (Site Reliability Engine) is an application in the **PerceptronES (Perceptron Engineering Solutions)** family. It is being built to help infrastructure and service teams move from “an alert fired” to “here is what the available evidence says, what it may mean for the service, and what to do next.”

**Project creator and architect:** Camden Charles Marble  
**Development:** Architected and designed by Camden Charles Marble, with AI-assisted coding.  
**Published by:** Perceptron Engineering Solutions (PerceptronES)

It is designed to work alongside existing monitoring and control systems—not replace them. Feed it metrics and events, let its engines add operational context, then send its findings back to the monitoring stack your team already uses.

> **Development status:** early, working single-host prototype. The current Go binary collects Linux host observations, runs host pressure/health reasoning, optionally assesses operator-supplied security evidence, offers a separate bounded Simulation page with configurable synthetic inputs and full event reports, produces mission assessments and provenance, serves a dashboard (loopback by default, configurable listen address), and exposes current-cycle JSON and Prometheus metrics. Automated security scanners, broad integrations, durable incident history, package isolation, and automated mitigation are not current capabilities.

## Why Perceptron SRE?

Most alerting starts with a signal: CPU is high, a queue is growing, or requests are failing. An engineer then has to correlate dashboards, understand dependencies, decide whether customers are affected, and determine what action is safe.

Perceptron SRE aims to make more of that reasoning visible and repeatable:

```text
Signals → evidence → component assessments → service/mission impact
        → explanation → recommendation → policy decision → verified outcome
```

The goal is to reduce routine on-call investigation and context switching, while making uncertainty and missing evidence clear. When an action is not available or permitted, the system should hand a useful explanation and incident record to a human—not quietly guess or act.

## Binary alerts and Perceptron-style reasoning

A conventional binary alert often asks one narrow question:

```text
Is CPU above 95%?  Yes / No
```

That can be useful, but it does not say whether the measurement is fresh, whether other required signals are missing, which component is affected, whether a service objective is threatened, or what evidence supports a response.

Perceptron SRE uses **perceptrons** as a system-design idea: small, focused reasoning units with explicit inputs and outputs contribute to a larger, inspectable assessment. A host pressure engine can interpret a metric; a health engine can derive component condition; other domain engines can contribute networking, DNS, queue, or service evidence; and the core can aggregate that evidence against a mission objective.

This is **not a claim that the current product is a trained neural network**. The current prototype uses explicit, deterministic rules. Its purpose is to compose bounded reasoning and show the evidence and provenance behind the result, rather than return an opaque binary alert.

### Deterministic by design

The core reasoning path is built around explicit rules, versioned events, configuration, and recorded inputs. Given the same engine version, configuration, and inputs, deterministic engines should produce the same result. Unknown, stale, unavailable, or contradictory evidence remains visible instead of being turned into “healthy.”

Perceptron SRE is **not an unbounded LLM making operational decisions**. The current core does not use an LLM to invent diagnoses or authorize actions. Future optional analytical engines would still need explicit contracts, provenance, validation, and policy boundaries; they would not become authority simply by being present.

## What you can do today

- Run a single binary on a Linux host to collect CPU, memory, filesystem, load, network, and process observations.
- Optionally probe DNS resolution for a configured name.
- Run the current host pressure and health engines and receive a core mission assessment.
- Optionally enable the bundled security-posture engine to assess configured, source-attributed security evidence; stale/missing evidence cannot support a favorable mission state.
- Explore baseline, pressure, missing/stale evidence, and security scenarios on a separate Simulation page. Adjust synthetic measurements and thresholds, then inspect the generated incident summary, all events, and full report without changing the live report.
- Inspect the assessment pyramid, component evidence, event lineage, and configuration in a local dashboard.
- Adjust supported thresholds and collector configuration through a live-reloadable JSON file or dashboard.
- Enable, disable, and order configured engines from the Engine Store page.
- Retrieve the latest full cycle as JSON and expose current-cycle metrics at `/metrics` for Prometheus-compatible scrapers such as Grafana.
- Run with no domain engines enabled; the core still reports a mission assessment with unknown/missing coverage rather than claiming health.

The current dashboard binds to loopback by default; a non-loopback listen address is an explicit opt-in without authentication or TLS. The metrics endpoint is a scrape view, not a remote-write exporter or paging service. The current prototype retains only the latest cycle in memory; it does not yet generate a durable incident report with one minute of pre-incident history and five minutes after mitigation.

## How it fits your existing observability stack

Perceptron SRE is intended to **complement** tools such as Prometheus, Grafana, and existing alert managers:

1. Existing collectors and systems remain in place.
2. A Perceptron adapter will normalize selected metrics/events into versioned observations with source, time, unit, and scope.
3. Configured engines derive assessments and findings with evidence lineage.
4. Perceptron exposes complete findings and bounded metrics for the existing monitoring system to consume.
5. Your existing alerting and paging rules can route the improved signals to the on-call team.

The current prototype provides a Prometheus-compatible `/metrics` scrape endpoint and a JSON findings API. Generic legacy-metric ingestion, external destination adapters, remote write, durable event history, and direct paging integrations are future work. Perceptron is not intended to require a replacement metrics database or dashboard.

## Engine Store and contribution model

An **engine** is a focused domain reasoning or observation package—for example, host health, networking, DNS, queues, or databases. Engines declare their category, version, event contracts, settings, and requested policy profile.

The Perceptron core remains responsible for cross-engine services such as event transport, mission aggregation, policy authority, and platform APIs. These are not installed as domain engines.

The dashboard’s Engine Store is currently a **local catalog and configuration surface**, not a hosted marketplace. It can list bundled engines and operator-declared local plugins, show their contracts and policy metadata, and enable or disable compatible entries. It does not download packages, discover a remote registry, verify publisher signatures, or sandbox code.

### Submitting an engine

We welcome domain expertise and integrations. The contribution path is:

1. **Start with a focused problem.** Describe the signal or operational question the engine addresses, its intended category, and the systems it needs to integrate with.
2. **Propose the contract.** Define versioned input/output event types, entity scope, units, freshness behavior, unknown/error handling, configuration settings, resource limits, and provenance.
3. **Declare the safety profile.** State whether the engine is reasoning-only, human-escalation, or proposes self-mitigation; list any tools/scripts and required privileges. A declaration is not a permission grant.
4. **Build and test against the protocol.** The current external-engine protocol is documented in [`main/ENGINE-PLUGIN-PROTOCOL.md`](main/ENGINE-PLUGIN-PROTOCOL.md). Include conformance tests for valid outputs, missing/stale inputs, malformed data, timeouts, and failures.
5. **Document trust and operations.** Include maintainer, versioning, dependencies, supported platforms, network/file access, upgrade and rollback notes, and example configuration.
6. **Open an issue or pull request.** Use the repository’s contribution process to discuss the proposal and submit code/docs/tests. Engine packages are intended for `main/external/<engine-name>`; the directory currently contains contributor guidance, while package discovery and first-party migration are still in progress.

External engines are executable code. Today, configured plugins run with the Perceptron process user’s operating-system privileges and are **not sandboxed**. Policy metadata such as `read_only` cannot enforce OS permissions. Only run code you trust. Tool/script brokerage and self-mitigation execution are not enabled in the current prototype.

## Project phases

The project is intentionally staged so operational reasoning and safety are demonstrated before autonomy or distributed deployment.

| Phase | Focus | Status |
|---|---|---|
| **0 — Contracts** | Define event, engine, evidence, and acceptance contracts. | Architecture/RFC work is underway; RFCs remain review-ready proposals unless explicitly accepted. |
| **1 — Single-host core** | Observe, reason, explain on one Linux host; dashboard and initial engine ecosystem. | **Current prototype.** Basic host chain, local dashboard, live config, external process protocol, JSON findings, and Prometheus scrape output exist. Important gaps remain, including durable history, generic legacy ingestion, engine sandboxing, and migration of bundled engines to the external package boundary. |
| **2 — Local DNS lab** | Exercise DNS health, traffic, impact, investigation, knowledge, and a local twin rehearsal. | Planned. |
| **3 — Integrated DNS control/data planes** | Validate end-to-end change workflows with isolated twins and policy-gated decisions. | Planned; production action paths stay disabled until required policy and security gates are accepted. |
| **4 — Regional operation** | Test locality, federation, ownership, partitions, and global mission coverage. | Planned. |
| **5 — Production readiness** | Complete security, reliability, operations, and any explicitly approved bounded autonomy. | Planned and gated; not implied by earlier phases. |

See the [implementation roadmap](Implementation/IMPLEMENTATION-ROADMAP.md) and [architecture RFC index](DesignDocuments/RFC-INDEX.md) for detail. The [diagram catalog](DesignDocuments/diagrams/README.md) illustrates the major flows and use cases.

## Build and run

Requires Go. From the `main` directory:

```bash
go test ./...
go build -o perceptron ./cmd/perceptron
./perceptron --format text
```

To build a static Linux amd64 binary from a Go-capable build host:

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o perceptron-linux-amd64 ./cmd/perceptron
```

Run the dashboard on the same host:

```bash
./perceptron --web --interval 15s
```

Then visit `http://127.0.0.1:8080`. To select a configuration file, use `--config path/to/perceptron.json`. A missing file is created with defaults; valid file edits are applied before the next collection cycle.

See [`main/README.md`](main/README.md) for the current implementation details, options, API paths, and limitations.

## Design principles

- **Evidence over assertion:** show the input, source, time, scope, and derivation behind conclusions.
- **Unknown is a real result:** missing or stale evidence is not healthy evidence.
- **No default green:** favorable mission status requires complete, valid evidence for the configured scope.
- **Deterministic where declared:** repeatability is an engine contract with explicit scope, not a blanket claim about every future extension.
- **Test failure behavior:** missing, stale, contradictory, invalid, and unavailable inputs are part of the product's test surface.
- **Separate reasoning from authority:** a finding or recommendation is not authorization.
- **Bounded components:** engines have explicit contracts and responsibilities.
- **Preserve existing systems:** integrate with legacy monitoring and owning control planes.
- **Human escalation remains first-class:** unsafe, unsupported, or uncertain cases should produce a useful handoff.
- **Earn autonomy:** automated mitigation requires explicit tools, least privilege, independent policy enforcement, approvals where required, validation, and outcome verification.
- **Simulation before exposure:** use synthetic, reproducible scenarios and isolated validation before production workflows where applicable.
- **Security is operational evidence:** security conditions should eventually contribute to mission reasoning with the same provenance and uncertainty discipline.

## Documentation

- [Overall architecture](DesignDocuments/01-overall-architecture.md) (includes engineering philosophy, section 4.23-4.29)
- [RFC index](DesignDocuments/RFC-INDEX.md)
- [Implementation roadmap](Implementation/IMPLEMENTATION-ROADMAP.md)
- [Architecture diagrams](DesignDocuments/diagrams/README.md)
- [Core application guide](main/README.md)
- [Engine plugin protocol](main/ENGINE-PLUGIN-PROTOCOL.md)
- [Changelog](CHANGELOG.md)

## License

Perceptron SRE is released under the [MIT License](LICENSE).
