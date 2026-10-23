# Perceptron SRE Design Knowledge Base

## Purpose

This directory contains the architecture and implementation RFCs for Perceptron SRE. It is intended to be readable by engineers and AI coding agents and to distinguish agreed principles from proposed designs and decisions that still require discussion.

The implementation sequence is maintained separately in the [Implementation Roadmap](../Implementation/IMPLEMENTATION-ROADMAP.md). The roadmap stages implementation work and provisional decisions; it does not replace or implicitly accept the RFCs.

## Reading order

1. [Engineering Philosophy](ENGINEERING-PHILOSOPHY.md) — rationale behind the evidence, reliability, determinism, and safety guardrails.
2. [Overall Architecture](01-overall-architecture.md) — system purpose, boundaries, logical components, principal workflows, and architecture decisions still open.
3. [RFC index](RFC-INDEX.md) — status, dependency notes, and proposed implementation order for the first ten contracts.
4. RFC-0001 through RFC-0004 — event model, engine runtime, operational entities, evidence and confidence.
5. RFC-0010 — policy and safety governance.
6. RFC-0008 and RFC-0009 — investigation and organizational knowledge.
7. RFC-0006 — twin architecture.
8. RFC-0005 — DNS Traffic Handler.
9. RFC-0007 — mission assurance.
10. [RFC-0012: Engine Store and Telemetry Interoperability](RFC-0012-engine-store-and-telemetry-interoperability.md) — engine catalog, package governance, and legacy metrics interoperability.
11. [Engine Platform Addendum](ENGINE-PLATFORM-ADDENDUM.md) — tap engines, contributor proof contracts, event origins, human-operation records, and simulation boundaries.
12. Follow-on RFCs in the overall architecture roadmap — simulation, DNS integration, security, deployment, and other concerns.

## Architecture diagrams

See the [diagram catalog](diagrams/README.md) for standalone Mermaid diagrams covering system context, the event/reasoning cascade, entity impact, evidence and confidence, DNS fast/slow paths, investigation and knowledge, twin validation, policy/action safety, legacy telemetry integration, regional federation, incident time windows, and the engine Store lifecycle. Each diagram is linked from its related RFC and is explicitly illustrative unless the RFC states otherwise.

## Document conventions

- **Established principle** means directly stated in the source design material.
- **Proposed architecture** means a coherent structure for implementation derived from those principles; it is not a final decision until reviewed.
- **Open decision** means the available source material does not settle the matter. Do not implement one alternative as if it were agreed.
- **Illustrative example** means an example of intended behavior, not a fixed schema, threshold, service-level objective, or algorithm.

RFCs should identify the problem, goals and non-goals, interfaces, behavior, safety and failure handling, observability, compatibility and rollout, testable acceptance criteria, and unresolved questions. An RFC is not an implementation specification until its open decisions are resolved.

The RFC numbers are stable identifiers, not an assertion that every draft is approved. Read each document's status and open decisions before treating its proposed contracts as implementation requirements.
