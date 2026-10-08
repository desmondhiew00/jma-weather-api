package jma

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/desmondhiew00/jma-weather-api/internal/domain"
)

// jst is the zone every JMA timestamp is expressed in. Several JMA payloads
// carry no offset at all, so it must be applied explicitly: letting a naive
// timestamp reach Postgres would read back as UTC and put every reading nine
// hours off.
var jst = time.FixedZone("JST", 9*60*60)

// value is one JMA measurement: a [value, qualityFlag] pair where the value may
// be null and the flag is 0 for a normal observation.
type value struct {
	v  *float64
	ok bool
}

func (m *value) UnmarshalJSON(b []byte) error {
	var pair []json.RawMessage
	if err := json.Unmarshal(b, &pair); err != nil {
		return err
	}

	if len(pair) != 2 {
		return fmt.Errorf("expected [value, flag], got %d elements", len(pair))
	}

	var flag int
	if err := json.Unmarshal(pair[1], &flag); err != nil {
		return fmt.Errorf("quality flag: %w", err)
	}

	// Flag 0 is a normal observation. Anything else (estimated, missing,
	// suspect) is discarded: the distinction between "not observed" and
	// "observed badly" is not something this API exposes.
	if flag != 0 {
		return nil
	}

	var v *float64
	if err := json.Unmarshal(pair[0], &v); err != nil {
		return fmt.Errorf("value: %w", err)
	}

	m.v, m.ok = v, v != nil

	return nil
}

// float returns the measurement, or nil when absent, null or badly flagged.
func (m value) float() *float64 {
	if !m.ok {
		return nil
	}

	return m.v
}

// mapRecord is one station's entry in the all-station snapshot. Absent keys are
// normal: stations observe different element sets.
type mapRecord struct {
	Temp             value `json:"temp"`
	Humidity         value `json:"humidity"`
	Pressure         value `json:"pressure"`
	Wind             value `json:"wind"`
	WindDirection    value `json:"windDirection"`
	Precipitation10m value `json:"precipitation10m"`
	Precipitation1h  value `json:"precipitation1h"`
	Precipitation3h  value `json:"precipitation3h"`
	Precipitation24h value `json:"precipitation24h"`
	Sun10m           value `json:"sun10m"`
	Sun1h            value `json:"sun1h"`
	Snow             value `json:"snow"`
	Visibility       value `json:"visibility"`
}

// parseObservations decodes an all-station snapshot.
//
// A station whose record cannot be decoded is skipped and counted rather than
// failing the batch: one malformed entry upstream must not cost us 1285 good
// readings.
func parseObservations(b []byte, at time.Time) ([]domain.Observation, int, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, 0, fmt.Errorf("decode snapshot: %w", err)
	}

	obs := make([]domain.Observation, 0, len(raw))
	skipped := 0

	for id, msg := range raw {
		var r mapRecord
		if err := json.Unmarshal(msg, &r); err != nil {
			skipped++

			continue
		}

		obs = append(obs, domain.Observation{
			StationID:          id,
			ObservedAt:         at.UTC(),
			TempC:              r.Temp.float(),
			HumidityPct:        r.Humidity.float(),
			PressureHpa:        r.Pressure.float(),
			WindMs:             r.Wind.float(),
			WindDirectionDeg:   windDirectionDeg(r.WindDirection),
			Precipitation10mMm: r.Precipitation10m.float(),
			Precipitation1hMm:  r.Precipitation1h.float(),
			Precipitation3hMm:  r.Precipitation3h.float(),
			Precipitation24hMm: r.Precipitation24h.float(),
			Sun10mMin:          r.Sun10m.float(),
			Sun1hH:             r.Sun1h.float(),
			SnowCm:             r.Snow.float(),
			VisibilityM:        r.Visibility.float(),
		})
	}

	return obs, skipped, nil
}

// windDirectionDeg converts JMA's 16-point compass index to degrees.
//
// 1..16 map to 22.5°..360°. Zero is not north: it means calm, with no
// meaningful direction, so it becomes nil rather than 0°.
func windDirectionDeg(m value) *float64 {
	f := m.float()
	if f == nil || *f < 1 || *f > 16 {
		return nil
	}

	deg := *f * 22.5

	return &deg
}

// tableEntry is one station in amedastable.json. Coordinates arrive as
// [degrees, decimal-minutes], not decimal degrees.
type tableEntry struct {
	Type   string     `json:"type"`
	Elems  string     `json:"elems"`
	Lat    [2]float64 `json:"lat"`
	Lon    [2]float64 `json:"lon"`
	Alt    *int32     `json:"alt"`
	KjName string     `json:"kjName"`
	KnName string     `json:"knName"`
	EnName string     `json:"enName"`
}

// parseStations decodes the station master list.
func parseStations(b []byte) ([]domain.Station, error) {
	var raw map[string]tableEntry
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("decode station table: %w", err)
	}

	stations := make([]domain.Station, 0, len(raw))

	for id, e := range raw {
		stations = append(stations, domain.Station{
			ID:        id,
			NameKanji: e.KjName,
			NameKana:  e.KnName,
			NameEn:    e.EnName,
			Lat:       degMinToDecimal(e.Lat),
			Lon:       degMinToDecimal(e.Lon),
			AltitudeM: e.Alt,
			Elems:     e.Elems,
		})
	}

	return stations, nil
}

// degMinToDecimal converts [degrees, decimal-minutes] to decimal degrees.
func degMinToDecimal(v [2]float64) float64 {
	return v[0] + v[1]/60
}

// parseLatestTime reads the contents of latest_time.txt, which carries an
// explicit +09:00 offset.
func parseLatestTime(b []byte) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, string(trimSpace(b)))
	if err != nil {
		return time.Time{}, fmt.Errorf("parse latest_time: %w", err)
	}

	return t.UTC(), nil
}

func trimSpace(b []byte) []byte {
	start, end := 0, len(b)
	for start < end && isSpace(b[start]) {
		start++
	}

	for end > start && isSpace(b[end-1]) {
		end--
	}

	return b[start:end]
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// snapshotPath is the all-station snapshot path for an instant. JMA publishes on
// ten-minute boundaries and rejects anything else, so the minute is floored.
func snapshotPath(at time.Time) string {
	t := at.In(jst).Truncate(10 * time.Minute)

	return fmt.Sprintf("/bosai/amedas/data/map/%s00.json", t.Format("200601021504"))
}
