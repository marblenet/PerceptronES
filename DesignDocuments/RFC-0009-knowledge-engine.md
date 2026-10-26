# RFC-0009: Knowledge Engine

**Status:** Architecture review ready  
**Category:** Reasoning and operations  
**Parent:** [Overall Architecture](01-overall-architecture.md)  
**Related RFCs:** RFC-0001 Event Bus and Event Model; RFC-0003 Operational Entity Model; RFC-0004 Confidence and Evidence; RFC-0006 Twin Architecture; RFC-0008 Investigation; RFC-0010 Policy Engine

**Visual references:** [Investigation and knowledge workflow](diagrams/D-06-investigation-knowledge.md); [evidence and confidence lifecycle](diagrams/D-04-evidence-confidence.md); [incident report time window](diagrams/D-11-incident-time-window.md).

## 1. Summary

This RFC proposes Perceptron SRE's organizational memory for incidents, FAQs, runbooks, near misses, and remediation history. Knowledge stores claims, observations, procedures, and lessons—not truth. Each item remains subject to provenance, trust, scope, freshness, applicability, and validation; it is not automatically true or an authorization to perform its suggested action.

Knowledge is evidence about prior events and practice. Trust in a knowledge item is distinct from confidence that it applies to a current case: a trusted runbook may have low applicability to a particular incident. The storage model, retrieval technology, trust formula, and lifecycle rules remain open.

## 2. Problem and motivation

Operational reasoning benefits from prior incidents and successful procedures, but traditional knowledge bases can preserve stale or contextually invalid instructions indefinitely. Perceptron SRE intends to track how knowledge has performed and whether it has been revalidated, including in twins.

## 3. Goals

- Define candidate knowledge objects and lifecycle.
- Track provenance, age, use, outcomes, and validation.
- Preserve knowledge lineage and distinguish item age from validation freshness.
- Represent applicability scope and coverage, including known gaps.
- Distinguish an empty knowledge result from retrieval/access failures and other known gaps.
- Preserve competing or invalidated knowledge with its history and provenance.
- Support trust decay and evidence-based renewal.
- Surface applicability and limitations during investigation.
- Preserve near misses as learning material.
- Close the learning loop from incidents and investigations through validation and outcomes to reviewed knowledge updates.
- Support converting reviewed incidents and near misses into reproducible regression scenarios without automatically promoting procedures or claims.

## 4. Non-goals

- Treating stored knowledge as authoritative truth.
- Automatically executing runbooks.
- Selecting a database, vector index, language model, or retrieval algorithm.
- Defining a universal numeric trust score in this draft.

## 5. Candidate knowledge objects

- **Incident:** a recorded operational event, timeline, evidence, impact, decisions, and outcome.
- **Runbook:** a procedure or diagnostic/remediation sequence with applicability and risk constraints.
- **FAQ:** a curated question and answer with sources and review state.
- **Near miss:** an event that approached failure or risk but recovered or avoided customer impact.
- **Remediation history:** proposed and executed changes with validation, policy, results, and side effects.

Objects may link to one another and to customers, services, regions, zones, records, incidents, and engine versions. These are conceptual types; schemas and ownership remain open.

## 6. Knowledge lifecycle and governance

The source context proposes:

```text
Created → Used → Validated → (renewed / remains trusted)
       ↘ stale or invalidated → Expired / requires review
```

This is not a final state machine. Knowledge may be created by a human, imported from an incident, or derived from outcomes, subject to review and provenance. Use does not itself prove correctness.

Operational lifecycle and trust are related but separate. Candidate lifecycle labels include **draft**, **approved**, **stale**, **invalidated**, **expired**, and **archived**; **trusted** should describe an assessed trust condition rather than imply approval or lifecycle state. The labels, transitions, and relationship to trust remain open. Invalidation or supersession must not silently erase the prior item or its history; retained historical knowledge can help explain what was believed or applicable at an earlier time.

Knowledge records should preserve authorship and governance provenance where available, including author/creator, reviewer, approver, validation history, and subsequent revisions. Roles, required approvals, and authority remain to be specified. Automated learning may propose updates but must not silently rewrite approved operational procedures.

### 6.1 Knowledge lineage and learning loop

Knowledge items should link to their origins and subsequent evidence where applicable, such as:

```text
Incident
    ↓
Investigation
    ↓
Proposed knowledge update
    ↓
Review / approval
    ↓
Twin or other validation
    ↓
Operational outcome
    ↓
Scoped trust renewal or review
    ↓
Future investigation
```

This is a conceptual learning loop, not an automatic promotion pipeline. Each link must preserve provenance and time context so a consumer can determine where a runbook or lesson came from and which evidence supported its revision or trust change.

Where an incident or near miss becomes a regression scenario, preserve its source, scope, relevant versions/configuration, expected behavior, and any redactions or synthetic substitutions. The scenario must remain distinguishable from a live observation, and a passing replay validates only the encoded scenario—not the universal correctness or current applicability of a runbook. Creating, editing, or approving operational knowledge remains subject to review.

### 6.2 Why/How chains as knowledge input (Proposed)

A completed incident's Why chain (how to recognize the problem) and How chain (how it was mitigated, and why that worked) are candidate inputs for knowledge items, runbooks, simulation scenarios, and regression tests. Knowledge derived from them must keep links to the source incident, evidence, mitigation class (temporary or permanent), and review status. Temporary mitigations must not be recorded as prevention. Promotion to approved knowledge requires review per section 6; a successful past mitigation does not establish that it is safe or applicable now.

## 7. Trust model

Candidate factors include:

- Age since creation and last validation.
- Number and context of uses.
- Historical success and failure outcomes.
- Twin validation results.
- Human verification.
- Applicability to the current system/version/configuration.
- Known adverse or contradictory outcomes.

Trust may decay and may be renewed by new evidence. A twin pass can increase trust only within the conditions and scope actually tested. Knowledge trust assesses the historical item; confidence assesses a current claim or its applicability to the current case. These can differ: a trusted runbook may still be a poor match for the present incident. Neither trust nor confidence is permission to act.

No formula, weighting, decay rate, expiration period, minimum sample size, or calibration is selected.

## 8. Retrieval, applicability, and coverage

Knowledge retrieval should return provenance and applicability context, not just text. Candidate factors include service/region, version, failure mode, action scope, prior outcome, age, freshness, and validation status. Age since creation and freshness since validation or relevant re-confirmation are distinct: an old item can have recent validation, while a recently created item can already be stale after a system change.

Knowledge applicability has a scope. A runbook validated in one region or software version does not thereby have validated coverage in another; an incident pattern observed in a control plane does not by itself establish applicability to the data plane. Retrieved knowledge should expose its declared and validated scope, known coverage gaps, freshness, trust, and applicability limitations. Coverage, trust, freshness, and current-case confidence must not be collapsed into one value.

The retrieval method and ranking algorithm are open. Retrieval results must allow investigation to distinguish exact precedent from a superficially similar case.

## 9. Knowledge gaps and unknowns

Knowledge absence and knowledge-system failure are different outcomes. A retrieval result should distinguish, as applicable:

- **No known precedent**, **no known procedure**, or **no validated remediation** for the requested context.
- **Incomplete coverage**, where knowledge exists but does not cover the requested region, version, failure mode, or action scope.
- **Knowledge retrieval failure**, such as an unavailable or failed source.
- **Knowledge access restricted**, where the requester cannot access potentially relevant items.
- **Knowledge expired or stale**, where relevant material exists but its current applicability or trust is limited.
- **Knowledge contradiction**, where applicable items make conflicting claims or recommend incompatible procedures.
- **Knowledge development gap**, where recurring operational need exists but no approved or validated knowledge has been developed.

These are candidate gap categories, not a final result schema. A retrieval failure, access denial, or incomplete scope must not be presented as “no known solution.” A confirmed absence may itself be a useful finding and may prompt investigation, validation, runbook creation, or future engineering work. Known gaps should be preserved rather than silently converted into certainty or absence of risk. Classification and representation remain open under ARCH-040.

## 10. Validation and renewal

Knowledge can be revalidated through a twin, simulation, observed production outcomes, or human verification, depending on the knowledge type and risk. Validation evidence must identify what was tested and its scope. Failed or contradictory outcomes should reduce trust or mark an item for review according to a future policy.

Validation does not make a runbook universally applicable and does not bypass RFC-0010.

## 11. Near misses and learning

Near misses should be retained even if no customer impact occurred. They can reveal pressure, unsafe conditions, or successful recovery paths. The platform should preserve the difference between observed near miss facts and inferred lessons.

How lessons are approved, edited, and attributed remains open; automated learning must not silently rewrite authoritative runbooks. Near-miss facts, investigation findings, inferred lessons, proposed knowledge changes, and approved knowledge must remain distinguishable and linked through provenance.

## 12. Failure, security, and privacy

Knowledge retrieval unavailable or corrupt must be visible and must not be reported as “no known issue” without distinguishing source failure from an empty result. Knowledge integrity, access control, tenant separation, secret handling, retention, and deletion require explicit requirements.

Runbooks may contain privileged commands or sensitive operational details. Retrieval is not authorization to execute them.

## 13. Acceptance criteria for an accepted RFC

- Knowledge object types, provenance, and lifecycle states are specified.
- Knowledge lineage links items to originating incidents, investigations, validation, revisions, and trust-renewal evidence where applicable.
- Author, reviewer, approver, and validation history are attributable where available.
- Age and freshness, declared applicability scope and coverage, trust, and current-case confidence are represented distinctly.
- Trust decay and renewal rules are testable and distinguishable from incident confidence.
- Twin/human/production validation evidence is scoped and linked.
- Stale, expired, failed, contradictory, and unavailable knowledge are surfaced distinctly.
- Competing, invalidated, and superseded knowledge remains retained with its provenance and history.
- Empty results, incomplete knowledge coverage, retrieval failure, access restriction, and known knowledge-development gaps are distinguishable.
- Near misses can be recorded with observed facts separated from inferred lessons.
- Learning from incidents through investigations, validation, outcomes, and reviewed knowledge updates is traceable and does not silently alter approved knowledge.
- Retrieval does not automatically execute knowledge or bypass policy.
- Integrity, access, retention, and privacy controls are defined.

## 14. Open decisions

1. Which knowledge objects are in the first release?
2. What system owns each knowledge type and its canonical version?
3. How are trust scores calculated, decayed, capped, and renewed?
4. What constitutes successful use or validation?
5. Who may create, edit, approve, invalidate, or expire knowledge?
6. How are applicability and system/version drift represented?
7. How are near misses reviewed and converted into approved knowledge?
8. What retrieval and privacy model is appropriate?
9. How are knowledge freshness, coverage, applicability, and trust represented independently (ARCH-038)?
10. How are conflicting, superseded, or invalidated knowledge items represented and retained (ARCH-039)?
11. How are knowledge gaps distinguished from empty results, incomplete coverage, access restriction, retrieval failure, and unavailable sources (ARCH-040)?
12. What lifecycle states and transitions apply, and how do lifecycle status, approval, and trust relate?

## 15. Dependencies

RFC-0001 provides event/provenance contracts. RFC-0003 provides entity references. RFC-0004 distinguishes knowledge evidence and current confidence. RFC-0006 provides twin-validation evidence. RFC-0008 consumes knowledge during investigation. RFC-0010 governs any action suggested by stored procedures.
