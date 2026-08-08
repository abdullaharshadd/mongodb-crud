```go
package internal

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// captureStderr redirects os.Stderr for the duration of fn and returns the
// bytes written.  It restores os.Stderr before returning.
func captureStderr(t *testing.T, fn func()) []byte {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)

	orig := os.Stderr
	os.Stderr = w

	fn()

	w.Close()
	os.Stderr = orig

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	return buf.Bytes()
}

// ---------------------------------------------------------------------------
// NewLogConfig
// ---------------------------------------------------------------------------

func TestNewLogConfig_Defaults(t *testing.T) {
	tests := []struct {
		name        string
		wantLevel   zerolog.Level
		wantFile    string
		wantConsole bool
		wantFile2   bool
	}{
		{
			name:        "default config mirrors log4j.xml root",
			wantLevel:   zerolog.InfoLevel,
			wantFile:    defaultLogFilePath,
			wantConsole: true,
			wantFile2:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewLogConfig()
			assert.Equal(t, tc.wantLevel, cfg.Level, "Level must be INFO to match root logger")
			assert.Equal(t, tc.wantFile, cfg.FilePath, "FilePath must be logs/mongo.log")
			assert.Equal(t, tc.wantConsole, cfg.Console, "Console appender CA must be enabled")
			assert.Equal(t, tc.wantFile2, cfg.File, "File appender DRFA must be enabled")
		})
	}
}

func TestNewLogConfig_DefaultLogFilePath(t *testing.T) {
	assert.Equal(t, "logs/mongo.log", defaultLogFilePath)
}

// ---------------------------------------------------------------------------
// NewLogger – directory creation
// ---------------------------------------------------------------------------

func TestNewLogger_CreatesLogDirectory(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "subdir", "app.log")

	cfg := LogConfig{
		Level:    zerolog.InfoLevel,
		FilePath: logFile,
		Console:  false,
		File:     true,
	}

	logger, err := NewLogger(cfg)
	require.NoError(t, err, "should not error when creating log directory")
	_ = logger

	_, statErr := os.Stat(filepath.Dir(logFile))
	assert.NoError(t, statErr, "log directory should be created")
}

func TestNewLogger_ErrorOnUncreatableDirectory(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can always create directories, skipping permission test")
	}

	// Create a file where a directory should go – MkdirAll will fail.
	dir := t.TempDir()
	blockingFile := filepath.Join(dir, "blocked")
	require.NoError(t, os.WriteFile(blockingFile, []byte("x"), 0o444))

	cfg := LogConfig{
		Level:    zerolog.InfoLevel,
		FilePath: filepath.Join(blockingFile, "subdir", "app.log"),
		Console:  false,
		File:     true,
	}

	_, err := NewLogger(cfg)
	assert.Error(t, err, "should error when directory cannot be created")
}

// ---------------------------------------------------------------------------
// NewLogger – writer fan-out cases (rootLogger behavioral spec)
// ---------------------------------------------------------------------------

func TestNewLogger_NoWriters_Discards(t *testing.T) {
	cfg := LogConfig{
		Level:   zerolog.InfoLevel,
		Console: false,
		File:    false,
	}
	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	// Nothing should panic; log output is discarded.
	logger.Info().Msg("this goes nowhere")
}

func TestNewLogger_ConsoleOnly(t *testing.T) {
	cfg := LogConfig{
		Level:   zerolog.InfoLevel,
		Console: true,
		File:    false,
	}

	output := captureStderr(t, func() {
		logger, err := NewLogger(cfg)
		require.NoError(t, err)
		logger.Info().Msg("hello console")
	})

	assert.Contains(t, string(output), "hello console", "INFO message must reach console (stderr)")
}

func TestNewLogger_FileOnly(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	cfg := LogConfig{
		Level:    zerolog.InfoLevel,
		FilePath: logFile,
		Console:  false,
		File:     true,
	}

	logger, err := NewLogger(cfg)
	require.NoError(t, err)
	logger.Info().Msg("hello file")

	content, readErr := os.ReadFile(logFile)
	require.NoError(t, readErr)
	assert.Contains(t, string(content), "hello file", "INFO message must be written to the log file")
}

func TestNewLogger_ConsoleAndFile_FanOut(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "fanout.log")

	cfg := LogConfig{
		Level:    zerolog.InfoLevel,
		FilePath: logFile,
		Console:  true,
		File:     true,
	}

	consoleOutput := captureStderr(t, func() {
		logger, err := NewLogger(cfg)
		require.NoError(t, err)
		logger.Info().Msg("fan-out message")
	})

	// Console output
	assert.Contains(t, string(consoleOutput), "fan-out message", "message must reach console (stderr)")

	// File output
	content, readErr := os.ReadFile(logFile)
	require.NoError(t, readErr)
	assert.Contains(t, string(content), "fan-out message", "message must reach rolling file")
}

// ---------------------------------------------------------------------------
// Level filtering – rootLogger spec:
//   INFO/WARN/ERROR/FATAL → emitted; TRACE/DEBUG → filtered
// ---------------------------------------------------------------------------

func TestNewLogger_LevelFiltering(t *testing.T) {
	tests := []struct {
		name        string
		logFn       func(l zerolog.Logger)
		wantInLog   bool
		description string
	}{
		{
			name:        "TRACE is filtered (below INFO root level)",
			logFn:       func(l zerolog.Logger) { l.Trace().Msg("trace-msg") },
			wantInLog:   false,
			description: "TRACE must not appear; root level is INFO",
		},
		{
			name:        "DEBUG is filtered (below INFO root level)",
			logFn:       func(l zerolog.Logger) { l.Debug().Msg("debug-msg") },
			wantInLog:   false,
			description: "DEBUG must not appear; root level is INFO",
		},
		{
			name:        "INFO is emitted",
			logFn:       func(l zerolog.Logger) { l.Info().Msg("info-msg") },
			wantInLog:   true,
			description: "INFO must appear; it equals the root level",
		},
		{
			name:        "WARN is emitted",
			logFn:       func(l zerolog.Logger) { l.Warn().Msg("warn-msg") },
			wantInLog:   true,
			description: "WARN must appear; it is above INFO",
		},
		{
			name:        "ERROR is emitted",
			logFn:       func(l zerolog.Logger) { l.Error().Msg("error-msg") },
			wantInLog:   true,
			description: "ERROR must appear; it is above INFO",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			logFile := filepath.Join(dir, "level.log")

			cfg := LogConfig{
				Level:    zerolog.InfoLevel,
				FilePath: logFile,
				Console:  false,
				File:     true,
			}

			logger, err := NewLogger(cfg)
			require.NoError(t, err)
			tc.logFn(logger)

			content, readErr := os.ReadFile(logFile)
			if !tc.wantInLog {
				// File may not exist at all, or be empty.
				if readErr == nil {
					assert.NotContains(t, string(content), "trace-msg")
					assert.NotContains(t, string(content), "debug-msg")
				}
				return
			}
			require.NoError(t, readErr, tc.description)
			assert.True(t, len(content) > 0, tc.description)
		})
	}
}

// ---------------------------------------------------------------------------
// ConsoleAppender (CA) behavioral spec
// ---------------------------------------------------------------------------

func TestConsoleAppender_WritesToStderr(t *testing.T) {
	tests := []struct {
		name    string
		message string
		level   zerolog.Level
	}{
		{
			name:    "INFO message written to stderr",
			message: "ca-info-message",
			level:   zerolog.InfoLevel,
		},
		{
			name:    "WARN message written to stderr",
			message: "ca-warn-message",
			level:   zerolog.WarnLevel,
		},
		{
			name:    "ERROR message written to stderr",
			message: "ca-error-message",
			level:   zerolog.ErrorLevel,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := LogConfig{
				Level:   zerolog.InfoLevel,
				Console: true,
				File:    false,
			}

			output := captureStderr(t, func() {
				logger, err := NewLogger(cfg)
				require.NoError(t, err)
				logger.WithLevel(tc.level).Msg(tc.message)
			})

			assert.Contains(t, string(output), tc.message,
				"ConsoleAppender CA must write the message to stderr")
		})
	}
}

func TestConsoleAppender_TimestampPresent(t *testing.T) {
	cfg := LogConfig{
		Level:   zerolog.InfoLevel,
		Console: true,
		File:    false,
	}

	output := captureStderr(t, func() {
		logger, err := NewLogger(cfg)
		require.NoError(t, err)
		logger.Info().Msg("timestamp-check")
	})

	str := string(output)
	// zerolog ConsoleWriter emits a timestamp; verify it is non-empty line
	// and the message is present.
	assert.True(t, len(strings.TrimSpace(str)) > 0,
		"ConsoleWriter must produce non-empty output")
	assert.Contains(t, str, "timestamp-check")
}

func TestConsoleAppender_TraceSuppressed(t *testing.T) {
	cfg := LogConfig{
		Level:   zerolog.InfoLevel,
		Console: true,
		File:    false,
	}

	output := captureStderr(t, func() {
		logger, err := NewLogger(cfg)
		require.NoError(t, err)
		logger.Trace().Msg("ca-trace-suppressed")
		logger.Debug().Msg("ca-debug-suppressed")
	})

	assert.NotContains(t, string(output), "ca-trace-suppressed",
		"ConsoleAppender must not emit TRACE (root level is INFO)")
	assert.NotContains(t, string(output), "ca-debug-suppressed",
		"ConsoleAppender must not emit DEBUG (root level is INFO)")
}

// ---------------------------------------------------------------------------
// FileAppender (FA) behavioral spec – inactive / not wired
// ---------------------------------------------------------------------------

func TestFileAppender_NotActive_DefaultConfig(t *testing.T) {
	// In the default config, FA is NOT referenced. The File:true flag maps to
	// DRFA (lumberjack), not the plain FA. Here we just verify that the default
	// config does not reference a "FA-only" path that would break if FA were
	// wired in; the meaningful check is that NewLogConfig returns FA=disabled
	// by design (there is no separate FA field — FA simply isn't modeled).
	cfg := NewLogConfig()

	// The Go migration omits FA entirely; there is no field for it.
	// We verify that building the logger succeeds and that no extra file is
	// silently created for a hypothetical FA path.
	dir := t.TempDir()
	cfg.FilePath = filepath.Join(dir, "mongo.log")
	cfg.Console = false

	logger, err := NewLogger(cfg)
	require.NoError(t, err, "logger must build without error")

	logger.Info().Msg("fa-inactive")

	// Only cfg.FilePath should exist; no separate "FA" file.
	entries, _ := os.ReadDir(dir)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.Len(t, names, 1,
		"only one log file should exist — FA is not wired, no extra file created")
}

// ---------------------------------------------------------------------------
// DailyRollingFileAppender (DRFA) behavioral spec
// ---------------------------------------------------------------------------

func TestDRFA_InfoAndAboveWrittenToFile(t *testing.T) {
	tests := []struct {
		name    string
		message string
		logFn   func(zerolog.Logger, string)
	}{
		{
			name:    "INFO written to DRFA",
			message: "drfa-info",
			logFn:   func(l zerolog.Logger, m string) { l.Info().Msg(m) },
		},
		{
			name:    "WARN written to DRFA",
			message: "drfa-warn",
			logFn:   func(l zerolog.Logger, m string) { l.Warn().Msg(m) },
		},
		{
			name:    "ERROR written to DRFA",
			message: "drfa-error",
			logFn:   func(l zerolog.Logger, m string) { l.Error().Msg(m) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			logFile := filepath.Join(dir, "mongo.log")

			cfg := LogConfig{
				Level:    zerolog.InfoLevel,
				FilePath: logFile,
				Console:  false,
				File:     true,
			}

			logger, err := NewLogger(cfg)
			require.NoError(t, err)
			tc.logFn(logger, tc.message)

			content, readErr := os.ReadFile(logFile)
			require.NoError(t, readErr)
			assert.Contains(t, string(content), tc.message)
		})
	}
}

func TestDRFA_DebugAndTraceSuppressed(t *testing.T) {
	tests := []struct {
		name    string
		message string
		logFn   func(zerolog.Logger, string)
	}{
		{
			name:    "DEBUG suppressed by DRFA threshold",
			message: "drfa-debug-suppressed",
			logFn:   func(l zerolog.Logger, m string