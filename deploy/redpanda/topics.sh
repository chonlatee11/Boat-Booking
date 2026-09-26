#!/bin/sh
# Provisions <svc>.events (6 partitions) and <svc>.events.dlq (1 partition)
# for every service name given as an argument, or read from
# ${SERVICES_FILE:-/services.txt} (one name per line, blank lines and lines
# starting with # skipped) when no arguments are given (D-24). Shared by
# `make up` (init container) and pkg/testenv (copied into the Redpanda
# testcontainer). Idempotent: re-running against existing topics succeeds.
set -eu

RETENTION_MS="${RETENTION_MS:-259200000}" # 3 days, dev default
# Default targets the redpanda testcontainer's *internal* listener
# (127.0.0.1:9093) when this script runs inside that same container (pkg/testenv):
# Redpanda's external listener (9092) advertises the host-mapped port, which
# only resolves from the host, not from a process inside the container itself.
# `make up`'s init container overrides this via the compose service address.
RPK_BROKERS="${RPK_BROKERS:-127.0.0.1:9093}"

services="$*"
if [ "$#" -eq 0 ]; then
  services_file="${SERVICES_FILE:-/services.txt}"
  if [ -f "$services_file" ]; then
    services=$(grep -v '^\s*#' "$services_file" | grep -v '^\s*$' || true)
  else
    services=""
  fi
fi

create_topic() {
  name="$1"
  partitions="$2"
  out=$(rpk topic create "$name" -p "$partitions" -r 1 -c "retention.ms=${RETENTION_MS}" -X "brokers=${RPK_BROKERS}" 2>&1) && return 0
  case "$out" in
    *TOPIC_ALREADY_EXISTS*) return 0 ;;
    *) echo "$out" >&2; return 1 ;;
  esac
}

if [ -z "$services" ]; then
  echo "topics.sh: no services given, nothing to provision"
  exit 0
fi

for svc in $services; do
  create_topic "${svc}.events" 6
  create_topic "${svc}.events.dlq" 1
done
