# Perceptron SRE Implementation Roadmap

**Status:** Proposed implementation plan  
**Audience:** Project owners, implementers, reviewers, and AI contributors  
**Architecture baseline:** [DesignDocuments](../DesignDocuments/README.md)

## 1. Purpose

This roadmap turns the reviewed architecture into a staged implementation plan. It prioritizes validating the reasoning chain on one Linux host before adding a local DNS environment, a realistic DNS control plane, or regional federation.

The architecture RFCs are **Architecture review ready**, not accepted implementation specifications. This roadmap proposes implementation sequencing; it does not approve unresolved architecture decisions, production actions, availability guarantees, or security requirements. Each phase must record its implementation decisions and preserve the relevant open questions for later phases.

## 2. Program goals

The implementation should progressively demonstrate that Perceptron SRE can:

1. Observe operational conditions and retain source, time, scope, and provenance.
2. Derive pressure and health without presenting missing data as healthy.
3. Explain assessments from inputs through derived results.
4. Represent uncertainty, coverage, freshness, and unknown states distinctly.
5. Assess whether a scoped service mission remains achievable.
6. Gather investigation evidence and preserve alternative explanations.
7. Validate proposed changes away from production before any policy-governed promotion.
8. Enforce a clear boundary between recommendation, authorization, execution, and outcome.
9. Add regional continuity only when a demonstrated workflow or mission requires it.

## 3. Implementation principles

- **Prove the smallest useful system first.** Begin with a single Go binary on one Linux host. Do not start with DNS federation, autonomous remediation, or an AI/ML subsystem.
- **Observe before acting.** Early phases collect, derive, explain, and recommend; they do not execute production changes.
- **Keep the DNS fast path independent.** Analytical workloads and metadata consumers must not block DNS responses.
- **Preserve uncertainty.** Missing, stale, contradictory, partial, or unavailable inputs remain visible and cannot become healthy or authorized results by default.
- **Keep dimensions distinct.** Status, confidence, coverage, freshness, health, trust, and authorization are not interchangeable.
- **Use measured complexity.** Prefer Go for the core. Add C/eBPF/XDP or distributed infrastructure only after a measured requirement justifies it.
- **Avoid premature persistence and federation.** Do not introduce a database, broker, knowledge service, twin fleet, or regional fabric in the single-host proof unless an accepted requirement demands it.
- **Gate each expansion.** A phase is complete only when its observable outcomes, failure cases, and exit criteria are demonstrated.
- **Do not mistake review readiness for approval.** Resolve implementation-critical decisions in a short decision record or accepted RFC before depending on them.

## 4. Roadmap at a glance

| Phase | Name | Primary result | Main architecture references |
|---:|---|---|---|
| 0 | Implementation contracts | Small, explicit v0.1 decisions and measurable acceptance tests | [RFC-0001](../DesignDocuments/RFC-0001-event-bus-and-event-model.md), [RFC-0002](../DesignDocuments/RFC-0002-engine-runtime.md), [RFC-0004](../DesignDocuments/RFC-0004-confidence-and-evidence.md), [RFC-0007](../DesignDocuments/RFC-0007-mission-assurance.md), [RFC-0012](../DesignDocuments/RFC-0012-engine-store-and-telemetry-interoperability.md) |
| 1 | Single-host Perceptron Core | One Go core observes a Linux host, runs selected catalog engines, and exports explained findings | RFC-0001, RFC-0002, RFC-0004, RFC-0007, RFC-0012 |
| 2 | Local DNS service and traffic | A local DNS environment exercises DNS health, customer impact, investigation, and knowledge concepts without distributed infrastructure | [RFC-0003](../DesignDocuments/RFC-0003-operational-entity-model.md), [RFC-0005](../DesignDocuments/RFC-0005-traffic-handler.md), RFC-0007, [RFC-0008](../DesignDocuments/RFC-0008-investigation-framework.md), [RFC-0009](../DesignDocuments/RFC-0009-knowledge-engine.md) |
| 3 | Integrated DNS control and data planes | A realistic control-plane workflow is validated end-to-end with isolated twins and policy-gated decisions | [RFC-0006](../DesignDocuments/RFC-0006-twin-architecture.md), [RFC-0010](../DesignDocuments/RFC-0010-policy-engine.md), DNS integration contracts |
| 4 | Regional operation and federation | Regional boundaries, work ownership, partial failures, and global mission assessment are tested | [RFC-0011](../DesignDocuments/RFC-0011-regional-architecture-and-federation.md), with RFC-0001, RFC-0003, RFC-0007, and RFC-0010 |
| 5 | Production readiness and bounded autonomy | Security, reliability, operational ownership, rollback, and any approved action classes are production-qualified | Accepted security, deployment, remediation, observability, and policy contracts |

Phases are dependency-oriented, not calendar commitments. Work may overlap where contracts permit, but later-phase capabilities must not be enabled before their safety and acceptance gates are met.

## 5. Phase 0 — Implementation contracts

### Goal

Resolve only the decisions needed to build and test the single-host proof. Avoid extending the architecture merely to defer a narrow implementation choice.

### Chapters / work packages

#### 0.1 Select v0.1 boundaries

- Confirm Linux as the first runtime target and Go as the core language.
- Confirm the first executable is observe-only: no production action adapter.
- Record the migration target: every domain engine, including defaults, is a catalog package; the current in-process built-ins are transitional.
- Keep the first phase observe/reason-only. External subprocess protocol is not a sandbox; no action capability is enabled.
- Keep DNS-specific components out of the Phase 1 binary except for an optional configured DNS health probe.

#### 0.2 Specify the minimum event contract

Agree and test a small v1 envelope before building engines. At minimum decide:

- Event identity, type, and schema version.
- Source identity and event/observation time.
- Entity references and scope, where applicable.
- Correlation, causation, and derivation references, where applicable.
- Payload representation and validation behavior.
- Classification of observations versus derived assessments.

The field list is a starting point, not a fixed schema. Keep transport replaceable. For a single-process prototype, an in-process dispatcher is the simplest candidate; do not add a broker unless a phase requirement demonstrates the need.

#### 0.3 Set temporary v0.1 assessment labels

For display and test fixtures only, choose a small, explicit initial vocabulary:

- Mission status: **GREEN**, **YELLOW**, **RED**, **UNKNOWN**.
- Confidence: **LOW**, **MEDIUM**, **HIGH**, only where a confidence-bearing output is appropriate.

These are provisional implementation labels, not calibrated measures, universal policy thresholds, or final RFC semantics. Preserve the option to add ORANGE or other states after defining their operational meaning. Never infer a favorable status or confidence from unavailable inputs.

#### 0.4 Define visible output and tests

- Decide how operators inspect the current assessment and its explanation (for example, a CLI report or a local read-only endpoint).
- Define deterministic fixture inputs for normal, degraded, stale, missing, and contradictory observations.
- Set initial metric sampling cadence and resource budget from the test host; record them as prototype limits, not production SLOs.
- Make event retention limitations explicit. Phase 1 has no database; an in-memory event stream and structured process output do not provide durable incident history.

### Deliverables

- A short v0.1 decision record covering the choices above.
- Event-envelope examples and validation tests.
- Engine registration and assessment test fixtures.
- Explicit list of features not yet implemented.

### Exit criteria

- The event and engine interfaces are sufficiently stable for Phase 1.
- The provisional status/confidence labels have testable display behavior and cannot imply calibration.
- Observe-only authority is enforced by the absence of production execution adapters.

## 6. Phase 1 — Single-host Perceptron Core

### Goal

Prove **Observe → Reason → Explain** on one Linux server with one Go binary. This phase validates the core contracts—not DNS, federation, automation, or AI.

### Chapters / work packages

#### 1.1 Core process

- Configuration loading and validation.
- Structured logging and explicit error reporting.
- Process lifecycle, graceful shutdown, and context cancellation.
- Basic self-observation for the core process.

#### 1.2 Event flow and engine runtime

- Implement the agreed v1 event envelope and in-process event flow.
- Provide a common package manifest/catalog for bundled, local, and operator-installed engines.
- Migrate bundled default engines to the common protocol and process boundary after validating startup, tests, resource use, and packaging; retain the core mission/event/policy services.
- Validate engine inputs/outputs and expose engine health and failures.
- Make event lineage sufficient to navigate from an assessment to its inputs.
- Group engine-specific thresholds/settings by owning engine and validate them against the engine's declared schema.

#### 1.6 Engine Store and telemetry interoperability

- Add an operator Store page for bundled and locally declared engines, with origin, version, category, contracts, policy metadata, and enable/disable controls.
- Document local contribution, protocol conformance, and third-party package review. Do not automatically download or trust code from a remote source.
- Export the full cycle/finding set as machine-readable JSON and expose bounded Prometheus metrics for existing scrapers such as Grafana.
- Define and implement a first legacy input adapter and destination adapter only after selecting their source contract, authentication, delivery, privacy, and cardinality behavior.
- Keep paging in the existing monitoring/incident-management path until a direct paging integration is separately accepted.

#### 1.3 Linux observations

Collect a bounded set of host signals:

- CPU and load.
- Memory.
- Disk and filesystem capacity.
- Network counters.
- Process availability/resource signals where supported.
- Optional DNS resolution health for an explicitly configured target.

Every observation records its source and time. Unsupported collectors or unavailable targets produce visible unavailable/unsupported results, not zero-valued healthy measurements.

#### 1.4 Reasoning cascade

Implement only:

```text
Observation → Pressure → Health → Mission assessment
```

Use transparent, deterministic initial rules. Avoid machine learning, LLMs, inferred root cause, automated investigation, policy actions, and hidden threshold defaults.

#### 1.5 Explanation and operator output

Produce a mission assessment with:

- Status and scoped subject.
- Contributing observations and derived pressure/health results.
- Confidence label where meaningful, plus its non-calibrated semantics.
- Coverage and freshness limitations.
- Human-readable explanation linking claims to evidence.
- Explicit unknown/degraded reporting when required inputs are absent.

### Deliverables

- One runnable Go binary on a Linux host.
- Configuration, logs, and metrics sufficient to operate the prototype.
- Host observation events and engine-produced pressure/health assessments.
- Operator engine catalog/Store and per-engine configuration surfaces.
- Bulk findings API and bounded Prometheus scrape output, with explicit current-cycle/no-retention semantics.
- Mission status with an explanation tree or equivalent navigable provenance.
- Automated tests for event validation, engine behavior, missing/stale inputs, and assessment output.
- A reproducible demo using captured or synthetic fixtures; live host observation is separately identifiable.

### Exit criteria

- A single binary starts, collects configured observations, runs registered engines, and emits a mission assessment.
- An operator can trace each assessment to its source observations and see missing or stale inputs.
- Repeating the same fixture and configuration produces the documented result.
- No production action, DNS traffic interception, database, knowledge service, twin, or distributed component is required.
- Process isolation is not described as a sandbox; no plugin receives tools/scripts or production action authority.

## 7. Phase 2 — Local DNS service and traffic

### Goal

Test Perceptron reasoning against a local DNS service and controlled traffic without introducing distributed systems.

### Chapters / work packages

#### 2.1 Local DNS lab

- Select a local DNS implementation for the lab (for example, BIND or PowerDNS) and record the choice as a test-environment decision.
- Keep the lab isolated from production DNS and customer traffic.
- Create reproducible normal and failure scenarios.

#### 2.2 DNS observations and pressure

Add DNS-specific observations such as query success, latency, response codes, query rate, NXDOMAIN rate, and resolver/authoritative health as applicable. Test controlled load, NXDOMAIN-heavy traffic, service failure, and propagation delay. Treat test traffic as synthetic and label it accordingly.

#### 2.3 Entities and impact

Introduce the minimum entity references needed for the lab: service, region (even if a single local lab region), zone, and test customer where useful. Keep observed impact separate from potential impact. Demonstrate a severe local/service impact cannot be hidden by aggregate health.

#### 2.4 Investigation and knowledge prototypes

- Add read-only investigation over event history and bounded diagnostic tools.
- Preserve hypotheses, alternatives, contradictions, and investigation coverage.
- Introduce a small, reviewed knowledge corpus with provenance and applicability metadata.
- Make absent knowledge distinct from retrieval failure.
- Do not automatically execute runbooks or mutate DNS configuration.

#### 2.5 Twin design rehearsal

Exercise the twin concepts with an isolated local twin/test instance and replay or mirrored **lab** traffic. Validate response suppression and production-write isolation as design tests; do not call the lab a production twin. Production-adjacent traffic mirroring and trust renewal remain Phase 3 concerns.

### Deliverables

- Local DNS lab and traffic/scenario generator.
- DNS health and pressure assessments with event lineage.
- Basic customer/service impact assessment with scope, coverage, and explanation.
- Read-only investigation and knowledge-gap demonstration.
- Documented test evidence for the proposed twin boundary, without production connectivity.

### Exit criteria

- Normal, overload, DNS failure, and propagation-delay scenarios produce explainable assessments.
- Synthetic data is never presented as observed customer impact.
- Investigation preserves contradictory evidence, alternatives, and unknown outcomes.
- DNS response processing is not blocked by analytical consumers in the lab design.
- Knowledge retrieval failures and “no applicable knowledge” are distinguishable.

## 8. Phase 3 — Integrated DNS control and data planes

### Goal

Validate a realistic DNS operational workflow across request/change interfaces, queue, state, workers, builders, master, and authoritative serving systems. This phase tests end-to-end behavior before regional federation.

### Chapters / work packages

#### 3.1 Reference control-plane workflow

Implement or integrate a non-production DNS control plane that can accept a change, record it, process work, build/publish DNS state, and verify propagation. Keep component ownership and external system-of-record boundaries explicit.

#### 3.2 End-to-end observability and mission assessment

Trace change acceptance through queueing, processing, build, publication, serving, and observed customer/service outcomes. Preserve claims from different sources when stages disagree; do not collapse “accepted” into “served.”

#### 3.3 Production-adjacent twin

Implement the relevant data-plane and/or control-plane twin boundary from RFC-0006:

- Evaluate committed candidates under production-derived or representative conditions without exposing customers to the candidate.
- Suppress all twin responses.
- Prevent twin writes from reaching production.
- Record twin identity/version, scope, coverage, freshness, drift, fidelity limitations, and validation outcome.

Begin with non-production traffic/data. Real production traffic or data requires security, privacy, authorization, and retention approval before connection.

#### 3.4 Policy and action boundary

Implement policy evaluation and explicit outcomes (including deny, hold, require validation, escalate, and unable-to-evaluate) before any production execution path exists. Exercise approvals, exact action/scope binding, freshness, audit, emergency controls, idempotency, and post-action verification in a safe non-production environment.

Production actions remain disabled unless RFC-0010 and target-system integration requirements are accepted and all required operational/security gates are met.

### Deliverables

- Reproducible DNS control-plane/data-plane lab.
- Traceable end-to-end change and propagation timeline.
- Twin validation result and isolation tests.
- Policy decision records and tests for missing/stale inputs and scoped approvals.
- Outcome verification, failure, partial-completion, and rollback demonstrations in non-production.

### Exit criteria

- Change state is distinguishable at every lifecycle stage, with source/time/provenance.
- Candidate actions cannot bypass the policy boundary.
- Twin pass cannot authorize or execute a production change.
- Replay/retry cannot cause unapproved duplicate side effects.
- Production connectivity is separately reviewed and remains disabled by default.

## 9. Phase 4 — Regional operation and federation

### Goal

Introduce multiple regions only after a single-region workflow is observable, explainable, and safe. Design availability per workflow and mission impact; do not assume “multi-region” means every subsystem is active-active.

### Chapters / work packages

#### 4.1 Regional identity and locality

Define region identifiers and represent observation, data-origin, derivation, evaluation, authorization, and execution locality in events and decisions.

#### 4.2 Subsystem availability and federation

For APIs, databases, queues, workers, builders, serving plane, mission assurance, policy, and event processing, record availability scope, ownership, read/write federation, replication, failure behavior, recovery, and mission-impact rationale.

#### 4.3 Queue continuity and work ownership

Compare HA queue/coordination with whole-control-plane availability. Define ownership, leases/fencing, transfer, idempotency, retries, duplicate prevention, and partial work recovery before permitting cross-region execution.

#### 4.4 Partition, divergence, and global mission

Test regional loss, stale or missing observations, conflicting state, recovery/rejoin, and global mission aggregation. Preserve severe local impact and report reduced coverage rather than silently treating absent regions as healthy.

#### 4.5 Regional twin and policy behavior

Define twin placement, data locality, policy versions, authorization authority, and emergency controls across regions. Do not permit partition-time production actions until their authority and duplicate-execution semantics are explicitly accepted.

### Deliverables

- Accepted regional deployment and federation decisions.
- Workflow dependency/availability matrix.
- Cross-region work ownership and reconciliation tests.
- Regional and global mission assessments with explicit coverage/freshness.
- Partition/rejoin and duplicate-action safety evidence.

### Exit criteria

- Each supported workflow has documented behavior under loss of each required subsystem.
- Cross-region reads and writes have explicit, tested ownership and conflict semantics.
- Work cannot be concurrently or repeatedly executed without the approved safety/idempotency contract.
- Regional and global decisions retain provenance and authority context.

## 10. Phase 5 — Production readiness and bounded autonomy

### Goal

Prepare only explicitly approved capabilities for production operation. Production automation is a separate gated milestone, not an automatic consequence of completing earlier phases.

### Chapters / work packages

- Complete threat modeling, tenant isolation, identity, secrets, privacy, data residency, retention, and audit controls.
- Establish SLOs, capacity and load tests, failure injection, backup/recovery, upgrade/rollback, and on-call ownership.
- Validate monitoring of Perceptron itself, including event flow, engine health, coverage, stale inputs, and degraded mode.
- Define per-action risk class, approvals, kill switch/global pause, execution adapter, idempotency, rollback/compensation, and verification.
- Begin with a narrow, reversible, low-risk action class only if owners approve it; expand only from measured evidence and review.

### Exit criteria

- Security and operational owners explicitly approve the target deployment and action class.
- Policy enforcement, authorization freshness, auditability, and emergency controls are tested end-to-end.
- Failure, degraded, unknown, and unable-to-evaluate behavior is demonstrated.
- Rollback/recovery and human escalation paths are staffed and rehearsed.

## 11. Architecture-to-implementation crosswalk

| RFC | First implementation relevance |
|---|---|
| [RFC-0001: Event Bus](../DesignDocuments/RFC-0001-event-bus-and-event-model.md) | Phase 0 contract; Phase 1 in-process event flow; durable/distributed semantics later |
| [RFC-0002: Engine Runtime](../DesignDocuments/RFC-0002-engine-runtime.md) | Phase 0 compile-time registration decision; Phase 1 lifecycle, health, and engine contracts |
| [RFC-0003: Entity Model](../DesignDocuments/RFC-0003-operational-entity-model.md) | Minimal host/service references in Phase 1; DNS entities in Phase 2 |
| [RFC-0004: Confidence and Evidence](../DesignDocuments/RFC-0004-confidence-and-evidence.md) | Phase 1 evidence lineage and provisional confidence labels; richer evidence handling thereafter |
| [RFC-0005: Traffic Handler](../DesignDocuments/RFC-0005-traffic-handler.md) | Local DNS observations first; production traffic boundary only after safety review |
| [RFC-0006: Twin Architecture](../DesignDocuments/RFC-0006-twin-architecture.md) | Local isolation rehearsal in Phase 2; production-adjacent twin in Phase 3 |
| [RFC-0007: Mission Assurance](../DesignDocuments/RFC-0007-mission-assurance.md) | Basic single-host status in Phase 1; scoped DNS and global mission evolution later |
| [RFC-0008: Investigation](../DesignDocuments/RFC-0008-investigation-framework.md) | Read-only investigation in Phase 2; broader workflows after validation contracts |
| [RFC-0009: Knowledge Engine](../DesignDocuments/RFC-0009-knowledge-engine.md) | Small reviewed corpus and gaps in Phase 2; learning loop as outcome history matures |
| [RFC-0010: Policy Engine](../DesignDocuments/RFC-0010-policy-engine.md) | Non-production evaluation in Phase 3; no production action before explicit acceptance |
| [RFC-0011: Regional Architecture](../DesignDocuments/RFC-0011-regional-architecture-and-federation.md) | Phase 4 only, after single-region workflow and ownership semantics are proven |

Implementation sequencing intentionally differs from RFC numbering and architecture-review order.

## 12. Phase completion and change control

Each phase should produce:

- A runnable, reproducible demonstration.
- Automated tests for normal behavior, missing inputs, failure, and recovery.
- A short record of implementation decisions and any RFC assumptions.
- Updated documentation when behavior or contract semantics change.
- A review of whether the next phase's risks and dependencies are understood.

Do not promote a provisional Phase 0 decision into a platform-wide guarantee without an explicit review. If implementation evidence changes the architecture, update the affected RFCs and decision register before depending on the changed behavior in later phases.

## 13. Explicitly deferred

Until justified by phase goals and accepted contracts, do not build:

- Machine-learning or LLM-based operational decisions.
- Autonomous production remediation.
- A distributed broker or regional federation for the single-host prototype.
- A general-purpose plugin marketplace or dynamic plugin loader.
- A universal confidence formula or assumed probability calibration.
- A production-connected twin before isolation, privacy, and security approval.
- A database-backed knowledge platform before the lifecycle, ownership, and retention requirements are agreed.
