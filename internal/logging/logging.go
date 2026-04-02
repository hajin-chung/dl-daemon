package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type multiHandler struct {
	handlers []slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var errs []string
	for _, handler := range h.handlers {
		if !handler.Enabled(ctx, record.Level) {
			continue
		}
		if err := handler.Handle(ctx, record.Clone()); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		next = append(next, handler.WithAttrs(attrs))
	}
	return &multiHandler{handlers: next}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		next = append(next, handler.WithGroup(name))
	}
	return &multiHandler{handlers: next}
}

func ParseLevel(raw string, fallback slog.Level) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "info", "":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return fallback
	}
}

func DefaultLogPaths() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	logDir := filepath.Join(baseDir, "dld", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(logDir, "dld.log.jsonl"), nil
}

func Setup(stderr io.Writer) (string, error) {
	consoleLevel := ParseLevel(os.Getenv("DLD_LOG_LEVEL"), slog.LevelInfo)
	fileLevel := ParseLevel(os.Getenv("DLD_FILE_LOG_LEVEL"), slog.LevelDebug)

	logPath, err := DefaultLogPaths()
	if err != nil {
		return "", err
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return "", err
	}

	consoleHandler := slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: consoleLevel})
	fileHandler := slog.NewJSONHandler(file, &slog.HandlerOptions{Level: fileLevel})

	logger := slog.New(&multiHandler{handlers: []slog.Handler{consoleHandler, fileHandler}})
	slog.SetDefault(logger)
	return logPath, nil
}
