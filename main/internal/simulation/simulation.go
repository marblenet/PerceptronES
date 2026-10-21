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
	ID                  string   `json:"id"`
	Title               string   `json:"title"`
	Status              string   `json:"status"`
	Summary             string   `json:"summary"`
	ContributingFactors []string `json:"contributing_factors"`
	MissingEvidence     []string `json:"missing_evidence"`
	Recommendations     []string `json:"recommendations"`
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
	now := time.Now().UTC()
	observations, evidence, err := selected.build(cfg, now)
	if err != nil {
		return Result{}, err
	}
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
	cycle, err := runtime.Run(ctx, cycleID, observations)
	if err != nil {
		return Result{}, fmt.Errorf("run synthetic scenario %q: %w", name, err)
	}
	incident := buildIncidentReport(cycleID, cycle.Mission)
	available := len(observations)
	return Result{
		Scenario:       name,
		Label:          selected.label,
		Synthetic:      true,
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
		if value := values[observations[index].Component]; value != nil {
			observations[index].Value = value
		}
	}
}

func buildIncidentReport(id string, mission *model.MissionAssessment) IncidentReport {
	result := IncidentReport{
		ID: id, Title: "Synthetic mission assessment", Status: string(model.MissionUnknown),
		Summary:             "No mission assessment was produced for this synthetic cycle.",
		ContributingFactors: []string{}, MissingEvidence: []string{}, Recommendations: []string{},
	}
	if mission == nil {
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
		result.Title = "Critical synthetic findings"
		result.Recommendations = append(result.Recommendations, "Review the critical evidence and follow the relevant operational runbook; this simulation does not authorize remediation.")
	case model.MissionYellow:
		result.Title = "Elevated synthetic findings"
		result.Recommendations = append(result.Recommendations, "Review elevated evidence and compare with current operational context before deciding on a response.")
	case model.MissionGreen:
		result.Title = "Synthetic assessment within configured criteria"
		result.Recommendations = append(result.Recommendations, "No simulated risk is reported; this result is not evidence about the live host.")
	default:
		result.Title = "Synthetic assessment is incomplete"
		result.Recommendations = append(result.Recommendations, "Resolve missing or uncertain evidence before treating the simulated mission as healthy.")
	}
	if len(result.ContributingFactors) == 0 && len(result.MissingEvidence) > 0 {
		result.ContributingFactors = append(result.ContributingFactors, "Required evidence is missing or unassessed.")
	}
	if len(result.ContributingFactors) == 0 {
		result.ContributingFactors = append(result.ContributingFactors, "No adverse contributing factors in this synthetic scenario.")
	}
	return result
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
