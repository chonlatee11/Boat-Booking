# Phase 1: Platform Foundation - Context

**Gathered:** 2026-09-26
**Status:** Ready for planning

<domain>
## Phase Boundary

A developer can scaffold a new service from a shared template (`make new-service <name>`), run the full local stack (`make up`), generate Go + TS from proto (`make proto-gen`), and see one real event flow end-to-end — `catalog.BoatUpserted` written via transactional outbox in `catalog`, applied exactly once in `schedule` via `processed_events` — as a single trace in Tempo spanning HTTP → outbox → Kafka → consumer. Failed processing retries then lands in `<topic>.dlq`. Jenkins CI builds images and runs unit + integration tests. A Next.js skeleton (TH/EN, mobile-first) calls the backend only through Kong with a verified JWT round-trip. Requirements: PLAT-01..PLAT-10.

Not in this phase: real identity/OTP (Phase 2), any catalog CRUD beyond what the proof event needs, booking logic, payment, production deploy (Phase 5).

</domain>

<decisions>
## Implementation Decisions

### Proof event flow + service template
- **D-01:** The "one real event" is `catalog.BoatUpserted` flowing from a real `services/catalog` skeleton to a real `services/schedule` skeleton, both scaffolded by `make new-service`. Nothing is thrown away; Phase 2/3 build on these. The `BoatUpserted` proto is the real one from day one. — **Reversibility:** costly — the proto and both service skeletons become the base Phase 2/3 extend.
- **D-02:** Go module layout: `go.work` with ONE `./pkg` module (subpackages `events`, `kafka`, `outbox`, `httpx`, `auth`, `pgx`, `clock`, `money`), `./gen/go` module, and one module per `services/<name>`. Not one module per `pkg/*`. — **Reversibility:** costly — every go.mod/import path depends on it.
- **D-03:** `make new-service <name>` = `cp -r services/_template services/<name>` + `sed` replacing `__NAME__`. `services/_template` is a real compiling service that CI builds and tests so the template cannot rot. No Go-based generator.
- **D-04:** One binary per service: `cmd/main.go` starts HTTP (chi + connect handlers), outbox relay and Kafka consumer as goroutines under `errgroup` with graceful shutdown. Consumer/relay can be disabled by env for services that don't need them (e.g. gateway). No subcommand-per-role.
- **D-05:** Template ships `migrations/00001_platform.sql` (outbox + processed_events) copied into every service. goose owns all DDL; `pkg/outbox`/`pkg/kafka` never run migrations themselves.

### Event envelope + proto layout
- **D-06:** Kafka value = proto `Envelope { event_id (uuid v7), event_type, aggregate_id, occurred_at, version, google.protobuf.Any payload }`. Trace context travels only in Kafka header `traceparent` (not duplicated in body). — **Reversibility:** one-way — a published wire contract every consumer decodes; changing it after Phase 2 means dual-decoding old records.
- **D-07:** Single buf module at `proto/`. Layout `proto/events/<svc>/v1/*.proto` and `proto/services/<svc>/v1/*.proto`; packages `boatbooking.<svc>.events.v1` and `boatbooking.<svc>.v1`; `go_package` → `gen/go/<svc>/v1`; TS → `gen/ts`. Generated code is committed. — **Reversibility:** costly — package names are baked into Any type URLs and generated imports.
- **D-08:** `pkg/kafka` producer contract: `Publish(ctx, topic, aggregateID, envelope)` — `aggregate_id` is a mandatory argument and becomes the record key; headers `traceparent`, `event_type`, `event_id` always set.
- **D-09:** CI runs `buf lint` (STANDARD), `buf breaking` against `main`, and a `gen/` diff check that fails if `make proto-gen` was not run.

### Outbox relay + DLQ behavior
- **D-10:** Relay wakes by polling every 500ms (env-configurable) plus an in-process nudge channel signalled after each tx commit. No `LISTEN/NOTIFY`.
- **D-11:** Relay reads batches of 100 `ORDER BY id` with `FOR UPDATE SKIP LOCKED`, publishes with `RequiredAcks(AllISR)`, sets `published_at` after broker ack; partial index `WHERE published_at IS NULL`; an hourly sweep deletes published rows older than 7 days.
- **D-12:** Consumer retries a failing handler 3× in-process with backoff 1s/5s/25s, then produces to `<topic>.dlq` (original envelope + headers `error`, `consumer_group`, `attempts`, `failed_at`, `source_topic`, `source_partition`, `source_offset`), logs at ERROR, increments `dlq_total`, and commits the source offset. `make dlq-list` wraps `rpk topic consume <topic>.dlq`.
- **D-13:** `pkg/kafka` owns idempotency: `Consumer.Handle(eventType, func(ctx, tx pgx.Tx, env) error)`. The wrapper opens a tx → `INSERT processed_events ON CONFLICT DO NOTHING` (0 rows = skip) → handler → commit tx → commit offset. Manual commit only (`kgo.DisableAutoCommit`). Services cannot forget the check.

### internal/ structure + sqlc + config
- **D-14:** Thin layers: `internal/domain` = types + rules, `internal/app` = use-case functions taking `pgx.Tx`, `internal/adapters/{postgres,kafka,http}`. No repository interfaces with a single implementation, no mocks; app code is tested against real Postgres under the integration tag.
- **D-15:** sqlc lives per service at `internal/adapters/postgres/{queries/*.sql, sqlc.yaml}` generating into that same package, `sql_package: pgx/v5`, schema read from the service's own `migrations/`. `make sqlc-gen`; generated code committed.
- **D-16:** Config is env-only via tiny helpers (`MustEnv`, `EnvOr`) in pkg; a `Config` struct per service in `cmd/main.go`; no config library. Standard names: `DATABASE_URL`, `KAFKA_BROKERS`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `HTTP_ADDR`, `INTERNAL_TOKEN`, `LOG_LEVEL`, `LOG_FORMAT`.
- **D-17:** One root `.env` (+ committed `.env.example`) used by Compose and by `make run-<svc>` (which overrides hosts to localhost). Per-service DB and topic names derive from the service name. This supersedes the seed's ".env.example per service".

### Dev stack, observability & CI
- **D-18:** PostgreSQL **17** (supersedes "Postgres 16" in ROADMAP/REQUIREMENTS wording — planner should note the doc update). Valkey instead of Redis (go-redis/v9 unchanged). Redpanda pinned tag. Kong pinned `kong:3.9.1`.
- **D-19:** Compose profiles: `make up` = infra + all Go services + web in Docker (PLAT-03); `make up-infra` = infra only, then `make run-<svc>` (`go run`) from host. No hot-reload tool for Go.
- **D-20:** Observability = OTel Collector + Tempo + Loki + Prometheus + Grafana from day one; every service exports traces, metrics and logs over OTLP to the single Collector. Config under `deploy/observability/`.
- **D-21:** Jenkins does not exist yet: run Jenkins controller + agent (docker.sock mounted so testcontainers works) in a separate `deploy/ci/docker-compose.yml` together with **Harbor** as the image registry (replaces ECR — chosen so a later Kubernetes move is easier). Same file runs locally (`make ci-up`) and on the single EC2. PLAT-08/DEP-02 wording "ECR" must be updated. — **Reversibility:** costly — Jenkinsfile push targets and EC2 pull config point at Harbor.
- **D-22:** Jenkinsfile builds/tests only services changed per `git diff` paths; any change under `pkg/`, `proto/`, `gen/`, `services/_template/`, root `go.work`, `Makefile` or the root Dockerfile rebuilds ALL services. Registry push + `:sha` tag only on `main`. Pipeline steps must also run locally via `make ci`.
- **D-23:** Tests: `//go:build integration` on `*_integration_test.go`; `make test` = `go test ./...` (no Docker); `make test-integration` = `-tags=integration`; one Postgres + Redpanda testcontainer per package via `TestMain`, fresh DB per test; images pinned to the same tags as Compose.

### Redpanda topics + Postgres DB provisioning
- **D-24:** One shared services list file drives provisioning. `deploy/redpanda/topics.sh` (init container on `make up`) creates `<svc>.events` (6 partitions) and `<svc>.events.dlq` (1 partition) with explicit retention (dev 3d); broker `auto_create_topics` disabled. `make new-service` appends to the list. testcontainers reuse the script.
- **D-25:** `deploy/postgres/init.sh` reads the same list → `CREATE ROLE <svc>` + `CREATE DATABASE <svc> OWNER <svc>`; each role can only connect to its own DB (database-per-service enforced at the DB level, not by convention).
- **D-26:** goose runs as a separate step before service start: Compose `migrate-<svc>` one-shot container (goose CLI in a small alpine migrate image, `depends_on postgres service_healthy`) and a CI step; the service binary does NOT auto-migrate. `make migrate-<svc>` for manual runs. Services `depends_on migrate-<svc> service_completed_successfully`.

### Kong spike + JWT round-trip (before identity exists)
- **D-27:** JWT = RS256. `pkg/auth` contains the real issuer (later reused by identity) and verifier; `make dev-token` mints a token with the dev keypair from `.env`; `kong.yml` has a dev consumer holding the same public key. Kong and BFF hold only the public key; the private key lives only in identity (dev: `.env`). — **Reversibility:** costly — Kong consumer config, cookie format and pkg/auth API all assume RS256.
- **D-28:** Kong spike is the first task of the phase, time-boxed to 1 day. Pass = DB-less `kong.yml` does JWT verify + reject, routing, CORS and rate-limiting. Claim→header remapping is NOT a Kong requirement (BFF does it). On failure or an Enterprise-only blocker: switch to Traefik and have the BFF verify JWT itself — no re-litigation.
- **D-29:** `services/gateway` (BFF) is scaffolded from the template (consumer/relay disabled by env). It parses the JWT from the cookie, sets `X-User-Id`, `X-Operator-Id`, `X-Role` + `X-Internal-Token`, and exposes one real REST route (e.g. `GET /api/v1/boats` → catalog connect client) so Next.js has real data to call.
- **D-30:** Downstream services trust claim headers only when (a) the request arrives on the Compose internal network (services expose no host ports) AND (b) `pkg/httpx` middleware finds `X-Internal-Token` equal to the env secret set by the BFF. Missing/mismatched → 401. Protected routes without claim headers → 401. `pkg/httpx` exposes `Claims` + `FromContext(ctx)`.
- **D-31:** Session = access JWT 15 min + refresh cookie 30 days (httpOnly, Secure, SameSite=Lax). Refresh endpoint + rotation is Phase 2 (identity) work, but `pkg/auth` must model both token kinds from Phase 1.

### Next.js skeleton
- **D-32:** Next.js **16** (App Router) — supersedes "Next.js 15" in seed text. Tailwind + shadcn/ui.
- **D-33:** i18n via `next-intl`, `app/[locale]/` with locale prefix always (`/th`, `/en`), default `th` when detection fails.
- **D-34:** Font: IBM Plex Sans Thai via `next/font/google`.
- **D-35:** Data access: one `apiFetch()` helper (base URL → Kong, `credentials: include`) wrapped by **TanStack Query**; `QueryClientProvider` in `app/[locale]/layout`. TS types from `protoc-gen-es` in `gen/ts`; no connect-es transport in the browser. **Client-only fetching**: all data fetched in Client Components; Server Components do layout/shell only. Server-rendered SEO data for pier/route pages is deferred to Phase 7.
- **D-36:** Compose profile `web` runs `next dev` with a bind mount (through Kong); `npm run dev` on the host with `NEXT_PUBLIC_API_URL` pointing at Kong on localhost is equally supported.

### Run everything locally via Docker
- **D-37:** One root `Dockerfile` with `ARG SERVICE`, multi-stage, build context = repo root, `go mod`/`go build` cache mounts, final stage `distroless/static`. The binary has a `-healthcheck` subcommand (GET localhost `/readyz`) because distroless has no shell. `services/<svc>/Dockerfile` from PLAT-01 becomes a thin stub referencing the root file or is dropped — planner notes the wording.
- **D-38:** Host-exposed ports in dev: Kong `:8000`, Grafana `:3000`, web `:3001`, plus Postgres/Redpanda/Valkey for `go run`/psql. Go services never publish host ports. Redpanda Console available under profile `tools`.

### Health/ready + graceful shutdown
- **D-39:** `/healthz` = 200 while the process is alive. `/readyz` = DB ping + Kafka broker metadata (+ Valkey only where configured), 2s timeout, result cached 1s, returns 503 with JSON `{db, kafka}` on failure.
- **D-40:** Shutdown on SIGTERM: readyz → 503 → stop accepting HTTP → consumer finishes in-flight batch and commits → relay flush → close DB pool; total budget 15s; Compose `stop_grace_period: 20s`.
- **D-41:** Compose `healthcheck` hits `/readyz`; Go services `depends_on` postgres/redpanda/valkey `service_healthy` and `migrate-<svc>` `service_completed_successfully`; Kong `depends_on` gateway healthy.

### Shared helpers in pkg (Phase 1)
- **D-42:** `pkg/clock`: `Asia/Bangkok` location, `LocalDate(t)`, `Now()` overridable in tests; boundary unit tests at 00:00–01:00 and 23:00–00:00 Bangkok. App code never calls `time.Now()` directly (lint-enforced).
- **D-43:** `pkg/money`: `type Satang int64`, `FormatBaht`, one documented rounding rule for percentage refunds with a unit test on odd amounts. Proto money fields are `int64` satang only.
- **D-44:** IDs = uuid v7 (`google/uuid`). Errors: connect error codes are canonical; `pkg/httpx.WriteError` maps code → HTTP status + JSON `{code, message}`; domain uses sentinel errors + `errors.Is`. No custom error hierarchy.
- **D-45:** PII guard by construction: `pkg/httpx` logs only method/path/status/duration/trace_id; `pkg/kafka` logs only event_id/event_type/aggregate_id; no request/event body logging at any level; span attribute allowlist in pkg instrumentation; proto review checklist rejects PII-shaped event fields.

### Lint + code quality gates
- **D-46:** Root `.golangci.yml` for all modules: defaults + `errcheck`, `govet`, `staticcheck`, `gosec`, `sqlclosecheck`, and `forbidigo` banning `time.Now` and `fmt.Print*` in app code; `gen/` excluded; CI fails on any issue.
- **D-47:** Web: ESLint (next preset) + Prettier + `tsc --noEmit` with `strict: true`; CI runs lint + typecheck + `next build`.
- **D-48:** `lefthook` pre-commit runs gofmt/goimports, `buf lint`, eslint on changed files only. CI is still the full gate (`make lint` = golangci + buf lint + eslint + tsc).
- **D-49:** Commit scopes for non-service code: `pkg | proto | deploy | web | ci | template`; services use their name. No commitlint.

### Grafana/metrics + log rules
- **D-50:** Metrics via OTel → Collector → Prometheus (no per-service `/metrics`): otelhttp request duration/count; `pkg/kafka` consumer lag + processed count; `pkg/outbox` backlog size, oldest-unpublished age, publish errors; `dlq_total`.
- **D-51:** Grafana provisioning from files: datasources (Tempo/Loki/Prometheus with trace↔log derived field on `trace_id`) + one dashboard `deploy/observability/grafana/dashboards/platform.json` (per-service HTTP, consumer lag, outbox backlog, DLQ).
- **D-52:** Logging: `pkg/httpx.NewLogger(service)` builds slog JSON with mandatory fields `service`, `env`, `trace_id`, `span_id` (+ `event_id`, `aggregate_id` in consumers). INFO default = one line per request/event; WARN = retries; ERROR = DLQ/5xx. `LOG_LEVEL` env; `LOG_FORMAT=text` for `go run`.
- **D-53:** Logs ship via the OTel logs SDK bridge from slog → Collector → Loki; stdout kept for `docker logs`.

### Claude's Discretion
- Exact chi middleware order, connect interceptor wiring, otel SDK bootstrap code, Makefile target internals, Compose service naming, Grafana panel layout, Redpanda single-node flags (per official single-broker lab config), Kong `kong.yml` structure beyond the spike criteria, `_template` sample HTTP endpoint used to trigger the proof event.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Seed + project definition
- `PROJECT.md` §5.5 (Kafka conventions), §5.6 (`pkg/` list), §9 (Technical Guardrails), §12 (target repo structure) — the seed; envelope fields, outbox/idempotency/DLQ rules, repo layout.
- `.planning/PROJECT.md` — Core value, constraints, key decisions (connect-go, Kong + BFF, Postgres single instance, Redpanda, Go outbox relay).
- `.planning/REQUIREMENTS.md` — PLAT-01..PLAT-10 (note: "Postgres 16" → 17 and "ECR" → Harbor per D-18/D-21).
- `.planning/ROADMAP.md` — Phase 1 goal + 5 success criteria.

### Research
- `.planning/research/STACK.md` — pinned versions (Go 1.25, connect-go v1.20, franz-go v1.22, testcontainers v0.44, Kong 3.9.1 OSS freeze, `@connectrpc/protoc-gen-connect-es` not the archived package), franz-go producer/consumer patterns, testcontainers module usage.
- `.planning/research/ARCHITECTURE.md` — Pattern 1 (outbox relay SQL + ordering by id), Pattern 2 (idempotent consumer), Pattern 4 (Kong + BFF claim→header), Pattern 8 (OTel over Kafka with franz-go carrier), Pattern 9 (go.work + buf), Anti-Pattern 2 (header trust boundary).
- `.planning/research/PITFALLS.md` — Pitfalls 1, 3, 6, 7, 9, 10, 11, 12, 13 all have Phase 0/1 prevention items; "Looks Done But Isn't" checklist.
- `.planning/research/SUMMARY.md` — phase rationale, research flags (Kong spike at Phase 1).

### Project conventions
- `.claude/CLAUDE.md` — stack table, Kong licensing note, franz-go/testcontainers guidance, user preferences (Thai responses, ponytail: simplest working solution, YAGNI, stdlib first).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- None — repo contains only `PROJECT.md`, `.planning/`, `.claude/`. Phase 1 creates the first code.

### Established Patterns
- None in code yet. Patterns are fixed by the decisions above and must be established in `services/_template` and `pkg/*` so every later service inherits them.

### Integration Points
- Everything later plugs into: `pkg/*` APIs (D-08, D-13, D-16, D-30, D-42..D-45), `services/_template`, the shared services list (D-24/D-25), root `Dockerfile` (D-37), `deploy/docker-compose.yml` profiles (D-19), `deploy/ci/docker-compose.yml` (D-21), `proto/` layout (D-07).

</code_context>

<specifics>
## Specific Ideas

- Time-box discipline from Pitfall 1: Phase 1 must end with `make up` + the catalog→schedule `BoatUpserted` proof + one Tempo trace crossing the Kafka hop, not "one more piece of infra".
- Kong spike first; Traefik fallback is a pre-approved decision, not a discussion.
- The `_template` must be genuinely copy-paste-and-go: adding identity/catalog in Phase 2 should cost "business logic time", not "infra time" (Pitfall 1 verification signal).
- DLQ must never be a black hole even in Phase 1: ERROR log + `dlq_total` metric + `make dlq-list`.
- User explicitly wants everything runnable locally through Docker (infra, services, web, and a separate CI compose with Jenkins + Harbor).

</specifics>

<deferred>
## Deferred Ideas

- Server-rendered / SEO data for pier and route pages (skeleton is client-only fetching) — Phase 7 (POL-03).
- Refresh-token endpoint + rotation — Phase 2 (identity), token shape reserved in `pkg/auth` now.
- DLQ viewer + replay UI — Phase 6 (REF-04); Phase 1 only has `make dlq-list`.
- Outbox relay via `LISTEN/NOTIFY` or separate relay process — only if latency or multi-instance needs arise (Growth).
- Kubernetes move — Harbor chosen partly to ease this, but k8s itself stays out of scope (GRW-06).
- Production Redpanda durability flags, topic retention sizing, backup story — Phase 5 (DEP-01).

None of the discussion strayed outside Phase 1 scope otherwise.

</deferred>

---

*Phase: 01-platform-foundation*
*Context gathered: 2026-09-26*
