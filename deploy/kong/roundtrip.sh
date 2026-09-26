#!/usr/bin/env bash
# Repeatable Kong spike acceptance script (D-28). Prints PASS/FAIL per check
# and exits 1 if any check fails.
set -euo pipefail

BASE="${KONG_URL:-http://localhost:8000}"
DEVTOKEN="go run github.com/chonlatee11/boat-booking/pkg/auth/cmd/devtoken token"

failed=0

check() {
	local name="$1"
	local ok="$2"
	if [ "$ok" = "1" ]; then
		echo "PASS $name"
	else
		echo "FAIL $name"
		failed=1
	fi
}

# (a) no cookie -> 401
status=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/api/v1/whoami")
[ "$status" = "401" ] && check "no-token-401" 1 || check "no-token-401" 0

# (b) foreign-key token -> 401
tok=$($DEVTOKEN -foreign-key)
status=$(curl -s -o /dev/null -w '%{http_code}' --cookie "access_token=$tok" "$BASE/api/v1/whoami")
[ "$status" = "401" ] && check "foreign-key-401" 1 || check "foreign-key-401" 0

# (c) expired token -> 401
tok=$($DEVTOKEN -age 1h)
status=$(curl -s -o /dev/null -w '%{http_code}' --cookie "access_token=$tok" "$BASE/api/v1/whoami")
[ "$status" = "401" ] && check "expired-401" 1 || check "expired-401" 0

# (d) refresh-kind token -> 401 (BFF expects access)
tok=$($DEVTOKEN -kind refresh)
status=$(curl -s -o /dev/null -w '%{http_code}' --cookie "access_token=$tok" "$BASE/api/v1/whoami")
[ "$status" = "401" ] && check "refresh-kind-401" 1 || check "refresh-kind-401" 0

# (e) valid token -> 200 with operator_id
tok=$($DEVTOKEN)
body=$(curl -s --cookie "access_token=$tok" "$BASE/api/v1/whoami")
status=$(curl -s -o /dev/null -w '%{http_code}' --cookie "access_token=$tok" "$BASE/api/v1/whoami")
if [ "$status" = "200" ] && echo "$body" | grep -q "operator_id"; then
	check "valid-token-200" 1
else
	check "valid-token-200" 0
fi

# (f) CORS preflight
headers=$(curl -s -i -X OPTIONS \
	-H "Origin: http://localhost:3001" \
	-H "Access-Control-Request-Method: GET" \
	"$BASE/api/v1/whoami")
if echo "$headers" | grep -qi "access-control-allow-origin: http://localhost:3001" \
	&& echo "$headers" | grep -qi "access-control-allow-credentials: true"; then
	check "cors-preflight" 1
else
	check "cors-preflight" 0
fi

# (g) rate-limit header present on the 200 response
headers=$(curl -s -i --cookie "access_token=$tok" "$BASE/api/v1/whoami")
if echo "$headers" | grep -qi "^ratelimit-limit:"; then
	check "ratelimit-header" 1
else
	check "ratelimit-header" 0
fi

# (i) public boats without a token -> 200 (the "-> stub service" hop of
# PLAT-10 is now catalog).
status=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/api/v1/public/boats")
[ "$status" = "200" ] && check "public-boats-200" 1 || check "public-boats-200" 0

# (j) POST /api/v1/admin/boatbooking.catalog.v1.CatalogService/UpsertBoat with
# a valid (pier_admin) token -> 200 (connect success is always 200; D-18).
status=$(curl -s -o /dev/null -w '%{http_code}' --cookie "access_token=$tok" \
	-X POST -H 'Content-Type: application/json' \
	-d '{"name":"Roundtrip Boat","defaultCapacity":10,"status":"BOAT_STATUS_ACTIVE"}' \
	"$BASE/api/v1/admin/boatbooking.catalog.v1.CatalogService/UpsertBoat")
[ "$status" = "200" ] && check "admin-proxy-upsert-boat-200" 1 || check "admin-proxy-upsert-boat-200" 0

# (k) whoami returns pier_ids for a devtoken minted with -pier-ids (D-06).
pier_tok=$($DEVTOKEN -pier-ids 00000000-0000-0000-0000-0000000000b1,00000000-0000-0000-0000-0000000000b2)
pier_body=$(curl -s --cookie "access_token=$pier_tok" "$BASE/api/v1/whoami")
if echo "$pier_body" | jq -e '.pier_ids == ["00000000-0000-0000-0000-0000000000b1","00000000-0000-0000-0000-0000000000b2"]' >/dev/null 2>&1; then
	check "whoami-pier-ids" 1
else
	check "whoami-pier-ids" 0
fi

# (l) admin proxy rejects a customer-role token -> 403 (T-02-04-02).
cust_tok=$($DEVTOKEN -role customer)
status=$(curl -s -o /dev/null -w '%{http_code}' --cookie "access_token=$cust_tok" \
	-X POST -H 'Content-Type: application/json' \
	-d '{"name":"x","defaultCapacity":1,"status":"BOAT_STATUS_ACTIVE"}' \
	"$BASE/api/v1/admin/boatbooking.catalog.v1.CatalogService/UpsertBoat")
[ "$status" = "403" ] && check "admin-proxy-customer-403" 1 || check "admin-proxy-customer-403" 0

# (m) LAST: send up to 130 authorized requests, PASS when a 429 appears
got429=0
for i in $(seq 1 130); do
	status=$(curl -s -o /dev/null -w '%{http_code}' --cookie "access_token=$tok" "$BASE/api/v1/whoami")
	if [ "$status" = "429" ]; then
		got429=1
		break
	fi
done
check "rate-limit-429" "$got429"

exit $failed
