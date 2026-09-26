#!/usr/bin/env bash
# Fills empty CI-only secrets in .env (idempotent — never rotates an existing
# value). Run via `make ci-keys`, which depends on `dev-keys` so the new keys
# already exist (empty) in .env before this script runs (D-21).
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

ENV_FILE=.env
[ -f "$ENV_FILE" ] || { echo "ci-keys: $ENV_FILE missing — run 'make dev-keys' first" >&2; exit 1; }

get() { grep -E "^$1=" "$ENV_FILE" | tail -1 | cut -d= -f2-; }
set_kv() {
  local key=$1 val=$2
  if grep -qE "^$key=" "$ENV_FILE"; then
    sed -i "s|^$key=.*|$key=$val|" "$ENV_FILE"
  else
    echo "$key=$val" >> "$ENV_FILE"
  fi
}

if [ -z "$(get JENKINS_ADMIN_PASSWORD)" ]; then
  set_kv JENKINS_ADMIN_PASSWORD "$(openssl rand -base64 32 | tr -dc 'A-Za-z0-9' | head -c 24)"
fi

if [ -z "$(get JENKINS_AGENT_SSH_PRIVATE_KEY_B64)" ]; then
  KEY_DIR=$(mktemp -d)
  trap 'rm -rf "$KEY_DIR"' EXIT
  ssh-keygen -t ed25519 -N '' -C jenkins-agent -f "$KEY_DIR/agent_key" -q
  set_kv JENKINS_AGENT_SSH_PRIVATE_KEY_B64 "$(base64 -w0 "$KEY_DIR/agent_key")"
  set_kv JENKINS_AGENT_SSH_PUBKEY "$(cat "$KEY_DIR/agent_key.pub")"
fi

if [ -z "$(get DOCKER_GID)" ]; then
  set_kv DOCKER_GID "$(stat -c %g /var/run/docker.sock)"
fi

if [ -z "$(get CI_REPO_URL)" ]; then
  set_kv CI_REPO_URL "file:///repo"
fi

echo "ci-keys: .env ready"
