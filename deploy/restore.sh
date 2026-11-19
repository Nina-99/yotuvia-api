#!/usr/bin/env bash
#
# Restore a Yotuvia backup set (database + uploads).
#
#   ./deploy/restore.sh --db /srv/yotuvia/backups/db-YYYYmmdd-HHMMSS.dump \
#                       --uploads /srv/yotuvia/backups/uploads-YYYYmmdd-HHMMSS.tar.gz \
#                       --yes
#
# Restoring only the database leaves rows pointing at files that are not there,
# so --uploads is required unless you pass --db-only on purpose.
#
# Overridable:
#   ENV_FILE     API env file            (default /srv/yotuvia/api/.env)
#   UPLOADS_DIR  uploads root            (default /srv/yotuvia/uploads)
#   CONTAINER    Postgres container name (default yotuvia_db)
#   SERVICE      systemd unit name       (default yotuvia-api)
#   HEALTH_URL   readiness probe         (default 127.0.0.1:8080/api/healthz)
set -euo pipefail

ENV_FILE="${ENV_FILE:-/srv/yotuvia/api/.env}"
UPLOADS_DIR="${UPLOADS_DIR:-/srv/yotuvia/uploads}"
UPLOADS_DIR="${UPLOADS_DIR%/}"
CONTAINER="${CONTAINER:-yotuvia_db}"
SERVICE="${SERVICE:-yotuvia-api}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:8080/api/healthz}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-60}"

DB_FILE=""
UPLOADS_FILE=""
CONFIRMED=""
DB_ONLY=""

usage() {
  sed -n '2,20p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --db)        DB_FILE="$2"; shift 2 ;;
    --uploads)   UPLOADS_FILE="$2"; shift 2 ;;
    --db-only)   DB_ONLY="1"; shift ;;
    --yes)       CONFIRMED="1"; shift ;;
    -h|--help)   usage 0 ;;
    *) echo "Unknown argument: $1" >&2; usage 1 ;;
  esac
done

if [ -z "$DB_FILE" ]; then
  echo "ERROR: --db <dump> is required." >&2
  exit 1
fi
if [ -z "$UPLOADS_FILE" ] && [ -z "$DB_ONLY" ]; then
  echo "ERROR: --uploads <archive> is required (or pass --db-only deliberately)." >&2
  exit 1
fi
for f in "$DB_FILE" ${UPLOADS_FILE:+"$UPLOADS_FILE"}; do
  [ -f "$f" ] || { echo "ERROR: file not found: $f" >&2; exit 1; }
done

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

# A dump whose header cannot be read is not restorable: stop before touching
# the live database.
if command -v pg_restore >/dev/null 2>&1; then
  pg_restore -l "$DB_FILE" >/dev/null
  RESTORE_CMD=(env PGPASSWORD="$DB_PASSWORD" pg_restore --clean --if-exists --no-owner
    -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME")
else
  docker exec -i "$CONTAINER" pg_restore -l < "$DB_FILE" >/dev/null
  RESTORE_CMD=(docker exec -i -e PGPASSWORD="$DB_PASSWORD" "$CONTAINER" pg_restore
    --clean --if-exists --no-owner -U "$DB_USER" -d "$DB_NAME")
fi

if [ -n "$UPLOADS_FILE" ]; then
  tar -tzf "$UPLOADS_FILE" >/dev/null
fi

echo "This will OVERWRITE database '$DB_NAME' on $DB_HOST"
[ -n "$UPLOADS_FILE" ] && echo "and replace $UPLOADS_DIR"
echo "from:"
echo "  db:      $DB_FILE"
[ -n "$UPLOADS_FILE" ] && echo "  uploads: $UPLOADS_FILE"

if [ -z "$CONFIRMED" ]; then
  printf 'Type "restore" to continue: '
  read -r answer
  [ "$answer" = "restore" ] || { echo "Aborted."; exit 1; }
fi

SUDO=""
[ "$(id -u)" -eq 0 ] || SUDO="sudo"

echo "==> Stopping $SERVICE"
$SUDO systemctl stop "$SERVICE" || true

if [ -n "$UPLOADS_FILE" ]; then
  ASIDE="$UPLOADS_DIR.pre-restore-$(date -u +%Y%m%d-%H%M%S)"
  echo "==> Moving current uploads to $ASIDE"
  if [ -d "$UPLOADS_DIR" ]; then
    mv "$UPLOADS_DIR" "$ASIDE"
  fi
  mkdir -p "$UPLOADS_DIR"
  echo "==> Extracting uploads"
  tar -xzf "$UPLOADS_FILE" -C "$(dirname "$UPLOADS_DIR")"
fi

echo "==> Restoring database"
# pg_restore returns non-zero for warnings (e.g. a missing extension role) while
# having restored the data. Report it instead of silently claiming success.
if [ "${RESTORE_CMD[0]}" = "docker" ]; then
  "${RESTORE_CMD[@]}" < "$DB_FILE" || echo "WARNING: pg_restore reported errors; review output above." >&2
else
  "${RESTORE_CMD[@]}" "$DB_FILE" || echo "WARNING: pg_restore reported errors; review output above." >&2
fi

echo "==> Starting $SERVICE"
$SUDO systemctl start "$SERVICE"

echo "==> Waiting for $HEALTH_URL"
for _ in $(seq 1 "$HEALTH_TIMEOUT"); do
  if curl -fsS --max-time 2 "$HEALTH_URL" >/dev/null 2>&1; then
    echo "==> Restore OK"
    exit 0
  fi
  sleep 1
done

echo "ERROR: $HEALTH_URL did not come back after restore." >&2
$SUDO systemctl status "$SERVICE" --no-pager -n 30 >&2 || true
exit 1
