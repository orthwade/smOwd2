package main

import (
	"context"
	"smOwd2/logs"
)

func Fatal(ctx context.Context, err error) {
	logger := logs.DefaultFromCtx(ctx)

	if err != nil {
		logger.Error("Fatal error", "error", err)
	} else {
		logger.Error("Fatal error")
	}

	cancel := ctx.Value("cancel")

	cancel()
}

func main() {
	logger := logs.NewDefault()
	logger.Info("Hello")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = context.WithValue(ctx, "logger", logger)
	ctx = context.WithValue(ctx, "cancelFunc", cancel)
}
