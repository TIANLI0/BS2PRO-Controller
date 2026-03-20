// Package logger provides zap-based logging functionality
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// CustomLogger zap logger wrapper
type CustomLogger struct {
	logger    *zap.Logger
	sugar     *zap.SugaredLogger
	debugMode bool
	logDir    string
	atom      zap.AtomicLevel
}

// NewCustomLogger creates a new logger
func NewCustomLogger(debugMode bool, installDir string) (*CustomLogger, error) {
	logDir := filepath.Join(installDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	// Main log file path
	logFilePath := filepath.Join(logDir, fmt.Sprintf("app_%s.log", time.Now().Format("2006-01-02")))
	debugFilePath := filepath.Join(logDir, fmt.Sprintf("debug_%s.log", time.Now().Format("2006-01-02")))

	// Create log rotation config
	appLogRotate := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    10, // MB
		MaxBackups: 7,
		MaxAge:     7, // days
		Compress:   true,
	}

	debugLogRotate := &lumberjack.Logger{
		Filename:   debugFilePath,
		MaxSize:    10,
		MaxBackups: 7,
		MaxAge:     7,
		Compress:   true,
	}

	// Encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	consoleEncoderConfig := encoderConfig
	consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	// Set log level
	atom := zap.NewAtomicLevel()
	if debugMode {
		atom.SetLevel(zapcore.DebugLevel)
	} else {
		atom.SetLevel(zapcore.InfoLevel)
	}

	// Create multiple cores
	consoleEncoder := zapcore.NewConsoleEncoder(consoleEncoderConfig)
	fileEncoder := zapcore.NewJSONEncoder(encoderConfig)

	appCore := zapcore.NewCore(
		fileEncoder,
		zapcore.AddSync(appLogRotate),
		zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= zapcore.InfoLevel
		}),
	)

	debugCore := zapcore.NewCore(
		fileEncoder,
		zapcore.AddSync(debugLogRotate),
		atom,
	)

	// Console output core
	consoleCore := zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(os.Stdout),
		atom,
	)

	// Merge cores
	core := zapcore.NewTee(appCore, debugCore, consoleCore)

	// Create logger
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	sugar := logger.Sugar()

	return &CustomLogger{
		logger:    logger,
		sugar:     sugar,
		debugMode: debugMode,
		logDir:    logDir,
		atom:      atom,
	}, nil
}

// Info logs an info message
func (l *CustomLogger) Info(format string, v ...any) {
	l.sugar.Infof(format, v...)
}

// Error logs an error message
func (l *CustomLogger) Error(format string, v ...any) {
	l.sugar.Errorf(format, v...)
}

// Debug logs a debug message
func (l *CustomLogger) Debug(format string, v ...any) {
	l.sugar.Debugf(format, v...)
}

// Warn logs a warning message
func (l *CustomLogger) Warn(format string, v ...any) {
	l.sugar.Warnf(format, v...)
}

// Fatal logs a fatal error message and exits
func (l *CustomLogger) Fatal(format string, v ...any) {
	l.sugar.Fatalf(format, v...)
}

// Close closes the logger
func (l *CustomLogger) Close() {
	if l.logger != nil {
		l.logger.Sync()
	}
}

// CleanOldLogs cleans up old log files (keeps 7 days)
func (l *CustomLogger) CleanOldLogs() {
	files, err := os.ReadDir(l.logDir)
	if err != nil {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -7) // 7 days ago
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".log") || strings.HasSuffix(file.Name(), ".log.gz") {
			info, err := file.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				os.Remove(filepath.Join(l.logDir, file.Name()))
			}
		}
	}
}

// SetDebugMode sets the debug mode
func (l *CustomLogger) SetDebugMode(enabled bool) {
	l.debugMode = enabled
	if enabled {
		l.atom.SetLevel(zapcore.DebugLevel)
	} else {
		l.atom.SetLevel(zapcore.InfoLevel)
	}
}

// GetLogDir gets the log directory
func (l *CustomLogger) GetLogDir() string {
	return l.logDir
}

// GetDebugMode gets the debug mode status
func (l *CustomLogger) GetDebugMode() bool {
	return l.debugMode
}

// GetZapLogger gets the underlying zap logger
func (l *CustomLogger) GetZapLogger() *zap.Logger {
	return l.logger
}

// GetSugar gets the sugar logger
func (l *CustomLogger) GetSugar() *zap.SugaredLogger {
	return l.sugar
}
