// +build !linux,!windows

package main

import (
	"os"
	"os/signal"
)

func shutdownSignals() (chan os.Signal, func()) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	return signals, func() {
		signal.Stop(signals)
	}
}
