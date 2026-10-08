// Command worker ingests JMA observations on a schedule and enforces retention.
//
// It also owns schema migrations: it runs at most one instance at a time, so
// there is no migration race, whereas several API instances starting together
// would contend for the lock.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"

	// JMA timestamps are parsed in Asia/Tokyo, so the binary carries the zone
	// database rather than depending on the base image providing one.
	_ "time/tzdata"

	"github.com/desmondhiew00/jma-weather-api/internal/config"
	"github.com/desmondhiew00/jma-weather-api/internal/jma"
	"github.com/desmondhiew00/jma-weather-api/internal/migrate"
	"github.com/desmondhiew00/jma-weather-api/internal/platform"
	"github.com/desmondhiew00/jma-weather-api/internal/repository"
	"github.com/desmondhiew00/jma-weather-api/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// Config errors predate the logger and list every problem at once.
		_, _ = os.Stderr.WriteString(err.Error() + "\n")

		os.Exit(1)
	}

	log := platform.Logger(cfg.LogLevel)

	if err := run(cfg, log); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("worker stopped", "error", err)
		os.Exit(1)
	}

	log.Info("worker stopped cleanly")
}

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := platform.SignalContext()
	defer stop()

	if err := migrate.Up(cfg.DatabaseURL); err != nil {
		return err
	}

	log.Info("migrations applied")

	pool, err := platform.Pool(ctx, cfg.DatabaseURL, 2)
	if err != nil {
		return err
	}

	defer pool.Close()

	repo := repository.New(pool)
	client := jma.New(cfg.JMABaseURL, cfg.JMATimeout)
	w := worker.New(client, repo, log, cfg.PollInterval, cfg.RetentionDays)

	if err := w.RefreshStations(ctx); err != nil {
		return err
	}

	log.Info("worker starting",
		"interval", cfg.PollInterval,
		"retention_days", cfg.RetentionDays,
		"sha", cfg.GitSHA)

	return w.Run(ctx)
}
