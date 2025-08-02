package traefik

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

// LogLevel is the level of a log message.
type LogLevel string

// List of supported LogLevel values.
const (
	LogLevelError LogLevel = "error"
	LogLevelWarn  LogLevel = "warning"
	LogLevelInfo  LogLevel = "info"
	LogLevelDebug LogLevel = "debug"
	LogLevelTrace LogLevel = "trace"
)

// Log is a line of log.
type Log struct {
	Message string `json:"message"`

	// The following fields are set only if the original message was JSON encoded.
	Timestamp string                 `json:"timestamp"`
	Level     LogLevel               `json:"level"`
	Error     string                 `json:"error"`
	Fields    map[string]interface{} `json:"fields"`
}

func ParseRawLogs(rawLogs string) []Log {
	rawLines := strings.Split(rawLogs, "\n")
	logs := make([]Log, 0, len(rawLines))
	excludedKeys := map[string]struct{}{
		"error":   {},
		"message": {},
		"level":   {},
		"time":    {},
	}

	for _, rawLine := range rawLines {
		rawLine = strings.TrimSpace(rawLine)
		if rawLine == "" {
			continue
		}

		var line map[string]interface{}
		if err := json.Unmarshal([]byte(rawLine), &line); err != nil {
			logs = append(logs, Log{Message: rawLine})

			continue
		}

		logLevel, err := extractLogLevel(line, "level")
		if err != nil {
			log.Error().Interface("log", line).Msgf("Invalid log level: %v", line["level"])
			logs = append(logs, Log{Message: rawLine})

			continue
		}

		// Collect additional fields
		fields := make(map[string]interface{})
		for key, value := range line {
			if _, excluded := excludedKeys[key]; !excluded {
				fields[key] = value
			}
		}

		logs = append(logs, Log{
			Timestamp: extractString(line, "time"),
			Message:   extractString(line, "message"),
			Error:     extractString(line, "error"),
			Level:     logLevel,
			Fields:    fields,
		})
	}

	return logs
}

func extractString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}

	return ""
}

func extractLogLevel(data map[string]interface{}, key string) (LogLevel, error) {
	switch data[key] {
	case string(LogLevelError):
	case string(LogLevelWarn):
	case string(LogLevelInfo):
	case string(LogLevelDebug):
	case string(LogLevelTrace):
	default:
		return LogLevelError, fmt.Errorf("invalid log level: %v", data["level"])
	}

	//nolint:forcetypeassert // Already check above.
	return LogLevel(data[key].(string)), nil
}
