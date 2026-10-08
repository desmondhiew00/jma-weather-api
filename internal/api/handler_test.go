package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/desmondhiew00/jma-weather-api/internal/domain"
	"github.com/desmondhiew00/jma-weather-api/internal/repository"
)

// Tokyo (44132) and Wakkanai (11016), roughly 1000 km apart.
var testStations = []domain.Station{
	{ID: "44132", NameKanji: "東京", NameEn: "Tokyo", Lat: 35.6917, Lon: 139.75},
	{ID: "11016", NameKanji: "稚内", NameEn: "Wakkanai", Lat: 45.415, Lon: 141.6785},
}

type stubStore struct {
	latest     domain.Observation
	latestErr  error
	history    []domain.Observation
	historyGot [2]time.Time
	at         domain.Observation
	atErr      error
	atGot      [2]time.Time
	all        []domain.Observation
	allGot     time.Time
	watermark  time.Time
}

func (s *stubStore) ListStations(context.Context) ([]domain.Station, error) {
	return testStations, nil
}

func (s *stubStore) LatestForStation(context.Context, string) (domain.Observation, error) {
	return s.latest, s.latestErr
}

func (s *stubStore) LatestForAllStations(_ context.Context, earliest time.Time) ([]domain.Observation, error) {
	s.allGot = earliest

	return s.all, nil
}

func (s *stubStore) HistoryForStation(_ context.Context, _ string, from, to time.Time) ([]domain.Observation, error) {
	s.historyGot = [2]time.Time{from, to}

	return s.history, nil
}

func (s *stubStore) ObservationAt(_ context.Context, _ string, bucket, earliest time.Time) (domain.Observation, error) {
	s.atGot = [2]time.Time{bucket, earliest}

	return s.at, s.atErr
}

func (s *stubStore) LatestObservedAt(context.Context) (time.Time, error) {
	return s.watermark, nil
}

func newTestHandler(t *testing.T, s Store) http.Handler {
	t.Helper()

	h := New(s, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{
		RetentionDays:      7,
		MaxDistanceKm:      100,
		StalenessThreshold: 30 * time.Minute,
		GitSHA:             "testsha",
	})

	if err := h.LoadStations(context.Background()); err != nil {
		t.Fatalf("LoadStations: %v", err)
	}

	return h.Routes()
}

func get(t *testing.T, h http.Handler, target string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))

	var body map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response %q: %v", rec.Body.String(), err)
		}
	}

	return rec, body
}

func TestLatestResolvesNearestStation(t *testing.T) {
	obs := domain.Observation{
		StationID:  "44132",
		ObservedAt: time.Date(2026, 9, 11, 15, 50, 0, 0, time.UTC),
		TempC:      ptr(19.5),
	}
	h := newTestHandler(t, &stubStore{latest: obs})

	// Shinjuku: a few km from the Tokyo station.
	rec, body := get(t, h, "/v1/weather/latest?lat=35.69&lon=139.70")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}

	station, _ := body["station"].(map[string]any)
	if station["id"] != "44132" {
		t.Errorf("resolved station %v, want 44132", station["id"])
	}

	if km, _ := station["distance_km"].(float64); km <= 0 || km > 20 {
		t.Errorf("distance_km %v looks wrong for Shinjuku", km)
	}

	attr, _ := body["attribution"].(map[string]any)
	if attr["publisher"] == "" || attr["url"] == "" || attr["notice_ja"] == "" {
		t.Errorf("attribution is a licence requirement and must always be complete: %v", body["attribution"])
	}
}

// A station that does not measure something must report null, not a missing key.
func TestNullMeasurementsAreExplicit(t *testing.T) {
	h := newTestHandler(t, &stubStore{latest: domain.Observation{
		StationID:  "44132",
		ObservedAt: time.Now().UTC(),
		TempC:      ptr(19.5),
	}})

	rec, body := get(t, h, "/v1/weather/latest?lat=35.69&lon=139.70")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}

	obs, _ := body["observation"].(map[string]any)

	v, present := obs["snow_cm"]
	if !present {
		t.Fatal("snow_cm key is absent; it must be present and null")
	}

	if v != nil {
		t.Errorf("snow_cm = %v, want null", v)
	}
}

func TestCoordinateValidation(t *testing.T) {
	h := newTestHandler(t, &stubStore{})

	for _, target := range []string{
		"/v1/weather/latest",
		"/v1/weather/latest?lat=35.69",
		"/v1/weather/latest?lat=abc&lon=139.7",
		"/v1/weather/latest?lat=99&lon=139.7",
		"/v1/weather/latest?lat=35.69&lon=999",
	} {
		if rec, _ := get(t, h, target); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", target, rec.Code)
		}
	}
}

// Outside Japan is 404 ("no station near here"), not 400: the input was fine.
func TestFarFromAnyStationIs404(t *testing.T) {
	h := newTestHandler(t, &stubStore{})

	rec, body := get(t, h, "/v1/weather/latest?lat=0&lon=0")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", rec.Code)
	}

	if body["error"] == nil {
		t.Error("expected an error message naming the distance limit")
	}
}

func TestHistoryClampsFromToRetentionWindow(t *testing.T) {
	s := &stubStore{}
	h := newTestHandler(t, s)

	rec, body := get(t, h,
		"/v1/weather/history?lat=35.69&lon=139.70&from=2020-01-01T00:00:00Z&to=2026-09-11T00:00:00Z")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}

	// Asking for 2020 returns the seven days that exist rather than an error.
	earliest := time.Now().UTC().AddDate(0, 0, -7)
	if d := s.historyGot[0].Sub(earliest); d > time.Minute || d < -time.Minute {
		t.Errorf("from was not clamped to the retention window: got %s", s.historyGot[0])
	}

	if body["retention_days"] != float64(7) {
		t.Errorf("retention_days = %v; the caller must be told why the range shrank", body["retention_days"])
	}
}

func TestHistoryRejectsInvertedRange(t *testing.T) {
	h := newTestHandler(t, &stubStore{})

	rec, _ := get(t, h,
		"/v1/weather/history?lat=35.69&lon=139.70&from=2026-09-11T00:00:00Z&to=2026-09-10T00:00:00Z")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", rec.Code)
	}
}

func TestHistoryRequiresBothBounds(t *testing.T) {
	h := newTestHandler(t, &stubStore{})

	rec, _ := get(t, h, "/v1/weather/history?lat=35.69&lon=139.70&from=2026-09-11T00:00:00Z")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", rec.Code)
	}
}

// The core /at rule: floor to the enclosing ten-minute bucket, never round up.
func TestAtFloorsToTenMinuteBucket(t *testing.T) {
	s := &stubStore{at: domain.Observation{
		StationID:  "44132",
		ObservedAt: time.Date(2026, 9, 11, 14, 20, 0, 0, time.UTC),
	}}
	h := newTestHandler(t, s)

	rec, body := get(t, h, "/v1/weather/at?lat=35.69&lon=139.70&at=2026-09-11T14:27:41Z")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}

	// 14:27 must floor to 14:20, not round to 14:30. A reading from after the
	// requested instant would be wrong.
	want := time.Date(2026, 9, 11, 14, 20, 0, 0, time.UTC)
	if !s.atGot[0].Equal(want) {
		t.Errorf("bucket = %s, want %s", s.atGot[0], want)
	}

	if got, want := s.atGot[1], want.Add(-30*time.Minute); !got.Equal(want) {
		t.Errorf("lookback floor = %s, want %s", got, want)
	}

	if body["requested_at"] != "2026-09-11T14:27:41Z" {
		t.Errorf("requested_at = %v; the request must be echoed so the gap is visible", body["requested_at"])
	}
}

func TestAtMissingDataIs404(t *testing.T) {
	h := newTestHandler(t, &stubStore{atErr: repository.ErrNotFound})

	rec, body := get(t, h, "/v1/weather/at?lat=35.69&lon=139.70&at=2020-01-01T00:00:00Z")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", rec.Code)
	}

	// The message should teach the retention rule without requiring docs.
	if msg, _ := body["error"].(string); msg == "" {
		t.Error("expected an error naming the retention window")
	}
}

func TestAtRequiresValidTimestamp(t *testing.T) {
	h := newTestHandler(t, &stubStore{})

	for _, target := range []string{
		"/v1/weather/at?lat=35.69&lon=139.70",
		"/v1/weather/at?lat=35.69&lon=139.70&at=yesterday",
	} {
		if rec, _ := get(t, h, target); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", target, rec.Code)
		}
	}
}

// healthz doubles as the worker's dead-man's switch, so fresh data is 200 and
// stalled ingestion must be 503 even though the process is perfectly alive.
func TestCoordinateFallsBackToEdgeLocation(t *testing.T) {
	h := newTestHandler(t, &stubStore{latest: domain.Observation{
		StationID:  "44132",
		ObservedAt: time.Date(2026, 9, 11, 15, 50, 0, 0, time.UTC),
	}})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/weather/latest", nil)
	req.Header.Set("CF-IPLatitude", "35.6895")
	req.Header.Set("CF-IPLongitude", "139.6917")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	coord, _ := body["coordinate"].(map[string]any)
	if coord["source"] != "network" {
		t.Errorf("source = %v, want network", coord["source"])
	}

	// The coordinate actually used is echoed, so an approximation cannot be
	// mistaken for something the caller asked for.
	if coord["lat"] != 35.6895 {
		t.Errorf("lat = %v, want 35.6895", coord["lat"])
	}

	station, _ := body["station"].(map[string]any)
	if station["id"] != "44132" {
		t.Errorf("station = %v, want the station nearest the edge location", station["id"])
	}
}

func TestQueryCoordinateBeatsEdgeLocation(t *testing.T) {
	h := newTestHandler(t, &stubStore{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/weather/latest?lat=45.4&lon=141.7", nil)
	req.Header.Set("CF-IPLatitude", "35.6895")
	req.Header.Set("CF-IPLongitude", "139.6917")
	h.ServeHTTP(rec, req)

	var body map[string]any

	_ = json.Unmarshal(rec.Body.Bytes(), &body)

	coord, _ := body["coordinate"].(map[string]any)
	if coord["source"] != "query" {
		t.Errorf("source = %v, want query", coord["source"])
	}

	station, _ := body["station"].(map[string]any)
	if station["id"] != "11016" {
		t.Errorf("station = %v, want the station nearest the query coordinate", station["id"])
	}
}

func TestMissingCoordinateWithoutEdgeLocationIs400(t *testing.T) {
	h := newTestHandler(t, &stubStore{})

	rec, body := get(t, h, "/v1/weather/latest")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	if msg, _ := body["error"].(string); msg == "" {
		t.Error("error message is empty")
	}
}

func TestUnusableEdgeLocationDoesNotBlameTheCaller(t *testing.T) {
	h := newTestHandler(t, &stubStore{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/weather/latest", nil)
	req.Header.Set("CF-IPLatitude", "nonsense")
	req.Header.Set("CF-IPLongitude", "139.6917")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	var body map[string]any

	_ = json.Unmarshal(rec.Body.Bytes(), &body)

	if msg, _ := body["error"].(string); !strings.Contains(msg, "network location") {
		t.Errorf("error = %q, want it to name the network location as the problem", msg)
	}
}

func TestNowReturnsOneEntryPerKnownStation(t *testing.T) {
	observed := time.Date(2026, 9, 11, 15, 50, 0, 0, time.UTC)
	s := &stubStore{all: []domain.Observation{
		{StationID: "44132", ObservedAt: observed, TempC: ptr(19.5)},
		{StationID: "11016", ObservedAt: observed.Add(-10 * time.Minute), TempC: ptr(8)},
		// A retired station: still in the observations table, no longer in the
		// master list, so it must not appear in the snapshot.
		{StationID: "99999", ObservedAt: observed, TempC: ptr(1)},
	}}

	rec, body := get(t, newTestHandler(t, s), "/v1/weather/now")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	if got := body["count"]; got != float64(2) {
		t.Errorf("count = %v, want 2", got)
	}

	// observed_at is the newest reading present, not the oldest or a shared one.
	if got := body["observed_at"]; got != observed.Format(time.RFC3339) {
		t.Errorf("observed_at = %v, want %s", got, observed.Format(time.RFC3339))
	}

	stations, _ := body["stations"].([]any)
	if len(stations) != 2 {
		t.Fatalf("len(stations) = %d, want 2", len(stations))
	}

	first, _ := stations[0].(map[string]any)
	if _, ok := first["distance_km"]; ok {
		t.Error("distance_km present, but no coordinate was asked for")
	}

	obs, _ := first["observation"].(map[string]any)
	if _, ok := obs["snow_cm"]; !ok {
		t.Error("snow_cm missing; unmeasured keys must still be present as null")
	}
}

func TestNowExcludesStationsSilentBeyondLookback(t *testing.T) {
	s := &stubStore{}
	get(t, newTestHandler(t, s), "/v1/weather/now")

	// The store is asked for a bounded window, so a station that stopped
	// reporting drops out instead of being served as current.
	if age := time.Since(s.allGot); age < 29*time.Minute || age > 31*time.Minute {
		t.Errorf("earliest = %v, want ~30 minutes ago", s.allGot)
	}
}

func TestHealthzReflectsDataFreshness(t *testing.T) {
	tests := []struct {
		name      string
		watermark time.Time
		want      int
		status    string
	}{
		{"fresh", time.Now().UTC().Add(-5 * time.Minute), http.StatusOK, "ok"},
		{"stale", time.Now().UTC().Add(-2 * time.Hour), http.StatusServiceUnavailable, "stale"},
		{"empty", time.Time{}, http.StatusServiceUnavailable, "stale"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandler(t, &stubStore{watermark: tc.watermark})

			rec, body := get(t, h, "/healthz")
			if rec.Code != tc.want {
				t.Errorf("status %d, want %d (%s)", rec.Code, tc.want, rec.Body)
			}

			if body["status"] != tc.status {
				t.Errorf("status field = %v, want %q", body["status"], tc.status)
			}

			if body["sha"] != "testsha" {
				t.Errorf("sha = %v; healthz must report the running build", body["sha"])
			}
		})
	}
}

func ptr(f float64) *float64 { return &f }
