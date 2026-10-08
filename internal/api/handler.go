// Package api serves the public HTTP interface.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/desmondhiew00/jma-weather-api/internal/domain"
	"github.com/desmondhiew00/jma-weather-api/internal/repository"
)

// attribution satisfies the Public Data License v1.0 (公共データ利用規約第1.0版).
// It ships on every response.
var attribution = attributionPayload{
	Publisher: "Japan Meteorological Agency",
	URL:       "https://www.jma.go.jp/",
	NoticeJA:  "出典：気象庁ホームページを加工して作成（編集責任：tenkinow）",
}

// observationInterval is JMA's publishing cadence. Point-in-time queries are
// floored to this boundary because no reading exists between them.
const observationInterval = 10 * time.Minute

// atLookback is how far before the requested bucket a point-in-time query will
// accept a reading. Beyond it the station was down, so the answer is 404.
const atLookback = 30 * time.Minute

// Where a request's coordinate came from. Query parameters win; failing those,
// the edge's IP geolocation stands in, which is how a caller whose browser will
// not share a location still gets an answer.
const (
	coordFromQuery   = "query"
	coordFromNetwork = "network"
)

// nowLookback bounds the all-stations snapshot. A station whose newest reading
// is older than this is omitted rather than drawn on a map as current weather.
const nowLookback = 30 * time.Minute

// Store is the read side the API needs.
type Store interface {
	ListStations(ctx context.Context) ([]domain.Station, error)
	LatestForStation(ctx context.Context, stationID string) (domain.Observation, error)
	LatestForAllStations(ctx context.Context, earliest time.Time) ([]domain.Observation, error)
	HistoryForStation(ctx context.Context, stationID string, from, to time.Time) ([]domain.Observation, error)
	ObservationAt(ctx context.Context, stationID string, bucket, earliest time.Time) (domain.Observation, error)
	LatestObservedAt(ctx context.Context) (time.Time, error)
}

// Handler serves the API.
type Handler struct {
	store  Store
	log    *slog.Logger
	gitSHA string

	retentionDays      int
	maxDistanceKm      float64
	stalenessThreshold time.Duration

	// Stations are loaded once at startup: ~1300 haversine calculations in
	// memory beat a network round trip to Postgres on every request.
	mu       sync.RWMutex
	stations []domain.Station
}

// Options configures a Handler.
type Options struct {
	RetentionDays      int
	MaxDistanceKm      float64
	StalenessThreshold time.Duration
	GitSHA             string
}

// New returns a Handler with no stations loaded yet; call LoadStations first.
func New(store Store, log *slog.Logger, opts Options) *Handler {
	return &Handler{
		store:              store,
		log:                log,
		gitSHA:             opts.GitSHA,
		retentionDays:      opts.RetentionDays,
		maxDistanceKm:      opts.MaxDistanceKm,
		stalenessThreshold: opts.StalenessThreshold,
	}
}

// LoadStations caches the station list for coordinate resolution.
func (h *Handler) LoadStations(ctx context.Context) error {
	stations, err := h.store.ListStations(ctx)
	if err != nil {
		return err
	}

	if len(stations) == 0 {
		return errors.New("no stations stored: run the worker first")
	}

	h.mu.Lock()
	h.stations = stations
	h.mu.Unlock()

	h.log.Info("stations loaded", "count", len(stations))

	return nil
}

// Routes returns the mux for the public API.
func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/weather/latest", h.latest)
	mux.HandleFunc("GET /v1/weather/history", h.history)
	mux.HandleFunc("GET /v1/weather/at", h.at)
	mux.HandleFunc("GET /v1/weather/now", h.now)
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /v1/docs", h.docs)
	mux.HandleFunc("GET /v1/docs/", h.docs)
	mux.HandleFunc("GET /v1/openapi.yaml", h.openAPI)

	return mux
}

func (h *Handler) latest(w http.ResponseWriter, r *http.Request) {
	station, distance, coord, err := h.resolve(r)
	if err != nil {
		h.fail(w, err)

		return
	}

	obs, err := h.store.LatestForStation(r.Context(), station.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			h.fail(w, notFound(fmt.Sprintf("no observations stored for station %s", station.ID)))

			return
		}

		h.internal(w, r, "latest", err, station.ID)

		return
	}

	// observed_at is always returned so a caller can judge staleness itself;
	// there is deliberately no server-side freshness policy here.
	h.write(w, http.StatusOK, latestResponse{
		Coordinate:  coord,
		Station:     newStationPayload(station, distance),
		Observation: newObservationPayload(obs),
		Attribution: attribution,
	})
}

// now serves every reporting station's newest reading in one response, for
// callers plotting a map rather than asking about a single point.
func (h *Handler) now(w http.ResponseWriter, r *http.Request) {
	obs, err := h.store.LatestForAllStations(r.Context(), time.Now().UTC().Add(-nowLookback))
	if err != nil {
		h.internal(w, r, "now", err, "")

		return
	}

	h.mu.RLock()
	stations := h.stations
	h.mu.RUnlock()

	byID := make(map[string]domain.Station, len(stations))
	for _, s := range stations {
		byID[s.ID] = s
	}

	var newest time.Time

	entries := make([]nowStation, 0, len(obs))

	for _, o := range obs {
		// A reading whose station is not in the current master list belongs to
		// a retired station. Retention will clear it; until then it is not
		// something to put on a map.
		s, ok := byID[o.StationID]
		if !ok {
			continue
		}

		if o.ObservedAt.After(newest) {
			newest = o.ObservedAt
		}

		entries = append(entries, nowStation{
			ID:          s.ID,
			Name:        s.NameKanji,
			NameEn:      s.NameEn,
			Lat:         round(s.Lat, 5),
			Lon:         round(s.Lon, 5),
			AltitudeM:   s.AltitudeM,
			Observation: newObservationPayload(o),
		})
	}

	body := nowResponse{
		Count:       len(entries),
		Stations:    entries,
		Attribution: attribution,
	}

	if !newest.IsZero() {
		t := newest.UTC()
		body.ObservedAt = &t
	}

	h.write(w, http.StatusOK, body)
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	station, distance, coord, err := h.resolve(r)
	if err != nil {
		h.fail(w, err)

		return
	}

	from, to, err := h.parseRange(r)
	if err != nil {
		h.fail(w, err)

		return
	}

	obs, err := h.store.HistoryForStation(r.Context(), station.ID, from, to)
	if err != nil {
		h.internal(w, r, "history", err, station.ID)

		return
	}

	payload := make([]observationPayload, 0, len(obs))
	for _, o := range obs {
		payload = append(payload, newObservationPayload(o))
	}

	h.write(w, http.StatusOK, historyResponse{
		Coordinate:    coord,
		Station:       newStationPayload(station, distance),
		From:          from,
		To:            to,
		RetentionDays: h.retentionDays,
		Count:         len(payload),
		Observations:  payload,
		Attribution:   attribution,
	})
}

func (h *Handler) at(w http.ResponseWriter, r *http.Request) {
	station, distance, coord, err := h.resolve(r)
	if err != nil {
		h.fail(w, err)

		return
	}

	raw := r.URL.Query().Get("at")
	if raw == "" {
		h.fail(w, badRequest("at is required (RFC3339, e.g. 2026-09-12T01:23:00Z)"))

		return
	}

	at, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		h.fail(w, badRequest("at must be RFC3339, e.g. 2026-09-12T01:23:00Z"))

		return
	}

	at = at.UTC()

	// Observations exist only on ten-minute boundaries, so an arbitrary instant
	// matches nothing. Floor rather than round: the reading current at 14:23 is
	// the one from 14:20, never a later one from 14:30.
	bucket := at.Truncate(observationInterval)

	obs, err := h.store.ObservationAt(r.Context(), station.ID, bucket, bucket.Add(-atLookback))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			h.fail(w, notFound(fmt.Sprintf(
				"no observation retained for that time (retention: %d days)", h.retentionDays)))

			return
		}

		h.internal(w, r, "at", err, station.ID)

		return
	}

	// Both the request and the actual reading are echoed so any gap is visible.
	h.write(w, http.StatusOK, atResponse{
		Coordinate:  coord,
		Station:     newStationPayload(station, distance),
		RequestedAt: at,
		Observation: newObservationPayload(obs),
		Attribution: attribution,
	})
}

// healthz reports liveness and data freshness together.
//
// Returning 503 when ingestion has stalled lets a single uptime check also
// serve as the worker's dead-man's switch. It catches a worker that is running
// but no longer storing anything, which a port check cannot.
func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	newest, err := h.store.LatestObservedAt(r.Context())
	if err != nil {
		h.log.Error("healthz: database unreachable", "error", err)
		h.write(w, http.StatusServiceUnavailable, healthResponse{
			Status: "unhealthy",
			SHA:    h.gitSHA,
			Detail: "database unreachable",
		})

		return
	}

	age := time.Since(newest)

	if newest.IsZero() || age > h.stalenessThreshold {
		detail := fmt.Sprintf("no observations stored (staleness threshold %s)", h.stalenessThreshold)
		if !newest.IsZero() {
			detail = fmt.Sprintf("newest observation is %s old (threshold %s)",
				age.Round(time.Second), h.stalenessThreshold)
		}

		h.write(w, http.StatusServiceUnavailable, healthResponse{
			Status:     "stale",
			SHA:        h.gitSHA,
			ObservedAt: timePtr(newest),
			Detail:     detail,
		})

		return
	}

	h.write(w, http.StatusOK, healthResponse{
		Status:     "ok",
		SHA:        h.gitSHA,
		ObservedAt: timePtr(newest),
	})
}

// resolve turns lat/lon query parameters into the nearest known station.
func (h *Handler) resolve(r *http.Request) (domain.Station, float64, coordinatePayload, error) {
	coord, err := requestCoordinate(r)
	if err != nil {
		return domain.Station{}, 0, coordinatePayload{}, err
	}

	lat, lon := coord.Lat, coord.Lon

	h.mu.RLock()
	stations := h.stations
	h.mu.RUnlock()

	station, km, ok := domain.Nearest(stations, lat, lon)
	if !ok {
		return domain.Station{}, 0, coordinatePayload{}, apiError{status: http.StatusServiceUnavailable, msg: "station list unavailable"}
	}

	// Coordinate validation is global bounds only; this cutoff is what rejects
	// points outside Japan, as "no station near here" rather than "bad input".
	if km > h.maxDistanceKm {
		return domain.Station{}, 0, coordinatePayload{}, notFound(fmt.Sprintf(
			"no station within %.0f km of that coordinate (nearest is %.0f km away)",
			h.maxDistanceKm, km))
	}

	return station, km, coord, nil
}

// requestCoordinate reads the coordinate to answer about.
//
// With no lat and lon, it falls back to the visitor location headers
// Cloudflare's "Add visitor location headers" managed transform sets. That is
// city-accurate at best and wrong behind a VPN, so the coordinate used is
// echoed back with its source rather than passed off as what the caller asked
// for. Absent headers (the transform off, or a request that did not come
// through the edge) are the same as no coordinate at all.
func requestCoordinate(r *http.Request) (coordinatePayload, error) {
	q := r.URL.Query()
	rawLat, rawLon := q.Get("lat"), q.Get("lon")
	source := coordFromQuery

	if rawLat == "" && rawLon == "" {
		rawLat = r.Header.Get("CF-IPLatitude")
		rawLon = r.Header.Get("CF-IPLongitude")
		source = coordFromNetwork

		if rawLat == "" || rawLon == "" {
			return coordinatePayload{}, badRequest(
				"lat and lon are required (omit both to use the network location, where the edge provides one)")
		}
	}

	lat, err := parseCoord(rawLat, "lat", 90)
	if err != nil {
		return coordinatePayload{}, edgeCoordError(source, err)
	}

	lon, err := parseCoord(rawLon, "lon", 180)
	if err != nil {
		return coordinatePayload{}, edgeCoordError(source, err)
	}

	return coordinatePayload{Lat: round(lat, 5), Lon: round(lon, 5), Source: source}, nil
}

// edgeCoordError keeps a bad edge-supplied location from reading as if the
// caller had sent a malformed parameter they never sent.
func edgeCoordError(source string, err error) error {
	if source == coordFromQuery {
		return err
	}

	return badRequest("the network location is unusable; pass lat and lon instead")
}

// parseRange reads from/to, clamping from to the retention window.
func (h *Handler) parseRange(r *http.Request) (from, to time.Time, err error) {
	q := r.URL.Query()

	from, err = parseTime(q.Get("from"), "from")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	to, err = parseTime(q.Get("to"), "to")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	if to.Before(from) {
		return time.Time{}, time.Time{}, badRequest("to must not be before from")
	}

	// Asking for last month returns the seven days that exist, rather than an
	// error; retention_days in the response explains the shortfall.
	if earliest := time.Now().UTC().AddDate(0, 0, -h.retentionDays); from.Before(earliest) {
		from = earliest
	}

	return from, to, nil
}

func parseTime(raw, name string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, badRequest(name + " is required (RFC3339, e.g. 2026-09-12T00:00:00Z)")
	}

	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, badRequest(name + " must be RFC3339, e.g. 2026-09-12T00:00:00Z")
	}

	return t.UTC(), nil
}

func parseCoord(raw, name string, limit float64) (float64, error) {
	if raw == "" {
		return 0, badRequest(name + " is required")
	}

	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, badRequest(name + " must be a number")
	}

	if math.IsNaN(v) || math.Abs(v) > limit {
		return 0, badRequest(fmt.Sprintf("%s must be between -%g and %g", name, limit, limit))
	}

	return v, nil
}

func (h *Handler) write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Public read-only data served to browser clients (the site's playground
	// among them). A wildcard is enough: there is nothing here to protect
	// with an origin check, and every route is a simple GET, so no
	// preflight is involved.
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if status == http.StatusOK {
		// Declared in code, not only in a CDN dashboard.
		w.Header().Set("Cache-Control", "public, max-age=60")
	} else {
		w.Header().Set("Cache-Control", "no-store")
	}

	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		h.log.Error("encode response", "error", err)
	}
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	var apiErr apiError
	if !errors.As(err, &apiErr) {
		apiErr = apiError{status: http.StatusInternalServerError, msg: "internal error"}
	}

	h.write(w, apiErr.status, errorResponse{Error: apiErr.msg})
}

// internal logs what the edge cannot know (the coordinate and the station the
// request resolved to), then returns an opaque 500.
func (h *Handler) internal(w http.ResponseWriter, r *http.Request, op string, err error, stationID string) {
	h.log.Error("request failed",
		"op", op,
		"error", err,
		"station_id", stationID,
		"lat", r.URL.Query().Get("lat"),
		"lon", r.URL.Query().Get("lon"))

	h.write(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
}

type apiError struct {
	status int
	msg    string
}

func (e apiError) Error() string { return e.msg }

func badRequest(msg string) error { return apiError{status: http.StatusBadRequest, msg: msg} }
func notFound(msg string) error   { return apiError{status: http.StatusNotFound, msg: msg} }

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}

	return &t
}
