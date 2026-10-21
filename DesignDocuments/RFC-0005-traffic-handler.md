# RFC-0005: DNS Traffic Handler

**Status:** Architecture review ready  
**Category:** DNS integration  
**Parent:** [Overall Architecture](01-overall-architecture.md)  
**Related RFCs:** RFC-0001 Event Bus and Event Model; RFC-0003 Operational Entity Model; RFC-0004 Confidence and Evidence; RFC-0006 Twin Architecture; RFC-0007 Mission Assurance; RFC-0010 Policy Engine

**Visual references:** [DNS traffic handler fast and slow paths](diagrams/D-05-dns-traffic-handler.md); [entity and impact graph](diagrams/D-03-entity-impact-graph.md).

## 1. Summary

This RFC defines the proposed functional boundary and responsibilities of the DNS Traffic Handler: a customer-aware traffic regulator placed before DNS service infrastructure. Its primary design objective is preserving legitimate customer DNS service; measuring and regulating traffic and producing operational metadata support that objective. The handler is not itself the authoritative DNS service.

The ownership, deployment, protocols, packet-processing technology, exact latency budget, and fail-open/fail-closed behavior remain open.

## 2. Problem and motivation

In a conventional DNS path, customer identification, billing, security analysis, pressure measurement, and analytics may be reconstructed later from logs or embedded in DNS-serving components. The Traffic Handler concept observes a request near the edge, identifies customer context where possible, extracts compact metadata once, and distributes it to consumers without placing complex analysis on the response path.

The design prioritizes preserving legitimate customer DNS service during overload rather than treating all traffic as equivalent. Customer experience is the primary objective; analytics, investigation, classification, billing, and telemetry are secondary and must not become dependencies that prevent DNS service.

## 3. Goals

- Define the Traffic Handler's role between clients and DNS infrastructure.
- Identify functional taps/components and the metadata they produce.
- Separate synchronous DNS-path work from asynchronous reasoning.
- Describe known-customer and unknown-traffic lanes.
- Specify how identity confidence and regional traffic observations are represented.
- Feed metrics, billing, security, twins, simulation, and Perceptron SRE through bounded metadata interfaces.
- Make pressure regulation and overload behavior explicit and testable.
- Define candidate degraded modes and how they preserve the DNS service path.

## 4. Non-goals

- Replacing `dnsdist`, BIND, authoritative DNS, or the DNS control plane.
- Performing customer-impact analysis, knowledge search, or mission assurance synchronously per query.
- Duplicating full packets to every analytical consumer as the default design.
- Selecting a network appliance, proxy, eBPF/XDP implementation, or language without measurement.
- Guaranteeing service for every customer request under every failure condition.

## 5. Functional boundary

Conceptually:

```text
Client traffic
     ↓
Traffic Handler
     ├── bounded DNS forwarding path ──► DNS service (e.g. LB, dnsdist, BIND)
     ├── request metadata ──────────────► billing / metrics / security
     ├── request metadata ──────────────► Perceptron SRE
     ├── optional mirrored requests ────► data-plane twin (response discarded)
     └── scenario/telemetry inputs ─────► simulation
```

The Traffic Handler's exact ownership—Perceptron subsystem, separate service, or DNS-environment integration—is open (ARCH-004). Its functional position and the requirement not to block DNS on slow analysis are established design intent.

The Traffic Handler is a **traffic regulator**, not merely a firewall, load balancer, or proxy. Its combined purpose is to protect service, measure traffic and operational pressure, regulate according to explicitly authorized policy, and distribute operational metadata. It does not replace the DNS service or acquire authority over DNS data-plane behavior by virtue of observing requests.

**Primary objective:** preserve legitimate customer DNS service. Analytics, investigation, classification, billing, and telemetry are secondary objectives. This is a prioritization principle, not a guarantee that every request can be served under every failure or attack condition.

The Traffic Handler is also an observation source for the Perceptron reasoning cascade. Traffic-derived pressure, behavior, threat, customer-attribution, and regional signals may feed downstream health, impact, investigation, and mission-assurance reasoning. It should provide early traffic and pressure observations, not perform the full reasoning chain synchronously.

## 6. Candidate internal responsibilities

The source material describes a set of independently testable functions:

- **Customer Tap:** identify known customer context and service tier where available.
- **Behavior Tap:** extract behavior signals such as source/traffic characteristics for threat analysis.
- **Validation Tap:** perform bounded request validation.
- **Query classification:** classify relevant DNS query properties and types.
- **Metadata Engine:** produce compact metadata for downstream consumers.
- **Pressure Engine:** sample traffic and service pressure inputs and derive or publish pressure signals.
- **Distribution Engine:** forward the DNS request and route metadata to consumers.
- **Cache-related processing:** provide cache-related signals or routing as supported by the eventual design.

This list consolidates concepts from the traffic-handler materials; it does not require one process or exactly six deployable services. Component ownership and order must be specified in implementation RFCs.

## 7. Customer lanes and identification

The proposed classification separates:

- **Attributed with high identity confidence:** traffic mapped to a customer under the configured identity rules and confidence criteria.
- **Attributed with low identity confidence:** a customer mapping exists, but its reliability is limited or uncertain.
- **Unattributed traffic:** customer ownership is not established.

Unattributed does not mean malicious. Low-confidence attribution is not equivalent to either high-confidence identification or no attribution; the handler must preserve that distinction for regulation, reporting, and audit. Confidence semantics and lane-assignment rules are open (ARCH-029).

Identity may be uncertain or spoofable depending on network context. The intended overload posture is to preserve legitimate customer service using mechanisms such as reserved capacity, deeper cache, or reduced inspection, while applying appropriately stricter scrutiny or controls to unattributed traffic. High-confidence customer attribution does not make abusive traffic safe or exempt it from protection. Service-tier semantics, fairness, abuse handling, lane limits, transitions, and the consequences of low-confidence attribution must be specified. Policy must prevent classification from becoming an unreviewed authorization to bypass protection.

## 8. Query classification and metadata

Candidate DNS query classes named in the source material include A, AAAA, MX, TXT, ANY, DNSSEC-related queries, AXFR, and IXFR. This list is illustrative and must be checked against supported protocol behavior and security needs.

Metadata may include customer reference, region, zone/name, query type, timestamp, and other bounded attributes needed by consumers. The architecture favors extracting metadata once over copying full packets into each analytical pipeline.

The exact schema, treatment of query names and other sensitive data, data minimization, identifier mapping, sampling, retention, and consumer authorization are open. Metadata extraction must not alter DNS semantics unless an explicit, authorized traffic policy does so.

Operational metadata is a product of the handler, not incidental packet logging. The intended pattern is **extract once, use many times**: publish bounded, versioned metadata for authorized consumers such as billing, security, metrics, investigation, impact analysis, simulation, and mission assurance. Traffic observations should identify their provenance and observation method where known, such as direct observation, mirrored traffic, aggregation, sampling, or estimation. Scope and sampling information must be retained so consumers do not treat partial or derived observations as equivalent to complete direct observation; confidence implications follow RFC-0004. Consumer unavailability or downstream backpressure must not block DNS responses; buffering, sampling, shedding, and loss reporting remain to be specified.

## 9. Pressure management and protection

Candidate pressure inputs include QPS, packet rate and size, CPU, latency, queue depth, cache misses, and network utilization. Source material also mentions source entropy and simple fast-path DDoS heuristics.

Pressure should represent operational strain in context, not raw utilization alone. Inputs, sampling windows, thresholds, pressure states, and control actions are not defined here. Example states GREEN/YELLOW/ORANGE/RED are not a Traffic Handler control algorithm.

DDoS posture in the concept is customer-aware: protect known customer service and regulate unknown traffic during attack conditions. Detection, mitigation actions, false-positive handling, escalation, audit, and emergency override need separate policy and security review.

This customer-aware posture is a defining design intent, not a guarantee that all attributed requests are legitimate or that unattributed requests are hostile. During an attack, the handler should preserve service for legitimate customers while applying policy-authorized, increasingly strict scrutiny or regulation to unattributed traffic. Exact controls, thresholds, identity-confidence requirements, fairness, and rollback behavior remain open.

Traffic and pressure observations should retain regional identity and scope so that regional traffic patterns, pressure, protection, and degradation can be assessed separately and consumed by regional or global reasoning. This RFC does not define regional federation, routing, or aggregation; those belong to RFC-0011.

## 10. Fast-path contract

Established intent:

- DNS response processing remains fast and bounded.
- Customer-impact, SLA, knowledge, investigation, and simulation work are asynchronous and must not be waited on per DNS request.
- Analytical workloads and metadata consumers must never become dependencies of DNS service or block the DNS response path.
- Packet handling should do only the classification, validation, sampling, and decisions required for the immediate path.

This is a design invariant across implementations, including when Perceptron SRE, the metadata bus, or analytical consumers are unavailable. The source material expresses a microsecond-scale aspiration but provides no benchmark definition or numeric SLO. The RFC must define what synchronous work is allowed and how metadata publication behaves when consumers are unavailable.

## 11. Failure and safety behavior

The Traffic Handler must define behavior for unknown customers, malformed requests, identity-service outage, metadata-bus outage, pressure-signal staleness, overloaded handler capacity, and DNS-backend failure.

No universal fail-open or fail-closed rule is selected. For each failure class, the design must document customer impact, security exposure, response-path behavior, observability, and policy owner. Failure to publish analytics metadata must not silently block DNS unless an explicitly approved safety/security requirement says otherwise.

### 11.1 Candidate degraded modes

The handler should define explicit operational modes that state which capabilities are available and which are sacrificed before customer DNS service. A candidate progression is:

| Mode | Candidate behavior | Design constraint |
|---|---|---|
| Normal | Classification, metadata production, pressure observation, and authorized protection are available. | Analytical work remains asynchronous from DNS response processing. |
| Degraded | Preserve the bounded DNS path while reducing optional classification or metadata detail and continuing only essential, available pressure/protection functions. | Lost coverage and metadata must be observable; omitted analysis must not be represented as a healthy finding. |
| Emergency | Prioritize service continuity and the minimum explicitly authorized protection behavior when handler or dependency capacity is critically constrained. | The allowed forwarding, regulation, and security posture must be defined by policy; no default fail-open or fail-closed behavior is implied. |

These are candidate modes, not an approved state machine or ordering of actions. Entry/exit conditions, hysteresis, capabilities retained, customer impact, operator visibility, and the handling of stale or missing signals remain open. The priority among service preservation, security, and regulation under each failure condition must be explicitly decided (ARCH-030).

## 12. Language and performance

C/eBPF/XDP are candidates for packet classification, metadata extraction, sampling, and simple fast-path heuristics. Go is preferred for higher-level distribution, state, policy integration, and platform runtime. Use the lowest-level language necessary; benchmark and threat-model the actual integration before choosing.

The boundary between low-level packet processing and Go, the handoff format, memory ownership, update mechanism, and supported operating systems remain open.

## 13. Acceptance criteria for an accepted RFC

- DNS path and asynchronous metadata path are separately specified.
- A slow or unavailable analytical consumer cannot block DNS responses.
- Customer and unknown-traffic classification have defined semantics, uncertainty handling, and audit.
- Customer identity confidence is distinct from high-confidence attribution, low-confidence attribution, and unattributed traffic; unattributed traffic is not presumed malicious.
- Customer-service preservation is the primary objective, and analytical workloads cannot become DNS-service dependencies.
- Query metadata is versioned, minimized, and access-controlled.
- Traffic observations preserve provenance and identify direct, mirrored, aggregated, sampled, or estimated origin where applicable, including relevant scope and sampling limitations.
- Traffic-derived observations preserve regional scope and can feed downstream pressure, health, impact, and mission reasoning.
- Pressure and overload behavior are testable, including stale/missing pressure inputs.
- Degraded modes and their capability trade-offs are explicit, observable, and do not imply fail-open/fail-closed defaults.
- Backpressure or unavailability of metadata consumers cannot block DNS responses, and metadata loss or shedding is observable.
- Benchmarks verify the accepted latency/throughput target under documented conditions.
- Twin mirroring cannot return twin responses to customers.

## 14. Open decisions

1. Who owns and operates the Traffic Handler?
2. What network placement and forwarding mechanism are used?
3. Which query operations and DNS protocol features are in scope?
4. How are customer identity, identity confidence, and lane assignment represented, validated, refreshed, and audited (ARCH-029)?
5. What are customer-lane guarantees, fairness policy, and abuse handling?
6. Which metadata fields and provenance descriptors are emitted, and which are prohibited or sensitive?
7. What work may block DNS, and what are the measurable latency and capacity budgets?
8. What is the overload policy when metadata distribution or Perceptron SRE is unavailable?
9. Which traffic controls are authorized by policy and how are they rolled back?
10. What packet-path work, if any, requires C/eBPF/XDP?
11. Which degraded operational modes exist, what functionality is sacrificed in each, and what priorities govern the trade-offs before customer service is sacrificed (ARCH-030)?
12. How are regional traffic, pressure, protection, and degradation observations represented and consumed without presupposing the RFC-0011 federation design?

## 15. Dependencies

RFC-0001 defines event and metadata publication contracts. RFC-0003 defines customer, zone, and region references. RFC-0004 informs identity and evidence confidence. RFC-0006 defines twin mirroring. RFC-0007 consumes traffic and customer-impact signals. RFC-0010 governs active protection and regulation.
