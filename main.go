package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kirrryu2k/nemu-manaka/config"
	"github.com/kirrryu2k/nemu-manaka/internal/app"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(errors.WithMessage(err, "load config"))
	}

	opts := make([]zap.Option, 0)
	if !cfg.UseDebugMode {
		opts = append(opts, zap.IncreaseLevel(zap.InfoLevel))
	}
	logger, err := zap.NewProduction(opts...)
	if err != nil {
		log.Fatal(errors.WithMessage(err, "new production logger"))
	}
	defer func() { _ = logger.Sync() }()

	app, err := app.New(cfg, logger.Sugar())
	if err != nil {
		logger.Fatal(errors.WithMessage(err, "new app").Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		logger.Info("starting...")
		if err := app.Run(ctx); err != nil {
			logger.Warn(errors.WithMessage(err, "app run").Error())
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	logger.Info("shutting down...")
}
