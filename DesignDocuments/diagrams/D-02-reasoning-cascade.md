# D-02 — Reasoning Cascade and Event Lineage

**Purpose:** Illustrate the progression from observed signals to higher-level operational conclusions, while preserving uncertainty and provenance.

```mermaid
flowchart TB
    Source["Source system claim or measurement"]
    Normalize["Input adapter normalizes units, time, scope, source"]
    Observation["Observation event"]
    Pressure["Pressure assessment"]
    Health["Component or service health"]
    Correlation["Correlation and competing hypotheses"]
    Impact["Observed or potential customer impact"]
    Investigation["Investigation findings and recommendation"]
    Mission["Core mission assessment"]
    Policy["Policy evaluation of a proposed action"]
    Outcome["Authorized external execution and verified outcome"]

    Source --> Normalize --> Observation --> Pressure --> Health
    Health --> Correlation --> Impact
    Health --> Mission
    Impact --> Mission
    Correlation --> Investigation
    Investigation --> Mission
    Investigation --> Policy
    Mission --> Policy
    Policy --> Outcome
    Outcome -->|"outcome evidence; not truth rewrite"| Observation

    Unknown["Missing, stale, contradictory, or unavailable evidence"]
    Unknown -.-> Pressure
    Unknown -.-> Health
    Unknown -.-> Impact
    Unknown -.-> Mission

    Lineage["Every derived event carries producer/version, scope, time,\nconfiguration, and links to contributing events"]
    Observation -.-> Lineage
    Pressure -.-> Lineage
    Health -.-> Lineage
    Correlation -.-> Lineage
    Impact -.-> Lineage
    Mission -.-> Lineage
```

**Interpretation:** The cascade is conceptual and is not a mandatory fixed chain. Engines may consume events from earlier stages, while the core mission service combines relevant inputs. Missing evidence can make conclusions unknown or reduce coverage; it is never silently converted into healthy. Policy evaluates action permissibility, not factual truth.

**Layer naming:** implemented stages use `Layer-<number> <Label>`: Observation = Layer-0 Input; Pressure = Layer-1 Information; knowledge-pattern determinations = Layer-2 Determinations; health, impact and mission = Layer-3 Assessment; investigation and recommendation = Layer-4 Recommendation; Policy = Layer-5; Action = Layer-6; verified outcome = Layer-7 Outcome; knowledge updates = Layer-8 Learning. Correlation is a conceptual stage with no assigned layer yet (Open).

**Use this view when:** explaining an assessment to an operator or tracing a claim back to its evidence.
