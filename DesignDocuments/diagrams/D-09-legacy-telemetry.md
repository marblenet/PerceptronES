# D-09 — Legacy Telemetry Interoperability

**Purpose:** Illustrate a vendor-neutral path for consuming existing metrics and returning richer findings to the same observability destination.

```mermaid
flowchart LR
    subgraph Existing["Existing monitoring estate"]
        Sources["Legacy metrics, logs,\nand application events"]
        Backend["Existing telemetry backend"]
        Dash["Grafana or existing dashboards"]
        Paging["Existing alerting and paging"]
    end

    subgraph Perceptron["Perceptron SRE"]
        In["Authenticated input adapter\nmapping, unit, scope, freshness"]
        Events["Canonical versioned events"]
        Engines["Selected domain engines"]
        Mission["Core mission and policy services"]
        Findings["Complete bulk findings API\nwith evidence provenance"]
        Metrics["Bounded metric projection\nstable labels, explicit unknowns"]
        Out["Configured output adapter"]
    end

    Sources --> Backend
    Backend -->|"configured scrape/query/stream"| In
    In --> Events --> Engines --> Mission
    Mission --> Findings
    Mission --> Metrics
    Findings -->|"API consumers"| Dash
    Metrics --> Out --> Backend
    Backend --> Dash
    Dash --> Paging

    AlertRule["Paging remains in existing alert rules\nunless a direct paging adapter is approved"]
    Paging -.-> AlertRule
```

**Interpretation:** A scrape endpoint is not a remote-write exporter. Detailed findings and arbitrary event payloads belong in the bulk API/event interface, not high-cardinality metric labels. Adapter failures, stale source data, privacy filtering, authentication, retries, and dropped data must be visible.

**Use this view when:** integrating Perceptron into a legacy monitoring system without replacing its metrics backend or paging routes.
