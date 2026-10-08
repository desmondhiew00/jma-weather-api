-- name: ListStations :many
SELECT * FROM stations ORDER BY id;

-- name: CountStations :one
SELECT count(*) FROM stations;

-- name: LatestObservedAt :one
-- The ingest watermark. Deriving it from the data means there is no separate
-- state that can disagree with what was actually written.
SELECT max(observed_at)::timestamptz FROM observations;

-- name: LatestForStation :one
SELECT * FROM observations
WHERE station_id = $1
ORDER BY observed_at DESC
LIMIT 1;

-- name: HistoryForStation :many
SELECT * FROM observations
WHERE station_id = @station_id
  AND observed_at >= @from_time
  AND observed_at <= @to_time
ORDER BY observed_at;

-- name: ObservationAt :one
-- Exact bucket first, then the most recent reading within the lookback window,
-- so a station outage yields 404 rather than a silently stale answer.
SELECT * FROM observations
WHERE station_id = @station_id
  AND observed_at <= @bucket
  AND observed_at >= @earliest
ORDER BY observed_at DESC
LIMIT 1;

-- name: DeleteOlderThan :execrows
DELETE FROM observations WHERE observed_at < @cutoff;

-- name: LatestForAllStations :many
-- One row per station: its newest reading, provided that reading is recent
-- enough to still describe the weather. DISTINCT ON walks the (station_id,
-- observed_at) primary key, so this is a single ordered scan of the window
-- rather than a per-station lookup.
SELECT DISTINCT ON (station_id) *
FROM observations
WHERE observed_at >= @earliest
ORDER BY station_id, observed_at DESC;
