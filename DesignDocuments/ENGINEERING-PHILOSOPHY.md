# Perceptron SRE Engineering Philosophy

**Status:** Engineering guidance for review  
**Audience:** Engineers, operators, architects, and AI coding agents  
**Related:** [Overall Architecture](01-overall-architecture.md), [Implementation Roadmap](../Implementation/IMPLEMENTATION-ROADMAP.md)

This document explains the rationale behind Perceptron SRE's engineering guardrails. It guides decisions where an RFC does not yet prescribe an implementation; it does not silently resolve the RFCs' open decisions or claim that planned capabilities exist.

## Why Perceptron SRE exists

Operational systems often provide either narrow deterministic rules ("if X, do Y") or flexible probabilistic analysis. Rules are useful and reproducible but can miss context when signals are partial or interact. Broad generative systems can interpret varied inputs, but their outputs are probabilistic and are not, by themselves, a dependable authority for operational conclusions or actions.

Perceptron SRE aims to make a bounded, inspectable reasoning layer between raw signals and operator response. Small components evaluate declared inputs and publish scoped results; the platform composes those results with evidence, coverage, freshness, and provenance. The objective is not to replace operators or existing monitoring. It is to reduce avoidable investigation work and provide a faster, more useful handoff when a human must decide.

## What "perceptron" means here

The name describes an architectural approach: focused computational units contribute to a larger assessment through explicit contracts. It does **not** mean the current prototype is a trained neural network, nor does the term alone establish a mathematical model, calibrated confidence, or causal inference algorithm.

A one-condition binary rule can answer whether one threshold was crossed. A bounded reasoning chain can additionally preserve the measurement's time and scope, identify missing or stale signals, combine related assessments, and explain how the resulting mission status follows from the available evidence. Threshold rules remain useful components; the distinction is the composition and traceability of evidence, not a claim that every individual rule is inherently intelligent.

The current Go prototype implements explicit rules for a narrow host pressure/health chain. It does not currently perform probabilistic learning, general root-cause inference, or LLM-based operational decision-making. New engines must declare their contracts and semantics; claims about confidence, learning, or determinism must match their actual behavior and tests.

## Reliability principles

### Unknown is better than wrong

Missing, stale, unavailable, contradictory, malformed, or insufficient evidence must remain visible. It must not silently become zero, healthy, safe, permitted, or successful. Unknown is a valid result. Invalid engine output and runtime failures must be surfaced explicitly rather than converted into a success-shaped assessment.

### No default green

GREEN is earned only when the required evidence for the stated mission scope and objective is present, valid, sufficiently fresh, and within the declared criteria. A running process, loaded engine, successful request, or absence of an alert is not proof of health. An engine-free or incomplete assessment must not claim GREEN.

### Determinism is a scoped contract

Where an engine declares deterministic behavior, the same normalized inputs, engine version, and configuration must yield the same decision and explanation content, apart from explicitly variable metadata such as evaluation timestamps or generated identifiers. Any non-deterministic engine must declare why and identify the affected output. Replay, audit, and certification claims apply only to the declared scope and validated implementation.

### Evidence and authority are separate

Observations, hypotheses, recommendations, validation, policy authorization, execution, and verified outcomes are distinct. Neither a high confidence value nor a successful simulation, twin, or prior incident grants authority to act. Human oversight and escalation remain valid intended outcomes; automation is introduced only for a bounded action class after enforcement, authorization, and recovery behavior are proven.

### Security is part of reliability

Security-relevant conditions—such as patch state, certificate validity, hardening, unsupported software, and configuration exposure—should eventually be represented as first-class operational evidence. They must follow the same rules for provenance, scope, freshness, coverage, and uncertainty as other domains. This is a future engine/integration direction, not a capability of the current host prototype.

### Test behavior, not only code paths

Tests are product assurance. Each reasoning or action capability should include normal, boundary, missing, stale, contradictory, invalid, and failure cases as applicable. Deterministic fixtures should support replay. A failure must not look healthy, and a degraded dependency must make its effect on coverage or confidence visible.

### Simulate before production; learn with review

Synthetic inputs and isolated simulations should make important operating and failure scenarios reproducible before production exposure. Where an incident or near miss is later converted into a regression scenario or knowledge item, retain its source, scope, and review status. Historical outcomes may inform future reasoning; they do not automatically validate or promote a procedure.

### Prefer dependable engineering over premature autonomy

Perceptron SRE is intended for long-lived, operationally consequential use. The useful analogy is mission-critical engineering: favor understood behavior, explicit failure modes, observability, limited privileges, ownership, rollback, and upgrade compatibility over adding capability quickly. This mindset does not claim that the project is certified to aerospace, medical, or industrial standards. Production changes remain gated by policy, approval where required, and post-action verification.

## Applying this guidance

When implementing a feature:

1. Identify the applicable RFC, unresolved decisions, and observable acceptance criteria.
2. State the scope and semantics of the evidence and result; keep status, confidence, coverage, freshness, trust, and authorization distinct.
3. Specify behavior for missing, stale, contradictory, malformed, and unavailable inputs before implementing a favorable outcome.
4. Add deterministic fixtures and tests for expected and adverse behavior; use isolated simulation for workflows whose effects cannot be safely tested on production.
5. Preserve provenance and explain limitations. Do not describe a prototype, declaration, or process boundary as a production guarantee, security control, or completed capability.
6. Keep human review and failure escalation usable even when automated reasoning or integrations are degraded.

These principles are guardrails, not a substitute for domain-specific hazard analysis or the approval of an RFC.
