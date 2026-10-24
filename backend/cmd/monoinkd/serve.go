package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/v1k0d3n/monoink/backend/internal/api"
	"github.com/v1k0d3n/monoink/backend/internal/conn"
	"github.com/v1k0d3n/monoink/backend/internal/engine"
	"github.com/v1k0d3n/monoink/backend/internal/paths"
	"github.com/v1k0d3n/monoink/backend/internal/power"
	"github.com/v1k0d3n/monoink/backend/internal/settings"
	"github.com/v1k0d3n/monoink/backend/internal/steam"
	"github.com/v1k0d3n/monoink/backend/internal/sysinfo"
	"github.com/v1k0d3n/monoink/backend/internal/weather"
)

func cmdServe(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	settingsDir := fs.String("settings", paths.SettingsDir(), "settings directory")
	dataDir := fs.String("data", paths.DataDir(), "cache directory")
	control := fs.String("control", paths.ControlSocket(), "control socket path")
	providers := fs.String("providers", paths.ProviderSocket(), "provider socket path")
	debug := fs.Bool("debug", false, "verbose logging")
	fs.Parse(args)

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)
	api.Version = version

	store, err := settings.Open(*settingsDir)
	if err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	eng := engine.New(&engine.Engine{
		Store:   store,
		Conn:    conn.New(conn.BlueZ, log.With("component", "ble")),
		Sys:     sysinfo.New(),
		Steam:   steam.New(home),
		Weather: weather.New(),
		DataDir: *dataDir,
		Home:    home,
		Log:     log.With("component", "engine"),
	})
	srv := &api.Server{Engine: eng, Log: log}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errc := make(chan error, 2)
	go func() { errc <- api.ListenUnix(ctx, *control, srv.Control(), log) }()
	go func() { errc <- api.ListenUnix(ctx, *providers, srv.Providers(), log) }()
	engineDone := make(chan struct{})
	go func() { eng.Run(ctx); close(engineDone) }()
	go func() {
		err := power.Watch(ctx, log.With("component", "power"), func(ev power.Event) {
			if ev == power.Resume {
				eng.Wake()
				return
			}
			sctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			eng.Sleep(sctx, ev == power.Shutdown)
		})
		if err != nil {
			log.Warn("suspend and shutdown screens unavailable", "err", err)
		}
	}()

	log.Info("monoinkd started", "version", version, "settings", *settingsDir)
	select {
	case <-ctx.Done():
	case err = <-errc:
	}
	log.Info("shutting down")
	cancel()
	// Wait for the engine (and its BLE manager) to disconnect cleanly, so
	// BlueZ isn't left holding an orphaned link to the display.
	select {
	case <-engineDone:
	case <-time.After(5 * time.Second):
		log.Warn("engine did not stop in time")
	}
	return err
}
