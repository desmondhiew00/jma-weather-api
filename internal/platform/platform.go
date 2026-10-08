// Package platform wires shared process concerns: logging, the database pool
// and signal handling.
package platform

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Logger returns a text logger. Text rather than JSON: the only consumer is a
// human running `docker compose logs`, and JSON is strictly worse for that.
func Logger(level string) *slog.Logger {
	var l slog.Level

	switch strings.ToLower(level) {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}

	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}

// Pool opens a connection pool and verifies it.
//
// maxConns is deliberately small: the production host has a single vCPU, and
// connection slots beyond that buy queueing rather than throughput.
func Pool(ctx context.Context, databaseURL string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	cfg.MaxConns = maxConns
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// SignalContext returns a context cancelled on SIGINT or SIGTERM. Docker and
// Compose send SIGTERM on every stop and redeploy.
func SignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}
