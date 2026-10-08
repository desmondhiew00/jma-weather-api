package api

import (
	"math"
	"time"

	"github.com/desmondhiew00/jma-weather-api/internal/domain"
)

// Measurements are pointers and are never tagged omitempty: an explicit null
// distinguishes "this station has no such sensor" from a field that was
// dropped, which omission cannot.
type observationPayload struct {
	ObservedAt         time.Time `json:"observed_at"`
	TempC              *float64  `json:"temp_c"`
	HumidityPct        *float64  `json:"humidity_pct"`
	PressureHpa        *float64  `json:"pressure_hpa"`
	WindMs             *float64  `json:"wind_ms"`
	WindDirectionDeg   *float64  `json:"wind_direction_deg"`
	Precipitation10mMm *float64  `json:"precipitation_10m_mm"`
	Precipitation1hMm  *float64  `json:"precipitation_1h_mm"`
	Precipitation3hMm  *float64  `json:"precipitation_3h_mm"`
	Precipitation24hMm *float64  `json:"precipitation_24h_mm"`
	Sun10mMin          *float64  `json:"sun_10m_min"`
	Sun1hH             *float64  `json:"sun_1h_h"`
	SnowCm             *float64  `json:"snow_cm"`
	VisibilityM        *float64  `json:"visibility_m"`
}

func newObservationPayload(o domain.Observation) observationPayload {
	return observationPayload{
		ObservedAt:         o.ObservedAt.UTC(),
		TempC:              o.TempC,
		HumidityPct:        o.HumidityPct,
		PressureHpa:        o.PressureHpa,
		WindMs:             o.WindMs,
		WindDirectionDeg:   o.WindDirectionDeg,
		Precipitation10mMm: o.Precipitation10mMm,
		Precipitation1hMm:  o.Precipitation1hMm,
		Precipitation3hMm:  o.Precipitation3hMm,
		Precipitation24hMm: o.Precipitation24hMm,
		Sun10mMin:          o.Sun10mMin,
		Sun1hH:             o.Sun1hH,
		SnowCm:             o.SnowCm,
		VisibilityM:        o.VisibilityM,
	}
}

// stationPayload names the station a coordinate resolved to. The answer is
// nearest-station data, not point data, so the station and its distance are
// always shown.
type stationPayload struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	NameEn     string  `json:"name_en"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	AltitudeM  *int32  `json:"altitude_m"`
	DistanceKm float64 `json:"distance_km"`
}

func newStationPayload(s domain.Station, distanceKm float64) stationPayload {
	return stationPayload{
		ID:         s.ID,
		Name:       s.NameKanji,
		NameEn:     s.NameEn,
		Lat:        round(s.Lat, 5),
		Lon:        round(s.Lon, 5),
		AltitudeM:  s.AltitudeM,
		DistanceKm: round(distanceKm, 2),
	}
}

// coordinatePayload is the coordinate the answer was computed from, and where
// it came from: "query" when the caller sent lat and lon, "network" when it was
// derived from the request's IP at the edge. The second is city-accurate at
// best, so the source tells the caller when the answer is an approximation.
type coordinatePayload struct {
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Source string  `json:"source"`
}

// attributionPayload satisfies the Public Data License v1.0
// (公共データ利用規約第1.0版), which requires both crediting JMA and, separately,
// recording that the data was reshaped and by whom. It ships on every response.
type attributionPayload struct {
	Publisher string `json:"publisher"`
	URL       string `json:"url"`
	NoticeJA  string `json:"notice_ja"`
}

type latestResponse struct {
	Coordinate  coordinatePayload  `json:"coordinate"`
	Station     stationPayload     `json:"station"`
	Observation observationPayload `json:"observation"`
	Attribution attributionPayload `json:"attribution"`
}

type historyResponse struct {
	Coordinate    coordinatePayload    `json:"coordinate"`
	Station       stationPayload       `json:"station"`
	From          time.Time            `json:"from"`
	To            time.Time            `json:"to"`
	RetentionDays int                  `json:"retention_days"`
	Count         int                  `json:"count"`
	Observations  []observationPayload `json:"observations"`
	Attribution   attributionPayload   `json:"attribution"`
}

type atResponse struct {
	Coordinate  coordinatePayload  `json:"coordinate"`
	Station     stationPayload     `json:"station"`
	RequestedAt time.Time          `json:"requested_at"`
	Observation observationPayload `json:"observation"`
	Attribution attributionPayload `json:"attribution"`
}

// nowStation is a station in the all-stations snapshot. There is no
// distance_km here: no coordinate was asked for, so there is nothing to measure
// the distance from.
type nowStation struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	NameEn      string             `json:"name_en"`
	Lat         float64            `json:"lat"`
	Lon         float64            `json:"lon"`
	AltitudeM   *int32             `json:"altitude_m"`
	Observation observationPayload `json:"observation"`
}

// nowResponse is the whole network at one moment. observed_at is the newest
// reading in the snapshot, not a single time every station shares: stations
// report on their own schedule.
type nowResponse struct {
	ObservedAt  *time.Time         `json:"observed_at"`
	Count       int                `json:"count"`
	Stations    []nowStation       `json:"stations"`
	Attribution attributionPayload `json:"attribution"`
}

type healthResponse struct {
	Status     string     `json:"status"`
	SHA        string     `json:"sha"`
	ObservedAt *time.Time `json:"observed_at,omitempty"`
	Detail     string     `json:"detail,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func round(v float64, places int) float64 {
	f := math.Pow(10, float64(places))

	return math.Round(v*f) / f
}
