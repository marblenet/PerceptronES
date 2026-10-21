# D-10 — Regional Federation and Degraded Operation

**Purpose:** Show candidate local reasoning and selective federation while preserving gaps during a partition. This is not an approved topology.

```mermaid
flowchart TB
    subgraph R1["Region A"]
        E1["Local source adapters"]
        B1["Regional event stream"]
        G1["Regional engines and mission"]
        E1 --> B1 --> G1
    end
    subgraph R2["Region B"]
        E2["Local source adapters"]
        B2["Regional event stream"]
        G2["Regional engines and mission"]
        E2 --> B2 --> G2
    end
    subgraph R3["Region C"]
        E3["Local source adapters"]
        B3["Regional event stream"]
        G3["Regional engines and mission"]
        E3 --> B3 --> G3
    end

    Federation["Federation boundary\nselected events, locality,\nprovenance, freshness"]
    Global["Global mission aggregation\ncoverage-aware; preserves local severity"]
    Partition{"Federation available\nand data fresh?"}
    Unknown["Global assessment is partial,\nstale, or UNKNOWN as appropriate"]
    Local["Continue local observation\nand permitted reasoning"]
    Request["Cross-region action proposal"]
    Policy["Core policy and authority check\nexact scope, owner, lease/fencing,\nidempotency, freshness"]
    Authorized["Authorized request to owning system"]
    Blocked["Hold or reject while authority,\ncoverage, or federation is insufficient"]

    G1 --> Federation
    G2 --> Federation
    G3 --> Federation
    Federation --> Partition
    Partition -->|"Yes"| Global
    Partition -->|"No"| Unknown
    G1 -.-> Local
    G2 -.-> Local
    G3 -.-> Local
    Global --> Request --> Policy
    Policy -->|"all required gates pass"| Authorized
    Policy -->|"otherwise"| Blocked
    Unknown -. "cannot authorize from missing global evidence" .-> Blocked
```

**Interpretation:** Regions may continue local observation when disconnected, but local visibility does not imply global completeness or cross-region action authority. Missing regional inputs must reduce coverage or produce an unknown/partial result. Queue replication alone does not settle work ownership or prevent duplicate execution.

**Use this view when:** reasoning about a region outage, network partition, stale federation link, or recovery/rejoin.
