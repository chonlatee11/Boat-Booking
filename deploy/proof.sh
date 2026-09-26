#!/usr/bin/env bash
# End-to-end proof of the walking skeleton (PLAT-05, PLAT-06): POST a boat
# via Kong -> gateway -> catalog (tx + outbox) -> Redpanda -> schedule
# projection applied exactly once, then confirm the whole hop is one Tempo
# trace with correlated Loki logs. Prints PASS/FAIL per check, exits 1 on
# any FAIL (or non-zero from an underlying command, via -e).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

COMPOSE="docker compose --env-file .env -f deploy/docker-compose.yml -f deploy/docker-compose.services.yml"
BASE="${KONG_URL:-http://localhost:8000}"
GRAFANA="${GRAFANA_URL:-http://localhost:3000}"

failed=0
check() {
	name="$1"
	ok="$2"
	if [ "$ok" = "1" ]; then
		echo "PASS $name"
	else
		echo "FAIL $name"
		failed=1
	fi
}

# devtoken's default operator/role (pier_admin) is fine here — proof only
# needs a validly-signed access token, not a specific tenant.
TOKEN=$(go run github.com/chonlatee11/boat-booking/pkg/auth/cmd/devtoken token)
NAME="Proof Boat $(date +%s)"
BODY=$(printf '{"name":"%s","defaultCapacity":42,"status":"BOAT_STATUS_ACTIVE"}' "$NAME")

http_code=$(curl -s -o /tmp/proof-upsert.json -w '%{http_code}' \
	-X POST "$BASE/api/v1/admin/boatbooking.catalog.v1.CatalogService/UpsertBoat" \
	--cookie "access_token=$TOKEN" \
	-H 'Content-Type: application/json' \
	-d "$BODY")

if [ "$http_code" != "200" ]; then
	echo "FAIL upsert-200 (got $http_code): $(cat /tmp/proof-upsert.json)"
	exit 1
fi

BOAT_ID=$(jq -r '.boat.boatId // empty' /tmp/proof-upsert.json)
if [ -z "$BOAT_ID" ]; then
	echo "FAIL upsert-200: no boat.boatId in response: $(cat /tmp/proof-upsert.json)"
	exit 1
fi
echo "proof.sh: boat id = $BOAT_ID"

# PASS applied: schedule's own projection holds the same capacity within 30s.
applied=0
for _ in $(seq 1 30); do
	cap=$($COMPOSE exec -T postgres psql -U schedule -d schedule -tAc \
		"select default_capacity from boats where boat_id='$BOAT_ID'" 2>/dev/null | tr -d '[:space:]')
	if [ "$cap" = "42" ]; then
		applied=1
		break
	fi
	sleep 1
done
check "applied" "$applied"

# PASS exactly-once: catalog's outbox event_id appears exactly once in
# schedule.processed_events.
once=0
EVENT_ID=$($COMPOSE exec -T postgres psql -U catalog -d catalog -tAc \
	"select event_id from outbox where aggregate_id='$BOAT_ID'" 2>/dev/null | tr -d '[:space:]')
if [ -n "$EVENT_ID" ]; then
	count=$($COMPOSE exec -T postgres psql -U schedule -d schedule -tAc \
		"select count(*) from processed_events where event_id='$EVENT_ID'" 2>/dev/null | tr -d '[:space:]')
	[ "$count" = "1" ] && once=1
fi
check "exactly-once" "$once"

# PASS public-list: GET /api/v1/public/boats lists it.
listed=0
list_body=$(curl -s "$BASE/api/v1/public/boats")
echo "$list_body" | grep -q "$BOAT_ID" && listed=1
check "public-list" "$listed"

# PASS single-trace: one Tempo trace (by span attribute aggregate_id) whose
# spans come from gateway, catalog and schedule.
GRAFANA_AUTH="admin:${GRAFANA_ADMIN_PASSWORD}"
TRACE_ID=""
SERVICES=""
for _ in $(seq 1 60); do
	search=$(curl -s -u "$GRAFANA_AUTH" \
		--get "$GRAFANA/api/datasources/proxy/uid/tempo/api/search" \
		--data-urlencode "q={ span.aggregate_id = \"$BOAT_ID\" }")
	TRACE_ID=$(printf '%s' "$search" | jq -r '.traces[0].traceID // empty')
	if [ -n "$TRACE_ID" ]; then
		SERVICES=$(printf '%s' "$search" | jq -r '.traces[0].serviceStats | keys | sort | join(",")')
		break
	fi
	sleep 1
done

single_trace=0
if [ -n "$TRACE_ID" ]; then
	has_gateway=0
	has_catalog=0
	has_schedule=0
	case ",$SERVICES," in *,gateway,*) has_gateway=1 ;; esac
	case ",$SERVICES," in *,catalog,*) has_catalog=1 ;; esac
	case ",$SERVICES," in *,schedule,*) has_schedule=1 ;; esac
	if [ "$has_gateway" = "1" ] && [ "$has_catalog" = "1" ] && [ "$has_schedule" = "1" ]; then
		single_trace=1
	fi
fi
check "single-trace" "$single_trace"
if [ "$single_trace" = "1" ]; then
	echo "proof.sh: trace id = $TRACE_ID (services: $SERVICES)"
fi

# PASS logs-correlated: Loki holds >=1 schedule log line carrying that
# trace_id.
logs_correlated=0
if [ -n "$TRACE_ID" ]; then
	now_ns=$(($(date +%s) * 1000000000))
	start_ns=$((now_ns - 3600 * 1000000000))
	logs=$(curl -s -u "$GRAFANA_AUTH" \
		--get "$GRAFANA/api/datasources/proxy/uid/loki/loki/api/v1/query_range" \
		--data-urlencode "query={service_name=\"schedule\"} | trace_id=\"$TRACE_ID\"" \
		--data-urlencode "start=$start_ns" --data-urlencode "end=$now_ns")
	count=$(printf '%s' "$logs" | jq -r '[.data.result[].values[]] | length' 2>/dev/null || echo 0)
	[ "${count:-0}" -ge 1 ] 2>/dev/null && logs_correlated=1
fi
check "logs-correlated" "$logs_correlated"

exit $failed
