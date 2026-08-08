// Package internal captures the migration of the legacy Log4j 1.x XML
// configuration (src/log4j.xml) into idiomatic Go logging setup.
//
// MIGRATION_NOTE: log4j.xml has no direct runtime counterpart in Go. There is
// no equivalent of Log4j's XML-driven appender/layout machinery in the Go
// ecosystem. Idiomatic Go configures logging programmatically via a structured
// logger (here, zerolog). This file therefore both documents the migration
// decisions and provides a small, reusable logger constructor that reproduces
// the original behavior:
//
//   log4j concept                       Go equivalent
//   -----------------------------       -----------------------------
//   ConsoleAppender "CA"                zerolog.ConsoleWriter -> os.Stderr
//   DailyRollingFileAppender "DRFA"     lumberjack.Logger (date-based rotation)
//   FileAppender "FA" (commented out)   omitted — it was disabled in the root
//   root level INFO                     zerolog.InfoLevel
//   PatternLayout ConversionPattern     zerolog time/level/caller field config
//
// The original <root> enabled appenders CA and DRFA (FA was commented out),
// so the migrated logger fans out to BOTH the console (stderr) and the
// date-rolling log file logs/mongo.log at INFO level.
//
// IMPORTANT (from migration debate): the console appender in the original
// Log4j config wrote to STDERR (Log4j's ConsoleAppender defaults to
// System.err, not System.out). Any "user-facing" message such as INVALID_MSG,
// if it was emitted through log4j, therefore went to STDERR / the log file —
// NOT stdout. Downstream migrated code should treat the log stream as stderr
// when choosing an io.Writer for such messages. This is flagged for manual
// review because it depends on how INVALID_MSG was actually emitted in the
// original Java source (log.info(...) vs System.out.println(...)).
package internal

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogConfig holds the settings that were previously expressed in log4j.xml.
// Zero values are replaced with the defaults from the original configuration
// via NewLogConfig.
type LogConfig struct {
	// Level is the minimum level that will be emitted. Mirrors the root
	// logger's <level value="INFO"/>.
	Level zerolog.Level

	// FilePath is the base log file, mirroring DRFA's File param
	// ("logs/mongo.log").
	FilePath string

	// Console enables writing to stderr, mirroring the ConsoleAppender "CA".
	// Log4j's ConsoleAppender writes to System.err by default.
	Console bool

	// File enables writing to the date-rolling file, mirroring the
	// DailyRollingFileAppender "DRFA".
	File bool
}

// Default logging values transcribed from the original log4j.xml.
const (
	defaultLogFilePath = "logs/mongo.log"
)

// NewLogConfig returns a LogConfig populated with the defaults from the
// original log4j.xml: INFO level, console (stderr) output enabled, and
// date-rolling file output to logs/mongo.log enabled. The commented-out
// FileAppender "FA" is intentionally not represented, as it was disabled in
// the source <root>.
func NewLogConfig() LogConfig {
	return LogConfig{
		Level:    zerolog.InfoLevel,
		FilePath: defaultLogFilePath,
		Console:  true,
		File:     true,
	}
}

// NewLogger builds a zerolog.Logger from the given LogConfig, reproducing the
// behavior of the original log4j.xml root logger. It fans out to the console
// (stderr) and/or the date-rolling file depending on the config.
//
// It returns the configured logger and an error if the log directory could not
// be prepared. Callers own the returned logger; the underlying file writer is
// managed by lumberjack and rotated automatically.
func NewLogger(cfg LogConfig) (zerolog.Logger, error) {
	zerolog.TimeFieldFormat = "2006-01-02 15:04:05.000"

	writers := make([]io.Writer, 0, 2)

	if cfg.Console {
		// ConsoleAppender "CA" -> System.err. The ConversionPattern with
		// timestamp/level/caller is approximated by zerolog's ConsoleWriter.
		console := zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: "2006-01-02 15:04:05",
		}
		writers = append(writers, console)
	}

	if cfg.File {
		// DailyRollingFileAppender "DRFA".
		//
		// MIGRATION_NOTE: Log4j's DailyRollingFileAppender rotates strictly on a
		// calendar boundary (DatePattern "'.'yyyy-MM-dd"). lumberjack rotates by
		// size/age rather than exact calendar day, so this is the closest
		// idiomatic equivalent rather than an exact reproduction. If strict
		// daily rotation is required, a cron/ticker-driven Rotate() call must be
		// added — flagged for manual review.
		if dir := filepath.Dir(cfg.FilePath); dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return zerolog.Logger{}, err
			}
		}
		fileWriter := &lumberjack.Logger{
			Filename: cfg.FilePath,
			MaxAge:   1, // days — approximates daily rotation
			Compress: false,
		}
		writers = append(writers, fileWriter)
	}

	var out io.Writer
	switch len(writers) {
	case 0:
		out = io.Discard
	case 1:
		out = writers[0]
	default:
		out = zerolog.MultiLevelWriter(writers...)
	}

	logger := zerolog.New(out).
		Level(cfg.Level).
		With().
		Timestamp().
		Caller(). // reproduces the [%c:%L] class:line part of the pattern
		Logger()

	return logger, nil
}

// ensure time is referenced so the intended TimeFormat constants are not
// accidentally dropped by future edits; also documents that the original
// PatternLayout used a second-precision timestamp for the console appender
// and millisecond precision for the file appenders.
var _ = time.Second
