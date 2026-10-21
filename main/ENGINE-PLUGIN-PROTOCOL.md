# Perceptron SRE Engine Plugin Protocol v1

The core uses an ordered engine pipeline. Built-in and external engines share the same declared event contract; An external engine is a separate executable and can be implemented in any language that reads and writes JSON. Bundled engines currently remain in-process; migrating them to this same package boundary is planned work.

## Configuration

`engines` is the ordered allow-list of engines to execute. It may be empty:

```json
{
  "engines": []
}
```

An empty list still collects configured observations and emits cycle/observation events, but creates no pressure, health, or mission assessments. The default configuration enables the initial host engine chain:

```json
{
  "engines": ["host-pressure", "host-health"]
}
```

Declare external programs under `plugins`, then enable their exact names in `engines`. The order is significant: inputs must be produced by observations or an earlier engine.

```json
{
  "engines": ["host-pressure", "host-health", "example-enrichment"],
  "plugins": [
    {
      "name": "example-enrichment",
      "version": "1.0.0",
      "command": "/opt/perceptron/plugins/example-enrichment",
      "args": [],
      "category": "host health",
      "role": "enrichment",
      "consumes": ["observation.host.v1"],
      "produces": ["example.enrichment.v1"],
      "timeout": "5s",
      "policy": {
        "mode": "reasoning_only",
        "read_only": true,
        "allowed_tools": [],
        "allowed_scripts": []
      },
      "settings": {"mode": "conservative"}
    }
  ]
}
```

The dashboard Store can enable or disable compatible bundled and already configured local plugins. Engine-specific JSON settings are edited in the dashboard; add or upgrade the executable, manifest, policy declaration, and `plugins` entry through an operator-controlled file change. Changes are applied after the next collection interval.

## Process exchange

The core starts the configured command once per engine invocation. It sends exactly one JSON object on stdin and expects exactly one JSON response object on stdout. Do not write logs or other text to stdout; write diagnostics to stderr.

Request shape:

```json
{
  "protocol_version": "perceptron-engine-v1",
  "name": "example-enrichment",
  "version": "1.0.0",
  "role": "enrichment",
  "cycle_id": "cycle-uuid",
  "configuration_id": "sha256-hex",
  "settings": {"mode": "conservative"},
  "inputs": [
    {
      "event_id": "event-uuid",
      "correlation_id": "cycle-uuid",
      "source": "host-observer",
      "type": "observation.host.v1",
      "schema_version": "v1",
      "occurred_at": "2026-10-20T12:00:00Z",
      "emitted_at": "2026-10-20T12:00:00Z",
      "entity_refs": [{"type": "host", "id": "host-a"}],
      "classification": {"reasoning_kind": "observed", "safety_class": "informational"},
      "payload": {"metric": "cpu.utilization_percent"}
    }
  ]
}
```

Response shape:

```json
{
  "protocol_version": "perceptron-engine-v1",
  "outputs": [
    {
      "event_id": "new-unique-event-uuid",
      "correlation_id": "cycle-uuid",
      "causation_id": "event-uuid",
      "derives_from": ["event-uuid"],
      "source": "example-enrichment",
      "producer_version": "1.0.0",
      "configuration_id": "sha256-hex",
      "type": "example.enrichment.v1",
      "schema_version": "v1",
      "occurred_at": "2026-10-20T12:00:00Z",
      "emitted_at": "2026-10-20T12:00:01Z",
      "entity_refs": [{"type": "host", "id": "host-a"}],
      "classification": {"reasoning_kind": "calculated", "safety_class": "reasoning"},
      "payload": {"summary": "example"}
    }
  ]
}
```

An engine error may be returned as `{"protocol_version":"perceptron-engine-v1","error":"description"}`. A failing process, timeout, malformed response, or invalid event fails the assessment cycle explicitly.

## Core validation and operational limits

- The configured timeout is greater than zero and at most five minutes. Each response is limited to 8 MiB; stderr is limited to 64 KiB.
- Each output must validate against the core event schema and exactly match the configured engine name, version, cycle ID, configuration fingerprint, and declared output type.
- Every output must declare direct input event IDs in `derives_from`; `causation_id`, when present, must also be a direct input. Output entity references must be within the input scope.
- The core executes only engines listed in `engines`; registration does not imply enablement.
- Each external declaration includes `category` and a policy object with `mode` (`reasoning_only`, `human_escalation`, or `self_mitigation`), `read_only`, `allowed_tools`, and `allowed_scripts`. The current runtime rejects enabled plugin declarations that are not read-only or request tools/scripts, and rejects self-mitigation. This is a configuration gate, not a sandbox or a guarantee about executable behavior.
- Plugins are arbitrary executable code. They run with the Perceptron process user's operating-system privileges and are **not sandboxed**. There is no automatic marketplace download, signature verification, privilege separation, or OS-enforced isolation in v0.1. Install only code the operator trusts, use a dedicated least-privilege service account, and keep plugin binaries/configuration administrator-controlled.
- The external package directory is `main/external/<engine-name>`. The current Store catalogs bundled engines and plugin declarations in the local config; it does not scan remote registries, verify publisher signatures, download, or install packages.
- `/api/findings` exports the latest full cycle as JSON and `/metrics` exports the current cycle as bounded Prometheus metrics. The latter is for scraping; remote write and arbitrary legacy metric ingestion are not implemented.

## Contributor checklist

1. Pick a globally unique engine name and version; document the role and event types.
2. Define stable, versioned input/output payload schemas and publish examples.
3. Preserve cycle/configuration provenance and direct derivation lineage.
4. Bound runtime and output size; send diagnostics to stderr only.
5. Test normal output, missing/unknown fields, malformed events, timeout, and failure behavior.
6. Document privileges, network/file access, configuration settings, and upgrade compatibility.
