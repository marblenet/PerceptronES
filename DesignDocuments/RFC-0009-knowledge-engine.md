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
- Provide a Knowledge Store with Core, Local, and Community sources, operator-configured source weights, and determination patterns that interpret metric combinations, prioritized ahead of additional Tap Engines (6.3).

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

### 6.3 Knowledge Store, Core/Local/Community sources, and determination patterns

The Knowledge Store is a first-class subsystem that translates information, determinations, and historical experience into assessments. **Knowledge creation takes priority over additional Tap Engines:** metrics become assessments only when combinations of observations are interpreted ("what do combinations of metrics mean?"). The target is common operational patterns, not exhaustive combinations (50! is computationally impossible and operationally useless). Examples: high CPU with normal memory and disk may be expected processing load; low CPU and network with high disk wait may be a storage bottleneck.

**Sources.** Knowledge comes from **Core** (Perceptron engineering: default assessments, patterns, and operational experience), **Local** (operators and organizations: company-specific systems, custom policies, historical knowledge), and **Community** (pull requests, Knowledge Store packages, contributions). Operators configure trust preferences as relative weights, for example Core 60 / Local 30 / Community 10, or Core 20 / Local 70 / Community 10. Assessments preserve provenance, source, coverage, and confidence where defined when sources disagree.

**Community governance.** Votes, reviews, comments, and confidence signals may influence review and visibility, but popularity is not truth: community voting never becomes production behavior automatically, and community knowledge follows the same review and provenance rules as any knowledge item.

**Initial domains** (priority order): CPU (saturation, runaway processes, normal utilization); memory (pressure, swapping, leaks, cache growth); storage (saturation, latency, full, inode exhaustion); network (congestion, packet loss, bandwidth saturation, connection storms); services (restart loops, crashes, dependency failures); security (missing patches, critical CVEs, expired certificates, unsupported software).

**Prototype semantics (implemented).**
- A determination pattern has `all` and optional `any` conditions over cataloged metrics (`>=`, `>`, `<`, `elevated`, `normal`); `elevated` and `normal` apply only to metrics the pressure engine evaluates. A pattern may carry a recommendation.
- Unavailable, stale, or missing evidence never matches: unknown is neither a match nor health. Critical evidence raises a WARNING pattern to CRITICAL. Each determination cites its evidence events, knowledge source, and weight.
- Core patterns use IDs `core.<domain>.<name>` (19 shipped: CPU, memory, storage, partial network, system limits). Local patterns (`knowledge.local_patterns`) and community patterns (`knowledge.community_patterns`) use the same format; `knowledge.disabled_patterns` removes patterns.
- `knowledge.weights` are relative values normalized to percent (default Core 30 / Local 60 / Community 10). When sources define the same pattern ID, the highest-weight source wins (ties: local, core, community); a zero weight ignores a source and all-zero weights select the defaults. The weight is a source-precedence rule, not a confidence value (RFC-0004).
- Not implemented: Services patterns (no inputs yet), Security patterns over host metrics, package distribution and signing, voting and review workflow, and feeding determinations into Layer-3 health and mission. Rate-based patterns use short two-sample rates and are noisy. Absence of a determination is not evidence of health.

Knowledge configuration is version-controlled operational knowledge: weights, thresholds, and store priorities must be auditable, reproducible, and reviewable, and the system changes only when humans change configuration, knowledge, or code.

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
13. How are Core/Local/Community packages distributed, signed, reviewed, and versioned, and how are community votes used for review only (6.3)?
14. When do determinations feed Layer-3 assessments, and which Services and Security patterns are first?

## 15. Dependencies

RFC-0001 provides event/provenance contracts. RFC-0003 provides entity references. RFC-0004 distinguishes knowledge evidence and current confidence. RFC-0006 provides twin-validation evidence. RFC-0008 consumes knowledge during investigation. RFC-0010 governs any action suggested by stored procedures.
