package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/desmondhiew00/jma-weather-api/internal/domain"
	"github.com/desmondhiew00/jma-weather-api/internal/jma"
)

type fakeProvider struct {
	latest      time.Time
	latestErr   error
	obs         []domain.Observation
	obsErr      error
	stations    []domain.Station
	stationsErr error

	fetchCalls int
}

func (f *fakeProvider) LatestTime(context.Context) (time.Time, error) {
	return f.latest, f.latestErr
}

func (f *fakeProvider) FetchObservations(context.Context, time.Time) ([]domain.Observation, error) {
	f.fetchCalls++

	return f.obs, f.obsErr
}

func (f *fakeProvider) FetchStations(context.Context) ([]domain.Station, error) {
	return f.stations, f.stationsErr
}

type fakeStore struct {
	watermark time.Time
	stations  int64
	written   []domain.Observation
	upserted  []domain.Station
	cutoff    time.Time
}

func (s *fakeStore) UpsertStations(_ context.Context, st []domain.Station) error {
	s.upserted = st

	return nil
}

func (s *fakeStore) CountStations(context.Context) (int64, error) { return s.stations, nil }

func (s *fakeStore) UpsertObservations(_ context.Context, o []domain.Observation) error {
	s.written = append(s.written, o...)

	return nil
}

func (s *fakeStore) LatestObservedAt(context.Context) (time.Time, error) { return s.watermark, nil }

func (s *fakeStore) DeleteOlderThan(_ context.Context, cutoff time.Time) (int64, error) {
	s.cutoff = cutoff

	return 0, nil
}

func newWorker(p domain.ObservationProvider, s Store) *Worker {
	return New(p, s, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Minute, 7)
}

func TestIngestSkipsWhenUpstreamIsNotNewer(t *testing.T) {
	at := time.Date(2026, 9, 11, 15, 50, 0, 0, time.UTC)
	p := &fakeProvider{latest: at}
	s := &fakeStore{watermark: at}

	n, err := newWorker(p, s).ingest(context.Background())
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}

	if n != 0 {
		t.Errorf("wrote %d observations, want 0", n)
	}

	// An unchanged timestamp means no snapshot fetch.
	if p.fetchCalls != 0 {
		t.Errorf("fetched the snapshot %d times, want 0", p.fetchCalls)
	}
}

func TestIngestWritesWhenUpstreamIsNewer(t *testing.T) {
	at := time.Date(2026, 9, 11, 16, 0, 0, 0, time.UTC)
	p := &fakeProvider{
		latest: at,
		obs:    []domain.Observation{{StationID: "44132", ObservedAt: at}},
	}
	s := &fakeStore{watermark: at.Add(-10 * time.Minute)}

	n, err := newWorker(p, s).ingest(context.Background())
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}

	if n != 1 || len(s.written) != 1 {
		t.Fatalf("wrote %d observations (store has %d), want 1", n, len(s.written))
	}
}

func TestIngestOnEmptyTable(t *testing.T) {
	at := time.Date(2026, 9, 11, 16, 0, 0, 0, time.UTC)
	p := &fakeProvider{latest: at, obs: []domain.Observation{{StationID: "1", ObservedAt: at}}}
	s := &fakeStore{} // zero watermark

	if n, err := newWorker(p, s).ingest(context.Background()); err != nil || n != 1 {
		t.Fatalf("got n=%d err=%v, want 1 and no error", n, err)
	}
}

func TestIngestKeepsPartialBatch(t *testing.T) {
	at := time.Date(2026, 9, 11, 16, 0, 0, 0, time.UTC)
	p := &fakeProvider{
		latest: at,
		obs:    []domain.Observation{{StationID: "1", ObservedAt: at}},
		obsErr: &jma.PartialError{Skipped: 3, Total: 4},
	}
	s := &fakeStore{}

	n, err := newWorker(p, s).ingest(context.Background())
	if err != nil {
		t.Fatalf("a partial decode must not fail the batch: %v", err)
	}

	if n != 1 {
		t.Errorf("wrote %d observations, want the 1 good record", n)
	}
}

func TestIngestPropagatesRealErrors(t *testing.T) {
	p := &fakeProvider{latestErr: errors.New("boom")}

	if _, err := newWorker(p, &fakeStore{}).ingest(context.Background()); err == nil {
		t.Error("expected the upstream error to propagate")
	}
}

func TestRetentionUsesWallClock(t *testing.T) {
	s := &fakeStore{}

	if _, err := newWorker(&fakeProvider{}, s).enforceRetention(context.Background()); err != nil {
		t.Fatalf("enforceRetention: %v", err)
	}

	want := time.Now().UTC().AddDate(0, 0, -7)
	if d := s.cutoff.Sub(want); d > time.Minute || d < -time.Minute {
		t.Errorf("cutoff %s is not ~7 days ago (%s)", s.cutoff, want)
	}
}

func TestRefreshStationsToleratesFailureWhenStationsExist(t *testing.T) {
	p := &fakeProvider{stationsErr: errors.New("jma down")}
	s := &fakeStore{stations: 1286}

	if err := newWorker(p, s).RefreshStations(context.Background()); err != nil {
		t.Errorf("a stale station list must not stop ingestion: %v", err)
	}
}

func TestRefreshStationsFailsOnColdStart(t *testing.T) {
	p := &fakeProvider{stationsErr: errors.New("jma down")}
	s := &fakeStore{stations: 0}

	if err := newWorker(p, s).RefreshStations(context.Background()); err == nil {
		t.Error("an empty station table with no upstream is fatal")
	}
}
