package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Aldiwildan77/home-server/host-discovery/config"
	"github.com/Aldiwildan77/home-server/host-discovery/discovery"
	"github.com/Aldiwildan77/home-server/host-discovery/worker"
	zlog "github.com/rs/zerolog/log"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		zlog.Err(err).Str("action", "LOAD_CONFIG").Msg("failed to load config")
		return
	}

	zlog.Debug().Any("config", cfg).Msg("config loaded")

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	dsc := discovery.NewDiscovery()
	w := worker.NewWorker(ticker)

	run := func(ctx context.Context) error {
		defer dsc.CleanUp()

		if err := dsc.Scan(ctx, cfg.Discovery.Command, cfg.Discovery.Args...); err != nil {
			zlog.Err(err).Str("action", "SCAN_TARGETS").Msg("failed to scan targets")
			return err
		}

		if err := dsc.Write(cfg.Output.Path); err != nil {
			zlog.Err(err).Str("action", "WRITE_HOSTS_FILE").Msg("failed to write hosts file")
			return err
		}

		zlog.Info().Msg("host IPs written to hosts file")
		return nil
	}

	if err := run(ctx); err != nil {
		zlog.Err(err).Msg("initial discovery failed")
		return
	}

	go func() {
		if err := w.Run(ctx, run); err != nil && !errors.Is(err, context.Canceled) {
			zlog.Err(err).Msg("worker error")
		}
	}()

	<-w.Done()

	zlog.Info().Msg("worker gracefully stopped")
}
