// logs/logs.go
package logs

import (
	"context"
	"log/slog"
	"os"
)

type Logger struct {
	*slog.Logger
}

func New(logs *slog.Logger) *Logger {
	return &Logger{Logger: logs}
}

func NewDefault() *Logger {
	return New(slog.New(slog.NewTextHandler(os.Stderr, nil)))
}

func DefaultFromCtx(ctx context.Context) *Logger {
	logger, ok := ctx.Value("logger").(*Logger)

	if !ok {
		logger = NewDefault()
	}

	return logger
}
