# D-12 — Engine Package, Store, and Trust Lifecycle

**Purpose:** Show how first-party and community engines enter an operator-controlled catalog, without confusing discovery, trust, enablement, and capability grants.

```mermaid
flowchart TD
    Author["Local author or third-party publisher"]
    Package["Versioned package:\nmanifest, contracts, settings,\nprovenance, policy declaration"]
    Review["Review source, version, dependencies,\nprivileges, compatibility, and tests"]
    Install["Operator installs under\nmain/external/<engine-name>"]
    Catalog["Local Engine Store catalog\n(Core / Local / Community sources)"]
    Configure["Engine-owned settings\nvalidated by schema"]
    Select{"Operator enables\ncompatible engine?"}
    Runtime["Runtime validates contracts,\nresource bounds, and order"]
    Process["Engine runs at declared boundary"]
    Finding["Validated findings and event lineage"]
    Disable["Disable / rollback package"]
    Capability["Requested tools/scripts/actions"]
    Broker{"Core-enforced broker\nand policy grant exist?"}
    Denied["Unavailable by default;\nmetadata is not enforcement"]

    Author --> Package --> Review --> Install --> Catalog
    Catalog --> Configure --> Select
    Select -->|"Yes"| Runtime --> Process --> Finding
    Select -->|"No"| Catalog
    Catalog --> Disable
    Package --> Capability --> Broker
    Broker -->|"No"| Denied
    Broker -->|"Yes, after separate policy and security approval"| Runtime
```

**Interpretation:** Bundled defaults, local packages, and third-party packages should share a common catalog contract. Remote marketplace discovery, signature verification, automatic installation, and OS sandboxing are separate capabilities and are not implied by a Store UI. Process isolation is not a security sandbox.

**Use this view when:** contributing an engine, evaluating a third-party package, or deciding whether a capability request can be enabled.
