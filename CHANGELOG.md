# Changelog

## 2026-10-23

### Dashboard appearance

- Corrected the dashboard palette in the PerceptronES application source: charcoal/black surfaces with green terminal-style accents replace the blue theme.
- Added a dashboard regression assertion for the embedded palette so future builds serve the intended colors.
- Reworked Operations with an event timeline at the top (newest on the left), expandable event payloads and provenance, and a top-right configuration gear.
- Added expandable reasoning-engine availability and a compact security status to the mission summary; moved detailed security findings to a dedicated Security page, separate from live alerting.
- Renamed the mission and recommendation panels “The Alert Window” and “Suggested Mitigation Window”; retained the separate Simulation page and labeled event identifiers as `Event_ID: <event_id>`.

### Engineering memo and platform contracts

- Applied the Perceptron SRE engineering philosophy and today's engine-platform addendum to the PerceptronES design knowledge base; documented tap-engine boundaries, contributor proof contracts, extensible domains, simulation/dry-run distinctions, human-action safeguards, and staged adoption.
- Added versioned event origins (`live`, `simulation`, `replay`, `human_action`) and propagate origins through engine and mission outputs; isolated simulation results are now explicitly marked `simulation`.
- Enforced observation-only output classification for tap-category plugins, upgraded the external engine protocol and event envelope to v2, and documented the compatibility contract.
- Added tests for required event origin, origin propagation, simulation labeling, and tap-engine output boundaries.

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
