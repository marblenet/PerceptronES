# RFC-0011: Regional Architecture and Federation

**Status:** Architecture review ready  
**Category:** Cross-cutting architecture  
**Parent:** [Overall Architecture](01-overall-architecture.md)  
**Related RFCs:** RFC-0001 Event Bus and Event Model; RFC-0003 Operational Entity Model; RFC-0004 Confidence and Evidence; RFC-0006 Twin Architecture; RFC-0007 Mission Assurance; RFC-0010 Policy Engine

**Visual references:** [Regional federation and degraded operation](diagrams/D-10-regional-federation.md); [system context and trust boundaries](diagrams/D-01-system-context.md).

## 1. Purpose

This RFC defines the regional architecture questions and candidate contracts for Perceptron SRE and its DNS reference environment. It addresses what must be regionalized, how regional observations, work, state, and decisions are owned or exchanged, and how they contribute to global mission assurance.

This RFC does not select an approved topology or availability design. RFC-0001 defines event locality/provenance needs; it does not prescribe one bus per region, one global bus, or a federation mechanism. Multi-region operation does not by itself imply that every control-plane subsystem must be active-active.

## 2. Established context

The DNS design envisions multiple simultaneously active regions rather than a primary/standby model. Example scenarios include regional health differences, zone/database divergence, regional propagation failures, regional failure simulations, and global mission assessment.

The number of regions and their deployment topology are examples from the reference environment, not a requirement for the first Perceptron SRE core binary.

## 3. Goals for the completed RFC

- Define regional identity and scope across event, entity, and mission contracts.
- Distinguish observation region, derivation region, evaluation region, and data origin.
- Define regional event routing, replication, filtering, and locality.
- Identify which subsystems require regional operation, federation, replication, or may remain local.
- Compare high availability of the work queue/coordination path with high availability of the entire control plane.
- Define ownership, leasing, and execution authority for work across regions.
- Distinguish observation, derivation, evaluation, authorization, and execution regions.
- Describe operation during regional partition, loss, recovery, and rejoin.
- Define active-active assumptions and how divergence is detected and represented.
- Define how regional assessments contribute to global mission assurance.
- Define twin and simulation locality and their relationship to regions.
- State security, privacy, audit, and policy boundaries across regions.

## 4. Topics to resolve

### 4.1 Regional identity and locality

Define canonical region IDs, aliases, lifecycle, and the relationship between physical/geographic regions and logical failure domains. Align with RFC-0003. Preserve local-versus-replicated provenance in events as specified by RFC-0001.

### 4.2 Event and state topology

Evaluate alternatives without presuming one:

- Independent regional event buses with selected global federation.
- A shared/global event fabric with regional locality controls.
- Hybrid models with local processing and a global mission-assurance layer.

Specify routing scope, replication filters, ordering assumptions, duplicate handling, retention ownership, and behavior when links are unavailable.

### 4.3 Active-active operation and divergence

Define what “active-active” means for Perceptron SRE and for the DNS environment. Distinguish independently owned local state from replicated state. Define how conflicting regional claims are represented and reconciled without silently overwriting evidence.

### 4.4 Mission aggregation

Define local and global mission scopes, aggregation semantics, impact precedence, missing-region behavior, stale regional information, and how a severe local impact remains visible in a global assessment.

### 4.5 Partitions, recovery, and split-brain

Specify what a region may observe or decide while disconnected; whether it may recommend or execute actions; how action authorization avoids unsafe duplication; how buffered events are reconciled; and what state is authoritative after rejoin.

### 4.6 Twin and simulation placement

Specify whether twins are regional, global, or both; how their configuration/data locality is recorded; how cross-region changes are tested; and how simulations model region loss, traffic shifts, and divergence.

### 4.7 Security and policy

Define identity and authorization across regional boundaries, tenant/data residency constraints, policy distribution and versioning, audit locality, and emergency controls during federation failure.

### 4.8 Regionalization boundaries and availability scope

Regional deployment is a per-subsystem decision, not a binary property of the whole control plane. The completed design should classify the availability and locality needs of, at minimum:

- Change APIs and request acceptance.
- Databases and authoritative state.
- Work queues and coordination.
- Workers and schedulers.
- Builders and propagation components.
- Regional DNS serving/data-plane components.
- Perceptron event processing and regional reasoning.
- Global mission-assurance and policy services.

For each subsystem, document whether it is region-local, replicated, federated, shared, or intentionally single-region; its state owner; dependencies; failure behavior; and recovery/reconciliation contract. These are candidate classifications, not topology selections.

Regionalization decisions should follow mission impact rather than apply uniform availability targets. Different subsystems may require different continuity objectives: for example, DNS serving may require active-active continuity, a change API may tolerate temporary regional loss, and mission assurance may continue with explicitly reduced coverage. These are illustrative distinctions, not selected SLOs or required behaviors.

A highly available distributed work queue or coordination layer may preserve accepted work and enable a subset of workflows to continue without making every API, database, worker, builder, and control-plane service active-active. Conversely, queue availability alone does not prove that the complete control plane can safely accept, process, build, or propagate changes. The architecture must explicitly compare these scopes rather than treating “HA queue” and “HA control plane” as equivalent.

The key open question is whether continuity requires a highly available/federated queue with region-local consumers, an active-active control plane, or a hybrid. This trade-off affects complexity, consistency, cost, operational ownership, and failure recovery (ARCH-042).

Regional read access and regional write authority must be analyzed separately. A design may allow every region to read shared or replicated state while limiting writes to one owner, or it may permit writes in multiple regions under an explicit consistency and conflict-resolution contract. Read federation does not imply write federation, and neither determines execution authorization. The required read/write model for each subsystem remains open.

For each major subsystem, the completed design must state its availability scope, ownership scope, replication model, failure behavior, recovery behavior, and rationale for requiring or not requiring active-active operation. It must also identify which end-to-end workflows remain possible when a dependency is unavailable; component availability alone does not establish workflow availability (ARCH-045).

### 4.9 Work ownership and regional execution authority

When work is visible to multiple regions, the system must define which region or worker owns each work item at a given time. Candidate mechanisms include explicit ownership, leases, claims, fencing, or another coordination contract; no mechanism is selected here.

The contract must address:

- Whether work may be claimed or executed concurrently by multiple regions.
- How ownership is acquired, renewed, transferred, expired, and fenced.
- What happens when a worker or its region fails after claiming work.
- How idempotency, retries, duplicate delivery, and partial completion are recorded.
- Which region is permitted to execute a particular item and which system authorizes that execution.
- How queued work is continued, transferred, or deliberately paused during control-plane failure.

Queue replication, work ownership, regional execution authority, and production authorization are related but distinct. A replicated item is not an execution grant, and a lease alone is not policy authorization. These semantics and their relationship to queue topology remain open (ARCH-043).

### 4.10 Regional provenance for decisions and actions

Regional records should distinguish, where applicable:

- **Observation region:** where the underlying fact or signal was observed.
- **Data-origin region:** where the source data originated or is owned.
- **Derivation region:** where a derived result was computed.
- **Evaluation region:** where a mission or policy assessment was evaluated.
- **Authorization region/authority:** where, or under which regional/global authority, the action was authorized.
- **Execution region:** where the action was carried out.

These are provenance dimensions, not necessarily distinct physical locations. A decision may combine evidence from several regions; the record must retain relevant locality and policy/version context so operators can determine which evidence was evaluated and which authority permitted execution. The authority model during federation loss or disagreement remains open (ARCH-044).

## 5. Candidate scenarios and acceptance criteria

The completed RFC should include testable scenarios for:

- One region reporting service health while another reports failure.
- Database or zone-count divergence across active regions.
- A regional partition followed by event and state reconciliation.
- Global mission assessment with a missing or stale region.
- A global action proposal during partial regional connectivity.
- Regional twin validation and simulated regional failure.
- Preventing duplicate or conflicting production actions across regions.
- Regional control-plane failure while the serving plane remains healthy, with queued work continued, transferred, or intentionally paused according to documented ownership and authorization rules.
- A work item visible to multiple regions, demonstrating that concurrent duplicate execution is prevented or handled by an explicitly safe, auditable contract.
- A decision whose observation, derivation, evaluation, authorization, and execution occur in different regions, with each locality represented in its provenance.

Acceptance requires the completed RFC to identify regionalization boundaries for the named subsystems; distinguish queue/coordination availability from whole-control-plane availability; define work ownership and failure/recovery behavior; preserve regional decision/action provenance; and test local/global mission behavior under missing, stale, divergent, and partitioned regional inputs. These are criteria for a future accepted RFC, not assertions that the mechanisms are selected now.

- For each major subsystem, document availability scope, ownership scope, read/write federation, replication model, failure behavior, recovery behavior, and the mission-impact rationale for its continuity objective.
- Identify the minimum subsystem capabilities and dependencies required to accept, process, validate, authorize, and execute each in-scope workflow, including degraded or unavailable outcomes.

## 6. Open decisions

1. What regional topology is required for the first distributed deployment?
2. What must be regionalized or federated, and what may remain local or single-region?
3. Is continuity provided by an HA/federated work queue, an HA control plane, or a hybrid, and which workflows does each support (ARCH-042)?
4. Is the bus regional, global, federated, or hybrid?
5. Which events and state are replicated, and with what delivery and ordering semantics?
6. Which system is authoritative for each regional and global state transition?
7. How are work ownership, leases/fencing, transfer, retries, and duplicate execution controlled across regions (ARCH-043)?
8. What actions, if any, are permitted during loss of federation, and which regional/global authority can authorize them (ARCH-044)?
9. How are split-brain, duplicate actions, and divergent regional policies handled?
10. What is the global mission aggregation model?
11. What privacy, data-residency, and retention constraints apply by region?
12. How are twins and simulations scoped across regions?
13. What is the minimum set of subsystems and dependencies that must remain available to continue accepting, processing, validating, authorizing, and executing each supported workflow (ARCH-045)?
14. For each subsystem, what read-federation and write-federation model is required, and how are write ownership and conflicts handled?

## 7. Dependencies

This RFC depends on the event, entity, evidence, policy, twin, and mission contracts. Its decisions must remain consistent with ARCH-010, ARCH-017, and ARCH-022 and must not redefine event-level locality fields without updating RFC-0001.
