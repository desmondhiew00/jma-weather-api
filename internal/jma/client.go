// Package jma implements domain.ObservationProvider against the Japan
// Meteorological Agency's public JSON endpoints.
//
// Those endpoints are undocumented, unauthenticated and may change without
// notice. JMA asks that inquiries not be filed about them, so this package
// keeps its footprint polite: a real User-Agent, connection reuse, and a
// cheap timestamp probe before every snapshot fetch.
package jma

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/desmondhiew00/jma-weather-api/internal/domain"
)

// DefaultBaseURL is JMA's public host. Overridable so the parser can be
// exercised against a local fixture server.
const DefaultBaseURL = "https://www.jma.go.jp"

const (
	userAgent   = "jma-weather-api/1.0 (+https://github.com/desmondhiew00/jma-weather-api)"
	maxBodySize = 8 << 20 // snapshots run ~350 KB; this is a sanity ceiling
)

// Client fetches observations from JMA.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a Client. A zero baseURL means DefaultBaseURL.
func New(baseURL string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: timeout},
	}
}

var _ domain.ObservationProvider = (*Client)(nil)

// LatestTime reports the newest observation timestamp JMA has published.
//
// This reads a 25-byte text file, which is why the worker can afford to call it
// on every tick: observations only advance every ten minutes, so roughly half
// of all ticks stop here without fetching the full snapshot.
func (c *Client) LatestTime(ctx context.Context) (time.Time, error) {
	b, err := c.get(ctx, "/bosai/amedas/data/latest_time.txt")
	if err != nil {
		return time.Time{}, err
	}

	return parseLatestTime(b)
}

// FetchObservations returns every station's reading for the given instant.
func (c *Client) FetchObservations(ctx context.Context, at time.Time) ([]domain.Observation, error) {
	b, err := c.get(ctx, snapshotPath(at))
	if err != nil {
		return nil, err
	}

	obs, skipped, err := parseObservations(b, at)
	if err != nil {
		return nil, err
	}

	if skipped > 0 {
		// Surfaced by the caller's logger rather than returned as an error:
		// a few bad records must not discard a good batch.
		return obs, &PartialError{Skipped: skipped, Total: len(obs) + skipped}
	}

	return obs, nil
}

// FetchStations returns the station master list.
func (c *Client) FetchStations(ctx context.Context) ([]domain.Station, error) {
	b, err := c.get(ctx, "/bosai/amedas/const/amedastable.json")
	if err != nil {
		return nil, err
	}

	return parseStations(b)
}

// PartialError reports that a batch decoded with some records skipped. The
// batch is still usable; callers should log and continue.
type PartialError struct {
	Skipped int
	Total   int
}

func (e *PartialError) Error() string {
	return fmt.Sprintf("skipped %d of %d records", e.Skipped, e.Total)
}

func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", path, err)
	}

	req.Header.Set("User-Agent", userAgent)

	// Accept-Encoding is deliberately not set: net/http negotiates gzip and
	// decompresses transparently only when it added the header itself. Setting
	// it by hand hands back raw gzip bytes.

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", path, err)
	}

	defer func() {
		// Drain before closing so the connection can be reused.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxBodySize))
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get %s: unexpected status %s", path, resp.Status)
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	return b, nil
}
