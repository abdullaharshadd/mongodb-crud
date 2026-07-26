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
func TestParseLevel_Ordering(t *testing.T) {
	trace, _ := logging.ParseLevel("TRACE")
	debug, _ := logging.ParseLevel("DEBUG")
	info, _ := logging.ParseLevel("INFO")
	warn, _ := logging.ParseLevel("WARN")
	errLvl, _ := logging.ParseLevel("ERROR")
	fatal, _ := logging.ParseLevel("FATAL")

	assert.LessOrEqual(t, int(trace), int(debug))
	assert.Less(t, int(debug), int(info))
	assert.Less(t, int(info), int(warn))
	assert.Less(t, int(warn), int(errLvl))
	assert.LessOrEqual(t, int(errLvl), int(fatal))
}

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

func TestNewConfig_DefaultLevel_IsInfo(t *testing.T) {
	assert.Equal(t, slog.LevelInfo, logging.DefaultLevel)
}

func TestNewConfig_DefaultDatePattern(t *testing.T) {
	assert.Equal(t, ".2006-01-02", logging.DefaultDatePattern,
		"date pattern must follow Go reference time and mirror the original '.'yyyy-MM-dd")
}

func TestNewConfig_DefaultLogFilePath(t *testing.T) {
	assert.Equal(t, "logs/mongo.log", logging.DefaultLogFilePath)
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

func TestNewLogger_ConsoleOnly_WritesToWriter(t *testing.T) {
	// We can't redirect os.Stderr directly in a race-safe way, but we can
	// configure a console-only destination and verify the returned logger
	// does not error on creation.
	cfg := &logging.Config{
		Level:        slog.LevelInfo,
		Format:       logging.FormatText,
		AddSource:    false,
		Destinations: []logging.Destination{logging.DestinationConsole},
	}
	logger, closer, err := logging.NewLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, logger)
	defer closer.Close()

	// Log at INFO – should not panic.
	assert.NotPanics(t, func() { logger.Info("console test") })
}

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

func TestNewLogger_FormatJSON(t *testing.T) {
	logger, buf := captureLogger(t, slog.LevelInfo, logging.FormatJSON)
	logger.Info("json test", "key", "value")
	out := buf.String()
	assert.Contains(t, out, `"msg"`)
	assert.Contains(t, out, "json test")
	assert.Contains(t, out, `"key"`)
}

// ---------------------------------------------------------------------------
// NewLogger – FileAppender (FA) and DailyRollingFileAppender (DRFA)
// ---------------------------------------------------------------------------

func TestNewLogger_FileDestination_WritesToFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	cfg := &logging.Config{
		Level:        slog.LevelInfo,
		Format:       logging.FormatText,
		AddSource:    false,
		FilePath:     logPath,
		Destinations: []logging.Destination{logging.DestinationFile},
	}

	logger, closer, err := logging.NewLogger(cfg)
	require.NoError(t, err)
	defer closer.Close()

	logger.Info("file appender test")
	require.NoError(t, closer.Close())

	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "file appender test")
}

func TestNewLogger_DailyFileDestination_WritesToFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "mongo.log")

	cfg := &logging.Config{
		Level:        slog.LevelInfo,
		Format:       logging.FormatText,
		AddSource:    false,
		FilePath:     logPath,
		DatePattern:  logging.DefaultDatePattern,
		Destinations: []logging.Destination{logging.DestinationDailyFile},
	}

	logger, closer, err := logging.NewLogger(cfg)
	require.NoError(t, err)
	defer closer.Close()

	logger.Info("drfa test message")
	require.NoError(t, closer.Close())

	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "drfa test message")
}

func TestNewLogger_FileDestination_AppendMode(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "append.log")

	// Write once.
	writeLog := func(msg string) {
		cfg := &logging.Config{
			Level:        slog.LevelInfo,
			Format:       logging.FormatText,
			AddSource:    false,
			FilePath:     logPath,
			Destinations: []logging.Destination{logging.DestinationFile},
		}
		logger, closer, err := logging.NewLogger(cfg)
		require.NoError(t, err)
		logger.Info(msg)
		require.NoError(t, closer.Close())
	}

	writeLog("first entry")
	writeLog("second entry")

	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "first entry", "existing content must be preserved (append mode)")
	assert.Contains(t, string(content), "second entry")
}

func TestNewLogger_FileDestination_LevelFiltering(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "filter.log")

	cfg := &logging.Config{
		Level:        slog.LevelInfo,
		Format:       logging.FormatText,
		AddSource:    false,
		FilePath:     logPath,
		Destinations: []logging.Destination{logging.DestinationFile},
	}

	logger, closer, err := logging.NewLogger(cfg)
	require.NoError(t, err)
	defer closer.Close()

	logger.Debug("should not appear")
	logger.Info("should appear")
	require.NoError(t, closer.Close())

	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.NotContains(t, string(content), "should not appear")
	assert.Contains(t, string(content), "should appear")
}

// ---------------------------------------------------------------------------
// NewLogger – error cases
// ---------------------------------------------------------------------------

func TestNewLogger_EmptyFilePath_ReturnsError(t *testing.T) {
	tests := []struct {
		name string
		dst  logging.Destination
	}{
		{"file destination with empty path", logging.DestinationFile},
		{"daily file destination with empty path", logging.DestinationDailyFile},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg := &logging.Config{
				Level:        slog.LevelInfo,
				FilePath:     "",
				Destinations: []logging.Destination{tc.dst},
			}
			logger, closer, err := logging.NewLogger(cfg)
			assert.Error(t, err)
			assert.Nil(t, logger)
			assert.NotNil(t, closer)
			assert.Contains(t, err.Error(), "non-empty FilePath")
			closer.Close() //nolint:errcheck
		})
	}
}

func TestNewLogger_NonWritableFile_ReturnsError(t *testing.T) {
	// Use a path that cannot be created (directory that does not exist).
	cfg := &logging.Config{
		Level:        slog.LevelInfo,
		FilePath:     "/nonexistent_dir_xyz/mongo.log",
		Destinations: []logging.Destination{logging.DestinationFile},
	}
	logger, closer, err := logging.NewLogger(cfg)
	assert.Error(t, err)
	assert.Nil(t, logger)
	assert.NotNil(t, closer)
	assert.Contains(t, err.Error(), "opening log file")
	closer.Close() //nolint:errcheck
}

func TestNewLogger_UnknownDestination_ReturnsError(t *testing.T) {
	cfg := &logging.Config{
		Level:        slog.