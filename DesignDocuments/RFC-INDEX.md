# Perceptron SRE RFC Index

**Status:** Initial draft index  
**Parent architecture:** [Overall Architecture](01-overall-architecture.md)

These RFCs turn the architecture into reviewable engineering contracts. RFC-0001 through RFC-0011 are **Architecture review ready**. RFC-0012 is a new **Proposed architecture update** reflecting the engine Store and telemetry interop requirements; it requires review. None is accepted as an implementation requirement until reviewed and approved. Where the source concepts do not choose an implementation detail, the RFC records an open decision rather than inventing one.

## Proposed implementation order

This order is the dependency order proposed in `Context/rfcContext`. It is not a commitment to parallelize or begin production integration before safety and security requirements are met.

| Order | RFC | Contract | Main dependency / note |
|---:|---|---|---|
| 1 | [RFC-0001](RFC-0001-event-bus-and-event-model.md) | Event model and event bus | Foundational input/output and provenance contract. |
| 2 | [RFC-0002](RFC-0002-engine-runtime.md) | Engine lifecycle and execution | Depends on event and engine boundary decisions. |
| 3 | [RFC-0003](RFC-0003-operational-entity-model.md) | Operational entity and relationship model | Establish identity, state, time-bound relationships, provenance, and references used by reasoning. |
| 4 | [RFC-0004](RFC-0004-confidence-and-evidence.md) | Evidence and confidence | Establish explanation and uncertainty conventions. |
| 5 | [RFC-0010](RFC-0010-policy-engine.md) | Safety governance | Must precede any production execution capability. |
| 6 | [RFC-0008](RFC-0008-investigation-framework.md) | Investigation workflow | Consumes events, entities, evidence, and policy constraints. |
| 7 | [RFC-0009](RFC-0009-knowledge-engine.md) | Organizational memory and trust | Supplies versioned knowledge and trust evidence to investigation. |
| 8 | [RFC-0006](RFC-0006-twin-architecture.md) | Twin and validation model | Depends on evidence, event, policy, and domain integration contracts. |
| 9 | [RFC-0005](RFC-0005-traffic-handler.md) | DNS traffic boundary | DNS-specific and subject to fast-path, security, and operational validation. |
| 10 | [RFC-0007](RFC-0007-mission-assurance.md) | Mission assessment and disposition | Consumes operational signals, impact, confidence, and platform health. |

The order is not strictly serial: reviews can overlap, but downstream contracts must not silently assume unresolved upstream semantics.

## Follow-on RFCs

- [RFC-0011: Regional Architecture and Federation](RFC-0011-regional-architecture-and-federation.md) — planned contract for regionalization boundaries, event locality, queue/control-plane availability, work ownership, federation, and mission aggregation. RFC-0001 carries only the event-locality fields needed to avoid foreclosing distribution; this RFC owns the detailed regional design.
- [RFC-0012: Engine Store and Telemetry Interoperability](RFC-0012-engine-store-and-telemetry-interoperability.md) — proposed contract for engine package metadata, local/third-party catalog behavior, action-policy declarations, and legacy telemetry ingestion/export.

## Architecture diagrams

The standalone [diagram catalog](diagrams/README.md) contains twelve Mermaid views for architecture boundaries and intended use cases. Each applicable RFC links directly to its related diagrams. The diagrams are illustrative and do not supersede RFC requirements or resolve open decisions.

## RFC lifecycle

1. **Draft:** captures intent, candidate contracts, and open decisions.
2. **Review:** owners and stakeholders resolve questions or explicitly accept deferrals.
3. **Accepted:** normative requirements are distinguished from rationale and examples; implementation and test criteria are sufficiently precise.
4. **Superseded or withdrawn:** the RFC links to its replacement or records why the proposal was abandoned.

An accepted RFC should record its decision date, reviewers/owners, compatibility policy, and relationship to architecture decision IDs. No draft in this index is currently represented as accepted.

## Common contract expectations

Each RFC should state:

- Purpose, problem, goals, and non-goals.
- Terminology and relationships to other RFCs.
- Inputs, outputs, interfaces, and versioning.
- Normal behavior and relevant lifecycle.
- Errors, missing or contradictory data, and degraded behavior.
- Security, privacy, and operational safety implications.
- Observability and audit needs.
- Testable acceptance criteria.
- Decisions made, decisions deferred, and dependencies.

Examples and candidate schemas are illustrative until explicitly made normative.
