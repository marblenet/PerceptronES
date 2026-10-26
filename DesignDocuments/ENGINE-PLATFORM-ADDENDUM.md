# Engine Platform Addendum

**Status:** Proposed architecture guidance  
**Scope:** Engine packages, observation taps, simulation, event origins, human operations, and contributor contracts  
**Related:** [RFC-0001](RFC-0001-event-bus-and-event-model.md), [RFC-0002](RFC-0002-engine-runtime.md), [RFC-0004](RFC-0004-confidence-and-evidence.md), [RFC-0010](RFC-0010-policy-engine.md), [RFC-0012](RFC-0012-engine-store-and-telemetry-interoperability.md)

This addendum extends the platform vision; it does not silently approve unresolved schemas or grant action authority. Existing behavior and stage gates remain authoritative until the relevant contracts and tests are accepted.

## Platform and engine model

Perceptron SRE comprises the core runtime, catalogued domain engines, observation taps, a dashboard, policy services, and an evolving simulation framework. The core owns event transport, cross-engine aggregation, mission assessment, policy authority, and platform APIs. Domain-specific capabilities should use the engine lifecycle and contracts; core services that reason across all domains are not Store packages.

An engine is a bounded component with a declared purpose, inputs, processing rationale, outputs, unknown conditions, and tests. The engine catalog and Store may eventually represent reasoning engines, tap engines, simulation packages, knowledge packs, and dashboard extensions. Discovery, installation, verification, enablement, and execution are separate stages. Catalog metadata is not a sandbox or permission grant.

### Tap engines and open domains

Tap engines observe, capture, normalize, and publish facts. They do not interpret those facts as recommendations, authorize changes, or execute actions. Reasoning engines consume their versioned observations. This separation enables independent testing, replay, and simulation.

Domain and category identifiers are extensible data, not a closed set in core code. Initial examples include `host.health`, `host.resources`, `host.security`, `process`, `application`, `database`, `dns`, `network`, `mission`, `policy`, and `simulation`; new domains must not require redesign. Names such as `weather.sleet` and `space.deep_space_link` are valid illustrative extensions.

### Reasoning layers (L1–L6)

Dashboards and reports name the forward-only cascade with these layers:

| Layer | Name | Meaning | Current event types |
|---|---|---|---|
| L1 | Metadata / Data | Raw observations and cycle context | `observation.host.v1`, `observation.security.v1`, `assessment.cycle.started.v1` |
| L2 | Information | Data interpreted into measures | `pressure.host.v1` |
| L3 | Determination | Per-component health/finding judgments | `health.host.v1`, `security.finding.v1` |
| L4 | Assessment | Scoped mission assessment | `mission.host.v1` |
| L5 | Incident Report | Why/How incident record (RFC-0008 7.1) | derived view only; no events |
| L6 | Mitigation | Recommended/executed mitigation | recommendations only; none executed or recorded |

After L6, the loop returns to **L5 Incident Report** to record resolution or recurrence. This return is not implemented: resolution/recurrence is shown as Unknown. Unmapped plugin event types are shown as "Unmapped" rather than guessed into a layer. Layer names are presentation labels, not new event contracts.

### Cascade engine classes

The platform distinguishes engine classes so contributors can reason about where logic belongs:

- **Tap engines:** collect, normalize, and publish observed facts.
- **Layer-1 perceptrons:** convert observed metadata/signals into intermediate information.
- **Layer-2 perceptrons:** convert information into determinations.
- **Assessment engines:** convert determinations into health/impact/mission/security assessments.
- **Recommendation engines:** convert assessments into suggested mitigations and investigation paths.
- **Policy engines:** determine authorization boundaries for any potential action.

Tap engines must not publish interpreted assessments, recommendations, or actions.

## Contributor contract and engine proof

Every engine package should document:

1. **Purpose:** the operational question it addresses.
2. **Inputs:** event types, schemas, source/scope, freshness, and required/optional status.
3. **Processing:** bounded, inspectable rules or model semantics, including how evidence contributes and conflicts are handled.
4. **Outputs:** event types, schemas, status semantics, derivation links, and claims made.
5. **Unknown conditions:** missing, stale, contradictory, invalid, and unavailable input behavior.
6. **Confidence:** optional and claim-specific only where meaningful. Confidence must not be inferred from coverage or emitted without semantics/calibration.
7. **Tests:** normal, boundary, failure, unknown, and synthetic scenario coverage.
8. **Operations and safety:** version, owner, determinism/state, resource limits, capabilities requested, policy profile, platform support, dependencies, and upgrade/rollback guidance.

Each derived claim should provide an evidence-linked explanation: what was assessed, why, supporting and contradictory evidence where known, and what prevents a stronger conclusion. This “engine proof” is an explanation and provenance contract, not a mathematical proof or a guarantee of correctness. Contributors should be able to implement a useful engine by understanding its purpose, inputs, outputs, and tests without coupling to unrelated platform internals.

Perceptron examples must distinguish threshold scripts from multi-signal weighted reasoning. A perceptron-style score may combine explicit evidence weights, but weights and thresholds require domain justification and tests. The name does not imply a trained neural network, calibrated probability, or unbounded AI. The current v0.1 runtime remains deterministic rule-based reasoning and does not emit confidence scores.

## Event origin and human operations

Events should carry an explicit, validated origin that propagates through derived outputs. The conceptual origins are `live`, `simulation`, `replay`, and `human_action`; exact schema/versioning is governed by RFC-0001. Simulated or replayed data must remain distinguishable from live observations. Simulation-origin events may be reasoned over and displayed but must never authorize production execution.

Human actions are evidence, not proof of correctness. A future action record should identify the actor, host/entity, time, command or procedure reference, incident/context, and provenance, subject to privacy and secret-redaction rules. It should distinguish a human-reported action from one verified by a tap or control-plane observation.

The proposed Perceptron Engineering Shell (PES) may eventually run approved legacy commands while recording attributable human-action events. This proposal does not authorize arbitrary command execution, privilege escalation, or capture of secret-bearing arguments. Before implementation, define authentication, actor identity, command allow/deny behavior, redaction, failure semantics, audit durability, and threat model. PES must preserve the command's normal exit/result semantics where it is approved, and cannot claim an action succeeded merely because it was launched.

## Simulation, dry run, and execution

Simulation and dry run are different workflows:

- **Simulation** evaluates synthetic inputs, captured snapshots, or explicit overrides without production effects.
- **Dry run** validates or predicts a proposed operation without executing it. It may be used in simulation or production context and must state which inputs and side effects are excluded.
- **Real run** executes only through a separately authorized production path and is not implied by either simulation or dry run.

Snapshot simulation should eventually support capture, immutable snapshot identity, explicit overrides, and rerun through the reasoning pipeline. Every result should show its baseline, overrides, configuration/engine versions, output, and provenance. Production snapshots require data minimization, access controls, retention rules, and clear separation from live streams.

Simulation must be isolated from production action adapters by construction. A human override is not an automatic grant: it requires authenticated identity, authorization, reason, incident reference, explicit scope, and durable audit, and produces a human-action event. The authority and emergency semantics remain open in RFC-0010. Human engineers remain accountable decision-makers; the system observes, reasons, explains, recommends, validates, and escalates.

## Plugin ecosystem direction

Everything outside the core runtime trends toward plugin-style packaging over time. That includes reasoning engines, taps, simulation bundles, and future dashboard/knowledge extensions. Contributor burden should stay centered on declaring inputs, outputs, purpose, tests, and simulation fixtures; core services continue to own registration lifecycle, runtime governance, visualization contracts, and storage interfaces.

## Adoption and acceptance gates

These proposals are adopted incrementally, not as a promise that every Store package type or workflow exists now. Before accepting a capability, update its RFC, implement versioned contracts, add conformance/failure tests, and document its operational limits. The initial application remains observe-only; current subprocess execution is not a sandbox. No simulation, confidence value, human override, tool declaration, or process boundary grants execution authority.

Open decisions include the manifest and proof schema, tap lifecycle and access boundaries, extensible domain namespace governance, simulation snapshot privacy/retention, PES authentication and redaction, and the policy semantics of emergency human overrides.
