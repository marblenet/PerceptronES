# Changelog

## 2026-10-21

### Mission assessment and dashboard

- Refined the mission dashboard to present a concise status headline and bullet-point contributing factors rather than a large explanation paragraph.
- Kept the mission objective visible as secondary context, and surfaced non-healthy component/security assessments and missing required evidence as factors.
- Labeled event identifiers as `Event_ID: <event_id>` in dashboard event views and evidence references.
- Updated dashboard copy and documentation to emphasize evidence, uncertainty, explicit unknown states, and the boundary between recommendations and authorization.

### Security posture

- Added the optional bundled `security-posture` evidence assessor for operator- or upstream-supplied security findings with source and observation-time provenance.
- Added validation for security evidence, stale/future-dated evidence handling, and mission status participation when the engine is enabled.
- Added `security_required_checks` so the configured security scope is explicit and each required check needs an exact matching finding. An enabled security engine without a declared scope remains incomplete rather than allowing a GREEN mission.
- Added security findings to dashboard, text/JSON/Prometheus reporting, and documented the assessor's limits: it is not a scanner and does not independently verify evidence sources or authorize remediation.
- Added dashboard configuration for required checks and supplied evidence.

### Isolated simulation

- Added deterministic built-in synthetic scenarios for baseline, CPU warning/critical pressure, missing/stale memory evidence, and a critical security finding.
- Added a separate Simulation dashboard page with editable synthetic measurements and warning/critical thresholds.
- Added a simulation-generated incident summary with contributing factors, missing evidence, and advisory recommendations, alongside all synthetic events and the complete cycle report.
- Kept simulation isolated from live reports and host state; simulation runs only built-in engines and is clearly labeled synthetic.

### Engineering guidance and verification

- Added and integrated the PerceptronSRE engineering philosophy covering deterministic bounded reasoning, no-default-green, unknown-state handling, security as operational evidence, simulation, and testing.
- Updated README, RFC, and implementation roadmap content to describe delivered security/simulation behavior and its limitations.
- Added regression coverage for mission uncertainty, exact security-check coverage, security config validation, dashboard controls, simulation input handling, and simulation isolation.
