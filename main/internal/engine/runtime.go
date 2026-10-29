package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	goRuntime "runtime"
	"sort"
	"strings"
	"time"

	"perceptronsre/internal/bus"
	"perceptronsre/internal/config"
	"perceptronsre/internal/metrics"
	"perceptronsre/internal/model"
)

type Engine interface {
	Name() string
	Version() string
	Role() string
	Consumes() []string
	Produces() []string
	Process(context.Context, []model.Event) ([]model.Event, error)
}

type EngineMetrics struct {
	Runs           int   `json:"runs"`
	Failures       int   `json:"failures"`
	DurationMillis int64 `json:"duration_millis"`
}

type RuntimeMetrics struct {
	EventsPublished int                      `json:"events_published"`
	Engines         map[string]EngineMetrics `json:"engines"`
	CoreServices    map[string]EngineMetrics `json:"core_services"`
	DurationMillis  int64                    `json:"duration_millis"`
	Goroutines      int                      `json:"goroutines"`
	HeapAllocBytes  uint64                   `json:"heap_alloc_bytes"`
}

type CycleResult struct {
	CycleID     string                   `json:"cycle_id"`
	StartedAt   time.Time                `json:"started_at"`
	CompletedAt time.Time                `json:"completed_at"`
	Events      []model.Event            `json:"events"`
	Mission     *model.MissionAssessment `json:"mission,omitempty"`
	Metrics     RuntimeMetrics           `json:"metrics"`
}

type Runtime struct {
	config          config.Config
	engines         []Engine
	configurationID string
}

func NewRuntime(cfg config.Config) (*Runtime, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid runtime configuration: %w", err)
	}
	plugins := make(map[string]config.Plugin, len(cfg.Plugins))
	for _, plugin := range cfg.Plugins {
		plugins[plugin.Name] = plugin
	}
	engines := make([]Engine, 0, len(cfg.Engines))
	for _, name := range cfg.Engines {
		var current Engine
		var err error
		switch name {
		case "host-pressure":
			current, err = newPressureEngine(cfg)
		case "host-health":
			current = healthEngine{}
		case "host-determination":
			current, err = newDeterminationEngine(cfg.Knowledge)
		case "host-recommendation":
			current = recommendationEngine{}
		case "host-policy":
			current = policyEngine{defaultDecision: cfg.Policy.DefaultDecision, rules: cfg.Policy.Rules}
		case "host-action":
			current = actionEngine{}
		case "host-outcome":
			current, err = newOutcomeEngine(cfg.Knowledge)
		case "host-learning":
			current = learningEngine{}
		case "host-mission":
			continue
		case "security-posture":
			current = securityEngine{}
		default:
			plugin, ok := plugins[name]
			if !ok {
				return nil, fmt.Errorf("engine %q is not registered", name)
			}
			current, err = newProcessEngine(plugin)
		}
		if err != nil {
			return nil, fmt.Errorf("initialize engine %s: %w", name, err)
		}
		engines = append(engines, current)
	}
	if err := validateEngineOrder(engines, []string{model.TypeObservation, model.TypeSecurityObservation, model.TypeCycle}); err != nil {
		return nil, err
	}
	configurationID, err := fingerprintConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("fingerprint runtime configuration: %w", err)
	}
	return &Runtime{config: cfg, engines: engines, configurationID: configurationID}, nil
}

func (runtime *Runtime) Run(ctx context.Context, cycleID string, observations []model.Observation) (CycleResult, error) {
	return runtime.RunWithOrigin(ctx, cycleID, observations, model.OriginLive)
}

func (runtime *Runtime) RunWithOrigin(ctx context.Context, cycleID string, observations []model.Observation, origin string) (CycleResult, error) {
	if strings.TrimSpace(cycleID) == "" {
		return CycleResult{}, errors.New("cycle ID must not be empty")
	}
	switch origin {
	case model.OriginLive, model.OriginSimulation, model.OriginReplay, model.OriginHumanAction:
	default:
		return CycleResult{}, fmt.Errorf("unsupported runtime event origin %q", origin)
	}
	startedAt := time.Now().UTC()
	eventBus := bus.New()
	metrics := RuntimeMetrics{Engines: make(map[string]EngineMetrics), CoreServices: make(map[string]EngineMetrics)}
	cycleEvent, err := model.NewEvent(cycleID, "perceptron-runtime", model.TypeCycle, []model.EntityRef{{Type: "host", ID: runtime.config.HostID}}, nil, model.ReasoningCalculated, model.SafetyInformational, model.CycleContext{HostID: runtime.config.HostID})
	if err != nil {
		return CycleResult{}, fmt.Errorf("create assessment cycle event: %w", err)
	}
	cycleEvent.Origin = origin
	if err := cycleEvent.Validate(); err != nil {
		return CycleResult{}, fmt.Errorf("validate assessment cycle event: %w", err)
	}
	cycleEvent.ProducerVersion = model.CoreVersion
	cycleEvent.ConfigurationID = runtime.configurationID
	if err := eventBus.Publish(ctx, cycleEvent); err != nil {
		return CycleResult{}, fmt.Errorf("publish assessment cycle event: %w", err)
	}
	metrics.EventsPublished++
	for i, observation := range observations {
		if err := validateObservation(observation); err != nil {
			return CycleResult{}, fmt.Errorf("observation %d: %w", i, err)
		}
		entityRefs := []model.EntityRef{{Type: "host", ID: runtime.config.HostID}}
		if observation.ScopeID != "" {
			entityRefs = append(entityRefs, model.EntityRef{Type: observation.Component, ID: observation.ScopeID})
		}
		if observation.Target != "" {
			entityRefs = append(entityRefs, model.EntityRef{Type: "dns-target", ID: observation.Target})
		}
		event, err := model.NewEventWithTimes(observation.ObservedAt, time.Now().UTC(), cycleID, "host-observer", model.TypeObservation, entityRefs, nil, model.ReasoningObserved, model.SafetyInformational, observation)
		if err != nil {
			return CycleResult{}, fmt.Errorf("create observation event %d: %w", i, err)
		}
		event.Origin = origin
		if err := event.Validate(); err != nil {
			return CycleResult{}, fmt.Errorf("validate observation event %d: %w", i, err)
		}
		event.ProducerVersion = model.CoreVersion
		event.ConfigurationID = runtime.configurationID
		if err := eventBus.Publish(ctx, event); err != nil {
			return CycleResult{}, fmt.Errorf("publish observation event %s: %w", event.ID, err)
		}
		metrics.EventsPublished++
	}
	for i, evidence := range runtime.config.SecurityEvidence {
		observation := model.SecurityObservation{
			Check:      evidence.Check,
			State:      evidence.State,
			Source:     evidence.Source,
			Summary:    evidence.Summary,
			ObservedAt: evidence.ObservedAt.UTC(),
		}
		event, err := model.NewEventWithTimes(
			evidence.ObservedAt, time.Now().UTC(), cycleID, "security-evidence-config",
			model.TypeSecurityObservation,
			[]model.EntityRef{{Type: "host", ID: runtime.config.HostID}, {Type: "security-check", ID: evidence.Check}},
			nil, model.ReasoningObserved, model.SafetyInformational, observation,
		)
		if err != nil {
			return CycleResult{}, fmt.Errorf("create security evidence event %d: %w", i, err)
		}
		event.Origin = origin
		if err := event.Validate(); err != nil {
			return CycleResult{}, fmt.Errorf("validate security evidence event %d: %w", i, err)
		}
		event.ProducerVersion = model.CoreVersion
		event.ConfigurationID = runtime.configurationID
		if err := eventBus.Publish(ctx, event); err != nil {
			return CycleResult{}, fmt.Errorf("publish security evidence %s: %w", evidence.Check, err)
		}
		metrics.EventsPublished++
	}

	for _, current := range runtime.engines {
		inputs := eventBus.Select(cycleID, current.Consumes())
		if len(inputs) == 0 {
			continue
		}
		engineMetrics := metrics.Engines[current.Name()]
		engineMetrics.Runs++
		engineStarted := time.Now()
		outputs, err := current.Process(ctx, inputs)
		engineMetrics.DurationMillis = time.Since(engineStarted).Milliseconds()
		if err != nil {
			engineMetrics.Failures++
			metrics.Engines[current.Name()] = engineMetrics
			return CycleResult{}, fmt.Errorf("engine %s failed: %w", current.Name(), err)
		}
		for outputIndex, output := range outputs {
			if err := inheritOrigin(inputs, &output); err != nil {
				engineMetrics.Failures++
				metrics.Engines[current.Name()] = engineMetrics
				return CycleResult{}, fmt.Errorf("engine %s output %d origin rejected: %w", current.Name(), outputIndex, err)
			}
			if err := validateOutput(current, cycleID, runtime.configurationID, inputs, output); err != nil {
				engineMetrics.Failures++
				metrics.Engines[current.Name()] = engineMetrics
				return CycleResult{}, fmt.Errorf("engine %s output %d rejected: %w", current.Name(), outputIndex, err)
			}
			if err := eventBus.Publish(ctx, output); err != nil {
				engineMetrics.Failures++
				metrics.Engines[current.Name()] = engineMetrics
				return CycleResult{}, fmt.Errorf("publish output from engine %s: %w", current.Name(), err)
			}
			metrics.EventsPublished++
		}
		metrics.Engines[current.Name()] = engineMetrics
	}
	mission := missionEngine{
		hostID: runtime.config.HostID, dnsProbeName: runtime.config.DNSProbeName,
		securityPostureEnabled: contains(runtime.config.Engines, "security-posture"),
		securityRequiredChecks: append([]string(nil), runtime.config.SecurityRequiredChecks...),
	}
	coreInputs := eventBus.Select(cycleID, mission.Consumes())
	coreMetrics := EngineMetrics{Runs: 1}
	coreStarted := time.Now()
	missionOutputs, err := mission.Process(ctx, coreInputs)
	coreMetrics.DurationMillis = time.Since(coreStarted).Milliseconds()
	if err != nil {
		coreMetrics.Failures++
		metrics.CoreServices[mission.Name()] = coreMetrics
		return CycleResult{}, fmt.Errorf("core mission assessment failed: %w", err)
	}
	for outputIndex, output := range missionOutputs {
		if err := inheritOrigin(coreInputs, &output); err != nil {
			coreMetrics.Failures++
			metrics.CoreServices[mission.Name()] = coreMetrics
			return CycleResult{}, fmt.Errorf("core mission output %d origin rejected: %w", outputIndex, err)
		}
		if err := validateOutput(mission, cycleID, runtime.configurationID, coreInputs, output); err != nil {
			coreMetrics.Failures++
			metrics.CoreServices[mission.Name()] = coreMetrics
			return CycleResult{}, fmt.Errorf("core mission output %d rejected: %w", outputIndex, err)
		}
		if err := eventBus.Publish(ctx, output); err != nil {
			coreMetrics.Failures++
			metrics.CoreServices[mission.Name()] = coreMetrics
			return CycleResult{}, fmt.Errorf("publish core mission output: %w", err)
		}
		metrics.EventsPublished++
	}
	metrics.CoreServices[mission.Name()] = coreMetrics
	completedAt := time.Now().UTC()
	metrics.DurationMillis = completedAt.Sub(startedAt).Milliseconds()
	var memoryStats goRuntime.MemStats
	goRuntime.ReadMemStats(&memoryStats)
	metrics.Goroutines = goRuntime.NumGoroutine()
	metrics.HeapAllocBytes = memoryStats.HeapAlloc
	events := eventBus.Snapshot(cycleID)
	result := CycleResult{
		CycleID:     cycleID,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		Events:      events,
		Metrics:     metrics,
	}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type != model.TypeMission {
			continue
		}
		var mission model.MissionAssessment
		if err := json.Unmarshal(events[i].Payload, &mission); err != nil {
			return CycleResult{}, fmt.Errorf("decode mission assessment event %s: %w", events[i].ID, err)
		}
		result.Mission = &mission
		break
	}
	return result, nil
}

func validateEngineOrder(engines []Engine, initialTypes []string) error {
	available := make(map[string]bool)
	for _, eventType := range initialTypes {
		available[eventType] = true
	}
	names := make(map[string]bool)
	producers := make(map[string]string)
	for _, current := range engines {
		if strings.TrimSpace(current.Name()) == "" || strings.TrimSpace(current.Version()) == "" || strings.TrimSpace(current.Role()) == "" {
			return errors.New("engine name, version, and role are required")
		}
		if names[current.Name()] {
			return fmt.Errorf("duplicate engine name %q", current.Name())
		}
		names[current.Name()] = true
		consumes, produces := current.Consumes(), current.Produces()
		if len(consumes) == 0 || len(produces) == 0 {
			return fmt.Errorf("engine %s must declare consumed and produced event types", current.Name())
		}
		hasAvailableInput := false
		for _, eventType := range consumes {
			if available[eventType] {
				hasAvailableInput = true
			} else if producer, exists := producers[eventType]; exists {
				return fmt.Errorf("engine %s consumes %s after producer %s is already ordered", current.Name(), eventType, producer)
			} else {
				return fmt.Errorf("engine %s consumes %s, which is not available from an initial event or prior engine", current.Name(), eventType)
			}
		}
		if !hasAvailableInput {
			return fmt.Errorf("engine %s has no input produced by the initial events or a prior engine", current.Name())
		}
		for _, eventType := range produces {
			if strings.TrimSpace(eventType) == "" {
				return fmt.Errorf("engine %s declares an empty output type", current.Name())
			}
			if eventType == model.TypeMission {
				return fmt.Errorf("engine %s cannot produce %s: mission assessment is core-owned", current.Name(), model.TypeMission)
			}
			if producer, exists := producers[eventType]; exists {
				return fmt.Errorf("event type %s has multiple engine producers: %s and %s", eventType, producer, current.Name())
			}
			producers[eventType] = current.Name()
			available[eventType] = true
		}
	}
	return nil
}

func validateOutput(current Engine, cycleID, configurationID string, inputs []model.Event, output model.Event) error {
	if err := output.Validate(); err != nil {
		return err
	}
	if output.Source != current.Name() {
		return fmt.Errorf("source %q does not match engine %q", output.Source, current.Name())
	}
	if output.ProducerVersion != current.Version() {
		return fmt.Errorf("producer version %q does not match engine version %q", output.ProducerVersion, current.Version())
	}
	if output.ConfigurationID != configurationID {
		return errors.New("output configuration ID does not match the active configuration")
	}
	if output.CorrelationID != cycleID {
		return errors.New("output correlation ID does not match the active cycle")
	}
	for _, input := range inputs {
		if output.Origin != input.Origin {
			return errors.New("output origin must match its source events")
		}
	}

	if !contains(current.Produces(), output.Type) {
		return fmt.Errorf("undeclared output event type %q", output.Type)
	}
	expectedReasoning := model.ReasoningCalculated
	if categoryEngine, ok := current.(interface{ Category() string }); ok && categoryEngine.Category() == "tap" {
		expectedReasoning = model.ReasoningObserved
	}
	if output.Classification.ReasoningKind != expectedReasoning {
		return fmt.Errorf("engine output must be classified as %s", expectedReasoning)
	}
	inputIDs := make(map[string]bool, len(inputs))
	inputEntities := make(map[model.EntityRef]bool)
	for _, input := range inputs {
		inputIDs[input.ID] = true
		for _, entity := range input.EntityRefs {
			inputEntities[entity] = true
		}
	}
	if len(output.DerivesFrom) == 0 {
		return errors.New("derived output must reference its input events")
	}
	for _, inputID := range output.DerivesFrom {
		if !inputIDs[inputID] {
			return fmt.Errorf("derivation reference %q is not an input to this engine invocation", inputID)
		}
	}
	for _, entity := range output.EntityRefs {
		if !inputEntities[entity] {
			return fmt.Errorf("output entity reference %s:%s is outside the engine input scope", entity.Type, entity.ID)
		}
	}
	if output.CausationID != "" && !inputIDs[output.CausationID] {
		return fmt.Errorf("causation reference %q is not an input to this engine invocation", output.CausationID)
	}
	return nil
}

func inheritOrigin(inputs []model.Event, output *model.Event) error {
	if len(inputs) == 0 {
		return errors.New("derived output has no source events")
	}
	origin := inputs[0].Origin
	for _, input := range inputs[1:] {
		if input.Origin != origin {
			return errors.New("cannot derive a single-origin event from mixed-origin inputs")
		}
	}
	output.Origin = origin
	return nil
}

func fingerprintConfig(cfg config.Config) (string, error) {
	encoded, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func validateObservation(observation model.Observation) error {
	if strings.TrimSpace(observation.Metric) == "" || strings.TrimSpace(observation.Component) == "" || strings.TrimSpace(observation.Unit) == "" {
		return errors.New("metric, component, and unit are required")
	}
	if observation.ObservedAt.IsZero() {
		return errors.New("observed_at is required")
	}
	if observation.Available != (observation.Value != nil) {
		return errors.New("available must match whether value is present")
	}
	if !observation.Available && strings.TrimSpace(observation.Reason) == "" {
		return errors.New("unavailable observations require an explicit reason")
	}
	expectedComponent, expectedUnit, knownMetric := observationContract(observation.Metric)
	if knownMetric && (observation.Component != expectedComponent || observation.Unit != expectedUnit) {
		return fmt.Errorf("%s must use component %q and unit %q", observation.Metric, expectedComponent, expectedUnit)
	}
	if observation.Value != nil && (math.IsNaN(*observation.Value) || math.IsInf(*observation.Value, 0)) {
		return errors.New("observation value must be finite")
	}
	if observation.Value != nil {
		switch observation.Metric {
		case "cpu.utilization_percent", "memory.used_percent", "filesystem.used_percent":
			if *observation.Value < 0 || *observation.Value > 100 {
				return fmt.Errorf("%s must be between 0 and 100", observation.Metric)
			}
		case "system.load_1m", "network.rx_bytes", "network.tx_bytes", "process.count":
			if *observation.Value < 0 {
				return fmt.Errorf("%s must not be negative", observation.Metric)
			}
		case "dns.resolution_success":
			if *observation.Value != 0 && *observation.Value != 1 {
				return errors.New("dns.resolution_success must be 0 or 1")
			}
		default:
			if def, ok := metrics.Lookup(observation.Metric); ok {
				switch {
				case def.Percent && (*observation.Value < 0 || *observation.Value > 100):
					return fmt.Errorf("%s must be between 0 and 100", observation.Metric)
				case def.Boolean && *observation.Value != 0 && *observation.Value != 1:
					return fmt.Errorf("%s must be 0 or 1", observation.Metric)
				case *observation.Value < 0:
					return fmt.Errorf("%s must not be negative", observation.Metric)
				}
			}
		}
	}
	if observation.Metric == "dns.resolution_success" {
		if strings.TrimSpace(observation.Target) == "" || observation.LatencyMillis == nil || *observation.LatencyMillis < 0 {
			return errors.New("DNS resolution observations require a target and non-negative latency")
		}
		if observation.Available && observation.Value != nil && *observation.Value == 0 && strings.TrimSpace(observation.Reason) == "" {
			return errors.New("failed DNS resolution observations require an explicit reason")
		}
	}
	return nil
}

func observationContract(metric string) (string, string, bool) {
	switch metric {
	case "cpu.utilization_percent":
		return "cpu", "percent", true
	case "system.load_1m":
		return "load", "load", true
	case "memory.used_percent":
		return "memory", "percent", true
	case "filesystem.used_percent":
		return "filesystem", "percent", true
	case "network.rx_bytes", "network.tx_bytes":
		return "network", "bytes", true
	case "process.count":
		return "process", "processes", true
	case "dns.resolution_success":
		return "dns", "boolean", true
	default:
		if def, ok := metrics.Lookup(metric); ok {
			return def.Component, def.Unit, true
		}
		return "", "", false
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func sortedUnique(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
