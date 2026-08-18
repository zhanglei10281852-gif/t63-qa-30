package observability

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

func NewLogger(output io.Writer, level string) (*slog.Logger, error) {
	parsed, err := parseLevel(level)
	if err != nil {
		return nil, err
	}
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{Level: parsed, AddSource: parsed <= slog.LevelDebug})
	return slog.New(handler).With("service", "sanitation-operations"), nil
}
func parseLevel(value string) (slog.Level, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "DEBUG":
		return slog.LevelDebug, nil
	case "INFO", "":
		return slog.LevelInfo, nil
	case "WARN", "WARNING":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unsupported log level %q", value)
	}
}
