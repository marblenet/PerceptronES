# RFC-0003: Operational Entity Model

**Status:** Architecture review ready  
**Category:** Domain model  
**Parent:** [Overall Architecture](01-overall-architecture.md)  
**Related RFCs:** RFC-0001 Event Bus and Event Model; RFC-0004 Confidence and Evidence; RFC-0005 Traffic Handler; RFC-0007 Mission Assurance; RFC-0008 Investigation

**Visual references:** [Entity and impact graph](diagrams/D-03-entity-impact-graph.md); [reasoning cascade and event lineage](diagrams/D-02-reasoning-cascade.md).

## 1. Summary

This RFC proposes a shared conceptual vocabulary and reference model for operational entities used across Perceptron SRE. Customer, zone, record, region, and incident are recurring concepts in the architecture and need consistent references. This draft does not claim that Perceptron SRE is the system of record for these entities or define a final database schema.

## 2. Problem and motivation

Impact analysis and investigation depend on connecting evidence to the things affected. If engines use incompatible identifiers or disagree about relationships, customer impact and cross-region reasoning become unreliable. The system must distinguish a stable reference to an entity from the authoritative owner of that entity's data.

## 3. Goals

- Establish candidate first-class operational entities.
- Define relationship semantics needed for impact and blast-radius reasoning.
- Keep canonical identity and data ownership explicit.
- Support event references, impact analysis, and incident reconstruction.
- Distinguish entity identity, state, and freshness.
- Preserve the provenance and time context of entities and relationships.

## 4. Non-goals

- Replacing customer, DNS, inventory, or CMDB systems of record.
- Declaring all listed fields mandatory.
- Selecting a database, global ID format, or tenancy architecture.
- Treating an entity's existence in Perceptron SRE as proof that the external resource is healthy or active.

## 5. Candidate entities

The following entities are identified by project context or additional architecture review. Attributes shown are candidate concepts, not final required schemas.

| Entity | Candidate attributes or relationships | Open concerns |
|---|---|---|
| Customer | Customer ID, service tier, region, service level; may own zones. | Canonical owner, tenant boundary, identity source, historical changes. |
| Zone | Zone ID, name, owner; contains records and is served across regions. | DNS naming rules, ownership lifecycle, aliases, delegation and region semantics. |
| Record | Record ID, zone, type, value and possibly region-specific state. | Sensitive data, versioning, TTL, record identity across updates, history. |
| Region | Region identity and relationships to service instances or infrastructure. | Global naming, failure domain, deployment topology. |
| Incident | Incident ID, severity, status, evidence, timeline, impacted entities, decisions, outcomes. | Ownership, severity model, lifecycle, links to external ticket systems. |
| Service | Service ID/type, owner, dependencies, regions, and service objectives/SLA references. | Service boundaries, decomposition, ownership, dependency source, and relationship to deployment/component entities. |
| Mission | Mission ID and the service/customer outcomes or scope being assured. | Whether mission is a persistent entity, a configured objective, or only an assessment scope; ownership and lifecycle. |
| Action | Action ID/type, target, status, validation, authorization, execution, and outcome references. | Action identity, lifecycle, idempotency, approval binding, and execution ownership. |
| Twin | Twin ID/type, represented system/scope, region, version/configuration, health, and fidelity metadata. | Twin lifecycle, synchronization, fidelity dimensions, and relationship to simulation environments. |
| Service tier | Tier identity/name and references to service level, policy, or customer allocation. | Whether tier is an entity or an attribute/value owned by the customer or service system. |
| Component / deployment | Component identity/type (for example, IXFR worker or DNS builder); deployment instance/scope (for example, IXFR worker in Region B). | Whether component and deployment are separate entities; relationship to service, region, runtime instance, and operational ownership. |

Service, mission, action, twin, service tier, component, and deployment are candidate concepts that need review; listing them does not make Perceptron SRE their system of record. Policies, SLAs, and other concepts may also need representation. The catalog is not exhaustive.

**Future modeling concern — coverage/scope:** an assessment may cover one region, entity set, input set, or time interval while omitting another. Coverage is not itself necessarily an entity; it may be a scope attached to an assessment or relationship traversal. RFC-0002 and RFC-0004 address engine coverage and confidence. This RFC records the domain-model concern without introducing a new entity class now.

**Business importance and service tier:** service tier and business importance may be related but are not assumed to be the same concept. A customer can have a contractual service tier and a separately governed business-importance designation. Impact and mission reasoning may need both; their ownership and representation remain open.

## 6. Identity and references

Each entity reference must be unambiguous within its stated namespace and stable enough for event correlation and historical reconstruction. The model must define:

- Canonical identifier owner and namespace.
- Whether IDs are globally unique or qualified by source/tenant.
- Alias and rename behavior.
- Reference behavior when the entity is unknown or deleted.
- Historical identity across migrations, splits, merges, or recreation.
- How external IDs map to internal references.

No identifier format is selected. Human-readable names should not be assumed to be stable identifiers.

## 7. Entity state, freshness, and time

Entity identity and entity state are separate. The existence of a customer, region, service, or twin does not establish that it is active, available, healthy, synchronized, or currently serving its intended role. State belongs to time-stamped claims and assessments, not to identity alone.

Entity attributes and references may become stale. The model should support the validity/effective time of a claim and enough observation/source time to determine whether it was applicable at the time of an incident or decision. “Current,” “historical,” “unknown,” and stale are candidate interpretations, not an approved universal state machine. Freshness requirements may differ by attribute and decision.

Historical reconstruction must be able to answer which identity, attributes, state, and relationships were believed to apply at the relevant time. It must preserve later corrections without rewriting the historical record of what was known when a decision was made.

## 8. Relationships and lifecycle

The source context suggests relationships such as:

```text
Customer owns Zone
Zone contains Record
Region hosts Service
Service depends on Service
Service serves Zone
Incident affects Customer / Zone / Record / Region / Service
Mission assures Service or Customer outcomes
Action targets an entity and may be validated by a Twin
```

These are candidate relationships, not a complete ontology. A mission may aggregate or assess Services, Customers, Regions, and Policies; it must not be reduced to a single “mission impacts service” relationship. The exact mission scope and aggregation semantics belong to RFC-0007.

Relationship validity may vary over time; event, investigation, and decision records should preserve the time context for relationships used in a decision. For example, a zone may have belonged to one customer yesterday and another today.

### 8.1 Directionality and traversal

Relationships are directed claims, even where a user interface may display a reciprocal view. Direction and relationship type matter for reasoning and blast-radius analysis. For example:

```text
Region → hosts → Service → serves → Zone → owned by → Customer
```

Traversal rules must specify which relationship directions and types support a particular inference. A path from region to customer may support a candidate blast-radius calculation, but does not by itself prove that the customer experienced impact.

### 8.2 Observed, authoritative, and inferred relationships

The model must distinguish:

- **Authoritative relationship claim:** asserted by the system designated as authoritative for that relationship and time.
- **Observed relationship:** directly reported or observed by a source, with its provenance and scope.
- **Inferred relationship:** derived from other entities or evidence; it is not an observed fact.

These categories can overlap in source characteristics but must not be collapsed into an unqualified relationship. In particular, “customer owns zone” and “customer was impacted by service failure” are different relationship claims with different evidentiary bases.

### 8.3 Relationship provenance

Every relationship claim should preserve, where applicable:

- Relationship type and direction.
- Source/producer and source namespace.
- Effective/valid time and observation/recording time.
- Whether it is authoritative, observed, or inferred.
- Evidence/provenance links.
- Freshness or validity limitations.
- Confidence when the relationship is inferred, using RFC-0004 semantics.

This is a proposed requirement. Exact fields, confidence applicability, temporal model, provenance retention, and reconciliation are unresolved. Relationships must not be inferred solely from matching display names.

## 9. Operational truth and system-of-record boundaries

Perceptron SRE may receive conflicting claims from APIs, queues, databases, builders, master servers, authoritative servers, twins, and telemetry. This RFC does not make any one system globally authoritative. The integration must identify, for each entity attribute and lifecycle transition, the source that owns the authoritative claim and any independent observations used to verify it.

A useful model may preserve multiple time-stamped claims instead of overwriting disagreement. The canonical truth and reconciliation strategy is open under ARCH-017.

## 10. Security and privacy

Customer and DNS record data may be sensitive. The entity model must support tenant-scoped access and data minimization once security requirements are established. This RFC does not define authorization, encryption, retention, or whether record values may be present in general-purpose events.

## 11. Versioning and compatibility

Entity references and relationship schemas need versioning and migration rules. Integrations must be able to identify the source namespace and schema version. Changes to IDs or relationships must not silently invalidate historical event and decision provenance.

## 12. Acceptance criteria for an accepted RFC

- Candidate entity definitions and relationships are reviewed and bounded.
- Service, mission, action, twin, and service-tier concepts are either accepted with definitions or explicitly deferred.
- Canonical ID ownership and external-reference behavior are specified.
- Event references can distinguish entity type, namespace, and identity without relying only on display names.
- Entity identity is distinct from entity state and health.
- Entity and relationship freshness/temporal semantics are specified for historical reconstruction.
- Relationship direction, traversal semantics, and provenance are documented.
- Impact and blast-radius analysis can be performed using the documented entity relationships and traversal rules, with observed impact distinguished from inferred or potential impact.
- Authoritative, observed, and inferred relationships are distinguishable.
- Unknown, stale, conflicting, deleted, and aliased entities and relationships have explicit handling.
- Tenant and sensitive-data requirements are addressed before production customer data is stored.

## 13. Open decisions

1. Which entities are required in the first core/event contracts?
2. Which external systems own canonical IDs and attributes?
3. Are customer and service tier identity shared across regions or locally mapped?
4. How are entity references represented in events and persisted history?
5. What are the lifecycle and temporal semantics for zones, records, and incidents?
6. How is operational truth represented when sources disagree?
7. Which customer and DNS fields are sensitive and what data minimization is required?
8. Should services be first-class operational entities, and what defines service boundaries (ARCH-025)?
9. Should mission, action, and twin be first-class entities or references to separately governed concepts?
10. Which relationships are authoritative, observed, or inferred, and which source owns each relationship type (ARCH-026)?
11. What temporal model records effective time, observation time, and correction history for entities and relationships?
12. Which relationship directions and traversal rules are valid for impact and blast-radius analysis?
13. Which relationship provenance fields are mandatory, and when is confidence required for an inferred relationship?
14. Is service tier a first-class entity or an attribute/value, and how does it relate to customer, service level, SLA, and business importance?
15. How are entity state and health reported without conflating them with identity or existence?
16. Are components and deployments separate entity classes, and how do they relate to services, regions, and each other (ARCH-027)?
17. How should mission scope reference or aggregate services, customers, regions, and policies?
18. How should business importance be represented independently from service tier?
19. Should coverage/scope be an entity, a relationship property, or assessment metadata, and how does it interact with entity traversal?

## 14. Dependencies

RFC-0001 needs entity reference conventions for event envelopes and payloads. RFC-0004 needs entity scoping for evidence. RFC-0005, RFC-0007, and RFC-0008 rely on consistent customer, service, region, and incident references.
