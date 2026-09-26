#!/usr/bin/env bash
# Idempotent Harbor v2.15.2 bootstrap (D-21): fills HARBOR_* secrets in .env,
# downloads the online installer once, renders harbor.yml, and runs
# `./prepare` to generate Harbor's own docker-compose.yml + component config.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

ENV_FILE=.env
[ -f "$ENV_FILE" ] || { echo "harbor-prepare: $ENV_FILE missing — run 'make dev-keys' first" >&2; exit 1; }

get() { grep -E "^$1=" "$ENV_FILE" | tail -1 | cut -d= -f2-; }
set_kv() {
  local key=$1 val=$2
  if grep -qE "^$key=" "$ENV_FILE"; then
    sed -i "s|^$key=.*|$key=$val|" "$ENV_FILE"
  else
    echo "$key=$val" >> "$ENV_FILE"
  fi
}

DEFAULT_DATA_DIR="$(pwd)/deploy/ci/harbor/data"

if [ -z "$(get HARBOR_ADMIN_PASSWORD)" ]; then
  set_kv HARBOR_ADMIN_PASSWORD "$(openssl rand -base64 32 | tr -dc 'A-Za-z0-9' | head -c 24)"
fi
if [ -z "$(get HARBOR_DB_PASSWORD)" ]; then
  set_kv HARBOR_DB_PASSWORD "$(openssl rand -base64 32 | tr -dc 'A-Za-z0-9' | head -c 24)"
fi
if [ -z "$(get HARBOR_DATA_DIR)" ]; then
  set_kv HARBOR_DATA_DIR "$DEFAULT_DATA_DIR"
fi

# Re-read what we just wrote (never trust the process's own stale env).
HARBOR_ADMIN_PASSWORD=$(get HARBOR_ADMIN_PASSWORD)
HARBOR_DB_PASSWORD=$(get HARBOR_DB_PASSWORD)
HARBOR_DATA_DIR=$(get HARBOR_DATA_DIR)

INSTALLER_DIR=deploy/ci/harbor/installer
if [ ! -f "$INSTALLER_DIR/docker-compose.yml" ]; then
  echo "harbor-prepare: downloading Harbor v2.15.2 installer"
  TMP=$(mktemp -d)
  trap 'rm -rf "$TMP"' EXIT
  curl -fsSL -o "$TMP/harbor-installer.tgz" \
    https://github.com/goharbor/harbor/releases/download/v2.15.2/harbor-online-installer-v2.15.2.tgz
  tar -xzf "$TMP/harbor-installer.tgz" -C "$TMP"
  mkdir -p "$INSTALLER_DIR"
  cp -r "$TMP/harbor/." "$INSTALLER_DIR/"

  mkdir -p "$HARBOR_DATA_DIR"

  # envsubst-equivalent: replace only our three placeholders (no new tool
  # dependency; the template has no other ${...} tokens).
  sed \
    -e "s|\${HARBOR_ADMIN_PASSWORD}|$HARBOR_ADMIN_PASSWORD|g" \
    -e "s|\${HARBOR_DB_PASSWORD}|$HARBOR_DB_PASSWORD|g" \
    -e "s|\${HARBOR_DATA_DIR}|$HARBOR_DATA_DIR|g" \
    deploy/ci/harbor/harbor.yml.tmpl > "$INSTALLER_DIR/harbor.yml"

  (cd "$INSTALLER_DIR" && ./prepare)
fi

echo "harbor-prepare: ready"
