# RFC-0012: Engine Store and Telemetry Interoperability

**Status:** Proposed architecture update  
**Category:** Core platform and integration  
**Parent:** [Overall Architecture](01-overall-architecture.md)  
**Related RFCs:** RFC-0001 Event Bus and Event Model; RFC-0002 Engine Runtime; RFC-0004 Confidence and Evidence; RFC-0007 Mission Assurance; RFC-0010 Policy Engine

**Visual references:** [Legacy telemetry interoperability](diagrams/D-09-legacy-telemetry.md); [engine package, Store, and trust lifecycle](diagrams/D-12-engine-store.md); [system context and trust boundaries](diagrams/D-01-system-context.md).

## 1. Summary

This RFC proposes a consistent engine package/catalog model and a vendor-neutral path for augmenting legacy observability systems. The dashboard Store presents bundled, locally authored, and locally declared third-party engines; it does not itself install executable code from the internet. Engine configuration is grouped by owning engine. Each package declares category, contracts, version, settings, and action-policy metadata.

The core accepts telemetry through versioned adapters, normalizes it into Perceptron events, runs configured reasoning engines, and exposes complete findings plus bounded metrics for downstream systems. This permits Perceptron SRE to consume existing signals and publish higher-level assessments back to an existing destination without making that vendor a core dependency.

## 2. Goals

- Treat each domain-specific engine, including first-party defaults, as a package with consistent metadata and lifecycle.
- Keep event transport, mission aggregation, policy authority, and core APIs in the platform core rather than the domain-engine Store.
- Support local engine development and operator-installed third-party engines through a common contract.
- Make enablement, ordering, origin, version, settings, capabilities requested, and action-policy profile visible to operators.
- Accept legacy telemetry through adapters that preserve source, scope, time, and provenance.
- Export complete findings in a machine-readable form and expose safe, bounded metrics for systems such as Grafana.
- Prevent policy metadata or a subprocess boundary from being represented as a security sandbox.

## 3. Non-goals

- Building a public package registry or automatic download/update service in the initial release.
- Declaring a package trusted solely because it appears in a catalog.
- Treating process isolation as privilege isolation.
- Requiring Grafana, Prometheus, OpenTelemetry, or any vendor as a core runtime dependency.
- Authorizing or executing production actions from an engine profile or confidence score.

## 4. Conceptual architecture

```text
Legacy telemetry source(s)
        │
        ▼
Versioned input adapter ──► Canonical Perceptron events
                                  │
                     ┌────────────┴────────────┐
                     ▼                         ▼
              Selected engines             Core services
              (domain reasoning)        (bus, mission, policy)
                     └────────────┬────────────┘
                                  ▼
                    Findings + provenance record
                         │                 │
                         ▼                 ▼
                  Bulk JSON API       Metrics adapter
                                            │
                                  Existing telemetry system
```

Adapters are explicit boundaries. Input adapters translate external measurements/events into canonical observations with source identity, timestamps, entity scope, units, and availability. Output adapters project core findings into destination-specific formats and apply bounded cardinality, privacy, retry, and authentication rules. A cycle or finding is not silently dropped when an adapter fails; adapter health and delivery status must be observable.

## 5. Engine package and Store

### 5.1 Package contents

A package under `main/external/<engine-name>` should include:

- A manifest with stable name, semantic version, publisher/owner, category, role, description, protocol version, executable entrypoint, and supported platforms.
- Versioned input/output event type and payload schemas.
- Engine-owned settings schema and defaults.
- Determinism/state declaration, resource and timeout limits, and health semantics.
- Action-policy profile, read/write declaration, requested tools/scripts, and privilege/network/file requirements.
- Provenance, licensing, dependency, upgrade, rollback, and compatibility information.
- Conformance tests and a contribution guide.

First-party defaults use the same catalog, configuration, and protocol contract. The core retains event routing, mission aggregation, policy authority, and dashboard/store services. This RFC does not require the immediate physical relocation of current in-process engine source code; migration is a separate implementation step.

### 5.2 Operator Store behavior

The Store has three conceptual sources, matching the platform-wide Core + Extension model (Overall Architecture 4.25):

1. **Core (bundled):** first-party engines and packs delivered with a Perceptron release.
2. **Local Operator:** engines, knowledge, assessments, dashboards, or simulation packs authored or installed by the operator under the external package directory; private to the organization.
3. **Community (published third party):** a package published by an external maintainer, reviewed through pull request or Store review, and installed locally after operator review.

Package types are not limited to engines: tap, information, determination, and assessment engines, knowledge stores (RFC-0009 6.3), dashboards, and simulation packs all follow the same tiers. Source weights for knowledge are operator-configured precedence, not trust or confidence. Community votes, reviews, and ratings may inform review and visibility only; popularity is not truth and never enables or promotes a package automatically. Provenance (source tier, publisher, version) is preserved on every item the package contributes.

The Store shows source, publisher, version, enabled state, category, settings, declared contracts, requested capabilities, policy profile, trust/verification state, and compatibility. Every engine type is tagged with the cascade layer it applies to (Layer-0 Input through Layer-8 Learning, or Unmapped when its role fits no layer), and the Store can be filtered by layer and searched. The dashboard page is named "Store". Install/upgrade is distinct from enablement. Discovery must not execute package code; install and enablement are separate operator actions. The initial dashboard can catalog bundled and locally declared engines but has no remote registry, signature verification, or automated installation.

## 6. Action-policy metadata and enforcement

Candidate policy profiles:

- `reasoning_only`: analyze inputs and publish assessments/recommendations.
- `human_escalation`: create an attributable handoff for human review.
- `self_mitigation`: propose a bounded action for an independent policy evaluation.

Every package also declares `read_only`, `allowed_tools`, and `allowed_scripts`. These are declarations for review and policy evaluation. They do not constrain arbitrary executable code. A process running under the Perceptron service identity can use any OS permission available to that identity unless an OS sandbox or broker enforces narrower access.

The initial runtime remains observation/reasoning only. An engine cannot perform a tool call, run an arbitrary script, or make a production change by declaring a capability. Any future execution path must go through a core-owned broker and RFC-0010 policy authorization bound to the exact tool, arguments, target, scope, identity, evidence, policy version, and expiration. Missing enforcement means the capability remains unavailable.

## 7. Legacy telemetry interoperability

### 7.1 Input path

An input adapter maps a source contract into canonical events. Each measurement should preserve source and producer version, observation and receipt time, entity and tenant scope where applicable, unit, quality/availability, and source-specific provenance. Adapter configuration must define mapping, authentication, buffering, retry, rate limits, stale data behavior, and error reporting. Unknown or malformed values remain explicit failures/unknown evidence; they are not defaulted to healthy or zero.

Initial adapters should remain narrowly scoped and operator-configured. Candidate sources include Prometheus-compatible query/scrape, OpenTelemetry metrics/events, files/streams, and application-specific APIs. The first release does not select a universal input transport or promise that all source formats are supported.

### 7.2 Output path

The core exposes:

- A versioned bulk report/API containing the complete current-cycle findings and provenance.
- A bounded metrics projection for common numeric observations, component/mission states, engine status, and cycle health.
- Optional destination-specific exporters, each with explicit authentication, retry, buffering, and delivery semantics.

Prometheus exposition is suitable for a Prometheus-compatible scraper, including Grafana-managed collection. Metric labels must avoid unbounded values such as raw explanations, event IDs, customer identifiers, and arbitrary payload content. Detailed findings remain in the bulk report/event interface. A scrape endpoint is not the same as a remote-write exporter.

Perceptron should enrich, not seize ownership of, the legacy destination: operators can route the derived metrics to the same monitoring backend already in use. Alert routing and paging remain the responsibility of the configured monitoring/incident-management system unless a separately accepted integration contract assigns that responsibility to Perceptron.

## 8. Configuration and dashboard

The dashboard separates Operations from the Engine Store. Operations presents mission and evidence. Configuration groups global collectors/runtime options separately from each engine's own settings. The Store supports inspection and enable/disable of installed, compatible engines and links contributors to package requirements.

Configuration files remain the authoritative local source until a persistent registry contract is accepted. Configuration is version-controlled operational knowledge organized by domain: the core file holds platform keys (dashboard, policy, engines, plugins, knowledge), and optional `config.d/<domain>.json` overlays (host, network, security, dns, application) may set only their own domain's keys; unknown domains or keys are rejected (implemented; Overall Architecture 8.4). The listen address is configurable. Updates are validated before atomic persistence and applied on a subsequent cycle. Invalid edits retain last-known-good configuration and emit a diagnostic. Package binary/manifest changes are administrator-controlled and are not performed by dashboard settings updates. before atomic persistence and applied on a subsequent cycle. Invalid edits retain last-known-good configuration and emit a diagnostic. Package binary/manifest changes are administrator-controlled and are not performed by dashboard settings updates.

## 9. Security, privacy, and availability

- Treat third-party and locally authored executables as untrusted until reviewed; process execution is not sandboxing.
- Run under a dedicated least-privilege identity; do not expose dashboard or ingestion endpoints publicly without authentication and authorization.
- Do not pass service credentials or secrets to engines unless a separately enforced, scoped capability explicitly requires them.
- Apply privacy classification and redaction to both input telemetry and exported findings.
- Bound plugin runtime/output, adapter queues/retries, metric label cardinality, and request sizes.
- Make input/export adapter failures, drops, lag, and backpressure visible.
- Preserve mission unknown/coverage degradation when required source telemetry is absent or stale.
- Treat security posture signals as future first-class operational evidence, with source, scope, freshness, coverage, and uncertainty preserved.

## 10. Acceptance criteria

- Bundled and external engines have a common versioned package contract and visible catalog state.
- Every package declares its tier (Core, Local, Community) and provenance; community votes never enable or promote behavior.
- Configuration is grouped by core and engine ownership and rejects invalid settings explicitly.
- Store discovery is separate from package installation and execution.
- Policy declarations cannot themselves grant access or authorize actions.
- The complete current-cycle finding set is available in a machine-readable export.
- Metrics export preserves unknown/unavailable states and has documented cardinality bounds.
- At least one legacy source adapter and one destination adapter can be configured without changing core reasoning code.
- Adapter failure, malformed input, stale data, authentication failure, and destination outage are visible and tested.
- An exported mission/finding can be traced back to canonical events and sources.

The v0.1 runtime now includes an optional bundled `security-posture` evidence assessor. Operators or upstream integrations supply normalized check, state, source, summary, and observation time through `security_evidence`, and declare the exact mission scope through `security_required_checks`; the engine emits provenance-linked findings and marks stale or future-dated evidence `UNKNOWN`. When enabled, every declared check requires a matching finding, so an unrelated finding cannot hide a coverage gap. Enabling the engine without declaring any required checks leaves a missing `security-scope` and prevents GREEN. Findings qualify mission status but remain distinct from host component health and are not action authority. This engine does not scan packages, query CVEs, validate certificates, or verify an asserted source. Automated collectors and domain-specific security checks remain future work. Findings must not be presented as comprehensive security assurance.

The local dashboard also offers a separate Simulation page and bounded service with fixed synthetic scenarios for baseline, CPU pressure, missing/stale host evidence, and a critical security finding. Operators can vary synthetic measurements and thresholds. The page presents a generated incident summary, all cycle events, and the complete synthetic report. It runs only built-in engines, labels all results synthetic, and keeps them separate from the live report. This is not a general simulation/twin environment, production replay, or incident learning system.

## 11. Open decisions

1. What manifest and settings-schema formats and compatibility rules are normative?
2. What package publisher identity, signature, provenance, review, and update process governs third-party engines?
3. What process sandbox and capability broker are supported on each target OS?
4. Which input adapter and source contract should be implemented first for legacy-system integration?
5. Which output adapters are required beyond the initial Prometheus scrape and JSON bulk report?
6. What authentication, delivery, retry, buffering, and data-retention guarantees apply to each adapter?
7. Which stable metric names/labels and cardinality limits are approved for platform and domain findings?
8. How are customer/tenant identifiers and sensitive payload fields redacted or aggregated?
9. What incident/paging destination, if any, should Perceptron integrate directly rather than through the existing metrics system?
10. How are knowledge, dashboard, and simulation packs packaged and distributed across Core, Local, and Community tiers, and how are community review signals used without becoming trust?
11. How are per-package resource budgets (CPU, memory, storage, network, execution) declared and enforced (RFC-0002 6.2)?

## 12. Dependencies

RFC-0001 supplies canonical events and provenance. RFC-0002 defines engine packaging, runtime, catalog, and capabilities. RFC-0004 governs evidence quality and confidence. RFC-0007 defines mission outputs. RFC-0010 governs any action proposal or execution capability. Security, privacy, and deployment requirements must be accepted before remote adapters or production actions are enabled.
