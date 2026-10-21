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
	Recommendation Recommendation `json:"recommendation"`
	ToolActivity   *ToolActivity  `json:"tool_activity,omitempty"`
	Configuration  *config.Config `json:"configuration,omitempty"`
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
		Rationale:          "The current assessment does not report elevated pressure. This is an advisory, not authorization.",
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
	switch mission.Status {
	case model.MissionRed:
		result.Title = "Review critical pressure"
		result.Rationale = "Critical pressure is reported for " + joinComponents(critical) + ". Review the evidence and use a fresh read-only collection to confirm current conditions; no remediation is authorized."
	case model.MissionYellow:
		result.Title = "Verify elevated pressure"
		result.Rationale = "Elevated pressure is reported for " + joinComponents(degraded) + ". Compare with a fresh read-only observation before deciding on an operational response."
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
	healthEventIDs := make(map[string]bool)
	var missionEventID string
	for _, event := range cycle.Runtime.Events {
		switch event.Type {
		case model.TypeHealth:
			var assessment model.HealthAssessment
			if err := json.Unmarshal(event.Payload, &assessment); err == nil {
				for _, component := range mission.Components {
					if component.Component == assessment.Component && component.Metric == assessment.Metric && component.ScopeID == assessment.ScopeID {
						healthEventIDs[event.ID] = true
					}
				}
			}
		case model.TypeMission:
			missionEventID = event.ID
		}
	}
	for _, event := range cycle.Runtime.Events {
		if healthEventIDs[event.ID] || event.ID == missionEventID {
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
    .hero{display:grid;grid-template-columns:1fr auto;align-items:center;gap:24px;padding:30px;border:1px solid var(--line);border-radius:22px;background:linear-gradient(120deg,#17243a,#111a2b 72%);box-shadow:var(--shadow)}.hero-copy{min-width:0}.hero-label{color:var(--muted);font-size:12px;text-transform:uppercase;letter-spacing:.14em}.hero h2{font-size:clamp(30px,5vw,48px);line-height:1.08;margin:8px 0 10px;letter-spacing:-.055em}.hero p{margin:0;color:#b5c2d8;max-width:760px}.badge{min-width:132px;text-align:center;padding:16px 18px;border:1px solid currentColor;border-radius:18px;font-size:18px;font-weight:800;letter-spacing:.09em;background:#ffffff08}.green{color:var(--green)}.yellow{color:var(--amber)}.red{color:var(--red)}.unknown{color:#b7c1d2}
    .grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:14px;margin:18px 0 26px}.stat,.section{border:1px solid var(--line);border-radius:16px;background:linear-gradient(150deg,#151f32,#101827)}.stat{padding:17px 18px}.stat-label{color:var(--muted);font-size:11px;text-transform:uppercase;letter-spacing:.12em}.stat-value{margin-top:5px;font-size:19px;font-weight:700;letter-spacing:-.025em;overflow-wrap:anywhere}.stat-note{color:var(--muted);font-size:12px;margin-top:3px}
    .section{padding:22px;margin-top:18px}.section-head{display:flex;justify-content:space-between;align-items:flex-end;gap:14px;margin-bottom:16px}.section h3{margin:0;font-size:16px;letter-spacing:-.02em}.section-sub{color:var(--muted);font-size:12px;margin-top:4px}.component-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.component{border:1px solid var(--line);border-radius:13px;padding:16px;background:#0d1524}.component-top{display:flex;align-items:center;justify-content:space-between;gap:12px}.component-name{font-weight:700;text-transform:capitalize}.state{font-size:10px;font-weight:800;letter-spacing:.1em;border-radius:999px;padding:4px 9px;background:#ffffff0d}.component p{color:#b3c0d4;font-size:13px;margin:12px 0 8px}.meta{display:flex;gap:12px;flex-wrap:wrap;color:var(--muted);font-size:11px}
    .table-wrap{overflow-x:auto}table{width:100%;border-collapse:collapse;font-size:13px}th{text-align:left;color:var(--muted);font-size:10px;text-transform:uppercase;letter-spacing:.12em;font-weight:700;padding:10px;border-bottom:1px solid var(--line)}td{padding:11px 10px;border-bottom:1px solid #26344d88;vertical-align:top}tbody tr:last-child td{border-bottom:0}.metric{font-weight:650}.value{font-variant-numeric:tabular-nums;color:#d8e5f8}.reason{color:var(--muted);max-width:420px}.empty{padding:24px;text-align:center;color:var(--muted)}
    details{margin-top:16px;border-top:1px solid var(--line);padding-top:14px}summary{cursor:pointer;color:#b7c7df;font-size:13px;font-weight:650}pre{white-space:pre-wrap;word-break:break-word;color:#b8c7dc;background:#0b1220;border:1px solid var(--line);border-radius:12px;padding:15px;max-height:440px;overflow:auto;font-size:11px}.footer{display:flex;justify-content:space-between;gap:14px;margin-top:18px;color:var(--muted);font-size:11px}.error{color:var(--red)}
    .layout{display:grid;grid-template-columns:minmax(0,1fr) 340px;align-items:start;gap:18px}.main-column{min-width:0}.sidebar{position:sticky;top:18px;max-height:calc(100vh - 36px);overflow:auto}.layout .section{margin-top:18px}.layout .main-column>.section:first-child{margin-top:0}.recommendation{border:1px solid #4b6484;border-radius:18px;padding:22px;background:linear-gradient(130deg,#1b2c43,#131d2e);box-shadow:var(--shadow)}.recommendation-head{display:flex;align-items:flex-start;justify-content:space-between;gap:14px}.recommendation h3{font-size:19px;margin:4px 0 7px}.recommendation p{color:#b8c6da;margin:0 0 14px}.action-btn{border:1px solid #6be3dc88;background:#6be3dc18;color:#c8fffa;padding:10px 14px;border-radius:11px;font:inherit;font-size:13px;font-weight:750;cursor:pointer;white-space:nowrap}.action-btn:hover:not(:disabled){background:#6be3dc30}.action-btn:disabled{opacity:.55;cursor:wait}.action-note{color:var(--muted);font-size:11px;margin-top:12px}.advisory{display:inline-flex;color:var(--amber);font-size:10px;text-transform:uppercase;letter-spacing:.12em;font-weight:800}.engine-list{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:9px}.engine-chip{border:1px solid var(--line);border-radius:11px;padding:11px;background:#0d1524}.engine-chip strong{display:block;font-size:12px}.engine-chip span{color:var(--muted);font-size:10px}.category-stack{display:grid;gap:14px}.category{border:1px solid var(--line);border-radius:15px;background:linear-gradient(150deg,#151f32,#101827);overflow:hidden}.category-head{display:flex;align-items:center;justify-content:space-between;padding:15px 17px;border-bottom:1px solid var(--line);background:#ffffff04}.category-title{font-weight:750}.category-note{color:var(--muted);font-size:11px;margin-top:2px}.category-items{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px;padding:12px}.metric-card{background:#0d1524;border:1px solid var(--line);border-radius:12px;padding:13px}.metric-card-head{display:flex;justify-content:space-between;align-items:center;gap:8px}.metric-name{font-size:13px;font-weight:700}.metric-value{font-size:18px;font-weight:750;font-variant-numeric:tabular-nums;margin:9px 0 3px}.metric-detail{font-size:11px;color:var(--muted);overflow-wrap:anywhere}.metric-card .meta{margin-top:9px;font-size:10px}.side-card{border:1px solid var(--line);border-radius:15px;background:linear-gradient(150deg,#151f32,#101827);padding:17px;margin-bottom:14px}.side-card h3{font-size:14px;margin:0 0 4px}.side-sub{font-size:11px;color:var(--muted);margin-bottom:13px}.rule-id{font:10px ui-monospace,SFMono-Regular,Consolas,monospace;color:var(--cyan);overflow-wrap:anywhere}.evidence-list,.event-list{display:grid;gap:9px}.evidence-item{font:10px ui-monospace,SFMono-Regular,Consolas,monospace;color:#b8c7dc;overflow-wrap:anywhere}.evidence-link{background:transparent;border:0;text-align:left;padding:0;cursor:pointer}.evidence-link:hover{color:var(--cyan)}.timeline-item{border-left:1px solid #40516d;padding:0 0 10px 12px;position:relative;transition:background .2s}.timeline-item:before{content:"";position:absolute;left:-4px;top:4px;width:7px;height:7px;border-radius:50%;background:var(--blue)}.timeline-item.evidence-highlight{background:#6be3dc14;outline:1px solid #6be3dc66;border-radius:5px}.timeline-item strong{display:block;font-size:11px}.timeline-item span{display:block;color:var(--muted);font-size:10px;margin-top:2px;overflow-wrap:anywhere}.timeline-item code{color:#8fa2c0;font-size:9px}.tool-status{padding:9px;border:1px solid var(--line);border-radius:10px;color:#bac8dd;font-size:11px}.tool-status.completed{border-color:#45d6a066;color:var(--green)}.tool-status.running,.tool-status.queued{border-color:#f3bd6266;color:var(--amber)}.tool-status.failed{border-color:#ff718566;color:var(--red)}
    @media(max-width:1020px){.layout{grid-template-columns:minmax(0,1fr) 300px}.engine-list{grid-template-columns:1fr}}@media(max-width:780px){.layout{grid-template-columns:1fr}.sidebar{position:static;max-height:none;display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.sidebar .side-card{margin:0}.sidebar .event-card{grid-column:1/-1}.engine-list{grid-template-columns:repeat(3,minmax(0,1fr))}}@media(max-width:540px){.sidebar{grid-template-columns:1fr}.sidebar .event-card{grid-column:auto}.engine-list{grid-template-columns:1fr 1fr}.recommendation-head{flex-direction:column}.action-btn{width:100%}.category-items{grid-template-columns:1fr}.category-head{align-items:flex-start;gap:8px;flex-direction:column}}
    .decision-grid{display:grid;grid-template-columns:minmax(0,1.1fr) minmax(320px,.9fr);gap:16px;align-items:stretch}.decision-grid .hero,.decision-grid .recommendation{min-height:100%;margin:0}.decision-grid .hero{padding:24px}.mission-facts{margin:14px auto 8px;max-width:1000px;grid-template-columns:repeat(4,minmax(0,1fr))}.mission-facts .stat{padding:13px 16px}.mission-facts .stat-value{font-size:16px}
    .config-panel{margin-bottom:14px}.config-panel>summary{font-size:14px}.config-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px;margin:16px 0}.config-grid label{display:grid;gap:6px;color:#b5c4da;font-size:12px}.config-grid input,.config-grid textarea,.config-thresholds input{width:100%;min-width:70px;border:1px solid #3a4c67;border-radius:8px;background:#0b1220;color:var(--text);padding:9px;font:inherit}.config-grid textarea{resize:vertical}.config-thresholds{margin:10px 0 16px}.engine-settings{border:1px solid var(--line);border-radius:12px;padding:12px}.engine-settings legend{color:#b5c4da;font-size:12px}.engine-options{display:flex;gap:10px;flex-wrap:wrap}.engine-option{display:flex;gap:7px;align-items:center;border:1px solid var(--line);border-radius:8px;padding:8px 10px;color:#d2def0;font-size:12px}.engine-option small{display:block;color:var(--muted)}.engine-move{border:1px solid var(--line);border-radius:5px;background:#172238;color:#d2def0;cursor:pointer}.config-actions{display:flex;align-items:center;gap:12px;flex-wrap:wrap;margin:14px 0}.secondary-btn{border:1px solid #43536c;background:#111b2b;color:#d2def0;padding:10px 14px;border-radius:11px;font:inherit;font-size:13px;cursor:pointer}.config-plugin-note{color:var(--amber);font-size:11px}.config-error{color:var(--red)!important}
    .pyramid{display:flex;flex-direction:column;align-items:center;gap:14px;margin-top:14px}.tier{width:100%;min-width:0}.tier-diagnosis{width:82%}.tier-engines{width:88%}.tier-components{width:94%}.tier-evidence{width:100%}.tier .section{margin-top:0}.tier-label{display:flex;align-items:center;gap:10px;margin:0 0 9px 3px;color:var(--muted);font-size:10px;text-transform:uppercase;letter-spacing:.14em;font-weight:750}.tier-number{display:grid;place-items:center;width:22px;height:22px;border:1px solid #47617e;border-radius:50%;color:var(--cyan);font-size:10px;letter-spacing:0}
    .diagnosis-summary{padding:17px;border:1px solid #344865;border-radius:13px;background:#0d1524;color:#dce8fa}.diagnosis-factors{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px;margin-top:12px}.diagnosis-factor{padding:13px;border:1px solid var(--line);border-radius:11px;background:#111b2b}.diagnosis-factor p{margin:7px 0 0;color:#b3c0d4;font-size:12px}.diagnosis-footnote{margin:12px 0 0;color:var(--muted);font-size:11px}.reasoning-evidence{margin-top:13px;padding-top:12px;border-top:1px solid var(--line)}.incident-note{margin:0 0 14px;padding:11px 13px;border:1px solid #675637;border-radius:10px;background:#2c2416;color:#e9d5a8;font-size:12px}.timeline-item{position:relative}.timeline-time{color:var(--cyan);font-size:11px;font-variant-numeric:tabular-nums}.evidence-highlight{outline:2px solid var(--cyan);outline-offset:2px}.pyramid .table-wrap{border:1px solid var(--line);border-radius:12px;padding:0 10px}.tier-evidence .section+.section{margin-top:14px}
    @media(max-width:900px){.decision-grid{grid-template-columns:1fr}.decision-grid .hero{grid-template-columns:1fr auto}.tier-diagnosis,.tier-engines,.tier-components{width:100%}.config-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}
    @media(max-width:780px){.grid{grid-template-columns:repeat(2,minmax(0,1fr))}.hero{grid-template-columns:1fr auto;padding:22px}.section{padding:17px}}@media(max-width:540px){.shell{padding:24px 14px 40px}.top{align-items:flex-start}.connection{font-size:0}.connection .dot{flex:none}.hero{grid-template-columns:1fr}.badge{justify-self:start}.component-grid,.diagnosis-factors{grid-template-columns:1fr}.grid{gap:9px}.stat{padding:13px}.stat-value{font-size:16px}.section-head{align-items:flex-start;flex-direction:column}.action-btn{white-space:normal}.recommendation-head{flex-direction:column}.config-grid{grid-template-columns:1fr}}
    .page-nav{display:flex;gap:8px;margin:-12px 0 20px}.page-nav button{border:1px solid var(--line);border-radius:10px;padding:9px 14px;background:#111a2b;color:var(--muted);font:inherit;font-size:13px;cursor:pointer}.page-nav button[aria-current=page]{border-color:#6be3dc88;color:var(--cyan);background:#6be3dc12}.store-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.store-card{border:1px solid var(--line);border-radius:14px;padding:17px;background:#0d1524}.store-card h3{margin:0;font-size:15px}.store-card p{color:#b3c0d4;font-size:12px}.store-meta{display:flex;gap:8px;flex-wrap:wrap;color:var(--muted);font-size:10px;margin-top:10px}.store-policy{font:11px ui-monospace,SFMono-Regular,Consolas,monospace;color:#c5d5eb;overflow-wrap:anywhere}.store-card button{margin-top:12px}.plugin-setting{display:grid;gap:6px;min-width:min(100%,360px);flex:1;color:#c5d5eb;font-size:11px}.plugin-setting textarea{width:100%;border:1px solid #3a4c67;border-radius:8px;background:#0b1220;color:var(--text);padding:9px;font:12px ui-monospace,SFMono-Regular,Consolas,monospace}
    @media(max-width:540px){.store-grid{grid-template-columns:1fr}}
  </style>
</head>
<body>
  <main class="shell">
    <header class="top">
      <div class="brand"><div class="mark" aria-hidden="true">P</div><div><div class="eyebrow">Local operations</div><h1>Perceptron SRE</h1></div></div>
      <div class="connection"><span id="connection-dot" class="dot offline"></span><span id="connection">Connecting to local runtime…</span></div>
    </header>
    <nav class="page-nav" aria-label="Dashboard pages"><button id="operations-tab" type="button" aria-current="page">Operations</button><button id="store-tab" type="button">Engine Store</button></nav>
    <div id="operations-page">
    <section class="decision-grid" aria-label="Mission decision and recommended next step">
      <section class="hero" aria-live="polite">
        <div class="hero-copy"><div class="hero-label" id="scope">Mission assessment</div><h2 id="objective">Waiting for first assessment</h2><p id="explanation">The dashboard will update as soon as the first observation cycle completes.</p></div>
        <div id="mission-badge" class="badge unknown">WAITING</div>
      </section>
      <section class="recommendation" aria-live="polite"><div class="recommendation-head"><div><span class="advisory">Advisory · no automatic execution</span><h3 id="recommendation-title">Waiting for recommendation</h3><p id="recommendation-rationale">A recommendation will appear after the first assessment.</p></div><button id="refresh-tool" class="action-btn" type="button">Run read-only refresh</button></div><div class="action-note">Tool: <strong>host-observation-refresh</strong> · Collects current host metrics only; makes no system changes.</div><div id="tool-activity" class="tool-status" style="margin-top:12px">No dashboard-triggered tool run yet.</div></section>
    </section>
    <section id="stats" class="grid mission-facts" aria-label="Mission evidence summary"></section>
    <details class="section config-panel"><summary>Runtime configuration · edit thresholds and enabled engines</summary><div class="section-sub">Changes are validated and written to the configuration file. External file edits are detected before the next collection cycle.</div><form id="configuration-form">
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
      <fieldset class="engine-settings"><legend>Enabled engines</legend><div id="engine-options" class="engine-options">Loading registered engines…</div><p class="section-sub">No engines is valid; that mode produces observations and events without domain assessments.</p></fieldset>
      <div class="config-actions"><button id="save-config" class="action-btn" type="submit">Save configuration</button><button id="discard-config" class="secondary-btn" type="button">Discard edits</button><span id="config-status" role="status" class="section-sub">No unsaved changes.</span></div>
      <p class="config-plugin-note">Plugins run as this application user and are not sandboxed. Current runtime only enables read-only plugins; policy declarations do not grant access to tools or scripts.</p>
    </form></details>
    <div class="pyramid" role="region" aria-label="Assessment reasoning pyramid">
      <section class="tier tier-diagnosis">
        <div class="tier-label"><span class="tier-number">1</span> Root-cause assessment</div>
        <section class="section" aria-labelledby="diagnosis-heading"><div class="section-head"><div><h3 id="diagnosis-heading">What is driving this assessment?</h3><div class="section-sub">Leading evidence and contributing component states; not a causal claim beyond the available evidence</div></div></div><div id="diagnosis-summary" class="diagnosis-summary">Waiting for assessment evidence…</div><div id="diagnosis-factors" class="diagnosis-factors"></div><p class="diagnosis-footnote">Unknown or unavailable evidence remains explicit; coverage does not imply health.</p><div class="reasoning-evidence"><div id="recommendation-rule" class="rule-id">Waiting for rule…</div><p id="recommendation-reason" class="section-sub">Waiting for evidence…</p><div id="recommendation-evidence" class="evidence-list"></div></div></section>
      </section>
      <section class="tier tier-engines">
        <div class="tier-label"><span class="tier-number">2</span> Reasoning engines</div>
        <section class="section"><div class="section-head"><div><h3>Engine execution</h3><div class="section-sub">Runtime availability is distinct from the health result an engine produces</div></div></div><div id="engine-status" class="engine-list"><div class="empty">Waiting for engine status…</div></div></section>
      </section>
      <section class="tier tier-components">
        <div class="tier-label"><span class="tier-number">3</span> Components</div>
        <section class="section"><div class="section-head"><div><h3>Component health by category</h3><div class="section-sub">Signals grouped by domain with their derived component assessment</div></div></div><div id="components" class="category-stack"><div class="empty">Waiting for assessment data…</div></div></section>
      </section>
      <section class="tier tier-evidence">
        <div class="tier-label"><span class="tier-number">4</span> Source evidence and event record</div>
        <section class="section"><div class="section-head"><div><h3>Metrics and observations</h3><div class="section-sub">Underlying values, unavailable reasons, and observation times</div></div></div><div class="table-wrap"><table><thead><tr><th>Signal</th><th>Value</th><th>Observation</th><th>Observed at</th></tr></thead><tbody id="observations"><tr><td colspan="4" class="empty">Waiting for observations…</td></tr></tbody></table></div></section>
        <section class="section"><div class="section-head"><div><h3>Application logs</h3><div class="section-sub">Runtime diagnostics are emitted separately from the assessment event stream</div></div></div><p class="section-sub">Structured JSON logs are written to stderr (normally collected by the system journal under systemd). Log history is not retained or displayed in this dashboard.</p></section>
        <section class="section"><div class="section-head"><div><h3>Event timeline and provenance</h3><div id="provenance" class="section-sub">Source observations → derived assessments</div></div></div><p id="incident-capture-note" class="incident-note">Current-cycle events only. This prototype does not retain the 60 seconds before an incident or track mitigation and its five-minute recovery window.</p><div id="event-list" class="event-list"><div class="empty">Waiting for event lineage…</div></div><details><summary>Inspect event payloads</summary><pre id="events">Waiting for events…</pre></details></section>
      </section>
    </div>
    </div>
    <section id="store-page" hidden>
      <section class="section"><div class="section-head"><div><h2>Engine Store</h2><div class="section-sub">All enabled reasoning modules, including Perceptron-maintained defaults, are catalogued here. Mission aggregation, event transport, policy enforcement, and dashboard services remain core responsibilities.</div></div></div><p class="section-sub">Enable or disable a read-only engine here. Configure per-engine settings in Operations → Runtime configuration. Policy declarations describe intent; they are not grants.</p><div id="store-status" class="section-sub" role="status">Loading the local engine catalog…</div><div id="store-entries" class="store-grid"></div></section>
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
    const builtInEngines = [{name:"host-pressure",description:"Pressure thresholds"},{name:"host-health",description:"Component health"}];
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
    byId("operations-tab").addEventListener("click",()=>{byId("operations-page").hidden=false;byId("store-page").hidden=true;byId("operations-tab").setAttribute("aria-current","page");byId("store-tab").removeAttribute("aria-current");});
    byId("store-tab").addEventListener("click",()=>{byId("operations-page").hidden=true;byId("store-page").hidden=false;byId("store-tab").setAttribute("aria-current","page");loadCatalog().catch(error=>byId("store-status").textContent=error.message||"Engine catalog unavailable.");});
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
    function renderEngines(metrics) {
      clear(byId("engine-status"));
      const engines = metrics.engines || {};
      const coreServices = metrics.core_services || {};
      const entries = Object.keys(engines).map(name=>({name,item:engines[name],core:false})).concat(Object.keys(coreServices).map(name=>({name,item:coreServices[name],core:true}))).sort((left,right)=>left.name.localeCompare(right.name));
      if (!entries.length) { byId("engine-status").append(text("div","empty","No engine executions are recorded.")); return; }
      entries.forEach(entry => {
        const item = entry.item;
        const failed = Number(item.failures || 0) > 0;
        const status = failed ? "FAILED" : (Number(item.runs || 0) > 0 ? "AVAILABLE" : "IDLE");
        const card = text("div","engine-chip","");
        card.append(text("strong","",(entry.core?"Core · ":"")+entry.name),text("span","",status + " · " + String(item.duration_millis || 0) + " ms"));
        byId("engine-status").append(card);
      });
    }
    function renderDiagnosis(mission) {
      const components = (mission.components || []).slice();
      const rank = {CRITICAL:0,DEGRADED:1,UNKNOWN:2,HEALTHY:3};
      components.sort((left,right) => (rank[left.state] == null ? 4 : rank[left.state]) - (rank[right.state] == null ? 4 : rank[right.state]));
      const leading = components.filter(item => item.state === "CRITICAL" || item.state === "DEGRADED");
      const unknown = components.filter(item => item.state === "UNKNOWN");
      byId("diagnosis-summary").textContent = leading.length
        ? "Leading assessed contributors: " + leading.slice(0,3).map(item => item.component + " (" + item.state.toLowerCase() + ")").join(", ") + ". " + (mission.explanation || "Review the component evidence below.")
        : (unknown.length
          ? "No confirmed pressure contributor is available. Evidence is unknown for " + unknown.slice(0,3).map(item => item.component).join(", ") + ". " + (mission.explanation || "Do not infer health from missing evidence.")
          : "No elevated component pressure is reported in this cycle. " + (mission.explanation || "Review the supporting evidence below."));
      clear(byId("diagnosis-factors"));
      const factors = components.slice(0,4);
      if (!factors.length) byId("diagnosis-factors").append(text("div","empty","No component assessments are available."));
      factors.forEach(item => {
        const factor = text("article","diagnosis-factor","");
        factor.append(text("strong","",item.component + " · " + item.state),text("p","",item.explanation || "No component explanation supplied."));
        byId("diagnosis-factors").append(factor);
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
        const link = text("button","evidence-item evidence-link","↳ " + id);
        link.type = "button";
        link.addEventListener("click",() => {
          const target = document.querySelector('[data-event-id="' + id + '"]');
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
      events.slice().sort((left,right) => new Date(left.occurred_at) - new Date(right.occurred_at)).forEach(event => {
        const item = text("div","timeline-item","");
        item.dataset.eventId = event.event_id || "";
        item.append(text("span","timeline-time",localTime(event.occurred_at)),text("strong","",names[event.type] || event.type),text("span","",(event.source || "unknown source") + (event.emitted_at && event.emitted_at !== event.occurred_at ? " · emitted " + localTime(event.emitted_at) : "")),text("code","",(event.event_id || "unknown id") + (event.derives_from && event.derives_from.length ? " · derives from " + event.derives_from.length + " event(s)" : "")));
        byId("event-list").append(item);
      });
    }
    function render(data) {
      const runtime = data.runtime || {};
      const mission = runtime.mission || {scope:{type:"host",id:data.host_id},objective:"Core mission assessment unavailable",status:"UNKNOWN",disposition:"unable_to_assess",explanation:"The core is collecting observations, but its mission assessment is unavailable.",components:[],coverage:{assessed_components:[],required_components:[],complete:false},freshness:"not_assessed"};
      const status = mission.status || "UNKNOWN";
      const badge = byId("mission-badge"); badge.textContent = status; badge.className = "badge " + stateClass(status);
      byId("scope").textContent = (mission.scope && mission.scope.type ? mission.scope.type : "mission") + " · " + (mission.scope && mission.scope.id ? mission.scope.id : data.host_id);
      byId("objective").textContent = mission.objective || "Mission objective unavailable";
      byId("explanation").textContent = mission.explanation || "No explanation supplied.";
      const metrics = runtime.metrics || {};
      renderConfiguration(data.configuration);
      clear(byId("stats"));
      byId("stats").append(
        stat("Mission status",status,mission.disposition || "No disposition"),
        stat("Assessment coverage",String((mission.coverage && mission.coverage.assessed_components || []).length) + " / " + String((mission.coverage && mission.coverage.required_components || []).length),mission.coverage && mission.coverage.complete ? "All required scopes represented" : "Coverage incomplete"),
        stat("Evidence freshness",mission.freshness || "unknown","Freshness is distinct from coverage"),
        stat("Observations",String(data.available_observations || 0) + " available",String(data.unavailable_observations || 0) + " unavailable")
      );
      const events = runtime.events || [];
      renderCategories(data,mission,events);
      renderDiagnosis(mission);
      renderEngines(metrics);
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
      byId("provenance").textContent = "Cycle " + (runtime.cycle_id || "unknown") + " · " + String(events.length) + " events · " + String(metrics.duration_millis || 0) + " ms";
      byId("events").textContent = JSON.stringify(events.map(event => ({event_id:event.event_id,type:event.type,source:event.source,producer_version:event.producer_version,configuration_id:event.configuration_id,occurred_at:event.occurred_at,derives_from:event.derives_from || [],payload:event.payload})),null,2);
      byId("updated").textContent = "Report evaluated " + localTime(runtime.completed_at || mission.evaluated_at);
      byId("connection").textContent = "Live · refreshes every 3 seconds";
      byId("connection-dot").className = "dot";
    }
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
