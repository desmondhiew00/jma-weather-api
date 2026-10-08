// Package worker ingests observations on a schedule and enforces retention.
package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/desmondhiew00/jma-weather-api/internal/domain"
	"github.com/desmondhiew00/jma-weather-api/internal/jma"
)

// Store is the persistence the worker needs.
type Store interface {
	UpsertStations(ctx context.Context, stations []domain.Station) error
	CountStations(ctx context.Context) (int64, error)
	UpsertObservations(ctx context.Context, obs []domain.Observation) error
	LatestObservedAt(ctx context.Context) (time.Time, error)
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// Worker polls the provider and writes to the store.
type Worker struct {
	provider      domain.ObservationProvider
	store         Store
	log           *slog.Logger
	interval      time.Duration
	retentionDays int
}

// New returns a Worker.
func New(p domain.ObservationProvider, s Store, log *slog.Logger, interval time.Duration, retentionDays int) *Worker {
	return &Worker{provider: p, store: s, log: log, interval: interval, retentionDays: retentionDays}
}

// RefreshStations loads the station master list.
//
// A failure here is tolerated when stations are already stored: the list
// changes roughly twice a year, so refusing to ingest observations over a stale
// lookup file would cause an outage rather than prevent one. An empty table is
// a cold start and fatal.
func (w *Worker) RefreshStations(ctx context.Context) error {
	stations, err := w.provider.FetchStations(ctx)
	if err != nil {
		n, countErr := w.store.CountStations(ctx)
		if countErr != nil {
			return countErr
		}

		if n == 0 {
			return errors.New("cannot fetch station list on a cold start: " + err.Error())
		}

		w.log.Warn("station refresh failed, continuing with stored list",
			"error", err, "stored_stations", n)

		return nil
	}

	if err := w.store.UpsertStations(ctx, stations); err != nil {
		return err
	}

	w.log.Info("stations refreshed", "count", len(stations))

	return nil
}

// Run ticks until the context is cancelled. It ingests immediately on start so
// a deploy does not wait a full interval.
func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

// tick runs one ingest cycle. Errors are logged, never returned: the next tick
// is the retry, so in-process backoff would only duplicate it.
func (w *Worker) tick(ctx context.Context) {
	start := time.Now()

	ingested, err := w.ingest(ctx)
	if err != nil {
		w.log.Error("ingest failed", "error", err, "took", time.Since(start))
	}

	deleted, err := w.enforceRetention(ctx)
	if err != nil {
		w.log.Error("retention failed", "error", err)

		return
	}

	if ingested > 0 || deleted > 0 {
		w.log.Info("tick complete",
			"ingested", ingested, "deleted", deleted, "took", time.Since(start))
	}
}

// ingest fetches and stores the newest snapshot, returning how many
// observations were written. Zero means upstream had nothing new.
func (w *Worker) ingest(ctx context.Context) (int, error) {
	upstream, err := w.provider.LatestTime(ctx)
	if err != nil {
		return 0, err
	}

	stored, err := w.store.LatestObservedAt(ctx)
	if err != nil {
		return 0, err
	}

	// Observations advance every ten minutes; a five-minute poll means roughly
	// half of all ticks stop here having transferred 25 bytes.
	if !stored.IsZero() && !upstream.After(stored) {
		w.log.Debug("no new observations upstream", "upstream", upstream, "stored", stored)

		return 0, nil
	}

	obs, err := w.provider.FetchObservations(ctx, upstream)

	var partial *jma.PartialError
	switch {
	case errors.As(err, &partial):
		// Some records were malformed. The rest of the batch is good, and one
		// bad station must not cost us the other twelve hundred.
		w.log.Warn("snapshot partially decoded",
			"skipped", partial.Skipped, "total", partial.Total)
	case err != nil:
		return 0, err
	}

	if len(obs) == 0 {
		return 0, nil
	}

	if err := w.store.UpsertObservations(ctx, obs); err != nil {
		return 0, err
	}

	return len(obs), nil
}

// enforceRetention deletes observations past the retention window. The cutoff
// is wall clock: deriving it from the newest stored observation would freeze
// deletion during a JMA outage and quietly grow the table past its limit.
func (w *Worker) enforceRetention(ctx context.Context) (int64, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -w.retentionDays)

	return w.store.DeleteOlderThan(ctx, cutoff)
}
