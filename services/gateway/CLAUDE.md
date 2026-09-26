# gateway — Agent Guide

The BFF behind Kong. Scaffolded from the same one-binary runtime as every
other service (`services/_template`) — see `services/_template/CLAUDE.md`
for the shared layout — but with no DATABASE_URL, so the outbox relay and
Kafka consumer are disabled by env (D-04, D-29).

## Owns

- No data. gateway has no Postgres database and no migrations.

## Publishes

- Nothing yet.

## Consumes

- Nothing — `internal/adapters/kafka/handlers.go`'s `Register` is
  intentionally empty, kept only for template parity.

## Sync API

- `GET /api/v1/whoami` — verified claims only (reads the `access_token`
  cookie itself; not a proxy call).
- `GET /api/v1/public/boats` — proxies `CatalogService.ListBoats` with only
  the internal token, no claims (a public catalog-wide read).
- `POST /api/v1/boats` — verifies the `access_token` cookie, then proxies
  `CatalogService.UpsertBoat` over connect-go with verified claims + the
  internal token forwarded (D-29, D-30).

## Trust Rules (D-27, D-29, D-30)

- Kong verifies the JWT's signature at the edge (JWT plugin); the gateway
  **re-verifies independently** with `pkg/auth.Verifier` and the public key
  only — it never holds the private key (D-27).
- Every route here is mounted directly on the router — **not** behind
  `httpx.RequireInternal` — because the gateway is where the internal-token
  trust boundary *originates*, not a consumer of it. Every other service's
  routes ARE behind `httpx.RequireInternal`.
- Before any outbound connect-go call, `httpx.ForwardClaims` (moved from this
  package to `pkg/httpx`, D-06) deletes whatever
  `X-User-Id`/`X-Operator-Id`/`X-Role`/`X-Pier-Ids`/`X-Internal-Token` the
  client sent and sets them fresh from verified claims plus the gateway's own
  configured `INTERNAL_TOKEN` — a client can never spoof its way past a
  downstream service's trust boundary (Anti-Pattern 2). `X-Pier-Ids` is the
  comma-joined uuid list from `Claims.PierIDs`, set only when non-empty.

## Rules

- Use `pkg/clock.Now()`, never `time.Now()` directly (lint-enforced).
- No personal data (PII) in logs or span attributes — ids and non-personal
  fields only (D-45).

## Layout

- `cmd/main.go` — the one binary: HTTP (chi), the outbox relay and Kafka
  consumer goroutines (both always disabled here — no `DATABASE_URL`), and
  ordered shutdown, identical in shape to every other service.
- `internal/adapters/http/bff.go` — `Routes` and the claim-verifying handlers
  (calls `httpx.ForwardClaims`, which now lives in `pkg/httpx`).
- `internal/adapters/kafka/handlers.go` — empty `Register` (template
  parity; gateway consumes nothing).

## Commands

- `make run-gateway` — run this service locally against the infra stack
  (`go run ./cmd`, DB/Kafka hosts overridden to localhost — unused here).
- `make test-integration` — run every module's `//go:build integration`
  tests against real Postgres + Redpanda testcontainers.

## Commit Scope

Commits touching only this service use `gateway` as the conventional-commit
scope (D-49), e.g. `feat(gateway): add booking status route`.
