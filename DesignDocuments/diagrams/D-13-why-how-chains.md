# D-13 — Why Chain and How Chain (Proposed)

**Purpose:** Show how an incident report pairs backward causal reasoning (Why) with forward remediation reasoning (How) and feeds reviewed knowledge.

```mermaid
flowchart LR
    subgraph Why["Why chain (backward)"]
        W1[Observed impact] --> W2[Proximate cause] --> W3[Contributing factors] --> W4[Root cause] --> W5[Process / control gap]
    end
    subgraph How["How chain (forward)"]
        H1[Mitigation action] --> H2[State change] --> H3[Service recovery] --> H4[Impact normalized] --> H5[Why the fix worked]
    end
    Evidence[(Events, provenance, human_action records)] --> Why
    Evidence --> How
    H1 -. temporary or permanent .-> W5
    Why --> Report[Incident report: Why view + How view]
    How --> Report
    Report --> Review{Human review}
    Review --> Knowledge[Knowledge / runbook candidates]
    Review --> Sim[Simulation scenarios and regression tests]
```

**Interpretation:** Depth is evidence-driven (five is a minimum goal, not a quota). Chains may end in "unknown". Each link cites evidence and provenance; confidence appears only once a model exists (RFC-0004). Recovery is shown by post-mitigation observations (D-11), not assumed. Outputs are candidates until reviewed (RFC-0009).

**Use this view when:** designing incident reports, knowledge promotion, or dashboard Why/How views.
