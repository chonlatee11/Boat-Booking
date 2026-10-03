#!/usr/bin/env bash
# End-to-end proof of OTP login through the edge (AUTH-01, AUTH-02, AUTH-03,
# D-09, D-10): Kong's unauthenticated/rate-limited api-auth route -> gateway
# cookie routes -> identity, reading the real code from Mailpit rather than
# stubbing it. Prints PASS/FAIL per check, exits 1 on any FAIL.
set -euo pipefail

BASE="${KONG_URL:-http://localhost:8000}"
MAILPIT="${MAILPIT_URL:-http://localhost:8025}"
JAR="$(mktemp)"
trap 'rm -f "$JAR" /tmp/auth-roundtrip-verify.json' EXIT

failed=0
check() {
	local name="$1" ok="$2"
	if [ "$ok" = "1" ]; then
		echo "PASS $name"
	else
		echo "FAIL $name"
		failed=1
	fi
}

# mailpit_code polls Mailpit's search API for a message delivered to $1 and
# prints the single 6-digit OTP code in its body, or returns 1 after ~15s.
mailpit_code() {
	local to="$1"
	local deadline=$(($(date +%s) + 15))
	while [ "$(date +%s)" -lt "$deadline" ]; do
		id=$(curl -s -G "$MAILPIT/api/v1/search" --data-urlencode "query=to:\"$to\"" | jq -r '.messages[0].ID // empty')
		if [ -n "$id" ]; then
			code=$(curl -s "$MAILPIT/api/v1/message/$id" | jq -r '.Text' | grep -oE '[0-9]{6}' | head -1)
			if [ -n "$code" ]; then
				echo "$code"
				return 0
			fi
		fi
		sleep 1
	done
	return 1
}

# (a) admin-origin CORS preflight: api-auth carries no jwt plugin but still
# sits behind the global cors plugin, whose origins now include the admin
# app (D-18).
headers=$(curl -s -i -X OPTIONS \
	-H "Origin: http://localhost:3002" \
	-H "Access-Control-Request-Method: POST" \
	"$BASE/api/v1/auth/otp/request")
if echo "$headers" | grep -qi "access-control-allow-origin: http://localhost:3002" &&
	echo "$headers" | grep -qi "access-control-allow-credentials: true"; then
	check "cors-preflight-admin" 1
else
	check "cors-preflight-admin" 0
fi

DEST="e2e-$(date +%s)@example.com"

# (b) first send succeeds.
status=$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' \
	-d "{\"destination\":\"$DEST\"}" "$BASE/api/v1/auth/otp/request")
[ "$status" = "200" ] && check "otp-request-200" 1 || check "otp-request-200" 0

# (c) an immediate repeat is rejected by the 60s cooldown (D-03, mapped to
# HTTP 429 by pkg/httpx's ResourceExhausted status).
status=$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' \
	-d "{\"destination\":\"$DEST\"}" "$BASE/api/v1/auth/otp/request")
[ "$status" = "429" ] && check "otp-cooldown-429" 1 || check "otp-cooldown-429" 0

# (d) the real code arrives at Mailpit.
CODE=$(mailpit_code "$DEST") && check "mailpit-code" 1 || check "mailpit-code" 0

# (e) a wrong code reports the D-03 attempts-left count.
WRONG="000000"
[ "$WRONG" = "$CODE" ] && WRONG="111111"
wrong_body=$(curl -s -X POST -H 'Content-Type: application/json' \
	-d "{\"destination\":\"$DEST\",\"code\":\"$WRONG\"}" "$BASE/api/v1/auth/otp/verify")
if echo "$wrong_body" | jq -e '.attemptsLeft == 4' >/dev/null 2>&1; then
	check "verify-wrong-400" 1
else
	check "verify-wrong-400" 0
fi

# (f) the correct code sets both cookies.
verify_headers=$(curl -s -D - -o /tmp/auth-roundtrip-verify.json -c "$JAR" -X POST \
	-H 'Content-Type: application/json' -d "{\"destination\":\"$DEST\",\"code\":\"$CODE\"}" \
	"$BASE/api/v1/auth/otp/verify")
if grep -qi '^set-cookie: access_token=.*httponly' <<<"$verify_headers" &&
	grep -qi '^set-cookie: refresh_token=.*httponly' <<<"$verify_headers" &&
	grep -q "access_token" "$JAR" && grep -q "refresh_token" "$JAR"; then
	check "verify-200-cookies" 1
else
	check "verify-200-cookies" 0
fi

# (g) whoami accepts the cookie session and reports the customer role.
whoami_body=$(curl -s -b "$JAR" "$BASE/api/v1/whoami")
if echo "$whoami_body" | jq -e '.role == "customer"' >/dev/null 2>&1; then
	check "whoami-customer" 1
else
	check "whoami-customer" 0
fi

# jar_value prints the (last, i.e. most recent) value of cookie $1 stored in
# the Netscape-format jar file $2.
jar_value() {
	awk -F'\t' -v n="$1" '$6==n{v=$7} END{print v}' "$2"
}

# (h) refresh rotates: the jar's refresh_token value changes, response is 200.
OLD_REFRESH=$(jar_value refresh_token "$JAR")
refresh_status=$(curl -s -o /dev/null -w '%{http_code}' -b "$JAR" -c "$JAR" -X POST "$BASE/api/v1/auth/refresh")
NEW_REFRESH=$(jar_value refresh_token "$JAR")
if [ "$refresh_status" = "200" ] && [ -n "$NEW_REFRESH" ] && [ "$NEW_REFRESH" != "$OLD_REFRESH" ]; then
	check "refresh-rotates" 1
else
	check "refresh-rotates" 0
fi

# (i) replaying the now-rotated old refresh token is rejected.
status=$(curl -s -o /dev/null -w '%{http_code}' -b "refresh_token=$OLD_REFRESH" -X POST "$BASE/api/v1/auth/refresh")
[ "$status" = "401" ] && check "refresh-reuse-401" 1 || check "refresh-reuse-401" 0

# (j) the reuse above revoked the whole family — the newest token is dead too.
status=$(curl -s -o /dev/null -w '%{http_code}' -b "refresh_token=$NEW_REFRESH" -X POST "$BASE/api/v1/auth/refresh")
[ "$status" = "401" ] && check "refresh-after-reuse-401" 1 || check "refresh-after-reuse-401" 0

# (k) a fresh login, then logout, then that same token is refused.
LOGOUT_JAR="$(mktemp)"
DEST2="e2e-logout-$(date +%s)@example.com"
curl -s -o /dev/null -X POST -H 'Content-Type: application/json' \
	-d "{\"destination\":\"$DEST2\"}" "$BASE/api/v1/auth/otp/request"
if CODE2=$(mailpit_code "$DEST2"); then
	curl -s -o /dev/null -c "$LOGOUT_JAR" -X POST -H 'Content-Type: application/json' \
		-d "{\"destination\":\"$DEST2\",\"code\":\"$CODE2\"}" "$BASE/api/v1/auth/otp/verify"
	logout_status=$(curl -s -o /dev/null -w '%{http_code}' -b "$LOGOUT_JAR" -X POST "$BASE/api/v1/auth/logout")
	[ "$logout_status" = "204" ] && check "logout-204" 1 || check "logout-204" 0

	LOGOUT_REFRESH=$(jar_value refresh_token "$LOGOUT_JAR")
	status=$(curl -s -o /dev/null -w '%{http_code}' -b "refresh_token=$LOGOUT_REFRESH" -X POST "$BASE/api/v1/auth/refresh")
	[ "$status" = "401" ] && check "refresh-after-logout-401" 1 || check "refresh-after-logout-401" 0
else
	check "logout-204" 0
	check "refresh-after-logout-401" 0
fi
rm -f "$LOGOUT_JAR"

# (l) SUPER_ADMIN_EMAIL always signs in as super_admin with no piers (D-09,
# AUTH-02). $SUPER_ADMIN_EMAIL comes from the Makefile's `-include .env` +
# `export`, never read directly by this script.
if [ -n "${SUPER_ADMIN_EMAIL:-}" ]; then
	ADMIN_JAR="$(mktemp)"
	curl -s -o /dev/null -X POST -H 'Content-Type: application/json' \
		-d "{\"destination\":\"$SUPER_ADMIN_EMAIL\"}" "$BASE/api/v1/auth/otp/request"
	if ADMIN_CODE=$(mailpit_code "$SUPER_ADMIN_EMAIL"); then
		curl -s -o /dev/null -c "$ADMIN_JAR" -X POST -H 'Content-Type: application/json' \
			-d "{\"destination\":\"$SUPER_ADMIN_EMAIL\",\"code\":\"$ADMIN_CODE\"}" "$BASE/api/v1/auth/otp/verify"
		admin_whoami=$(curl -s -b "$ADMIN_JAR" "$BASE/api/v1/whoami")
		if echo "$admin_whoami" | jq -e '.role == "super_admin" and .pier_ids == []' >/dev/null 2>&1; then
			check "super-admin-login" 1
		else
			check "super-admin-login" 0
		fi
	else
		check "super-admin-login" 0
	fi
	rm -f "$ADMIN_JAR"
else
	check "super-admin-login" 0
fi

exit $failed
