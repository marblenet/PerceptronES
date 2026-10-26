package simulation

import (
	"context"
	"fmt"
	"math"
	"time"

	"perceptronsre/internal/config"
	"perceptronsre/internal/engine"
	"perceptronsre/internal/model"
	"perceptronsre/internal/report"
)

type Result struct {
	Scenario       string             `json:"scenario"`
	Label          string             `json:"label"`
	Synthetic      bool               `json:"synthetic"`
	SnapshotOf     string             `json:"snapshot_of,omitempty"`
	Inputs         Inputs             `json:"inputs"`
	IncidentReport IncidentReport     `json:"incident_report"`
	Report         report.CycleReport `json:"report"`
}

type Inputs struct {
	CPUPercent        *float64           `json:"cpu_percent,omitempty"`
	MemoryPercent     *float64           `json:"memory_percent,omitempty"`
	LoadPerCPU        *float64           `json:"load_per_cpu,omitempty"`
	FilesystemPercent *float64           `json:"filesystem_percent,omitempty"`
	Thresholds        *config.Thresholds `json:"thresholds,omitempty"`
}

type IncidentReport struct {
	ID                  string      `json:"id"`
	Title               string      `json:"title"`
	Status              string      `json:"status"`
	Summary             string      `json:"summary"`
	ContributingFactors []string    `json:"contributing_factors"`
	MissingEvidence     []string    `json:"missing_evidence"`
	Recommendations     []string    `json:"recommendations"`
	WhyChain            []ChainLink `json:"why_chain"`
	HowChain            []ChainLink `json:"how_chain"`
	ChainNote           string      `json:"chain_note"`
}

// ChainLink is one step of a Why (backward) or How (forward) chain.
// Status is "observed", "proposed", or "unknown"; Class is only meaningful for How links.
type ChainLink struct {
	Step      int      `json:"step"`
	Statement string   `json:"statement"`
	Status    string   `json:"status"`
	Class     string   `json:"class,omitempty"`
	Evidence  []string `json:"evidence"`
	Origin    string   `json:"origin"`
}

type scenario struct {
	label string
	build func(config.Config, time.Time) ([]model.Observation, []config.SecurityEvidence, error)
}

var scenarios = map[string]scenario{
	"normal": {
		label: "Normal baseline",
		build: func(cfg config.Config, now time.Time) ([]model.Observation, []config.SecurityEvidence, error) {
			return baseline(cfg, now), baselineSecurity(now), nil
		},
	},
	"cpu-warning": {
		label: "Elevated CPU pressure",
		build: func(cfg config.Config, now time.Time) ([]model.Observation, []config.SecurityEvidence, error) {
			observations := baseline(cfg, now)
			value := cfg.Thresholds.CPUPercent.Warning
			observations[0].Value = &value
			return observations, baselineSecurity(now), nil
		},
	},
	"cpu-critical": {
		label: "Critical CPU pressure",
		build: func(cfg config.Config, now time.Time) ([]model.Observation, []config.SecurityEvidence, error) {
			observations := baseline(cfg, now)
			value := cfg.Thresholds.CPUPercent.Critical
			observations[0].Value = &value
			return observations, baselineSecurity(now), nil
		},
	},
	"missing-memory": {
		label: "Missing memory evidence",
		build: func(cfg config.Config, now time.Time) ([]model.Observation, []config.SecurityEvidence, error) {
			observations := baseline(cfg, now)
			return append(observations[:2], observations[3:]...), baselineSecurity(now), nil
		},
	},
	"stale-memory": {
		label: "Stale memory evidence",
		build: func(cfg config.Config, now time.Time) ([]model.Observation, []config.SecurityEvidence, error) {
			observations := baseline(cfg, now)
			maxAge, err := cfg.MaxAgeDuration()
			if err != nil {
				return nil, nil, fmt.Errorf("parse maximum observation age for simulation: %w", err)
			}
			observations[2].ObservedAt = now.Add(-maxAge - time.Second)
			return observations, baselineSecurity(now), nil
		},
	},
	"security-critical": {
		label: "Security finding: critical",
		build: func(cfg config.Config, now time.Time) ([]model.Observation, []config.SecurityEvidence, error) {
			evidence := baselineSecurity(now)
			evidence[0].State = "CRITICAL"
			evidence[0].Summary = "Synthetic critical posture finding for dashboard demonstration."
			return baseline(cfg, now), evidence, nil
		},
	},
}

func ScenarioNames() []string {
	return []string{"normal", "cpu-warning", "cpu-critical", "missing-memory", "stale-memory", "security-critical"}
}

func RunScenario(ctx context.Context, cfg config.Config, name string) (Result, error) {
	return RunScenarioWithInputs(ctx, cfg, name, Inputs{})
}

func RunScenarioWithInputs(ctx context.Context, cfg config.Config, name string, inputs Inputs) (Result, error) {
	selected, ok := scenarios[name]
	if !ok {
		return Result{}, fmt.Errorf("unknown simulation scenario %q", name)
	}
	cfg, err := applyInputs(cfg, inputs)
	if err != nil {
		return Result{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Result{}, fmt.Errorf("invalid simulation configuration: %w", err)
	}
	observations, evidence, err := selected.build(cfg, time.Now().UTC())
	if err != nil {
		return Result{}, err
	}
	return execute(ctx, cfg, name, selected.label, inputs, observations, evidence, "")
}

// RunSnapshot forks a simulation from observations captured from a live cycle.
// Captured observation times are preserved, so stale evidence stays stale. The
// live report and runtime are never modified; security evidence is a synthetic
// placeholder because the snapshot carries host observations only.
func RunSnapshot(ctx context.Context, cfg config.Config, snapshotCycleID string, snapshot []model.Observation, inputs Inputs) (Result, error) {
	if len(snapshot) == 0 {
		return Result{}, fmt.Errorf("snapshot contains no observations")
	}
	cfg, err := applyInputs(cfg, inputs)
	if err != nil {
		return Result{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Result{}, fmt.Errorf("invalid simulation configuration: %w", err)
	}
	observations := make([]model.Observation, len(snapshot))
	copy(observations, snapshot)
	for index := range observations {
		if observations[index].Value != nil {
			value := *observations[index].Value
			observations[index].Value = &value
		}
	}
	return execute(ctx, cfg, "snapshot", "Snapshot of live cycle "+snapshotCycleID, inputs, observations, baselineSecurity(time.Now().UTC()), snapshotCycleID)
}

func execute(ctx context.Context, cfg config.Config, name, label string, inputs Inputs, observations []model.Observation, evidence []config.SecurityEvidence, snapshotOf string) (Result, error) {
	applyMeasurementInputs(observations, inputs)
	simulationConfig := cfg
	simulationConfig.Engines = []string{"host-pressure", "host-health", "security-posture"}
	simulationConfig.SecurityRequiredChecks = []string{"sample-security-control"}
	simulationConfig.SecurityEvidence = evidence
	runtime, err := engine.NewRuntime(simulationConfig)
	if err != nil {
		return Result{}, fmt.Errorf("create isolated simulation runtime: %w", err)
	}
	id, err := model.NewID()
	if err != nil {
		return Result{}, fmt.Errorf("create simulation cycle ID: %w", err)
	}
	cycleID := "simulation-" + id
	cycle, err := runtime.RunWithOrigin(ctx, cycleID, observations, model.OriginSimulation)
	if err != nil {
		return Result{}, fmt.Errorf("run synthetic scenario %q: %w", name, err)
	}
	incident := buildIncidentReport(cycleID, cycle.Mission)
	available := len(observations)
	return Result{
		Scenario:       name,
		Label:          label,
		Synthetic:      true,
		SnapshotOf:     snapshotOf,
		Inputs:         inputs,
		IncidentReport: incident,
		Report: report.CycleReport{
			HostID:                  cfg.HostID,
			AvailableObservations:   available,
			UnavailableObservations: 0,
			Runtime:                 cycle,
		},
	}, nil
}

func applyInputs(cfg config.Config, inputs Inputs) (config.Config, error) {
	if inputs.Thresholds != nil {
		cfg.Thresholds = *inputs.Thresholds
	}
	for name, value := range map[string]*float64{
		"cpu_percent": inputs.CPUPercent, "memory_percent": inputs.MemoryPercent,
		"filesystem_percent": inputs.FilesystemPercent,
	} {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 || *value > 100) {
			return config.Config{}, fmt.Errorf("simulation %s must be between 0 and 100", name)
		}
	}
	if inputs.LoadPerCPU != nil && (math.IsNaN(*inputs.LoadPerCPU) || math.IsInf(*inputs.LoadPerCPU, 0) || *inputs.LoadPerCPU < 0) {
		return config.Config{}, fmt.Errorf("simulation load_per_cpu must be finite and not negative")
	}
	return cfg, nil
}

func applyMeasurementInputs(observations []model.Observation, inputs Inputs) {
	values := map[string]*float64{
		"cpu": inputs.CPUPercent, "memory": inputs.MemoryPercent,
		"load": inputs.LoadPerCPU, "filesystem": inputs.FilesystemPercent,
	}
	for index := range observations {
		if value := values[observations[index].Component]; value != nil && observations[index].Available {
			observations[index].Value = value
		}
	}
}

func buildIncidentReport(id string, mission *model.MissionAssessment) IncidentReport {
	return buildReport(id, mission, model.OriginSimulation)
}

// BuildLiveIncident derives a Why/How incident view from the live assessment.
// It claims no root cause, executed mitigation, recovery, or confidence.
func BuildLiveIncident(id string, mission *model.MissionAssessment) IncidentReport {
	return buildReport(id, mission, model.OriginLive)
}

func buildReport(id string, mission *model.MissionAssessment, origin string) IncidentReport {
	kind, Kind := "synthetic", "Synthetic"
	if origin == model.OriginLive {
		kind, Kind = "current", "Current"
	}
	result := IncidentReport{
		ID: id, Title: Kind + " mission assessment", Status: string(model.MissionUnknown),
		Summary:             "No mission assessment was produced for this " + kind + " cycle.",
		ContributingFactors: []string{}, MissingEvidence: []string{}, Recommendations: []string{},
	}
	if mission == nil {
		buildChains(&result, nil, origin)
		return result
	}
	result.Status = string(mission.Status)
	result.Summary = mission.Explanation
	result.MissingEvidence = append(result.MissingEvidence, mission.Coverage.MissingComponents...)
	for _, component := range mission.Components {
		if component.State != model.HealthHealthy {
			result.ContributingFactors = append(result.ContributingFactors, component.Component+": "+string(component.State)+" — "+component.Explanation)
		}
	}
	for _, finding := range mission.SecurityFindings {
		if finding.State != "HEALTHY" {
			result.ContributingFactors = append(result.ContributingFactors, finding.Check+": "+finding.State+" — "+finding.Explanation)
		}
	}
	switch mission.Status {
	case model.MissionRed:
		result.Title = "Critical " + kind + " findings"
		result.Recommendations = append(result.Recommendations, "Review the critical evidence and follow the relevant operational runbook; this report does not authorize remediation.")
	case model.MissionYellow:
		result.Title = "Elevated " + kind + " findings"
		result.Recommendations = append(result.Recommendations, "Review elevated evidence and compare with current operational context before deciding on a response.")
	case model.MissionGreen:
		result.Title = Kind + " assessment within configured criteria"
		result.Recommendations = append(result.Recommendations, "No risk is reported by this assessment; absence of findings is limited to the evidence and coverage shown.")
	default:
		result.Title = Kind + " assessment is incomplete"
		result.Recommendations = append(result.Recommendations, "Resolve missing or uncertain evidence before treating the mission as healthy.")
	}
	if len(result.ContributingFactors) == 0 && len(result.MissingEvidence) > 0 {
		result.ContributingFactors = append(result.ContributingFactors, "Required evidence is missing or unassessed.")
	}
	if len(result.ContributingFactors) == 0 {
		result.ContributingFactors = append(result.ContributingFactors, "No adverse contributing factors in this "+kind+" assessment.")
	}
	buildChains(&result, mission, origin)
	return result
}

// buildChains derives Why/How chains from the assessment only. No root cause,
// executed mitigation, recovery, or confidence is claimed: those are Unknown.
func buildChains(report *IncidentReport, mission *model.MissionAssessment, origin string) {
	add := func(chain *[]ChainLink, statement, status, class string, evidence ...string) {
		if evidence == nil {
			evidence = []string{}
		}
		*chain = append(*chain, ChainLink{Step: len(*chain) + 1, Statement: statement, Status: status, Class: class, Evidence: evidence, Origin: origin})
	}
	report.WhyChain, report.HowChain = []ChainLink{}, []ChainLink{}
	scope := "synthetic assessment only. Not live history"
	if origin == model.OriginLive {
		scope = "the current live assessment only. No incident history, executed mitigation, or operator action is recorded here"
	}
	report.ChainNote = "Proposed chains derived from " + scope + ". Root cause, executed mitigation, recovery, and link confidence are not determined; recommendations are not authorization."
	add(&report.WhyChain, "Observed impact: mission status "+report.Status+". "+report.Summary, "observed", "", "mission-assessment")
	if mission != nil {
		for _, component := range mission.Components {
			if component.State != model.HealthHealthy {
				add(&report.WhyChain, "Contributing factor: "+component.Component+" is "+string(component.State), "observed", "", "component:"+component.Component)
			}
		}
		for _, finding := range mission.SecurityFindings {
			if finding.State != "HEALTHY" {
				add(&report.WhyChain, "Contributing factor: security check "+finding.Check+" is "+finding.State, "observed", "", "security:"+finding.Check)
			}
		}
	}
	add(&report.WhyChain, "Root cause and process/control gap: unknown; rule-based findings are not proof of root cause.", "unknown", "")
	for _, rec := range report.Recommendations {
		add(&report.HowChain, "Candidate mitigation: "+rec, "proposed", "unclassified")
	}
	add(&report.HowChain, "Executed mitigation and state change: none recorded.", "unknown", "")
	add(&report.HowChain, "Recovery and why a fix would work: unknown; requires post-mitigation observations.", "unknown", "")
}

func baseline(cfg config.Config, now time.Time) []model.Observation {
	cpu := 20.0
	load := 0.5
	memory := 40.0
	filesystem := 50.0
	return []model.Observation{
		{Metric: "cpu.utilization_percent", Component: "cpu", Value: &cpu, Unit: "percent", Available: true, ObservedAt: now},
		{Metric: "system.load_1m", Component: "load", Value: &load, Unit: "load", Available: true, ObservedAt: now},
		{Metric: "memory.used_percent", Component: "memory", Value: &memory, Unit: "percent", Available: true, ObservedAt: now},
		{Metric: "filesystem.used_percent", Component: "filesystem", ScopeID: cfg.FilesystemPaths[0], Value: &filesystem, Unit: "percent", Available: true, ObservedAt: now},
	}
}

func baselineSecurity(now time.Time) []config.SecurityEvidence {
	return []config.SecurityEvidence{{
		Check: "sample-security-control", State: "HEALTHY", Source: "synthetic-fixture",
		Summary: "Synthetic baseline only; not a statement about the host.", ObservedAt: now,
	}}
}
