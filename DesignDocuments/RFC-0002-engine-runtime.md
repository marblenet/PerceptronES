# RFC-0002: Engine Runtime

**Status:** Architecture review ready  
**Category:** Core platform  
**Parent:** [Overall Architecture](01-overall-architecture.md)  
**Related RFCs:** RFC-0001 Event Bus and Event Model; RFC-0003 Operational Entity Model; RFC-0004 Confidence and Evidence; RFC-0010 Policy Engine

**Visual references:** [System context and trust boundaries](diagrams/D-01-system-context.md); [reasoning cascade and event lineage](diagrams/D-02-reasoning-cascade.md); [engine package, Store, and trust lifecycle](diagrams/D-12-engine-store.md).

## 1. Summary

This RFC defines the proposed lifecycle and responsibilities of an engine in Perceptron SRE. An engine is a small, bounded unit that consumes defined inputs and produces defined outputs through platform contracts. The target architecture treats every domain-specific reasoning module—including first-party defaults—as an engine package selected through the same catalog and event protocol. Event transport, mission aggregation, policy authority, and dashboard services remain core platform responsibilities, not domain-engine packages.

It also describes what an engine is in the Perceptron architecture: its reasoning role, output class, event contracts, dependencies, capabilities, state policy, and self-description. These declarations are proposed runtime requirements; their exact schemas and enforcement mechanisms remain open.

## 2. Problem and motivation

The system is intended to grow by adding many small reasoning and operational components after the Go core binary exists. Without a common engine contract, each component will invent lifecycle behavior, state access, health reporting, and event interactions. That would undermine the intended decoupling and safe replacement of engines.

## 3. Goals

- Establish a common logical engine contract.
- Define lifecycle and failure visibility.
- Prevent direct engine-to-engine coupling.
- Make state ownership explicit.
- Require health and execution observability.
- Provide a place to declare determinism, resource needs, and compatibility.
- Describe engine categories and roles in the operational reasoning model.
- Declare capabilities and event-contract dependencies before runtime loading.
- Define failure containment and degraded coverage expectations.

## 4. Non-goals

- Providing a remote marketplace or automatic third-party installation.
- Treating a process boundary as an OS security sandbox.
- Defining every engine's domain logic.
- Choosing numeric CPU, memory, or timeout limits without workload evidence.

## 5. Candidate engine contract

The following interface comes from the project context and is illustrative:

```go
type Engine interface {
    Name() string
    Initialize() error
    Process(Event) ([]Event, error)
    Shutdown() error
}
```

It is not yet sufficient as a normative interface. An accepted contract must consider context cancellation, configuration, event metadata, output validation, health, concurrency, error classification, capability access, resource control, and version compatibility.

## 6. Engine categories and reasoning role

An engine's category describes its architectural role in the operational reasoning model; it does not prescribe its implementation, language, or deployment process. Candidate categories are:

| Category | Responsibility | Example output |
|---|---|---|
| Observation | Acquire or normalize facts from an integration or telemetry source. | Observation event |
| Pressure | Interpret raw signals as operational pressure. | Pressure assessment |
| Health | Assess service or component health from pressure and other evidence. | Health assessment |
| Correlation | Relate evidence and produce hypotheses or predictions. | Correlation/hypothesis event |
| Impact | Assess observed or potential customer/service impact. | Impact assessment |
| Mission-related domain reasoning | Produce scoped impact or mission-relevant evidence for the core mission service. | Impact or mission-input assessment |
| Investigation | Coordinate evidence gathering and hypothesis evaluation. | Findings and/or recommendation |
| Validation | Evaluate a proposal using a twin or simulation. | Validation result |
| Policy | Evaluate authorization constraints and produce a policy decision. | Authorization decision |
| Remediation | Propose or carry out an authorized operation through a controlled integration. | Recommendation or execution/outcome event |
| Meta | Assess Perceptron SRE itself, such as event-bus, engine, knowledge, or twin health. | Platform-health assessment |

These categories align with the cascade but are not necessarily mutually exclusive. Each engine should declare a primary role and may declare additional roles only if the catalog contract supports them. Meta engines are distinct by subject—the platform itself—not necessarily by runtime. They reason about the integrity, availability, trustworthiness, or effectiveness of Perceptron SRE's own reasoning system; examples include knowledge-trust assessment, mission-assurance health, confidence drift, event-bus health, engine health, and twin health.

Final mission aggregation is a core service, not an installable engine. Domain engines provide evidence and assessments that it consumes. For backward compatibility, the current config reader accepts the legacy `host-mission` engine identifier but does not register it as a domain plugin; the core always emits its mission assessment, including `UNKNOWN` when no required engine evidence is available.

An engine must state where it participates in the operational reasoning model (for example, pressure, health, impact, or mission). This role is descriptive and does not grant authority.

### 6.1 Canonical layers and engine classes

Engines are classified by cascade layer. Layers are written `Layer-<number> <Label>`; Layer-0 Input is numbered because the Knowledge Base memo leaves it unnumbered while fixing Layer 1 and Layer 2. Reasoning is forward-only: there is no backpropagation, runtime retraining, or hidden weight adjustment.

| Layer | Engine class | Consumes | Produces | Prototype engine |
|---|---|---|---|---|
| Layer-0 Input | Tap engine | Host sources (`/proc`, `/sys`), supplied evidence | Observations only | collectors in `internal/observe` |
| Layer-1 Information | Information engine (Layer-1 perceptron) | Observations | Information, not conclusions | `host-pressure` |
| Layer-2 Determinations | Determination engine (Layer-2 perceptron) | Information | Determinations | `host-determination`, `security-posture` |
| Layer-3 Assessment | Assessment engine | Determinations and health inputs | Assessments (host, security, DNS, mission) | `host-health`; core mission service |
| Layer-4 Recommendation | Recommendation engine | Assessments or determinations | Advisory recommendations | `host-recommendation` |
| Layer-5 Policy | Policy engine | Recommendations | Policy decisions (RFC-0010 8.1) | `host-policy` |
| Layer-6 Action | Action engine | Authorized recommendations | Action records | `host-action` (record only) |
| Layer-7 Outcome | Outcome engine | Determinations over time, actions | Outcomes | `host-outcome` |
| Layer-8 Learning | Learning engine | Outcomes, investigations, Why/How chains | Knowledge, simulation, and regression-test candidates | `host-learning` |

Source files in the Go core are named `layer<N>_<name>.go` and engines run in the configured order; the runtime requires every consumed event type to come from an initial event or an earlier engine. Open: Layer-2 determinations do not yet feed Layer-3 health or mission; Layer-4 currently consumes determinations rather than assessments.

### 6.2 Core + Extension and resource budgets

Every engine class supports **Core**, **Local Operator**, and **Community** variants (Overall Architecture 4.25). Core engines ship with the platform; Local engines are private to an organization; Community engines arrive through reviewed pull requests or Store packages. The platform grows by extension rather than modification of the core, and the core stays deterministic, small, and efficient while extensions may be flexible or experimental.

Critical-path components (tap engines, event bus, core runtime, assessment pipeline, policy runtime) prioritize correctness, determinism, efficiency, stability, and predictability over feature velocity. Each critical-path component and each engine declares CPU, memory, storage, network, and execution budgets; none may have unlimited growth characteristics, and Perceptron SRE must not cause the incidents it reasons about (for example metrics collection exhausting CPU, logging filling a disk, an engine consuming excess memory, or the pipeline becoming a bottleneck). Budgets are declarations until the runtime enforces them; the prototype enforces only plugin timeouts and output bounds, and numeric defaults need workload evidence.

Configuration is version-controlled operational knowledge organized by domain (Overall Architecture 8.4): optional `config.d/<domain>.json` files overlay domain settings, each limited to its own keys, while platform settings stay in the core file.

## 7. Engine self-description and declarations

**Proposed requirement:** before an engine is loaded or enabled, it declares enough metadata for the runtime and operators to understand and govern it. Candidate fields include:

- Stable engine name and version.
- Owner or responsible team.
- Category and reasoning role.
- Determinism declaration.
- Required and produced event/schema contracts.
- Requested capabilities.
- Configuration schema/version.
- Health and resource-reporting contract.
- Statefulness and persistence requirements, if any.

Determinism is a scoped declaration, not an inference from using rules or a fixed seed. A deterministic engine must produce the same decision and explanation content for identical normalized inputs, engine version, and configuration. Evaluation timestamps, generated IDs, and other explicitly variable metadata may differ without changing the decision. Engines that depend on nondeterministic sources must identify those sources and the affected outputs. Replay and audit claims are limited to the declared, tested behavior.

Illustrative metadata only:

```yaml
engine:
  name: dns_health
  version: "1.0"
  owner: platform
  category: health
  deterministic: true
  consumes:
    - dns.pressure.v1
  produces:
    - dns.health.v1
  capabilities:
    - read_events
    - publish_events
```

This example does not define final field names, catalog format, or a normative schema.

### 7.1 Engine catalog

**Proposed architecture:** the runtime should expose an engine catalog/registry describing the engines known to that runtime and their declared metadata. The catalog supports discovery, validation, operations, and governance; it does not imply a centralized global registry or a particular persistence technology.

Candidate catalog entries include engine name and version, owner, category/role, lifecycle status, requested and granted capabilities, required and produced contracts, determinism, health, and coverage. Status should distinguish at least registered/available, enabled/running, degraded, failed, and stopped if those states are adopted by the lifecycle contract.

The operator-facing Engine Store is a catalog view, not an automatic code installer. It should show first-party defaults, locally developed packages, and locally declared third-party packages with origin, version, category, input/output contracts, settings schema, and policy declaration. Local authors need a contribution checklist; published third-party entries require a separate source, review, provenance, and upgrade process. Remote discovery, package signing, publisher identity, automated updates, and marketplace hosting remain future decisions. The catalog must not treat declared capabilities as granted capabilities or health as proof of complete coverage.

Engine configuration is namespaced by engine identity and version. Global collector/runtime settings remain core configuration; thresholds and domain-specific settings belong to the engine that interprets them. Configuration updates must be validated against the declared schema and attributed to derived outputs.

### 7.2 Capability declaration

Candidate capabilities include reading events, publishing events, running diagnostic tools, reading knowledge, using a twin, requesting an action, and executing an action. Capabilities must be least-privilege: an engine receives only what its declared responsibility requires.

Declaring a capability is a request, not a grant. The runtime or an explicit authorization service must validate, enforce, and audit granted capabilities. High-risk capabilities such as action execution remain subject to RFC-0010 policy and must not be obtained merely by engine registration. The exact capability vocabulary, grant authority, enforcement boundaries, and audit record are open under ARCH-023.

Each package declares an action-policy profile: `reasoning_only`, `human_escalation`, or `self_mitigation`. It also declares whether it is intended to be read-only and which named tools/scripts it requests. These declarations are policy inputs, not grants. `self_mitigation` permits an engine to propose a bounded action for independent policy evaluation; it never authorizes execution. `human_escalation` permits an attributable request for human review, not human approval.

Until a core-owned capability broker and OS sandbox exist, tool/script access and self-mitigation execution remain unavailable. A `read_only: true` declaration cannot prevent arbitrary process code from writing files, launching processes, or accessing the network. External engine code is trusted executable code and must not be represented as sandboxed.

### 7.3 Required and produced contracts

An engine declares the event contracts it consumes and produces. Dependencies between engines are expressed through these contracts and event routing, not direct implementation references or engine-to-engine calls. A runtime may use declarations for validation, graph inspection, startup checks, and operator documentation; it must not assume that declaring a dependency guarantees an event will arrive.

Dependencies may have different importance to a consuming engine. A candidate distinction is **required dependency**, whose loss prevents a valid result for the declared scope, versus **optional dependency**, whose loss permits a result with reduced coverage or confidence. The exact vocabulary, declaration model, transitive behavior, and runtime enforcement remain open under ARCH-024. This RFC does not decide whether the overall cascade is statically or dynamically composed (ARCH-013).

## 8. Output classes and confidence

Engine outputs must distinguish fundamentally different kinds of information. Candidate output classes are:

- **Fact/observation:** a source measurement or assertion with provenance.
- **Assessment:** a calculated or inferred health, pressure, impact, hypothesis, prediction, or validation result.
- **Recommendation:** a proposed next step or change.
- **Authorization decision:** a policy outcome permitting, denying, or conditioning an action.
- **Execution/outcome:** a record of an operation attempted and its observed result.

The runtime and event contracts should preserve these distinctions. A measurement such as CPU utilization is not interchangeable with a recommended action such as restarting a worker. An engine may produce confidence-bearing assessments or consume confidence as input; raw observations need not be assigned artificial confidence. Confidence semantics belong to RFC-0004.

Only the policy authority defined by RFC-0010 may produce an authorization decision. A non-policy engine cannot convert a recommendation or confidence score into authorization. Execution is separate from authorization and must be performed through an explicitly granted integration.

**Engines cannot self-promote their outputs.** An engine producing an assessment or recommendation cannot relabel it as a policy authorization or execution result, regardless of its confidence or granted event-publishing capability. Output-class validation and policy authority must enforce this separation.

## 9. Lifecycle

The proposed lifecycle states are:

```text
Created → Initialized → Running → Stopped
                    ↘ Failed → (recovery policy, if supported)
```

`Failed` and recovery transitions must be observable. The runtime must define what happens to in-flight work, owned resources, and unacknowledged inputs during failure or shutdown. Automatic restart and retry behavior are not assumed.

Candidate lifecycle operations are registration, declaration validation, initialization, readiness, processing, health reporting, graceful stop, and failure reporting. Their exact API and state machine remain open.

## 10. Engine ownership and state rules

Proposed rules:

1. An engine owns one declared responsibility and publishes a contract for its inputs and outputs.
2. Engines communicate through events or another explicitly versioned platform contract; they do not directly invoke one another.
3. **Stateless execution is preferred.** An engine may keep local state only when that state materially improves correctness, performance, or safety and the rationale is documented.
4. An engine does not own shared mutable state. It may hold private local state or use explicitly granted storage capabilities.
5. Any persistent state must have documented ownership, recovery, migration, and replay behavior.
6. State must not become an undocumented communication channel or undermine event lineage and replay.
7. An engine must not silently treat absent or invalid inputs as healthy.
8. An engine's failure and health must be visible independently of its business output.

Stateless preference supports replay, testing, replacement, and isolated failure handling. State is not prohibited, but its necessity and lifecycle must be explicit. These rules require review and enforcement mechanisms, especially if engines run in one process.

## 11. Execution and isolation

The target direction is a Go core with all domain engines—including first-party defaults—represented as packages under `main/external`, with versioned manifests and the shared engine protocol. The core retains orchestration, event transport, mission aggregation, policy authority, and operator-facing services.

Out-of-process execution is the preferred failure-isolation boundary and permits engines written in other languages. It is not sufficient security isolation: separate processes still share the service account and host permissions unless an explicit OS sandbox constrains them. Privileged credentials must not be passed directly to engines; capability requests must be mediated through core-owned APIs.

The current v0.1 implementation is transitional: bundled engines are compiled into the core and execute in-process, while configured external engines use a subprocess JSON protocol. It does not yet satisfy the target that every default engine uses the same external package boundary. Process isolation, sandboxing, and the migration plan remain separate requirements under ARCH-001.

## 12. Resource limits and concurrency

The runtime contract must define or configure CPU and memory limits, processing deadlines, queue bounds, concurrency, cancellation, and behavior when limits are exceeded. Values must be based on measured workloads and deployment constraints; this draft does not invent them.

The runtime must prevent a slow or faulty engine from silently exhausting shared resources or blocking unrelated engines. The appropriate mechanism depends on the isolation model and must be addressed in the selected failure-domain design.

## 13. Health, failure domains, and operational reporting

The project context expects each engine to expose health, execution time/latency, failures, and decisions; confidence is also an important platform signal. The exact health model and metric contract remain open.

Engine health must be distinguishable from the operational health result produced by the engine. A failed DNS Health Engine, for example, is not equivalent to a healthy DNS service. Likewise, **engine availability, output coverage, and confidence are separate dimensions**:

- **Availability/health:** whether the engine is able to execute and produce outputs.
- **Coverage:** which expected entities, regions, inputs, or time windows are actually represented by its outputs.
- **Confidence:** how strongly the evidence supports a particular conclusion.

An engine may be healthy but have limited coverage (for example, receiving data for only one region). An engine may be available but produce a low-confidence assessment. A failed engine may leave a dependent consumer with no coverage rather than a healthy result.

**Proposed failure-domain principle:** when an engine becomes unavailable, dependent reasoning should reduce the affected coverage and, where meaningful, confidence before producing a conclusion. Missing engine output must not be interpreted as healthy or as evidence of no impact. Failure of one engine should not automatically make unrelated engines or the whole platform unavailable; consumers and mission assurance must distinguish “no adverse signal” from “the engine that could report this signal is unavailable.”

This is a degradation goal, not a guarantee that all failures can be contained. Dependency chains, critical engine loss, shared-process failures, stale outputs, and platform-level failure require explicit health/coverage propagation and policy. The runtime must not silently substitute healthy outputs when an engine fails.

## 14. Determinism and replay

An engine should declare whether its output is deterministic for a given engine version, configuration, and input set. Probabilistic or ranking engines may not be deterministic; deterministic pressure calculation may be. This declaration supports testing and honest replay expectations but does not itself guarantee reproducibility.

RFC-0001 defines event replay. This RFC must define engine/configuration identity and any state required to reproduce processing. ARCH-020 remains open.

## 15. Configuration and compatibility

Engine identity, version, configuration schema, required capabilities, and input/output schema versions need explicit representation. Runtime upgrades must define behavior for unsupported engine versions or invalid configuration. Configuration changes must be observable and attributable to resulting outputs.

No configuration syntax or loading model is selected. Configuration must not silently grant capabilities beyond those authorized for the engine.

## 16. Failure and security behavior

The engine contract must explicitly cover initialization errors, processing errors, invalid outputs, timeouts, cancellation, panic/crash containment, shutdown failure, and dependency unavailability. There must be no broad silent fallback that emits a success-shaped result after failure.

Engines capable of tools, storage, twin access, or production actions require least-privilege capabilities and policy enforcement. Requested capabilities must be validated and their grants auditable. An engine must not gain action authority merely by being loaded.

## 17. Acceptance criteria for an accepted RFC

- Lifecycle states and transitions are testable.
- Each engine declares its category and role in the operational reasoning model before loading.
- Each engine declares its capabilities, determinism, required/produced contracts, and dependency semantics before loading.
- An engine catalog exposes declared metadata and runtime status without conflating declaration, capability grant, health, or coverage.
- Runtime validates declarations and rejects incompatible or unauthorized engines with an explicit diagnostic.
- Capability grants are least-privilege, enforceable, and auditable.
- Output classes distinguish facts/observations, assessments, recommendations, policy authorization decisions, and execution/outcomes.
- Each engine is assigned a canonical cascade layer (6.1), supports the Core/Local/Community model, and declares resource budgets; critical-path components have enforced or explicitly labelled-as-declared budgets.
- Domain configuration overlays reject keys outside their domain.
- Engines cannot self-promote a recommendation or assessment into an authorization or execution result.
- Engine inputs and outputs are validated against versioned contracts.
- A failing engine reports failure without being represented as a healthy domain result.
- Failure of an engine reduces affected coverage or confidence without unnecessarily collapsing unrelated runtime functions.
- Consumers and mission assurance can identify availability, coverage, and confidence separately.
- Required and optional dependency loss have explicit, testable effects on dependent outputs.
- Shutdown and cancellation behavior is tested.
- Resource and concurrency limits are observable and enforced according to the selected isolation model.
- Direct engine invocation and shared mutable state rules have an enforcement or review mechanism.
- Stateless engines can be replayed/tested from declared inputs; stateful engines document and test their state lifecycle.
- Engine health is distinguishable from domain health.
- Determinism and replay expectations are declared and tested for representative engine classes.
- Deterministic fixtures produce the same decision and explanation content for identical versioned inputs/configuration, excluding explicitly variable metadata.
- Missing, stale, contradictory, malformed, and unavailable dependencies cannot be converted into success-shaped engine results.

## 18. Open decisions

1. What package manifest, settings-schema format, and compatibility policy are normative?
2. What is the normative engine interface and how is configuration supplied?
3. Which engine categories and role metadata are normative, and may an engine declare multiple roles?
4. What are the self-description and contract-declaration schema/versioning rules, and what catalog/registry scope is required?
5. What is the capability vocabulary, grant authority, enforcement mechanism, revocation behavior, and audit format (ARCH-023)?
6. How are required versus optional dependencies declared, validated, and propagated through the graph? How are incomplete chains represented in coverage/confidence (ARCH-024)?
7. What coverage dimensions are required, and how do availability, coverage, and confidence compose in outputs?
8. Which local or persistent state is permitted, and how is its justification, recovery, and replay handled?
9. What resource budgets (CPU, memory, storage, network, execution) are global defaults versus per-engine, which are enforced, and how are overruns surfaced?
10. How are concurrency, retries, idempotency, and backpressure handled?
11. What health states and signals are required, and how does degraded coverage propagate through dependencies?
12. Which engine classes must be deterministic?
13. How are engines registered, upgraded, disabled, and rolled back?
14. How will first-party engines migrate from the transitional in-process implementation to the common package boundary?
15. Which OS sandbox and capability broker are supported across deployment platforms?
16. What publisher identity, signing, provenance, review, and update rules govern third-party Store entries?
17. Which legacy telemetry input and output adapters are required, and what authentication, delivery, and cardinality behavior applies?

## 19. Dependencies

RFC-0001 defines event inputs, outputs, replay, and delivery context. RFC-0003 and RFC-0004 define domain references and evidence/confidence contracts. RFC-0010 governs any engine capability that can request or perform an action.

## 20. Engine Store and integration outputs

The dashboard Store is an operator catalog for bundled defaults and locally declared engines. It supports inspection, enable/disable, ordering, and engine-specific settings; it must not silently download or execute newly discovered packages. Third-party packages remain operator-installed and require a trust, version, provenance, privilege, and compatibility review.

Every assessment cycle must make the complete finding/event set available as a machine-readable bulk report. Metrics adapters may expose bounded, stable time-series projections, such as Prometheus exposition for Grafana scraping, but must not flatten arbitrary event payloads into unbounded labels. External adapters should accept legacy-system measurements through versioned observation/event contracts and export derived findings to the existing telemetry destination. The core event model must not depend on Grafana or a vendor-specific backend.

The current v0.1 dashboard exposes current-cycle JSON and a Prometheus-compatible `/metrics` endpoint. It does not provide legacy-metric ingestion, remote-write delivery, durable incident history, package repositories, signature validation, or process sandboxing. These capabilities are not implied by the Store UI or subprocess protocol.

## 21. Acceptance criteria for the target engine ecosystem

- Bundled defaults and third-party engines use equivalent catalog metadata, settings ownership, event-contract validation, and enablement controls.
- Mission aggregation, event transport, policy enforcement, and core APIs remain core responsibilities.
- Engine settings are grouped and validated by engine; global collector settings remain distinct.
- Every engine declares category, version, contracts, settings schema, and action-policy profile.
- Policy declarations are never treated as granted capabilities or as an OS sandbox.
- Tool/script execution and production changes require a core-enforced capability grant and RFC-0010 authorization.
- Store entries identify origin and make version/trust/install state visible.
- Full findings are exportable in a versioned machine-readable form; metric projections have bounded cardinality and preserve unknown/unavailable states.
- Legacy ingestion and export use versioned adapters and can augment an existing telemetry destination.
