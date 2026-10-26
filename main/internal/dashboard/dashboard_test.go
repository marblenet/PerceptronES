package dashboard

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"perceptronsre/internal/config"
	"perceptronsre/internal/engine"
	"perceptronsre/internal/model"
	"perceptronsre/internal/report"
	"perceptronsre/internal/simulation"
)

func TestListenLoopbackBindsOnlyIPv4Loopback(t *testing.T) {
	listener, err := ListenLoopback(0)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || !address.IP.IsLoopback() || !address.IP.Equal(net.ParseIP("127.0.0.1")) {
		t.Fatalf("dashboard listener is not bound to IPv4 loopback: %#v", listener.Addr())
	}
}

func TestListenLoopbackRejectsInvalidPort(t *testing.T) {
	for _, port := range []int{-1, 65536} {
		if _, err := ListenLoopback(port); err == nil {
			t.Errorf("expected port %d to be rejected", port)
		}
	}
}

func TestDashboardServesPageAndLatestReport(t *testing.T) {
	dashboard := New()
	server := httptest.NewServer(dashboard.Handler())
	defer server.Close()

	pageResponse, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer pageResponse.Body.Close()
	if pageResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected dashboard page status: %d", pageResponse.StatusCode)
	}
	if pageResponse.Header.Get("Content-Security-Policy") == "" || pageResponse.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("security headers are missing: %#v", pageResponse.Header)
	}
	page, err := ioutil.ReadAll(pageResponse.Body)
	if err != nil {
		t.Fatalf("read dashboard page: %v", err)
	}
	pageHTML := string(page)
	if !strings.Contains(pageHTML, "Mission decision") ||
		!strings.Contains(pageHTML, "Root-cause assessment") ||
		!strings.Contains(pageHTML, "Component health by category") ||
		!strings.Contains(pageHTML, "Application logs") ||
		!strings.Contains(pageHTML, "Event timeline &amp; provenance") ||
		!strings.Contains(pageHTML, "newest on left") ||
		!strings.Contains(pageHTML, "event-node") ||
		!strings.Contains(pageHTML, "settings-tab") ||
		!strings.Contains(pageHTML, "security-tab") ||
		!strings.Contains(pageHTML, "security-page") ||
		!strings.Contains(pageHTML, "The Alert Window") ||
		!strings.Contains(pageHTML, "Suggested Mitigation Window") ||
		!strings.Contains(pageHTML, "Reasoning engines") ||
		!strings.Contains(pageHTML, "Runtime configuration") ||
		!strings.Contains(pageHTML, "Engine Store") ||
		!strings.Contains(pageHTML, "Available core capabilities") ||
		!strings.Contains(pageHTML, "Security posture engine") ||
		!strings.Contains(pageHTML, "data-capability=\"security-posture\"") ||
		!strings.Contains(pageHTML, "data-capability=\"simulation\"") ||
		!strings.Contains(pageHTML, "Available · synthetic only") ||
		!strings.Contains(pageHTML, "Available · evidence-driven") ||
		!strings.Contains(pageHTML, "Simulation page") ||
		!strings.Contains(pageHTML, "simulation-tab") ||
		!strings.Contains(pageHTML, "mission-factors") ||
		!strings.Contains(pageHTML, "Synthetic measurements") ||
		!strings.Contains(pageHTML, "All synthetic events") ||
		!strings.Contains(pageHTML, "Complete simulation report") ||
		!strings.Contains(pageHTML, "Run isolated simulation") ||
		!strings.Contains(pageHTML, "Required security checks") ||
		!strings.Contains(pageHTML, "Manually supplied security evidence") ||
		!strings.Contains(pageHTML, "Security posture findings") ||
		!strings.Contains(pageHTML, "enabled findings qualify mission status") ||
		!strings.Contains(pageHTML, "main/external/") ||
		!strings.Contains(pageHTML, "plugin-settings") ||
		!strings.Contains(pageHTML, "Policy metadata is declarative") ||
		!strings.Contains(pageHTML, "threshold-cpu-warning") ||
		!strings.Contains(pageHTML, "Enabled engines") ||
		!strings.Contains(pageHTML, "read-only refresh") ||
		!strings.Contains(pageHTML, "Unknown is a valid result") ||
		!strings.Contains(pageHTML, "GREEN requires complete, valid, fresh required evidence") ||
		!strings.Contains(pageHTML, "Event_ID: ") ||
		!strings.Contains(pageHTML, "Reasoning cascade (L1") ||
		!strings.Contains(pageHTML, "Fork from live snapshot") ||
		!strings.Contains(pageHTML, "live-why") ||
		!strings.Contains(pageHTML, "Why chain (backward causality)") ||
		!strings.Contains(pageHTML, "How chain (forward remediation)") ||
		!strings.Contains(pageHTML, "Event_ID:\"Event_ID: \"") ||
		!strings.Contains(pageHTML, "(event.derives_from || []).map(id => \"Event_ID: \" + id)") ||
		!strings.Contains(pageHTML, "textContent") ||
		!strings.Contains(pageHTML, "does not retain the 60 seconds before an incident") ||
		!strings.Contains(pageHTML, "--bg:#080b09") ||
		!strings.Contains(pageHTML, "--green:#83c987") ||
		!strings.Contains(pageHTML, ".hero,.recommendation{background:linear-gradient(120deg,#172019,#101510 75%)") {
		t.Fatal("dashboard page is missing pyramid levels, available security/simulation capabilities, philosophy guardrails, labeled event IDs, refresh control, safety semantics, or safe text insertion")
	}
	levels := []string{"Event timeline", "The Alert Window", "Root-cause assessment", "Component health by category", "Metrics and observations"}
	lastIndex := -1
	for _, level := range levels {
		index := strings.Index(pageHTML, level)
		if index <= lastIndex {
			t.Fatalf("dashboard pyramid level %q is missing or out of order", level)
		}
		lastIndex = index
	}

	before := httptest.NewRecorder()
	dashboard.Handler().ServeHTTP(before, httptest.NewRequest(http.MethodGet, "/api/report", nil))
	if before.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected waiting dashboard API to return 503, got %d", before.Code)
	}

	expected := testCycleReport(t)
	if err := dashboard.Update(expected); err != nil {
		t.Fatal(err)
	}
	apiResponse, err := http.Get(server.URL + "/api/report")
	if err != nil {
		t.Fatal(err)
	}
	defer apiResponse.Body.Close()
	if apiResponse.StatusCode != http.StatusOK || apiResponse.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected report response: status=%d headers=%v", apiResponse.StatusCode, apiResponse.Header)
	}
	var actual report.CycleReport
	if err := json.NewDecoder(apiResponse.Body).Decode(&actual); err != nil {
		t.Fatalf("decode report response: %v", err)
	}
	if actual.Runtime.CycleID != expected.Runtime.CycleID || actual.Runtime.Mission.Status != model.MissionGreen {
		t.Fatalf("dashboard did not return latest report: %#v", actual)
	}
	metricsResponse, err := http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer metricsResponse.Body.Close()
	metrics, err := ioutil.ReadAll(metricsResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	if metricsResponse.StatusCode != http.StatusOK || !strings.Contains(string(metrics), "perceptron_mission_status") {
		t.Fatalf("Prometheus metrics endpoint did not export mission status: status=%d body=%s", metricsResponse.StatusCode, metrics)
	}
	findingsResponse, err := http.Get(server.URL + "/api/findings")
	if err != nil {
		t.Fatal(err)
	}
	defer findingsResponse.Body.Close()
	var findings report.CycleReport
	if err := json.NewDecoder(findingsResponse.Body).Decode(&findings); err != nil {
		t.Fatalf("decode bulk findings: %v", err)
	}
	if findings.Runtime.CycleID != expected.Runtime.CycleID {
		t.Fatalf("bulk findings response omitted current engine results: %#v", findings.Runtime)
	}
}

func TestEngineCatalogIncludesBuiltInsAndPolicy(t *testing.T) {
	path := "unused-dashboard-test-config.json"
	initial := config.Default()
	store, err := config.NewStore(path, initial)
	if err != nil {
		t.Fatal(err)
	}
	dashboard := New()
	dashboard.SetConfigStore(store)
	response := httptest.NewRecorder()
	dashboard.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/engines", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("engine catalog returned %d: %s", response.Code, response.Body.String())
	}
	var entries []engineCatalogEntry
	if err := json.NewDecoder(response.Body).Decode(&entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 || entries[0].Name != "host-pressure" || entries[0].Policy.Mode != "reasoning_only" || entries[2].Name != "security-posture" {
		t.Fatalf("catalog is missing built-in engine policy metadata: %#v", entries)
	}
}

func TestDashboardRunsIsolatedSimulationWithoutReplacingLiveReport(t *testing.T) {
	cfg := config.Default()
	store, err := config.NewStore("unused-simulation-dashboard-test.json", cfg)
	if err != nil {
		t.Fatal(err)
	}
	dashboard := New()
	dashboard.SetConfigStore(store)
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/api/simulate", strings.NewReader(`{"scenario":"normal","inputs":{"cpu_percent":99,"memory_percent":40,"load_per_cpu":0.5,"filesystem_percent":50,"thresholds":{"cpu_percent":{"warning":85,"critical":95},"memory_percent":{"warning":85,"critical":95},"filesystem_percent":{"warning":85,"critical":95},"load_per_cpu":{"warning":1.5,"critical":3}}}}`))
	request.Header.Set("Origin", "http://127.0.0.1:8080")
	request.Header.Set("X-Requested-With", "PerceptronDashboard")
	response := httptest.NewRecorder()
	dashboard.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("simulation endpoint returned %d: %s", response.Code, response.Body.String())
	}
	var result simulation.Result
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if !result.Synthetic || result.Report.Runtime.Mission == nil || result.Report.Runtime.Mission.Status != model.MissionRed ||
		result.IncidentReport.Status != string(model.MissionRed) || len(result.IncidentReport.ContributingFactors) == 0 ||
		len(result.IncidentReport.WhyChain) < 2 || len(result.IncidentReport.HowChain) < 2 {
		t.Fatalf("simulation endpoint returned unexpected result: %#v", result)
	}
	live := httptest.NewRecorder()
	dashboard.Handler().ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/api/report", nil))
	if live.Code != http.StatusServiceUnavailable {
		t.Fatalf("simulation unexpectedly populated/replaced live report: %d %s", live.Code, live.Body.String())
	}
}

func TestDashboardSnapshotSimulationForksLatestReportWithoutChangingIt(t *testing.T) {
	store, err := config.NewStore("unused-snapshot-dashboard-test.json", config.Default())
	if err != nil {
		t.Fatal(err)
	}
	dashboard := New()
	dashboard.SetConfigStore(store)
	post := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/api/simulate", strings.NewReader(body))
		request.Header.Set("Origin", "http://127.0.0.1:8080")
		request.Header.Set("X-Requested-With", "PerceptronDashboard")
		response := httptest.NewRecorder()
		dashboard.Handler().ServeHTTP(response, request)
		return response
	}
	if response := post(`{"use_snapshot":true,"inputs":{}}`); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("snapshot without live report returned %d", response.Code)
	}
	seed, err := simulation.RunScenario(context.Background(), config.Default(), "normal")
	if err != nil {
		t.Fatal(err)
	}
	live := seed.Report
	live.Runtime.CycleID = "live-cycle-fixture"
	if err := dashboard.Update(live); err != nil {
		t.Fatal(err)
	}
	response := post(`{"use_snapshot":true,"inputs":{"cpu_percent":99}}`)
	if response.Code != http.StatusOK {
		t.Fatalf("snapshot simulation returned %d: %s", response.Code, response.Body.String())
	}
	var result simulation.Result
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if !result.Synthetic || result.SnapshotOf != live.Runtime.CycleID {
		t.Fatalf("snapshot result not labeled: synthetic=%t of=%q", result.Synthetic, result.SnapshotOf)
	}
	report := httptest.NewRecorder()
	dashboard.Handler().ServeHTTP(report, httptest.NewRequest(http.MethodGet, "/api/report", nil))
	if report.Code != http.StatusOK || !strings.Contains(report.Body.String(), `"live_incident"`) {
		t.Fatalf("live report missing live_incident: %d", report.Code)
	}
}

func TestDashboardSimulationRequiresSameOriginAndKnownScenario(t *testing.T) {
	store, err := config.NewStore("unused-simulation-security-test.json", config.Default())
	if err != nil {
		t.Fatal(err)
	}
	dashboard := New()
	dashboard.SetConfigStore(store)
	handler := dashboard.Handler()

	for _, test := range []struct {
		name       string
		body       string
		origin     string
		wantStatus int
	}{
		{name: "cross origin", body: `{"scenario":"normal"}`, origin: "http://attacker.invalid", wantStatus: http.StatusForbidden},
		{name: "unknown scenario", body: `{"scenario":"arbitrary"}`, origin: "http://127.0.0.1:8080", wantStatus: http.StatusBadRequest},
	} {
		request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/api/simulate", strings.NewReader(test.body))
		request.Header.Set("Origin", test.origin)
		request.Header.Set("X-Requested-With", "PerceptronDashboard")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.wantStatus {
			t.Errorf("%s: expected status %d, got %d: %s", test.name, test.wantStatus, response.Code, response.Body.String())
		}
	}
}

func TestDashboardReadOnlyRefreshQueuesAndTracksActivity(t *testing.T) {
	refreshRequests := make(chan ToolActivity, 1)
	dashboard := New(refreshRequests)
	if err := dashboard.Update(testCycleReport(t)); err != nil {
		t.Fatal(err)
	}
	handler := dashboard.Handler()
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/api/tools/refresh", nil)
	request.Header.Set("Origin", "http://127.0.0.1:8080")
	request.Header.Set("X-Requested-With", "PerceptronDashboard")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("expected read-only refresh to queue with 202, got %d: %s", response.Code, response.Body.String())
	}
	var activity ToolActivity
	if err := json.NewDecoder(response.Body).Decode(&activity); err != nil {
		t.Fatal(err)
	}
	if activity.Name != "host-observation-refresh" || activity.Status != "queued" {
		t.Fatalf("unexpected queued tool activity: %#v", activity)
	}
	select {
	case queued := <-refreshRequests:
		if queued.ID != activity.ID {
			t.Fatalf("request ID differs in queue: %#v", queued)
		}
	default:
		t.Fatal("refresh request was not queued")
	}

	dashboard.MarkToolStarted(activity.ID)
	dashboard.CompleteTool(activity.ID, "cycle-refreshed", nil)
	api := httptest.NewRecorder()
	handler.ServeHTTP(api, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/api/report", nil))
	var payload map[string]interface{}
	if err := json.NewDecoder(api.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	tool, ok := payload["tool_activity"].(map[string]interface{})
	if !ok || tool["status"] != "completed" || tool["cycle_id"] != "cycle-refreshed" {
		t.Fatalf("tool outcome was not surfaced through the dashboard: %#v", payload["tool_activity"])
	}
}

func TestDashboardLoadsAndUpdatesConfiguration(t *testing.T) {
	file, err := ioutil.TempFile("", "perceptron-dashboard-config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	configFile, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WriteDefault(configFile); err != nil {
		configFile.Close()
		t.Fatal(err)
	}
	if err := configFile.Close(); err != nil {
		t.Fatal(err)
	}
	initial, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	store, err := config.NewStore(path, initial)
	if err != nil {
		t.Fatal(err)
	}
	dashboard := New()
	dashboard.SetConfigStore(store)
	handler := dashboard.Handler()

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("expected config GET 200, got %d: %s", get.Code, get.Body.String())
	}
	var loaded config.Config
	if err := json.NewDecoder(get.Body).Decode(&loaded); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Engines) != 2 {
		t.Fatalf("expected default built-in engine set, got %#v", loaded.Engines)
	}

	updated := loaded
	updated.Thresholds.CPUPercent = config.Threshold{Warning: 70, Critical: 90}
	updated.Engines = []string{}
	updated.SecurityRequiredChecks = []string{"patch-posture", "certificate-expiry"}
	body, err := json.Marshal(updated)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPut, "http://127.0.0.1:8080/api/config", strings.NewReader(string(body)))
	request.Header.Set("Origin", "http://127.0.0.1:8080")
	request.Header.Set("X-Requested-With", "PerceptronDashboard")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected valid config update to return 200, got %d: %s", response.Code, response.Body.String())
	}
	forged := httptest.NewRequest(http.MethodPut, "http://127.0.0.1:8080/api/config", strings.NewReader(string(body)))
	forged.Header.Set("Origin", "http://attacker.example")
	forged.Header.Set("X-Requested-With", "PerceptronDashboard")
	forgedResponse := httptest.NewRecorder()
	handler.ServeHTTP(forgedResponse, forged)
	if forgedResponse.Code != http.StatusForbidden {
		t.Fatalf("cross-origin configuration write was not rejected: %d", forgedResponse.Code)
	}
	persisted, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Thresholds.CPUPercent.Warning != 70 || len(persisted.Engines) != 0 ||
		len(persisted.SecurityRequiredChecks) != 2 || persisted.SecurityRequiredChecks[0] != "patch-posture" {
		t.Fatalf("configuration update did not persist values: %#v", persisted)
	}

	updated.Engines = []string{"host-health"}
	body, err = json.Marshal(updated)
	if err != nil {
		t.Fatal(err)
	}
	invalid := httptest.NewRequest(http.MethodPut, "http://127.0.0.1:8080/api/config", strings.NewReader(string(body)))
	invalid.Header.Set("Origin", "http://127.0.0.1:8080")
	invalid.Header.Set("X-Requested-With", "PerceptronDashboard")
	rejected := httptest.NewRecorder()
	handler.ServeHTTP(rejected, invalid)
	if rejected.Code != http.StatusBadRequest {
		t.Fatalf("invalid engine dependency graph should be rejected, got %d", rejected.Code)
	}
	if len(store.Current().Engines) != 0 {
		t.Fatal("rejected configuration changed active settings")
	}
}

func TestDashboardRefreshRequiresSameOriginAndRequestedHeader(t *testing.T) {
	dashboard := New(make(chan ToolActivity, 1))
	for _, headers := range []map[string]string{
		{},
		{"Origin": "http://attacker.example", "X-Requested-With": "PerceptronDashboard"},
		{"Origin": "http://127.0.0.1:8080"},
	} {
		request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/api/tools/refresh", nil)
		for name, value := range headers {
			request.Header.Set(name, value)
		}
		response := httptest.NewRecorder()
		dashboard.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Errorf("expected cross-site or unmarked refresh to be rejected, got %d for %#v", response.Code, headers)
		}
	}
}

func TestDashboardRefreshPreventsOverlappingRequests(t *testing.T) {
	refreshRequests := make(chan ToolActivity, 1)
	dashboard := New(refreshRequests)
	if err := dashboard.Update(testCycleReport(t)); err != nil {
		t.Fatal(err)
	}
	handler := dashboard.Handler()
	for attempt := 0; attempt < 2; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/api/tools/refresh", nil)
		request.Header.Set("Origin", "http://127.0.0.1:8080")
		request.Header.Set("X-Requested-With", "PerceptronDashboard")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		expected := http.StatusAccepted
		if attempt == 1 {
			expected = http.StatusConflict
		}
		if response.Code != expected {
			t.Fatalf("attempt %d: expected status %d, got %d", attempt, expected, response.Code)
		}
	}
}

func TestDashboardRestrictsExpectedHostHeader(t *testing.T) {
	dashboard := New()
	if err := dashboard.SetAllowedHost("127.0.0.1:8080"); err != nil {
		t.Fatal(err)
	}
	handler := dashboard.Handler()
	for host, want := range map[string]int{
		"127.0.0.1:8080":      http.StatusServiceUnavailable,
		"rebind.example:8080": http.StatusForbidden,
	} {
		request := httptest.NewRequest(http.MethodGet, "http://"+host+"/api/report", nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != want {
			t.Errorf("Host %q: expected %d, got %d", host, want, response.Code)
		}
	}
}

func TestDashboardRejectsUnknownPathsAndNonGetMethods(t *testing.T) {
	handler := New().Handler()
	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("expected unknown route to return 404, got %d", unknown.Code)
	}
	method := httptest.NewRecorder()
	handler.ServeHTTP(method, httptest.NewRequest(http.MethodPost, "/api/report", nil))
	if method.Code != http.StatusMethodNotAllowed || method.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("expected GET-only API, got status=%d headers=%v", method.Code, method.Header())
	}
	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected readiness endpoint to show not ready, got %d", health.Code)
	}
}

func TestDashboardSupportsReportWithoutEnabledMissionEngine(t *testing.T) {
	if err := New().Update(report.CycleReport{Runtime: engine.CycleResult{CycleID: "cycle"}}); err != nil {
		t.Fatalf("engine-free cycle should be displayed: %v", err)
	}
}

func testCycleReport(t *testing.T) report.CycleReport {
	t.Helper()
	mission := &model.MissionAssessment{
		Scope:       model.EntityRef{Type: "host", ID: "test-host"},
		Objective:   "test host mission",
		Status:      model.MissionGreen,
		Disposition: "continue",
		EvaluatedAt: time.Now().UTC(),
	}
	return report.CycleReport{
		HostID: "test-host",
		Runtime: engine.CycleResult{
			CycleID:     "cycle-dashboard-test",
			CompletedAt: time.Now().UTC(),
			Mission:     mission,
			Events:      []model.Event{},
		},
	}
}
