package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"taskservice/internal/app"
	"taskservice/internal/config"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		log.Error("configuration error", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, cfg, log); err != nil {
		log.Error("application stopped", "error", err)
		os.Exit(1)
	}
}
