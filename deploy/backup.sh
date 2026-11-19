#!/usr/bin/env bash
#
# Back up the Yotuvia database and uploads into a timestamped, verifiable set.
#
# Run from the API checkout on the server (or via yotuvia-backup.timer):
#   ./deploy/backup.sh
#
# Overridable:
#   ENV_FILE     API env file            (default /srv/yotuvia/api/.env)
#   BACKUP_DIR   output directory        (default /srv/yotuvia/backups)
#   UPLOADS_DIR  uploads root            (default /srv/yotuvia/uploads)
#   CONTAINER    Postgres container name (default yotuvia_db)
#   RETENTION_DAYS  delete older sets    (default 14)
#
# The database and the uploads are one restore unit: a database row points at a
# file under uploads/, so restoring one without the other leaves broken images
# and orphaned files. This script always writes both, and restore.sh expects
# both.
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${ENV_FILE:-/srv/yotuvia/api/.env}"
BACKUP_DIR="${BACKUP_DIR:-/srv/yotuvia/backups}"
UPLOADS_DIR="${UPLOADS_DIR:-/srv/yotuvia/uploads}"
UPLOADS_DIR="${UPLOADS_DIR%/}"
CONTAINER="${CONTAINER:-yotuvia_db}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"

if [ ! -f "$ENV_FILE" ]; then
  echo "ERROR: env file not found: $ENV_FILE" >&2
  exit 1
fi
if [ ! -d "$UPLOADS_DIR" ]; then
  echo "ERROR: uploads directory not found: $UPLOADS_DIR" >&2
  exit 1
fi

# Read one KEY=value without sourcing the file: a stray space in a secret would
# make `. env` execute part of it.
env_value() {
  local key="$1" line
  line="$(grep -E "^${key}=" "$ENV_FILE" | tail -n 1 || true)"
  [ -n "$line" ] || return 0
  printf '%s' "${line#*=}" | sed -e 's/^"//' -e 's/"$//' -e "s/^'//" -e "s/'$//"
}

DB_HOST="$(env_value DB_HOST)"; DB_HOST="${DB_HOST:-localhost}"
DB_PORT="$(env_value DB_PORT)"; DB_PORT="${DB_PORT:-5432}"
DB_USER="$(env_value DB_USER)"; DB_USER="${DB_USER:-yotuvia}"
DB_PASSWORD="$(env_value DB_PASSWORD)"
DB_NAME="$(env_value DB_NAME)"; DB_NAME="${DB_NAME:-yotuvia}"

if [ -z "$DB_PASSWORD" ]; then
  echo "ERROR: DB_PASSWORD is empty in $ENV_FILE" >&2
  exit 1
fi

mkdir -p "$BACKUP_DIR"

# One backup at a time: overlapping timers plus a slow dump can interleave
# files and produce a set that looks complete but is not.
exec 9>"$BACKUP_DIR/.lock"
if ! flock -n 9; then
  echo "ERROR: another backup is already running." >&2
  exit 1
fi

STAMP="$(date -u +%Y%m%d-%H%M%S)"
DB_OUT="$BACKUP_DIR/db-$STAMP.dump"
UPLOADS_OUT="$BACKUP_DIR/uploads-$STAMP.tar.gz"

# Host client when present, otherwise the postgres image's own client. Both
# paths produce the same custom-format dump.
if command -v pg_dump >/dev/null 2>&1; then
  echo "==> Dumping database with host pg_dump"
  PGPASSWORD="$DB_PASSWORD" pg_dump \
    -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
    -Fc -f "$DB_OUT"
  VERIFY_CMD=(pg_restore -l "$DB_OUT")
else
  if ! docker inspect "$CONTAINER" >/dev/null 2>&1; then
    echo "ERROR: no pg_dump on the host and container '$CONTAINER' does not exist." >&2
    exit 1
  fi
  echo "==> Dumping database through container $CONTAINER"
  docker exec -i -e PGPASSWORD="$DB_PASSWORD" "$CONTAINER" \
    pg_dump -U "$DB_USER" -d "$DB_NAME" -Fc > "$DB_OUT"
  VERIFY_CMD=(docker exec -i "$CONTAINER" pg_restore -l)
fi

echo "==> Archiving uploads"
tar -czf "$UPLOADS_OUT" -C "$(dirname "$UPLOADS_DIR")" "$(basename "$UPLOADS_DIR")"

# A dump that cannot be listed cannot be restored. Fail the backup here rather
# than discovering it during an incident.
echo "==> Verifying dump"
if [ "${VERIFY_CMD[0]}" = "docker" ]; then
  "${VERIFY_CMD[@]}" < "$DB_OUT" >/dev/null
else
  "${VERIFY_CMD[@]}" >/dev/null
fi

echo "==> Verifying archive"
tar -tzf "$UPLOADS_OUT" >/dev/null

echo "==> Removing sets older than ${RETENTION_DAYS} days"
find "$BACKUP_DIR" -maxdepth 1 -type f \( -name 'db-*.dump' -o -name 'uploads-*.tar.gz' \) \
  -mtime +"$RETENTION_DAYS" -print -delete

echo
echo "Backup OK"
ls -lh "$DB_OUT" "$UPLOADS_OUT"
echo
echo "Copy these off the host: a backup on the same disk as the database does"
echo "not survive the failure it exists to cover."
