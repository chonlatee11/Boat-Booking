# Walking Skeleton — Boat-Booking

**Phase:** 1
**Generated:** 2026-09-26

## Capability Proven End-to-End

A developer POSTs a boat through Kong with a dev JWT; `catalog` writes the boat and a `catalog.BoatUpserted` outbox row in one Postgres transaction; the outbox relay publishes it to Redpanda; `schedule` applies it exactly once into its own database (`processed_events`); the whole path is one trace in Tempo with `trace_id`-correlated logs in Loki; and the Next.js page (TH/EN) lists the boat by calling the backend only through Kong.

Proof commands (end of phase): `make up && make proof && make kong-roundtrip`.

## Architectural Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Backend framework | Go 1.25, one binary per service: chi HTTP + connect-go handlers + outbox relay + Kafka consumer under `errgroup` (D-04) | One container per service; relay/consumer toggled by env (`RELAY_ENABLED`, `CONSUMER_ENABLED`) |
| Module layout | `go.work` with ONE `./pkg` module, `./gen/go` module, one module per `services/<name>` (D-02); every service `go.mod` has `require` + `replace` for `pkg` and `gen/go` | `go mod tidy` and `GOWORK=off` Docker builds both work; import paths never change |
| Service template | `services/_template` is a real, CI-tested service; `make new-service name=<svc>` = `cp -r` + `sed __NAME__` (D-03) | Template cannot rot; new service costs business-logic time only |
| Data layer | PostgreSQL 17, one instance, one DB + one role per service, role can CONNECT only to its own DB (D-18, D-25); goose SQL migrations run by a separate `migrate-<svc>` step (D-26); sqlc (`pgx/v5`) per service (D-15) | Database-per-service enforced by Postgres, not convention |
| Messaging | Redpanda; franz-go + kotel; transactional outbox (poll 500 ms + nudge, batch 100, `FOR UPDATE SKIP LOCKED`) (D-10, D-11); `pkg/kafka.Consumer.Handle` wraps `processed_events` + handler in one tx, manual offset commit, 3 retries (1s/5s/25s) then `<topic>.dlq` (D-12, D-13) | At-least-once delivery, exactly-once effect, ordering per `aggregate_id` |
| Wire contract | Kafka value = proto `Envelope{event_id(uuid v7), event_type, aggregate_id, occurred_at, version, Any payload}`; `traceparent` only in Kafka headers (D-06); `event_type = <svc>.<Message>`, topic `<svc>.events` | One-way door, recorded; PII-free by construction |
| Schemas | Single buf module `proto/`; `proto/events/<svc>/v1`, `proto/services/<svc>/v1`; Go in `gen/go/<svc>/v1`, TS in `gen/ts`; generated code committed (D-07, D-09) | Go and TS share one source of truth |
| Auth | RS256 JWT: access 15 min + refresh 30 days, httpOnly/Secure/SameSite=Lax cookies (D-27, D-31); Kong 3.9.1 DB-less verifies at the edge; the gateway (BFF) re-verifies with `pkg/auth`, sets `X-User-Id`/`X-Operator-Id`/`X-Role` + `X-Internal-Token`; services trust claim headers only with the matching internal token on the internal network (D-29, D-30) | Private key lives only in `.env` (dev) / identity (Phase 2) |
| Gateway fallback | Traefik + BFF-only JWT verification if the Kong spike fails (D-28, pre-approved) | BFF already verifies, so the swap is config-only |
| Frontend | Next.js 16 App Router, TS strict, Tailwind v4 + shadcn/ui, next-intl `/th` `/en` (default `th`), IBM Plex Sans Thai, TanStack Query + one `apiFetch()` to Kong, client-only data fetching (D-32..D-36) | Mobile-first PWA base; SEO rendering deferred to Phase 7 |
| Observability | OTel SDK in every service → one OTel Collector → Tempo / Loki / Prometheus → Grafana (provisioned datasources + `platform.json`) (D-20, D-50..D-53); slog JSON with `service`, `env`, `trace_id`, `span_id` | One trace across HTTP + outbox + Kafka |
| Deployment | Docker Compose profiles: `make up` (infra + `app` + `web`), `make up-infra` + `make run-<svc>` (D-19); root `Dockerfile` `ARG SERVICE`, distroless, `-healthcheck` subcommand (D-37) | Same compose runs locally and on one EC2 |
| CI | Jenkins controller + SSH agent (docker.sock) + Harbor in `deploy/ci/docker-compose.yml` (D-21); `Jenkinsfile` stages = `make` targets, changed-services scoping, push `:sha` to Harbor only on `main` (D-22) | `make ci` reproduces Jenkins locally |
| Directory layout | `pkg/`, `gen/{go,ts}/`, `proto/`, `services/{_template,gateway,catalog,schedule}/` each with `cmd/`, `internal/{domain,app,adapters/{postgres,kafka,http}}`, `migrations/`; `deploy/`, `apps/web/` | Matches seed §12 |

## Stack Touched in Phase 1

- [ ] Project scaffold (framework, build, lint, test runner) — plans 01, 02, 05, 09, 10
- [ ] Routing — at least one real route: Kong `/api` + `/api/v1/public`, gateway `GET /api/v1/whoami`, `GET /api/v1/public/boats`, `POST /api/v1/boats` — plans 01, 11
- [ ] Database — real write (`catalog` boats + outbox, `schedule` boats projection + processed_events) and real read (`ListBoats`) — plans 10, 11
- [ ] UI — boat list with refresh button wired through TanStack Query → Kong — plan 05
- [ ] Deployment — `make up` full local stack, `make proof` end-to-end — plan 12

## Out of Scope (Deferred to Later Slices)

- OTP login, identity service, refresh-token endpoint + rotation (Phase 2)
- Catalog CRUD beyond boat upsert/list: operators, piers, routes, prices, admin UI (Phase 2)
- Schedule templates and departures (Phase 3); booking, holds, capacity race (Phase 4)
- Payment, ticket, notification, `docker-compose.prod.yml`, production Redpanda durability flags (Phase 5)
- DLQ viewer/replay UI (Phase 6) — Phase 1 has `make dlq-list` only
- PWA service worker, SSE, SEO/server-rendered data (Phase 7)
- Kubernetes, MSK, LISTEN/NOTIFY relay (Growth)

## Subsequent Slice Plan

Each later phase adds one vertical slice on top of this skeleton without altering its architectural decisions:

- Phase 2: customer OTP login + staff roles; pier_admin manages piers/routes/boats/prices; public search lists piers/routes
- Phase 3: schedule templates generate departures that admins manage; `schedule.Departure*` events
- Phase 4: search → hold seats → mock checkout with no-overbook proven under concurrency in CI
- Phase 5: real payment, QR ticket, email, full booking saga, production deploy
