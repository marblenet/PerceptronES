# RFC-0010: Policy Engine

**Status:** Architecture review ready  
**Category:** Operational safety  
**Parent:** [Overall Architecture](01-overall-architecture.md)  
**Related RFCs:** RFC-0001 Event Bus and Event Model; RFC-0002 Engine Runtime; RFC-0004 Confidence and Evidence; RFC-0006 Twin Architecture; RFC-0008 Investigation; RFC-0009 Knowledge Engine

**Visual references:** [Policy, authorization, and action lifecycle](diagrams/D-08-policy-action-lifecycle.md); [twin validation and promotion](diagrams/D-07-twin-validation.md); [engine package, Store, and trust lifecycle](diagrams/D-12-engine-store.md).

## 1. Summary

This RFC defines the proposed policy engine as the safety governor between Perceptron SRE analysis and production action. Investigation assesses what may be happening; mission assurance assesses whether the scoped objective remains achievable; policy evaluates the risk and permissibility of a proposed action. Policy governs action risk, not factual truth. It separates recommendation, validation, authorization, and execution; identifies action classes and approval levels; and calls for emergency controls such as a kill switch, global pause, and safe mode.

The policy language, approval tiers, authority model, emergency-control semantics, and initial release action capability remain open. No confidence score, recommendation, twin pass, or human request alone bypasses policy.

## 2. Problem and motivation

Perceptron SRE aims to recommend and eventually perform operational actions. Without a centralized, auditable policy boundary, an engine could turn a high-confidence hypothesis or runbook into an unsafe production change. Policy must decide whether a requested action is permitted in context and record why.

## 3. Goals

- Separate analysis, recommendation, validation, approval, authorization, execution, and outcome.
- Define action classes and approval requirements.
- Evaluate action risk and eligibility without claiming to establish factual truth.
- Account for the scope, coverage, completeness, confidence, and freshness of decision inputs.
- Make decisions explainable, versioned, and auditable.
- Provide emergency controls and a safe degraded mode.
- Prevent policy bypass through direct engine invocation or replay.

## 4. Non-goals

- Selecting a policy language or authorization framework in this draft.
- Defining all operational actions and their risk classifications.
- Assuming the initial core can execute production actions.
- Treating confidence or successful validation as sufficient authorization.

## 5. Decision model

Candidate flow:

```text
Analysis / finding
        ↓
Recommendation (proposed action and expected outcome)
        ↓
Validation evidence (twin/simulation where applicable)
        ↓
Policy evaluation and required approval
        ↓
Authorization decision
        ↓
Execution by the owning system integration
        ↓
Outcome verification and audit
```

Each stage has a distinct identity and record. A denied or held action remains a recommendation or request; it is not an execution. The system that owns the target service's control plane should execute changes through its authorized interface.

Policy is conceptually a top-level decision authority immediately before authorization and execution, even though its RFC identifier is later than the foundational and reasoning RFCs. Mission assessment and investigation are policy inputs; policy does not replace either process.

## 6. Action classes

The project context identifies response categories such as:

- Observe.
- Investigate.
- Recommend.
- Validate.
- Execute/repair.
- Escalate.

These categories describe operational intent and are not yet a complete risk taxonomy. The action model should identify target, exact scope (including region and affected resources), preconditions, expected effects, risk, reversibility, validation evidence, and responsible execution adapter where applicable. Authorization for one target, region, action, or scope must not be reusable for another.

### 6.1 Engine and tool policy declarations

Each engine/tool package declares an action-policy profile (`reasoning_only`, `human_escalation`, or `self_mitigation`), whether it is intended to be read-only, and which named tools/scripts it requests. These are inputs to policy review, not grants. An engine cannot authorize itself by choosing a permissive profile, and an allow-list in package metadata is not enforcement.

The platform must mediate every tool invocation and script execution through a core-owned capability broker that validates engine identity/version, exact tool, arguments, target scope, identity, policy version, approval, and authorization freshness. Engines must not receive reusable credentials or unrestricted access to an execution adapter. Until this broker and a suitable OS sandbox exist, the runtime remains observe/reason-only: tool/script capabilities and self-mitigation execution are unavailable. Arbitrary subprocess plugins still run with the service account's OS privileges, so `read_only` metadata alone cannot enforce filesystem or network restrictions.

`self_mitigation` means only that an engine may formulate a proposed bounded action for independent policy evaluation; it does not authorize or execute the action. `human_escalation` permits an attributable handoff request; it does not authenticate or approve a human.

## 7. Approval classes

The source context gives candidate examples:

- Class 1: automatic.
- Class 2: twin validation.
- Class 3: human approval.
- Class 4: executive approval.

These examples are not approved levels or mappings. An accepted RFC must define whether approval class is a risk tier, required gate, or both; which actor can approve; separation of duties; expiration; and how approval is bound to the exact action and evidence. Twin validation is a validation gate, not necessarily an approval authority.

## 8. Policy inputs and outcomes

Potential decision inputs include action type and target, exact action scope, recommendation provenance, evidence and confidence domains, observed/potential customer impact, mission status and coverage, validation result and coverage, relevant freshness/completeness, system state, identity/authorization, time constraints, change windows, and emergency-control state. Policy must evaluate action risk and permissibility, not decide whether a factual hypothesis is true: a low-confidence root-cause diagnosis can coexist with a well-supported low-risk action, while a high-confidence diagnosis can coexist with an unsafe action.

Candidate outcomes include permit, deny, hold for approval, require additional validation, escalate, or **unable to evaluate / insufficient information**. Missing required evidence, identity, coverage, or policy inputs must not be silently converted into permit, deny, or an empty/default result. The required safe handling of an inability to evaluate remains an open decision. The output must state the policy version/rule basis and any unmet preconditions.

Policy constraints govern action eligibility; they must not be treated as evidence that a factual hypothesis is correct. Policy evaluation confidence, if represented, describes confidence in the evaluation result—not confidence in a root cause or proposed action's effectiveness—and remains an open design choice.

### 8.1 Layer-5 Policy in the cascade (implemented prototype)

In the canonical cascade (Overall Architecture 6.4), **Layer-5 Policy** answers "Can we do this?" for each **Layer-4 Recommendation** and produces policy decisions only. The prototype `host-policy` engine emits `policy.decision.v1` with one of four decisions: `authorized`, `require_approval`, `simulation_only`, or `denied`. The decision comes from configuration (`policy.default_decision`, default `require_approval`, with optional per-recommendation `policy.rules`); a recommendation, confidence, or simulation never raises it. Simulation- or replay-origin recommendations are never `authorized`. **Layer-6 Action** (`host-action`) turns a decision into an action record with `executed: false` always, because no capability broker, approval workflow, or executor exists; nothing in the prototype can carry out an action. **Layer-7 Outcome** (`host-outcome`) reports `resolved`, `improved`, `unchanged`, `worse`, or `unknown` for a determined condition by comparing live cycles in memory; it never attributes an outcome to an action and `resolved` requires fresh evidence. **Layer-8 Learning** (`host-learning`) emits learning candidates (a regression-test proposal when resolved, a knowledge-review proposal when worse) that are never applied automatically. Open: approval workflow and authority, durable decision/outcome history, binding a decision to exact tool, arguments, and scope, and the meaning of `simulation_only` for real adapters.

## 9. Emergency controls and safe mode

Candidate controls named in the project context:

- **Kill switch:** halt a defined class or all automated production actions.
- **Global pause:** pause action workflows while preserving observation and evidence collection where safe.
- **Safe mode:** restrict the platform to a defined reduced capability set, such as observation/recommendation, when core safety signals are degraded.

Their scope, authority, activation method, persistence, precedence, audit, propagation, and recovery process are unresolved. Controls must fail in a documented direction and should be independently operable from the component they disable where feasible.

## 10. Policy invariants

1. Production execution requires a policy decision.
2. Recommendation is not authorization; authorization is not execution.
3. Confidence, twin validation, and human intent do not individually override policy.
4. Policy decisions are versioned and auditable.
5. Replay must not cause unapproved side effects.
6. Missing or stale required evidence cannot silently satisfy a precondition.
7. Policy/engine degradation must not silently widen permissions.

These are proposed invariants consistent with the overall architecture and require operational threat review.

## 11. Execution, verification, and rollback

Policy authorization should be bound to the exact proposed action, target, scope, validation evidence, and applicable time/context so that a modified action cannot reuse an unrelated approval.

Authorization and approval freshness must be explicit. A previously valid approval or decision may no longer apply after its validity window, relevant system state, mission assessment, evidence, validation, policy version, or emergency-control state changes. Expiration, reevaluation triggers, and time bounds must be defined; a stale decision must not be presented as current authorization.

Execution should occur through the owning system's integration. The platform must then verify the intended outcome and check for new customer/service impact. Rollback, compensation, partial execution, and incident closure must be addressed by action-specific policy and remediation RFCs.

## 12. Audit and observability

Record recommendation, policy version, evaluated inputs, validation references, approval actor/process where applicable, decision, execution request/result, emergency controls, and outcome verification. Retention and access controls are open but must be resolved before production actions are enabled.

Policy evaluation errors, unavailable policy data, expired approvals, and inconsistent state must have explicit safe behavior. No generic default-allow behavior is proposed.

## 13. Acceptance criteria for an accepted RFC

- Recommendation, validation, approval, authorization, execution, and outcome are separate records/states.
- Every production action path passes through a policy-enforced boundary.
- Policy decisions identify the version/rules and relevant inputs.
- Decision rationale distinguishes action risk/eligibility from factual claims and identifies relevant scope, coverage, confidence, and freshness.
- Every authorization decision is traceable to its evidence, mission assessment, validation results, approvals, and policy rules.
- Approval, when required, is bound to the exact action and scope and expires or is invalidated under defined conditions.
- Stale approvals and authorizations cannot be represented as current; reevaluation conditions are defined.
- Denied, held, expired, and failed evaluations cannot be mistaken for permission.
- Missing or insufficient inputs can be reported as unable to evaluate/insufficient information and cannot produce a success-shaped authorization.
- Emergency controls and safe mode have testable scope and precedence.
- Replay and retries cannot duplicate production side effects without explicit authorization/idempotency handling.
- Post-action verification and adverse impact handling are defined.

## 14. Open decisions

1. Is the initial Perceptron SRE release observe-only, recommendation-capable, or action-capable?
2. What policy language, policy owner, and distribution model are used?
3. How are actions classified by risk, reversibility, and blast radius?
4. What are the approval classes and what actors/roles may satisfy them?
5. Is twin validation required for particular actions, and what exceptions are allowed?
6. What are kill switch, global pause, and safe-mode semantics and authority?
7. What happens when policy evaluation or required identity/approval systems are unavailable?
8. How are authorization, idempotency, retries, rollback, and partial execution handled?
9. What audit retention and separation-of-duties requirements apply?
10. How are policy coverage, completeness, freshness, and evaluation confidence represented in authorization decisions (ARCH-041)?
11. What semantics and safe handling apply to unable-to-evaluate or insufficient-information outcomes?

## 15. Dependencies

RFC-0001 supplies event and replay behavior. RFC-0002 defines capabilities and execution isolation. RFC-0004 defines evidence and confidence inputs. RFC-0006 provides validation results. RFC-0008 and RFC-0009 provide recommendations and knowledge. Production execution must not be implemented before this RFC and target-system integration requirements are accepted.
