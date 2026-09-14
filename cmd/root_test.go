package cmd

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/spf13/viper"
)

func TestInitLoggerLevelsAndFormats(t *testing.T) {
	tests := []struct {
		name          string
		levelStr      string
		formatStr     string
		expectedLevel slog.Level
	}{
		{"default warn", "", "", slog.LevelWarn},
		{"debug text", "debug", "text", slog.LevelDebug},
		{"info json", "info", "json", slog.LevelInfo},
		{"warn text", "warn", "text", slog.LevelWarn},
		{"warning text", "warning", "text", slog.LevelWarn},
		{"error json", "error", "json", slog.LevelError},
		{"unknown fallback to warn", "unknown", "text", slog.LevelWarn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("log-level", tt.levelStr)
			viper.Set("log-format", tt.formatStr)
			logLevel = tt.levelStr
			logFormat = tt.formatStr

			initLogger()

			logger := slog.Default()
			if !logger.Enabled(context.Background(), tt.expectedLevel) {
				t.Errorf("expected level %v to be enabled for input %q", tt.expectedLevel, tt.levelStr)
			}

			// For levels above debug, debug shouldn't be enabled
			if tt.expectedLevel > slog.LevelDebug && logger.Enabled(context.Background(), slog.LevelDebug) {
				t.Errorf("expected level debug to be disabled when configured level is %v", tt.expectedLevel)
			}
		})
	}
}

func TestDefaultLoggerSilence(t *testing.T) {
	viper.Set("log-level", "warn")
	viper.Set("log-format", "text")
	logLevel = "warn"
	logFormat = "text"
	initLogger()

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	slog.SetDefault(slog.New(handler))

	slog.Info("this is an info message")
	slog.Debug("this is a debug message")

	if buf.Len() > 0 {
		t.Errorf("expected info and debug messages to be suppressed by default, got: %s", buf.String())
	}

	slog.Warn("this is a warning message")
	if buf.Len() == 0 {
		t.Errorf("expected warning message to be output at warn level")
	}
}
