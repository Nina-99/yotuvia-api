#!/usr/bin/env bash
#
# Build and deploy the Yotuvia API on the VPS.
#
# Run from the API checkout on the server:
#   ./deploy/deploy-api.sh
#
# Overridable:
#   TARGET_DIR     where the binary and .env live   (default /srv/yotuvia/api)
#   SERVICE        systemd unit name                (default yotuvia-api)
#   HEALTH_URL     readiness probe                  (default 127.0.0.1:8080/api/healthz)
#   HEALTH_TIMEOUT seconds to wait for readiness    (default 30)
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET_DIR="${TARGET_DIR:-/srv/yotuvia/api}"
SERVICE="${SERVICE:-yotuvia-api}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:8080/api/healthz}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-30}"
BIN_NAME="yotuvia-api"

cd "$REPO_DIR"

if [ ! -f "$TARGET_DIR/.env" ]; then
  echo "ERROR: $TARGET_DIR/.env not found." >&2
  echo "       Copy .env.example there, fill it in, and chmod 600. Refusing to deploy." >&2
  exit 1
fi

# root needs no sudo; otherwise sudo is required for the install and restart.
if [ "$(id -u)" -eq 0 ]; then
  SUDO=""
else
  SUDO="sudo"
fi

echo "==> Building $BIN_NAME (static)"
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "/tmp/$BIN_NAME.build" .

echo "==> Installing into $TARGET_DIR"
# install next to the target, then mv: rename is atomic, so the running process
# keeps the old inode until systemd restarts it and a failed copy never leaves
# a truncated binary in place.
$SUDO install -m 0755 "/tmp/$BIN_NAME.build" "$TARGET_DIR/$BIN_NAME.new"
$SUDO mv -f "$TARGET_DIR/$BIN_NAME.new" "$TARGET_DIR/$BIN_NAME"
rm -f "/tmp/$BIN_NAME.build"

echo "==> Restarting $SERVICE"
$SUDO systemctl restart "$SERVICE"

echo "==> Waiting for $HEALTH_URL"
for _ in $(seq 1 "$HEALTH_TIMEOUT"); do
  if curl -fsS --max-time 2 "$HEALTH_URL" >/dev/null 2>&1; then
    curl -fsS --max-time 2 "$HEALTH_URL"
    echo
    echo "==> Deploy OK"
    exit 0
  fi
  sleep 1
done

echo "ERROR: $HEALTH_URL did not become healthy within ${HEALTH_TIMEOUT}s." >&2
$SUDO systemctl status "$SERVICE" --no-pager -n 30 >&2 || true
exit 1
