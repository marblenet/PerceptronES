# RFC-0007: Mission Assurance

**Status:** Architecture review ready  
**Category:** Reasoning and operations  
**Parent:** [Overall Architecture](01-overall-architecture.md)  
**Related RFCs:** RFC-0001 Event Bus and Event Model; RFC-0003 Operational Entity Model; RFC-0004 Confidence and Evidence; RFC-0008 Investigation; RFC-0009 Knowledge Engine; RFC-0010 Policy Engine

**Visual references:** [Reasoning cascade and event lineage](diagrams/D-02-reasoning-cascade.md); [entity and impact graph](diagrams/D-03-entity-impact-graph.md); [regional federation](diagrams/D-10-regional-federation.md).

## 1. Summary

This RFC defines Perceptron SRE's top-level mission assessment: whether the scoped mission can still be accomplished, what operational posture is recommended, and why. Mission assurance combines customer and service outcomes, impact, SLA exposure, confidence, blast radius, business impact, and platform health into a traceable assessment.

Mission assurance is not component-health aggregation. Healthy components do not by themselves prove that a mission is healthy, and an individual component failure does not necessarily mean the mission has failed if its objective remains achievable. GREEN, YELLOW, ORANGE, and RED are candidate mission states from the project vision. Their definitions, thresholds, precedence, transition behavior, and action authority are not established here.

Mission aggregation is a core platform service, not an installable domain engine. Domain engines may supply pressure, health, impact, confidence, and other evidence; the core mission service combines those inputs and produces an explicit assessment, including `UNKNOWN` when required evidence is absent, invalid, or insufficient. GREEN is earned only when all required evidence for the declared scope and objective is valid and sufficiently fresh; absence of evidence, engines, or alerts is never evidence of health.

## 2. Problem and motivation

Operators need more than raw component metrics or isolated alerts. They need an explainable view of whether the service mission is healthy, who is affected, what risk exists, and what response posture is appropriate. Mission assurance is intended as the system's top-level operational output, not just a score. The central question is "Can we still accomplish the mission?", not "Are all components healthy?" Mission assurance is an assessment and recommended operational posture, not merely an alert or dashboard color.

## 3. Goals

- Define mission assurance as a scoped assessment with distinct status, disposition, confidence, coverage, freshness, and explanation.
- Identify input domains and evidence requirements.
- Distinguish observed from potential impact.
- Include confidence and Perceptron SRE platform health.
- Preserve severe localized impact through aggregation.
- Make status calculation, transitions, and decisions auditable.

## 4. Non-goals

- Defining numeric thresholds or a universal aggregation formula in this draft.
- Replacing detailed telemetry, investigations, or service dashboards.
- Treating mission status as authorization to execute an action.
- Assuming one global mission state is sufficient for every region, customer, or service.

## 5. Candidate inputs

The project context identifies:

- Customer and service impact.
- SLA risk and service objectives.
- Platform/service health and pressure.
- Blast radius and affected regions.
- Confidence and evidence quality.
- Business or revenue impact.
- Perceptron SRE's own runtime, event-bus, engine, knowledge, and twin health.

The input schema, freshness requirements, weighting, conflict handling, and per-entity scope are open. Observed impact and modeled potential impact must remain distinguishable.

## 6. Mission state and operational disposition

Candidate states are:

- GREEN
- YELLOW
- ORANGE
- RED

Candidate dispositions include continue operations, observe, prepare intervention, intervene, and escalate. These are not yet mapped one-to-one. Status is an assessment; disposition is the operational posture suggested or selected under applicable policy.

Mission status must not be treated as the same thing as confidence, service health, or authorization. A RED state does not itself execute a repair, and GREEN does not prove every component is healthy. Disposition is a recommendation or operational posture, not execution authority: a recommendation to intervene does not authorize an action; RFC-0010 governs authorization.

## 7. Scope and aggregation

Mission assessment may exist at a hierarchy of scopes, such as customer, service, region, platform, and global mission. The canonical hierarchy and relationships remain open and must support assessments both at a local scope and across composed scopes.

An assessment may ultimately need to be defined by both a **scope** and an **objective**. Candidate objectives include DNS availability, zone provisioning, record propagation, and control-plane operations. Whether objectives are explicit mission entities, configured criteria, or another model remains open (ARCH-035).

Aggregation across scopes must preserve significant local impact rather than allowing a healthy majority to conceal a severe affected subset. For example, a critical customer outage must not disappear into an average simply because most customers remain healthy. Aggregation rules, time windows, status transitions, hysteresis, and handling of missing inputs remain open.

### 7.1 Assessment dimensions

Mission output should keep these dimensions distinct:

- **Mission status:** the categorical assessment, using candidate states such as GREEN, YELLOW, ORANGE, or RED; whether UNKNOWN is a distinct state is open under ARCH-034.
- **Disposition:** the recommended operational posture, such as continue, observe, prepare intervention, intervene, or escalate. It does not authorize execution.
- **Mission confidence:** support for the mission assessment, distinct from the status itself.
- **Mission coverage:** the portion of the requested mission scope, inputs, regions, and relevant reasoning chain represented by the assessment. A favorable status with limited coverage must not imply a complete assessment.
- **Mission freshness:** when the assessment was produced and whether its contributing evidence remains current for the stated scope. An old status must not be presented as current without qualification.

These dimensions are candidate outputs, not a selected schema or algorithm. Their required representation and composition are open under ARCH-033.

Mission assurance may also need to describe trajectory—such as stable, improving, or degrading—so emerging risk is visible before a categorical state changes. Trajectory semantics and whether they belong in this RFC or a later contract remain open.

## 8. Recursive platform assurance

Perceptron SRE is itself a protected mission. Mission assurance should account for degradation in the reasoning system, including event backlog or loss, engine failures, knowledge integrity concerns, twin degradation, and confidence drift. The system must communicate when its own degraded state limits the reliability or completeness of its conclusions.

An independent monitoring path is desirable but not defined. The platform must avoid circularly declaring itself healthy solely from signals produced by the component whose health is being assessed.

## 9. Explanation and provenance

Every mission assessment should reference the evidence and derived results that contributed to it, identify unavailable or contradictory evidence, and state the time/scope it covers. A reader should be able to trace the explanation through contributing assessments, impact, confidence, coverage, and underlying evidence. Status changes and disposition recommendations should be reconstructable from recorded inputs and policy context.

The exact explanation format and retention requirements depend on RFC-0001 and RFC-0004.

## 10. Failure and safety behavior

Missing, stale, contradictory, or low-confidence inputs must be visible. They must not be treated as zero impact or healthy state by default. If mission assessment is unavailable, operators and policy must be able to distinguish “unknown/unavailable” from GREEN.

Mission assurance informs response; RFC-0010 remains responsible for action authorization.

## 11. Acceptance criteria for an accepted RFC

- Mission status answers whether the scoped mission objective remains achievable and is not inferred solely from component-health aggregation.
- Mission status, disposition, mission confidence, coverage, and freshness have distinct, testable semantics.
- Assessment scope and aggregation rules are specified.
- Observed and potential impact are distinguishable.
- Missing, stale, contradictory, and platform-degraded inputs have defined handling.
- Each assessment retains its input evidence and version/configuration provenance.
- Mission explanations provide traceable links to evidence, impact, confidence, coverage, and contributing assessments.
- Aggregation cannot conceal severe localized impact behind a healthy majority or average.
- Low coverage and stale assessments are explicitly visible and cannot be presented as complete/current without qualification.
- Status does not directly authorize production actions.
- Tests cover cross-region aggregation and severe localized impact.
- No missing, invalid, stale, contradictory, or unavailable required evidence can produce a favorable status by default; tests exercise these cases.
- Recursive assurance reports when the platform's own degradation limits conclusions.

## 12. Open decisions

1. What do GREEN, YELLOW, ORANGE, and RED mean operationally?
2. Should mission assessment expose confidence, coverage, and freshness as separate outputs (ARCH-033)?
3. What are the exact taxonomy and transition semantics for mission states (ARCH-034)? `UNKNOWN` is required whenever the implementation cannot establish the requested assessment from required evidence.
4. What inputs and aggregation rules determine status at each mission scope and across the scope hierarchy?
5. How are missing inputs, conflicting truth, and confidence handled?
6. How are observed and potential customer impacts represented in mission output?
7. What operational disposition maps to each mission status, and who has authority to recommend it?
8. How is Perceptron SRE independently monitored?
9. How are state transitions, trajectory, hysteresis, and recovery/closure handled? Trajectory such as stable, improving, or degrading may distinguish otherwise identical mission states, but its semantics remain open.
10. Is a mission defined by both scope and objective, and how are objectives represented (ARCH-035)?
11. Mission assessment is Layer-3 Assessment in the canonical cascade (Overall Architecture 6.4). In the prototype, Layer-2 determinations do not yet feed health or mission; how and when they should is open.

## 13. Dependencies

RFC-0003 provides entity and scope references. RFC-0004 defines evidence and confidence. RFC-0008 and RFC-0009 provide investigation and knowledge inputs. RFC-0010 governs actions resulting from disposition.
