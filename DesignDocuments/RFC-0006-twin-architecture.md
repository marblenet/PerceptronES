# RFC-0006: Twin Architecture

**Status:** Architecture review ready  
**Category:** Operational safety  
**Parent:** [Overall Architecture](01-overall-architecture.md)  
**Related RFCs:** RFC-0001 Event Bus and Event Model; RFC-0002 Engine Runtime; RFC-0004 Confidence and Evidence; RFC-0005 Traffic Handler; RFC-0009 Knowledge Engine; RFC-0010 Policy Engine

**Visual references:** [Twin validation and promotion](diagrams/D-07-twin-validation.md); [evidence and confidence lifecycle](diagrams/D-04-evidence-confidence.md); [policy action lifecycle](diagrams/D-08-policy-action-lifecycle.md).

## 1. Summary

This RFC defines the proposed role of twins in validating changes against production-derived conditions without affecting production. It distinguishes a data-plane twin, which operates alongside production and may receive mirrored queries but must suppress its responses, from a control-plane twin, which may replay changes against isolated, production-derived state without modifying production.

A twin is production-safety infrastructure: its primary value is reducing the likelihood that production becomes the first environment in which a change, repair, or remediation is exercised. A twin may also provide an independent second opinion, subject to common-mode limitations. A twin result is evidence, not approval. Validation scope, coverage, freshness, drift, confidence semantics, isolation, and promotion criteria remain open.

## 2. Problem and motivation

Production should not be the first environment in which a corrective action is tested when a relevant twin or simulation is available. The twin's primary purpose is risk reduction, not general feature testing. Validation can reveal whether a proposed change behaves as intended and can contribute scoped evidence to confidence and knowledge trust.

## 3. Goals

- Define data-plane and control-plane twin roles.
- State the production-isolation and response-suppression invariants.
- Define candidate validation outcomes.
- Require validation results to identify the action, conditions, scope, coverage, freshness, and known drift to which they apply.
- Explain the proposed promotion ladder without treating it as an approved deployment pipeline.
- Identify twin health and how twin results inform confidence, policy, and knowledge trust renewal.

## 4. Non-goals

- Guaranteeing that a twin exactly reproduces production.
- Treating twin success as authorization for production execution.
- Selecting a virtualization, container, DNS, or traffic-replay technology.
- Replacing simulation; a twin and a simulation answer related but distinct questions.

## 5. Twin types

### 5.1 Data-plane twin

A data-plane twin operates adjacent to production hosts and may process live mirrored production queries. The production DNS service remains authoritative and returns the customer response; twin responses are always suppressed and must never replace or affect the production response. The twin evaluates how a proposed configuration, software version, rule set, or operational change behaves under actual production traffic conditions without impacting customer service.

The twin may support behavior comparison, configuration evaluation, and validation. Whether full packets or minimized metadata are mirrored, and how sensitive data is handled, remain open. A cold twin may instead use selected replay or production-derived inputs rather than continuous live mirroring; the required operating modes remain open.

### 5.2 Control-plane twin

A control-plane twin enables committed changes, repairs, migrations, and operational actions to be replayed against isolated, production-derived state before production execution. It must be isolated from production write paths and cannot modify production state.

The exact replay source, ordering, data synchronization, external side effects, and treatment of irreversible operations require definition. Replaying a change in the twin does not execute or authorize that change in production.

### 5.3 Candidate operating classes

The promotion ladder uses "cold" and "hot" twin without defining them. Candidate meanings for review are:

- **Cold twin:** a validation environment that is not continuously receiving live production traffic and is exercised through selected replay, snapshot, or test inputs.
- **Hot twin:** a validation environment that continuously or near-continuously receives mirrored production traffic, with responses suppressed and production isolation preserved.

These labels do not select a required architecture. Whether they describe traffic mode, readiness, synchronization state, or promotion stage—and whether both classes are needed—remains open.

### 5.4 Twins versus traditional environments

Development, integration, test, QA, and staging environments remain important for validating correctness before deployment; the twin does not replace them. A typical change path may be:

```text
Development lab
    ↓
Integration lab
    ↓
Committed candidate
    ↓
Twin alongside production
    ↓
Policy-governed production promotion
```

Traditional pre-production environments generally exercise synthetic traffic, selected test cases, curated datasets, or pre-production workflows. A twin's distinguishing capability is to evaluate a candidate against production-derived traffic, data, behavior, or change streams while remaining isolated from production service delivery and state modification. For a data-plane twin, this may include live mirrored production traffic; for a control-plane twin, it may include replay against production-derived state.

The twin is alongside production, not simply another pre-production stage. It tests a committed candidate against representative production conditions before production execution or promotion, while the existing production service continues to handle customer traffic. This relationship does not prescribe an organization-wide deployment pipeline or make twin validation sufficient for production authorization.

A twin evaluates committed candidates under production-derived conditions without exposing customers to the candidate being evaluated.

## 6. Isolation and fidelity

Isolation must prevent twin activity from changing production data, serving customer responses, or invoking production actions unintentionally. The architecture requires an explicit trust boundary and testable proof of separation before real production data or mirrored traffic is used.

Fidelity is multidimensional: configuration, data, traffic shape, dependencies, timing, regional topology, and failure conditions may differ. Twins can drift while remaining available: production and twin may differ in software/configuration versions, dataset or zone inventory, dependencies, topology, or traffic representation. A validation result must record the twin identity/version, relevant configuration and data versions, test inputs, validation time, and known divergence where available. Drift and freshness qualify applicability of a result; neither is replaced by a simple healthy/unhealthy twin status.

Twin freshness includes how current its configuration, data, dependencies, and (where applicable) mirrored traffic are relative to the production state relevant to the proposed validation. No universal freshness window or minimum fidelity threshold is specified. The twin must report limitations rather than presenting an unrepresentative test as production-equivalent.

## 7. Validation outcome model

Candidate outcomes:

- **Pass:** declared validation criteria were met for the tested scope.
- **Fail:** criteria were not met or an unacceptable effect was observed.
- **Ambiguous:** evidence is insufficient, conflicting, or not representative enough to conclude.

The status must be scoped to a proposed action/change and validation objective, and must state the conditions under which it was tested: target, region, configuration and data state, inputs or traffic profile, and applicable time window. A pass for one action and scope does not authorize generalization to other actions, targets, regions, or conditions.

Validation **coverage** describes how much of the requested target, traffic, data, region, dependency, or condition scope was actually exercised. Coverage is distinct from confidence: a result may be strongly supported within a narrow tested scope while providing little coverage of the requested scope. Validation confidence describes the support for the scoped result, taking into account evidence quality, freshness, fidelity limitations, and drift. A pass/fail outcome, coverage, confidence, and twin health are separate dimensions; no universal formula or threshold is selected here.

Twin health should expose at least the candidate states **healthy**, **stale**, **degraded**, **unsynchronized**, **failed**, and **unavailable**. Health describes the twin's operational/readiness condition, not the result of a particular validation. State semantics, transitions, and freshness limits remain open. A stale or unsynchronized twin may still produce a result, but that result must disclose those limitations and must not imply current representativeness.

## 8. Twin as second opinion

The twin may independently exercise a proposal and report expected behavior, providing a second opinion to the reasoning system or human operator. Independence is not automatic: shared code, configuration, data, or assumptions may produce correlated errors. Validation reports should identify relevant common dependencies and limitations where feasible.

Twin results can contribute evidence to confidence and knowledge trust renewal only for the tested action, environment, scope, and conditions. Validation may renew trust in a knowledge item or procedure only to the extent that the twin evidence is relevant, sufficiently current, and representative; renewal is not automatic or general-purpose. Trust remains distinct from confidence, and a pass does not establish general correctness or bypass policy or approval. Result provenance should include twin identity and version, data and configuration versions, traffic/input source and method, tested scope and coverage, relevant drift and fidelity limitations, outcome, and validation time, with confidence semantics defined under RFC-0004.

## 9. Promotion ladder

The source concept proposes:

```text
Cold Twin
    ↓
Hot Twin
    ↓
Cold Region Production
    ↓
Hot Region Production
```

This is a candidate staged promotion model, not a finalized deployment workflow. The terms, required soak times, traffic percentages, rollback triggers, approval gates, and meaning of “cold” and “hot” are undefined. Promotion must be supported by validation coverage appropriate to the next stage; what coverage is sufficient is open under ARCH-032. Production stages must remain subject to RFC-0010 policy and service-owner controls.

## 10. Failure and safety behavior

The twin system must expose its health/readiness state (including candidate states healthy, stale, degraded, unsynchronized, failed, and unavailable) separately from validation outcomes such as pass, fail, and ambiguous. A missing twin result is not a pass. Twin failure must not cause automatic production execution.

Mirroring and replay require limits, authentication, authorization, privacy controls, resource isolation, and audit. A production connection must be unable to accept writes or responses from the twin unless explicitly designed and independently authorized.

## 11. Acceptance criteria for an accepted RFC

- Data-plane response suppression is demonstrably enforced.
- Control-plane twin writes are isolated from production systems.
- Development, integration, and other pre-production environments are distinguished from the twin; the twin's production-adjacent placement and production-derived validation role are explicit.
- Production traffic remains served by the production system; data-plane twin responses cannot affect customers.
- Committed changes can be evaluated against production-derived traffic, data, behavior, or change streams without modifying production state.
- Twin identity/version, tested action and scope, coverage, drift, freshness, fidelity limitations, configuration/data versions, input or traffic provenance, and validation time are recorded with results.
- Pass, fail, and ambiguous outcomes have testable criteria.
- Validation outcome, coverage, confidence, and twin health are distinct and cannot be conflated.
- Unavailable or stale twin state cannot be interpreted as successful validation.
- Drift and synchronization limitations are observable and inform the applicability of validation results.
- Twin results are represented as evidence and cannot directly authorize production changes.
- Promotion stages have explicit gates, rollback behavior, and accountable owners before use.
- Promotion requirements define sufficient validation coverage for each stage.
- Knowledge trust renewal from twin validation is scoped, evidence-based, and not automatic or general-purpose.
- Mirrored production data has reviewed privacy, access, and retention controls.

## 12. Open decisions

1. What constitutes a twin and how is it provisioned and synchronized?
2. How are twin health, drift, coverage, freshness, and synchronization represented and incorporated into validation confidence (ARCH-031)?
3. What fidelity dimensions and thresholds are required for each validation type?
4. How is production data or traffic minimized, protected, and retained?
5. How are pass, fail, and ambiguous criteria defined?
6. What validation coverage is sufficient for promotion to each next stage (ARCH-032)?
7. What does each promotion ladder stage mean and who approves progression?
8. What are the operational definitions and required capabilities of cold and hot twins?
9. How are twin independence and common-mode failure assessed?
10. What happens when a relevant twin is unavailable or a change cannot be twin-tested?
11. How do twin results affect confidence and knowledge trust without replacing policy?

## 13. Dependencies

RFC-0001 provides replay and provenance contracts. RFC-0004 defines validation evidence. RFC-0005 defines traffic mirroring. RFC-0009 defines knowledge trust renewal. RFC-0010 governs production gates and approvals.
