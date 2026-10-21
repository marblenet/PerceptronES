# D-01 — System Context and Trust Boundaries

**Purpose:** Show Perceptron SRE's core responsibilities and its integration boundaries. This is a logical context view, not a deployment topology.

```mermaid
flowchart LR
    subgraph Sources["Operational systems and legacy telemetry"]
        Host["Hosts and services"]
        DNS["DNS control and data planes"]
        Legacy["Existing metrics and event systems"]
        Human["Operators and engineers"]
    end

    subgraph Core["Perceptron SRE core"]
        Input["Versioned input adapters"]
        Bus["Event bus and provenance"]
        Runtime["Engine runtime and catalog"]
        Mission["Core mission aggregation"]
        Policy["Core policy authority"]
        API["Dashboard and core APIs"]
        Export["Bulk findings and metrics exporter"]
    end

    subgraph Engines["Domain engine packages"]
        Observe["Observation adapters"]
        Reason["Pressure, health, correlation, impact"]
        Investigate["Investigation and knowledge engines"]
        Validate["Validation engines"]
    end

    subgraph Outside["External validation and execution boundaries"]
        Twin["Isolated data/control-plane twins"]
        Action["Owning system action adapter"]
    end

    subgraph Destinations["Existing operational destinations"]
        Monitor["Prometheus-compatible scraper / Grafana"]
        Pager["Existing alert and paging system"]
        LegacyTarget["Legacy telemetry destination"]
    end

    Host --> Input
    DNS --> Input
    Legacy --> Input
    Input --> Bus
    Bus <--> Runtime
    Runtime <--> Observe
    Runtime <--> Reason
    Runtime <--> Investigate
    Runtime <--> Validate
    Runtime --> Mission
    Mission --> Bus
    Bus --> API
    Human <--> API
    API --> Runtime
    Bus --> Export
    Export --> Monitor
    Monitor --> Pager
    Export --> LegacyTarget
    Validate <--> Twin
    Policy --> Action
    Action --> DNS
    Bus -. "proposed action and evidence" .-> Policy
    Policy -. "decision, approval, audit" .-> Bus
```

**Interpretation:** Domain engines produce claims and evidence; core services own event transport, mission aggregation, policy authority, and operator APIs. Input/output adapters connect existing systems without making them Perceptron-owned. Paging may remain in the existing monitoring path. A dashed policy path represents a decision boundary, not an always-enabled action capability. External engine processes are not automatically sandboxed.

**Use this view when:** deciding whether a capability belongs in the core, an engine, an integration adapter, or an external system.
