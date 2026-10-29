# Changelog

## 2026-10-28
### Store and dashboard pages
- "Engine Store" renamed "Store"; it filters by cascade layer (Layer-0..8, Unmapped) and text search, and every engine type is tagged with its layer.
- New Domains page (Domain Exploration memo) with a Host / DNS / Security / Application selector and per-domain Layer-0..8 drill-down. Host is full; Security shows evidence and findings only; DNS and Application have no engines and say nothing is assessed.

### Documentation (memos folded into design docs)
- Rule: memos update `01-overall-architecture.md` and the RFCs; the design docs are the source of truth for code.
- `01-overall-architecture.md`: system definition (section 2), Core + Extension (4.25), reliability through simplicity and the Perceptron Neural Network Cascade (4.26), perceptron meaning (4.27), Why/How chains (4.28), applying guidance plus quality gates, break glass override (Proposed) and the two-page dashboard (4.29), tap engines and contributor contract (6.3), canonical Layer-# table (6.4), Knowledge Store (6.6), event origin, PES and simulation (6.8), repository architecture and domain configuration (8.4). `ENGINEERING-PHILOSOPHY.md` and `ENGINE-PLATFORM-ADDENDUM.md` were fully merged into these sections and removed; older entries below that cite them refer to those former files.
- RFC updates: RFC-0001 maps event types to Layer-0..8; RFC-0002 adds canonical layers (6.1), Core + Extension and resource budgets (6.2); RFC-0009 adds Knowledge Store sources, weights, pattern semantics and domain roadmap (6.3); RFC-0010 adds Layer-5..7 semantics (8.1); RFC-0012 adds Core/Local/Community tiers for all package types, the community-vote caveat and domain configuration; notes in RFC-0004/0007/0008; RFC-INDEX (13 diagrams), roadmap 1.4 cascade and D-12 tiers updated. Unbuilt items are marked Open/Proposed.
- Example IPs in `main/README.md`, tests and this file now use documentation addresses (`10.0.0.5`, `203.0.113.10`); `.gitignore` also excludes `DesignDocuments/Memos`, the root Linux binary and `*.key`.

### Config: domain-oriented configuration
- Optional `config.d/<domain>.json` next to the core config overlays domain settings (`host`, `network`, `security`, `dns`, `application`); each domain may set only its own keys, platform keys (dashboard, policy, engines, plugins, knowledge) stay in the core file, and unknown domains or keys are rejected. Not built: Core/Local/Community directories per layer, Store packages, resource-budget enforcement.

### Layer naming and file architecture
- Every layer is now `Layer-<number> <Label>` (Layer-0 Input … Layer-8 Learning) in the dashboard, code comments and documents. Input is numbered Layer-0 because the memo leaves it unnumbered while fixing Layer-1 Information and Layer-2 Determinations.
- `main/internal/engine` files are renamed by layer (`layer1_information.go`, `layer2_determination.go`, `layer2_security.go`, `layer3_assessment_*.go`, `layer4_recommendation.go` … `layer8_learning.go`, `layers_common.go`). No behavior change. Layout is described in `01-overall-architecture.md` sections 6.4 and 8.4.

### Cascade layers implemented (Recommendation, Policy, Action, Outcome, Learning)
- New engines: `host-recommendation`, `host-policy`, `host-action`, `host-outcome`, `host-learning`, producing `recommendation.host.v1`, `policy.decision.v1`, `action.record.v1`, `outcome.host.v1`, `learning.candidate.v1`. All are in the default engine list; explicit `engines` lists must add them.
- Recommendations are advisory (CRITICAL escalates to a human, others investigate). Policy config `policy.default_decision` and `policy.rules` (pattern ID or `domain:<name>`): authorized, require_approval (default), simulation_only, denied.
- Action records state what happened to a decision; no executor exists, so `executed` is always false.
- Outcome compares determinations across live cycles in memory (lost on restart). `resolved` requires fresh evidence for every metric the pattern uses, otherwise `unknown`. Outcomes are never attributed to an action.
- Learning emits candidates only (regression-test proposal when resolved, knowledge review when worse); nothing is applied.
- Knowledge: `community_patterns`, `weights` (default core 30 / local 60 / community 10; highest weight wins on ID collisions, zero ignores a source), optional pattern `recommendation` text; determinations report source and weight.
- Dashboard cascade shows all layers with per-event detail; engine catalog lists the new engines. `perceptron.json` updated.
- Still Open: determinations feeding health/mission, Security and Services determinations, action executor and approval workflow, durable outcome history, PES (needs a threat model before any implementation).

### Knowledge base and determinations
- New `internal/knowledge` package: pattern model, validation, Core/Local resolution, 19 core patterns (CPU, memory, storage, network, system limits).
- New `host-determination` engine (consumes pressure, produces `determination.host.v1`), added to default engines after `host-pressure`. Existing configs that list engines must add it.
- Config `knowledge.local_patterns` / `knowledge.disabled_patterns`; local pattern with a core ID overrides it.
- Dashboard: determination shown in the Layer-2 cascade column and engine catalog.
- Unavailable or stale evidence never matches. Determinations do not yet feed health/mission (Open). Community packages and Services/Security patterns are not implemented (source weights were added later; see the cascade entry above).
- Docs: canonical cascade and Knowledge Store model (now in `01-overall-architecture.md` 6.4 and 6.6). Dashboard uses the memo layer names (Layer-1 Information, Layer-2 Determinations, and so on); the earlier L1–L6 labels are retired.

### Configurable dashboard listen address
- New `dashboard` config section: `listen_address` (literal IP, default `127.0.0.1`), `port` (default 8080), `allowed_hosts`, `allow_remote_config`. New `--web-addr` flag; `--web-addr`/`--web-port` override the file.
- Motivated by an OCI host where the loopback-only bind was unreachable; set `listen_address` to the VNIC address (e.g. `10.0.0.5`) or `0.0.0.0`.
- Safeguards for non-loopback binds: Host header allow-list (own/interface addresses plus `allowed_hosts`), web config editing disabled unless `allow_remote_config`, startup warning logged. Listener settings are startup-only and preserved when the web UI saves config.
- Still no authentication or TLS; use firewalld and the OCI security list (open the TCP port) or an SSH tunnel. Verified on a live OCI host (Oracle Linux 9): dashboard reachable on the VNIC address.

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
