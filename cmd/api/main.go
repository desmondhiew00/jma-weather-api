// Command api serves the public weather API.
//
// It never applies migrations: that is the worker's job. See internal/migrate.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "time/tzdata"

	"github.com/desmondhiew00/jma-weather-api/internal/api"
	"github.com/desmondhiew00/jma-weather-api/internal/config"
	"github.com/desmondhiew00/jma-weather-api/internal/platform"
	"github.com/desmondhiew00/jma-weather-api/internal/repository"
)

const shutdownGrace = 15 * time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")

		os.Exit(1)
	}

	log := platform.Logger(cfg.LogLevel)

	if err := run(cfg, log); err != nil {
		log.Error("api stopped", "error", err)
		os.Exit(1)
	}

	log.Info("api stopped cleanly")
}

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := platform.SignalContext()
	defer stop()

	pool, err := platform.Pool(ctx, cfg.DatabaseURL, 5)
	if err != nil {
		return err
	}

	defer pool.Close()

	handler := api.New(repository.New(pool), log, api.Options{
		RetentionDays:      cfg.RetentionDays,
		MaxDistanceKm:      cfg.MaxStationDistanceKm,
		StalenessThreshold: cfg.StalenessThreshold,
		GitSHA:             cfg.GitSHA,
	})

	// The station list is cached in memory for coordinate resolution, so the
	// worker must have populated it at least once before the API can serve.
	if err := handler.LoadStations(ctx); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errc := make(chan error, 1)

	go func() {
		log.Info("api listening", "addr", srv.Addr, "sha", cfg.GitSHA)

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	// Compose and Docker send SIGTERM on every redeploy; draining in-flight
	// requests is what keeps a deploy from returning errors to callers.
	log.Info("shutting down", "grace", shutdownGrace)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}
