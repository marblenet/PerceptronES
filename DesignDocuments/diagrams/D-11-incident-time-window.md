# D-11 — Incident Report Time Window

**Purpose:** Define the records needed to reconstruct the requested interval: one minute before an incident, the incident and response, and five minutes after mitigation.

```mermaid
sequenceDiagram
    participant Source as Source systems
    participant Bus as Event/history store
    participant Engine as Engines and core mission
    participant Human as On-call engineer
    participant Action as Policy-gated action adapter
    participant Report as Incident report

    Note over Source,Report: T-60 seconds through T+300 seconds after mitigation
    Source->>Bus: Continuous timestamped observations and operational events
    Bus->>Bus: Retain bounded pre-incident window
    Source->>Bus: Incident trigger with scope and correlation identity
    Bus->>Engine: Incident evidence and recent history
    Engine->>Bus: Findings, lineage, uncertainty, and recommendations
    Bus->>Human: Explainable incident view and escalation
    Human->>Bus: Attributed acknowledgement or decision
    Human->>Action: Request action through policy boundary
    Action->>Bus: Authorization and execution outcome
    Source->>Bus: Post-mitigation observations and service impact
    Bus->>Bus: Capture at least five minutes after mitigation
    Bus->>Report: Export events, gaps, provenance, and timeline
    Report->>Report: Mark missing data, clock uncertainty, and retention gaps
```

**Interpretation:** The time window anchors to explicit incident and mitigation timestamps, not report generation time. The record should include observation/event time, ingestion time, source, scope, evidence lineage, recommendation, human decisions, policy result, execution, and post-action verification. If a source is unavailable or the retention window is incomplete, the report states the gap rather than interpolating a fact.

**Use this view when:** designing an incident report that allows a reviewing engineer to reconstruct what Perceptron knew before and after a mitigation.
