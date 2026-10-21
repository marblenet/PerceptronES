# External engine packages

Place locally authored or operator-installed engine packages under:

```text
main/external/<engine-name>/
```

Each package should include a versioned manifest, executable for supported platforms, input/output event schemas, engine-owned settings schema, conformance tests, provenance, resource limits, and an action-policy declaration. See [ENGINE-PLUGIN-PROTOCOL.md](../ENGINE-PLUGIN-PROTOCOL.md) for the current JSON process protocol.

The runtime currently reads external executable declarations from `perceptron.json`; the Store does not yet discover package directories or install packages. First-party host engines are still compiled into the core and are not yet migrated into this directory. Keep installed package files and configuration administrator-controlled.

Policy declarations do not sandbox code. External processes run with the Perceptron service account's OS privileges. Do not install untrusted executables; tool/script access and self-mitigation remain unavailable until a separately enforced broker and sandbox are delivered.
