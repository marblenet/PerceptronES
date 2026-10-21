# D-03 — Entity and Impact Graph

**Purpose:** Show how source observations can be scoped to infrastructure and related to services and affected customers without claiming a single authoritative database.

```mermaid
flowchart LR
    Region["Region"]
    Host["Host or deployment"]
    Component["Component"]
    Service["Service"]
    Zone["DNS zone"]
    Record["DNS record"]
    Customer["Customer"]
    Mission["Mission objective"]

    Region -->|"contains / serves"| Host
    Host -->|"runs"| Component
    Component -->|"supports"| Service
    Service -->|"serves"| Zone
    Zone -->|"contains"| Record
    Customer -->|"owns or depends on"| Service
    Customer -->|"owns"| Zone
    Service -->|"contributes to"| Mission
    Customer -->|"impact scope"| Mission

    Observation["Observation with source, time, and entity references"]
    Assessment["Derived assessment with evidence lineage"]
    Observation --> Component
    Assessment --> Service
    Assessment --> Mission

    Provenance["Relationship provenance:\nsource, valid time, observed time,\nauthority or inference, confidence"]
    Region -.-> Provenance
    Service -.-> Provenance
    Customer -.-> Provenance
```

**Interpretation:** This is a candidate relationship graph, not a universal schema. Edges may be authoritative, observed, or inferred and must preserve their origin and time validity. Customer/service links enable impact analysis; they do not imply that every installation stores customer identity inside Perceptron.

**Use this view when:** deciding what entity a metric refers to and how a component condition could affect a customer-level mission.
