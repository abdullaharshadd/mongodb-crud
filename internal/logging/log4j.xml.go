// Package logging translates the legacy log4j 1.x XML configuration
// (src/log4j.xml) into an idiomatic Go logging setup built on log/slog.
//
// MIGRATION_NOTE: log4j.xml is a *configuration file*, not application source
// code, and it targets the Java log4j 1.x framework which has no direct Go
// equivalent. Rather than reproduce log4j's Appender/Layout/Logger hierarchy,
// the Go standard library's structured logger (log/slog, Go 1.21+) is used.
// The original configuration is captured below as typed defaults so the intent
// (log level, destinations, timestamped/leveled/caller format) is preserved and
// discoverable, then applied via a slog.Handler.
//
// Mapping from the original log4j appenders to Go concepts:
//
//   - ConsoleAppender "CA"            -> slog handler writing to os.Stderr
//   - FileAppender "FA" (commented)   -> optional file destination (disabled,
//     matching the original which had its <appender-ref> commented out)
//   - DailyRollingFileAppender "DRFA" -> a file destination. Go's stdlib has no
//     built-in daily log rotation; use an external rotator such as
//     gopkg.in/natefinch/lumberjack.v2 (with a cron/date-based rename) or the
//     OS logrotate utility. See MIGRATION_NOTE on DestinationDailyFile below.
//   - PatternLayout ConversionPattern -> slog.HandlerOptions (AddSource for
//     [%c:%L] caller info) plus the chosen handler (Text mirrors the bracketed
//     human-readable pattern; JSON is available for structured ingestion).
//   - root level="INFO"               -> slog.LevelInfo
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Destination identifies where log records are written, mirroring the
// log4j appenders declared in the original log4j.xml.
type Destination int

const (
	// DestinationConsole corresponds to the log4j ConsoleAppender "CA".
	DestinationConsole Destination = iota
	// DestinationFile corresponds to the log4j FileAppender "FA".
	//
	// MIGRATION_NOTE: In the source XML the <appender-ref ref="FA"/> was
	// commented out, so this destination is preserved but disabled by default.
	DestinationFile
	// DestinationDailyFile corresponds to the log4j DailyRollingFileAppender
	// "DRFA".
	//
	// MIGRATION_NOTE: DailyRollingFileAppender is deprecated even in log4j 1.x,
	// and the Go standard library provides no daily rotation. To reproduce the
	// original '.'yyyy-MM-dd rollover, wire an external rotator (for example
	// gopkg.in/natefinch/lumberjack.v2) as the io.Writer for this destination.
	DestinationDailyFile
)

// Default values migrated verbatim from src/log4j.xml.
const (
	// DefaultLevel is the root logger level (<level value="INFO"/>).
	DefaultLevel = slog.LevelInfo

	// DefaultLogFilePath is the File param shared by the FA and DRFA appenders
	// (<param name="File" value="logs/mongo.log"/>).
	DefaultLogFilePath = "logs/mongo.log"

	// DefaultDatePattern is the DRFA DatePattern used for daily rotation
	// (<param name="DatePattern" value="'.'yyyy-MM-dd"/>). It is retained for
	// reference by whichever external rotator is configured.
	DefaultDatePattern = ".2006-01-02"
)

// Format selects the record encoding for the slog handler. It replaces the
// log4j PatternLayout ConversionPattern: Text approximates the original
// bracketed, human-readable layout, while JSON produces structured output.
type Format int

const (
	// FormatText emulates the log4j PatternLayout "[%d] [%p] [%c:%L] %m%n"
	// style with a human-readable text handler.
	FormatText Format = iota
	// FormatJSON emits structured JSON records.
	FormatJSON
)

// Config holds the logging configuration migrated from log4j.xml. Use
// NewConfig for a value populated with the defaults from the original XML.
type Config struct {
	// Level is the minimum level that will be emitted (root level="INFO").
	Level slog.Level
	// Format is the record encoding (mirrors PatternLayout).
	Format Format
	// AddSource includes the caller file:line, mirroring the log4j
	// ConversionPattern "[%c:%L]" tokens.
	AddSource bool
	// FilePath is the log file location for the FA/DRFA appenders.
	FilePath string
	// DatePattern is the rotation suffix for the DRFA appender.
	DatePattern string
	// Destinations lists the enabled destinations. The original XML enabled
	// only CA and DRFA (FA's appender-ref was commented out), so the default
	// mirrors that exactly.
	Destinations []Destination
}

// NewConfig returns a Config populated with the defaults migrated from
// src/log4j.xml: INFO level, caller information enabled, and the console plus
// daily-file destinations active (the file-only appender remains disabled, as
// in the source).
func NewConfig() *Config {
	return &Config{
		Level:        DefaultLevel,
		Format:       FormatText,
		AddSource:    true,
		FilePath:     DefaultLogFilePath,
		DatePattern:  DefaultDatePattern,
		Destinations: []Destination{DestinationConsole, DestinationDailyFile},
	}
}

// ParseLevel converts a log4j level name ("TRACE", "DEBUG", "INFO", "WARN",
// "ERROR", "FATAL") into the closest slog.Level. It returns an error for any
// unrecognized name.
//
// MIGRATION_NOTE: slog has no TRACE or FATAL levels. TRACE maps to
// slog.LevelDebug and FATAL maps to slog.LevelError, which are the nearest
// idiomatic equivalents.
func ParseLevel(name string) (slog.Level, error) {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "TRACE":
		return slog.LevelDebug, nil
	case "DEBUG":
		return slog.LevelDebug, nil
	case "INFO":
		return slog.LevelInfo, nil
	case "WARN", "WARNING":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	case "FATAL":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("logging: unknown level %q", name)
	}
}

// NewLogger builds a *slog.Logger from the given Config. It opens any file
// destinations, combines all enabled destinations into a single writer, and
// applies the configured level, format, and source settings.
//
// The caller is responsible for the lifecycle of the returned closer, which
// closes any files opened for file-based destinations. It is never nil.
func NewLogger(cfg *Config) (*slog.Logger, io.Closer, error) {
	if cfg == nil {
		cfg = NewConfig()
	}

	var (
		writers []io.Writer
		closers []io.Closer
	)

	closeAll := func() error {
		var firstErr error
		for _, c := range closers {
			if err := c.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}

	for _, dst := range cfg.Destinations {
		switch dst {
		case DestinationConsole:
			writers = append(writers, os.Stderr)
		case DestinationFile, DestinationDailyFile:
			if cfg.FilePath == "" {
				closeAll() //nolint:errcheck // best-effort cleanup on error path
				return nil, closerFunc(func() error { return nil }),
					fmt.Errorf("logging: file destination requires a non-empty FilePath")
			}
			f, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err != nil {
				closeAll() //nolint:errcheck // best-effort cleanup on error path
				return nil, closerFunc(func() error { return nil }),
					fmt.Errorf("logging: opening log file %q: %w", cfg.FilePath, err)
			}
			writers = append(writers, f)
			closers = append(closers, f)
		default:
			closeAll() //nolint:errcheck // best-effort cleanup on error path
			return nil, closerFunc(func() error { return nil }),
				fmt.Errorf("logging: unknown destination %d", dst)
		}
	}

	if len(writers) == 0 {
		writers = append(writers, os.Stderr)
	}

	out := io.MultiWriter(writers...)
	opts := &slog.HandlerOptions{
		Level:     cfg.Level,
		AddSource: cfg.AddSource,
	}

	var handler slog.Handler
	switch cfg.Format {
	case FormatJSON:
		handler = slog.NewJSONHandler(out, opts)
	default:
		handler = slog.NewTextHandler(out, opts)
	}

	return slog.New(handler), closerFunc(closeAll), nil
}

// closerFunc adapts a plain func() error into an io.Closer.
type closerFunc func() error

// Close invokes the underlying function.
func (f closerFunc) Close() error { return f() }
