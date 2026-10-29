# RFC-0008: Investigation Framework

**Status:** Architecture review ready  
**Category:** Reasoning and operations  
**Parent:** [Overall Architecture](01-overall-architecture.md)  
**Related RFCs:** RFC-0001 Event Bus and Event Model; RFC-0002 Engine Runtime; RFC-0003 Operational Entity Model; RFC-0004 Confidence and Evidence; RFC-0007 Mission Assurance; RFC-0009 Knowledge Engine; RFC-0010 Policy Engine

**Visual references:** [Investigation and knowledge workflow](diagrams/D-06-investigation-knowledge.md); [evidence and confidence lifecycle](diagrams/D-04-evidence-confidence.md); [incident report time window](diagrams/D-11-incident-time-window.md).

## 1. Summary

This RFC proposes an investigation process that reduces uncertainty about an operational condition and produces traceable understanding. Investigation gathers evidence, consults knowledge, evaluates and preserves multiple candidate causes and alternatives, and may produce recommendations for policy review. Finding one definitive root cause is not required for a useful or successful investigation: narrowing possibilities, ruling out alternatives, or reducing uncertainty about impact may be valuable outcomes.

Investigation must not equate a detected symptom with a proven root cause. Unknown or unresolved is a valid outcome and is preferable to an unsupported conclusion. Recommendations are not execution.

## 2. Problem and motivation

Traditional alerting often stops at declaring that a threshold was crossed. Perceptron SRE aims to gather the evidence an SRE would use to answer what happened, why, when, who was affected, and what response is appropriate.

## 3. Goals

- Define the lifecycle and outputs of an investigation.
- Represent investigation coverage, confidence, and freshness as distinct dimensions.
- Support multiple and alternative root-cause hypotheses.
- Preserve supporting, contradictory, and inconclusive evidence.
- Use bounded toolbox operations and knowledge retrieval.
- Accept human findings and observations as attributed evidence.
- Produce explainable findings and recommendations with uncertainty.
- Allow investigation to request or consume validation evidence, including twin results, where relevant.
- Keep investigation separate from authorization and execution.

## 4. Non-goals

- Guaranteeing that every incident has one discoverable root cause.
- Treating a ranked hypothesis as established fact.
- Allowing investigative tools to mutate production without policy.
- Mandating a particular AI model or orchestration implementation.

## 5. Candidate workflow

```text
Signal / incident trigger
        ↓
Scope and context collection
        ↓
Evidence gathering (events, tools, service state)
        ↓
Knowledge retrieval
        ↓
Candidate hypotheses and alternatives
        ↓
Evidence comparison and confidence
        ↓
Findings / recommendation / unresolved questions
        ↓
Policy or human review
```

The stages may be iterative. New or contradictory evidence can reopen or revise a hypothesis. This is not a fixed sequential pipeline, and investigation may end as inconclusive.

Candidate lifecycle states include **created**, **gathering evidence**, **forming hypotheses**, **validating**, **awaiting input**, **completed**, **inconclusive**, and **abandoned**. These are illustrative states, not an approved state machine. Investigations may skip, repeat, or revisit stages; transitions, terminal-state semantics, reopening, and ownership remain open.

## 6. Investigation record

An investigation should be linkable to an incident or trigger and preserve, where applicable:

- Scope, relevant entities, time window, and initiating signals.
- Lifecycle state and state-transition history.
- Investigation coverage: the regions, entities, time windows, evidence sources, and reasoning scope actually examined, including known gaps.
- Investigation freshness: when the assessment was last updated and whether its evidence remains current for the stated scope.
- Questions asked and evidence requested.
- Toolbox operations invoked, results, errors, and timing.
- Knowledge items retrieved and their trust context.
- Candidate causes, alternatives, and causal relationships.
- Supporting, contradictory, and inconclusive evidence per hypothesis.
- Human findings, observations, validation, or overrides, with attribution, time, scope, and provenance.
- Observed and potential impact, kept distinct.
- Finding, hypothesis, and overall conclusion confidence where applicable, with semantics, evidence lineage, and limitations per RFC-0004.
- Recommendation, if any, separately from findings.
- Policy or human decisions and subsequent outcome links.

Coverage, confidence, and freshness are distinct: a well-supported finding may apply to only a small examined scope, and a once-current investigation may become stale as conditions change. An investigation record must not imply complete scope or current validity merely because it reached a terminal state. This is a candidate record, not an approved schema. Sensitive evidence may be referenced through access-controlled storage rather than copied into an investigation event.

## 7. Root-cause and hypothesis handling

The framework must support zero, one, or multiple candidate causes. It must preserve alternatives as useful findings, not discard them as noise, and avoid collapsing correlation into causation. Hypotheses may be ranked where useful, but rankings must not imply certainty without evidence. If a ranking or confidence is emitted, its meaning, evidence, and limitations must be explainable.

Contradictory evidence—such as an empty queue when a queue backlog is proposed as the cause—must be retained and considered. Evidence that is unavailable, stale, or inconclusive must be recorded as such rather than treated as support.

Human observations and judgments may support, contradict, or qualify hypotheses; they must retain who provided them, when, and for what scope. Human validation or override must remain distinguishable from machine-generated evidence and from policy authorization.

Unknown is preferable to a wrong or unsupported cause. Investigation may validly conclude that the cause remains unknown, while still reporting what was ruled out, what alternatives remain, what impact is understood, and what evidence is missing.

No causal inference algorithm or confidence update formula is selected.

### 7.1 Why chain and How chain (Proposed)

A completed incident report should link two chains to the same incident scope and time window (see D-11 and [D-13](diagrams/D-13-why-how-chains.md)):

- **Why chain (backward):** observed impact -> proximate cause -> contributing factors -> root cause -> process/control gap. Each link is a hypothesis or finding with supporting and contradictory evidence, provenance, and limitations.
- **How chain (forward):** mitigation action -> state change -> service/mission recovery -> user/impact normalization -> why the mitigation worked -> prevention. Each link cites post-action observations, not only the action itself.

Requirements:

1. Depth is evidence-driven; five links per chain is a minimum goal, not a required count. A chain may end in "unknown" and must say what evidence is missing.
2. Each How link is classified **temporary** or **permanent** mitigation, or **unclassified** when unknown. Permanent mitigations should link to the Why-chain gap they address.
3. Human actions (`human_action` origin, PES procedures) appear as attributed How evidence; they remain distinct from machine findings and from policy authorization.
4. Recovery is shown by post-mitigation observations (at least the D-11 window), never assumed from action success ("no default green").
5. Link confidence, where emitted, follows RFC-0004 semantics; no confidence formula is selected.
6. Chains are derived from retained events and are replayable; simulated or replayed incidents are labeled by origin and must not be presented as live history.
7. Domain-neutral terminology is used in the core model; industry profiles (for example IT, industrial, telecom, healthcare technology, utilities) may map fields to their reporting conventions without changing evidence rules.

Reports should expose a **Why** view and a **How** view, with the how-to-repeat steps shown as knowledge candidates, not authorization.

## 8. Toolbox execution

Toolbox components provide bounded evidence such as CPU, memory, disk, process, DNS, network, database, and queue diagnostics. Investigation may request tools based on policy, capability, and relevance.

Before an accepted implementation, tool contracts must specify permissions, targets, deadlines, resource limits, data handling, failure semantics, and whether the operation is read-only or mutating. Mutating tools require explicit policy-governed authorization and must not be invoked merely because an investigation is active.

## 9. Knowledge use

Knowledge retrieval can surface prior incidents, FAQs, runbooks, near misses, and remediation history. Retrieved knowledge is evidence with age, trust, provenance, and applicability limitations; it is not automatically correct or safe to execute.

RFC-0009 defines the knowledge lifecycle and trust model. Retrieved knowledge may suggest hypotheses or diagnostic steps, but similarity to a prior incident does not establish the current cause.

## 10. Recommendations and handoff

An investigation may produce:

- Findings and unresolved questions.
- A cause hypothesis or multiple alternatives.
- Risk/impact assessment.
- A proposed next diagnostic step.
- A remediation recommendation with expected outcomes and risks.
- An escalation recommendation.

Recommendation, validation, policy authorization, and execution are separate stages. Investigation output cannot authorize a production change.

Investigation may request additional validation, such as testing a proposed remediation or hypothesis in an applicable twin. Twin validation is scoped evidence governed by RFC-0006; it can update investigation findings or confidence but is neither required for every investigation nor a substitute for policy authorization. The investigation must preserve the tested scope, outcome, confidence, and limitations of any validation result.

## 11. Failure, safety, and observability

Investigation must report partial completion, tool failures, timeouts, missing inputs, conflicting sources, and resource exhaustion. It must not present incomplete evidence as a complete investigation. Limits and cancellation behavior must prevent investigation fan-out from overwhelming the system or target infrastructure.

Investigation state transitions and evidence access must be auditable. Repeated execution and replay must not cause side effects through tools.

## 12. Acceptance criteria for an accepted RFC

- An investigation can start from a defined event or incident reference and record its scope and lifecycle.
- Multiple hypotheses and alternatives can be represented.
- Supporting, contradictory, unavailable, and inconclusive evidence are distinguishable.
- Investigation coverage, confidence, and freshness are represented distinctly and their limitations remain visible.
- Human-provided evidence and decisions preserve attribution and provenance.
- Tool invocation is bounded, permissioned, and reports errors and partial results.
- Knowledge is presented with provenance and trust context.
- Investigation can validly conclude that a cause is unknown or unresolved without fabricating certainty.
- Alternative explanations and the evidence for and against them remain available in the investigation output.
- Investigation explanations trace findings to supporting and contradictory evidence, alternatives, and confidence.
- Twin validation, when used, remains scoped evidence and cannot authorize execution.
- Recommendations are represented separately from policy decisions and execution.
- Investigation reconstruction identifies its inputs, engine/tool versions, and outputs.
- Completion, inconclusive, and abandoned outcomes have explicit semantics.

## 13. Open decisions

Note: the prototype's advisory recommendations are Layer-4 Recommendation outputs (RFC-0002 6.1); they are not investigations, and policy (RFC-0010 8.1) gates them.

1. What signals or policies initiate investigations?
2. Who/what owns investigation lifecycle and incident association?
3. How are tools selected, sequenced, bounded, and permissioned?
4. What hypothesis structure and causal links are required?
5. How should evidence update confidence and rankings?
6. How are conflicting system-of-record claims represented?
7. Which investigation outputs are recommendations versus requests for more evidence?
8. What retention and access controls apply to investigation evidence?
9. How are investigation coverage, confidence, freshness, and completeness represented (ARCH-036)?
10. What constitutes investigation completion, including inconclusive or abandoned outcomes, and who may close or reopen an investigation (ARCH-037)?
11. Which lifecycle states and transitions are required, and how are human evidence, validation requests, and new contradictory evidence incorporated?
12. What is the Why/How chain schema (link types, branching, cross-links between a How link and the Why gap it addresses)?
13. How are temporary versus permanent mitigations classified and who confirms permanence?
14. How are industry profiles defined and versioned without altering core evidence rules?

## 14. Dependencies

RFC-0001 provides events and replay; RFC-0002 provides engine execution; RFC-0003 provides entity references; RFC-0004 defines evidence and confidence; RFC-0009 provides knowledge; RFC-0010 governs tool capabilities and any action authorization.
