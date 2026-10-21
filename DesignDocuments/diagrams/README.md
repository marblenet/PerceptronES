# Perceptron SRE Architecture Diagrams

This directory contains standalone Mermaid diagrams used by the architecture RFCs. They are navigational views of the written contracts, not independent normative specifications. Read the linked RFC for requirements, failure behavior, open decisions, and acceptance criteria.

Diagrams are intentionally separated by question/use case so reviewers can inspect or reuse them individually. Unless a diagram explicitly describes existing v0.1 behavior, it depicts proposed architecture or an illustrative workflow. Boxes do not imply deployment boundaries, and arrows do not imply synchronous calls or a selected transport.

| ID | Diagram | Main question/use case | Related RFCs |
|---|---|---|---|
| D-01 | [System context and trust boundaries](D-01-system-context.md) | What is inside the core, what is external, and where do telemetry and actions cross boundaries? | Overall, RFC-0001, RFC-0002, RFC-0010, RFC-0012 |
| D-02 | [Reasoning cascade and event lineage](D-02-reasoning-cascade.md) | How do source observations become traceable operational assessments? | RFC-0001, RFC-0002, RFC-0004, RFC-0007 |
| D-03 | [Entity and impact graph](D-03-entity-impact-graph.md) | How can a component signal be related to services, customers, and mission scope? | RFC-0003, RFC-0005, RFC-0007 |
| D-04 | [Evidence and confidence lifecycle](D-04-evidence-confidence.md) | How do supporting, contradictory, missing, and stale evidence affect a claim? | RFC-0004, RFC-0006, RFC-0008, RFC-0010 |
| D-05 | [DNS traffic handler fast and slow paths](D-05-dns-traffic-handler.md) | How can the DNS service fast path remain independent of slow reasoning? | RFC-0005, RFC-0001, RFC-0007 |
| D-06 | [Investigation and knowledge workflow](D-06-investigation-knowledge.md) | How does an incident trigger evidence gathering, hypotheses, recommendations, and learning? | RFC-0008, RFC-0009, RFC-0004, RFC-0010 |
| D-07 | [Twin validation and promotion](D-07-twin-validation.md) | How can a candidate change be tested away from production before a separate policy decision? | RFC-0006, RFC-0010 |
| D-08 | [Policy, authorization, and action lifecycle](D-08-policy-action-lifecycle.md) | What gates separate a recommendation from an executed and verified operation? | RFC-0010, RFC-0002, RFC-0006 |
| D-09 | [Legacy telemetry interoperability](D-09-legacy-telemetry.md) | How can Perceptron consume legacy metrics and return useful findings to the same observability stack? | RFC-0001, RFC-0002, RFC-0012 |
| D-10 | [Regional federation and degraded operation](D-10-regional-federation.md) | What evidence remains local, what can be federated, and how is a partition represented? | RFC-0001, RFC-0003, RFC-0007, RFC-0011 |
| D-11 | [Incident report time window](D-11-incident-time-window.md) | What records are required for one minute before an incident through five minutes after mitigation? | RFC-0001, RFC-0004, RFC-0008, RFC-0009, RFC-0010 |
| D-12 | [Engine package, Store, and trust lifecycle](D-12-engine-store.md) | How are engines contributed, reviewed, configured, enabled, and governed? | RFC-0002, RFC-0010, RFC-0012 |

## Rendering

The diagrams use Mermaid flowcharts and sequence diagrams in Markdown. A renderer must support Mermaid. Keep the source diagrams in this directory and link to them from RFCs rather than copying diagram source into multiple documents. When a contract changes, update both the relevant RFC and its source diagram in the same review.
