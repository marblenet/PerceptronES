# D-05 — DNS Traffic Handler: Fast and Slow Paths

**Purpose:** Illustrate the customer-service fast path and the asynchronous analytical path for the DNS reference use case.

```mermaid
flowchart LR
    Client["DNS clients"]
    Handler["Traffic Handler\nbounded classification and regulation"]
    Known["Known-customer lane"]
    Unknown["Unattributed / unknown lane"]
    DNS["DNS service path\nload balancer, dnsdist, authoritative servers"]
    Metadata["Compact traffic metadata"]
    Events["Perceptron event adapters and bus"]
    Engines["Async pressure, health,\nimpact, and investigation"]
    Mission["Core mission assessment"]
    Twin["Optional data-plane twin"]
    Dropped["Twin response sink\nresponse discarded"]

    Client --> Handler
    Handler --> Known
    Handler --> Unknown
    Known --> DNS
    Unknown --> DNS
    Handler --> Metadata
    Metadata --> Events --> Engines --> Mission
    Handler -. "mirrored request if approved" .-> Twin
    Twin -. "response discarded; never returned to client" .-> Dropped
    Twin -->|"validation evidence"| Events

    FastRule["Fast path must not wait for\nknowledge search, mission reasoning,\nor simulation"]
    Handler -.-> FastRule
```

**Interpretation:** This functional diagram does not assign ownership or specify traffic policy. The two lanes are a candidate design; identity confidence, fairness, capacity reservations, and degraded behavior need explicit contracts. Analytics and twin work are decoupled from customer DNS responses. The twin's response is always suppressed.

**Use this view when:** evaluating NXDOMAIN storms, latency spikes, or overload while protecting customer DNS service from slow analytical processing.
