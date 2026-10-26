# Changelog

## 2026-10-26

### Top 50 host metrics
- Added `internal/metrics`, a catalog of 50 host metrics (original 7 + 43 new): CPU detail, load 5m/15m, memory/swap/commit/OOM, PSI pressure, filesystem inodes/available/read-only, disk I/O, network errors/drops/rates/TCP/conntrack, and system (zombies, threads, file descriptors, PID usage, uptime, kernel taint). See roadmap Phase 1.3 for the full table.
- Linux collector reads `/proc` and `/sys` only (`observe/extended_linux.go`; platform-neutral parsers in `observe/parse_proc.go` with fixture tests). Rates come from the two samples around the CPU interval and are short-window.
- Unavailable sources produce unavailable observations with a reason, never zeros. Non-Linux builds report every catalog metric unavailable.
- Observation validation and pressure thresholds are catalog-driven; new optional `metric_thresholds` config overrides catalog defaults (validated).
- Extended metrics are advisory: they can raise the mission to YELLOW/RED, but an unavailable one does not force UNKNOWN. Core coverage requirements are unchanged.
- Verification: gofmt, vet (also `GOOS=linux`), all tests, and build pass. A Linux binary ran on WSL1, where PSI, `/proc/vmstat`, `/proc/diskstats`, conntrack and others are absent and reported unavailable (20 of 51 observations). Not yet tested on Oracle Linux 9, the target platform.
- Not done: dashboard presentation of the new metrics beyond the existing component views.

## 2026-10-26

### Five Whys / Five Hows incident reporting (documentation only; code not yet changed)

- Added a domain-neutral Why chain / How chain section to [ENGINEERING-PHILOSOPHY.md](./DesignDocuments/ENGINEERING-PHILOSOPHY.md).
- Added RFC-0008 section 7.1 (chain requirements, temporary vs. permanent mitigations, human actions as How evidence, recovery proven by post-mitigation observations, Why/How report views) and open decisions 12-14.
- Added RFC-0009 section 6.2 (chains as knowledge, runbook, and simulation inputs, subject to review).
- Confidence on chain links is marked as dependent on a future confidence model; none is emitted today.
- Added diagram [D-13](./DesignDocuments/diagrams/D-13-why-how-chains.md), and linked it from the design README, diagram index, and roadmap (Phase 2.4).

### Cascade, snapshot, and live Why/How in the dashboard

- Operations page: "Reasoning cascade (L1 → L6)" view using the memo layer names (L1 Metadata/Data, L2 Information, L3 Determination, L4 Assessment, L5 Incident Report, L6 Mitigation, ↺ L5 resolution/recurrence). Events map to L1–L4 by event type (table in ENGINE-PLATFORM-ADDENDUM.md); unknown types show as "Unmapped". L6 is recommendation-only and resolution/recurrence is shown Unknown.
- Operations page: Why/How tabs now also render for the live assessment (`live_incident` in `/api/report`), origin `live`, root cause/executed mitigation/recovery `unknown`, no confidence.
- Simulation page: "Fork from live snapshot" runs `/api/simulate` with `use_snapshot`, replaying the latest live host observations (original timestamps preserved, so stale stays stale) through the isolated runtime as `simulation` origin. Security evidence in a snapshot fork is a synthetic placeholder. The live report is not modified. Simulation also shows its own cascade.
- Tests cover snapshot fork, input immutability, live chain labels, and the new dashboard endpoints.

### Why/How chains in code

- Simulation incident reports now include `why_chain`, `how_chain`, and `chain_note` (`ChainLink`: step, statement, status observed/proposed/unknown, mitigation class, evidence, origin=simulation). Root cause, executed mitigation, and recovery are explicitly `unknown`; candidate mitigations are `unclassified`; no confidence is emitted.
- Simulation page shows Why / How tabs on the synthetic incident report.
- Tests cover chain presence, simulation origin labeling, and unknown root cause.

## 2026-10-26

### Engineering memo alignment (documentation first)

- Updated [ENGINEERING-PHILOSOPHY.md](./DesignDocuments/ENGINEERING-PHILOSOPHY.md) with an explicit documentation-first workflow (`Document -> Review -> Approve -> Implement -> Test -> Validate`), clarified perceptron semantics (multi-input weighted reasoning), and documented the forward-only deterministic cascade (no runtime backpropagation/self-learning).
- Expanded [ENGINE-PLATFORM-ADDENDUM.md](./DesignDocuments/ENGINE-PLATFORM-ADDENDUM.md) with explicit cascade engine classes (tap, layer-1, layer-2, assessment, recommendation, policy), plugin-ecosystem direction, and contributor expectations.
- Updated [DesignDocuments/README.md](./DesignDocuments/README.md) and [IMPLEMENTATION-ROADMAP.md](./Implementation/IMPLEMENTATION-ROADMAP.md) to reinforce the documentation-before-implementation gate.

### Dashboard behavior updates from memo direction

- Kept runtime configuration in a dedicated Settings page opened from a top-right gear, while preserving Simulation and Engine Store pages.
- Added a Security page in dashboard navigation for focused security review, separate from live alert handling.
- Promoted event timeline/provenance to the top of Operations and rendered events as newest-left expandable cards with payload/provenance details.
- Added explicit top-row incident and confidence context cards (`Active incidents`, `Confidence`) without implying unavailable confidence modeling.

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
