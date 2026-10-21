package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"perceptronsre/internal/model"
)

func WritePrometheus(writer io.Writer, cycle CycleReport) error {
	var output strings.Builder
	fmt.Fprintln(&output, "# HELP perceptron_observation_available Whether an observation is available.")
	fmt.Fprintln(&output, "# TYPE perceptron_observation_available gauge")
	fmt.Fprintln(&output, "# HELP perceptron_observation_value Latest numeric observation value.")
	fmt.Fprintln(&output, "# TYPE perceptron_observation_value gauge")
	fmt.Fprintln(&output, "# HELP perceptron_component_health Current component finding, encoded as a one-hot state.")
	fmt.Fprintln(&output, "# TYPE perceptron_component_health gauge")
	fmt.Fprintln(&output, "# HELP perceptron_mission_status Current mission finding, encoded as a one-hot state.")
	fmt.Fprintln(&output, "# TYPE perceptron_mission_status gauge")
	fmt.Fprintln(&output, "# HELP perceptron_engine_finding Current engine finding event.")
	fmt.Fprintln(&output, "# TYPE perceptron_engine_finding gauge")
	fmt.Fprintln(&output, "# HELP perceptron_engine_runs_total Engine invocations in the latest assessment cycle.")
	fmt.Fprintln(&output, "# TYPE perceptron_engine_runs_total gauge")
	fmt.Fprintln(&output, "# HELP perceptron_engine_failures_total Engine failures in the latest assessment cycle.")
	fmt.Fprintln(&output, "# TYPE perceptron_engine_failures_total gauge")
	fmt.Fprintln(&output, "# HELP perceptron_engine_duration_milliseconds Engine execution time in the latest assessment cycle.")
	fmt.Fprintln(&output, "# TYPE perceptron_engine_duration_milliseconds gauge")
	fmt.Fprintln(&output, "# HELP perceptron_assessment_events Current-cycle events published.")
	fmt.Fprintln(&output, "# TYPE perceptron_assessment_events gauge")

	host := prometheusLabel(cycle.HostID)
	for _, event := range cycle.Runtime.Events {
		switch event.Type {
		case model.TypeObservation:
			var observation model.Observation
			if err := json.Unmarshal(event.Payload, &observation); err != nil {
				return fmt.Errorf("decode observation event %s for Prometheus output: %w", event.ID, err)
			}
			available := 0
			if observation.Available {
				available = 1
			}
			labels := fmt.Sprintf("host_id=%s,metric=%s,component=%s,scope=%s,unit=%s",
				host, prometheusLabel(observation.Metric), prometheusLabel(observation.Component),
				prometheusLabel(observation.ScopeID), prometheusLabel(observation.Unit))
			fmt.Fprintf(&output, "perceptron_observation_available{%s} %d\n", labels, available)
			if observation.Value != nil {
				fmt.Fprintf(&output, "perceptron_observation_value{%s} %s\n", labels, strconv.FormatFloat(*observation.Value, 'g', -1, 64))
			}
		case model.TypeHealth:
			var health model.HealthAssessment
			if err := json.Unmarshal(event.Payload, &health); err != nil {
				return fmt.Errorf("decode health event %s for Prometheus output: %w", event.ID, err)
			}
			fmt.Fprintf(&output, "perceptron_component_health{host_id=%s,component=%s,scope=%s,state=%s} 1\n",
				host, prometheusLabel(health.Component), prometheusLabel(health.ScopeID), prometheusLabel(string(health.State)))
			writeEngineFinding(&output, host, event, string(health.State), health.Component, health.ScopeID)
		case model.TypePressure:
			var pressure model.PressureAssessment
			if err := json.Unmarshal(event.Payload, &pressure); err != nil {
				return fmt.Errorf("decode pressure event %s for Prometheus output: %w", event.ID, err)
			}
			writeEngineFinding(&output, host, event, string(pressure.State), pressure.Component, eventScope(event))
		case model.TypeMission:
			var mission model.MissionAssessment
			if err := json.Unmarshal(event.Payload, &mission); err != nil {
				return fmt.Errorf("decode mission event %s for Prometheus output: %w", event.ID, err)
			}
			fmt.Fprintf(&output, "perceptron_mission_status{host_id=%s,state=%s} 1\n", host, prometheusLabel(string(mission.Status)))
			writeEngineFinding(&output, host, event, string(mission.Status), mission.Scope.Type, mission.Scope.ID)
		default:
			if event.Classification.ReasoningKind == model.ReasoningCalculated {
				writeEngineFinding(&output, host, event, "reported", "", eventScope(event))
			}
		}
	}
	for name, metrics := range cycle.Runtime.Metrics.Engines {
		engine := prometheusLabel(name)
		fmt.Fprintf(&output, "perceptron_engine_runs_total{host_id=%s,engine=%s} %d\n", host, engine, metrics.Runs)
		fmt.Fprintf(&output, "perceptron_engine_failures_total{host_id=%s,engine=%s} %d\n", host, engine, metrics.Failures)
		fmt.Fprintf(&output, "perceptron_engine_duration_milliseconds{host_id=%s,engine=%s} %d\n", host, engine, metrics.DurationMillis)
	}
	fmt.Fprintf(&output, "perceptron_assessment_events{host_id=%s} %d\n", host, len(cycle.Runtime.Events))
	if _, err := io.WriteString(writer, output.String()); err != nil {
		return fmt.Errorf("write Prometheus metrics: %w", err)
	}
	return nil
}

func writeEngineFinding(output *strings.Builder, host string, event model.Event, state, category, scope string) {
	fmt.Fprintf(output, "perceptron_engine_finding{host_id=%s,engine=%s,event_type=%s,state=%s,category=%s,scope=%s} 1\n",
		host, prometheusLabel(event.Source), prometheusLabel(event.Type), prometheusLabel(state),
		prometheusLabel(category), prometheusLabel(scope))
}

func eventScope(event model.Event) string {
	for _, entity := range event.EntityRefs {
		if entity.Type != "host" {
			return entity.ID
		}
	}
	return ""
}

func prometheusLabel(value string) string {
	return strconv.Quote(value)
}
