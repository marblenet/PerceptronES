# D-08 — Policy, Authorization, and Action Lifecycle

**Purpose:** Make the safety gates explicit between an engine recommendation and a side effect.

```mermaid
flowchart TD
    Finding["Engine finding and provenance"]
    Recommendation["Proposed action:\nexact tool, arguments, target, scope,\nexpected effects, reversibility"]
    Validate{"Validation required\nand available?"}
    Twin["Twin or simulation evidence"]
    Inputs["Policy inputs:\nidentity, evidence, mission,\ncoverage, freshness, current state"]
    Evaluate["Core policy authority\nversioned rule evaluation"]
    Outcome{"Decision"}
    Deny["Deny or unable to evaluate"]
    Hold["Hold for approval / more evidence"]
    Escalate["Escalate to human"]
    Authorization["Short-lived authorization bound\nto exact action and scope"]
    Broker["Core-enforced capability broker"]
    Owner["Owning system integration"]
    Verify["Verify outcome and impact"]
    Audit["Audit recommendation, decision,\napproval, execution, and result"]

    Finding --> Recommendation --> Validate
    Validate -->|"Yes"| Twin --> Inputs
    Validate -->|"No"| Inputs
    Inputs --> Evaluate --> Outcome
    Outcome -->|"deny"| Deny --> Audit
    Outcome -->|"insufficient information"| Deny
    Outcome -->|"human gate"| Hold --> Escalate --> Audit
    Outcome -->|"permit with constraints"| Authorization --> Broker --> Owner --> Verify --> Audit
    Recommendation -. "never equals authorization" .-> Authorization
```

**Interpretation:** Recommendations, validation, approval, authorization, execution, and outcome are distinct records. An authorization is specific, expires, and becomes invalid when relevant context changes. Confidence cannot bypass policy. If the broker or policy service is unavailable, no execution is permitted.

**Use this view when:** reviewing any proposed remediation or debating whether a default engine can be allowed to self-mitigate.
