package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"perceptronsre/internal/config"
	"perceptronsre/internal/dashboard"
	"perceptronsre/internal/engine"
	"perceptronsre/internal/model"
	"perceptronsre/internal/observe"
	"perceptronsre/internal/report"
	"perceptronsre/internal/telemetry"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("perceptron", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "perceptron.json", "path to a live-reloadable JSON configuration file")
	outputFormat := flags.String("format", "json", "output format: json or text")
	watch := flags.Bool("watch", false, "collect repeatedly until interrupted")
	interval := flags.Duration("interval", 15*time.Second, "delay between watch cycles")
	web := flags.Bool("web", false, "serve a live read-only dashboard on localhost (implies --watch)")
	webPort := flags.Int("web-port", 8080, "dashboard port; overrides dashboard.port in the config file (0 selects an available port)")
	webAddr := flags.String("web-addr", "", "dashboard listen IP; overrides dashboard.listen_address in the config file (default 127.0.0.1)")
	printDefaults := flags.Bool("print-default-config", false, "print the default JSON configuration and exit")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *printDefaults {
		if err := config.WriteDefault(stdout); err != nil {
			fmt.Fprintf(stderr, "write default configuration: %v\n", err)
			return 1
		}
		return 0
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected positional arguments: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}
	if *outputFormat != "json" && *outputFormat != "text" {
		fmt.Fprintf(stderr, "invalid --format %q: expected json or text\n", *outputFormat)
		return 2
	}
	if *watch && *interval <= 0 {
		fmt.Fprintln(stderr, "--interval must be positive when --watch is enabled")
		return 2
	}
	if *web && *interval <= 0 {
		fmt.Fprintln(stderr, "--interval must be positive when --web is enabled")
		return 2
	}
	if *web && (*webPort < 0 || *webPort > 65535) {
		fmt.Fprintln(stderr, "--web-port must be between 0 and 65535")
		return 2
	}
	cfg, err := config.LoadOrCreate(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load configuration: %v\n", err)
		return 2
	}
	configStore, err := config.NewStore(*configPath, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "initialize configuration store: %v\n", err)
		return 1
	}
	collector, err := observe.NewHostCollector(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "initialize collector: %v\n", err)
		return 1
	}
	runtime, err := engine.NewRuntime(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "initialize engine runtime: %v\n", err)
		return 1
	}
	logger := telemetry.NewLogger(stderr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var liveDashboard *dashboard.Dashboard
	var dashboardErrors chan error
	var refreshRequests chan dashboard.ToolActivity
	var activeRefresh *dashboard.ToolActivity
	if *web {
		listenAddress, listenPort := cfg.Dashboard.ListenAddress, cfg.Dashboard.Port
		flags.Visit(func(item *flag.Flag) {
			switch item.Name {
			case "web-addr":
				listenAddress = *webAddr
			case "web-port":
				listenPort = *webPort
			}
		})
		listener, err := dashboard.Listen(listenAddress, listenPort)
		if err != nil {
			logFailure(logger, stderr, "start dashboard failed", map[string]interface{}{"error": err.Error()})
			return 1
		}
		loopback := config.DashboardConfig{ListenAddress: listenAddress}.IsLoopback()
		refreshRequests = make(chan dashboard.ToolActivity, 1)
		liveDashboard = dashboard.New(refreshRequests)
		liveDashboard.SetConfigStore(configStore)
		hosts := dashboard.AllowedHostsFor(listener.Addr(), cfg.Dashboard.AllowedHosts)
		if err := liveDashboard.SetAllowedHost(listener.Addr().String()); err != nil {
			listener.Close()
			logFailure(logger, stderr, "configure dashboard host restriction failed", map[string]interface{}{"error": err.Error()})
			return 1
		}
		liveDashboard.SetExtraAllowedHosts(hosts)
		liveDashboard.SetConfigWritesDisabled(!loopback && !cfg.Dashboard.AllowRemoteConfig)
		server := &http.Server{
			Handler:           liveDashboard.Handler(),
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       30 * time.Second,
		}
		dashboardErrors = make(chan error, 1)
		go func() {
			dashboardErrors <- server.Serve(listener)
		}()
		defer func() {
			shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			if err := server.Shutdown(shutdownContext); err != nil {
				logFailure(logger, stderr, "stop dashboard failed", map[string]interface{}{"error": err.Error()})
			}
		}()
		startup := map[string]interface{}{
			"url": listener.Addr().String(),
		}
		level := "info"
		if loopback {
			startup["note"] = "bound to loopback only"
		} else {
			level = "warn"
			startup["note"] = "bound to a non-loopback address; the dashboard has no authentication or TLS, so restrict access with a firewall or security list"
			startup["config_editing"] = cfg.Dashboard.AllowRemoteConfig
		}
		if err := logger.Emit(level, "local dashboard started", startup); err != nil {
			fmt.Fprintf(stderr, "write dashboard startup log: %v\n", err)
			return 1
		}
	}
	watchMode := *watch || *web
	if watchMode {
		signals, stopSignals := shutdownSignals()
		defer stopSignals()
		go func() {
			select {
			case <-signals:
				cancel()
			case <-ctx.Done():
			}
		}()
	}

	for {
		if _, err := configStore.ReloadValidated(func(candidate config.Config) error {
			_, err := engine.NewRuntime(candidate)
			return err
		}); err != nil {
			logFailure(logger, stderr, "configuration reload rejected; retaining last-known-good values", map[string]interface{}{
				"config_path": *configPath,
				"error":       err.Error(),
			})
		}
		nextConfig := configStore.Current()
		if !reflect.DeepEqual(cfg, nextConfig) {
			nextCollector, err := observe.NewHostCollector(nextConfig)
			if err != nil {
				logFailure(logger, stderr, "apply updated collector configuration failed", map[string]interface{}{"error": err.Error()})
				return 1
			}
			nextRuntime, err := engine.NewRuntime(nextConfig)
			if err != nil {
				logFailure(logger, stderr, "apply updated engine configuration failed", map[string]interface{}{"error": err.Error()})
				return 1
			}
			cfg = nextConfig
			collector = nextCollector
			runtime = nextRuntime
			if err := logger.Emit("info", "live configuration applied", map[string]interface{}{
				"config_path": *configPath,
				"engines":     cfg.Engines,
			}); err != nil {
				fmt.Fprintf(stderr, "write configuration reload log: %v\n", err)
				return 1
			}
		}
		if dashboardErrors != nil {
			select {
			case err := <-dashboardErrors:
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					logFailure(logger, stderr, "dashboard server failed", map[string]interface{}{"error": err.Error()})
					return 1
				}
			default:
			}
		}
		cycleStarted := time.Now()
		collectionStarted := time.Now()
		observations, err := collector.Collect(ctx)
		collectionDuration := time.Since(collectionStarted)
		if err != nil {
			if ctx.Err() != nil {
				return 0
			}
			if activeRefresh != nil {
				liveDashboard.CompleteTool(activeRefresh.ID, "", err)
				activeRefresh = nil
			}
			logFailure(logger, stderr, "observation collection failed", map[string]interface{}{"error": err.Error()})
			return 1
		}
		cycleID, err := model.NewID()
		if err != nil {
			if activeRefresh != nil {
				liveDashboard.CompleteTool(activeRefresh.ID, "", err)
				activeRefresh = nil
			}
			logFailure(logger, stderr, "generate cycle ID failed", map[string]interface{}{"error": err.Error()})
			return 1
		}
		result, err := runtime.Run(ctx, cycleID, observations)
		if err != nil {
			if ctx.Err() != nil {
				return 0
			}
			if activeRefresh != nil {
				liveDashboard.CompleteTool(activeRefresh.ID, cycleID, err)
				activeRefresh = nil
			}
			logFailure(logger, stderr, "assessment cycle failed", map[string]interface{}{"cycle_id": cycleID, "error": err.Error()})
			return 1
		}
		cycleReport := makeReport(cfg.HostID, observations, collectionDuration, result)
		if liveDashboard != nil {
			if err := liveDashboard.Update(cycleReport); err != nil {
				if activeRefresh != nil {
					liveDashboard.CompleteTool(activeRefresh.ID, cycleID, err)
					activeRefresh = nil
				}
				logFailure(logger, stderr, "update dashboard report failed", map[string]interface{}{"cycle_id": cycleID, "error": err.Error()})
				return 1
			}
			if activeRefresh != nil {
				liveDashboard.CompleteTool(activeRefresh.ID, cycleID, nil)
				activeRefresh = nil
			}
		} else {
			if *outputFormat == "text" {
				err = report.WriteText(stdout, cycleReport)
			} else {
				err = report.WriteJSON(stdout, cycleReport)
			}
			if err != nil {
				logFailure(logger, stderr, "write cycle report failed", map[string]interface{}{"cycle_id": cycleID, "error": err.Error()})
				return 1
			}
		}
		level := "info"
		missionStatus := string(model.MissionUnknown)
		if result.Mission != nil {
			missionStatus = string(result.Mission.Status)
		}
		if result.Mission == nil || result.Mission.Status != model.MissionGreen {
			level = "warn"
		}
		if err := logger.Emit(level, "assessment cycle completed", map[string]interface{}{
			"cycle_id":                 cycleID,
			"host_id":                  cfg.HostID,
			"mission_status":           missionStatus,
			"observation_count":        len(observations),
			"available_observations":   cycleReport.AvailableObservations,
			"unavailable_observations": cycleReport.UnavailableObservations,
			"duration_millis":          time.Since(cycleStarted).Milliseconds(),
			"goroutines":               result.Metrics.Goroutines,
			"heap_alloc_bytes":         result.Metrics.HeapAllocBytes,
		}); err != nil {
			fmt.Fprintf(stderr, "write structured completion log: %v\n", err)
			return 1
		}
		if !watchMode {
			return 0
		}
		timer := time.NewTimer(*interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return 0
		case <-timer.C:
		case request := <-refreshRequests:
			liveDashboard.MarkToolStarted(request.ID)
			activeRefresh = &request
			timer.Stop()
			continue
		case err := <-dashboardErrors:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				logFailure(logger, stderr, "dashboard server failed", map[string]interface{}{"error": err.Error()})
				return 1
			}
		}
	}
}

func logFailure(logger *telemetry.Logger, stderr io.Writer, message string, fields map[string]interface{}) {
	if err := logger.Emit("error", message, fields); err != nil {
		fmt.Fprintf(stderr, "%s; also failed to write structured log: %v\n", message, err)
	}
}

func makeReport(hostID string, observations []model.Observation, collectionDuration time.Duration, result engine.CycleResult) report.CycleReport {
	available := 0
	for _, observation := range observations {
		if observation.Available {
			available++
		}
	}
	return report.CycleReport{
		HostID:                   hostID,
		CollectionDurationMillis: collectionDuration.Milliseconds(),
		AvailableObservations:    available,
		UnavailableObservations:  len(observations) - available,
		Runtime:                  result,
	}
}
