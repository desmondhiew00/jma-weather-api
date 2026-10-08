// Package config loads configuration from the environment.
//
// Every missing or malformed variable is collected and reported in a single
// error, so a misconfigured deploy tells you everything that is wrong on the
// first start rather than one variable per restart.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the full configuration for both binaries. Each binary reads the
// whole thing; unused fields cost nothing and keep one source of truth.
type Config struct {
	DatabaseURL string
	Port        string
	GitSHA      string
	LogLevel    string

	JMABaseURL   string
	JMATimeout   time.Duration
	PollInterval time.Duration

	RetentionDays        int
	MaxStationDistanceKm float64
	StalenessThreshold   time.Duration
}

// Load reads configuration from the environment.
func Load() (Config, error) {
	l := &loader{}

	cfg := Config{
		DatabaseURL: l.required("DATABASE_URL"),
		Port:        l.optional("PORT", "8080"),
		GitSHA:      l.optional("GIT_SHA", "dev"),
		LogLevel:    l.optional("LOG_LEVEL", "info"),

		JMABaseURL:   l.optional("JMA_BASE_URL", "https://www.jma.go.jp"),
		JMATimeout:   l.duration("JMA_TIMEOUT", 30*time.Second),
		PollInterval: l.duration("POLL_INTERVAL", 5*time.Minute),

		RetentionDays:        l.intVal("RETENTION_DAYS", 30),
		MaxStationDistanceKm: l.floatVal("MAX_STATION_DISTANCE_KM", 100),
		StalenessThreshold:   l.duration("STALENESS_THRESHOLD", 30*time.Minute),
	}

	if err := l.err(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

type loader struct {
	problems []string
}

func (l *loader) required(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		l.problems = append(l.problems, key+" is required")
	}

	return v
}

func (l *loader) optional(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}

	return fallback
}

func (l *loader) duration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	v, err := time.ParseDuration(raw)
	if err != nil {
		l.problems = append(l.problems, fmt.Sprintf("%s: %q is not a duration (e.g. 5m)", key, raw))

		return fallback
	}

	return v
}

func (l *loader) intVal(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	v, err := strconv.Atoi(raw)
	if err != nil {
		l.problems = append(l.problems, fmt.Sprintf("%s: %q is not an integer", key, raw))

		return fallback
	}

	return v
}

func (l *loader) floatVal(key string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		l.problems = append(l.problems, fmt.Sprintf("%s: %q is not a number", key, raw))

		return fallback
	}

	return v
}

func (l *loader) err() error {
	if len(l.problems) == 0 {
		return nil
	}

	return errors.New("invalid configuration:\n  - " + strings.Join(l.problems, "\n  - "))
}
