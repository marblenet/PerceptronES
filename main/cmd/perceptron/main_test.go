package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"perceptronsre/internal/config"
	"perceptronsre/internal/report"
)

func TestRunPrintsDefaultConfiguration(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--print-default-config"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run returned %d: %s", code, stderr.String())
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &cfg); err != nil {
		t.Fatalf("default configuration is invalid JSON: %v", err)
	}
}

func TestRunRejectsInvalidFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--format", "yaml"}, &stdout, &stderr); code != 2 {
		t.Fatalf("expected usage error exit 2, got %d", code)
	}
}

func TestRunRejectsInvalidDashboardPort(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--web", "--web-port", "70000"}, &stdout, &stderr); code != 2 {
		t.Fatalf("expected invalid dashboard port to return usage error, got %d", code)
	}
}

func TestRunHelpReturnsSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected help to exit successfully, got %d", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("Usage of perceptron")) {
		t.Fatalf("help output is missing usage information: %s", stderr.String())
	}
}

func TestRunOnceEmitsAssessmentReport(t *testing.T) {
	var stdout, stderr bytes.Buffer
	configPath := temporaryRunConfigPath(t)
	defer os.Remove(configPath)
	code := run([]string{"--config", configPath, "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run returned %d: %s", code, stderr.String())
	}
	var cycle report.CycleReport
	if err := json.Unmarshal(stdout.Bytes(), &cycle); err != nil {
		t.Fatalf("stdout did not contain a valid cycle report: %v\n%s", err, stdout.String())
	}
	if cycle.Runtime.Mission == nil || cycle.Runtime.Mission.Status == "" {
		t.Fatalf("cycle report has no mission assessment: %#v", cycle)
	}
	if len(cycle.Runtime.Events) == 0 {
		t.Fatal("cycle report contains no events")
	}
}

func TestRunSupportsEmptyEngineSelection(t *testing.T) {
	configPath := temporaryRunConfigPath(t)
	defer os.Remove(configPath)
	cfg := config.Default()
	cfg.Engines = []string{}
	file, err := os.Create(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(file).Encode(cfg); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--config", configPath, "--format", "text"}, &stdout, &stderr); code != 0 {
		t.Fatalf("engine-free run returned %d: %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Mission: UNKNOWN")) {
		t.Fatalf("engine-free report did not explain missing assessment: %s", stdout.String())
	}
}

func temporaryRunConfigPath(t *testing.T) string {
	t.Helper()
	file, err := ioutil.TempFile("", "perceptron-run-config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	return path
}
