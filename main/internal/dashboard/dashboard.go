package dashboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"perceptronsre/internal/config"
	"perceptronsre/internal/engine"
	"perceptronsre/internal/model"
	"perceptronsre/internal/report"
	"perceptronsre/internal/simulation"
)

type Dashboard struct {
	mu              sync.RWMutex
	latest          *report.CycleReport
	toolActivity    *ToolActivity
	refreshRequests chan<- ToolActivity
	allowedHost     string
	configStore     *config.Store
}

type ToolActivity struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	RequestedAt time.Time `json:"requested_at"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	CycleID     string    `json:"cycle_id,omitempty"`
	Message     string    `json:"message,omitempty"`
}

type Recommendation struct {
	RuleID             string   `json:"rule_id"`
	Title              string   `json:"title"`
	Rationale          string   `json:"rationale"`
	ToolName           string   `json:"tool_name"`
	ToolDescription    string   `json:"tool_description"`
	EvidenceEventIDs   []string `json:"evidence_event_ids"`
	AutomaticExecution bool     `json:"automatic_execution"`
}

type dashboardReport struct {
	report.CycleReport
	Recommendation Recommendation            `json:"recommendation"`
	ToolActivity   *ToolActivity             `json:"tool_activity,omitempty"`
	Configuration  *config.Config            `json:"configuration,omitempty"`
	LiveIncident   simulation.IncidentReport `json:"live_incident"`
}

func ListenLoopback(port int) (net.Listener, error) {
	if port < 0 || port > 65535 {
		return nil, fmt.Errorf("dashboard port must be between 0 and 65535, got %d", port)
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("listen on dashboard loopback address: %w", err)
	}
	return listener, nil
}

func New(refreshRequests ...chan<- ToolActivity) *Dashboard {
	dashboard := &Dashboard{}
	if len(refreshRequests) > 0 {
		dashboard.refreshRequests = refreshRequests[0]
	}
	return dashboard
}

func (dashboard *Dashboard) SetAllowedHost(host string) error {
	if strings.TrimSpace(host) == "" {
		return errors.New("dashboard allowed host must not be empty")
	}
	dashboard.mu.Lock()
	dashboard.allowedHost = host
	dashboard.mu.Unlock()
	return nil
}

func (dashboard *Dashboard) SetConfigStore(store *config.Store) {
	dashboard.mu.Lock()
	dashboard.configStore = store
	dashboard.mu.Unlock()
}

func (dashboard *Dashboard) Update(cycle report.CycleReport) error {
	if cycle.Runtime.CycleID == "" {
		return errors.New("dashboard report requires a cycle ID")
	}
	if _, err := json.Marshal(cycle); err != nil {
		return fmt.Errorf("encode dashboard cycle: %w", err)
	}
	dashboard.mu.Lock()
	copy := cycle
	dashboard.latest = &copy
	dashboard.mu.Unlock()
	return nil
}

func (dashboard *Dashboard) MarkToolStarted(id string) {
	dashboard.mu.Lock()
	defer dashboard.mu.Unlock()
	if dashboard.toolActivity == nil || dashboard.toolActivity.ID != id {
		return
	}
	now := time.Now().UTC()
	dashboard.toolActivity.Status = "running"
	dashboard.toolActivity.StartedAt = now
	dashboard.toolActivity.Message = "Collecting a fresh read-only host observation snapshot."
}

func (dashboard *Dashboard) CompleteTool(id, cycleID string, toolErr error) {
	dashboard.mu.Lock()
	defer dashboard.mu.Unlock()
	if dashboard.toolActivity == nil || dashboard.toolActivity.ID != id {
		return
	}
	dashboard.toolActivity.CompletedAt = time.Now().UTC()
	dashboard.toolActivity.CycleID = cycleID
	if toolErr != nil {
		dashboard.toolActivity.Status = "failed"
		dashboard.toolActivity.Message = toolErr.Error()
		return
	}
	dashboard.toolActivity.Status = "completed"
	dashboard.toolActivity.Message = "Read-only host collection completed; no host changes were made."
}

func (dashboard *Dashboard) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", dashboard.handleIndex)
	mux.HandleFunc("/api/report", dashboard.handleReport)
	mux.HandleFunc("/api/findings", dashboard.handleFindings)
	mux.HandleFunc("/api/engines", dashboard.handleEngines)
	mux.HandleFunc("/api/config", dashboard.handleConfig)
	mux.HandleFunc("/api/tools/refresh", dashboard.handleRefresh)
	mux.HandleFunc("/api/simulate", dashboard.handleSimulation)
	mux.HandleFunc("/metrics", dashboard.handleMetrics)
	mux.HandleFunc("/healthz", dashboard.handleHealth)
	return securityHeaders(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		dashboard.mu.RLock()
		allowedHost := dashboard.allowedHost
		dashboard.mu.RUnlock()
		if allowedHost != "" && request.Host != allowedHost {
			http.Error(writer, "unexpected Host header", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(writer, request)
	}))
}

func (dashboard *Dashboard) handleIndex(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	if request.URL.Path != "/" {
		http.NotFound(writer, request)
		return
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	_, _ = writer.Write([]byte(indexHTML))
}

func (dashboard *Dashboard) latestReport(writer http.ResponseWriter, request *http.Request) (*report.CycleReport, bool) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return nil, false
	}
	dashboard.mu.RLock()
	defer dashboard.mu.RUnlock()
	if dashboard.latest == nil {
		http.Error(writer, "no assessment is available yet", http.StatusServiceUnavailable)
		return nil, false
	}
	cycle := *dashboard.latest
	return &cycle, true
}

func (dashboard *Dashboard) handleReport(writer http.ResponseWriter, request *http.Request) {
	cycle, ok := dashboard.latestReport(writer, request)
	if !ok {
		return
	}
	dashboard.mu.RLock()
	var activity *ToolActivity
	if dashboard.toolActivity != nil {
		copy := *dashboard.toolActivity
		activity = &copy
	}
	var configuration *config.Config
	if dashboard.configStore != nil {
		current := dashboard.configStore.Current()
		configuration = &current
	}
	dashboard.mu.RUnlock()
	response := dashboardReport{
		CycleReport:    *cycle,
		Recommendation: recommend(*cycle),
		ToolActivity:   activity,
		Configuration:  configuration,
		LiveIncident:   simulation.BuildLiveIncident(cycle.Runtime.CycleID, cycle.Runtime.Mission),
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(writer).Encode(response); err != nil {
		return
	}
}

func (dashboard *Dashboard) handleFindings(writer http.ResponseWriter, request *http.Request) {
	cycle, ok := dashboard.latestReport(writer, request)
	if !ok {
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(writer).Encode(cycle); err != nil {
		return
	}
}

type engineCatalogEntry struct {
	Name       string              `json:"name"`
	Version    string              `json:"version"`
	Category   string              `json:"category"`
	Role       string              `json:"role"`
	Source     string              `json:"source"`
	Enabled    bool                `json:"enabled"`
	Policy     config.EnginePolicy `json:"policy"`
	Consumes   []string            `json:"consumes"`
	Produces   []string            `json:"produces"`
	InstallRef string              `json:"install_ref,omitempty"`
}

func (dashboard *Dashboard) handleEngines(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	dashboard.mu.RLock()
	store := dashboard.configStore
	dashboard.mu.RUnlock()
	if store == nil {
		http.Error(writer, "engine catalog is unavailable", http.StatusServiceUnavailable)
		return
	}
	cfg := store.Current()
	enabled := make(map[string]bool, len(cfg.Engines))
	for _, name := range cfg.Engines {
		enabled[name] = true
	}
	entries := []engineCatalogEntry{
		{Name: "host-pressure", Version: model.CoreVersion, Category: "host health", Role: "pressure", Source: "bundled", Enabled: enabled["host-pressure"], Policy: config.EnginePolicy{Mode: "reasoning_only", ReadOnly: true}, Consumes: []string{model.TypeObservation}, Produces: []string{model.TypePressure}},
		{Name: "host-health", Version: model.CoreVersion, Category: "host health", Role: "health", Source: "bundled", Enabled: enabled["host-health"], Policy: config.EnginePolicy{Mode: "reasoning_only", ReadOnly: true}, Consumes: []string{model.TypePressure}, Produces: []string{model.TypeHealth}},
		{Name: "security-posture", Version: model.CoreVersion, Category: "security", Role: "security assessment", Source: "bundled", Enabled: enabled["security-posture"], Policy: config.EnginePolicy{Mode: "reasoning_only", ReadOnly: true}, Consumes: []string{model.TypeSecurityObservation}, Produces: []string{model.TypeSecurityFinding}},
	}
	for _, plugin := range cfg.Plugins {
		entries = append(entries, engineCatalogEntry{
			Name: plugin.Name, Version: plugin.Version, Category: plugin.Category, Role: plugin.Role,
			Source: "local", Enabled: enabled[plugin.Name], Policy: plugin.Policy,
			Consumes: append([]string(nil), plugin.Consumes...), Produces: append([]string(nil), plugin.Produces...),
			InstallRef: "main/external/" + plugin.Name,
		})
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(writer).Encode(entries); err != nil {
		return
	}
}

func (dashboard *Dashboard) handleSimulation(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !sameOriginDashboardRequest(request) {
		http.Error(writer, "same-origin dashboard request required", http.StatusForbidden)
		return
	}
	dashboard.mu.RLock()
	store := dashboard.configStore
	dashboard.mu.RUnlock()
	if store == nil {
		http.Error(writer, "simulation requires an available runtime configuration", http.StatusServiceUnavailable)
		return
	}
	var input struct {
		Scenario    string            `json:"scenario"`
		UseSnapshot bool              `json:"use_snapshot"`
		Inputs      simulation.Inputs `json:"inputs"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 8192))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		http.Error(writer, "invalid simulation request: "+err.Error(), http.StatusBadRequest)
		return
	}
	var result simulation.Result
	var err error
	if input.UseSnapshot {
		dashboard.mu.RLock()
		var latest *report.CycleReport
		if dashboard.latest != nil {
			copy := *dashboard.latest
			latest = &copy
		}
		dashboard.mu.RUnlock()
		if latest == nil {
			http.Error(writer, "no live assessment is available to snapshot yet", http.StatusServiceUnavailable)
			return
		}
		var snapshot []model.Observation
		for _, event := range latest.Runtime.Events {
			if event.Type != model.TypeObservation {
				continue
			}
			var observation model.Observation
			if json.Unmarshal(event.Payload, &observation) == nil {
				snapshot = append(snapshot, observation)
			}
		}
		result, err = simulation.RunSnapshot(request.Context(), store.Current(), latest.Runtime.CycleID, snapshot, input.Inputs)
	} else {
		result, err = simulation.RunScenarioWithInputs(request.Context(), store.Current(), input.Scenario, input.Inputs)
	}
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(writer).Encode(result); err != nil {
		return
	}
}

func (dashboard *Dashboard) handleMetrics(writer http.ResponseWriter, request *http.Request) {
	cycle, ok := dashboard.latestReport(writer, request)
	if !ok {
		return
	}
	writer.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	if err := report.WritePrometheus(writer, *cycle); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

func (dashboard *Dashboard) handleConfig(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		dashboard.mu.RLock()
		store := dashboard.configStore
		dashboard.mu.RUnlock()
		if store == nil {
			http.Error(writer, "configuration editing is unavailable", http.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(writer).Encode(store.Current()); err != nil {
			return
		}
		return
	}
	if request.Method != http.MethodPut {
		writer.Header().Set("Allow", "GET, PUT")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !sameOriginDashboardRequest(request) {
		http.Error(writer, "same-origin dashboard request required", http.StatusForbidden)
		return
	}
	dashboard.mu.RLock()
	store := dashboard.configStore
	dashboard.mu.RUnlock()
	if store == nil {
		http.Error(writer, "configuration editing is unavailable", http.StatusServiceUnavailable)
		return
	}
	configuration, err := config.LoadFrom(http.MaxBytesReader(writer, request.Body, 64*1024))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := engine.NewRuntime(configuration); err != nil {
		http.Error(writer, "invalid engine selection: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := store.Update(configuration); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(writer).Encode(store.Current()); err != nil {
		return
	}
}

func (dashboard *Dashboard) handleRefresh(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !sameOriginDashboardRequest(request) {
		http.Error(writer, "same-origin dashboard request required", http.StatusForbidden)
		return
	}
	if dashboard.refreshRequests == nil {
		http.Error(writer, "read-only refresh tool is unavailable", http.StatusServiceUnavailable)
		return
	}
	id, err := model.NewID()
	if err != nil {
		http.Error(writer, "could not create tool request identity", http.StatusInternalServerError)
		return
	}
	activity := ToolActivity{
		ID:          id,
		Name:        "host-observation-refresh",
		Status:      "queued",
		RequestedAt: time.Now().UTC(),
		Message:     "Read-only host observation refresh queued.",
	}
	dashboard.mu.Lock()
	if dashboard.toolActivity != nil && (dashboard.toolActivity.Status == "queued" || dashboard.toolActivity.Status == "running") {
		dashboard.mu.Unlock()
		http.Error(writer, "a read-only refresh is already in progress", http.StatusConflict)
		return
	}
	select {
	case dashboard.refreshRequests <- activity:
		dashboard.toolActivity = &activity
		dashboard.mu.Unlock()
	default:
		dashboard.mu.Unlock()
		http.Error(writer, "read-only refresh queue is full", http.StatusServiceUnavailable)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(writer).Encode(activity)
}

func sameOriginDashboardRequest(request *http.Request) bool {
	if request.Header.Get("X-Requested-With") != "PerceptronDashboard" {
		return false
	}
	origin, err := url.Parse(request.Header.Get("Origin"))
	if err != nil || origin.Host == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return false
	}
	return origin.Scheme == "http" && origin.Host == request.Host
}

func recommend(cycle report.CycleReport) Recommendation {
	mission := cycle.Runtime.Mission
	result := Recommendation{
		RuleID:             "dashboard-advisory-v0.1",
		Title:              "Continue observing",
		Rationale:          "The current assessment does not report elevated host or configured security risk. This is an advisory, not authorization.",
		ToolName:           "host-observation-refresh",
		ToolDescription:    "Collect a fresh read-only host snapshot and recalculate the assessment. No changes are made to the host.",
		EvidenceEventIDs:   make([]string, 0),
		AutomaticExecution: false,
	}
	if mission == nil {
		result.Title = "Assessment unavailable"
		result.Rationale = "No mission assessment is available; do not infer a healthy state."
		return result
	}
	var critical, degraded, unknown []string
	for _, component := range mission.Components {
		switch component.State {
		case model.HealthCritical:
			critical = append(critical, component.Component)
		case model.HealthDegraded:
			degraded = append(degraded, component.Component)
		case model.HealthUnknown:
			unknown = append(unknown, component.Component)
		}
	}
	for _, finding := range mission.SecurityFindings {
		switch finding.State {
		case "CRITICAL":
			critical = append(critical, finding.Check)
		case "WARNING":
			degraded = append(degraded, finding.Check)
		case "UNKNOWN":
			unknown = append(unknown, finding.Check)
		}
	}
	switch mission.Status {
	case model.MissionRed:
		result.Title = "Review critical findings"
		result.Rationale = "Critical findings are reported for " + joinComponents(critical) + ". Review the evidence and use a fresh read-only collection to confirm current conditions; no remediation is authorized."
	case model.MissionYellow:
		result.Title = "Verify elevated findings"
		result.Rationale = "Elevated findings are reported for " + joinComponents(degraded) + ". Compare with fresh read-only evidence before deciding on an operational response."
		if len(unknown) > 0 {
			result.Rationale += " Evidence is also unknown for " + joinComponents(unknown) + "."
		}
	case model.MissionUnknown:
		result.Title = "Resolve assessment gaps"
		if len(unknown) > 0 {
			result.Rationale = "Required evidence is unknown for " + joinComponents(unknown) + ". A read-only refresh may clarify current coverage; it cannot establish missing sensor support."
		} else {
			result.Rationale = "Required component assessments are missing. A read-only refresh may fill current observation gaps."
		}
	default:
		if len(unknown) > 0 {
			result.Rationale += " Evidence is also unknown for " + joinComponents(unknown) + "."
		}
	}
	evidenceEventIDs := make(map[string]bool)
	var missionEventID string
	for _, event := range cycle.Runtime.Events {
		switch event.Type {
		case model.TypeHealth:
			var assessment model.HealthAssessment
			if err := json.Unmarshal(event.Payload, &assessment); err == nil {
				for _, component := range mission.Components {
					if component.Component == assessment.Component && component.Metric == assessment.Metric && component.ScopeID == assessment.ScopeID {
						evidenceEventIDs[event.ID] = true
					}
				}
			}
		case model.TypeMission:
			missionEventID = event.ID
		case model.TypeSecurityFinding:
			var finding model.SecurityFinding
			if err := json.Unmarshal(event.Payload, &finding); err == nil {
				for _, assessed := range mission.SecurityFindings {
					if finding.Check == assessed.Check && finding.Source == assessed.Source && finding.ObservedAt.Equal(assessed.ObservedAt) {
						evidenceEventIDs[event.ID] = true
					}
				}
			}
		}
	}
	for _, event := range cycle.Runtime.Events {
		if evidenceEventIDs[event.ID] || event.ID == missionEventID {
			result.EvidenceEventIDs = append(result.EvidenceEventIDs, event.ID)
		}
	}
	return result
}

func joinComponents(components []string) string {
	if len(components) == 0 {
		return "one or more required components"
	}
	return strings.Join(components, ", ")
}

func (dashboard *Dashboard) handleHealth(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	dashboard.mu.RLock()
	ready := dashboard.latest != nil
	dashboard.mu.RUnlock()
	if !ready {
		http.Error(writer, "waiting for first assessment", http.StatusServiceUnavailable)
		return
	}
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = writer.Write([]byte("ok\n"))
}

func methodNotAllowed(writer http.ResponseWriter) {
	writer.Header().Set("Allow", http.MethodGet)
	http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(writer, request)
	})
}

const indexHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="color-scheme" content="dark">
  <title>Perceptron SRE · Local Operations</title>
  <style>
    :root{color-scheme:dark;--bg:#0b1020;--panel:#121a2c;--panel2:#172238;--line:#26344d;--text:#edf3ff;--muted:#92a2bd;--cyan:#6be3dc;--blue:#82aaff;--green:#45d6a0;--amber:#f3bd62;--red:#ff7185;--shadow:0 18px 55px #03071366}
    *{box-sizing:border-box}body{margin:0;background:radial-gradient(ellipse at 85% -15%,#18304c 0,transparent 38%),var(--bg);color:var(--text);font:15px/1.5 Inter,ui-sans-serif,system-ui,-apple-system,"Segoe UI",sans-serif}
    .shell{max-width:1200px;margin:0 auto;padding:38px 26px 56px}.top{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:30px}.brand{display:flex;align-items:center;gap:14px}.mark{width:42px;height:42px;border:1px solid #6be3dc66;border-radius:14px;display:grid;place-items:center;color:var(--cyan);background:#6be3dc12;font-size:22px;font-weight:800}.eyebrow{color:var(--cyan);font-size:11px;text-transform:uppercase;letter-spacing:.18em;font-weight:750}.brand h1{font-size:20px;margin:2px 0 0;letter-spacing:-.03em}.connection{display:flex;align-items:center;gap:9px;color:var(--muted);font-size:12px}.dot{height:8px;width:8px;border-radius:50%;background:var(--green);box-shadow:0 0 12px #45d6a088}.dot.offline{background:var(--red);box-shadow:0 0 12px #ff718588}
    .hero{display:grid;grid-template-columns:1fr auto;align-items:center;gap:24px;padding:30px;border:1px solid var(--line);border-radius:22px;background:linear-gradient(120deg,#17243a,#111a2b 72%);box-shadow:var(--shadow)}.hero-copy{min-width:0}.hero-label{color:var(--muted);font-size:12px;text-transform:uppercase;letter-spacing:.14em}.hero h2{font-size:clamp(22px,3vw,30px);line-height:1.15;margin:8px 0 7px;letter-spacing:-.035em}.hero p{margin:0;color:#b5c2d8;max-width:760px;font-size:12px}.mission-factors{margin:10px 0 0;padding-left:20px;color:#dce8fa;font-size:12px;display:grid;gap:4px}.badge{min-width:132px;text-align:center;padding:16px 18px;border:1px solid currentColor;border-radius:18px;font-size:18px;font-weight:800;letter-spacing:.09em;background:#ffffff08}.green{color:var(--green)}.yellow{color:var(--amber)}.red{color:var(--red)}.unknown{color:#b7c1d2}
    .grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:14px;margin:18px 0 26px}.stat,.section{border:1px solid var(--line);border-radius:16px;background:linear-gradient(150deg,#151f32,#101827)}.stat{padding:17px 18px}.stat-label{color:var(--muted);font-size:11px;text-transform:uppercase;letter-spacing:.12em}.stat-value{margin-top:5px;font-size:19px;font-weight:700;letter-spacing:-.025em;overflow-wrap:anywhere}.stat-note{color:var(--muted);font-size:12px;margin-top:3px}
    .section{padding:22px;margin-top:18px}.section-head{display:flex;justify-content:space-between;align-items:flex-end;gap:14px;margin-bottom:16px}.section h3{margin:0;font-size:16px;letter-spacing:-.02em}.section-sub{color:var(--muted);font-size:12px;margin-top:4px}.component-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.component{border:1px solid var(--line);border-radius:13px;padding:16px;background:#0d1524}.component-top{display:flex;align-items:center;justify-content:space-between;gap:12px}.component-name{font-weight:700;text-transform:capitalize}.state{font-size:10px;font-weight:800;letter-spacing:.1em;border-radius:999px;padding:4px 9px;background:#ffffff0d}.component p{color:#b3c0d4;font-size:13px;margin:12px 0 8px}.meta{display:flex;gap:12px;flex-wrap:wrap;color:var(--muted);font-size:11px}
    .table-wrap{overflow-x:auto}table{width:100%;border-collapse:collapse;font-size:13px}th{text-align:left;color:var(--muted);font-size:10px;text-transform:uppercase;letter-spacing:.12em;font-weight:700;padding:10px;border-bottom:1px solid var(--line)}td{padding:11px 10px;border-bottom:1px solid #26344d88;vertical-align:top}tbody tr:last-child td{border-bottom:0}.metric{font-weight:650}.value{font-variant-numeric:tabular-nums;color:#d8e5f8}.reason{color:var(--muted);max-width:420px}.empty{padding:24px;text-align:center;color:var(--muted)}
    details{margin-top:16px;border-top:1px solid var(--line);padding-top:14px}summary{cursor:pointer;color:#b7c7df;font-size:13px;font-weight:650}pre{white-space:pre-wrap;word-break:break-word;color:#b8c7dc;background:#0b1220;border:1px solid var(--line);border-radius:12px;padding:15px;max-height:440px;overflow:auto;font-size:11px}.footer{display:flex;justify-content:space-between;gap:14px;margin-top:18px;color:var(--muted);font-size:11px}.error{color:var(--red)}
    .layout{display:grid;grid-template-columns:minmax(0,1fr) 340px;align-items:start;gap:18px}.main-column{min-width:0}.sidebar{position:sticky;top:18px;max-height:calc(100vh - 36px);overflow:auto}.layout .section{margin-top:18px}.layout .main-column>.section:first-child{margin-top:0}.recommendation{border:1px solid #4b6484;border-radius:18px;padding:22px;background:linear-gradient(130deg,#1b2c43,#131d2e);box-shadow:var(--shadow)}.recommendation-head{display:flex;align-items:flex-start;justify-content:space-between;gap:14px}.recommendation h3{font-size:19px;margin:4px 0 7px}.recommendation p{color:#b8c6da;margin:0 0 14px}.action-btn{border:1px solid #6be3dc88;background:#6be3dc18;color:#c8fffa;padding:10px 14px;border-radius:11px;font:inherit;font-size:13px;font-weight:750;cursor:pointer;white-space:nowrap}.action-btn:hover:not(:disabled){background:#6be3dc30}.action-btn:disabled{opacity:.55;cursor:wait}.action-note{color:var(--muted);font-size:11px;margin-top:12px}.advisory{display:inline-flex;color:var(--amber);font-size:10px;text-transform:uppercase;letter-spacing:.12em;font-weight:800}.engine-list{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:9px}.engine-chip{border:1px solid var(--line);border-radius:11px;padding:11px;background:#0d1524}.engine-chip strong{display:block;font-size:12px}.engine-chip span{color:var(--muted);font-size:10px}.category-stack{display:grid;gap:14px}.category{border:1px solid var(--line);border-radius:15px;background:linear-gradient(150deg,#151f32,#101827);overflow:hidden}.category-head{display:flex;align-items:center;justify-content:space-between;padding:15px 17px;border-bottom:1px solid var(--line);background:#ffffff04}.category-title{font-weight:750}.category-note{color:var(--muted);font-size:11px;margin-top:2px}.category-items{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px;padding:12px}.metric-card{background:#0d1524;border:1px solid var(--line);border-radius:12px;padding:13px}.metric-card-head{display:flex;justify-content:space-between;align-items:center;gap:8px}.metric-name{font-size:13px;font-weight:700}.metric-value{font-size:18px;font-weight:750;font-variant-numeric:tabular-nums;margin:9px 0 3px}.metric-detail{font-size:11px;color:var(--muted);overflow-wrap:anywhere}.metric-card .meta{margin-top:9px;font-size:10px}.side-card{border:1px solid var(--line);border-radius:15px;background:linear-gradient(150deg,#151f32,#101827);padding:17px;margin-bottom:14px}.side-card h3{font-size:14px;margin:0 0 4px}.side-sub{font-size:11px;color:var(--muted);margin-bottom:13px}.rule-id{font:10px ui-monospace,SFMono-Regular,Consolas,monospace;color:var(--cyan);overflow-wrap:anywhere}.evidence-list,.event-list{display:grid;gap:9px}.evidence-item{font:10px ui-monospace,SFMono-Regular,Consolas,monospace;color:#b8c7dc;overflow-wrap:anywhere}.evidence-link{background:transparent;border:0;text-align:left;padding:0;cursor:pointer}.evidence-link:hover{color:var(--cyan)}.timeline-item{border-left:1px solid #40516d;padding:0 0 10px 12px;position:relative;transition:background .2s}.timeline-item:before{content:"";position:absolute;left:-4px;top:4px;width:7px;height:7px;border-radius:50%;background:var(--blue)}.timeline-item.evidence-highlight{background:#6be3dc14;outline:1px solid #6be3dc66;border-radius:5px}.timeline-item strong{display:block;font-size:11px}.timeline-item span{display:block;color:var(--muted);font-size:10px;margin-top:2px;overflow-wrap:anywhere}.timeline-item code{color:#8fa2c0;font-size:9px}.tool-status{padding:9px;border:1px solid var(--line);border-radius:10px;color:#bac8dd;font-size:11px}.tool-status.completed{border-color:#45d6a066;color:var(--green)}.tool-status.running,.tool-status.queued{border-color:#f3bd6266;color:var(--amber)}.tool-status.failed{border-color:#ff718566;color:var(--red)}
    @media(max-width:1020px){.layout{grid-template-columns:minmax(0,1fr) 300px}.engine-list{grid-template-columns:1fr}}@media(max-width:780px){.layout{grid-template-columns:1fr}.sidebar{position:static;max-height:none;display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.sidebar .side-card{margin:0}.sidebar .event-card{grid-column:1/-1}.engine-list{grid-template-columns:repeat(3,minmax(0,1fr))}}@media(max-width:540px){.sidebar{grid-template-columns:1fr}.sidebar .event-card{grid-column:auto}.engine-list{grid-template-columns:1fr 1fr}.recommendation-head{flex-direction:column}.action-btn{width:100%}.category-items{grid-template-columns:1fr}.category-head{align-items:flex-start;gap:8px;flex-direction:column}}
    .decision-grid{display:grid;grid-template-columns:minmax(0,1.1fr) minmax(320px,.9fr);gap:16px;align-items:stretch}.decision-grid .hero,.decision-grid .recommendation{min-height:100%;margin:0}.decision-grid .hero{padding:24px}.mission-facts{margin:14px auto 8px;max-width:1000px;grid-template-columns:repeat(4,minmax(0,1fr))}.mission-facts .stat{padding:13px 16px}.mission-facts .stat-value{font-size:16px}
    .config-panel{margin-bottom:14px}.config-panel>summary{font-size:14px}.config-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px;margin:16px 0}.config-grid label{display:grid;gap:6px;color:#b5c4da;font-size:12px}.config-grid input,.config-grid textarea,.config-thresholds input,.config-actions select{width:100%;min-width:70px;border:1px solid #3a4c67;border-radius:8px;background:#0b1220;color:var(--text);padding:9px;font:inherit}.config-grid textarea{resize:vertical}.config-thresholds{margin:10px 0 16px}.engine-settings{border:1px solid var(--line);border-radius:12px;padding:12px}.engine-settings legend{color:#b5c4da;font-size:12px}.engine-options{display:flex;gap:10px;flex-wrap:wrap}.engine-option{display:flex;gap:7px;align-items:center;border:1px solid var(--line);border-radius:8px;padding:8px 10px;color:#d2def0;font-size:12px}.engine-option small{display:block;color:var(--muted)}.engine-move{border:1px solid var(--line);border-radius:5px;background:#172238;color:#d2def0;cursor:pointer}.config-actions{display:flex;align-items:center;gap:12px;flex-wrap:wrap;margin:14px 0}.secondary-btn{border:1px solid #43536c;background:#111b2b;color:#d2def0;padding:10px 14px;border-radius:11px;font:inherit;font-size:13px;cursor:pointer}.config-plugin-note{color:var(--amber);font-size:11px}.config-error{color:var(--red)!important}
    .pyramid{display:flex;flex-direction:column;align-items:center;gap:14px;margin-top:14px}.tier{width:100%;min-width:0}.tier-diagnosis{width:82%}.tier-engines{width:88%}.tier-components{width:94%}.tier-evidence{width:100%}.tier .section{margin-top:0}.tier-label{display:flex;align-items:center;gap:10px;margin:0 0 9px 3px;color:var(--muted);font-size:10px;text-transform:uppercase;letter-spacing:.14em;font-weight:750}.tier-number{display:grid;place-items:center;width:22px;height:22px;border:1px solid #47617e;border-radius:50%;color:var(--cyan);font-size:10px;letter-spacing:0}
    .diagnosis-summary{padding:17px;border:1px solid #344865;border-radius:13px;background:#0d1524;color:#dce8fa}.diagnosis-factors{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px;margin-top:12px}.diagnosis-factor{padding:13px;border:1px solid var(--line);border-radius:11px;background:#111b2b}.diagnosis-factor p{margin:7px 0 0;color:#b3c0d4;font-size:12px}.diagnosis-footnote{margin:12px 0 0;color:var(--muted);font-size:11px}.reasoning-evidence{margin-top:13px;padding-top:12px;border-top:1px solid var(--line)}.incident-note{margin:0 0 14px;padding:11px 13px;border:1px solid #675637;border-radius:10px;background:#2c2416;color:#e9d5a8;font-size:12px}.timeline-item{position:relative}.timeline-time{color:var(--cyan);font-size:11px;font-variant-numeric:tabular-nums}.evidence-highlight{outline:2px solid var(--cyan);outline-offset:2px}.pyramid .table-wrap{border:1px solid var(--line);border-radius:12px;padding:0 10px}.tier-evidence .section+.section{margin-top:14px}
    @media(max-width:900px){.decision-grid{grid-template-columns:1fr}.decision-grid .hero{grid-template-columns:1fr auto}.tier-diagnosis,.tier-engines,.tier-components{width:100%}.config-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}
    @media(max-width:780px){.grid{grid-template-columns:repeat(2,minmax(0,1fr))}.hero{grid-template-columns:1fr auto;padding:22px}.section{padding:17px}}@media(max-width:540px){.shell{padding:24px 14px 40px}.top{align-items:flex-start}.connection{font-size:0}.connection .dot{flex:none}.hero{grid-template-columns:1fr}.badge{justify-self:start}.component-grid,.diagnosis-factors{grid-template-columns:1fr}.grid{gap:9px}.stat{padding:13px}.stat-value{font-size:16px}.section-head{align-items:flex-start;flex-direction:column}.action-btn{white-space:normal}.recommendation-head{flex-direction:column}.config-grid{grid-template-columns:1fr}}
    .page-nav{display:flex;gap:8px;margin:-12px 0 20px}.page-nav button{border:1px solid var(--line);border-radius:10px;padding:9px 14px;background:#111a2b;color:var(--muted);font:inherit;font-size:13px;cursor:pointer}.page-nav button[aria-current=page]{border-color:#6be3dc88;color:var(--cyan);background:#6be3dc12}.store-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.store-card{border:1px solid var(--line);border-radius:14px;padding:17px;background:#0d1524}.store-card h3{margin:0;font-size:15px}.store-card p{color:#b3c0d4;font-size:12px}.store-meta{display:flex;gap:8px;flex-wrap:wrap;color:var(--muted);font-size:10px;margin-top:10px}.store-policy{font:11px ui-monospace,SFMono-Regular,Consolas,monospace;color:#c5d5eb;overflow-wrap:anywhere}.store-card button{margin-top:12px}.plugin-setting{display:grid;gap:6px;min-width:min(100%,360px);flex:1;color:#c5d5eb;font-size:11px}.plugin-setting textarea{width:100%;border:1px solid #3a4c67;border-radius:8px;background:#0b1220;color:var(--text);padding:9px;font:12px ui-monospace,SFMono-Regular,Consolas,monospace}
    .simulation-inputs{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px}.simulation-inputs label{display:grid;gap:6px;color:#b5c4da;font-size:11px}.simulation-inputs input{width:100%;min-width:0;border:1px solid #3a4c67;border-radius:8px;background:#0b1220;color:var(--text);padding:9px;font:inherit}.simulation-output{display:grid;gap:14px}.simulation-events{display:grid;gap:8px}.simulation-event{padding:11px;border:1px solid var(--line);border-radius:9px;background:#0d1524}.simulation-event code{display:block;margin-top:5px;color:var(--muted);font-size:10px}.simulation-factors{margin:8px 0;padding-left:20px}.chain-tabs{display:flex;gap:6px;margin:12px 0 4px}.cascade{display:flex;gap:10px;align-items:stretch;overflow:auto;padding:4px 0}.cascade-layer{min-width:190px;flex:1;border:1px solid var(--line);padding:10px;background:#0b110d}.cascade-layer h4{margin:0 0 6px;color:var(--green);font-size:12px}.cascade-item{font:11px ui-monospace,Consolas,monospace;color:var(--muted);padding:2px 0;word-break:break-word}.cascade-arrow{align-self:center;color:var(--green)}.chain-tab{border:1px solid var(--line);background:transparent;color:var(--muted);padding:5px 14px;cursor:pointer;font:inherit}.chain-tab.active{color:var(--green);border-color:var(--green)}.simulation-report{overflow:auto;max-height:540px;padding:14px;background:#09111d;border:1px solid var(--line);border-radius:10px;font:11px ui-monospace,SFMono-Regular,Consolas,monospace;color:#c5d5eb}
    @media(max-width:900px){.simulation-inputs{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:540px){.simulation-inputs{grid-template-columns:1fr}}
    .planned-card{border-style:dashed;opacity:.86}.planned-status{display:inline-block;margin:9px 0 2px;padding:4px 8px;border:1px solid #f3bd6266;border-radius:999px;color:var(--amber);font-size:10px;font-weight:800;letter-spacing:.08em;text-transform:uppercase}
    @media(max-width:540px){.store-grid{grid-template-columns:1fr}}
    :root{--bg:#080b09;--panel:#101512;--panel2:#151c17;--line:#29342c;--text:#dce9dc;--muted:#91a192;--cyan:#8fca91;--blue:#8fca91;--green:#83c987;--shadow:0 18px 55px #0009}
    body{background:radial-gradient(ellipse at 85% -15%,#1a281c 0,transparent 38%),var(--bg)}
    .mark{border-color:#83c98766;background:#83c98712;color:var(--green)}
    .hero,.recommendation{background:linear-gradient(120deg,#172019,#101510 75%);border-color:#344336}
    .recommendation{background:linear-gradient(120deg,#192219,#101510);border-color:#405342}
    .section,.stat,.category,.side-card,.component,.metric-card,.engine-chip,.store-card,.simulation-event{background:#111612}
    .component,.metric-card,.engine-chip,.store-card,.simulation-event{background:#0b100c}
    .category{background:#111612}
    .page-nav button{background:#101510;color:var(--text)}
    .page-nav button[aria-current=page]{border-color:#83c987;color:#a5dbaa;background:#83c98714}
    .action-btn{border-color:#83c98788;background:#83c98718;color:#c8e9c9}
    .action-btn:hover:not(:disabled){background:#83c98730}
    .gear-btn{background:#101510;color:var(--green)}
    .config-grid input,.config-grid textarea,.config-thresholds input,.plugin-setting textarea,.simulation-inputs input{border-color:#344336;background:#0b100c}
    .engine-move{background:#101510}
    .tier-number{border-color:#405342;color:var(--green)}
    .timeline-item:before{background:var(--green)}
    .timeline-item.evidence-highlight{background:#83c98714;outline-color:#83c98766}
    .event-rail{border-color:var(--line)}
    .event-node{border-color:var(--line);border-top-color:var(--green);background:#0e130f}
    .event-node summary:before{color:var(--green)}
    .event-node time,.event-node code,.rule-id{color:var(--green)}
    .stat-link:focus-visible,.gear-btn:focus-visible,.page-nav button:focus-visible{outline-color:var(--green)}
    .simulation-report{background:#0b100c;color:var(--text)}
    .mission-facts{grid-template-columns:repeat(6,minmax(0,1fr));max-width:none}.mission-facts .stat-value{font-size:14px}
    .event-rail{display:flex;gap:10px;overflow-x:auto;padding:3px 2px 12px;border-left:1px solid var(--line);border-right:1px solid var(--line);scroll-snap-type:x mandatory}
    .event-node{flex:0 0 260px;scroll-snap-align:start;border:1px solid var(--line);border-top:2px solid var(--green);border-radius:10px;background:#0e130f;padding:11px}
    .event-node summary{display:grid;gap:5px;list-style:none}.event-node summary::-webkit-details-marker{display:none}.event-node summary:before{content:"›";color:var(--green);float:right}.event-node[open] summary:before{content:"⌄"}
    .event-node time,.event-node code{font:10px ui-monospace,SFMono-Regular,Consolas,monospace;color:var(--muted)}.event-node strong{color:var(--text);font-size:12px}.event-node pre{max-height:260px}
    .top-actions{display:flex;align-items:center;gap:12px}.gear-btn{width:40px;height:40px;border:1px solid var(--line);border-radius:12px;background:#101510;color:var(--green);font-size:20px;cursor:pointer}
    .page-title{display:flex;align-items:center;justify-content:space-between;gap:12px}.page-title h2{margin:0}.stat-link{cursor:pointer}.stat-link:focus-visible,.gear-btn:focus-visible,.page-nav button:focus-visible{outline:2px solid var(--green);outline-offset:2px}
    .engine-stat{padding:0;overflow:hidden}.engine-stat>summary{list-style:none;padding:14px 16px;cursor:pointer}.engine-stat>summary::-webkit-details-marker{display:none}.engine-stat>summary .stat-value{display:flex;justify-content:space-between;align-items:center;gap:10px}
    #operations-page>.config-panel{display:none}
    @media(max-width:1100px){.mission-facts{grid-template-columns:repeat(3,minmax(0,1fr))}}@media(max-width:540px){.mission-facts{grid-template-columns:repeat(2,minmax(0,1fr))}.event-node{flex-basis:210px}}
  </style>
</head>
<body>
  <main class="shell">
    <header class="top">
      <div class="brand"><div class="mark" aria-hidden="true">P</div><div><div class="eyebrow">Local operations</div><h1>Perceptron SRE</h1></div></div>
      <div class="top-actions"><div class="connection"><span id="connection-dot" class="dot offline"></span><span id="connection">Connecting to local runtime…</span></div><button id="settings-tab" class="gear-btn" type="button" title="Runtime configuration" aria-label="Open runtime configuration">⚙</button></div>
    </header>
    <nav class="page-nav" aria-label="Dashboard pages"><button id="operations-tab" type="button" aria-current="page">Operations</button><button id="security-tab" type="button">Security</button><button id="simulation-tab" type="button">Simulation</button><button id="store-tab" type="button">Engine Store</button></nav>
    <div id="operations-page">
    <section class="section" aria-labelledby="timeline-heading"><div class="section-head"><div><div class="header-label">Event stream · newest on left</div><h3 id="timeline-heading">Event timeline &amp; provenance</h3><div id="provenance" class="section-sub">Expandable event cards · left is newest, right is oldest</div></div></div><div id="event-list" class="event-rail"><div class="empty">Waiting for event lineage…</div></div></section>
    <section class="decision-grid" aria-label="Mission decision and recommended next step">
      <section class="hero" aria-live="polite">
        <div class="hero-copy"><div class="hero-label">The Alert Window · <span id="scope">Mission assessment</span></div><h2 id="mission-headline">Waiting for first assessment</h2><p>Objective: <span id="objective">Waiting for mission objective</span></p><ul id="mission-factors" class="mission-factors"><li>The dashboard will update as soon as the first observation cycle completes.</li></ul><button id="alert-security" class="section-sub stat-link" type="button">Security posture: not assessed · view assessment</button></div>
        <div id="mission-badge" class="badge unknown">WAITING</div>
      </section>
      <section class="recommendation" aria-live="polite"><div class="recommendation-head"><div><div class="hero-label">Suggested Mitigation Window</div><span class="advisory">Advisory · no automatic execution</span><h3 id="recommendation-title">Waiting for recommendation</h3><p id="recommendation-rationale">A recommendation will appear after the first assessment.</p></div><button id="refresh-tool" class="action-btn" type="button">Run read-only refresh</button></div><div class="action-note">Tool: <strong>host-observation-refresh</strong> · Collects current host metrics only; makes no system changes.</div><div id="tool-activity" class="tool-status" style="margin-top:12px">No dashboard-triggered tool run yet.</div><details><summary>Recommendation evidence and policy</summary><div id="recommendation-rule" class="rule-id"></div><p id="recommendation-reason" class="section-sub"></p><div id="recommendation-evidence" class="evidence-list"></div></details></section>
    </section>
    <section id="stats" class="grid mission-facts" aria-label="Mission evidence summary"></section>
    <details id="configuration-panel" class="section config-panel"><summary>Runtime configuration · edit thresholds and enabled engines</summary><div class="section-sub">Changes are validated and written to the configuration file. External file edits are detected before the next collection cycle.</div><form id="configuration-form">
      <div class="config-grid">
        <label>Host identifier<input id="cfg-host" type="text" required></label>
        <label>DNS probe target (blank disables)<input id="cfg-dns-name" type="text"></label>
        <label>CPU sample interval<input id="cfg-cpu-interval" type="text" required></label>
        <label>Maximum observation age<input id="cfg-max-age" type="text" required></label>
        <label>DNS probe timeout<input id="cfg-dns-timeout" type="text" required></label>
        <label>Filesystem paths (one per line)<textarea id="cfg-filesystems" rows="2" required></textarea></label>
      </div>
      <fieldset class="engine-settings"><legend>host-pressure · Host health · warning and critical thresholds</legend><div class="table-wrap config-thresholds"><table><thead><tr><th>Input</th><th>Warning</th><th>Critical</th><th>Unit</th></tr></thead><tbody>
        <tr><td>CPU utilization</td><td><input id="threshold-cpu-warning" type="number" step="0.1"></td><td><input id="threshold-cpu-critical" type="number" step="0.1"></td><td>%</td></tr>
        <tr><td>Memory used</td><td><input id="threshold-memory-warning" type="number" step="0.1"></td><td><input id="threshold-memory-critical" type="number" step="0.1"></td><td>%</td></tr>
        <tr><td>Filesystem used</td><td><input id="threshold-filesystem-warning" type="number" step="0.1"></td><td><input id="threshold-filesystem-critical" type="number" step="0.1"></td><td>%</td></tr>
        <tr><td>Load per CPU</td><td><input id="threshold-load-warning" type="number" step="0.1"></td><td><input id="threshold-load-critical" type="number" step="0.1"></td><td>load / CPU</td></tr>
      </tbody></table></div><p class="section-sub">Thresholds currently apply to CPU, memory, filesystem utilization, and load. Network counters and process count are observed but do not yet have pressure rules.</p></fieldset>
      <fieldset class="engine-settings"><legend>Configured external engine settings</legend><div id="plugin-settings" class="engine-options">No external engine settings.</div></fieldset>
      <fieldset class="engine-settings"><legend>Security evidence input</legend><label class="plugin-setting">Required security checks (one exact check name per line)<textarea id="security-required-checks" rows="3" spellcheck="false"></textarea></label><p class="section-sub">When security-posture is enabled, every listed check must have a matching evidence record for coverage to be complete. Leave the list empty only if you want security scope explicitly incomplete (mission UNKNOWN absent a known warning/critical finding).</p><label class="plugin-setting">Manually supplied security evidence (JSON array)<textarea id="security-evidence" rows="6" spellcheck="false"></textarea></label><p class="section-sub">Evidence records require check, state (HEALTHY, WARNING, CRITICAL, or UNKNOWN), source, summary, and RFC3339 observed_at. Check names must match the required list exactly. This prototype assesses supplied evidence only; it does not scan hosts, install updates, or verify the source.</p></fieldset>
      <fieldset class="engine-settings"><legend>Enabled engines</legend><div id="engine-options" class="engine-options">Loading registered engines…</div><p class="section-sub">No engines is valid; that mode produces observations and events without domain assessments.</p></fieldset>
      <div class="config-actions"><button id="save-config" class="action-btn" type="submit">Save configuration</button><button id="discard-config" class="secondary-btn" type="button">Discard edits</button><span id="config-status" role="status" class="section-sub">No unsaved changes.</span></div>
      <p class="config-plugin-note">Plugins run as this application user and are not sandboxed. Current runtime only enables read-only plugins; policy declarations do not grant access to tools or scripts.</p>
    </form></details>
    <div class="pyramid" role="region" aria-label="Assessment reasoning pyramid">
      <section class="tier tier-diagnosis">
        <div class="tier-label"><span class="tier-number">1</span> Root-cause assessment</div>
        <section class="section" aria-labelledby="diagnosis-heading"><div class="section-head"><div><h3 id="diagnosis-heading">What is driving this assessment?</h3><div class="section-sub">Leading evidence and contributing component states; not a causal claim beyond the available evidence</div></div></div><div id="diagnosis-summary" class="diagnosis-summary">Waiting for assessment evidence…</div><div id="diagnosis-factors" class="diagnosis-factors"></div><p class="diagnosis-footnote">Unknown is a valid result. GREEN requires complete, valid, fresh required evidence; missing or stale evidence is never healthy by default. Recommendations are not authorization, and this prototype's rule-based findings are not proof of root cause.</p></section>
        <section class="section" aria-labelledby="cascade-heading"><div class="section-head"><div><h3 id="cascade-heading">Reasoning cascade (L1 → L6)</h3><div class="section-sub">Named layers L1 Metadata/Data, L2 Information, L3 Determination, L4 Assessment, L5 Incident Report, L6 Mitigation, then back to L5 for resolution/recurrence</div></div></div><div id="cascade-live" class="cascade"><div class="empty">Waiting for events…</div></div></section>
        <section class="section" aria-labelledby="live-incident-heading"><div class="section-head"><div><h3 id="live-incident-heading">Incident reasoning: Why / How</h3><div class="section-sub">Derived from the current assessment only; unknowns stay unknown</div></div></div><div class="chain-tabs" role="tablist"><button type="button" id="livechain-why-tab" class="chain-tab active" role="tab">Why</button><button type="button" id="livechain-how-tab" class="chain-tab" role="tab">How</button></div><div id="livechain-why-panel"><h4>Why chain (backward causality)</h4><ol id="live-why" class="simulation-factors"></ol></div><div id="livechain-how-panel" hidden><h4>How chain (forward remediation)</h4><ol id="live-how" class="simulation-factors"></ol></div><p id="live-chain-note" class="diagnosis-footnote"></p></section>
      </section>
      <section class="tier tier-components">
        <div class="tier-label"><span class="tier-number">3</span> Components</div>
        <section class="section"><div class="section-head"><div><h3>Component health by category</h3><div class="section-sub">Signals grouped by domain with their derived component assessment</div></div></div><div id="components" class="category-stack"><div class="empty">Waiting for assessment data…</div></div></section>
      </section>
      <section class="tier tier-evidence">
        <div class="tier-label"><span class="tier-number">4</span> Source evidence and event record</div>
        <section class="section"><div class="section-head"><div><h3>Metrics and observations</h3><div class="section-sub">Underlying values, unavailable reasons, and observation times</div></div></div><div class="table-wrap"><table><thead><tr><th>Signal</th><th>Value</th><th>Observation</th><th>Observed at</th></tr></thead><tbody id="observations"><tr><td colspan="4" class="empty">Waiting for observations…</td></tr></tbody></table></div></section>
        <section class="section"><div class="section-head"><div><h3>Application logs</h3><div class="section-sub">Runtime diagnostics are emitted separately from the assessment event stream</div></div></div><p class="section-sub">Structured JSON logs are written to stderr (normally collected by the system journal under systemd). Log history is not retained or displayed in this dashboard.</p></section>
        <section class="section"><div class="section-head"><div><h3>Incident evidence limits</h3></div></div><p id="incident-capture-note" class="incident-note">Current-cycle events only. This prototype does not retain the 60 seconds before an incident or track mitigation and its five-minute recovery window.</p><details><summary>Complete event record</summary><pre id="events">Waiting for events…</pre></details></section>
      </section>
    </div>
    </div>
    <section id="security-page" hidden><section class="section"><div class="page-title"><div><div class="header-label">Risk posture</div><h2>Security assessment</h2><div class="section-sub">Security findings are reviewed separately from urgent on-call alerts. No findings does not mean security is cleared.</div></div><div id="security-page-status" class="badge unknown">NOT ASSESSED</div></div><div id="security-page-summary" class="diagnosis-summary">Waiting for security assessment…</div><h3>Security posture findings</h3><div id="security-findings" class="category-stack"><div class="empty">Waiting for security evidence…</div></div></section></section>
    <section id="settings-page" hidden><div class="page-title"><div><div class="header-label">Runtime controls</div><h2>Configuration</h2></div><button id="settings-back" class="secondary-btn" type="button">Back to operations</button></div></section>
    <section id="simulation-page" hidden>
      <section class="section"><div class="section-head"><div><h2>Simulation lab</h2><div class="section-sub">Explore deterministic synthetic conditions and alternate thresholds without affecting the live system.</div></div></div><p class="incident-note">Simulation runs in a fresh built-in-engine runtime using synthetic inputs. It does not read or write host state, run plugins, replace the live report, or authorize actions. Results are clearly labeled synthetic and are not production observations.</p>
        <div class="config-actions"><label for="simulation-scenario">Scenario</label><select id="simulation-scenario"><option value="normal">Normal baseline</option><option value="cpu-warning">Elevated CPU pressure</option><option value="cpu-critical">Critical CPU pressure</option><option value="missing-memory">Missing memory evidence</option><option value="stale-memory">Stale memory evidence</option><option value="security-critical">Security finding: critical</option></select><label for="sim-snapshot"><input id="sim-snapshot" type="checkbox"> Fork from live snapshot (latest live observations; scenario and synthetic measurements ignored, thresholds apply)</label></div>
        <fieldset class="engine-settings"><legend>Synthetic measurements</legend><div class="simulation-inputs"><label>CPU utilization (%)<input id="sim-cpu" type="number" min="0" max="100" step="0.1" value="20"></label><label>Memory used (%)<input id="sim-memory" type="number" min="0" max="100" step="0.1" value="40"></label><label>Load per CPU<input id="sim-load" type="number" min="0" step="0.1" value="0.5"></label><label>Filesystem used (%)<input id="sim-filesystem" type="number" min="0" max="100" step="0.1" value="50"></label></div></fieldset>
        <fieldset class="engine-settings"><legend>Simulation thresholds (warning / critical)</legend><div class="simulation-inputs"><label>CPU (%)<input id="sim-cpu-warning" type="number" min="0" max="100" step="0.1" value="85"><input id="sim-cpu-critical" type="number" min="0" max="100" step="0.1" value="95"></label><label>Memory (%)<input id="sim-memory-warning" type="number" min="0" max="100" step="0.1" value="85"><input id="sim-memory-critical" type="number" min="0" max="100" step="0.1" value="95"></label><label>Filesystem (%)<input id="sim-filesystem-warning" type="number" min="0" max="100" step="0.1" value="85"><input id="sim-filesystem-critical" type="number" min="0" max="100" step="0.1" value="95"></label><label>Load per CPU<input id="sim-load-warning" type="number" min="0" step="0.1" value="1.5"><input id="sim-load-critical" type="number" min="0" step="0.1" value="3"></label></div></fieldset>
        <div class="config-actions"><button id="run-simulation" class="action-btn" type="button">Run isolated simulation</button><span id="simulation-status" class="section-sub" role="status">No simulation has been run.</span></div>
      </section>
      <div id="simulation-output" class="simulation-output" hidden>
        <section class="section"><div class="section-head"><div><h3 id="simulation-incident-title">Synthetic incident report</h3><div id="simulation-incident-status" class="section-sub"></div></div></div><p id="simulation-incident-summary" class="diagnosis-summary"></p><h4>Contributing factors</h4><ul id="simulation-factors" class="simulation-factors"></ul><h4>Missing evidence</h4><ul id="simulation-missing" class="simulation-factors"></ul><h4>Suggested response</h4><ul id="simulation-recommendations" class="simulation-factors"></ul><div class="chain-tabs" role="tablist"><button type="button" id="chain-why-tab" class="chain-tab active" role="tab">Why</button><button type="button" id="chain-how-tab" class="chain-tab" role="tab">How</button></div><div id="chain-why-panel"><h4>Why chain (backward causality)</h4><ol id="simulation-why" class="simulation-factors"></ol></div><div id="chain-how-panel" hidden><h4>How chain (forward remediation)</h4><ol id="simulation-how" class="simulation-factors"></ol></div><p id="simulation-chain-note" class="diagnosis-footnote"></p></section>
        <section class="section"><div class="section-head"><div><h3>All synthetic events</h3><div id="simulation-event-meta" class="section-sub"></div></div></div><div id="simulation-events" class="simulation-events"></div></section>
        <section class="section"><div class="section-head"><div><h3>Synthetic reasoning cascade</h3><div class="section-sub">Same derivation-depth view as the live page, for this simulation only</div></div></div><div id="cascade-sim" class="cascade"></div></section>
        <section class="section"><div class="section-head"><div><h3>Complete simulation report</h3><div class="section-sub">Includes inputs, incident summary, assessments, runtime metrics, and event payloads.</div></div></div><pre id="simulation-report" class="simulation-report"></pre></section>
      </div>
    </section>
    <section id="store-page" hidden>
      <section class="section"><div class="section-head"><div><h2>Engine Store</h2><div class="section-sub">All enabled reasoning modules, including Perceptron-maintained defaults, are catalogued here. Mission aggregation, event transport, policy enforcement, and dashboard services remain core responsibilities.</div></div></div><p class="section-sub">Enable or disable a read-only engine here. Configure per-engine settings in Operations → Runtime configuration. Policy declarations describe intent; they are not grants.</p><div id="store-status" class="section-sub" role="status">Loading the local engine catalog…</div><div id="store-entries" class="store-grid"></div></section>
      <section class="section" aria-labelledby="core-capabilities-heading"><div class="section-head"><div><h2 id="core-capabilities-heading">Available core capabilities</h2><div class="section-sub">Security posture assessment and synthetic scenario simulation are available in this build with the limitations shown here.</div></div></div><div class="store-grid">
        <article class="store-card" data-capability="simulation"><h3>Simulation lab</h3><span class="planned-status">Available · synthetic only</span><p>Explore deterministic baseline, pressure, stale/missing evidence, and security scenarios with adjustable measurements and thresholds. Full synthetic incident reports and event lineage are separate from live operations.</p><div class="store-meta">Type: core platform capability · Open the Simulation page</div></article>
        <article class="store-card" data-capability="security-posture"><h3>Security posture engine</h3><span class="planned-status">Available · evidence-driven</span><p>Enable <code>security-posture</code> in the local engine catalog and provide current security evidence under Operations configuration. It assesses the supplied states, provenance, and freshness; enabled findings qualify mission status but do not alter host component health or authorize remediation.</p><div class="store-meta">Type: bundled domain engine · Requires supplied evidence</div></article>
      </div></section>
      <section class="section"><h3>Contributing and installing engines</h3><p class="section-sub"><strong>Locally crafted:</strong> place a reviewed package under <code>main/external/&lt;engine-name&gt;</code>, document its manifest, versioned event schemas, source, settings, resource needs, and policy declaration, then add it to the local plugin configuration. Submit the package and protocol tests through the project's contribution process.</p><p class="section-sub"><strong>Third party:</strong> install only a versioned package from a source you trust, verify it independently, and declare it locally. This build has no remote marketplace, signature verification, automatic download, or sandbox. Untrusted executables must not be enabled.</p><p class="config-plugin-note">Policy metadata is declarative and is not a security sandbox: enabled external programs run with this application's OS privileges. Self-mitigation, tool, and script access remain disabled in core until an independently enforced broker exists.</p></section>
    </section>
    <footer class="footer"><span>Observe · Reason · Explain</span><span id="updated">No report received</span></footer>
  </main>
  <script>
    const byId = id => document.getElementById(id);
    const clear = node => { while (node.firstChild) node.removeChild(node.firstChild); };
    const text = (tag, className, value) => { const node = document.createElement(tag); if (className) node.className = className; node.textContent = value == null ? "—" : String(value); return node; };
    const stateClass = value => { const state = String(value || "unknown").toLowerCase(); return ["green","yellow","red","unknown","healthy","degraded","critical"].includes(state) ? state : "unknown"; };
    const localTime = value => { const date = new Date(value); return Number.isNaN(date.getTime()) ? "unknown time" : date.toLocaleString(); };
    function stat(label, value, note) { const card = text("article","stat",""); card.append(text("div","stat-label",label),text("div","stat-value",value),text("div","stat-note",note)); return card; }
    let configurationDirty = false;
    let currentConfiguration = null;
    let currentCatalog = [];
    let simulationDefaultsLoaded = false;
    const builtInEngines = [{name:"host-pressure",description:"Pressure thresholds"},{name:"host-health",description:"Component health"},{name:"security-posture",description:"Assess manually supplied, time-bounded security evidence"}];
    function setConfigStatus(message,isError) { const status=byId("config-status");status.textContent=message;status.className="section-sub"+(isError?" config-error":""); }
    function renderConfiguration(configuration) {
      if (!configuration || configurationDirty) return;
      currentConfiguration = JSON.parse(JSON.stringify(configuration));
      byId("cfg-host").value=configuration.host_id||"";
      byId("cfg-dns-name").value=configuration.dns_probe_name||"";
      byId("cfg-cpu-interval").value=configuration.cpu_sample_interval||"";
      byId("cfg-max-age").value=configuration.max_observation_age||"";
      byId("cfg-dns-timeout").value=configuration.dns_probe_timeout||"";
      byId("cfg-filesystems").value=(configuration.filesystem_paths||[]).join("\n");
      byId("security-required-checks").value=(configuration.security_required_checks||[]).join("\n");
      byId("security-evidence").value=JSON.stringify(configuration.security_evidence||[],null,2);
      const thresholdFields={cpu:["cpu_percent",configuration.thresholds.cpu_percent],memory:["memory_percent",configuration.thresholds.memory_percent],filesystem:["filesystem_percent",configuration.thresholds.filesystem_percent],load:["load_per_cpu",configuration.thresholds.load_per_cpu]};
      Object.keys(thresholdFields).forEach(key=>{const field=thresholdFields[key][1]||{};byId("threshold-"+key+"-warning").value=field.warning;byId("threshold-"+key+"-critical").value=field.critical;});
      const plugins=configuration.plugins||[];
      const available=builtInEngines.concat(plugins.map(plugin=>({name:plugin.name,description:"Plugin "+plugin.version+" · "+plugin.role})));
      clear(byId("engine-options"));
      available.sort((left,right)=>{const leftIndex=(configuration.engines||[]).indexOf(left.name);const rightIndex=(configuration.engines||[]).indexOf(right.name);return (leftIndex<0?Number.MAX_SAFE_INTEGER:leftIndex)-(rightIndex<0?Number.MAX_SAFE_INTEGER:rightIndex);});
      available.forEach(item=>{const label=text("div","engine-option","");const checkbox=document.createElement("input");checkbox.type="checkbox";checkbox.dataset.engine=item.name;checkbox.checked=(configuration.engines||[]).includes(item.name);const copy=text("span","",item.name);copy.append(text("small","",item.description));const up=text("button","engine-move","↑");up.type="button";up.setAttribute("aria-label","Move "+item.name+" earlier");up.addEventListener("click",()=>{const previous=label.previousElementSibling;if(previous){label.parentNode.insertBefore(label,previous);configurationDirty=true;setConfigStatus("Unsaved engine order change.",false);}});const down=text("button","engine-move","↓");down.type="button";down.setAttribute("aria-label","Move "+item.name+" later");down.addEventListener("click",()=>{const next=label.nextElementSibling;if(next){label.parentNode.insertBefore(next,label);configurationDirty=true;setConfigStatus("Unsaved engine order change.",false);}});label.append(checkbox,copy,up,down);byId("engine-options").append(label);});
      setConfigStatus("Configuration loaded from "+(configuration.config_path||"the runtime store")+".",false);
      const settings=byId("plugin-settings");clear(settings);
      plugins.forEach(plugin=>{const label=text("label","plugin-setting","");label.append(text("strong","",plugin.name+" settings (JSON)"));const area=document.createElement("textarea");area.rows=3;area.dataset.plugin=plugin.name;area.value=JSON.stringify(plugin.settings||{},null,2);label.append(area);settings.append(label);});
    }
    function configurationFromForm() {
      const next=JSON.parse(JSON.stringify(currentConfiguration||{}));
      next.host_id=byId("cfg-host").value.trim();
      next.dns_probe_name=byId("cfg-dns-name").value.trim();
      next.dns_probe_timeout=byId("cfg-dns-timeout").value.trim();
      next.cpu_sample_interval=byId("cfg-cpu-interval").value.trim();
      next.max_observation_age=byId("cfg-max-age").value.trim();
      next.filesystem_paths=byId("cfg-filesystems").value.split(/\r?\n/).map(path=>path.trim()).filter(Boolean);
      next.security_required_checks=byId("security-required-checks").value.split(/\r?\n/).map(check=>check.trim()).filter(Boolean);
      next.security_evidence=JSON.parse(byId("security-evidence").value||"[]");
      const thresholdKeys={cpu:"cpu_percent",memory:"memory_percent",filesystem:"filesystem_percent",load:"load_per_cpu"};
      Object.keys(thresholdKeys).forEach(key=>{next.thresholds[thresholdKeys[key]]={warning:Number(byId("threshold-"+key+"-warning").value),critical:Number(byId("threshold-"+key+"-critical").value)};});
      (next.plugins||[]).forEach(plugin=>{const editor=[...byId("plugin-settings").querySelectorAll("textarea")].find(item=>item.dataset.plugin===plugin.name);if(editor)plugin.settings=JSON.parse(editor.value);});
      next.engines=[...byId("engine-options").querySelectorAll(".engine-option")].filter(option=>option.querySelector("input[data-engine]").checked).map(option=>option.querySelector("input[data-engine]").dataset.engine);
      return next;
    }
    async function loadCatalog() {
      const response=await fetch("/api/engines",{cache:"no-store"});
      if(!response.ok)throw new Error("Engine catalog request failed ("+response.status+")");
      currentCatalog=await response.json();const container=byId("store-entries");clear(container);
      currentCatalog.forEach(entry=>{
        const card=text("article","store-card","");card.append(text("h3","",entry.name+" · "+entry.version));
        card.append(text("p","",(entry.category||"uncategorized")+" / "+(entry.role||"engine")+" · "+(entry.source||"local")));
        card.append(text("div","store-policy","Policy: "+((entry.policy&&entry.policy.mode)||"undeclared")+" · read-only="+String(Boolean(entry.policy&&entry.policy.read_only))+" · tools="+JSON.stringify((entry.policy&&entry.policy.allowed_tools)||[])+" · scripts="+JSON.stringify((entry.policy&&entry.policy.allowed_scripts)||[])));
        card.append(text("div","store-meta","Consumes: "+(entry.consumes||[]).join(", ")+" · Produces: "+(entry.produces||[]).join(", ")));
        const button=text("button","action-btn",entry.enabled?"Disable":"Enable");button.type="button";button.disabled=Boolean(entry.source==="local"&&entry.policy&&(entry.policy.mode==="self_mitigation"||!entry.policy.read_only||(entry.policy.allowed_tools||[]).length||(entry.policy.allowed_scripts||[]).length));
        if(button.disabled)button.title="Current runtime accepts read-only engines only.";
        button.addEventListener("click",async()=>{button.disabled=true;try{const next=JSON.parse(JSON.stringify(currentConfiguration||{}));const names=next.engines||[];next.engines=entry.enabled?names.filter(name=>name!==entry.name):[...new Set(names.concat(entry.name))];const saved=await fetch("/api/config",{method:"PUT",headers:{"Content-Type":"application/json","X-Requested-With":"PerceptronDashboard"},body:JSON.stringify(next),cache:"no-store"});if(!saved.ok)throw new Error((await saved.text())||"Engine selection save failed ("+saved.status+")");currentConfiguration=await saved.json();configurationDirty=false;renderConfiguration(currentConfiguration);await loadCatalog();byId("store-status").textContent="Engine selection saved; it applies on the next assessment cycle.";}catch(error){byId("store-status").textContent=error.message||"Engine selection failed.";button.disabled=false;}});
        card.append(button);container.append(card);
      });
      byId("store-status").textContent=currentCatalog.length+" catalog engines. No remote packages are downloaded.";
    }
    byId("settings-page").append(byId("configuration-panel"));
    function showPage(page) {
      ["operations","security","simulation","store","settings"].forEach(name=>{byId(name+"-page").hidden=name!==page;});
      document.querySelectorAll(".page-nav button").forEach(button=>button.removeAttribute("aria-current"));
      const activeTab=page==="settings"?"operations":page;
      const tab=byId(activeTab+"-tab");
      if(tab)tab.setAttribute("aria-current","page");
      if(page==="store")loadCatalog().catch(error=>byId("store-status").textContent=error.message||"Engine catalog unavailable.");
    }
    byId("operations-tab").addEventListener("click",()=>showPage("operations"));
    byId("security-tab").addEventListener("click",()=>showPage("security"));
    byId("simulation-tab").addEventListener("click",()=>showPage("simulation"));
    byId("store-tab").addEventListener("click",()=>showPage("store"));
    bindChainTabs("chain");bindChainTabs("livechain");
    byId("settings-tab").addEventListener("click",()=>showPage("settings"));
    byId("settings-back").addEventListener("click",()=>showPage("operations"));
    byId("alert-security").addEventListener("click",()=>showPage("security"));
    byId("configuration-form").addEventListener("input",()=>{configurationDirty=true;setConfigStatus("Unsaved changes.",false);});
    byId("configuration-form").addEventListener("change",()=>{configurationDirty=true;setConfigStatus("Unsaved changes.",false);});
    byId("discard-config").addEventListener("click",()=>{configurationDirty=false;renderConfiguration(currentConfiguration);});
    byId("configuration-form").addEventListener("submit",async event=>{
      event.preventDefault();
      const button=byId("save-config");button.disabled=true;setConfigStatus("Validating and saving configuration…",false);
      try {
        const response=await fetch("/api/config",{method:"PUT",headers:{"Content-Type":"application/json","X-Requested-With":"PerceptronDashboard"},body:JSON.stringify(configurationFromForm()),cache:"no-store"});
        if(!response.ok)throw new Error((await response.text())||"Configuration save failed ("+response.status+")");
        const saved=await response.json();configurationDirty=false;renderConfiguration(saved);setConfigStatus("Saved. New values and engine selection apply on the next assessment cycle.",false);
      } catch(error) { setConfigStatus(error.message||"Configuration save failed.",true); }
      finally { button.disabled=false; }
    });
    const componentLabels = {cpu:"CPU",memory:"Memory",load:"System load",filesystem:"Filesystems",network:"Network",process:"Processes",dns:"DNS resolution"};
    const signalLabels = {"cpu.utilization_percent":"CPU utilization","memory.used_percent":"Memory used","system.load_1m":"1-minute load","filesystem.used_percent":"Filesystem used","network.rx_bytes":"Network received","network.tx_bytes":"Network transmitted","process.count":"Process count","dns.resolution_success":"DNS resolution"};
    function hostComponent(item, hostID) { return item.component + "|" + (item.scope_id || hostID || ""); }
    function categoryFor(component) { return component === "dns" ? "DNS services" : "Host metrics"; }
    function measurementValue(item) {
      if (!item.available || item.value == null) return "UNKNOWN";
      if (item.unit === "boolean") return Number(item.value) === 1 ? "RESOLVED" : "FAILED";
      return Number(item.value).toFixed(2) + " " + (item.unit || "");
    }
    function categoryStatus(items) {
      const states = items.map(item => item.health && item.health.state).filter(Boolean);
      if (states.includes("CRITICAL")) return "CRITICAL";
      if (states.includes("DEGRADED")) return "DEGRADED";
      if (states.includes("UNKNOWN")) return "UNKNOWN";
      if (states.length) return "HEALTHY";
      return "NOT ASSESSED";
    }
    function renderCategories(data, mission, events) {
      const observations = events.filter(event => event.type === "observation.host.v1").map(event => ({event,value:event.payload || {}}));
      const healthByKey = new Map();
      (mission.components || []).forEach(item => healthByKey.set(hostComponent(item,data.host_id),item));
      const groups = new Map();
      observations.forEach(entry => {
        const value = entry.value;
        const component = value.component || "unknown";
        const scopeID = value.scope_id || (component === "dns" ? value.target : data.host_id);
        const key = component + "|" + (scopeID || "");
        if (!groups.has(key)) groups.set(key,{component,scopeID,observations:[],health:healthByKey.get(key)});
        groups.get(key).observations.push(entry);
      });
      (mission.components || []).forEach(health => {
        const key = hostComponent(health,data.host_id);
        if (!groups.has(key)) groups.set(key,{component:health.component,scopeID:health.scope_id,observations:[],health});
      });
      const categories = new Map();
      groups.forEach(group => {
        const category = categoryFor(group.component);
        if (!categories.has(category)) categories.set(category,[]);
        categories.get(category).push(group);
      });
      clear(byId("components"));
      if (!categories.size) { byId("components").append(text("div","empty","No component evidence is available.")); return; }
      categories.forEach((items,name) => {
        const section = text("section","category","");
        const head = text("div","category-head","");
        const title = text("div","","");
        title.append(text("div","category-title",name),text("div","category-note",name === "Host metrics" ? "CPU · memory · load · storage · network · processes" : "Configured name-resolution probes"));
        head.append(title,text("span","state " + stateClass(categoryStatus(items)),categoryStatus(items)));
        section.append(head);
        const cards = text("div","category-items","");
        items.forEach(group => {
          const card = text("article","metric-card","");
          const head = text("div","metric-card-head","");
          head.append(text("div","metric-name",(componentLabels[group.component] || group.component) + (group.scopeID && group.scopeID !== data.host_id ? " · " + group.scopeID : "")));
          const health = group.health;
          head.append(text("span","state " + stateClass(health ? health.state : "unknown"),health ? health.state : "NOT ASSESSED"));
          card.append(head);
          group.observations.forEach(({event,value}) => {
            const title = text("div","metric-detail",signalLabels[value.metric] || value.metric || "Signal");
            const shown = text("div","metric-value " + (value.available ? "" : "unknown"),measurementValue(value));
            const detail = value.available ? (value.target ? "Target: " + value.target : "Observed " + localTime(value.observed_at || event.occurred_at)) : (value.unavailable_reason || "Unavailable; no reason supplied");
            card.append(title,shown,text("div","metric-detail",detail));
            if (value.latency_millis != null) card.append(text("div","metric-detail","Resolution latency: " + Number(value.latency_millis).toFixed(2) + " ms"));
          });
          if (health) {
            const meta = text("div","meta","");
            meta.append(text("span","","Evidence: " + (health.coverage || "unknown")),text("span","","Freshness: " + (health.freshness || "unknown")));
            card.append(text("div","metric-detail",health.explanation || "No component explanation supplied."),meta);
          } else if (!group.observations.length) {
            card.append(text("div","metric-detail","No source observation is available for this component assessment."));
          } else {
            card.append(text("div","metric-detail","No domain-health rule is currently defined for this signal."));
          }
          cards.append(card);
        });
        section.append(cards);
        byId("components").append(section);
      });
    }
    function renderEngines(metrics, target) {
      clear(target);
      const engines = metrics.engines || {};
      const coreServices = metrics.core_services || {};
      const entries = Object.keys(engines).map(name=>({name,item:engines[name],core:false})).concat(Object.keys(coreServices).map(name=>({name,item:coreServices[name],core:true}))).sort((left,right)=>left.name.localeCompare(right.name));
      if (!entries.length) { target.append(text("div","empty","No engine executions are recorded.")); return "NONE RECORDED"; }
      const available=entries.filter(entry=>Number(entry.item.runs||0)>0&&Number(entry.item.failures||0)===0).length;
      entries.forEach(entry => {
        const item = entry.item;
        const failed = Number(item.failures || 0) > 0;
        const status = failed ? "FAILED" : (Number(item.runs || 0) > 0 ? "AVAILABLE" : "IDLE");
        const card = text("div","engine-chip","");
        card.append(text("strong","",(entry.core?"Core · ":"")+entry.name),text("span","",status + " · " + String(item.duration_millis || 0) + " ms"));
        target.append(card);
      });
      return available+" / "+entries.length+" available";
    }
    function engineStat(metrics) {
      const card=document.createElement("details");
      card.className="stat engine-stat";
      const summary=document.createElement("summary");
      summary.append(text("div","stat-label","Reasoning engines"),text("div","stat-value","Expand availability"));
      const list=text("div","engine-list","");
      card.append(summary,list);
      summary.querySelector(".stat-value").textContent=renderEngines(metrics,list);
      return card;
    }
    function renderDiagnosis(mission) {
      const components = (mission.components || []).slice();
      const security = (mission.security_findings || []).slice();
      const rank = {CRITICAL:0,DEGRADED:1,UNKNOWN:2,HEALTHY:3};
      components.sort((left,right) => (rank[left.state] == null ? 4 : rank[left.state]) - (rank[right.state] == null ? 4 : rank[right.state]));
      const leading = components.filter(item => item.state === "CRITICAL" || item.state === "DEGRADED");
      const securityLeading = security.filter(item => item.state === "CRITICAL" || item.state === "WARNING");
      const unknown = components.filter(item => item.state === "UNKNOWN").map(item => item.component).concat(security.filter(item => item.state === "UNKNOWN").map(item => item.check));
      byId("diagnosis-summary").textContent = leading.length
        ? "Leading assessed contributors: " + leading.slice(0,3).map(item => item.component + " (" + item.state.toLowerCase() + ")").join(", ") + (securityLeading.length ? "; security: " + securityLeading.slice(0,3).map(item => item.check + " (" + item.state.toLowerCase() + ")").join(", ") : "") + ". " + (mission.explanation || "Review the component evidence below.")
        : (securityLeading.length
          ? "Security findings contributing to this assessment: " + securityLeading.slice(0,3).map(item => item.check + " (" + item.state.toLowerCase() + ")").join(", ") + ". " + (mission.explanation || "Review the finding evidence below.")
          : (unknown.length
            ? "No confirmed pressure contributor is available. Evidence is unknown for " + unknown.slice(0,3).join(", ") + ". " + (mission.explanation || "Do not infer health from missing evidence.")
            : (mission.status === "UNKNOWN"
              ? "The mission cannot be assessed as healthy because required evidence is incomplete or uncertain. " + (mission.explanation || "Review coverage and freshness.")
              : "No elevated host or security risk is reported in this cycle. " + (mission.explanation || "Review the supporting evidence below."))));
      clear(byId("diagnosis-factors"));
      const factors = components.slice(0,4);
      if (!factors.length) byId("diagnosis-factors").append(text("div","empty","No component assessments are available."));
      factors.forEach(item => {
        const factor = text("article","diagnosis-factor","");
        factor.append(text("strong","",item.component + " · " + item.state),text("p","",item.explanation || "No component explanation supplied."));
        byId("diagnosis-factors").append(factor);
      });
    }
    function renderSecurityFindings(events) {
      const container=byId("security-findings");clear(container);
      const findings=events.filter(event=>event.type==="security.finding.v1").map(event=>({event,finding:event.payload||{}}));
      if(!findings.length){container.append(text("div","empty","No security findings are available. This is not a security clearance; enable security-posture and supply evidence."));return;}
      findings.forEach(({event,finding})=>{
        const card=text("article","metric-card","");
        const head=text("div","metric-card-head","");
        head.append(text("div","metric-name",finding.check||"Unnamed check"),text("span","state",finding.state||"UNKNOWN"));
        card.append(head);
        card.append(text("div","metric-detail",finding.explanation||finding.summary||"No explanation supplied."));
        card.append(text("div","metric-detail","Source: "+(finding.source||"unknown")+" · Freshness: "+(finding.freshness||"unknown")+" · Observed: "+localTime(finding.observed_at)));
        card.append(text("code","rule-id","Event_ID: "+(event.event_id||"unknown id")));
        container.append(card);
      });
    }
    function renderRecommendation(recommendation, activity) {
      byId("recommendation-title").textContent = recommendation.title || "No recommendation";
      byId("recommendation-rationale").textContent = recommendation.rationale || "No rationale supplied.";
      byId("recommendation-rule").textContent = "Rule: " + (recommendation.rule_id || "unspecified");
      byId("recommendation-reason").textContent = recommendation.tool_description || "No tool description supplied.";
      clear(byId("recommendation-evidence"));
      const ids = recommendation.evidence_event_ids || [];
      if (!ids.length) byId("recommendation-evidence").append(text("div","evidence-item","No supporting event references."));
      ids.forEach(id => {
        const link = text("button","evidence-item evidence-link","↳ Event_ID: " + id);
        link.type = "button";
        link.addEventListener("click",() => {
          const target = Array.from(document.querySelectorAll("[data-event-id]")).find(item=>item.dataset.eventId===id);
          if (target) { target.scrollIntoView({behavior:"smooth",block:"center"}); target.classList.add("evidence-highlight"); window.setTimeout(() => target.classList.remove("evidence-highlight"),1800); }
        });
        byId("recommendation-evidence").append(link);
      });
      const button = byId("refresh-tool");
      button.disabled = !!(activity && (activity.status === "queued" || activity.status === "running"));
      button.textContent = activity && activity.status === "running" ? "Refresh running…" : (activity && activity.status === "queued" ? "Refresh queued…" : "Run read-only refresh");
      const status = activity ? activity.status : "idle";
      const message = activity ? activity.message : "No dashboard-triggered tool run yet.";
      byId("tool-activity").className = "tool-status " + status;
      byId("tool-activity").textContent = activity ? status.toUpperCase() + " · " + message + (activity.cycle_id ? " Cycle: " + activity.cycle_id : "") : message;
    }
    function renderTimeline(events) {
      clear(byId("event-list"));
      if (!events.length) { byId("event-list").append(text("div","empty","No events are available.")); return; }
      const names = {"assessment.cycle.started.v1":"Cycle started","observation.host.v1":"Host observation","pressure.host.v1":"Pressure assessment","health.host.v1":"Component health","mission.host.v1":"Mission assessment"};
      events.slice().sort((left,right) => new Date(right.occurred_at) - new Date(left.occurred_at)).forEach(event => {
        const item = document.createElement("details");
        item.className="event-node";
        item.dataset.eventId = event.event_id || "";
        const summary=document.createElement("summary");
        summary.append(text("time","timeline-time",localTime(event.occurred_at)),text("strong","",names[event.type]||event.type),text("span","",(event.source||"unknown source")+" · origin: "+(event.origin||"unknown")),text("code","","Event_ID: "+(event.event_id||"unknown id")));
        item.append(summary,text("div","event-provenance","Emitted: "+localTime(event.emitted_at||event.occurred_at)+" · Derives from: "+((event.derives_from||[]).map(id=>"Event_ID: "+id).join(", ")||"none")),text("pre","",JSON.stringify(event.payload==null?{}:event.payload,null,2)));
        byId("event-list").append(item);
      });
    }
    function renderChains(incident, whyId, howId, noteId) {
      [[whyId,incident.why_chain],[howId,incident.how_chain]].forEach(([id,links])=>{
        const list=byId(id);clear(list);
        if(!links||!links.length){list.append(text("li","","None reported."));return;}
        links.forEach(link=>list.append(text("li","",link.statement+" ["+String(link.status||"unknown").toUpperCase()+(link.class?" · "+link.class:"")+" · origin: "+(link.origin||"unknown")+(link.evidence&&link.evidence.length?" · evidence: "+link.evidence.join(", "):"")+"]")));
      });
      byId(noteId).textContent=incident.chain_note||"";
    }
    function renderCascade(events, targetId, incident) {
      const target=byId(targetId);clear(target);
      if(!events||!events.length){target.append(text("div","empty","No events available."));return;}
      const layerOf={"observation.host.v1":1,"observation.security.v1":1,"assessment.cycle.started.v1":1,"pressure.host.v1":2,"health.host.v1":3,"security.finding.v1":3,"mission.host.v1":4};
      const names={1:"L1 · Metadata / Data",2:"L2 · Information",3:"L3 · Determination",4:"L4 · Assessment",0:"Unmapped event types"};
      const layers={};events.forEach(event=>{const d=layerOf[event.type]||0;(layers[d]=layers[d]||[]).push(event);});
      const addColumn=(title,lines)=>{
        if(target.children.length)target.append(text("div","cascade-arrow","→"));
        const column=text("div","cascade-layer","");column.append(text("h4","",title));
        lines.forEach(line=>column.append(text("div","cascade-item",line)));target.append(column);
      };
      [1,2,3,4,0].forEach(d=>{
        if(!layers[d]){if(d)addColumn(names[d],["No events at this layer."]);return;}
        const counts={};
        layers[d].forEach(event=>{const key=event.source+" → "+event.type+" ["+event.origin+"]";counts[key]=(counts[key]||0)+1;});
        addColumn(names[d],Object.keys(counts).map(key=>key+(counts[key]>1?" ×"+counts[key]:"")));
      });
      const inc=incident||{};
      addColumn("L5 · Incident Report",[(inc.title||"Not assessed")+" · "+(inc.status||"UNKNOWN"),"Why / How chains: see Incident reasoning"]);
      addColumn("L6 · Mitigation",["Recommendation only; no mitigation executed or recorded","Not authorization"]);
      addColumn("↺ L5 · Resolution or recurrence",["UNKNOWN: not tracked in this build"]);
    }
    function bindChainTabs(prefix) {
      [["why","how"],["how","why"]].forEach(([on,off])=>byId(prefix+"-"+on+"-tab").addEventListener("click",()=>{byId(prefix+"-"+on+"-panel").hidden=false;byId(prefix+"-"+off+"-panel").hidden=true;byId(prefix+"-"+on+"-tab").classList.add("active");byId(prefix+"-"+off+"-tab").classList.remove("active");}));
    }
    function render(data) {
      renderCascade((data.runtime||{}).events||[],"cascade-live",data.live_incident);
      renderChains(data.live_incident||{},"live-why","live-how","live-chain-note");
      const runtime = data.runtime || {};
      const mission = runtime.mission || {scope:{type:"host",id:data.host_id},objective:"Core mission assessment unavailable",status:"UNKNOWN",disposition:"unable_to_assess",explanation:"The core is collecting observations, but its mission assessment is unavailable.",components:[],coverage:{assessed_components:[],required_components:[],complete:false},freshness:"not_assessed"};
      const status = mission.status || "UNKNOWN";
      const badge = byId("mission-badge"); badge.textContent = status; badge.className = "badge " + stateClass(status);
      byId("scope").textContent = (mission.scope && mission.scope.type ? mission.scope.type : "mission") + " · " + (mission.scope && mission.scope.id ? mission.scope.id : data.host_id);
      byId("objective").textContent = mission.objective || "Mission objective unavailable";
      byId("mission-headline").textContent = missionHeadline(mission);
      renderMissionFactors(mission);
      const metrics = runtime.metrics || {};
      renderConfiguration(data.configuration);
      renderSimulationDefaults(data.configuration);
      clear(byId("stats"));
      const events = runtime.events || [];
      const securityFindings=mission.security_findings||[];
      const securityAssessed=(data.configuration&&data.configuration.engines||[]).includes("security-posture");
      const securityStatus=securityFindings.some(item=>item.state==="CRITICAL"||item.state==="WARNING")?"REVIEW":(securityFindings.length?"ASSESSED":"NOT ASSESSED");
      byId("alert-security").textContent="Security posture: "+securityStatus+" · view assessment";
      byId("security-page-status").textContent=securityStatus;
      byId("security-page-status").className="badge "+(securityStatus==="REVIEW"?"yellow":"unknown");
      byId("security-page-summary").textContent=securityFindings.length
        ? securityFindings.length+" assessed finding(s) are shown below. Review evidence and source freshness; this summary is not an independent security verification."
        : securityAssessed?"Security posture is enabled, but this cycle contains no assessed findings. Missing evidence is not a clearance."
        :"The security-posture engine is not enabled and no security findings are available. This is not a security clearance.";
      byId("stats").append(
        stat("Mission status",status,mission.disposition || "No disposition"),
        stat("Active incidents", status === "RED" ? "1 critical" : (status === "YELLOW" ? "1 elevated" : "0"), status === "UNKNOWN" ? "Assessment incomplete" : "Derived from current mission status"),
        stat("Assessment coverage", String((mission.coverage && mission.coverage.assessed_components || []).length) + " / " + String((mission.coverage && mission.coverage.required_components || []).length), mission.coverage && mission.coverage.complete ? "All required scopes represented" : "Coverage incomplete"),
        stat("Confidence", "Not modeled", "Deterministic rules currently report evidence, coverage, and freshness instead"),
        stat("Evidence freshness", mission.freshness || "unknown", "Freshness is distinct from coverage"),
        stat("Observations", String(data.available_observations || 0) + " available", String(data.unavailable_observations || 0) + " unavailable"),
        engineStat(metrics)
      )
      const securityCard=stat("Security posture",securityStatus,securityFindings.length?securityFindings.length+" finding(s) · review separately":"Not assessed · not a clearance");
      securityCard.classList.add("stat-link");
      securityCard.tabIndex=0;
      securityCard.setAttribute("role","button");
      securityCard.addEventListener("click",()=>showPage("security"));
      securityCard.addEventListener("keydown",event=>{if(event.key==="Enter"||event.key===" "){event.preventDefault();showPage("security");}});
      byId("stats").append(securityCard);
      renderCategories(data,mission,events);
      renderDiagnosis(mission);
      renderSecurityFindings(events);
      renderRecommendation(data.recommendation || {},data.tool_activity || null);
      renderTimeline(events);
      const observations = events.filter(event => event.type === "observation.host.v1").map(event => ({event, value:event.payload || {}}));
      clear(byId("observations"));
      if (!observations.length) { const row = document.createElement("tr"); const cell = text("td","empty","No observations are available."); cell.colSpan = 4; row.append(cell); byId("observations").append(row); }
      observations.forEach(({event,value}) => {
        const row = document.createElement("tr");
        const val = value.available ? (value.value == null ? "—" : Number(value.value).toFixed(2) + " " + (value.unit || "")) : "UNKNOWN";
        row.append(text("td","metric",value.metric || "unknown signal"),text("td","value",val),text("td","reason",value.available ? (value.target ? "Target: " + value.target : "Available") : (value.unavailable_reason || "Unavailable; reason not supplied")),text("td","",localTime(value.observed_at || event.occurred_at)));
        byId("observations").append(row);
      });
      byId("provenance").textContent = "Cycle " + (runtime.cycle_id || "unknown") + " · " + String(events.length) + " events · " + String(metrics.duration_millis || 0) + " ms · expandable evidence";
      byId("events").textContent = JSON.stringify(events.map(event => ({Event_ID:"Event_ID: " + (event.event_id || "unknown id"),type:event.type,source:event.source,origin:event.origin,producer_version:event.producer_version,configuration_id:event.configuration_id,occurred_at:event.occurred_at,derives_from:(event.derives_from || []).map(id => "Event_ID: " + id),payload:event.payload})),null,2);
      byId("updated").textContent = "Report evaluated " + localTime(runtime.completed_at || mission.evaluated_at);
      byId("connection").textContent = "Live · refreshes every 3 seconds";
      byId("connection-dot").className = "dot";
    }
    function missionHeadline(mission) {
      switch(mission.status) {
      case "GREEN": return "Required evidence is within configured criteria";
      case "YELLOW": return "Elevated findings need attention";
      case "RED": return "Critical findings require review";
      default: return "Mission health cannot be confirmed";
      }
    }
    function renderMissionFactors(mission) {
      const list=byId("mission-factors");clear(list);
      const factors=[];
      (mission.components||[]).forEach(item=>{
        if(item.state!=="HEALTHY")factors.push((item.component||"component")+" · "+(item.state||"UNKNOWN")+": "+(item.explanation||"No explanation supplied."));
      });
      (mission.security_findings||[]).forEach(item=>{
        if(item.state!=="HEALTHY")factors.push("Security · "+(item.check||"check")+" · "+(item.state||"UNKNOWN")+": "+(item.explanation||"No explanation supplied."));
      });
      const missing=mission.coverage&&mission.coverage.missing_components||[];
      missing.forEach(item=>factors.push("Missing required evidence · "+item));
      if(!factors.length)factors.push(mission.status==="GREEN"?"No adverse contributors; required evidence is covered and within criteria.":"No adverse factor was isolated from the available evidence.");
      factors.slice(0,6).forEach(factor=>list.append(text("li","",factor)));
    }
    function renderSimulationDefaults(configuration) {
      if(!configuration||simulationDefaultsLoaded)return;
      const thresholds=configuration.thresholds||{};
      const values={cpu:thresholds.cpu_percent,memory:thresholds.memory_percent,filesystem:thresholds.filesystem_percent,load:thresholds.load_per_cpu};
      Object.keys(values).forEach(key=>{
        const item=values[key]||{};
        byId("sim-"+key+"-warning").value=item.warning;
        byId("sim-"+key+"-critical").value=item.critical;
      });
      simulationDefaultsLoaded=true;
    }
    function simulationInput() {
      const number=id=>Number(byId(id).value);
      return {
        cpu_percent:number("sim-cpu"),memory_percent:number("sim-memory"),load_per_cpu:number("sim-load"),filesystem_percent:number("sim-filesystem"),
        thresholds:{
          cpu_percent:{warning:number("sim-cpu-warning"),critical:number("sim-cpu-critical")},
          memory_percent:{warning:number("sim-memory-warning"),critical:number("sim-memory-critical")},
          filesystem_percent:{warning:number("sim-filesystem-warning"),critical:number("sim-filesystem-critical")},
          load_per_cpu:{warning:number("sim-load-warning"),critical:number("sim-load-critical")}
        }
      };
    }
    function renderSimulation(simulation) {
      const incident=simulation.incident_report||{};
      const cycle=simulation.report&&simulation.report.runtime||{};
      const events=cycle.events||[];
      byId("simulation-output").hidden=false;
      byId("simulation-incident-title").textContent=(incident.title||"Synthetic incident report")+" · SIMULATED";
      byId("simulation-incident-status").textContent="Incident_ID: "+(incident.id||cycle.cycle_id||"unknown")+" · Mission: "+(incident.status||"UNKNOWN")+" · Scenario: "+(simulation.label||simulation.scenario);
      byId("simulation-incident-summary").textContent=incident.summary||"No incident summary supplied.";
      [["simulation-factors",incident.contributing_factors],["simulation-missing",incident.missing_evidence],["simulation-recommendations",incident.recommendations]].forEach(([id,items])=>{
        const list=byId(id);clear(list);
        if(!items||!items.length){list.append(text("li","","None reported."));return;}
        items.forEach(item=>list.append(text("li","",item)));
      });
      [["simulation-why",incident.why_chain],["simulation-how",incident.how_chain]].forEach(([id,links])=>{
        const list=byId(id);clear(list);
        if(!links||!links.length){list.append(text("li","","None reported."));return;}
        links.forEach(link=>list.append(text("li","",link.statement+" ["+String(link.status||"unknown").toUpperCase()+(link.class?" · "+link.class:"")+" · origin: "+(link.origin||"unknown")+(link.evidence&&link.evidence.length?" · evidence: "+link.evidence.join(", "):"")+"]")));
      });
      byId("simulation-chain-note").textContent=incident.chain_note||"";
      renderCascade(events,"cascade-sim",incident);
      byId("simulation-event-meta").textContent="Synthetic cycle "+(cycle.cycle_id||"unknown")+" · "+events.length+" events · all events below are generated from synthetic input.";
      const eventList=byId("simulation-events");clear(eventList);
      if(!events.length)eventList.append(text("div","empty","No events were emitted."));
      events.forEach(event=>{
        const row=text("article","simulation-event","");
        row.append(text("strong","",event.type||"unknown event"),text("span","",localTime(event.occurred_at)+" · "+(event.source||"unknown source")));
        row.append(text("code","","Event_ID: "+(event.event_id||"unknown id")+(event.derives_from&&event.derives_from.length?" · derives from "+event.derives_from.map(id=>"Event_ID: "+id).join(", "):"")));
        eventList.append(row);
      });
      byId("simulation-report").textContent=JSON.stringify(simulation,null,2);
    }
    byId("run-simulation").addEventListener("click",async()=>{
      const button=byId("run-simulation");button.disabled=true;
      const status=byId("simulation-status");status.textContent="Running an isolated synthetic cycle…";
      try{
        const response=await fetch("/api/simulate",{method:"POST",headers:{"Content-Type":"application/json","X-Requested-With":"PerceptronDashboard"},body:JSON.stringify({scenario:byId("simulation-scenario").value,use_snapshot:byId("sim-snapshot").checked,inputs:(()=>{const inputs=simulationInput();if(byId("sim-snapshot").checked){delete inputs.cpu_percent;delete inputs.memory_percent;delete inputs.load_per_cpu;delete inputs.filesystem_percent;}return inputs;})()}),cache:"no-store"});
        if(!response.ok)throw new Error((await response.text())||"Simulation failed ("+response.status+")");
        const result=await response.json();renderSimulation(result);status.textContent="Simulation complete · synthetic only · live report unchanged.";
      }catch(error){status.textContent="Simulation failed: "+(error.message||"unknown error");}
      finally{button.disabled=false;}
    });
    byId("refresh-tool").addEventListener("click",async () => {
      const button = byId("refresh-tool");
      button.disabled = true;
      byId("tool-activity").className = "tool-status queued";
      byId("tool-activity").textContent = "Submitting a read-only host observation refresh…";
      try {
        const response = await fetch("/api/tools/refresh",{method:"POST",headers:{"X-Requested-With":"PerceptronDashboard"},cache:"no-store"});
        if (!response.ok) {
          const message = await response.text();
          throw new Error(message || "Tool request failed (" + response.status + ")");
        }
        const activity = await response.json();
        byId("tool-activity").className = "tool-status " + activity.status;
        byId("tool-activity").textContent = activity.status.toUpperCase() + " · " + activity.message;
      } catch (error) {
        button.disabled = false;
        byId("tool-activity").className = "tool-status failed";
        byId("tool-activity").textContent = error.message || "Tool request failed";
      }
    });
    async function refresh() {
      try {
        const response = await fetch("/api/report",{cache:"no-store"});
        if (!response.ok) throw new Error(response.status === 503 ? "Waiting for first assessment…" : "Dashboard request failed (" + response.status + ")");
        render(await response.json());
      } catch (error) {
        byId("connection").textContent = error.message || "Dashboard disconnected";
        byId("connection-dot").className = "dot offline";
      }
    }
    refresh();
    window.setInterval(refresh,3000);
  </script>
</body>
</html>`
