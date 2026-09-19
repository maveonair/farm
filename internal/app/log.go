package app

import (
	"errors"
	"io"
	"log/slog"

	"github.com/maveonair/farm/internal/config"
)

func newLogger(cfg config.Logging, output io.Writer) (*slog.Logger, error) {
	level := slog.LevelInfo
	switch cfg.Level {
	case "", config.LogLevelInfo:
	case config.LogLevelDebug:
		level = slog.LevelDebug
	case config.LogLevelWarn:
		level = slog.LevelWarn
	case config.LogLevelError:
		level = slog.LevelError
	default:
		return nil, errors.New("invalid log level")
	}

	options := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	switch cfg.Format {
	case "", config.LogFormatJSON:
		handler = slog.NewJSONHandler(output, options)
	case config.LogFormatText:
		handler = slog.NewTextHandler(output, options)
	default:
		return nil, errors.New("invalid log format")
	}
	return slog.New(handler), nil
}
