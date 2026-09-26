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
- `internal/domain/{user,destination,errors}.go` — `User`, `Destination`,
  `NormalizeDestination`, and the sentinel/`CodeMismatchError` errors —
  types and validation rules only, no persistence or transport concerns.
- `internal/app/{otp,session,convert}.go` — `Auth.RequestOtp`/`VerifyOtp` use-case
  methods taking a `*pgxpool.Pool`/`*redis.Client` directly, plus `issueSession`
  and sqlc row <-> domain converters. No repository interfaces, no mocks.
- `internal/adapters/http/routes.go` — the `AuthService` connect handler,
  mounted behind the internal-token trust boundary set up in `cmd/main.go`.
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
