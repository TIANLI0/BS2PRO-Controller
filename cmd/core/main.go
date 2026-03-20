package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var app *CoreApp

	defer func() {
		if r := recover(); r != nil {
			capturePanic(app, "main", r)

			if app != nil {
				func() {
					defer func() {
						if stopPanic := recover(); stopPanic != nil {
							capturePanic(app, "main.Stop", stopPanic)
						}
					}()
					app.Stop()
				}()
			}

			os.Exit(1)
		}
	}()

	// Parse command line arguments
	debugMode := false
	isAutoStart := false

	for _, arg := range os.Args {
		switch arg {
		case "--debug", "/debug", "-debug":
			debugMode = true
		case "--autostart", "/autostart", "-autostart":
			isAutoStart = true
		}
	}

	// Create core application
	app = NewCoreApp(debugMode, isAutoStart)

	// Start application
	if err := app.Start(); err != nil {
		panic(fmt.Sprintf("Failed to start core service: %v", err))
	}

	// Wait for exit signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		app.logInfo("Received system exit signal")
	case <-app.quitChan:
		app.logInfo("Received application quit request")
	}

	app.Stop()
}
