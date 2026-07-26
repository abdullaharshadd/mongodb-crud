```go
package logging_test

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"internal/logging"
)

// ---------------------------------------------------------------------------
// ParseLevel
// ---------------------------------------------------------------------------

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLevel slog.Level
		wantErr   bool
	}{
		// Canonical names
		{name: "TRACE maps to DEBUG", input: "TRACE", wantLevel: slog.LevelDebug},
		{name: "DEBUG maps to DEBUG", input: "DEBUG", wantLevel: slog.LevelDebug},
		{name: "INFO maps to INFO", input: "INFO", wantLevel: slog.LevelInfo},
		{name: "WARN maps to WARN", input: "WARN", wantLevel: slog.LevelWarn},
		{name: "WARNING maps to WARN", input: "WARNING", wantLevel: slog.LevelWarn},
		{name: "ERROR maps to ERROR", input: "ERROR", wantLevel: slog.LevelError},
		{name: "FATAL maps to ERROR", input: "FATAL", wantLevel: slog.LevelError},
		// Case insensitivity
		{name: "lowercase info", input: "info", wantLevel: slog.LevelInfo},
		{name: "mixed case Warn", input: "Warn", wantLevel: slog.LevelWarn},
		{name: "lowercase trace", input: "trace", wantLevel: slog.LevelDebug},
		{name: "lowercase fatal", input: "fatal", wantLevel: slog.LevelError},
		// Surrounding whitespace
		{name: "INFO with leading space", input: "  INFO", wantLevel: slog.LevelInfo},
		{name: "DEBUG with trailing space", input: "DEBUG  ", wantLevel: slog.LevelDebug},
		{name: "WARN with surrounding spaces", input: "  WARN  ", wantLevel: slog.LevelWarn},
		// Error cases
		{name: "empty string", input: "", wantErr: true},
		{name: "unknown level OFF", input: "OFF", wantErr: true},
		{name: "unknown level ALL", input: "ALL", wantErr: true},
		{name: "numeric string", input: "1", wantErr: true},
		{name: "garbage input", input: "garbage", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := logging.ParseLevel(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "unknown level")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantLevel, got)
			}
		})
	}
}

// Invariant: log level ordering TRACE < DEBUG < INFO < WARN < ERROR < FATAL.

// ---------------------------------------------------------------------------
// NewConfig
// ---------------------------------------------------------------------------

func TestNewConfig_Defaults(t *testing.T) {
	cfg := logging.NewConfig()
	require.NotNil(t, cfg)

	assert.Equal(t, logging.DefaultLevel, cfg.Level, "root level must be INFO")
	assert.Equal(t, logging.FormatText, cfg.Format)
	assert.True(t, cfg.AddSource, "caller source must be enabled by default")
	assert.Equal(t, logging.DefaultLogFilePath, cfg.FilePath)
	assert.Equal(t, logging.DefaultDatePattern, cfg.DatePattern)

	// Destinations: CA and DRFA only (FA is inactive, as in the original XML).
	require.Len(t, cfg.Destinations, 2)
	assert.Contains(t, cfg.Destinations, logging.DestinationConsole)
	assert.Contains(t, cfg.Destinations, logging.DestinationDailyFile)
	assert.NotContains(t, cfg.Destinations, logging.DestinationFile,
		"FA must not be in default destinations (it was commented out in the XML)")
}




// ---------------------------------------------------------------------------
// NewLogger – nil config (falls back to defaults)
// ---------------------------------------------------------------------------

func TestNewLogger_NilConfig_UsesDefaults(t *testing.T) {
	logger, closer, err := logging.NewLogger(nil)
	require.NoError(t, err)
	require.NotNil(t, logger)
	require.NotNil(t, closer)

	// Closer must not panic on a double close-ish call.
	assert.NoError(t, closer.Close())
}

// ---------------------------------------------------------------------------
// NewLogger – ConsoleAppender (CA)
// ---------------------------------------------------------------------------


// ---------------------------------------------------------------------------
// NewLogger – level filtering (root level INFO)
// ---------------------------------------------------------------------------

// captureLogger wires the logger to an in-memory buffer for inspection.
func captureLogger(t *testing.T, level slog.Level, format logging.Format) (*slog.Logger, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	opts := &slog.HandlerOptions{Level: level, AddSource: false}
	var h slog.Handler
	if format == logging.FormatJSON {
		h = slog.NewJSONHandler(buf, opts)
	} else {
		h = slog.NewTextHandler(buf, opts)
	}
	return slog.New(h), buf
}

func TestLogger_LevelFiltering(t *testing.T) {
	tests := []struct {
		name          string
		logLevel      slog.Level // root level
		emitLevel     slog.Level // level of the message emitted
		expectOutput  bool
	}{
		// Events at or above INFO should be dispatched.
		{"INFO emits INFO", slog.LevelInfo, slog.LevelInfo, true},
		{"INFO emits WARN", slog.LevelInfo, slog.LevelWarn, true},
		{"INFO emits ERROR", slog.LevelInfo, slog.LevelError, true},
		// Events below INFO are suppressed.
		{"INFO suppresses DEBUG", slog.LevelInfo, slog.LevelDebug, false},
		// Verify at DEBUG root the suppressed case is now visible.
		{"DEBUG emits DEBUG", slog.LevelDebug, slog.LevelDebug, true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			logger, buf := captureLogger(t, tc.logLevel, logging.FormatText)
			switch tc.emitLevel {
			case slog.LevelDebug:
				logger.Debug("test message")
			case slog.LevelInfo:
				logger.Info("test message")
			case slog.LevelWarn:
				logger.Warn("test message")
			case slog.LevelError:
				logger.Error("test message")
			}
			if tc.expectOutput {
				assert.NotEmpty(t, buf.String(), "expected output at level %s", tc.emitLevel)
				assert.Contains(t, buf.String(), "test message")
			} else {
				assert.Empty(t, buf.String(), "expected no output for level %s below root %s",
					tc.emitLevel, tc.logLevel)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NewLogger – FormatJSON
// ---------------------------------------------------------------------------


// ---------------------------------------------------------------------------
// NewLogger – FileAppender (FA) and DailyRollingFileAppender (DRFA)
// ---------------------------------------------------------------------------





// ---------------------------------------------------------------------------
// NewLogger – error cases
// ---------------------------------------------------------------------------



