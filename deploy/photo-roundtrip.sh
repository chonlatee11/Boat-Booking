#!/usr/bin/env bash
# End-to-end proof of the pier-photo presigned upload (D-19, CAT-06): presign
# through Kong's admin proxy -> browser-direct PUT to object storage -> public
# GET -> UpsertPier wires photo_url into the public API -> CORS preflight.
# Prints PASS/FAIL per check, exits 1 on any FAIL.
set -euo pipefail

BASE="${KONG_URL:-http://localhost:8000}"
PHOTO_BASE="${PHOTO_PUBLIC_BASE_URL:-http://localhost:8333/pier-photos}"
DEVTOKEN="go run github.com/chonlatee11/boat-booking/pkg/auth/cmd/devtoken token"
ADMIN="$BASE/api/v1/admin/boatbooking.catalog.v1.CatalogService"

PNG_FILE=$(mktemp)
LONG_FILE=$(mktemp)
trap 'rm -f "$PNG_FILE" "$LONG_FILE" /tmp/photo-roundtrip-*.json' EXIT

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

# 1 KiB PNG-signature file for the upload.
printf '\x89PNG\r\n\x1a\n' >"$PNG_FILE"
head -c 1016 /dev/urandom >>"$PNG_FILE"
SIZE_BYTES=$(wc -c <"$PNG_FILE" | tr -d '[:space:]')

# super_admin seeds an operator + pier through the admin proxy (D-08).
TOKEN=$($DEVTOKEN -role super_admin -operator "")
SUFFIX=$(date +%s)

op_body=$(curl -s --cookie "access_token=$TOKEN" \
	-X POST -H 'Content-Type: application/json' \
	-d "$(printf '{"name":"Photo Roundtrip Operator %s"}' "$SUFFIX")" \
	"$ADMIN/UpsertOperator")
OPERATOR_ID=$(echo "$op_body" | jq -r '.operator.operatorId // empty')

pier_body=$(curl -s --cookie "access_token=$TOKEN" \
	-X POST -H 'Content-Type: application/json' \
	-d "$(printf '{"operatorId":"%s","nameTh":"ท่าเรือทดสอบรูป","nameEn":"Photo Roundtrip Pier","lat":7.88,"lng":98.39}' "$OPERATOR_ID")" \
	"$ADMIN/UpsertPier")
PIER_ID=$(echo "$pier_body" | jq -r '.pier.pierId // empty')

if [ -z "$OPERATOR_ID" ] || [ -z "$PIER_ID" ]; then
	echo "FAIL setup: could not seed operator/pier ($op_body / $pier_body)" >&2
	exit 1
fi

# PresignPierPhoto -> presign-200.
presign_status=$(curl -s -o /tmp/photo-roundtrip-presign.json -w '%{http_code}' \
	--cookie "access_token=$TOKEN" \
	-X POST -H 'Content-Type: application/json' \
	-d "$(printf '{"contentType":"image/png","sizeBytes":"%s"}' "$SIZE_BYTES")" \
	"$ADMIN/PresignPierPhoto")
UPLOAD_URL=$(jq -r '.uploadUrl // empty' /tmp/photo-roundtrip-presign.json)
PHOTO_KEY=$(jq -r '.photoKey // empty' /tmp/photo-roundtrip-presign.json)
if [ "$presign_status" = "200" ] && [ -n "$UPLOAD_URL" ] && [ -n "$PHOTO_KEY" ]; then
	check "presign-200" 1
else
	check "presign-200" 0
	echo "presign response: $(cat /tmp/photo-roundtrip-presign.json)" >&2
	exit 1
fi

# Browser-direct PUT with the exact signed Content-Type -> put-200.
put_status=$(curl -s -o /dev/null -w '%{http_code}' -X PUT \
	-H 'Content-Type: image/png' \
	--data-binary "@$PNG_FILE" \
	"$UPLOAD_URL")
[ "$put_status" = "200" ] && check "put-200" 1 || check "put-200" 0

# Public GET returns the same bytes -> public-get-200.
get_status=$(curl -s -o /tmp/photo-roundtrip-get.bin -w '%{http_code}' "$PHOTO_BASE/$PHOTO_KEY")
if [ "$get_status" = "200" ] && cmp -s "$PNG_FILE" /tmp/photo-roundtrip-get.bin; then
	check "public-get-200" 1
else
	check "public-get-200" 0
fi

# A PUT whose Content-Length differs from the signed size is rejected -> 403.
cp "$PNG_FILE" "$LONG_FILE"
printf 'x' >>"$LONG_FILE"
wrong_status=$(curl -s -o /dev/null -w '%{http_code}' -X PUT \
	-H 'Content-Type: image/png' \
	--data-binary "@$LONG_FILE" \
	"$UPLOAD_URL")
[ "$wrong_status" = "403" ] && check "put-wrong-length-403" 1 || check "put-wrong-length-403" 0

# UpsertPier with the photoKey, then the public API carries photo_url.
upsert_status=$(curl -s -o /tmp/photo-roundtrip-upsert.json -w '%{http_code}' \
	--cookie "access_token=$TOKEN" \
	-X POST -H 'Content-Type: application/json' \
	-d "$(printf '{"pierId":"%s","operatorId":"%s","nameTh":"ท่าเรือทดสอบรูป","nameEn":"Photo Roundtrip Pier","lat":7.88,"lng":98.39,"photoKey":"%s"}' "$PIER_ID" "$OPERATOR_ID" "$PHOTO_KEY")" \
	"$ADMIN/UpsertPier")
public_body=$(curl -s "$BASE/api/v1/public/piers")
if [ "$upsert_status" = "200" ] && echo "$public_body" | jq -e --arg id "$PIER_ID" --arg key "$PHOTO_KEY" \
	'.piers[] | select(.pierId == $id) | .photoUrl | contains($key)' >/dev/null 2>&1; then
	check "public-photo-url" 1
else
	check "public-photo-url" 0
fi

# CORS preflight from the admin app origin against the storage upload URL.
headers=$(curl -s -i -X OPTIONS \
	-H "Origin: http://localhost:3002" \
	-H "Access-Control-Request-Method: PUT" \
	"$UPLOAD_URL")
if echo "$headers" | grep -qi "access-control-allow-origin: http://localhost:3002"; then
	check "cors-preflight-storage" 1
else
	check "cors-preflight-storage" 0
fi

exit $failed
