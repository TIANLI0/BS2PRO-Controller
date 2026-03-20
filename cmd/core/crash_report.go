package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/TIANLI0/BS2PRO-Controller/internal/config"
)

func capturePanic(app *CoreApp, source string, recovered any) string {
	stack := debug.Stack()
	logDir := resolveCrashLogDir(app)

	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create crash log directory: %v\n", err)
		fmt.Fprintf(os.Stderr, "panic source: %s, panic: %v\n%s\n", source, recovered, string(stack))
		return ""
	}

	fileName := fmt.Sprintf("crash_%s.log", time.Now().Format("2006-01-02_15-04-05.000"))
	filePath := filepath.Join(logDir, fileName)

	var builder strings.Builder
	builder.WriteString("=== BS2PRO Core Crash Report ===\n")
	builder.WriteString(fmt.Sprintf("time: %s\n", time.Now().Format(time.RFC3339Nano)))
	builder.WriteString(fmt.Sprintf("source: %s\n", source))
	builder.WriteString(fmt.Sprintf("panic: %v\n", recovered))
	builder.WriteString(fmt.Sprintf("pid: %d\n", os.Getpid()))
	builder.WriteString(fmt.Sprintf("args: %v\n", os.Args))
	builder.WriteString("\n--- stack ---\n")
	builder.Write(stack)
	builder.WriteString("\n")

	if err := os.WriteFile(filePath, []byte(builder.String()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write crash report: %v\n", err)
		fmt.Fprintf(os.Stderr, "panic source: %s, panic: %v\n%s\n", source, recovered, string(stack))
		return ""
	}

	if app != nil {
		app.logError("[%s] Caught panic: %v", source, recovered)
		app.logError("[%s] Panic stack trace:\n%s", source, string(stack))
		if app.logger != nil {
			app.logger.Close()
		}
	}

	fmt.Fprintf(os.Stderr, "Program panic occurred, crash report written to: %s\n", filePath)
	return filePath
}

func resolveCrashLogDir(app *CoreApp) string {
	if app != nil && app.logger != nil {
		if logDir := app.logger.GetLogDir(); logDir != "" {
			return logDir
		}
	}

	installDir := config.GetInstallDir()
	if installDir == "" {
		return "logs"
	}

	return filepath.Join(installDir, "logs")
}
