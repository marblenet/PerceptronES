package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"perceptronsre/internal/config"
	"perceptronsre/internal/model"
)

const PluginProtocolVersion = "perceptron-engine-v1"

const (
	pluginOutputLimit = 8 * 1024 * 1024
	pluginErrorLimit  = 64 * 1024
)

type PluginRequest struct {
	ProtocolVersion string          `json:"protocol_version"`
	Name            string          `json:"name"`
	Version         string          `json:"version"`
	Role            string          `json:"role"`
	CycleID         string          `json:"cycle_id"`
	ConfigurationID string          `json:"configuration_id"`
	Settings        json.RawMessage `json:"settings,omitempty"`
	Inputs          []model.Event   `json:"inputs"`
}

type PluginResponse struct {
	ProtocolVersion string        `json:"protocol_version"`
	Outputs         []model.Event `json:"outputs"`
	Error           string        `json:"error,omitempty"`
}

type processEngine struct {
	plugin  config.Plugin
	timeout time.Duration
}

func newProcessEngine(plugin config.Plugin) (*processEngine, error) {
	timeout, err := time.ParseDuration(plugin.Timeout)
	if err != nil {
		return nil, fmt.Errorf("parse timeout: %w", err)
	}
	return &processEngine{plugin: plugin, timeout: timeout}, nil
}

func (engine *processEngine) Name() string    { return engine.plugin.Name }
func (engine *processEngine) Version() string { return engine.plugin.Version }
func (engine *processEngine) Role() string    { return engine.plugin.Role }
func (engine *processEngine) Consumes() []string {
	return append([]string(nil), engine.plugin.Consumes...)
}
func (engine *processEngine) Produces() []string {
	return append([]string(nil), engine.plugin.Produces...)
}

func (engine *processEngine) Process(ctx context.Context, inputs []model.Event) ([]model.Event, error) {
	cycleID := ""
	configurationID := ""
	if len(inputs) > 0 {
		cycleID = inputs[0].CorrelationID
		configurationID = inputs[0].ConfigurationID
	}
	request := PluginRequest{
		ProtocolVersion: PluginProtocolVersion,
		Name:            engine.Name(),
		Version:         engine.Version(),
		Role:            engine.Role(),
		CycleID:         cycleID,
		ConfigurationID: configurationID,
		Settings:        append(json.RawMessage(nil), engine.plugin.Settings...),
		Inputs:          inputs,
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode plugin request: %w", err)
	}
	pluginContext, cancel := context.WithTimeout(ctx, engine.timeout)
	defer cancel()
	command := exec.CommandContext(pluginContext, engine.plugin.Command, engine.plugin.Args...)
	command.Stdin = bytes.NewReader(encoded)
	stdout := &limitedBuffer{limit: pluginOutputLimit}
	stderr := &limitedBuffer{limit: pluginErrorLimit}
	command.Stdout = stdout
	command.Stderr = stderr
	runErr := command.Run()
	if stdout.exceeded {
		return nil, fmt.Errorf("plugin %s exceeded the %d-byte response limit", engine.Name(), pluginOutputLimit)
	}
	if stderr.exceeded {
		return nil, fmt.Errorf("plugin %s exceeded the %d-byte stderr limit", engine.Name(), pluginErrorLimit)
	}
	if runErr != nil {
		if pluginContext.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("plugin %s exceeded timeout %s", engine.Name(), engine.timeout)
		}
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("plugin %s failed: %w: %s", engine.Name(), runErr, stderr.String())
		}
		return nil, fmt.Errorf("plugin %s failed: %w", engine.Name(), runErr)
	}
	var response PluginResponse
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("decode plugin %s response: %w", engine.Name(), err)
	}
	if response.ProtocolVersion != PluginProtocolVersion {
		return nil, fmt.Errorf("plugin %s returned unsupported protocol version %q", engine.Name(), response.ProtocolVersion)
	}
	if response.Error != "" {
		return nil, fmt.Errorf("plugin %s reported an error: %s", engine.Name(), response.Error)
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, errors.New("plugin response contains multiple JSON values")
		}
		return nil, fmt.Errorf("decode plugin %s trailing response data: %w", engine.Name(), err)
	}
	return response.Outputs, nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (buffer *limitedBuffer) Write(value []byte) (int, error) {
	remaining := buffer.limit - buffer.Len()
	if remaining <= 0 {
		buffer.exceeded = true
		return 0, errors.New("output limit exceeded")
	}
	if len(value) > remaining {
		buffer.exceeded = true
		_, _ = buffer.Buffer.Write(value[:remaining])
		return remaining, errors.New("output limit exceeded")
	}
	return buffer.Buffer.Write(value)
}
