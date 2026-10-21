package telemetry

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestEmitWritesStructuredJSON(t *testing.T) {
	var output bytes.Buffer
	logger := NewLogger(&output)
	if err := logger.Emit("info", "cycle complete", map[string]interface{}{"cycle_id": "cycle-1"}); err != nil {
		t.Fatal(err)
	}
	var record map[string]interface{}
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("log record is not JSON: %v", err)
	}
	if record["level"] != "info" || record["message"] != "cycle complete" || record["cycle_id"] != "cycle-1" {
		t.Fatalf("unexpected structured log: %#v", record)
	}
}
