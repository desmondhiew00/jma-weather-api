package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/desmondhiew00/jma-weather-api/internal/domain"
	"github.com/desmondhiew00/jma-weather-api/internal/migrate"
	"github.com/desmondhiew00/jma-weather-api/internal/repository"
)

// setup starts a throwaway Postgres, migrates it, and returns a repository.
//
// sqlc queries are only meaningfully testable against the real engine, so these
// tests use no database mocks.
func setup(t *testing.T) (*repository.Repository, context.Context) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping database test in -short mode")
	}

	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:18-alpine",
		tcpostgres.WithDatabase("weather"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(90*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	if err := migrate.Up(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}

	t.Cleanup(pool.Close)

	return repository.New(pool), ctx
}

var testStations = []domain.Station{
	{ID: "44132", NameKanji: "東京", NameKana: "トウキョウ", NameEn: "Tokyo", Lat: 35.6917, Lon: 139.75, Elems: "11111111"},
	{ID: "11016", NameKanji: "稚内", NameKana: "ワッカナイ", NameEn: "Wakkanai", Lat: 45.415, Lon: 141.6785, Elems: "11111111"},
}

func TestUpsertStationsIsIdempotent(t *testing.T) {
	repo, ctx := setup(t)

	for range 2 {
		if err := repo.UpsertStations(ctx, testStations); err != nil {
			t.Fatalf("UpsertStations: %v", err)
		}
	}

	n, err := repo.CountStations(ctx)
	if err != nil {
		t.Fatalf("CountStations: %v", err)
	}

	if n != 2 {
		t.Errorf("got %d stations after two upserts, want 2", n)
	}

	stations, err := repo.ListStations(ctx)
	if err != nil {
		t.Fatalf("ListStations: %v", err)
	}

	// Round-tripping the kanji name proves the encoding survives the driver.
	var found bool

	for _, s := range stations {
		if s.ID == "44132" {
			found = true

			if s.NameKanji != "東京" || s.NameEn != "Tokyo" {
				t.Errorf("names round-tripped as %q/%q", s.NameKanji, s.NameEn)
			}
		}
	}

	if !found {
		t.Error("station 44132 missing after upsert")
	}
}

// The upsert must absorb a re-read of the same ten-minute reading by a
// five-minute poll, and must take the newer values when JMA corrects them.
func TestUpsertObservationsUpsertsRatherThanDuplicating(t *testing.T) {
	repo, ctx := setup(t)

	if err := repo.UpsertStations(ctx, testStations); err != nil {
		t.Fatalf("UpsertStations: %v", err)
	}

	at := time.Date(2026, 9, 11, 15, 50, 0, 0, time.UTC)

	if err := repo.UpsertObservations(ctx, []domain.Observation{
		{StationID: "44132", ObservedAt: at, TempC: ptr(19.0)},
	}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	// Same key, corrected value.
	if err := repo.UpsertObservations(ctx, []domain.Observation{
		{StationID: "44132", ObservedAt: at, TempC: ptr(19.4), HumidityPct: ptr(82)},
	}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	obs, err := repo.HistoryForStation(ctx, "44132", at.Add(-time.Hour), at.Add(time.Hour))
	if err != nil {
		t.Fatalf("HistoryForStation: %v", err)
	}

	if len(obs) != 1 {
		t.Fatalf("got %d rows, want 1 (the composite key must dedupe)", len(obs))
	}

	if obs[0].TempC == nil || *obs[0].TempC != 19.4 {
		t.Errorf("temp = %v, want the corrected 19.4", obs[0].TempC)
	}

	if obs[0].HumidityPct == nil || *obs[0].HumidityPct != 82 {
		t.Errorf("humidity = %v, want 82", obs[0].HumidityPct)
	}
}

// Nil measurements must survive as NULL, not become zero.
func TestNullMeasurementsRoundTrip(t *testing.T) {
	repo, ctx := setup(t)

	if err := repo.UpsertStations(ctx, testStations); err != nil {
		t.Fatalf("UpsertStations: %v", err)
	}

	at := time.Date(2026, 9, 11, 15, 50, 0, 0, time.UTC)

	if err := repo.UpsertObservations(ctx, []domain.Observation{
		{StationID: "44132", ObservedAt: at, TempC: ptr(19.0)},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	obs, err := repo.LatestForStation(ctx, "44132")
	if err != nil {
		t.Fatalf("LatestForStation: %v", err)
	}

	if obs.SnowCm != nil {
		t.Errorf("snow_cm = %v, want nil", *obs.SnowCm)
	}

	if obs.TempC == nil || *obs.TempC != 19.0 {
		t.Errorf("temp_c = %v, want 19", obs.TempC)
	}

	// Timestamps must come back as the same instant in UTC.
	if !obs.ObservedAt.Equal(at) {
		t.Errorf("observed_at = %s, want %s", obs.ObservedAt, at)
	}
}

func TestLatestObservedAtIsTheWatermark(t *testing.T) {
	repo, ctx := setup(t)

	// An empty table must report the zero time, which the worker treats as a
	// cold start rather than as an error.
	got, err := repo.LatestObservedAt(ctx)
	if err != nil {
		t.Fatalf("LatestObservedAt on empty table: %v", err)
	}

	if !got.IsZero() {
		t.Errorf("got %s, want the zero time for an empty table", got)
	}

	if err := repo.UpsertStations(ctx, testStations); err != nil {
		t.Fatalf("UpsertStations: %v", err)
	}

	newest := time.Date(2026, 9, 11, 16, 0, 0, 0, time.UTC)
	if err := repo.UpsertObservations(ctx, []domain.Observation{
		{StationID: "44132", ObservedAt: newest.Add(-10 * time.Minute)},
		{StationID: "44132", ObservedAt: newest},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err = repo.LatestObservedAt(ctx)
	if err != nil {
		t.Fatalf("LatestObservedAt: %v", err)
	}

	if !got.Equal(newest) {
		t.Errorf("got %s, want %s", got, newest)
	}
}

func TestObservationAtRespectsLookbackWindow(t *testing.T) {
	repo, ctx := setup(t)

	if err := repo.UpsertStations(ctx, testStations); err != nil {
		t.Fatalf("UpsertStations: %v", err)
	}

	at := time.Date(2026, 9, 11, 14, 20, 0, 0, time.UTC)
	if err := repo.UpsertObservations(ctx, []domain.Observation{
		{StationID: "44132", ObservedAt: at, TempC: ptr(19.0)},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Inside the window: the 14:20 reading answers a 14:20 bucket.
	obs, err := repo.ObservationAt(ctx, "44132", at, at.Add(-30*time.Minute))
	if err != nil {
		t.Fatalf("ObservationAt: %v", err)
	}

	if !obs.ObservedAt.Equal(at) {
		t.Errorf("got %s, want %s", obs.ObservedAt, at)
	}

	// A bucket hours later must not reach back to it: during a station outage
	// a 404 is correct and a stale reading is not.
	future := at.Add(3 * time.Hour)
	if _, err := repo.ObservationAt(ctx, "44132", future, future.Add(-30*time.Minute)); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestDeleteOlderThanEnforcesRetention(t *testing.T) {
	repo, ctx := setup(t)

	if err := repo.UpsertStations(ctx, testStations); err != nil {
		t.Fatalf("UpsertStations: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Minute)
	if err := repo.UpsertObservations(ctx, []domain.Observation{
		{StationID: "44132", ObservedAt: now.AddDate(0, 0, -10)},
		{StationID: "44132", ObservedAt: now.AddDate(0, 0, -8)},
		{StationID: "44132", ObservedAt: now.AddDate(0, 0, -1)},
		{StationID: "44132", ObservedAt: now},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	deleted, err := repo.DeleteOlderThan(ctx, now.AddDate(0, 0, -7))
	if err != nil {
		t.Fatalf("DeleteOlderThan: %v", err)
	}

	if deleted != 2 {
		t.Errorf("deleted %d rows, want 2", deleted)
	}

	obs, err := repo.HistoryForStation(ctx, "44132", now.AddDate(0, 0, -30), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("HistoryForStation: %v", err)
	}

	if len(obs) != 2 {
		t.Errorf("%d rows remain, want 2", len(obs))
	}
}

func TestLatestForStationNotFound(t *testing.T) {
	repo, ctx := setup(t)

	if _, err := repo.LatestForStation(ctx, "99999"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

// The real snapshot size, to prove one round trip handles the whole batch.
func TestUpsertFullSnapshotSize(t *testing.T) {
	repo, ctx := setup(t)

	stations := make([]domain.Station, 0, 1300)
	obs := make([]domain.Observation, 0, 1300)
	at := time.Date(2026, 9, 11, 15, 50, 0, 0, time.UTC)

	for i := range 1300 {
		id := "S" + itoa(i)
		stations = append(stations, domain.Station{
			ID: id, NameKanji: "駅", Lat: 35 + float64(i)/1000, Lon: 139 + float64(i)/1000,
		})
		obs = append(obs, domain.Observation{StationID: id, ObservedAt: at, TempC: ptr(float64(i % 40))})
	}

	if err := repo.UpsertStations(ctx, stations); err != nil {
		t.Fatalf("UpsertStations(1300): %v", err)
	}

	if err := repo.UpsertObservations(ctx, obs); err != nil {
		t.Fatalf("UpsertObservations(1300): %v", err)
	}

	if n, err := repo.CountStations(ctx); err != nil || n != 1300 {
		t.Errorf("got %d stations (err %v), want 1300", n, err)
	}
}

func ptr(f float64) *float64 { return &f }

func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	var b []byte

	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}

	return string(b)
}
