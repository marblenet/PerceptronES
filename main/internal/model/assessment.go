package model

import "time"

const (
	TypeObservation         = "observation.host.v1"
	TypeCycle               = "assessment.cycle.started.v1"
	TypePressure            = "pressure.host.v1"
	TypeHealth              = "health.host.v1"
	TypeDetermination       = "determination.host.v1"
	TypeMission             = "mission.host.v1"
	TypeSecurityObservation = "observation.security.v1"
	TypeSecurityFinding     = "security.finding.v1"
	TypeRecommendation      = "recommendation.host.v1"
	TypePolicyDecision      = "policy.decision.v1"
	TypeAction              = "action.record.v1"
	TypeOutcome             = "outcome.host.v1"
	TypeLearning            = "learning.candidate.v1"
)

// Recommendation is advisory. It is never authorization to act.
type Recommendation struct {
	PatternID      string    `json:"pattern_id"`
	Domain         string    `json:"domain"`
	Title          string    `json:"title"`
	Severity       string    `json:"severity"`
	Priority       int       `json:"priority"`
	ActionKind     string    `json:"action_kind"`
	Recommendation string    `json:"recommendation"`
	Advisory       bool      `json:"advisory"`
	EvaluatedAt    time.Time `json:"evaluated_at"`
}

// PolicyDecision answers "can we do this?" for one recommendation.
type PolicyDecision struct {
	PatternID   string    `json:"pattern_id"`
	ActionKind  string    `json:"action_kind"`
	Decision    string    `json:"decision"`
	Rule        string    `json:"rule"`
	Explanation string    `json:"explanation"`
	EvaluatedAt time.Time `json:"evaluated_at"`
}

// ActionRecord states what happened to a policy decision. Executed is false
// in this build: no action executor exists.
type ActionRecord struct {
	PatternID   string    `json:"pattern_id"`
	ActionKind  string    `json:"action_kind"`
	Decision    string    `json:"decision"`
	Status      string    `json:"status"`
	Executed    bool      `json:"executed"`
	Reason      string    `json:"reason"`
	EvaluatedAt time.Time `json:"evaluated_at"`
}

// Outcome reports how a previously determined condition changed between
// cycles. It is never attributed to an action unless one was executed.
type Outcome struct {
	PatternID        string    `json:"pattern_id"`
	Title            string    `json:"title"`
	Domain           string    `json:"domain"`
	Status           string    `json:"status"`
	PreviousSeverity string    `json:"previous_severity"`
	CurrentSeverity  string    `json:"current_severity,omitempty"`
	ActionAttributed bool      `json:"action_attributed"`
	Explanation      string    `json:"explanation"`
	EvaluatedAt      time.Time `json:"evaluated_at"`
}

// LearningCandidate proposes knowledge or a regression test. It is never applied automatically.
type LearningCandidate struct {
	PatternID   string    `json:"pattern_id"`
	Kind        string    `json:"kind"`
	Basis       string    `json:"basis"`
	Proposal    string    `json:"proposal"`
	Status      string    `json:"status"`
	Applied     bool      `json:"applied"`
	EvaluatedAt time.Time `json:"evaluated_at"`
}

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

type PatternEvidence struct {
	Metric  string   `json:"metric"`
	Value   *float64 `json:"value,omitempty"`
	Unit    string   `json:"unit"`
	State   string   `json:"state"`
	EventID string   `json:"event_id"`
}

// Determination is a knowledge-pattern interpretation of combined information.
// It is an interpretation of what the evidence may indicate, not a confirmed
// cause or an authorization to act.
type Determination struct {
	PatternID       string            `json:"pattern_id"`
	KnowledgeSource string            `json:"knowledge_source"`
	KnowledgeWeight float64           `json:"knowledge_weight"`
	Domain          string            `json:"domain"`
	Title           string            `json:"title"`
	Severity        string            `json:"severity"`
	Interpretation  string            `json:"interpretation"`
	Recommendation  string            `json:"recommendation,omitempty"`
	Evidence        []PatternEvidence `json:"evidence"`
	EvaluatedAt     time.Time         `json:"evaluated_at"`
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
