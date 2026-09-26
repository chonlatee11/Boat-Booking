---
phase: 01-platform-foundation
reviewed: 2026-09-26T00:00:00Z
depth: standard
files_reviewed: 133
files_reviewed_list:
  - .dockerignore
  - .env.example
  - .gitignore
  - .golangci.yml
  - Dockerfile
  - Jenkinsfile
  - Makefile
  - apps/web/.env.example
  - apps/web/.gitignore
  - apps/web/.prettierignore
  - apps/web/eslint.config.mjs
  - apps/web/next.config.ts
  - apps/web/postcss.config.mjs
  - apps/web/src/app/[locale]/layout.tsx
  - apps/web/src/app/[locale]/page.tsx
  - apps/web/src/app/[locale]/providers.tsx
  - apps/web/src/app/favicon.ico
  - apps/web/src/app/globals.css
  - apps/web/src/components/boat-list.tsx
  - apps/web/src/components/locale-switcher.tsx
  - apps/web/src/i18n/navigation.ts
  - apps/web/src/i18n/request.ts
  - apps/web/src/i18n/routing.ts
  - apps/web/src/lib/api.ts
  - apps/web/src/lib/utils.ts
  - apps/web/src/proxy.ts
  - buf.gen.yaml
  - buf.yaml
  - deploy/ci/agent/Dockerfile
  - deploy/ci/agent/entrypoint.sh
  - deploy/ci/changed-services.sh
  - deploy/ci/changed-services_test.sh
  - deploy/ci/ci-keys.sh
  - deploy/ci/docker-compose.yml
  - deploy/ci/harbor-prepare.sh
  - deploy/ci/harbor/harbor.yml.tmpl
  - deploy/ci/jenkins/Dockerfile
  - deploy/ci/jenkins/casc.yaml
  - deploy/ci/jenkins/plugins.txt
  - deploy/ci/smoke.sh
  - deploy/compose/service.yml.tmpl
  - deploy/docker-compose.yml
  - deploy/kong/kong.yml.tmpl
  - deploy/kong/roundtrip.sh
  - deploy/migrate/Dockerfile
  - deploy/observability/check.sh
  - deploy/observability/grafana/provisioning/dashboards/dashboards.yaml
  - deploy/observability/grafana/provisioning/datasources/datasources.yaml
  - deploy/observability/loki.yaml
  - deploy/observability/otel-collector.yaml
  - deploy/observability/prometheus.yml
  - deploy/observability/tempo.yaml
  - deploy/postgres/init.sh
  - deploy/postgres/isolation-check.sh
  - deploy/proof.sh
  - deploy/redpanda/topics.sh
  - deploy/services.txt
  - go.work
  - lefthook.yml
  - pkg/auth/auth.go
  - pkg/auth/auth_test.go
  - pkg/auth/cmd/devtoken/main.go
  - pkg/clock/clock.go
  - pkg/clock/clock_test.go
  - pkg/events/events.go
  - pkg/events/events_test.go
  - pkg/go.mod
  - pkg/httpx/claims.go
  - pkg/httpx/claims_test.go
  - pkg/httpx/connect.go
  - pkg/httpx/env.go
  - pkg/httpx/errors.go
  - pkg/httpx/health.go
  - pkg/httpx/health_test.go
  - pkg/httpx/logger.go
  - pkg/httpx/middleware.go
  - pkg/httpx/otel.go
  - pkg/httpx/otel_test.go
  - pkg/kafka/consumer.go
  - pkg/kafka/consumer_integration_test.go
  - pkg/kafka/producer.go
  - pkg/money/money.go
  - pkg/money/money_test.go
  - pkg/outbox/outbox.go
  - pkg/outbox/outbox_integration_test.go
  - pkg/pgx/pgx.go
  - pkg/testenv/images_test.go
  - pkg/testenv/testenv.go
  - proto/events/catalog/v1/boat.proto
  - proto/events/platform/v1/envelope.proto
  - proto/pii-check.sh
  - proto/services/catalog/v1/catalog.proto
  - services/_template/cmd/main.go
  - services/_template/cmd/main_integration_test.go
  - services/_template/go.mod
  - services/_template/internal/adapters/http/routes.go
  - services/_template/internal/adapters/kafka/handlers.go
  - services/_template/internal/adapters/postgres/queries/pings.sql
  - services/_template/internal/adapters/postgres/sqlc.yaml
  - services/_template/internal/app/ping.go
  - services/_template/internal/domain/errors.go
  - services/_template/internal/domain/ping.go
  - services/_template/migrations/00001_platform.sql
  - services/_template/migrations/00002_pings.sql
  - services/catalog/cmd/main.go
  - services/catalog/cmd/main_integration_test.go
  - services/catalog/go.mod
  - services/catalog/internal/adapters/http/routes.go
  - services/catalog/internal/adapters/kafka/handlers.go
  - services/catalog/internal/adapters/postgres/queries/boats.sql
  - services/catalog/internal/adapters/postgres/sqlc.yaml
  - services/catalog/internal/app/boat.go
  - services/catalog/internal/domain/boat.go
  - services/catalog/internal/domain/errors.go
  - services/catalog/migrations/00001_platform.sql
  - services/catalog/migrations/00002_boats.sql
  - services/gateway/cmd/main.go
  - services/gateway/go.mod
  - services/gateway/internal/adapters/http/bff.go
  - services/gateway/internal/adapters/http/bff_test.go
  - services/gateway/internal/adapters/kafka/handlers.go
  - services/schedule/cmd/main.go
  - services/schedule/cmd/main_integration_test.go
  - services/schedule/go.mod
  - services/schedule/internal/adapters/http/routes.go
  - services/schedule/internal/adapters/kafka/handlers.go
  - services/schedule/internal/adapters/postgres/queries/boats.sql
  - services/schedule/internal/adapters/postgres/sqlc.yaml
  - services/schedule/internal/app/boat.go
  - services/schedule/internal/domain/boat.go
  - services/schedule/internal/domain/errors.go
  - services/schedule/migrations/00001_platform.sql
  - services/schedule/migrations/00002_boats.sql
findings:
  critical: 1
  warning: 3
  info: 2
  total: 6
status: issues_found
---

# Phase 01: Code Review Report

**Reviewed:** 2026-09-26T00:00:00Z
**Depth:** standard
**Files Reviewed:** 133
**Status:** issues_found

## Summary

Reviewed every listed source file for Phase 1 (platform foundation): the shared
`pkg/*` libraries (auth, httpx claims/trust-boundary, kafka, outbox, pgx,
events, clock, money, testenv), the `catalog`/`schedule`/`gateway`/`_template`
services end to end (domain/app/adapters/migrations), the gateway BFF's
claim-forwarding trust boundary, Kong/Postgres/CI deployment config, and the
Next.js frontend shell.

Overall the platform code is careful and well tested — the JWT issuer/verifier,
the trust-boundary middleware (`RequireInternal`/`RequireClaims`/
`ForwardClaims`), the Kafka consumer's idempotency+DLQ path, and
database-per-service isolation all have dedicated integration tests that
actually exercise the adversarial cases (foreign-signing key, alg confusion,
expired tokens, cross-operator `boat_id` reuse, uncommitted-record
redelivery, cross-database `permission denied`). No hardcoded secrets,
`eval`/injection sinks, or missing-`operator_id`-scoping bugs were found in
the reviewed service code.

One genuine correctness bug was found in `pkg/outbox`'s relay (see CR-01):
an unparseable outbox row aborts the whole batch transaction *after* already
publishing earlier rows in the batch to Kafka, which discards their
`published_at` progress and both (a) causes those already-delivered rows to
be republished forever and (b) permanently head-of-line-blocks every row
after it for that service, with no skip/DLQ path — unlike the symmetric,
carefully-handled case in `pkg/kafka`'s consumer. Three further points
(missing role-based authorization, an incomplete HTTP status mapping in
`httpx.WriteError`, and two minor ops-script fragility issues) are recorded
below.

## Critical Issues

### CR-01: Outbox relay discards already-published rows' progress on an unmarshal failure, causing duplicate republish + permanent head-of-line block

**File:** `pkg/outbox/outbox.go:236-253` (`Relay.publishOnce`)
**Issue:**
`publishOnce` selects a batch of unpublished rows (ordered by `id`) and loops
over them inside one `bbpgx.WithTx` closure:

```go
published := make([]int64, 0, len(batch))
for _, row := range batch {
    env, err := events.Unmarshal(row.payload)
    if err != nil {
        return fmt.Errorf("outbox: unmarshal row %d: %w", row.id, err)   // <-- BUG
    }
    ...
    if err := r.producer.Publish(rowCtx, row.topic, row.aggregateID, env); err != nil {
        r.log.Warn("outbox: publish failed, stopping batch", ...)
        r.publishErrors.Add(ctx, 1)
        break                                                            // correct: preserves `published` so far
    }
    published = append(published, row.id)
}
if len(published) == 0 {
    return nil
}
if _, err := tx.Exec(ctx, `update outbox set published_at = now() where id = any($1)`, published); err != nil {
    return fmt.Errorf("outbox: mark published: %w", err)
}
return nil
```

The Kafka-publish-failure branch correctly `break`s the loop, so whatever
rows already succeeded earlier in the same batch still get marked
`published_at` before the transaction commits. The `events.Unmarshal` failure
branch instead `return`s an error straight out of the `WithTx` closure. Since
`bbpgx.WithTx` rolls back on any non-nil error (`pkg/pgx/pgx.go:39-41`), this
discards the whole transaction — including the `UPDATE ... published_at`
step that would have recorded any rows *already sent to Kafka* earlier in
the same `batch` loop (rows are processed in ascending `id` order, and the
unmarshal check runs before the Kafka publish, so any row before the
corrupt one has already been durably produced via `ProduceSync`/`acks=all`
by the time this error fires).

Concretely, if row A (older `id`) publishes successfully and row B (younger
`id`, later in the same batch) has a corrupt/unparseable `payload`:
1. Row A is durably published to Kafka, but its `published_at` is never
   committed (transaction rolled back) — the next poll re-selects row A as
   still-unpublished and republishes it to Kafka again. This repeats on
   every `PollInterval` tick forever (idempotent consumers absorb the
   duplicates, but this is unbounded, continuous duplicate production).
2. Row B (and every row after it in `id` order, since the same batch query
   `ORDER BY id LIMIT BatchSize` always resurfaces the same head-of-line
   corrupt row first) never gets published — it is retried identically,
   forever, since there is no skip/DLQ path for an unmarshal failure. This
   is a permanent availability regression for that service's entire outbox,
   not just the affected aggregate.

This is the exact scenario `pkg/kafka/consumer.go`'s `processRecord`
explicitly defends against on the consumer side (a comment there reads:
*"our own producer never emits malformed envelopes; a corrupt record here
can't be fixed by retrying. Log and commit past it rather than wedge the
partition forever"* — and it does so by logging and returning `true` to
commit past it). The outbox relay has no equivalent, despite carrying the
identical risk profile (and D-11's own doc comment on this function assumes
only the Kafka-publish-failure case needs "stopping the batch", not the
decode-failure case).

**Fix:** Treat an unmarshal failure the same way the consumer does — log
and skip past the poison row (mark it published, or move it to a dedicated
dead letter, but never let it block the batch or roll back rows that
already made it to Kafka):

```go
for _, row := range batch {
    env, err := events.Unmarshal(row.payload)
    if err != nil {
        r.log.Error("outbox: unmarshal failed, skipping poison row",
            "id", row.id, "event_id", row.eventID, "event_type", row.eventType, "error", err)
        published = append(published, row.id) // never retryable — mark it and move on
        continue
    }
    ...
}
```
At minimum, change `return fmt.Errorf(...)` to the same `break` used by the
publish-failure branch so already-published rows in this batch are not
forgotten — though `break` alone still permanently stalls every row after
the poison one; the skip-and-continue form above is needed to fully match
the consumer side's guarantee.

## Warnings

### WR-01: `Role` claim is carried end-to-end but never enforced — any authenticated user can write catalog boats for their operator

**File:** `services/gateway/internal/adapters/http/bff.go:78-114` (`upsertBoatHandler`), `services/catalog/internal/adapters/http/routes.go:48-56` (`UpsertBoat`)
**Issue:** `pkg/auth.Claims`, `pkg/httpx.Claims`, and the `X-Role` header
all carry a `Role` field, and `ForwardClaims` forwards it faithfully
end-to-end. However, a grep across every non-test `.go` file shows `Role`
is only ever *set* or *forwarded* — no handler anywhere compares it against
an allowed set of values before performing a write. `upsertBoatHandler`
checks only that the access-cookie JWT is valid (`v.Verify(tok,
auth.KindAccess)`); it never inspects `claims.Role`. `catalog`'s
`UpsertBoat` likewise only checks that claims are present
(`httpx.FromContext`), not what `Role` they carry. Any user holding a
validly-signed access token — regardless of role — can create/update boats
for their own `operator_id`.
**Fix:** If `pier_admin` vs. some lower-privilege role (e.g. a future
`staff`/`customer` role) is meant to gate catalog writes, add an explicit
check (e.g. `if claims.Role != "pier_admin" { return
connect.NewError(connect.CodePermissionDenied, ...) }`) in
`upsertBoatHandler` and/or `UpsertBoat`. If role-based authorization is
intentionally deferred to a later phase (only one role, `pier_admin`,
exists today), leave a `// TODO`/tracked backlog item so this isn't
silently forgotten once a second role is introduced.

### WR-02: `httpx.WriteError`'s status mapping is incomplete — unmapped connect codes get HTTP 500 while still leaking their real error message

**File:** `pkg/httpx/errors.go:18-50`
**Issue:** The `switch code` only maps 6 of connect's ~17 codes to a
specific HTTP status; every other code (e.g. `CodeFailedPrecondition`,
`CodeResourceExhausted`, `CodeAborted`, `CodeOutOfRange`,
`CodeUnimplemented`, `CodeDataLoss`, `CodeCanceled`,
`CodeDeadlineExceeded`) falls through to `status := http.StatusInternalServerError`.
Separately, the message is only replaced with the generic `"internal
error"` when `code == connect.CodeInternal || code == connect.CodeUnknown`
(`errors.go:38`). For any of those unmapped-but-not-Internal codes, the
response is therefore HTTP 500 **plus** the real, specific error message —
an inconsistent combination (a 500 status implies "don't show internal
detail", but the message is shown anyway) and a misleading status code for
clients (e.g. `ResourceExhausted` semantically maps to 429, not 500). No
current handler in this phase emits one of the unmapped codes, so this is
latent rather than actively triggered, but it is a real logic gap a future
handler will hit silently.
**Fix:** Either extend the `switch` to cover the codes services can
realistically emit (at minimum `CodeFailedPrecondition` → 412,
`CodeResourceExhausted` → 429, `CodeAborted` → 409, `CodeDeadlineExceeded` →
504), or make the message-hiding condition match the status-hiding
condition exactly (e.g. hide the message whenever `status ==
http.StatusInternalServerError`, not just for the two named codes) so the
two never drift apart again.

### WR-03: `deploy/postgres/init.sh` interpolates `$SERVICE_DB_PASSWORD` unescaped into a single-quoted SQL string literal

**File:** `deploy/postgres/init.sh:42-50`
**Issue:**
```sh
psql_admin <<SQL
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '$svc') THEN
    CREATE ROLE $svc LOGIN PASSWORD '$SERVICE_DB_PASSWORD';
  END IF;
END
\$\$;
SQL
```
`$svc` is validated against `^[a-z][a-z0-9]*$` before use (good), but
`$SERVICE_DB_PASSWORD` is substituted directly inside a single-quoted SQL
literal with no escaping. If an operator sets a password containing a
single quote (`'`) or backslash, this breaks the generated SQL (syntax
error, provisioning fails) rather than merely being cosmetically wrong.
Low severity — the value is operator-controlled (`.env`), not
attacker-controlled — but it's a real robustness gap for a script explicitly
documented as idempotent/re-runnable infra provisioning.
**Fix:** Escape embedded single quotes before interpolation, e.g.
`escaped=$(printf '%s' "$SERVICE_DB_PASSWORD" | sed "s/'/''/g")` and use
`$escaped` in the `CREATE ROLE` statement, or generate/require passwords
from an alphanumeric-only charset (as `deploy/ci/ci-keys.sh` already does
via `tr -dc 'A-Za-z0-9'`) and document that constraint for
`SERVICE_DB_PASSWORD` too.

## Info

### IN-01: `Makefile`'s `push` target passes the Harbor password through `echo | docker login --password-stdin`

**File:** `Makefile:310-311`
**Issue:**
```make
push:
	@echo "$(HARBOR_PASSWORD)" | docker login localhost:8880 -u "$(HARBOR_USER)" --password-stdin
```
`--password-stdin` is the recommended way to avoid a password appearing as a
`docker login` CLI argument, but the password still appears as an argument
to `echo` in the same pipeline, which is briefly visible to other local
users via `ps`/`/proc` on a multi-user CI host. Minor, since this only runs
on the single dedicated Jenkins agent container in this design.
**Fix:** `printf '%s' "$(HARBOR_PASSWORD)" | docker login ...` avoids `echo`
quoting/flag edge cases but doesn't remove the `ps` visibility; if this
matters for the deployment target, pipe from a file descriptor or use
`docker login --password-stdin < <(printf ...)` via a credential helper
instead.

### IN-02: `pkg/httpx/otel.go`'s span-attribute allowlist is a hand-maintained prefix list that silently drops future attributes rather than failing closed loudly in tests

**File:** `pkg/httpx/otel.go:91-115`
**Issue:** `spanAttrAllowlist` is the single source of truth for which span
attributes are allowed to leave the process (D-45, PII exposure control).
It's well-tested for the attributes used today, but it's a plain string/
prefix list with no compile-time link to what instrumentation libraries
(`otelhttp`, `kotel`, future manual `SetAttributes` calls) actually emit —
a new library attribute (e.g. `otelhttp`'s `http.request.header.*` if ever
enabled) added later without a matching allowlist entry is silently
dropped, which is the *safe* failure direction for PII, but also means a
legitimately useful new attribute added by a future contributor won't
appear in traces with no error or warning pointing at why.
**Fix:** No change required for correctness/security (fails safe). Consider
a code comment at the call site of `SetAttributes` reminding contributors
that new attributes must be added to `spanAttrAllowlist` or they'll be
silently dropped — this is a maintainability note only.

---

_Reviewed: 2026-09-26T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
