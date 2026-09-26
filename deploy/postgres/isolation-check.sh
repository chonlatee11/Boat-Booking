#!/bin/sh
# Proves database-per-service isolation (D-25): every service role can
# connect to its own database, and is denied CONNECT to every other
# service's database. For each ordered pair of distinct services in
# deploy/services.txt: `-U <a> -d <b>` must FAIL with "permission denied for
# database"; `-U <a> -d <a>` must succeed. Prints PASS/FAIL per check,
# exits 1 on any FAIL. Run against the stack brought up by `make up` or
# `make up-infra`.
set -u

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

# No shell-sourcing of .env needed: local Postgres connections authenticate
# by role privilege (pg_hba trust for the compose network), not password,
# and `--env-file .env` below is docker compose's own substitution — it
# reads the file itself, it does not need it sourced into this shell.
COMPOSE="docker compose --env-file .env -f deploy/docker-compose.yml -f deploy/docker-compose.services.yml"
SERVICES_FILE="deploy/services.txt"

services=$(grep -v '^[[:space:]]*#' "$SERVICES_FILE" | grep -v '^[[:space:]]*$')

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

for a in $services; do
  out=$($COMPOSE exec -T postgres psql -U "$a" -d "$a" -tAc 'select 1' 2>&1)
  if [ "$(printf '%s' "$out" | tr -d '[:space:]')" = "1" ]; then
    check "own-db-access $a/$a" 1
  else
    check "own-db-access $a/$a" 0
    echo "  $out"
  fi

  for b in $services; do
    [ "$a" = "$b" ] && continue
    out=$($COMPOSE exec -T postgres psql -U "$a" -d "$b" -tAc 'select 1' 2>&1)
    rc=$?
    if [ $rc -ne 0 ] && printf '%s' "$out" | grep -qi "permission denied for database"; then
      check "cross-db-denied $a->$b" 1
    else
      check "cross-db-denied $a->$b" 0
      echo "  $out"
    fi
  done
done

exit $failed
