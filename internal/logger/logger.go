package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Configure configures the log level.
func Configure(level, format string) error {
	var logLevel zerolog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = zerolog.DebugLevel
	case "info":
		logLevel = zerolog.InfoLevel
	case "error":
		logLevel = zerolog.ErrorLevel
	default:
		return fmt.Errorf("unsupported log-level value %q, must be one of [debug, info, error]", level)
	}

	// By default zerolog uses the JSON logging format.
	var w io.Writer = os.Stderr
	if strings.ToLower(format) != "json" {
		w = zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
			NoColor:    true,
		}
	}

	logCtx := zerolog.New(w).With().Timestamp()
	if logLevel <= zerolog.DebugLevel {
		logCtx = logCtx.Caller()
	}

	logger := logCtx.Logger().Level(logLevel)
	log.Logger = logger

	zerolog.DefaultContextLogger = &log.Logger
	zerolog.SetGlobalLevel(logLevel)

	return nil
}
