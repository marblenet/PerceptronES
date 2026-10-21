# D-04 — Evidence and Confidence Lifecycle

**Purpose:** Distinguish evidence quality and coverage from confidence in a particular claim.

```mermaid
flowchart LR
    Inputs["Observations, events,\ntool results, history,\ntwin/simulation, human review"]
    Scope["Normalize identity, scope,\nsource, time, freshness"]
    Evidence["Evidence set"]
    Support["Supporting evidence"]
    Contradict["Contradictory evidence"]
    Missing["Missing, stale,\ninconclusive evidence"]
    Claim["Claim or hypothesis"]
    Confidence["Claim-specific confidence\nand limitations"]
    Recommendation["Recommendation and expected outcome"]
    Policy["Policy evaluates action risk\nusing evidence, scope, coverage,\nfreshness, identity, and rules"]

    Inputs --> Scope --> Evidence
    Evidence --> Support --> Claim
    Evidence --> Contradict --> Claim
    Evidence --> Missing --> Claim
    Claim --> Confidence
    Claim --> Recommendation
    Confidence --> Recommendation
    Recommendation --> Policy

    NotTruth["Policy constraint is not evidence\nthat a factual hypothesis is true"]
    Policy -.-> NotTruth
    Confidence -.-> NotTruth
```

**Interpretation:** Confidence belongs to a claim and its evidence domain; it is not a universal health score or authorization. Missing evidence can lower coverage or make evaluation impossible. Policy consumes confidence where appropriate but independently evaluates action risk and permissibility.

**Use this view when:** reviewing an investigation or proposed action and asking which facts support it, which contradict it, and what remains unknown.
