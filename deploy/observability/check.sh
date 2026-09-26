#!/usr/bin/env bash
# Synthetic OTLP round-trip check: sends a span/log/metric through the
# Collector and polls Grafana's datasource proxy until it shows up in
# Tempo/Loki/Prometheus. Used by `make obs-check` (D-20, D-51).
set -euo pipefail

NETWORK="boatbooking_default"
CURL_IMAGE="curlimages/curl:8.17.0"
COLLECTOR_HTTP="http://otel-collector:4318"
GRAFANA_URL="http://localhost:3000"
GRAFANA_USER="admin"
GRAFANA_PASSWORD="${GRAFANA_ADMIN_PASSWORD:-}"
if [ -z "$GRAFANA_PASSWORD" ] && [ -f .env ]; then
  GRAFANA_PASSWORD=$(grep -E '^GRAFANA_ADMIN_PASSWORD=' .env | tail -1 | cut -d= -f2-)
fi

rand_hex() { # $1 = byte count
  head -c "$1" /dev/urandom | od -An -tx1 | tr -d ' \n'
}

now_ns() { date +%s%N; }

# send_otlp <path> <json-payload> — POSTs to the Collector's OTLP/HTTP
# endpoint from a throwaway container on the compose network (the Collector
# itself publishes no host port).
send_otlp() {
  local path="$1" payload="$2"
  docker run --rm --network "$NETWORK" "$CURL_IMAGE" \
    -sS -o /dev/null -w '%{http_code}' \
    -X POST "$COLLECTOR_HTTP$path" \
    -H 'Content-Type: application/json' \
    -d "$payload"
}

# grafana_get <path> — GETs from Grafana's own API (published on
# 127.0.0.1:3000, reachable from the host directly).
grafana_get() {
  curl -sS -u "$GRAFANA_USER:$GRAFANA_PASSWORD" "$GRAFANA_URL$1"
}

check_traces() {
  local trace_id span_id ts payload i body
  trace_id=$(rand_hex 16)
  span_id=$(rand_hex 8)
  ts=$(now_ns)
  payload=$(printf '{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"obs-check"}}]},"scopeSpans":[{"spans":[{"traceId":"%s","spanId":"%s","name":"obs-check","kind":1,"startTimeUnixNano":"%s","endTimeUnixNano":"%s"}]}]}]}' \
    "$trace_id" "$span_id" "$ts" "$ts")
  send_otlp /v1/traces "$payload" >/dev/null

  for i in $(seq 1 30); do
    body=$(grafana_get "/api/datasources/proxy/uid/tempo/api/traces/$trace_id" || true)
    if echo "$body" | grep -q "obs-check"; then
      echo "PASS traces"
      return 0
    fi
    sleep 1
  done
  echo "FAIL traces"
  return 1
}

main() {
  case "${1:-}" in
    traces) check_traces ;;
    logs) check_logs ;;
    metrics) check_metrics ;;
    all)
      local rc=0
      check_traces || rc=1
      check_logs || rc=1
      check_metrics || rc=1
      exit "$rc"
      ;;
    *)
      echo "usage: check.sh [traces|logs|metrics|all]" >&2
      exit 1
      ;;
  esac
}

main "$@"
