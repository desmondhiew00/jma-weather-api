package jma

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func read(t *testing.T, name string) []byte {
	t.Helper()

	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}

	return b
}

func TestParseLatestTime(t *testing.T) {
	got, err := parseLatestTime(read(t, "latest_time.txt"))
	if err != nil {
		t.Fatalf("parseLatestTime: %v", err)
	}

	// 2026-09-12T00:50:00+09:00 is 2026-09-11T15:50:00Z. A naive parse would
	// land nine hours out, which is the bug this asserts against.
	want := time.Date(2026, 9, 11, 15, 50, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestSnapshotPathFloorsToTenMinutes(t *testing.T) {
	// JMA 404s on any minute that is not a multiple of ten.
	at := time.Date(2026, 9, 11, 15, 57, 30, 0, time.UTC) // 00:57:30 JST

	if got, want := snapshotPath(at), "/bosai/amedas/data/map/20260912005000.json"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestParseStations(t *testing.T) {
	stations, err := parseStations(read(t, "amedastable.json"))
	if err != nil {
		t.Fatalf("parseStations: %v", err)
	}

	if len(stations) < 1000 {
		t.Fatalf("got %d stations, expected the full national list", len(stations))
	}

	var tokyo bool

	for _, s := range stations {
		if s.ID != "44132" {
			continue
		}

		tokyo = true

		if s.NameKanji != "東京" || s.NameEn != "Tokyo" {
			t.Errorf("names: got %q/%q", s.NameKanji, s.NameEn)
		}

		// [35, 41.5] and [139, 45.0] are degrees and decimal minutes.
		if want := 35 + 41.5/60; !closeTo(s.Lat, want) {
			t.Errorf("lat: got %v, want %v", s.Lat, want)
		}

		if want := 139 + 45.0/60; !closeTo(s.Lon, want) {
			t.Errorf("lon: got %v, want %v", s.Lon, want)
		}
	}

	if !tokyo {
		t.Error("station 44132 (Tokyo) missing")
	}
}

func TestParseObservations(t *testing.T) {
	at := time.Date(2026, 9, 11, 15, 50, 0, 0, time.UTC)

	obs, skipped, err := parseObservations(read(t, "map.json"), at)
	if err != nil {
		t.Fatalf("parseObservations: %v", err)
	}

	if skipped != 0 {
		t.Errorf("skipped %d records from a known-good fixture", skipped)
	}

	if len(obs) < 1000 {
		t.Fatalf("got %d observations, expected the full national snapshot", len(obs))
	}

	var checked bool

	for _, o := range obs {
		if !o.ObservedAt.Equal(at) {
			t.Fatalf("station %s: observed_at %s, want %s", o.StationID, o.ObservedAt, at)
		}

		if o.StationID == "11001" {
			checked = true

			// {"temp":[19.0,0],"humidity":[82,0],"windDirection":[10,0],"wind":[8.8,0], ...}
			mustBe(t, "temp", o.TempC, 19.0)
			mustBe(t, "humidity", o.HumidityPct, 82)
			mustBe(t, "wind", o.WindMs, 8.8)
			// Index 10 of 16 compass points.
			mustBe(t, "windDirection", o.WindDirectionDeg, 225)

			// The station reports no pressure or snow: absent, not zero.
			if o.PressureHpa != nil {
				t.Errorf("pressure: got %v, want nil", *o.PressureHpa)
			}

			if o.SnowCm != nil {
				t.Errorf("snow: got %v, want nil", *o.SnowCm)
			}
		}
	}

	if !checked {
		t.Error("station 11001 missing from fixture")
	}
}

// TestParseObservationsEdgeCases pins the three upstream quirks seen in the
// live data.
func TestParseObservationsEdgeCases(t *testing.T) {
	at := time.Date(2026, 9, 11, 15, 50, 0, 0, time.UTC)

	body := []byte(`{
      "00001": {"temp": [19.0, 0], "windDirection": [0, 0]},
      "00002": {"precipitation24h": [12.5, 1], "temp": [7.0, 0]},
      "00003": {"sun10m": [null, 0], "temp": [3.0, 0]},
      "00004": {"windDirection": [16, 0]},
      "00005": "not an object"
    }`)

	obs, skipped, err := parseObservations(body, at)
	if err != nil {
		t.Fatalf("parseObservations: %v", err)
	}

	if skipped != 1 {
		t.Errorf("skipped %d, want 1 (the malformed record)", skipped)
	}

	if len(obs) != 4 {
		t.Fatalf("got %d observations, want 4 good records", len(obs))
	}

	byID := map[string]int{}
	for i, o := range obs {
		byID[o.StationID] = i
	}

	// windDirection 0 means calm, not due north.
	if got := obs[byID["00001"]].WindDirectionDeg; got != nil {
		t.Errorf("calm wind: got %v degrees, want nil", *got)
	}

	// A non-zero quality flag discards the value but keeps the record.
	if o := obs[byID["00002"]]; o.Precipitation24hMm != nil {
		t.Errorf("flagged precipitation: got %v, want nil", *o.Precipitation24hMm)
	} else {
		mustBe(t, "temp alongside flagged value", o.TempC, 7.0)
	}

	// A null value with a normal flag is still absent.
	if o := obs[byID["00003"]]; o.Sun10mMin != nil {
		t.Errorf("null sun10m: got %v, want nil", *o.Sun10mMin)
	} else {
		mustBe(t, "temp alongside null value", o.TempC, 3.0)
	}

	// Index 16 is 360 degrees, not 0.
	mustBe(t, "windDirection 16", obs[byID["00004"]].WindDirectionDeg, 360)
}

func mustBe(t *testing.T, field string, got *float64, want float64) {
	t.Helper()

	if got == nil {
		t.Errorf("%s: got nil, want %v", field, want)

		return
	}

	if !closeTo(*got, want) {
		t.Errorf("%s: got %v, want %v", field, *got, want)
	}
}

func closeTo(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}

	return d < 1e-9
}

// TestClientAgainstFixtureServer exercises the whole client path without
// touching JMA, which is what JMA_BASE_URL exists for.
func TestClientAgainstFixtureServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bosai/amedas/data/latest_time.txt":
			w.Write(read(t, "latest_time.txt"))
		case "/bosai/amedas/data/map/20260912005000.json":
			w.Write(read(t, "map.json"))
		case "/bosai/amedas/const/amedastable.json":
			w.Write(read(t, "amedastable.json"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, 10*time.Second)
	ctx := context.Background()

	at, err := c.LatestTime(ctx)
	if err != nil {
		t.Fatalf("LatestTime: %v", err)
	}

	obs, err := c.FetchObservations(ctx, at)
	if err != nil {
		t.Fatalf("FetchObservations: %v", err)
	}

	if len(obs) < 1000 {
		t.Errorf("got %d observations", len(obs))
	}

	stations, err := c.FetchStations(ctx)
	if err != nil {
		t.Fatalf("FetchStations: %v", err)
	}

	// Every observed station must resolve in the station table, or the API
	// cannot answer for it and the foreign key would reject the row.
	known := make(map[string]bool, len(stations))
	for _, s := range stations {
		known[s.ID] = true
	}

	for _, o := range obs {
		if !known[o.StationID] {
			t.Fatalf("observation for unknown station %s", o.StationID)
		}
	}
}

// TestNotFoundIsAnError guards the 404 JMA returns for a misaligned minute.
func TestNotFoundIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	if _, err := New(srv.URL, time.Second).LatestTime(context.Background()); err == nil {
		t.Error("expected an error for a 404 response")
	}
}
