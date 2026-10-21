package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"perceptronsre/internal/engine"
	"perceptronsre/internal/model"
)

type CycleReport struct {
	HostID                   string             `json:"host_id"`
	CollectionDurationMillis int64              `json:"collection_duration_millis"`
	AvailableObservations    int                `json:"available_observations"`
	UnavailableObservations  int                `json:"unavailable_observations"`
	Runtime                  engine.CycleResult `json:"runtime"`
}

func WriteJSON(writer io.Writer, cycle CycleReport) error {
	encoder := json.NewEncoder(writer)
	if err := encoder.Encode(cycle); err != nil {
		return fmt.Errorf("encode cycle report: %w", err)
	}
	return nil
}

func WriteText(writer io.Writer, cycle CycleReport) error {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Perceptron SRE v%s — host %s\n", model.CoreVersion, cycle.HostID)
	mission := cycle.Runtime.Mission
	if mission == nil {
		fmt.Fprintln(&builder, "Mission: UNKNOWN (core mission assessment unavailable)")
	} else {
		fmt.Fprintf(&builder, "Mission: %s (%s) — %s\n", mission.Status, mission.Disposition, mission.Objective)
		fmt.Fprintf(&builder, "Coverage: %d/%d required component assessments represented; complete=%t; freshness=%s\n",
			len(mission.Coverage.AssessedComponents), len(mission.Coverage.RequiredComponents), mission.Coverage.Complete, mission.Freshness)
		fmt.Fprintf(&builder, "Explanation: %s\n", mission.Explanation)
	}
	fmt.Fprintln(&builder, "Component assessments:")
	if mission == nil || len(mission.Components) == 0 {
		fmt.Fprintln(&builder, "  - none available")
	}
	if mission != nil {
		for _, component := range mission.Components {
			fmt.Fprintf(&builder, "  - %s (%s): %s; evidence=%s; freshness=%s; %s\n",
				component.Component, component.ScopeID, component.State, component.Coverage, component.Freshness, component.Explanation)
		}
	}
	if mission != nil && len(mission.Coverage.MissingComponents) > 0 {
		fmt.Fprintf(&builder, "Missing assessments: %s\n", strings.Join(mission.Coverage.MissingComponents, ", "))
	}
	fmt.Fprintf(&builder, "Observations: %d available, %d unavailable\n", cycle.AvailableObservations, cycle.UnavailableObservations)
	for _, event := range cycle.Runtime.Events {
		if event.Type != model.TypeObservation {
			continue
		}
		var observation model.Observation
		if err := json.Unmarshal(event.Payload, &observation); err != nil {
			return fmt.Errorf("decode observation event %s for text report: %w", event.ID, err)
		}
		if !observation.Available {
			fmt.Fprintf(&builder, "  - %s: UNKNOWN (%s)\n", observation.Metric, observation.Reason)
			continue
		}
		fmt.Fprintf(&builder, "  - %s: %.2f %s\n", observation.Metric, *observation.Value, observation.Unit)
	}
	if _, err := io.WriteString(writer, builder.String()); err != nil {
		return fmt.Errorf("write text report: %w", err)
	}
	return nil
}
