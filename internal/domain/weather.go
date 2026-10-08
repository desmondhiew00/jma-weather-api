// Package domain holds the internal weather model and the provider seam.
//
// Nothing in this package knows anything about JMA. Upstream quirks (paired
// [value, qualityFlag] arrays, JST timestamps, [degrees, minutes] coordinates)
// are normalized away inside internal/jma so a second provider can be added
// without touching the worker or the API.
package domain

import (
	"context"
	"math"
	"time"
)

// Station is a weather observation site.
type Station struct {
	ID        string
	NameKanji string
	NameKana  string
	NameEn    string
	Lat       float64
	Lon       float64
	AltitudeM *int32
	Elems     string
}

// Observation is one reading from one station at one instant.
//
// Every measurement is a pointer: nil means "not reported by this station, or
// reported with a non-normal quality flag". Stations observe different element
// sets, so nil is the common case rather than an error.
type Observation struct {
	StationID  string
	ObservedAt time.Time

	TempC              *float64
	HumidityPct        *float64
	PressureHpa        *float64
	WindMs             *float64
	WindDirectionDeg   *float64
	Precipitation10mMm *float64
	Precipitation1hMm  *float64
	Precipitation3hMm  *float64
	Precipitation24hMm *float64
	Sun10mMin          *float64
	Sun1hH             *float64
	SnowCm             *float64
	VisibilityM        *float64
}

// ObservationProvider is the upstream weather source.
//
// Implemented by internal/jma. Declared here, consumer-side, so implementations
// depend on the domain and not the reverse.
type ObservationProvider interface {
	// FetchStations returns the full station master list.
	FetchStations(ctx context.Context) ([]Station, error)

	// LatestTime reports the newest observation timestamp available upstream.
	// It is expected to be cheap enough to call on every tick.
	LatestTime(ctx context.Context) (time.Time, error)

	// FetchObservations returns every station's reading for the given instant,
	// which must be a timestamp previously returned by LatestTime.
	FetchObservations(ctx context.Context, at time.Time) ([]Observation, error)
}

const earthRadiusKm = 6371.0

// DistanceKm is the great-circle distance between two coordinates.
func DistanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dLat, dLon := p2-p1, (lon2-lon1)*math.Pi/180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(p1)*math.Cos(p2)*math.Sin(dLon/2)*math.Sin(dLon/2)

	return 2 * earthRadiusKm * math.Asin(math.Sqrt(a))
}

// Nearest returns the station closest to the given coordinate and its distance
// in kilometres. ok is false when stations is empty.
func Nearest(stations []Station, lat, lon float64) (nearest Station, km float64, ok bool) {
	best := math.Inf(1)

	for _, s := range stations {
		if d := DistanceKm(lat, lon, s.Lat, s.Lon); d < best {
			best, nearest, ok = d, s, true
		}
	}

	return nearest, best, ok
}
