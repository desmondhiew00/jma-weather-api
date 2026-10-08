package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadReportsEveryProblemAtOnce(t *testing.T) {
	// Cleared explicitly: the Makefile exports DATABASE_URL for the repository
	// tests, and inheriting it here would hide the missing-required case.
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POLL_INTERVAL", "soon")
	t.Setenv("RETENTION_DAYS", "a week")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error")
	}

	// One restart should reveal all three problems, not just the first.
	for _, want := range []string{"DATABASE_URL", "POLL_INTERVAL", "RETENTION_DAYS"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %s:\n%s", want, err)
		}
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/x")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.PollInterval != 5*time.Minute {
		t.Errorf("PollInterval: got %s", cfg.PollInterval)
	}

	if cfg.RetentionDays != 30 {
		t.Errorf("RetentionDays: got %d", cfg.RetentionDays)
	}

	if cfg.MaxStationDistanceKm != 100 {
		t.Errorf("MaxStationDistanceKm: got %v", cfg.MaxStationDistanceKm)
	}
}
