# D-06 — Investigation and Knowledge Workflow

**Purpose:** Show how a symptom can trigger an evidence-oriented investigation without prematurely declaring a root cause or executing a runbook.

```mermaid
flowchart TD
    Signal["Detection or mission degradation"]
    Case["Open scoped investigation record"]
    Evidence["Collect event lineage and current evidence"]
    Gap{"Evidence sufficient to\nnarrow hypotheses?"}
    Tools["Request bounded diagnostic tools"]
    ToolPolicy["Core capability broker and\npolicy check (future gated capability)"]
    Results["Tool results with source,\ntime, scope, and errors"]
    Knowledge["Retrieve applicable incidents,\nFAQs, and runbooks"]
    Trust["Evaluate knowledge provenance,\nfreshness, trust, and applicability"]
    Hypotheses["Maintain competing hypotheses,\nsupporting and contradictory evidence"]
    Outcome{"Investigation outcome"}
    Recommend["Recommendation with rationale\nand evidence references"]
    Escalate["Human handoff with unresolved\nquestions and evidence"]
    Policy["Independent RFC-0010 policy evaluation"]
    Learning["Verified outcome, near miss,\nor human correction"]
    Renewal["Review and version knowledge;\nrenew or decay trust"]

    Signal --> Case --> Evidence --> Gap
    Gap -->|"No"| Tools --> ToolPolicy
    ToolPolicy --> Results --> Evidence
    Gap -->|"Search prior knowledge"| Knowledge --> Trust --> Hypotheses
    Evidence --> Hypotheses
    Hypotheses --> Outcome
    Outcome -->|"Actionable recommendation"| Recommend --> Policy
    Outcome -->|"Unknown, contradictory, or high risk"| Escalate
    Policy -->|"policy decision required"| Escalate
    Results --> Learning
    Policy --> Learning
    Learning --> Renewal --> Knowledge
```

**Interpretation:** Tool execution is only a proposed future capability and must be core-mediated. A knowledge item can guide the search but does not prove the current cause or authorize an action. “Unable to determine” and escalation are valid outcomes.

**Use this view when:** investigating a DNS propagation delay, database divergence, queue backlog, or host-pressure event and preserving alternatives rather than overclaiming root cause.
