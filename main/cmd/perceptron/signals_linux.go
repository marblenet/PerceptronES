package main

import (
	"os"
	"os/signal"
	"syscall"
)

func shutdownSignals() (chan os.Signal, func()) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	return signals, func() {
		signal.Stop(signals)
	}
}
