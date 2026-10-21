# RFC-0004: Confidence and Evidence Framework

**Status:** Architecture review ready  
**Category:** Reasoning contract  
**Parent:** [Overall Architecture](01-overall-architecture.md)  
**Related RFCs:** RFC-0001 Event Bus and Event Model; RFC-0002 Engine Runtime; RFC-0003 Operational Entity Model; RFC-0006 Twin Architecture; RFC-0008 Investigation; RFC-0009 Knowledge Engine; RFC-0010 Policy Engine

**Visual references:** [Evidence and confidence lifecycle](diagrams/D-04-evidence-confidence.md); [investigation and knowledge workflow](diagrams/D-06-investigation-knowledge.md); [twin validation and promotion](diagrams/D-07-twin-validation.md).

## 1. Summary

This RFC proposes a common way for Perceptron SRE to represent evidence, uncertainty, and confidence. Outputs should be explainable and should not present hypotheses as facts. Evidence may support or contradict a hypothesis, or be inconclusive/unavailable. Confidence is not authorization, and policy constraints are not factual evidence.

No confidence formula, scale calibration, weighting, threshold, or universal requirement to emit a numeric confidence is approved by this draft.

## 2. Problem and motivation

The cascade combines measurements, tool results, historical knowledge, twin results, simulations, and human verification. Without shared evidence semantics, confidence scores will be inconsistent and unexplainable. Ignoring contradictory evidence can cause the system to reinforce a favored but incorrect root cause.

## 3. Goals

- Define candidate evidence categories and provenance questions.
- Preserve supporting, contradictory, and inconclusive evidence.
- Distinguish different kinds of confidence.
- Represent confidence as dynamic, qualified, and explainable rather than as a timeless score.
- Preserve evidence independence and the provenance of confidence results.
- Make confidence relevant to action distinct from confidence in an explanation.
- Make uncertainty visible to operators and policy.
- Prevent confidence from being treated as authorization.
- Enable tests of evidence handling and confidence explanation.
- Make missing, stale, contradictory, malformed, and unavailable evidence explicit rather than silently favorable.

## 4. Non-goals

- Choosing a universal probabilistic model or machine-learning algorithm.
- Claiming that a numeric score is calibrated probability.
- Treating all evidence sources as equally reliable or independent.
- Defining mission-state or action thresholds.

## 5. Terms

- **Evidence:** an observation, result, assertion, or validation relevant to a claim or decision.
- **Claim/hypothesis:** a proposition under assessment, such as a candidate root cause or predicted impact.
- **Confidence:** an expression of support or uncertainty associated with a specific claim or output; its exact semantics must be declared.
- **Source trust:** an assessment of how much a source or knowledge item should be relied on, based on its provenance, age, validation, and history. Source trust is not confidence in the current conclusion.
- **Coverage:** the entities, regions, inputs, or time window represented by an assessment. Coverage describes completeness/scope, not the strength of support for a conclusion.
- **Provenance:** information identifying how evidence was produced, when, by whom/what, from which source, and under what version/configuration.
- **Contradictory evidence:** evidence that conflicts with or weakens a claim.
- **Inconclusive evidence:** evidence that is missing, stale, incomplete, or does not discriminate among explanations.
- **Confidence result:** a claim-scoped assessment of support, with its semantics, evidence lineage, scope, and time context.

## 6. Candidate evidence categories

- Observed telemetry or service events.
- Toolbox and diagnostic results.
- Historical incidents and knowledge items.
- Twin validation results.
- Simulation results.
- Human verification or operator observations.
- External system-of-record claims.
- Policy constraints and approvals.

These sources have different roles. Policy determines what may be done; it is not proof that a root cause is true. Simulation predicts a modeled outcome; it is not an observed production outcome. A twin result is only as relevant as its fidelity and test conditions.

Confidence and trust must not be used synonymously. For example, a runbook may have low trust because it is old or unvalidated, while a particular twin result may provide high confidence for the narrow proposal and conditions actually tested. The trust model for knowledge is developed in RFC-0009; this RFC covers confidence in claims and results and how source trust may qualify evidence.

## 7. Evidence representation

Evidence should be associated with the claim or output it informs and should preserve, where available:

- Evidence identity and type.
- Producing source/tool/engine and version.
- Subject entity and scope.
- Observation/event time and processing time.
- Input references or provenance links.
- Freshness, completeness, and quality limitations.
- Relationship to the claim: supporting, contradictory, or inconclusive.
- Whether the source is observed, modeled, replayed, or human-reported.
- Observation method and completeness where applicable, such as direct, mirrored, aggregated, sampled, or estimated traffic data.

This list is a candidate conceptual contract, not a final schema. Evidence may be large or sensitive; events may reference protected records rather than inline raw data.

### 7.1 Evidence quality factors

Evidence evaluation should be able to account for factors such as freshness, completeness, independence, directness, reproducibility, and validation status. These factors qualify how relevant and strong a piece of evidence is for a particular claim and scope. For example, a direct current measurement may be more relevant to the present state than similarity to a historical incident, but no factor has a universal weight independent of context.

Evidence quality is related to, but distinct from, source trust and confidence: quality characterizes evidence for a claim, trust characterizes reliance on a source or knowledge item, and confidence characterizes support for the claim or result. This RFC does not prescribe a quality score or combination algorithm.

## 8. Confidence types and semantics

Potential distinct forms include:

- **Observation confidence:** support that an observation or measurement is accurate for its stated scope and time.
- **Claim/root-cause confidence:** support that a hypothesis explains observed conditions.
- **Impact confidence:** support that impact occurred or that a modeled impact estimate is applicable.
- **Prediction confidence:** support for a forecast or modeled future outcome.
- **Action confidence:** support that a proposed action is likely to achieve its stated objective under stated conditions, with acceptable side effects.
- **Outcome confidence:** support that an observed result followed from or satisfies the intended outcome of an action.

These must not be conflated. Knowledge trust is a separate concept defined in RFC-0009: it qualifies how much a knowledge source or item should be relied on, while confidence qualifies support for a particular claim or result. The architecture has not established whether confidence is one value or multiple domains, or whether it is a 0–1 value, percentage, categorical range, calibrated probability, or multiple measures. Numeric examples in project context are illustrative.

Every confidence-bearing result should identify what the confidence refers to and what evidence it considered. It should also retain enough provenance to explain how the assessment was produced and how it changed. Whether every engine output must include confidence remains open (ARCH-014).

Confidence must not hide incomplete coverage. A result may be strongly supported within a narrow scope while failing to cover the full requested scope. Outputs and policy inputs should preserve coverage separately; a missing dependency may reduce coverage and may also affect confidence, but the relationship must be explicit rather than silently converting missing evidence into certainty.

Confidence is dynamic, not a timeless score. Its strength may increase or decrease as evidence arrives, changes, becomes stale, or is invalidated by changes in the environment. A confidence result therefore needs a time context and may need re-evaluation or expiry; the rules and representation are not selected by this draft.

## 9. Confidence generation and updates

Confidence may accumulate from multiple relevant evidence sources over time; it is not assumed to originate from a single observation, engine, or calculation. More evidence does not necessarily mean more confidence. New evidence may strengthen, weaken, or leave a claim unchanged.

An accepted RFC must explain how engines may derive or update confidence, including:

- Combining evidence from multiple sources.
- Assessing evidence independence and identifying duplicated, correlated, or commonly sourced evidence so that it is not treated as independent corroboration without justification.
- Reducing support when contradictory evidence appears, including evidence that disproves an expected condition.
- Handling stale, missing, incomplete, or low-quality inputs, and environmental changes that make prior evidence less applicable.
- Separating confidence in an observation from confidence in a conclusion.
- Recording confidence changes as new results with their causes rather than silently rewriting prior assessments or decisions.
- Recording the provenance of the confidence result itself, including the claim, scope, evidence considered, derivation method and version, and relevant evaluation time.

No additive, Bayesian, weighted, neural, or other formula is selected in this draft. Confidence must not be treated as more precise than the evidence supports.

### 9.1 Confidence and evidence lineage

Each confidence result should be traceable to its supporting, contradictory, and inconclusive evidence and to the provenance of those inputs. A downstream engine may consume an upstream confidence result as qualified derived evidence where its semantics, scope, time, and lineage are available. It must not treat the result as an unqualified fact or silently sever it from the underlying evidence. Whether consumers may rely on the confidence result directly, or must inspect the underlying evidence, is open under ARCH-028.

### 9.2 Confidence for action

Perceptron should distinguish confidence to act from confidence to explain. Confidence that a root-cause hypothesis is correct does not by itself establish that a proposed action is safe or effective. Conversely, uncertainty about root cause does not automatically rule out a well-validated, low-risk action. Action confidence concerns the proposed action's objective, conditions, expected benefit, and potential side effects; it remains separate from policy authorization.

Low confidence may appropriately lead to continued observation, additional investigation, a request for human verification, or escalation. These are valid operational outcomes, not necessarily system failures, and must not be converted into an attempted action merely to avoid an inconclusive result.

## 10. Relationship to policy

Confidence is an input to policy where policy chooses to use it, not a permission token. A high action-confidence assessment does not independently authorize execution, override missing validation, or supersede an approval requirement. Policy constraints affect action eligibility, not factual truth. Policy should be able to distinguish action confidence from confidence in a diagnosis or impact estimate.

## 11. Observed and potential impact

Evidence and confidence reporting must distinguish:

- **Observed impact:** measured or confirmed customer/service effect.
- **Potential impact:** modeled or projected exposure under a scenario or failure hypothesis.

An estimate must not be displayed as confirmed customer harm. Source, time window, affected-entity scope, assumptions, and uncertainty should accompany impact results.

## 12. Acceptance criteria for an accepted RFC

- Evidence categories and provenance fields are defined and versioned.
- Supporting, contradictory, inconclusive, and unavailable evidence are distinguishable.
- Evidence quality factors, including freshness, completeness, independence, directness, reproducibility, and validation status, can be represented or accounted for without requiring a universal quality score.
- Confidence outputs identify the claim and semantics they represent.
- Confidence results preserve their own provenance, evidence lineage, scope, and relevant time context.
- Evidence independence, duplication, and shared provenance can be represented or otherwise accounted for without assuming correlated evidence is independent.
- Confidence may be strengthened or reduced as evidence changes; prior assessments are not silently rewritten.
- Confidence reductions caused by contradictory, stale, missing, incomplete, or lower-quality evidence are observable and explainable.
- Confidence freshness or validity limitations can be represented without conflating confidence with source trust.
- Coverage and confidence are represented as separate dimensions when an assessment has incomplete scope.
- Tests demonstrate how contradictory and stale evidence affect results.
- Tests demonstrate that insufficient, invalid, or unavailable evidence cannot produce unsupported certainty or a favorable result by default.
- Action confidence is distinguishable from confidence in a root cause, impact, prediction, or outcome.
- Low confidence can result in observation, further investigation, human verification, or escalation without being treated as an execution failure.
- Rules for downstream use of confidence results define the required semantics, scope, time context, and evidence lineage.
- Policy cannot treat confidence alone as authorization.
- Observed and potential impacts are distinguishable in outputs and reporting.
- Confidence can be explained from recorded evidence without implying unsupported precision.

## 13. Open decisions

1. Which confidence types are required for the first engine contracts, including observation, hypothesis, impact, prediction, action, and outcome confidence?
2. Is confidence numeric, categorical, calibrated, or a combination?
3. Must every engine output include confidence, or only derived/probabilistic claims?
4. What quality, freshness, independence, and source-trust metadata are mandatory, and how should source trust qualify confidence without being folded into it?
5. How are evidence items combined, how are correlated sources identified, and how is contradictory evidence handled?
6. How should confidence decay, expire, or be revalidated as evidence ages or the environment changes?
7. How are knowledge trust and confidence related without conflating them?
8. What evidence is retained or redacted for privacy and audit?
9. What is the minimum explanation required for a policy decision?
10. Should confidence be represented as one universal measure or distinct domains such as observation, hypothesis, impact, prediction, action, and outcome confidence (ARCH-021)?
11. How do incomplete inputs and lost engine dependencies affect confidence versus coverage, and how are these dimensions composed for policy (ARCH-024)?
12. Can downstream engines consume an upstream confidence result as evidence, or must they always reason from underlying evidence; what provenance and semantic guarantees are required (ARCH-028)?

## 14. Dependencies

RFC-0001 supplies event provenance and versioning. RFC-0002 supplies engine identity and determinism context. RFC-0003 supplies entity scope. RFC-0006 and RFC-0009 provide validation and knowledge evidence. RFC-0008 consumes evidence during investigation. RFC-0010 must treat confidence as one policy input rather than authorization.
