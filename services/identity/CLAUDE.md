# identity — Agent Guide

Scaffolded from `services/_template` by `make new-service name=identity`
(D-03) — see `services/_template/CLAUDE.md` for the shared template rules
and layout this service still follows.

## Owns

- `users` — login identities (email and/or phone, role, operator_id,
  pier_ids). `refresh_tokens` — sha256 hashes of issued opaque refresh
  tokens, never the raw token.

## Publishes

- `identity.UserCreated` on the `identity.events` topic, keyed by `user_id` —
  emitted only the first time a destination completes `VerifyOtp` (auto-created
  as role `customer`, D-04). Ids and role only, no personal data (D-45).

## Consumes

- Nothing yet.

## Sync API

- `AuthService.RequestOtp` — needs only the internal token, no claims (the
  caller isn't authenticated yet). Always returns the same empty success for
  a well-formed destination whether or not a user exists (Pitfall 3).
- `AuthService.VerifyOtp` — needs only the internal token. Returns a signed
  access JWT (role/operator_id/pier_ids from the matched or newly-created
  user) plus an opaque refresh token on success.
- `AuthService.Refresh` — rotates the presented refresh token: revokes it and
  issues a fresh access+refresh pair, re-reading role/operator_id/pier_ids/
  disabled from the `users` row every time (D-10) — a role/pier change or
  disable takes effect at the next refresh, at most one access-token TTL
  later. Presenting an already-rotated token revokes every refresh token for
  that user (reuse detection, T-02-05-02).
- `AuthService.Logout` — revokes the presented refresh token. Idempotent: an
  unknown or already-revoked token is not an error.
- `UserService.{ListUsers,UpsertUser,SetUserDisabled}` — **super_admin only**
  (AUTH-04, D-08); every RPC needs verified claims (no claims →
  `Unauthenticated`, any other role → `PermissionDenied`). `UpsertUser`
  creates or updates a `staff`/`pier_admin` user bound to one operator and
  1-50 piers — `customer` and `super_admin` can never be assigned here (D-09
  is the only path to `super_admin`). Before persisting, it forwards the
  caller's claims + the internal token to catalog's `ListPiers` (filtered by
  the target operator) and rejects with `InvalidArgument` naming any
  requested pier that catalog didn't return non-archived under that operator
  — database-per-service forbids a foreign key, so this synchronous call is
  the only ownership check (research Pattern 3). Create is idempotent: an
  email that already belongs to staff/pier_admin/super_admin returns
  `AlreadyExists` and changes nothing; an existing `customer` email is
  promoted in place (same user id, no second `identity.UserCreated`). Update
  keeps `email` immutable and rejects a `customer`/`super_admin` target row
  with `FailedPrecondition`. `SetUserDisabled(true)` sets `disabled_at` and
  revokes every refresh token for that user in the same tx (D-10) — refresh
  fails immediately and access ends within one access-token TTL (≤15 min);
  disabling the caller's own id or a `super_admin` row is rejected with
  `FailedPrecondition`. `ListUsers` never returns `customer` rows (PDPA
  minimal exposure), ordered by `email` then `id`.

## Startup bootstrap (D-09)

`cmd/main.go` calls `app.EnsureSuperAdmin(ctx, pool, SUPER_ADMIN_EMAIL)` after
the pool is created and before the HTTP server starts accepting traffic — a
failure aborts startup. It idempotently makes that email a non-disabled
`super_admin`: inserts a fresh row (and publishes `identity.UserCreated`), or
promotes/re-enables an existing row (no second event). There is no API or CLI
path that can mint a `super_admin` — this env var is the only way.

## Rules

- Never read another service's database — every service owns its own
  Postgres database (database-per-service, enforced at the DB level).
- Publish only via `outbox.Insert` in the same `pgx.Tx` as the state change it
  describes — app/adapter code never calls `pkg/kafka.Producer` directly.
- Consume only via `kafka.Consumer.Handle` — the idempotency check
  (`processed_events`) is built into the wrapper; there is no way to bypass
  it from service code.
- Use `pkg/clock.Now()`, never `time.Now()` directly (lint-enforced), and
  `pkg/money.Satang` for any money field, never a float.
- Trust claims only via `httpx.FromContext(ctx)` — never trust a
  client-supplied operator/user id. Every operator-scoped query must filter
  by `operator_id` taken from claims.
- No personal data (PII) in events, logs, or span attributes — ids and
  non-personal business fields only (proto review checklist, D-45).
- **Never let an OTP code reach a log line, span attribute, error message,
  Postgres row, Valkey key/value, or event.** Only its HMAC may be stored
  (Valkey `otp:code:{dh}` hash field `h`) — see T-02-02-03 in the phase
  threat model. `math/rand` must never be imported in this service; codes
  come from `crypto/rand` only (T-02-02-04).

## Layout

- `cmd/` — the one binary: HTTP (chi + the `AuthService` connect handler),
  outbox relay, and Kafka consumer running as `errgroup` goroutines with
  ordered shutdown.
- `internal/domain/{user,destination,errors}.go` — `User`, `StaffUserInput`
  (+ `Validate`), `Destination`, `NormalizeDestination`, and the
  sentinel/`CodeMismatchError` errors — types and validation rules only, no
  persistence or transport concerns.
- `internal/app/{otp,session,bootstrap,users,convert}.go` —
  `Auth.RequestOtp`/`VerifyOtp`/`Refresh`/`Logout`, `Users.{UpsertUser,
  ListUsers,SetUserDisabled}` (+ `validatePiers`, the catalog call) use-case
  methods taking a `*pgxpool.Pool`/`*redis.Client`/`catalogv1connect.CatalogServiceClient`
  directly, `issueSession`, `EnsureSuperAdmin` (D-09), and sqlc row <->
  domain converters. No repository interfaces, no mocks.
- `internal/adapters/http/routes.go` — mounts the `AuthService` and
  `UserService` connect handlers, both behind the internal-token trust
  boundary set up in `cmd/main.go`. `internal/adapters/http/users.go` holds
  `UserService`'s handler methods (`routes.go` only wires the two services
  together, matching catalog's split between its route-mounting file and
  per-entity handler files).
- `internal/adapters/notify/notify.go` — `Sender` interface (`SendOtp`) plus
  the SMTP/Resend/dev-SMS implementations.
- `internal/adapters/postgres/` — sqlc-generated code from
  `queries/{users,sessions}.sql`.
- `internal/adapters/kafka/handlers.go` — `Register` (empty; identity
  consumes nothing yet).
- `migrations/` — `00001_platform.sql` (outbox + processed_events, copied
  verbatim from the template), `00002_users.sql`, `00003_refresh_tokens.sql`.

## Env vars

- `VALKEY_ADDR` — Valkey address holding every OTP code/counter (no `otps`
  table exists).
- `JWT_PRIVATE_KEY_B64`, `JWT_ISSUER` — access/refresh token signing (D-06).
- `OTP_HASH_SECRET` — the HMAC pepper (>= 32 bytes) used to hash both
  destinations (so no raw email/phone appears in a Valkey key) and codes (so
  a Postgres/Valkey dump alone can never brute-force a code offline).
- `SMTP_ADDR`, `OTP_EMAIL_FROM` — dev/CI email transport (SMTP to Mailpit).
- `RESEND_API_KEY` — prod email transport (REST, no SDK dependency).
- `SUPER_ADMIN_EMAIL` — the only email the startup bootstrap (D-09) ever
  promotes to `super_admin`.
- `CATALOG_URL` (default `http://catalog:8080`) — catalog's base URL,
  used only by `UserService.UpsertUser`'s `ListPiers` call (research
  Pattern 3).

## Valkey key scheme

- `otp:code:{dh}` — hash `{h: hex HMAC of the code, a: wrong-attempt count}`,
  TTL 300s.
- `otp:cooldown:{dh}` — `SET NX EX 60`, blocks a resend within 60s of the
  last send.
- `otp:hourly:{dh}` — `INCR` + `EXPIRE 3600` on the first increment, caps
  sends per destination per hour.
- `dh` = `hex(HMAC-SHA256(OTP_HASH_SECRET, "dest:" + normalised destination))`
  — never the raw email/phone.

## Commands

- `make run-identity` — run this service locally against the infra stack
  (`go run ./cmd`, DB/Kafka hosts overridden to localhost).
- `make migrate-identity` — run this service's goose migrations.
- `make sqlc-gen` — regenerate every service's `internal/adapters/postgres/*.go`
  from `queries/*.sql`.
- `make test-integration` — run every module's `//go:build integration`
  tests against real Postgres + Redpanda testcontainers.

## Commit Scope

Commits touching only this service use `identity` as the conventional-commit
scope (D-49), e.g. `feat(identity): add otp rate limiting`.
