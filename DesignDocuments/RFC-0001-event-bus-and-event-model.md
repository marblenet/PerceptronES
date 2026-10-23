# RFC-0001: Event Bus and Event Model

**Status:** Architecture review ready  
**Category:** Core platform  
**Parent:** [Overall Architecture](01-overall-architecture.md)  
**Related RFCs:** RFC-0002 Engine Runtime; RFC-0003 Operational Entity Model; RFC-0004 Confidence and Evidence; RFC-0010 Policy Engine

**Visual references:** [Reasoning cascade and event lineage](diagrams/D-02-reasoning-cascade.md); [incident report time window](diagrams/D-11-incident-time-window.md); [regional federation](diagrams/D-10-regional-federation.md).

## 1. Summary

This RFC proposes the event contract and event-bus responsibilities that allow Perceptron SRE components to exchange observations and derived operational results without direct engine-to-engine calls. It describes event immutability, provenance, taxonomy, routing, replay, retention, and versioning as requirements to resolve before the bus becomes a dependency for operational reasoning.

The exact envelope, transport, durability, ordering, delivery semantics, and retention periods are open decisions. The sample below is illustrative and must not be treated as an approved schema.

## 2. Problem and motivation

Perceptron SRE is composed of small components that need to transform telemetry into progressively higher-level operational understanding. Without a common event contract, engines will develop incompatible payloads, hidden dependencies, and untraceable decisions. Without replay and provenance, incidents cannot be reconstructed reliably.

The event bus is the platform's **operational nervous system**: the primary integration mechanism for operational reasoning, investigation, validation, policy evaluation, remediation, mission assurance, and platform self-observation. It must support decoupled publication and subscription while making event lineage and failure behavior explicit.

This does not mean every low-level runtime operation must be represented as a bus event. Explicitly approved runtime services may use internal calls where necessary, but this exception must not become a path for engines to bypass event contracts, policy, provenance, or audit.

## 3. Goals

- Define a common, versionable event envelope and taxonomy.
- Support publication, subscription, filtering, correlation, and replay.
- Preserve event origin, time context, and derivation provenance.
- Reference the operational entities to which events apply.
- Distinguish association, direct causation, and derivation.
- Distinguish event safety class from operational severity.
- Make replay safety explicit so replay cannot accidentally execute production actions.
- Emit events about event-bus and platform health.
- Make events suitable for audit and incident reconstruction.
- Keep event transport replaceable behind a stable contract.
- Specify testable behavior for duplicates, invalid events, unavailable consumers, and replay.

## 4. Non-goals

- Selecting a specific broker or serialization technology in this draft.
- Requiring every event to be persisted forever.
- Defining the detailed payload of every engine's events.
- Making the event bus itself the universal source of operational truth.
- Assuming confidence is applicable or numeric on every event.

## 5. Principles and proposed invariants

1. **Immutable event records:** once accepted, an event is not edited in place; corrections or new conclusions are represented by additional events linked to the prior record.
2. **Append-oriented history:** the logical record of accepted events is append-only. Physical storage, compaction, retention, and privacy deletion mechanisms are implementation decisions.
3. **Provenance:** events identify their source and preserve links to causal inputs where available.
4. **Decoupling:** producers publish events; consumers subscribe through platform contracts rather than calling other engines directly.
5. **Explicit versioning:** event type and schema version must be identifiable.
6. **Explicit origin:** events identify whether they are live, simulated, replayed, or human-action records; derived events preserve their input origin.
7. **Replay safety:** replay must not be confused with a new live occurrence; consumers need enough context to avoid unintended duplicate side effects.
8. **No success-shaped missing data:** invalid, unavailable, or incomplete event inputs must not be silently interpreted as healthy state.
9. **Producer-owned creation:** the producer is responsible for creating and identifying an event. Consumers may interpret it and publish new derived events, but do not mutate the original event.
10. **Evidence lineage:** a derived event must be able to reference the observations, evidence, or prior events from which it was derived, where applicable.
11. **Separate relationship semantics:** correlation, causation, and derivation are distinct relationships and must not be represented by one ambiguous identifier.
12. **Safety-class awareness:** event replay, routing, and authorization behavior consider the event's safety class; severity is a separate property.
13. **Facts and conclusions are distinct:** observations report what a source measured or asserted; derived events report an interpretation, calculation, or prediction based on inputs.
14. **Events carry claims, not universal truth:** an event records what a source claims or observed within its scope. Conflicting claims from different sources may coexist; authority and reconciliation are defined by domain contracts.
15. **Freshness is explicit where relevant:** event age and applicability affect whether a value is useful for a current decision. Freshness is not the same as source trust or confidence.
16. **Human-originated decisions are representable:** human validation, approval, escalation, and override can be recorded as attributable events with appropriate authorization context.

These are proposed contract principles. Their exact enforcement varies with the selected transport and runtime.

## 6. Candidate event envelope

The following fields are candidates drawn from project context and feedback, not an approved schema:

```json
{
  "event_id": "unique event identity",
  "correlation_id": "workflow or incident association",
  "causation_id": "direct triggering event, when applicable",
  "derives_from": [],
  "incident_id": "incident reference, when applicable",
  "source": "producer identity",
  "type": "dns.propagation.observed",
  "schema_version": "version identifier",
  "origin": "live | simulation | replay | human_action",
  "occurred_at": "source observation time",
  "emitted_at": "event publication time",
  "locality": {
    "observed_in_region": "region reference, when applicable",
    "derived_in_region": "region reference, when applicable",
    "evaluated_for_region": "region reference, when applicable",
    "data_origin": "local or replicated"
  },
  "classification": {
    "reasoning_kind": "observed",
    "safety_class": "informational"
  },
  "freshness": {},
  "entity_refs": [],
  "payload": {},
  "provenance": {},
  "confidence": {}
}
```

The example separates event identity, workflow correlation, and direct causation. `derives_from` is a candidate explicit relationship for one or more events/evidence items that contributed to a derived result; it is distinct from direct causation. `origin` distinguishes live evidence from simulation, replay, and human-action records and must propagate to derived events; mixed-origin inputs require an explicit future contract rather than an implicit conversion. It also separates reasoning kind from safety class. The exact nesting and names are illustrative.

The example separates occurrence time from publication time to avoid conflating source time with bus arrival time. This is a candidate distinction, not a settled clock model. Regional locality distinguishes where an observation occurred, where a result was derived, and which region it assesses; these locations can differ. `freshness` is contextual and does not imply a universal time-to-live. `confidence` is an illustrative placeholder for an optional, claim-specific confidence value or domain model pending RFC-0004 and ARCH-014/ARCH-021; policy context must not be encoded as factual evidence. A source trust measure, if represented, is distinct from confidence in the event's conclusion.

**Open schema questions:** identifier format and scope; entity-reference structure; direct-causation and multi-input derivation links; timestamp precision and clock source; regional identity and event locality; local versus replicated origin semantics; payload encoding; provenance structure; confidence-domain and source-trust applicability; freshness representation; reasoning-kind and safety-class vocabularies; severity; sensitivity labels; human actor identity and authorization references; trace context; schema compatibility rules; unknown-field behavior.

## 7. Facts, conclusions, and claims

An observation and a conclusion are different classes of operational information:

- **Observation/fact report:** states what a source measured, received, or asserted within a time and scope, such as an observed IXFR duration.
- **Conclusion/derived result:** interprets one or more observations or evidence items, such as a propagation-risk assessment.

An observation is not automatically universal truth: it remains a source claim with provenance, locality, time, and possible limitations. A derived result is an interpretation and must be classified as calculated, predicted, simulated, or otherwise applicable. Consumers and operators must be able to distinguish these classes and not present an interpretation as a directly observed fact.

The authoritative-source model and reconciliation of contradictory claims are defined by the relevant domain contract (ARCH-017), not by the event bus. Events from a database, builder, DNS server, twin, or human may all be preserved as separate claims with their own provenance.

## 8. Event taxonomy

The proposed high-level categories map to the operational reasoning model:

- **Observation:** raw measurements, state changes, reports, and integration facts.
- **Pressure:** derived operational pressure.
- **Health:** service or component condition.
- **Correlation:** associations, candidate relationships, and hypotheses across observations or health events.
- **Impact:** observed or potential customer/service impact.
- **Mission:** mission status and operational disposition.
- **Investigation:** requests, evidence, hypotheses, and investigation results.
- **Validation:** twin or simulation validation requests and results.
- **Remediation:** proposals, execution requests, and execution outcomes.
- **Policy:** evaluations, approvals, denials, holds, and emergency-control state.
- **Outcome:** post-action verification and incident/knowledge feedback.
- **Platform health:** core, bus, engine, knowledge, twin, and other Perceptron SRE health observations.
- **Human decision:** human validation, approval, escalation, or override, with attributable actor and authorization context.

Categories are for discoverability and governance; they do not define a fixed pipeline or one event per category. Event names and payload schemas should be owned in a registry or equivalent versioned contract. The registry mechanism remains open.

## 9. Event ownership and entity references

### 9.1 Ownership

The producer owns the creation of an event and the accuracy of its source identity, payload, and stated provenance. The event record is immutable after acceptance. A consumer may interpret an event, attach its own assessment through a new event, or issue a correction/superseding event; it must not edit the producer's event in place.

Producer ownership does not make the producer the authority for every fact in its payload. Operational source authority is defined by the relevant integration/domain contract (see ARCH-017 and RFC-0003).

### 9.2 Entity references

Events should reference relevant first-class operational entities, such as customer, zone, record, region, and incident, using stable, namespaced references defined by RFC-0003. Payloads should not rely solely on display names for identity.

Illustrative shape only:

```json
{
  "entity_refs": [
    {"entity_type": "customer", "namespace": "dns-platform", "id": "opaque-id"},
    {"entity_type": "zone", "namespace": "dns-platform", "id": "opaque-id"}
  ]
}
```

Not every event requires every entity type. Exact identity, tenant scoping, relationship history, privacy, and schema conventions belong to RFC-0003 and remain open here.

## 10. Event relationships and lineage

The event contract must distinguish:

- **Correlation:** events are associated with the same incident, workflow, or analysis context. Association alone does not claim causal influence.
- **Causation:** an event was directly triggered by another event or action, where that relationship is known.
- **Derivation:** an event's value or conclusion was calculated or inferred from one or more inputs, evidence items, or prior events. The candidate `derives_from` relationship makes this lineage explicit rather than burying it in generic provenance.

These may have different cardinalities: a result can be correlated with many events, directly caused by one triggering event, and derived from multiple inputs. An event may have correlation without known causation; causal links must not be fabricated merely to complete a graph.

**Proposed requirement:** every derived event can reference the observations, evidence, and prior events that contributed to it, where available. Lineage must preserve enough identity and version context to support explanation and reconstruction without requiring sensitive raw data to be copied into every event.

The exact lineage graph, handling of unavailable inputs, retention, cycles, and independent evidence sources remain open. This contract is related to but distinct from an event's `correlation_id`.

## 11. Classification: epistemic kind, safety class, and severity

Event classification must not collapse different dimensions into a single label:

### 11.1 Epistemic/reasoning kind

Candidate kinds:

- **Observed:** directly reported or measured.
- **Calculated/derived:** computed from identified inputs.
- **Predicted:** a forecast about a future or hypothetical state.
- **Simulated:** produced by a simulation scenario.
- **Replayed:** emitted or processed as part of replay.

An event can need more than one descriptor (for example, a calculated result generated during replay). The vocabulary and combination rules are open. This classification supports the observed-versus-potential-impact distinction in ARCH-018.

### 11.2 Safety class

Candidate safety classes:

- Informational.
- Reasoning.
- Validation.
- Policy.
- Action.
- Mission-critical.

These labels are illustrative and not an approved ordering. Safety class governs handling constraints such as replay, authorization, routing, persistence, and audit. It is **not** operational severity.

### 11.3 Severity

Severity represents urgency or impact level as defined by the relevant domain or incident policy. It must remain separate from safety class: an informational observation may describe a severe outage, while an action event may be safety-critical even if its expected customer impact is low.

The severity model and required fields are out of scope for this event RFC and must align with the entity, mission, and policy contracts.

## 12. Freshness and locality

### 12.1 Freshness

Freshness is the age and applicability of information relative to a particular evaluation. The same observed value may be useful for one decision and stale for another. Event occurrence and publication timestamps are inputs to freshness assessment, but timestamps alone do not define freshness.

Where relevant, events or consuming evaluations should be able to represent source time, evaluation time, and freshness context/limitations. No universal expiry duration or freshness algorithm is defined. Freshness must not be conflated with source trust (whether the source is reliable) or confidence (support for a conclusion).

### 12.2 Regional locality

Regional deployments require events to preserve locality when applicable. At minimum, the contract should allow the system to distinguish:

- **Observation region:** where the underlying observation was made.
- **Derivation region:** where the event/result was produced.
- **Evaluation region/scope:** which region or set of regions the result assesses.
- **Data origin:** whether inputs were locally observed or replicated from another region.

These locations may differ. A locally derived global mission assessment, for example, may use observations from multiple regions. Region references must use RFC-0003 entity conventions. This RFC does not choose regional bus topology, routing, replication, or federation behavior; those belong in the regional architecture RFC and ARCH-022.

## 13. Replay safety

Replay is needed for incident reconstruction, testing changed engines against historical inputs, and investigation. Replay must never be mistaken for a new live occurrence or accidentally cause production execution.

Candidate replay classes:

- **Safe replay candidates:** observations and derived reasoning results such as pressure, health, impact, and mission. Safety still depends on consumer behavior and replay context.
- **Controlled replay candidates:** investigation and validation events, which may start work or consume resources and therefore require explicit scope and controls.
- **Restricted replay candidates:** policy, remediation, and execution-related events. Replaying them must not recreate approval, authorization, or production side effects. Any exceptional use requires an explicit, audited authorization.

These categories are proposals, not an automatic safety guarantee. Event class alone cannot make arbitrary consumers safe; consumer contracts must declare replay behavior.

Every replay operation should have a replay identity/context, selected input scope, operator or initiating process, and recorded start/completion or failure. Replayed outputs must retain their replay provenance and not overwrite live events. Side-effecting consumers must distinguish replay from live processing and default to no production side effects unless explicitly authorized under RFC-0010.

## 14. Bus behavior

The bus contract must eventually specify:

- **Publish:** validation, acceptance result, event identity, and producer-visible errors.
- **Subscribe:** subscription lifecycle, consumer identity, and filtering.
- **Filter and route:** matching rules, authorization, fan-out, and routing-cycle protection.
- **Delivery:** at-most-once, at-least-once, or other semantics and how consumers handle duplicates.
- **Ordering:** scope and guarantees, if any; global ordering must not be assumed.
- **Backpressure:** bounded buffering, admission behavior, and reporting under overload.
- **Consumer failure:** retry policy, poison-event handling, isolation, and recovery.
- **Replay:** selection, ordering, rate controls, replay identity, safety classes, and side-effect suppression or authorization.
- **Correlation:** relation between events for the same incident or workflow without assuming correlation means causation.
- **Derivation lineage:** input references and producing engine/configuration context for derived events.

No delivery or ordering guarantee is approved by this draft.

## 15. Replay, retention, and provenance

Replay is needed to reconstruct incidents, test changed engines against historical inputs, and support investigations. A replay should retain or reference the original event data and relevant schema and engine/configuration versions where available. Reprocessing must be distinguishable from live processing.

The source context suggests hot, warm, and cold retention tiers. This RFC records the tiering concept but does not choose durations, storage media, compression, access controls, or deletion requirements.

Provenance should make it possible to ask:

- Who or what produced this event?
- Which prior events or evidence contributed to it, and are the links correlations, direct causes, or derivation inputs?
- Which operational entities does it concern?
- Which region observed it, which region derived it, and what scope does it assess?
- Was its input data local, replicated, or mixed?
- Which engine and configuration version derived it?
- When did the source observation occur and when was it processed?
- Is the result observed, calculated, predicted, simulated, corrected, or replayed?
- Is it an observation/claim or a derived conclusion?
- Is the information fresh enough for the current evaluation, and what limits that judgment?
- What confidence applies to the conclusion, and what source-trust information applies to the evidence?
- Was this a live event, a correction, or replay?

Exact provenance graphs, retention and privacy constraints, and deterministic replay requirements are open.

## 16. Platform self-observation

The event bus and runtime should publish their own health events so Perceptron SRE can reason about its ability to observe and respond. Candidate event types include:

- `eventbus.backpressure`
- `eventbus.consumer_lag`
- `eventbus.storage_failure`
- `eventbus.replay_started`
- `eventbus.replay_completed`
- Engine lifecycle and health events.
- Knowledge-store integrity/availability and twin-health events.

Names and payloads are examples, not a namespace commitment. Self-observation must avoid unbounded recursive event generation when health events themselves trigger analysis. Loop prevention and independent health reporting remain open.

## 17. Versioning and compatibility

Event types and payload schemas require explicit versions. The compatibility policy must define additive and breaking changes, consumer handling of unknown versions or fields, deprecation windows, and historical decoding. No wire format or compatibility scheme is selected.

Event envelope evolution and payload schema evolution may need separate versioning; this remains to be decided.

## 18. Failure and safety behavior

The implementation must expose failures for invalid schema, unauthorized publication/subscription, unavailable storage or transport, consumer failures, replay failures, and backpressure. It must not silently discard safety-relevant events or report successful delivery when acceptance is unknown.

The bus alone cannot guarantee action safety. Replay of action-related events must not automatically re-execute production changes. Side-effecting consumers require explicit policy-controlled behavior defined jointly with RFC-0010.

## 19. Observability

At minimum, the bus should report event acceptance/rejection, publication and consumption rates, consumer lag, processing failures, replay activity, backpressure, and health through metrics/logging and suitable platform-health events. The core should make all current-cycle findings available as a machine-readable bulk report and expose bounded metrics suitable for a Prometheus scraper such as Grafana. Arbitrary event payloads must not be mapped wholesale into metric labels; cardinality and privacy limits apply. Legacy telemetry ingestion and export belong behind versioned adapters and must not make the event model vendor-specific. Metric names, labels, cardinality controls, event thresholds, and alert thresholds are defined by RFC-0012.

## 20. Acceptance criteria for an accepted RFC

- Event envelopes and at least one observation and derived-event contract are versioned and validated.
- Publication, subscription, filtering, and correlation behavior have executable tests.
- Correlation, direct causation, and derivation are represented distinctly.
- Entity references are supported and validated against the accepted entity-reference contract.
- Derived events preserve lineage to contributing evidence or explicitly mark lineage unavailable.
- Observation claims are distinguishable from calculated, predicted, or simulated conclusions.
- Events communicate sourced claims and do not imply universal truth; conflicting-source claims can coexist.
- Freshness can be assessed without conflating age, source trust, and confidence.
- Regional observation, derivation, and evaluation locality can be represented independently where applicable.
- Human validation, approval, escalation, and override can be recorded with actor and authorization provenance.
- Observed, calculated, predicted, simulated, and replayed results cannot be confused in relevant output contracts.
- Safety class is distinct from operational severity and affects replay/side-effect handling.
- Duplicate, malformed, unknown-version, consumer-failure, and backpressure behavior is explicit and tested.
- A bounded incident replay can be performed and is distinguishable from live traffic.
- Replay cannot cause production side effects without explicit policy authorization; restricted classes are tested.
- Event provenance can be inspected for a derived result.
- Derived decisions can be explained from event lineage and preserved provenance records, subject to documented source/retention limits.
- At least one complete incident timeline can be reconstructed six months after occurrence using event lineage, causation links, entity references, and preserved provenance, within the approved retention and privacy policy.
- Event bus self-observation reports failures and backpressure without unbounded feedback loops.
- Consumers cannot mutate the producer's accepted event; corrections are represented by new events.
- Retention, deletion, and security behavior are documented for the selected deployment.

## 21. Open decisions

1. Which transport is used for the first single-binary core and for distributed deployments?
2. Which delivery and ordering guarantees are required per event class?
3. Which event classes require durable storage, and for how long? What retention/deletion policy preserves enough linked records to reconstruct designated incident timelines at least six months after occurrence?
4. What are the canonical event ID, correlation ID, causation ID, incident ID, and explicit `derives_from` semantics?
5. How are schema registration and compatibility managed?
6. What is the entity-reference envelope, and how are tenant and namespace applied?
7. Which event safety classes exist, who assigns them, and what constraints follow from each?
8. Which epistemic/reasoning-kind labels are required, and can an event carry multiple kinds?
9. What replay classifications and consumer safeguards are normative?
10. What privacy, deletion, tenant, and authorization requirements constrain event retention?
11. How should routing cycles, fan-out, and self-observation feedback loops be detected and controlled?
12. How are confidence domains and source trust represented without conflation?
13. What freshness metadata is required, and which component evaluates freshness for each decision?
14. What regional identity and locality fields are required, and how is mixed local/replicated provenance represented?
15. How are human-originated decisions authenticated, attributed, and linked to policy approvals?
16. What limits apply to explainability when source records are unavailable, redacted, or expired?

## 22. Dependencies

The event contract is foundational to every engine and integration. RFC-0002 depends on it. RFC-0003 defines entity and region references; RFC-0004 defines evidence, confidence domains, freshness use, and source trust; RFC-0010 governs side effects and human approval records. RFC-0005 and RFC-0006 depend on metadata and replay behavior. RFC-0007 through RFC-0009 depend on mission, investigation, and knowledge event lineage. Regional routing, replication, and global federation are explicitly deferred to RFC-0011.
