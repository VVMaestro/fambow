package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"fambow/internal/app"
)

func main() {
	cfg, err := app.LoadConfig()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := app.NewLogger(cfg.LogLevel)

	application, err := app.New(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := application.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("application stopped with error", "error", err)
		os.Exit(1)
	}

	logger.Info("application stopped")
}
