package model

import "time"

const (
	TypeObservation         = "observation.host.v1"
	TypeCycle               = "assessment.cycle.started.v1"
	TypePressure            = "pressure.host.v1"
	TypeHealth              = "health.host.v1"
	TypeMission             = "mission.host.v1"
	TypeSecurityObservation = "observation.security.v1"
	TypeSecurityFinding     = "security.finding.v1"
)

type CycleContext struct {
	HostID string `json:"host_id"`
}

type Observation struct {
	Metric        string    `json:"metric"`
	Component     string    `json:"component"`
	ScopeID       string    `json:"scope_id,omitempty"`
	Target        string    `json:"target,omitempty"`
	Value         *float64  `json:"value,omitempty"`
	LatencyMillis *float64  `json:"latency_millis,omitempty"`
	Unit          string    `json:"unit"`
	Available     bool      `json:"available"`
	Reason        string    `json:"unavailable_reason,omitempty"`
	ObservedAt    time.Time `json:"observed_at"`
}

type SecurityObservation struct {
	Check      string    `json:"check"`
	State      string    `json:"state"`
	Source     string    `json:"source"`
	Summary    string    `json:"summary"`
	ObservedAt time.Time `json:"observed_at"`
}

type SecurityFinding struct {
	Check       string    `json:"check"`
	State       string    `json:"state"`
	Source      string    `json:"source"`
	Summary     string    `json:"summary"`
	Freshness   string    `json:"freshness"`
	ObservedAt  time.Time `json:"observed_at"`
	EvaluatedAt time.Time `json:"evaluated_at"`
	Explanation string    `json:"explanation"`
}

type PressureState string

const (
	PressureNormal       PressureState = "NORMAL"
	PressureHigh         PressureState = "HIGH"
	PressureCritical     PressureState = "CRITICAL"
	PressureUnknown      PressureState = "UNKNOWN"
	PressureNotEvaluated PressureState = "NOT_EVALUATED"
)

type PressureAssessment struct {
	Metric      string        `json:"metric"`
	Component   string        `json:"component"`
	State       PressureState `json:"state"`
	Value       *float64      `json:"value,omitempty"`
	WarningAt   *float64      `json:"warning_at,omitempty"`
	CriticalAt  *float64      `json:"critical_at,omitempty"`
	Unit        string        `json:"unit"`
	Freshness   string        `json:"freshness"`
	ObservedAt  time.Time     `json:"observed_at"`
	EvaluatedAt time.Time     `json:"evaluated_at"`
	Explanation string        `json:"explanation"`
}

type HealthState string

const (
	HealthHealthy  HealthState = "HEALTHY"
	HealthDegraded HealthState = "DEGRADED"
	HealthCritical HealthState = "CRITICAL"
	HealthUnknown  HealthState = "UNKNOWN"
)

type HealthAssessment struct {
	Component   string      `json:"component"`
	State       HealthState `json:"state"`
	Metric      string      `json:"metric"`
	ScopeID     string      `json:"scope_id"`
	ObservedAt  time.Time   `json:"observed_at"`
	EvaluatedAt time.Time   `json:"evaluated_at"`
	Freshness   string      `json:"freshness"`
	Coverage    string      `json:"coverage"`
	Explanation string      `json:"explanation"`
}

type Coverage struct {
	RequiredComponents []string `json:"required_components"`
	AssessedComponents []string `json:"assessed_components"`
	MissingComponents  []string `json:"missing_components,omitempty"`
	Complete           bool     `json:"complete"`
}

type MissionStatus string

const (
	MissionGreen   MissionStatus = "GREEN"
	MissionYellow  MissionStatus = "YELLOW"
	MissionRed     MissionStatus = "RED"
	MissionUnknown MissionStatus = "UNKNOWN"
)

type MissionAssessment struct {
	Scope            EntityRef          `json:"scope"`
	Objective        string             `json:"objective"`
	Status           MissionStatus      `json:"status"`
	Disposition      string             `json:"disposition"`
	Coverage         Coverage           `json:"coverage"`
	Freshness        string             `json:"freshness"`
	EvaluatedAt      time.Time          `json:"evaluated_at"`
	Explanation      string             `json:"explanation"`
	Components       []HealthAssessment `json:"components"`
	SecurityFindings []SecurityFinding  `json:"security_findings,omitempty"`
}
