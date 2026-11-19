#!/usr/bin/env bash
#
# Deploy the Yotuvia API on the VPS.
#
# Two modes:
#   ARTIFACT_URL   download the release asset built by CI (recommended)
#   (none)         build from this checkout
#
#   ./deploy/deploy-api.sh
#   ARTIFACT_URL=https://github.com/Nina-99/yotuvia-api/releases/download/v1.0.0/yotuvia-api-linux-amd64.tar.gz \
#     ./deploy/deploy-api.sh
#   ARTIFACT_FILE=/tmp/yotuvia-api-linux-amd64.tar.gz ./deploy/deploy-api.sh
#
# Overridable:
#   ARCH           artifact architecture          (default amd64)
#   TARGET_DIR     where the binary and .env live  (default /srv/yotuvia/api)
#   SERVICE        systemd unit name               (default yotuvia-api)
#   HEALTH_URL     readiness probe                 (default 127.0.0.1:8080/api/healthz)
#   HEALTH_TIMEOUT seconds to wait for readiness   (default 30)
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET_DIR="${TARGET_DIR:-/srv/yotuvia/api}"
SERVICE="${SERVICE:-yotuvia-api}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:8080/api/healthz}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-30}"
ARCH="${ARCH:-amd64}"
ARTIFACT_URL="${ARTIFACT_URL:-}"
ARTIFACT_FILE="${ARTIFACT_FILE:-}"
BIN_NAME="yotuvia-api"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

if [ ! -f "$TARGET_DIR/.env" ]; then
  echo "ERROR: $TARGET_DIR/.env not found." >&2
  echo "       Copy .env.example there, fill it in, and chmod 600. Refusing to deploy." >&2
  exit 1
fi

BIN_SRC=""

if [ -n "$ARTIFACT_URL" ]; then
  ASSET="$(basename "$ARTIFACT_URL")"
  echo "==> Downloading $ASSET"
  curl -fsSL -o "$WORK/$ASSET" "$ARTIFACT_URL"

  if curl -fsSL -o "$WORK/$ASSET.sha256" "$ARTIFACT_URL.sha256" 2>/dev/null; then
    echo "==> Verifying checksum"
    (cd "$WORK" && sha256sum -c "$ASSET.sha256")
  else
    echo "WARNING: no $ASSET.sha256 published next to the artifact; skipping checksum." >&2
  fi

  tar -xzf "$WORK/$ASSET" -C "$WORK" "$BIN_NAME"
  BIN_SRC="$WORK/$BIN_NAME"

elif [ -n "$ARTIFACT_FILE" ]; then
  if [ ! -f "$ARTIFACT_FILE" ]; then
    echo "ERROR: artifact not found: $ARTIFACT_FILE" >&2
    exit 1
  fi
  echo "==> Extracting $ARTIFACT_FILE"
  tar -xzf "$ARTIFACT_FILE" -C "$WORK" "$BIN_NAME"
  BIN_SRC="$WORK/$BIN_NAME"

else
  echo "==> Building $BIN_NAME from source (static)"
  ( cd "$REPO_DIR" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$WORK/$BIN_NAME" . )
  BIN_SRC="$WORK/$BIN_NAME"
fi

if [ ! -x "$BIN_SRC" ]; then
  echo "ERROR: no executable binary at $BIN_SRC; the archive layout is wrong." >&2
  exit 1
fi

# root needs no sudo; otherwise sudo is required for the install and restart.
if [ "$(id -u)" -eq 0 ]; then
  SUDO=""
else
  SUDO="sudo"
fi

echo "==> Installing into $TARGET_DIR"
# install next to the target, then mv: rename is atomic, so the running process
# keeps the old inode until systemd restarts it and a failed copy never leaves
# a truncated binary in place.
$SUDO install -m 0755 "$BIN_SRC" "$TARGET_DIR/$BIN_NAME.new"
$SUDO mv -f "$TARGET_DIR/$BIN_NAME.new" "$TARGET_DIR/$BIN_NAME"

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
