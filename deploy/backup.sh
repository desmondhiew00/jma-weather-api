#!/bin/sh
# Nightly logical backup to Cloudflare R2.
#
# The box itself is disposable (Terraform, cloud-init, GHCR, compose), so this
# backs up the only things git does not hold: the Postgres data and .env.
#
# JMA's own per-station history reaches back roughly three days, which is
# shorter than our 30-day retention: a loss inside that window is permanent,
# so this is the sole recovery path. Test the restore at least once.
set -eu

DIR=/var/lib/tenkinow/backups
KEEP_DAYS=7
STAMP=$(date -u +%Y%m%dT%H%M%SZ)

cd /srv/tenkinow

# shellcheck disable=SC1091
. ./.env

if [ -z "${R2_ACCESS_KEY_ID:-}" ] || [ -z "${R2_SECRET_ACCESS_KEY:-}" ] || [ -z "${R2_ACCOUNT_ID:-}" ]; then
    echo "R2 credentials missing from /srv/tenkinow/.env; set R2_ACCOUNT_ID," >&2
    echo "R2_ACCESS_KEY_ID and R2_SECRET_ACCESS_KEY (R2 > API > Manage API tokens)." >&2
    exit 1
fi

# rclone is configured entirely from the environment, so there is no
# rclone.conf to write, keep in sync, or leave lying around with credentials.
export RCLONE_CONFIG_R2_TYPE=s3
export RCLONE_CONFIG_R2_PROVIDER=Cloudflare
export RCLONE_CONFIG_R2_ACCESS_KEY_ID="$R2_ACCESS_KEY_ID"
export RCLONE_CONFIG_R2_SECRET_ACCESS_KEY="$R2_SECRET_ACCESS_KEY"
export RCLONE_CONFIG_R2_ENDPOINT="https://${R2_ACCOUNT_ID}.r2.cloudflarestorage.com"
export RCLONE_CONFIG_R2_ACL=private
# The token is scoped to one bucket, so it cannot probe for bucket existence.
export RCLONE_CONFIG_R2_NO_CHECK_BUCKET=true

mkdir -p "$DIR"

DUMP="$DIR/weather-$STAMP.sql.gz"

docker compose exec -T postgres pg_dump -U weather -d weather | gzip > "$DUMP"

# A dump that is suspiciously small means pg_dump failed mid-stream; gzip would
# still exit 0 through the pipe, so check before shipping it as a "backup".
SIZE=$(wc -c < "$DUMP")
if [ "$SIZE" -lt 10000 ]; then
    echo "dump is only $SIZE bytes, refusing to treat it as a backup" >&2
    exit 1
fi

rclone copy "$DUMP" "r2:$R2_BUCKET/postgres/"
rclone copy ./.env "r2:$R2_BUCKET/config/"

find "$DIR" -name 'weather-*.sql.gz' -mtime "+$KEEP_DAYS" -delete

echo "backup complete: $(basename "$DUMP") ($SIZE bytes)"
