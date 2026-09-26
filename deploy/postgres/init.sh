#!/bin/sh
# Idempotent per-service Postgres role + database provisioning (D-25). Run
# once by the postgres-init one-shot container against `postgres`. Reads
# service names from SERVICES_FILE (default /services.txt; blank lines and
# lines starting with # are skipped), rejects any name not matching
# ^[a-z][a-z0-9]*$ BEFORE it is ever interpolated into SQL (T-12-04), then
# per service: creates role <svc> LOGIN PASSWORD $SERVICE_DB_PASSWORD if
# absent, creates database <svc> OWNER <svc> if absent (CREATE DATABASE
# cannot run inside a transaction block, so a DO block won't work here — the
# classic "SELECT ... WHERE NOT EXISTS ... \gexec" trick generates and runs
# it only when missing), then revokes PUBLIC's CONNECT and grants CONNECT
# only to the owning role (database-per-service enforced at the DB level).
# Re-running against an already-provisioned cluster is a no-op.
set -eu

SERVICES_FILE="${SERVICES_FILE:-/services.txt}"
PGHOST="${PGHOST:-postgres}"
PGUSER="${PGUSER:-postgres}"

psql_admin() {
  psql -v ON_ERROR_STOP=1 -h "$PGHOST" -U "$PGUSER" -d postgres "$@"
}

psql_admin -c 'REVOKE CONNECT ON DATABASE postgres FROM PUBLIC;'

services=""
if [ -f "$SERVICES_FILE" ]; then
  services=$(grep -v '^[[:space:]]*#' "$SERVICES_FILE" | grep -v '^[[:space:]]*$' || true)
fi

if [ -z "$services" ]; then
  echo "init.sh: no services given, nothing to provision"
  exit 0
fi

for svc in $services; do
  echo "$svc" | grep -Eq '^[a-z][a-z0-9]*$' || {
    echo "init.sh: invalid service name '$svc'" >&2
    exit 1
  }

  psql_admin <<SQL
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '$svc') THEN
    CREATE ROLE $svc LOGIN PASSWORD '$SERVICE_DB_PASSWORD';
  END IF;
END
\$\$;
SQL

  psql_admin <<SQL
SELECT 'CREATE DATABASE $svc OWNER $svc'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$svc') \gexec
SQL

  psql_admin -c "REVOKE CONNECT ON DATABASE $svc FROM PUBLIC;"
  psql_admin -c "GRANT CONNECT ON DATABASE $svc TO $svc;"
done

echo "init.sh: provisioned: $(printf '%s' "$services" | tr '\n' ' ')"
