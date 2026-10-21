# Perceptron SRE: Overall Architecture

**Status:** Architecture draft for review  
**Audience:** Engineers, operators, architects, and AI coding agents working on Perceptron SRE  
**Scope:** Overall system purpose, conceptual architecture, component responsibilities, primary data and decision flows, and architecture-level open decisions. This document does not define engine algorithms, wire formats, storage schemas, deployment manifests, or a specific DNS implementation.

## 1. How to read this document

This design is intentionally explicit about certainty:

- **Established principle** — stated directly in the design notes supplied for this project.
- **Proposed architecture** — a recommended way to organize those principles for implementation; it still requires review.
- **Open decision** — the source material does not settle the choice. Implementations must not silently choose a value on behalf of the architecture.
- **Illustrative example** — demonstrates intent and is not a normative contract or performance guarantee.

The source material expresses a strong product vision, but not every implementation decision. The target direction is now a common package/catalog contract for all domain engines, with the core retaining event transport, mission aggregation, policy authority, and dashboard services. Process sandboxing, event delivery semantics, security and tenancy, persistence, and the mathematics used by engines remain open decisions rather than implied guarantees.

## 2. Executive summary

Perceptron SRE is an **autonomous operational reasoning and mission-assurance platform**. Its purpose is to reason about infrastructure and services in terms of their health, customer impact, service-level exposure, operational risk, and safe response.

It is not intended to replace the systems it operates on. For the DNS reference environment, DNS infrastructure such as `dnsdist` and BIND remains responsible for serving DNS. Perceptron SRE observes and reasons about the service and its control plane. A separate Traffic Handler concept sits at the client/service boundary to classify and regulate traffic, protect customer service, and emit metadata for analytical consumers.

The platform turns low-level observations into progressively more operationally meaningful results:

> Telemetry → pressure → health → correlation and hypotheses → customer impact and SLA risk → mission assessment → investigation and recommendation → twin or simulation validation → policy decision → permitted action or escalation

These are conceptual stages, not yet a fixed engine graph or synchronous call chain. Components are intended to have bounded responsibilities and communicate through events. Each reasoning result should be explainable and include evidence and confidence where appropriate. Policy is the authority boundary between a recommendation and a production action. Twins and simulations provide ways to test proposed changes without making production the first test environment.

## 2.1 Architecture diagram set

The [standalone architecture diagrams](diagrams/README.md) provide complementary views of system boundaries, the reasoning cascade, evidence, entity impact, DNS traffic, investigation, twins, policy, telemetry interoperability, federation, incident reporting, and engine governance. They are illustrative companions to the contracts below; they do not resolve open architecture decisions or assert that proposed capabilities are implemented.

- [System context and trust boundaries](diagrams/D-01-system-context.md)
- [Reasoning cascade and event lineage](diagrams/D-02-reasoning-cascade.md)
- [Entity and impact graph](diagrams/D-03-entity-impact-graph.md)
- [Evidence and confidence lifecycle](diagrams/D-04-evidence-confidence.md)
- [DNS traffic handler fast and slow paths](diagrams/D-05-dns-traffic-handler.md)
- [Investigation and knowledge workflow](diagrams/D-06-investigation-knowledge.md)
- [Twin validation and promotion](diagrams/D-07-twin-validation.md)
- [Policy, authorization, and action lifecycle](diagrams/D-08-policy-action-lifecycle.md)
- [Legacy telemetry interoperability](diagrams/D-09-legacy-telemetry.md)
- [Regional federation and degraded operation](diagrams/D-10-regional-federation.md)
- [Incident report time window](diagrams/D-11-incident-time-window.md)
- [Engine package, Store, and trust lifecycle](diagrams/D-12-engine-store.md)

The initial implementation direction is a **Go core binary**. The source material places platform runtime responsibilities in Go and the high-performance packet path in C/eBPF/XDP where justified. This is a language direction, not a decision that all engines must be dynamic Go plugins or that a specific packet technology must be used.

## 3. Product intent and non-goals

### 3.1 Product intent

Perceptron SRE aims to:

1. Ingest operational events and telemetry from infrastructure and service integrations.
2. Derive higher-level operational state through small, bounded reasoning components.
3. Correlate evidence and form hypotheses rather than treating a single metric as a complete explanation.
4. Prioritize customer experience, customer impact, service objectives, and mission risk over isolated component status.
5. Gather investigation evidence through operational tools and institutional knowledge.
6. Produce recommendations with supporting evidence and confidence.
7. Validate changes through a twin and/or simulation where applicable.
8. Enforce policy before allowing production actions.
9. Observe its own runtime, engines, event flow, confidence, decisions, failures, and actions.
10. Learn from outcomes, incidents, near misses, and validation results while allowing knowledge trust to decay and be re-established.

### 3.2 Non-goals

The platform is not, by itself:

- A replacement for DNS authoritative servers, `dnsdist`, load balancers, databases, or queues.
- Merely a metrics collector, dashboard, alert router, SIEM, or network-management tool.
- A claim that all operational actions can or should be autonomous.
- A single monolithic AI model that makes opaque end-to-end decisions.
- A commitment to any specific message broker, database, API framework, machine-learning algorithm, or deployment topology.

## 4. Foundational principles

The rationale and implementation guidance for these principles are collected in the [Engineering Philosophy](ENGINEERING-PHILOSOPHY.md). The principles below combine established direction with proposed refinements; unresolved choices remain open and must not be silently treated as settled.

### 4.1 Mission assurance, not monitoring as the end goal

Telemetry collection is an input. The desired output is an operational assessment: what is happening, why it may be happening, who or what is affected, what could happen next, and what response is safe.

### 4.2 Customer impact is a primary prioritization signal

Infrastructure health is relevant, but the system should determine customer, zone, record, query, region, and service-level effects when evidence permits. A component failure does not automatically imply a customer-impacting incident, and a small component fault may still be urgent if it materially harms a customer.

The exact impact model and the business rules for prioritization remain to be specified.

### 4.3 Cascaded abstraction

The system derives increasingly abstract operational state from lower-level signals. A primitive pressure estimate is not the same thing as service health; service health is not the same thing as root cause; root cause is not the same thing as customer impact or mission status.

The cascade is a conceptual architecture. It does not yet specify a universal layer count, fixed topology, or scoring formula.

### 4.4 Small responsibilities and event-oriented integration

Engines should each have a focused responsibility and explicit input and output contracts. Components should not rely on direct knowledge of each other's implementation. Event-oriented communication is the intended decoupling mechanism; the concrete transport and delivery guarantees remain open.

### 4.5 Evidence and confidence

Operational conclusions should expose their supporting evidence and degree of confidence. The system should represent uncertainty rather than converting every hypothesis into a binary fact. Confidence semantics, calibration, provenance format, and thresholds are not yet defined.

### 4.6 Policy governs production action

An engine's recommendation or confidence score is not authorization. Policy decides which classes of response are allowed under the applicable operating constraints. The exact policy model, policy ownership, and approval workflow require RFCs.

### 4.7 Production is not the first test environment

The source design establishes a twin-first principle for corrective changes and calls for simulation where relevant. A data-plane twin may receive mirrored queries while discarding its responses; a control-plane twin may replay changes without changing production. The exact fidelity, isolation, data handling, availability, and exceptions for actions that cannot be validated remain open.

### 4.8 Knowledge can age

Incidents, runbooks, FAQs, repairs, and near misses are operational knowledge, not eternal truth. Age, use, success, and validation should affect trust. The scoring model and revalidation schedule remain open.

### 4.9 Near misses are valuable

An automatically recovered event with no customer impact can still reveal risk and should be available to investigation and learning workflows.

### 4.10 The platform observes itself

Core runtime and engine health, event flow, execution time, failures, confidence, and decisions are themselves operational signals. The initial self-observation contract is to be defined by the Core RFC.

### 4.11 Pressure is an operational signal, not a synonym for utilization

The source material repeatedly distinguishes raw utilization from operational pressure. A utilization value alone may not indicate risk; pressure is interpreted from relevant signals and context, such as utilization, latency, queue growth, and capacity. Pressure is intended to inform service health, which informs impact and mission assessment.

**Proposed architecture:** make pressure a first-class reasoning concept between observation and health. The inputs, units, temporal windows, aggregation, and score semantics must be defined by the relevant engine RFCs; no universal pressure formula is established here.

### 4.12 Traffic protection prioritizes known customer service

The DNS Traffic Handler design distinguishes a known-customer lane from an unknown-traffic lane. During overload, the intent is to preserve customer service through mechanisms such as reserved capacity and reduced inspection on the known-customer path, while applying additional analysis or regulation to unknown traffic.

This is a design principle for the Traffic Handler, not an unconditional guarantee that every known request can be served or that unknown traffic is always deprioritized. Customer identity, service tiers, fairness, abuse handling, and lane policy require the Traffic Handler RFC.

### 4.13 Production safety is a staged decision process

The safety workflow is expressed as:

> Observe → Investigate → Recommend → Validate → Approve → Execute

This is a proposed production-safety sequence, not a requirement that every low-risk operation use identical stages. The specific validation and approval requirements must be defined by action class. Policy remains the authority boundary; confidence or twin success alone does not authorize an action.

### 4.14 Trust is renewed by evidence as well as reduced by age

Knowledge trust should not only decay over time. Validation outcomes—including successful twin validation, historical outcomes, and human verification—may provide evidence to renew or increase trust. A successful test of a proposed action can update knowledge about that action, but it does not make the knowledge universally safe or bypass policy.

The trust model, scope of evidence, attribution, and renewal calculation remain open.

### 4.15 A twin can provide an independent second opinion

A twin is both a validation environment and a potential independent check on a proposed change: it can help answer whether the proposal behaves as expected in a representative environment. The “second opinion” analogy describes its safety role; it does not imply that the twin is intelligent, independent by default, or sufficient to approve a production action. Independence, fidelity, and validation criteria must be established.

### 4.16 AI-native maintainability is an architectural goal

The design intends small modules, strict interfaces, low coupling, and replaceable components to support maintenance by both people and AI coding agents. This is a maintainability goal, not a reason to weaken review, testing, security, ownership, or compatibility requirements.

### 4.17 Operational reasoning model

The following is the proposed canonical conceptual model:

```text
Observations
    ↓
Pressure
    ↓
Health
    ↓
Correlation and hypotheses
    ↓
Customer / service impact
    ↓
Mission assurance
    ↓
Response: observe, investigate, recommend, validate,
          approve, execute, or escalate
```

This model captures the progression from facts to operational understanding and response. It is a conceptual cascade, not a fixed sequential pipeline. Events may enter at different stages, some stages may be inapplicable, and outputs may be recomputed as new or corrected evidence arrives.

**Open decision:** whether engine composition is represented by explicit static dependencies, dynamically routed events, or a combination. A dynamic event-driven composition is a candidate direction because it allows different domains and deployments to reuse engines without hardcoding one universal pipeline. The event bus contract, filtering, causality, cycle handling, and configuration model must be specified before selecting that design.

Confidence and evidence accompany reasoning throughout the model. Whether every engine output must carry an explicit confidence value is unresolved; the representation must not imply false precision for outputs that are categorical, deterministic, or merely observational.

### 4.18 Recommendation, authorization, and execution are distinct

A finding or confidence score is analysis. A proposed repair is a recommendation. Policy evaluation and any required human approval provide authorization. A separate execution step changes the operated system. These are distinct states and responsibilities; a recommendation must never be represented as though it has already been approved or executed.

### 4.19 Evidence includes support and contradiction

Reasoning and investigation should preserve evidence that supports a hypothesis and evidence that contradicts it. Contradictory evidence is informative and must not be discarded or treated as neutral merely because it weakens a favored explanation. The evidence model and how conflicting sources affect confidence remain open.

### 4.20 Decisions should be reconstructable

Major assessments and actions should retain provenance sufficient to reconstruct later what was known, what was concluded, what was recommended, what policy decided, what was executed, and what outcome followed. This is the operational equivalent of an incident reconstruction record, not merely a free-form explanation.

The required provenance fields, retention, access controls, replay guarantees, and degree of reproducibility remain to be specified.

### 4.21 Outcomes require verification

Issuing a repair is not proof of success. The system should verify that the intended condition improved, check for new or worsened customer and service impact, and only then determine whether an incident can be considered resolved. Verification criteria and incident-closure authority require dedicated RFCs.

### 4.22 Mission assurance applies recursively to Perceptron SRE

Perceptron SRE itself is part of the mission it protects. Its runtime, event flow, engines, knowledge integrity, twin health, and confidence behavior should be observable by mission assurance. Failures or degradation in the reasoning platform must be visible as limitations on the platform's own conclusions and actions, not silently treated as healthy operation.

The scope and independent monitoring path for this recursive assurance remain open.

### 4.23 Operational invariants

The following are proposed architecture guardrails. RFCs and implementations should preserve them or explicitly propose a reviewed change. They are not substitutes for detailed safety, security, and reliability requirements.

1. **Production is not the first test environment for a corrective change** when a relevant twin or simulation can be used.
2. **Policy is required for production execution.** Confidence, recommendation, validation, or human intent alone does not bypass the policy boundary.
3. **Analysis, recommendation, authorization, and execution are distinct** and must be represented separately.
4. **Actions and major decisions are auditable and reconstructable** from their provenance and recorded outcomes.
5. **Customer impact is a primary prioritization signal**, while component health remains supporting evidence; infrastructure status alone does not establish customer impact.
6. **Engines do not directly invoke other engines**; coordination uses explicit platform contracts.
7. **Missing or contradictory evidence is not silently converted into a healthy result, certainty, or authorization.**
8. **DNS analytical consumers do not block the DNS response fast path.**

The handling of cases where a preferred safety step is unavailable must be defined by policy and action class; this list does not imply that validation can always be performed.

### 4.24 Reliability and evidence invariants

The following guardrails apply to implementation and review:

1. **No default green:** a favorable mission state requires valid, sufficiently fresh evidence for all required inputs in the declared scope. A running process or missing alert is not proof of health.
2. **Unknown is a valid outcome:** missing, stale, unavailable, contradictory, malformed, or insufficient evidence remains explicit. Invalid output or runtime failure must be surfaced, not converted to a success-shaped result.
3. **Determinism is declared and scoped:** deterministic engines must reproduce their decision and explanation content for the same normalized inputs, engine version, and configuration. Variable evaluation metadata (for example, timestamps and generated IDs) is not itself a changed decision. Non-deterministic behavior and its scope must be declared.
4. **Tests include behavior and failure:** tests cover normal and boundary behavior plus missing, stale, contradictory, invalid, and unavailable inputs, and relevant failures. Synthetic fixtures enable reproducible replay; simulation is required before production exposure where applicable.
5. **Security is reliability evidence:** security-relevant conditions should eventually be assessed with the same evidence, scope, freshness, coverage, and uncertainty semantics as other operational domains. This is a direction for future engines, not a current prototype capability.
6. **Incidents can inform regression tests, not auto-promote knowledge:** incident-derived scenarios or knowledge require preserved provenance and review; prior success alone does not establish current applicability or authorization.

The [Engineering Philosophy](ENGINEERING-PHILOSOPHY.md) explains these invariants. They do not imply certification to aerospace, medical, or industrial standards; any such claim would require a separate applicable assurance process.

## 5. System context and boundaries

### 5.1 Logical context

```text
                         ┌─────────────────────────────────────┐
                         │          Perceptron SRE              │
                         │                                     │
                         │  Core runtime + event contracts     │
                         │  Reasoning engines + investigation  │
                         │  Knowledge + twin/simulation        │
                         │  Policy + mission assurance         │
                         └───────────────┬─────────────────────┘
                                         │ events, evidence,
                                         │ recommendations, policy
                                         ▼
Clients ──► Traffic Handler ──► DNS service infrastructure
                 │                    (e.g. load balancer,
                 │                     dnsdist, BIND)
                 ├──► billing / security / analytics consumers
                 ├──► optional data-plane twin
                 └──► metadata and operational event sources

DNS control-plane changes ──► API / queue / data stores /
                               DNS builder / authoritative fleet
                       │
                       ├──► control-plane twin
                       └──► Perceptron SRE observation and analysis
```

This diagram expresses conceptual roles and data relationships. It does not assert that all boxes are deployed together, that all traffic passes through a new Perceptron-owned network hop, or that the event path is synchronous.

### 5.2 DNS service boundary

For DNS, the data plane answers DNS questions. The control plane manages customer changes, zones, records, configuration, and propagation. Perceptron SRE observes and reasons across both.

The Traffic Handler is a DNS-specific service boundary described in the source material:

- It identifies known customers and service tiers where configured.
- It classifies query characteristics and extracts request metadata.
- It supports pressure measurement, customer-aware regulation, and protection.
- It may separate traffic into a known-customer lane and an unknown-traffic lane, with different service and inspection policies intended to preserve customer service during overload.
- It distributes metadata to analytical or operational consumers.
- It preserves a fast path to DNS and does not wait for slow analysis such as customer-impact reasoning, knowledge search, or simulation.
- It may mirror traffic to a data-plane twin whose response is discarded.

The Traffic Handler is not BIND, a DNS resolver/authoritative server, or merely a firewall or load balancer. The exact ownership boundary—whether it is a Perceptron subsystem or a separately deployed DNS integration—is an open decision. Its intended functional position is between clients and the DNS service path. Customer-lane behavior, including identity confidence, reserved capacity, fairness, unknown-traffic controls, and failure behavior, must be defined and validated rather than assumed by this architecture.

### 5.3 Control-plane boundary

The DNS control-plane design describes a workflow from API request through queue, databases, DNS builder, master DNS, and propagation to authoritative servers. It also describes the queue as an authoritative operational ledger whose changes can be replayed and investigated.

This is a DNS environment design, not a commitment that Perceptron SRE itself owns the DNS API, queue, database cluster, or DNS builder. The integration contract and ownership of change execution must be defined in the DNS integration RFC.

## 6. Logical architecture

The following components are architectural responsibilities. They need not map one-to-one to processes, packages, or deployable services.

### 6.1 Core runtime

The core is the small Go foundation that starts and manages the Perceptron SRE runtime. Its responsibilities, based on the source material, include:

- Configuration and startup.
- Engine lifecycle: initialization, execution, health reporting, and shutdown.
- Event publication and subscription.
- Structured logging and error reporting.
- Metrics collection and reporting.
- Policy evaluation and decision recording.
- Storage and reporting capabilities or their interfaces.
- Core self-observation.

The source material's initial core completion criteria are one binary starts, events flow, engines load, and logs and metrics work. The precise minimum viable core milestone and whether storage, policy, or reporting are fully implemented in that milestone remain open.

**Proposed architecture:** define stable interfaces and contracts at the core boundary before adding domain engines. Keep integrations and engine-specific business logic out of the core. Avoid making core changes necessary for each new engine.

### 6.2 Event and metadata flow

The event system is the platform's integration and reasoning substrate. Conceptually it carries observations, derived states, hypotheses, investigation results, validation results, recommendations, policy decisions, actions, and outcomes.

The Traffic Handler's metadata stream is a related but distinct concern: it emits compact request metadata to consumers so they do not each need to duplicate or inspect the full packet. This traffic metadata path must not make DNS response processing wait for analytical consumers.

No event envelope, schema registry, ordering model, persistence/replay behavior, delivery guarantee, backpressure behavior, or transport has yet been chosen. The Event Bus RFC must specify these before distributed or safety-sensitive workflows depend on them.

**Proposed architecture — first-class operational entities:** customer, zone, record, region, and incident recur across the DNS and mission-assurance concepts and should be identifiable entities in the domain model. Events and derived results should be able to refer to the relevant entities consistently. This does not prescribe a shared database schema or require every deployment to own those entities. Identity, lifecycle, relationship, tenancy, and reference semantics belong in a domain-model RFC.

**Open decision — operational truth:** related systems can report conflicting states. For example, a change queue or database may show an update as accepted while a DNS builder, master, or authoritative server does not show it as applied; a twin may also disagree with production. The platform must preserve each source's claim, time, provenance, and scope rather than silently declaring one system authoritative for every question. The system-of-record and reconciliation rules may differ by entity and lifecycle stage and must be established in integration RFCs.

### 6.3 Engine runtime and engine contracts

An engine is a bounded unit of processing that accepts defined inputs and may produce defined outputs and errors. The language-and-engines notes give this illustrative Go interface:

```go
type Engine interface {
    Name() string
    Initialize() error
    Process(Event) ([]Event, error)
    Shutdown() error
}
```

This is a candidate interface, not an approved API. It does not yet express context cancellation, health, concurrency, event metadata, configuration, retries, idempotency, or resource limits.

The design notes describe engines as small, independently testable components and also refer to them as Go plugins. They do not settle whether "plugin" means a Go package loaded into the core process, a separately managed process/service, a configuration-defined engine, or a combination. This distinction is an architecture-level open decision.

**Proposed engine ownership rules:**

- An engine has a focused responsibility and explicit input and output contracts.
- An engine consumes and emits events through platform-defined interfaces; it does not directly invoke another engine.
- An engine does not own shared mutable state. It may keep private local state or use an explicitly assigned storage capability, but that state must not become an undocumented communication channel or undermine replay and observability.
- Failures, lifecycle, health, resource limits, and state recovery must be visible at the runtime boundary.

These rules are architectural proposals from the additional concepts, not finalized runtime guarantees. The Engine Runtime RFC must define how they apply to in-process and out-of-process engines.

**Proposed engine category — meta engines:** in addition to engines that reason about the operated environment, the architecture may classify engines that assess Perceptron SRE itself as meta engines. Examples include event-bus health, engine health, knowledge health/trust, and mission-assurance health. This is a classification for ownership and observability; it does not require separate runtime infrastructure or imply that every named example is a separate plugin.

### 6.4 Reasoning cascade

The cascade groups reasoning by increasing abstraction. Pressure is the proposed primitive derived operational signal after raw observations; it feeds health, which feeds impact and mission assessment. The names below are conceptual categories assembled from the source material, not a finalized dependency graph.

| Conceptual stage | Example inputs | Example derived outputs |
|---|---|---|
| Observation | CPU, memory, latency, QPS, queue depth, IXFR time, zone/record counts | Raw metric or event |
| Pressure | Utilization, latency trend, queue growth, cache misses, capacity context | CPU, DNS, network, filesystem, or traffic pressure |
| Service health | Pressure and service signals | DNS, queue, database, or platform health |
| Correlation | Health signals and temporal/contextual evidence | Root-cause candidates, propagation risk, failure prediction |
| Impact | Affected customers, zones, records, queries, regions, service objectives | Customer impact, blast radius, SLA risk |
| Mission assurance | Impact, SLA risk, platform health, confidence, business impact | Mission state and operational disposition |
| Investigation and recommendation | Hypotheses, toolbox evidence, trusted knowledge, history | Investigation findings and recommended response |
| Validation | Proposed response, twin observations, simulation inputs | Validation result and predicted impact |
| Policy and action | Recommendation, evidence, confidence, validation, applicable policy | Permit, deny, require approval, observe, or escalate |
| Outcome and learning | Actual result, customer impact, validation, operator feedback | Updated incident history and knowledge trust evidence |

The example mission states GREEN, YELLOW, ORANGE, and RED appear in the design notes. Mission assurance should be understood as an **operational disposition**, not merely a health score: a status should help determine whether to continue operations, observe, prepare intervention, intervene, or escalate. Those example mappings are illustrative only; exact definitions, precedence, transition rules, authority, and relationship to actions are not defined.

The cascade should not be interpreted as requiring every event to pass through every stage, nor as requiring one centralized model to compute all stages. Dynamic event routing is a candidate, not an approved decision; the cascade may use configured dependencies, event subscriptions, or a combination once ARCH-013 is resolved.

Every reasoning result should preserve its supporting evidence and uncertainty. Requiring a numeric confidence field on every engine output is a separate open decision (ARCH-014); raw observations and deterministic outputs may need a different representation from probabilistic hypotheses.

**Proposed evidence categories** include observed metrics and events, toolbox results, historical knowledge, twin validation, simulation, human verification, and policy constraints. These categories describe provenance, not equal evidentiary weight: a policy constraint governs permitted action but is not evidence that a root-cause hypothesis is factually true. Confidence must be explainable in terms of the evidence considered and its limitations.

Evidence should distinguish at least:

- **Supporting evidence:** raises support for a hypothesis or expected outcome.
- **Contradictory evidence:** weakens or conflicts with a hypothesis or expected outcome.
- **Unavailable or inconclusive evidence:** was absent, stale, incomplete, or unable to discriminate between explanations.

These are proposed concepts; evidence identity, quality, freshness, independence, and influence on confidence require a dedicated RFC.

**Open decision — observed versus potential impact:** observed impact is grounded in measurements or confirmed reports (for example, failed customer queries). Potential impact is estimated from a modeled blast radius or hypothetical failure (for example, customers that could be affected if a region fails). They must be distinguishable in results and reporting; a calculated exposure must not be presented as an observed outage. The calculation method, evidence thresholds, and relationship between the two remain open.

**Open decision — determinism:** some engines may be expected to return the same result for the same version, configuration, and inputs, while others (such as ranking or probabilistic analysis) may not. Each engine contract should declare its determinism and relevant version/configuration context so testing, replay, and audit do not imply reproducibility where it is not guaranteed. The required declarations and replay semantics are unresolved (ARCH-020).

### 6.5 Toolbox and investigation

The toolbox provides operational evidence-gathering functions, with examples spanning CPU, memory, disk, processes, DNS validation, networking, databases, and queues. A tool result is evidence for reasoning; it is not automatically a remediation.

The investigation framework coordinates a progression such as detection → evidence collection → toolbox execution → knowledge search → root-cause hypothesis. The exact orchestration model, permissions for tools, deadlines, resource limits, and output contracts are not yet specified.

### 6.6 Knowledge system

The knowledge system stores incidents, FAQs, runbooks, near misses, and remediation history. It should make age, use, success, twin validation, and human validation available as evidence relevant to trust. Knowledge retrieval should inform investigation and recommendation without making a stored runbook unconditionally authoritative.

The storage model, retrieval method, trust calculation, retention policy, and review workflow remain open.

### 6.7 Twin system

The source design distinguishes two forms:

- **Data-plane twin:** operates adjacent to production and may receive a copy of live production queries; production remains authoritative and twin responses are discarded and never returned to customers.
- **Control-plane twin:** receives or replays committed changes against isolated, production-derived state but cannot change production.

Development, integration, test, QA, and staging environments validate changes before deployment and are not replaced by the twin. The twin's distinguishing role is to evaluate a committed candidate against production-derived traffic, data, behavior, or change streams while remaining isolated from customer service and production writes. It exists alongside production, which continues to serve customers; it is not merely another pre-production stage. A twin result is evidence, not by itself production authorization. Fidelity, state synchronization, test isolation, sensitive-data handling, validation criteria, and the definition of "twin unavailable" behavior must be decided in a dedicated RFC.

### 6.8 Simulation system

Simulation answers "what if?" questions using production-derived telemetry, historical events, or proposed scenarios without affecting production. Examples in the source material include traffic growth, region failure, queue interruption, rate-limit changes, hardware loss, and BIND upgrades.

Simulation and twin validation are related but different: a twin evaluates a proposed change against a representative environment; a simulation estimates outcomes under a modeled scenario. Exact boundaries and interfaces remain open.

As a potential confidence and knowledge input, a successful twin validation may contribute evidence to renew trust in a runbook, remediation, or hypothesis. Trust renewal must be scoped to what was actually tested and observed; it must not imply universal correctness or bypass policy.

### 6.9 Policy and remediation

The policy engine is a safety governor between intelligence and action. The source material describes response levels including observe, investigate, recommend, repair, and escalate. A remediation component may propose or execute a change only through the appropriate policy-controlled workflow.

The architecture does not yet decide:

- Whether the initial release may execute production actions.
- Whether actions require human approval, twin validation, both, or neither for particular classes.
- How authorization, identity, audit, rollback, and emergency stop are implemented.
- How policies are authored, versioned, tested, and distributed.

These are blockers to autonomous production execution and must be resolved before such execution is implemented.

### 6.10 Mission assurance and reporting

Mission assurance combines operational, customer, SLA, confidence, blast-radius, and business-impact evidence into a high-level mission assessment. The output should be concise enough for operators while retaining access to explanation, evidence, and provenance.

Mission assurance is the top-level operational disposition, not merely a health score and not a replacement for detailed telemetry or investigation records. It should communicate the recommended operational posture and its rationale. State thresholds, aggregation across regions, reporting formats, decision authority, and escalation behavior remain to be specified.

## 7. End-to-end operating flows

### 7.1 Observe and reason

1. An integration or service emits an observation.
2. The core receives or routes the observation using the agreed event contract.
3. Relevant pressure and health engines derive operational states.
4. Correlation and impact engines enrich the state where inputs are available.
5. Mission assurance produces a status with evidence and confidence.
6. The platform records and exposes the results for operators and downstream workflows.

This flow does not assume that every stage executes synchronously or that absent inputs can be treated as zero or healthy.

### 7.2 Investigate and recommend

1. A signal or policy initiates investigation.
2. Investigation requests bounded evidence from applicable toolbox components.
3. Results are correlated with recent events and knowledge.
4. The system records candidate causes, supporting and conflicting evidence, and confidence.
5. A recommendation is produced with expected effects and known uncertainty.
6. The recommendation is presented to policy and/or a human review workflow.

The design material gives examples but does not establish that investigation is always automatic, that a specific tool must run, or that recommendations are executable commands.

### 7.3 Validate and act

1. A proposed change is described independently from its execution.
2. Where applicable, it is tested in a twin and/or evaluated in simulation.
3. Validation evidence and predicted impact are recorded.
4. Policy considers the recommendation, evidence, confidence, validation status, and applicable constraints.
5. Any required human or organizational approval is obtained; approval requirements are determined by policy and action class.
6. The decision and any approval are recorded as permitted, denied, held, or escalated.
7. If permitted, the change is executed through the owning system's integration, not by bypassing that system.
8. The platform verifies whether the intended condition improved.
9. It checks for new or worsened customer and service impact, not only whether the targeted metric changed.
10. The outcome, including partial success or regression, is recorded and informs mission status, incident handling, and knowledge trust.
11. Incident closure occurs only under an explicitly defined verification and ownership process.

This is the proposed safety progression Observe → Investigate → Recommend → Validate → Approve → Execute → Verify Outcome. The allowed decision vocabulary, which stages are required for each action class, and the exact enforcement point are open for the Policy and Remediation RFCs.

### 7.4 DNS propagation example

The following is illustrative, based on the supplied IXFR scenario:

1. A DNS control-plane change is accepted and recorded by the DNS environment's change workflow.
2. Propagation telemetry indicates a delay beyond the relevant service objective.
3. Propagation and health reasoning identifies elevated risk.
4. Impact reasoning determines which customers, zones, records, and queries are affected, if that evidence is available.
5. Investigation gathers region, transfer-queue, and service evidence and searches incident history.
6. A candidate response, such as restarting an IXFR worker, is proposed.
7. A control-plane or service twin validates the proposal where applicable.
8. Policy determines whether the change may proceed and whether human authorization is required.
9. The change is issued through the DNS system's authorized control plane.
10. Propagation, customer impact, and mission state are observed afterward; the result is recorded.

The example does not define an actual restart mechanism, target threshold, customer impact formula, or autonomy threshold.

## 8. Runtime, languages, and deployment model

### 8.1 Language direction

The supplied language design establishes a preferred division of concerns:

- **Go:** core platform runtime, event handling, engine runtime, metrics, logging, policy, storage, APIs, twin/simulation frameworks, and higher-level reasoning.
- **C/eBPF/XDP:** possible packet-path components where low-level performance and deterministic processing justify them, including classification, metadata extraction, pressure sampling, and simple fast-path heuristics.
- **Python:** optional for reporting, static analysis, and experiments; it is not identified as a core runtime requirement.

The governing principle is to use the lowest-level language necessary, not the lowest-level language possible. Specific packet technology and boundaries between C and Go require measurement and an integration RFC.

### 8.2 Core binary and plugins

The implementation intent is to create the core binary before adding the operational engines. The core establishes runtime capabilities and stable contracts; later engines provide domain-specific behavior.

All domain-specific reasoning modules—including first-party defaults—are engines and should have consistent package metadata, per-engine settings, event contracts, enablement, and policy declarations. The target is to place engine packages under `main/external` and use a common process protocol. The current v0.1 runtime remains transitional: bundled engines execute in-process while configured external engines execute as subprocesses.

An engine process boundary is failure isolation, not an OS sandbox. Engines run with the service identity's privileges until a separate sandbox and capability broker are implemented. Policy metadata is declarative and never grants tools or action authority.

The core includes services that reason across the engine graph or govern the platform: event bus/transport, mission aggregation, policy authority, configuration/catalog API, and dashboard. These are not categorized as installable domain engines, even though core services may consume engine outputs.

### 8.3 Legacy telemetry interoperability

Perceptron SRE should integrate with existing monitoring rather than replace it. Versioned input adapters translate legacy telemetry into canonical events; output adapters export complete findings and bounded metrics back to the existing destination. The initial implementation exposes bulk JSON findings and a Prometheus-compatible scrape endpoint; it does not yet provide generic legacy ingestion, remote write, durable history, or a vendor-specific integration. Grafana and paging systems may consume exported metrics through their existing scrape/alert paths.

Adapters preserve source, scope, time, freshness, and provenance. Unknown/unavailable input remains explicit, and export failures/cardinality limits must be visible. See [RFC-0012](RFC-0012-engine-store-and-telemetry-interoperability.md).

### 8.4 Deployment locations

The design notes envision Perceptron SRE functions deployed near several operational domains:

- Traffic-handler deployment for customer detection, DDoS signals, pressure, and traffic analytics.
- Control-plane deployment for queue health, database divergence, zone generation, and propagation.
- Data-plane deployment for latency, query success, DNS health, and customer impact.
- Global mission-assurance deployment for cross-region analysis, business impact, mission state, twin validation, and simulation.

These are functional placement concepts, not an agreed topology. Whether components are co-located, regionally replicated, centrally coordinated, or deployed as one binary is open.

The DNS notes envision three active regions. This is a target environment concept, not a validated availability design or a requirement for the first Perceptron SRE release.

Regional operation does not imply that the entire control plane must be active-active. The availability boundary may differ among work queues, APIs, databases, workers, builders, and serving components; whether an HA queue is sufficient for particular workflows or a broader HA control plane is required remains open under RFC-0011.

## 9. Cross-cutting requirements to carry into RFCs

These are architectural concerns, not yet complete implementation requirements.

### 9.1 Fast path and isolation

DNS query response must not wait for asynchronous analytical work. If a Traffic Handler is implemented, its synchronous work must be bounded and its behavior under overload must be specified. The source material expresses a microsecond-scale aspiration for the DNS path but does not define a measured threshold or benchmark conditions.

### 9.2 Failure behavior

Every component boundary needs explicit behavior for missing, malformed, stale, duplicated, delayed, or unavailable inputs. No component should silently convert missing evidence into a healthy result or an authorized action. Exact error, retry, degradation, and dead-letter semantics belong in the relevant RFCs.

### 9.3 Explainability and audit

Reasoning results and action decisions need traceable inputs, producing component identity/version, time context, evidence considered (including contradictory or unavailable evidence), confidence where applicable, policy outcome, and resulting action where applicable. Major decisions should have provenance sufficient to reconstruct the incident or decision sequence later, including what was known at the time rather than only the final state.

The event schema, audit-retention requirements, reproducibility guarantees, and access controls have not been defined. Reconstructability does not necessarily mean deterministic re-execution; that depends on the engine's declared determinism and the availability of its original inputs and configuration.

### 9.4 Security and privacy

The source material does not define authentication, authorization, tenant isolation, secrets handling, encryption, data retention, or privacy requirements for mirrored DNS traffic and customer metadata. These must be resolved before production integrations or twins handle real customer data.

### 9.5 Configuration and compatibility

The project intends to add engines without rewriting the core. That requires explicit versioning and compatibility rules for engine configuration, event schemas, and runtime interfaces. Those rules are not yet specified.

### 9.6 Operability

At minimum, core and engine health, execution time, confidence, failures, decisions, and event flow are identified as important operational signals. Required logs, metrics, reports, dashboards, and alerting behavior need definition in Core and Observability RFCs.

## 10. Implementation progression

The source material calls for the Perceptron Core to precede DNS-specific implementation and lays out a broader progression. The following captures that intent without asserting detailed sprint sequencing:

1. **Core binary:** startup, configuration, structured logging, metrics, error handling, event flow, and engine lifecycle.
2. **Engine contracts and first cascade:** establish event and engine contracts; implement representative raw-signal-to-pressure-to-health reasoning without requiring DNS.
3. **Core reasoning capabilities:** add correlation, customer impact, mission assurance, investigation, and confidence with explicit evidence.
4. **Toolbox and knowledge:** provide bounded evidence gathering and trust-aware operational knowledge.
5. **Twin and simulation:** establish safety validation before production automation.
6. **DNS reference integration:** connect Traffic Handler, control plane, data plane, DNS engines, and twin workflows.
7. **Autonomous operations:** expand policy-governed action only after safety behavior, validation, auditability, and operational acceptance are defined.

The exact acceptance criteria and dependency order must be made testable in RFCs. In particular, "configuration without code changes" and "50 Toolbox Modules" are goals in the source notes, not requirements for the first core release.

## 11. RFC roadmap

The knowledge base should define the following RFCs. The list is a proposed decomposition; RFC ordering and ownership remain to be agreed.

### Foundation

1. Core binary, configuration, lifecycle, and self-observation.
2. [RFC-0001: Event model and event bus](RFC-0001-event-bus-and-event-model.md) — delivery semantics, replay, ordering, and backpressure.
3. [RFC-0002: Engine runtime](RFC-0002-engine-runtime.md) — engine contract, registration/loading, execution isolation, configuration, health, and compatibility.
4. Logging, metrics, error model, reporting, and audit records.
5. Storage abstraction, persistence boundaries, retention, and recovery.
6. [RFC-0010: Policy engine](RFC-0010-policy-engine.md) — action classes, approvals, authorization, audit, and emergency controls.

### Reasoning and operations

7. [RFC-0004: Confidence and evidence](RFC-0004-confidence-and-evidence.md) and cascade model — typed outputs, evidence provenance, confidence semantics, and engine composition.
8. [RFC-0003: Operational entity model](RFC-0003-operational-entity-model.md) — operational identities, state, relationships, provenance, and time.
9. Operational truth, source authority, lifecycle state, reconciliation, and conflicting observations.
10. Observed versus potential impact and blast-radius model.
11. [RFC-0007: Mission assurance](RFC-0007-mission-assurance.md) — mission states, operational disposition, and aggregation semantics.
12. [RFC-0008: Investigation framework](RFC-0008-investigation-framework.md) — investigation orchestration, evidence quality, contradictory evidence, and toolbox contract.
13. [RFC-0009: Knowledge engine](RFC-0009-knowledge-engine.md) — trust decay and renewal, validation, and near-miss handling.
14. Remediation proposal, authorization/execution boundary, outcome verification, incident closure, and rollback.
15. Engine determinism declarations, replay expectations, and reproducibility.
16. Meta-engine classification and recursive platform mission assurance.

### Safety and integration

17. [RFC-0006: Twin architecture](RFC-0006-twin-architecture.md) — data-plane and control-plane twins, isolation, fidelity, and validation results.
18. Simulation system and scenario model.
19. [RFC-0005: Traffic Handler](RFC-0005-traffic-handler.md) — packet/metadata boundary, customer lanes, pressure regulation, and fast-path safety.
20. DNS reference environment and integration contracts for control plane, data plane, propagation, and regional operation.
21. Security, tenancy, privacy, and threat model across core, plugins, integrations, and mirrored traffic.
22. Deployment topology, upgrades, high availability, and disaster recovery.
23. [RFC-0011: Regional Architecture and Federation](RFC-0011-regional-architecture-and-federation.md) — regional identity, event locality, routing, replication, active-active operation, and mission aggregation.

RFCs must link to this architecture and identify any proposed change to its principles or component boundaries.

## 12. Open architecture decisions

These items are intentionally unresolved. They should be worked through before the corresponding implementation becomes dependent on them.

| ID | Decision | Why it matters | Candidate direction / known constraint |
|---|---|---|---|
| ARCH-001 | What package protocol, process sandbox, and migration path make every domain engine (including bundled defaults) a consistent plugin? | Determines isolation, deployment, failure containment, loading, language interoperability, and compatibility. | Target direction is common packages under `main/external` using the process protocol. Current built-in engines remain in-process; subprocess execution alone is not a security sandbox. |
| ARCH-002 | What is the first event transport and what delivery semantics are required? | Determines core implementation, event durability, replay, ordering, and distributed operation. | Event-oriented communication is established; broker and delivery guarantees are not. |
| ARCH-003 | Is the first core milestone observe-only, recommendation-capable, or action-capable? | Determines policy, security, audit, twin, and control-plane integration scope. | Safety requires policy above action; initial authority is not specified. |
| ARCH-004 | Is the Traffic Handler part of Perceptron SRE, a separate service, or an integration-owned component? | Determines ownership, deployment, operational responsibility, and the network boundary. | Its functional role and DNS fast-path constraints are described; organizational/runtime ownership is not. |
| ARCH-005 | What exact responsibilities are inside the core binary, and which are separate services? | Prevents a monolith or premature distribution and defines the first executable milestone. | One Go core binary is the starting point; component-to-process mapping is open. |
| ARCH-006 | What are the event envelope, schema evolution, identity, time, provenance, and correlation conventions? | Required for interoperable engines and explainable reasoning. | No wire format or schema rules are established. |
| ARCH-007 | What are the confidence and mission-state semantics, calibration requirements, and thresholds? | Required to interpret scores and safely govern decisions. | Confidence and GREEN/YELLOW/ORANGE/RED concepts are established; their meanings and thresholds are not. |
| ARCH-008 | What constitutes an acceptable twin or simulation result, and what happens when validation is impossible or unavailable? | Required to safely distinguish tested changes from untested changes. | Twin-first and simulation principles are established; fidelity and exceptions are not. |
| ARCH-009 | What security, tenant-isolation, privacy, and retention requirements apply? | Production telemetry and customer metadata can be sensitive and actions are privileged. | No requirements are specified in the source material. |
| ARCH-010 | What regional deployment and availability model is required? | Affects event routing, state ownership, global mission assessment, and recovery. | Three active DNS regions are envisioned; Perceptron topology is not decided. |
| ARCH-011 | How are production changes authorized and executed, and which external system owns execution? | Avoids bypassing existing control planes and determines audit/rollback behavior. | DNS control-plane workflow is described; integration authority is not. |
| ARCH-012 | Which packet-path work actually needs C/eBPF/XDP, based on benchmarks and operational constraints? | Avoids premature complexity and makes fast-path performance measurable. | C/eBPF/XDP are candidates for the packet path; no technology choice or performance SLO is fixed. |
| ARCH-013 | Is the cascade a statically declared dependency graph, dynamically composed through event routing, or a combination? | Determines how engines are configured, discovered, routed, tested, and prevented from forming cycles. | A dynamic event-driven cascade is a candidate to support multiple domains without hardcoding one pipeline; routing and cycle semantics are unspecified. |
| ARCH-014 | Must every engine output carry an explicit confidence value? | A universal field can improve consistency but may imply false precision for raw observations or deterministic results. | Evidence and uncertainty should be represented; the required shape and applicability of confidence are undecided. |
| ARCH-015 | Are customer, zone, record, region, and incident first-class domain entities with shared identity/reference semantics? | Cross-engine impact reasoning requires consistent references, but entity ownership and schema affect integrations and storage. | Treat these as first-class conceptual entities; canonical identifiers and persistence ownership are not decided. |
| ARCH-016 | What engine state may be private/local, and what restrictions enforce the no-shared-state principle? | Defines isolation, replay, persistence, recovery, and whether engines can communicate indirectly. | Proposed rule: no shared mutable state and no direct engine-to-engine invocation; local state may be permitted through explicit contracts. |
| ARCH-017 | How does the platform establish operational truth when systems disagree? | Investigations and actions depend on distinguishing accepted, generated, propagated, served, mirrored, and observed state. | Preserve claims with source, time, and scope; authoritative source and reconciliation rules may differ by entity and lifecycle stage. |
| ARCH-018 | Is customer impact observed, calculated, or both? | Prevents modeled exposure from being reported as confirmed customer harm and informs mission prioritization. | Represent observed and potential impact distinctly; evidence threshold and calculation method are undecided. |
| ARCH-019 | What evidence types and qualities support confidence, and how is contradictory evidence represented? | Confidence otherwise becomes inconsistent or unexplainable across engines. | Candidate evidence includes telemetry, tools, knowledge, twin, simulation, human verification, and policy context; their distinct roles and weighting are not specified. |
| ARCH-020 | Must engines be deterministic, and what reproducibility guarantees apply? | Affects testing, event replay, audit reconstruction, and engine suitability for safety-sensitive decisions. | Different engine classes may have different determinism; declare behavior and version/configuration context, with contract details to be decided. |
| ARCH-021 | Is confidence one universal measure or a set of distinct confidence domains? | Observation, hypothesis, impact, prediction, action, and outcome claims may have different meanings and evidence; one scalar could obscure those differences. | Preserve claim-specific semantics; decide the initial domains and whether/how they compose in RFC-0004. |
| ARCH-022 | What regional identity, event locality, routing, replication, and federation model is required? | Active regions need unambiguous observations and derived assessments; bus topology affects partitioning, duplication, consistency, and global mission assurance. | RFC-0001 should carry locality/provenance without selecting topology. Detailed regional buses/federation and active-active behavior belong in RFC-0011. |
| ARCH-023 | How are engine capabilities declared, granted, enforced, revoked, and audited? | Engines may need different access to events, tools, knowledge, twins, and action interfaces; capability mistakes can create safety and security vulnerabilities. | Engines should self-describe requested capabilities; declaration is not a grant, and least privilege plus policy enforcement is required. Vocabulary and mechanism are unresolved in RFC-0002. |
| ARCH-024 | How are coverage, confidence, and engine availability represented when reasoning chains are incomplete? | Prevents missing upstream outputs from being treated as healthy and lets downstream engines distinguish optional gaps from invalid/incomplete assessments. | Keep health/availability, coverage, and confidence distinct; define required/optional dependency semantics and propagation in RFC-0002 and RFC-0004. |
| ARCH-025 | Should services be first-class operational entities, and what defines their boundaries? | Pressure and component state affect a service, which in turn affects customers, zones, and missions; impact reasoning needs stable service references and dependencies. | Service is a strong candidate with owner, type, dependencies, regions, and SLA references; canonical boundaries and ownership remain unresolved in RFC-0003. |
| ARCH-026 | Which relationships are authoritative, observed, or inferred, and who owns each relationship type? | Impact, blast-radius, investigation, and confidence depend on safe traversal of time-bound relationships with known provenance. | Preserve direction, time, source, freshness, and inference confidence where applicable; relationship authority and semantics remain unresolved in RFC-0003. |
| ARCH-027 | How are services, components, and deployments modeled: as separate entity classes or related forms of one entity? | Incidents and tools often target components or deployments, while impact reasoning traverses from them through services and regions to customers. | Preserve the conceptual distinctions (service, component, deployment); exact entity classes, identifiers, and relationships remain unresolved in RFC-0003. |
| ARCH-028 | Can downstream engines consume confidence results as evidence, or must they reason from the underlying evidence? | Confidence can be reused by mission assurance and other downstream reasoning, but unqualified reuse may hide the original evidence, semantics, scope, age, or limitations. | Confidence results may be consumed as qualified derived evidence only when their semantics, scope, time context, and provenance are preserved; the required guarantees and consumer obligations remain unresolved in RFC-0004. |
| ARCH-029 | How are customer identity, identity confidence, and Traffic Handler lane assignment represented, validated, and refreshed? | Incorrect or stale attribution can undermine fairness, customer protection, security controls, and auditability. | Distinguish high-confidence attribution, low-confidence attribution, and unattributed traffic; identity evidence, thresholds, lane transitions, and policy consequences remain unresolved in RFC-0005. Unattributed traffic is not inherently malicious. |
| ARCH-030 | Which degraded Traffic Handler modes exist, and what functionality is sacrificed before customer DNS service? | Dependency or capacity failures require explicit trade-offs among service continuity, security, metadata, and regulation; implicit fallback behavior is unsafe. | Define observable modes and their retained capabilities, transitions, and policy ownership; do not imply fail-open/fail-closed defaults. Detailed behavior remains unresolved in RFC-0005. |
| ARCH-031 | How are twin drift, coverage, freshness, and synchronization represented and incorporated into validation confidence? | A healthy or passing twin may still be stale, unrepresentative, or materially different from the production target, making results unsafe to generalize. | Preserve twin health, validation outcome, coverage, confidence, freshness, and drift as distinct but related dimensions; required fields, thresholds, and composition remain unresolved in RFC-0006 and RFC-0004. |
| ARCH-032 | What validation coverage is sufficient to promote a change to each subsequent stage? | A pass over a narrow target, region, dataset, or traffic sample may not support a broader or higher-risk production stage. | Promotion gates should consider the proposed next-stage scope and documented validation coverage; required thresholds, owners, and exceptions remain unresolved in RFC-0006 and RFC-0010. |
| ARCH-033 | Should mission assessment expose confidence, coverage, and freshness as separate outputs? | A categorical mission status alone can conceal weak evidence, incomplete scope, or an assessment too old to represent current conditions. | Candidate output keeps mission status, disposition, confidence, coverage, freshness, and explanation distinct; exact semantics and composition remain unresolved in RFC-0007 and RFC-0004. |
| ARCH-034 | Should UNKNOWN be a first-class mission state, and what does it mean? | A missing or inconclusive assessment must not be mistaken for a healthy mission or confused with an unhealthy one. | Preserve unknown/unavailable distinctly from GREEN; whether UNKNOWN is a formal state, and its transitions and use, remain unresolved in RFC-0007. |
| ARCH-035 | Is a mission defined by both a scope and an objective, and how are objectives represented? | A scope such as a customer, service, region, or platform does not by itself identify which outcome is being assured. | Candidate objectives include DNS availability, zone provisioning, record propagation, and control-plane operations; whether objectives are entities, configured criteria, or another model remains unresolved in RFC-0007. |
| ARCH-036 | How are investigation coverage, confidence, freshness, and completeness represented? | A completed investigation may examine only a subset of regions or evidence and may become stale; a conclusion's support must not imply full coverage or current validity. | Preserve examined scope/gaps, finding and conclusion confidence, and evidence freshness as distinct outputs; exact semantics and composition remain unresolved in RFC-0008 and RFC-0004. |
| ARCH-037 | What constitutes completion of an investigation, including inconclusive or abandoned outcomes? | Investigations may reduce uncertainty without finding a cause, await human input, exhaust available evidence, or be closed and later reopened; lifecycle affects operator and mission-assurance interpretation. | Define lifecycle states, terminal outcomes, closure/reopening authority, and evidence/completion criteria; unresolved in RFC-0008. |
| ARCH-038 | How are knowledge freshness, coverage, applicability, and trust represented independently? | A knowledge item may be old but recently validated, trusted but outside the current case scope, or only partially applicable; collapsing these dimensions can mislead investigation. | Preserve age, freshness, declared/validated coverage, applicability, and trust distinctly from current-case confidence; semantics remain unresolved in RFC-0009 and RFC-0004. |
| ARCH-039 | How are conflicting, superseded, or invalidated knowledge items represented and retained? | Competing procedures and historically valid but now-invalid guidance must remain explainable without silently selecting or erasing one item. | Preserve competing claims, supersession/invalidation links, provenance, and history; reconciliation, retention, and review rules remain unresolved in RFC-0009. |
| ARCH-040 | How are knowledge gaps distinguished from empty results, incomplete coverage, restricted access, and retrieval failure? | Treating source failure or inaccessible knowledge as “no known solution” can create false assurance and hide operational risk. | Represent known absence, scope gaps, stale/expired knowledge, contradiction, access restriction, retrieval failure, and development gaps distinctly; taxonomy and contract remain unresolved in RFC-0009. |
| ARCH-041 | How are policy coverage, completeness, freshness, and evaluation confidence represented in authorization decisions? | An authorization can be unsafe or misleading if it omits decision scope, relies on incomplete or stale inputs, or presents uncertain evaluation as definitive. | Preserve exact action scope, input coverage/completeness, evidence and approval freshness, and (if adopted) policy-evaluation confidence distinctly; semantics, safe handling, and required fields remain unresolved in RFC-0010 and related RFCs. |
| ARCH-042 | What regionalization and availability boundaries are required: HA work queue, HA control plane, or a hybrid? | Queue continuity may preserve accepted work without requiring every control-plane service to be active-active, but a queue alone may not sustain full workflows. | Classify queues, APIs, databases, workers, builders, serving plane, and reasoning services by locality/availability; supported workflows and topology remain unresolved in RFC-0011. |
| ARCH-043 | How are work ownership and execution authority coordinated across regions? | Multiple regions may see the same queued work; duplicate or concurrent execution can create conflicting production changes, especially during failure and recovery. | Define ownership/lease/fencing, transfer, retry, idempotency, and regional execution rules separately from queue replication and policy authorization; mechanisms remain unresolved in RFC-0011 and RFC-0010. |
| ARCH-044 | Which regional or global authority evaluates and authorizes actions during federation loss or cross-region workflows? | Observation, derivation, evaluation, authorization, and execution may occur in different regions; unclear authority can cause stale or conflicting decisions. | Preserve each locality and authority in provenance; define availability, freshness, and partition-time authorization rules before production execution, without assuming a global or regional authority model. |
| ARCH-045 | What minimum set of subsystems must remain available for each supported end-to-end workflow? | An available queue or service does not guarantee that work can be safely accepted, processed, validated, authorized, and executed when dependent subsystems are unavailable. | Define workflow dependency sets, degraded/unavailable outcomes, and mission-impact-driven continuity objectives; per-subsystem availability, ownership, read/write federation, replication, failure, and recovery contracts remain unresolved in RFC-0011. |
| ARCH-046 | What engine package manifest, local Store, publisher identity, provenance, and review process are required? | Operators need to distinguish bundled defaults, local development, and third-party code without accidentally installing or enabling untrusted executables. | Use one catalog contract for domain engines; start with local/operator-controlled packages. Remote discovery, signatures, automated updates, and publisher governance remain open in RFC-0002/RFC-0012. |
| ARCH-047 | What OS sandbox and capability broker enforce engine tool/script and filesystem/network permissions? | A subprocess still runs with the service account's privileges; descriptive policy metadata cannot prevent arbitrary code from performing host operations. | Treat process boundaries as failure isolation only. Keep action capabilities unavailable until independently enforced and tested under RFC-0010/RFC-0012. |
| ARCH-048 | Which legacy telemetry input contracts and output destinations should be supported first? | Perceptron must enrich existing telemetry and paging workflows without replacing the legacy system or coupling the core to one vendor. | Normalize through versioned adapters and export bulk findings plus bounded metrics; select source, protocol, delivery/authentication, cardinality, and privacy rules in RFC-0012. |

## 13. Review questions

Architecture review should resolve or explicitly accept deferral of the following:

1. Which open decisions must be resolved to build the first core binary, and which can safely remain behind interfaces?
2. Should engines initially execute inside the Go process, as isolated processes, or through a staged approach?
3. What is the minimum event behavior needed for a useful single-binary core, and what must be durable from day one?
4. Is the first release limited to observation and recommendations?
5. Is the Traffic Handler a Perceptron-owned subsystem or a DNS-environment integration with a Perceptron event contract?
6. Should the conceptual cascade use dynamically routed events, and what safeguards prevent cycles or unintended fan-out?
7. What initial trust, confidence, and policy principles are mandatory even before numeric algorithms are designed?
8. Which entities (including service, mission, action, and twin) are first-class, and which systems own their canonical identity?
9. Which source is authoritative for each operational state transition, and how should disagreements be represented and reconciled?
10. How should observed customer impact differ from calculated potential impact in event contracts and mission status?
11. What provenance must be retained to reconstruct decisions, and which engines must support deterministic replay?
12. Should confidence be one scalar or distinct domains for hypotheses, impact, predictions, and actions?
13. What regional locality and federation semantics must be agreed before multi-region deployment?
14. What capability declaration, grant, enforcement, revocation, and audit model is required for engines?
15. How should required and optional engine dependencies affect result validity, coverage, and confidence?
16. Should services be first-class entities, and what defines authoritative service boundaries?
17. Which relationships are authoritative, observed, or inferred, and how are their direction, freshness, time validity, and provenance represented?
18. Are services, components, and deployments distinct entity classes, and how should impact traverse between them?
19. How does mission assurance aggregate service, customer, region, and policy scope, and how is business importance kept distinct from service tier?
20. What security and privacy decisions must precede any use of production traffic or customer data in twins?
21. What engine package review, publisher verification, and upgrade process should the Store enforce?
22. Which legacy telemetry source and output destination should be the first end-to-end adapter pair?
23. What sandbox/capability enforcement is available on Oracle Linux and other target platforms before any plugin is described as read-only?
24. Should paging remain an integration responsibility of the existing monitoring system, or should Perceptron support a direct incident-management adapter?

## 14. Source material

This draft is synthesized from the project context documents:

- `Context/FullSummary.txt.txt`
- `Context/EngineeringSummary.txt.txt`
- `Context/EngineeringScenarios.txt.txt`
- `Context/GoalSetting.txt.txt`
- `Context/PerceptronCascade.txt.txt`
- `Context/CoreConcepts.txt.txt`
- `Context/additionalConcepts1.txt`
- `Context/additionalConcepts2`
- `Context/TrafficHandler.txt.txt`
- `Context/LanguageAndEngines.txt`

Where those sources describe a vision but omit implementation semantics, this document preserves that uncertainty as an open decision rather than inventing a contract.
