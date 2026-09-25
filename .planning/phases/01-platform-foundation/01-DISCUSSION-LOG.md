# Phase 1: Platform Foundation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-26
**Phase:** 01-platform-foundation
**Areas discussed:** Event flow proof + service template, Dev stack/observability/CI, Kong spike + JWT round-trip, Next.js skeleton, Event envelope + proto layout, Outbox relay + DLQ, internal/ + sqlc + config, Redpanda topics + Postgres provisioning, Run everything locally via Docker, Health/ready + shutdown, Shared helpers in pkg, Lint + quality gates, Grafana/metrics + log rules

---

## Event flow proof + service template

| Option | Description | Selected |
|--------|-------------|----------|
| catalog → schedule real skeleton | Real `BoatUpserted`, Phase 2 builds on it | ✓ |
| ping → pong throwaway | Deleted after phase | |
| Single service self-consume | Shortest, no cross-DB proof | |

| Option | Description | Selected |
|--------|-------------|----------|
| One `./pkg` module + module per service | Fewer go.mod files, per-service Docker build | ✓ |
| Single module whole repo | Simplest but conflicts with go.work in seed | |
| Module per `pkg/*` and per service | Maximum separation, many go.mod | |

| Option | Description | Selected |
|--------|-------------|----------|
| cp `services/_template` + sed | Template compiles and is CI-tested | ✓ |
| Go text/template generator | More flexible, template not compiled | |

| Option | Description | Selected |
|--------|-------------|----------|
| One binary, goroutines via errgroup | 1 service = 1 container | ✓ |
| Subcommands serve/relay/consume | More containers | |

| Option | Description | Selected |
|--------|-------------|----------|
| `00001_platform.sql` copied per service | goose owns all DDL | ✓ |
| pkg/outbox self-migrates on boot | Two DDL paths | |

**User's choice:** All recommended options.

---

## Dev stack, observability & CI

| Option | Description | Selected |
|--------|-------------|----------|
| PostgreSQL 17 | Research recommendation | ✓ |
| PostgreSQL 16 | Seed text | |
| PostgreSQL 18 | Newest | |

| Option | Description | Selected |
|--------|-------------|----------|
| Valkey | BSD-3, go-redis compatible | ✓ |
| Redis 8 | AGPLv3 | |

| Option | Description | Selected |
|--------|-------------|----------|
| Infra in Compose, services both ways (profiles) | `make up` / `make up-infra` + go run | ✓ |
| All Docker + air hot reload | | |
| Infra only, go run always | | |

| Option | Description | Selected |
|--------|-------------|----------|
| OTel Collector + full LGTM | | ✓ |
| Tempo + Grafana first | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Jenkins in Compose on same EC2 | Not existing yet | ✓ |
| Existing Jenkins | | |
| Jenkinsfile local-only until Phase 5 | | |

| Option | Description | Selected |
|--------|-------------|----------|
| All services every push | | |
| Only changed services (git diff paths) | pkg/proto/gen/template change = rebuild all | ✓ |

| Option | Description | Selected |
|--------|-------------|----------|
| `//go:build integration` tag | `make test` vs `make test-integration` | ✓ |
| `testing.Short()` | | |

**User's choice:** Recommended except "changed services only" for CI scope.
**Notes:** User raised (free text): use **Harbor** instead of ECR as the image registry to ease a future Kubernetes move. Follow-up: Harbor runs in Compose on the same single EC2 as Jenkins + app stack (option 1 of 3).

---

## Kong spike + JWT round-trip

| Option | Description | Selected |
|--------|-------------|----------|
| `pkg/auth` issuer + `make dev-token` | Reused by identity in Phase 2 | ✓ |
| jwt.io / external script | | |

| Option | Description | Selected |
|--------|-------------|----------|
| RS256 | Kong/BFF hold public key only | ✓ |
| HS256 | Shared secret everywhere | |

| Option | Description | Selected |
|--------|-------------|----------|
| Narrow criteria (jwt/route/cors/rate-limit), 1-day timebox → Traefik | | ✓ |
| Skip spike, Traefik now | | |
| Minimal criteria (jwt + route only) | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Scaffold from template + claim→header + 1 real route | | ✓ |
| whoami stub only | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Network isolation + `X-Internal-Token` | | ✓ |
| Network isolation only | | |
| mTLS | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Single 24h cookie, no refresh | | |
| Access 15 min + refresh 30 days | | ✓ |

| Option | Description | Selected |
|--------|-------------|----------|
| `X-User-Id`, `X-Operator-Id`, `X-Role` | | ✓ |
| Single `X-Claims` JSON header | | |

**User's choice:** Recommended except session = 15-min access + 30-day refresh.

---

## Next.js skeleton

| Option | Description | Selected |
|--------|-------------|----------|
| Next.js 16 | | ✓ |
| Next.js 15 | | |

| Option | Description | Selected |
|--------|-------------|----------|
| next-intl, `[locale]` prefix always | | ✓ |
| next-intl, th without prefix | | |
| Hand-rolled | | |

| Option | Description | Selected |
|--------|-------------|----------|
| IBM Plex Sans Thai | | ✓ |
| Noto Sans Thai | | |
| Prompt | | |

| Option | Description | Selected |
|--------|-------------|----------|
| fetch → BFF REST + protoc-gen-es types | | ✓ |
| connect-es client | | |

**User's choice:** Recommended options.
**Notes:** User raised (free text): client-side data fetching with **TanStack Query**. Follow-up on RSC: chose **client-only** (option 2) — Server Components do layout/shell only; SEO deferred to Phase 7.

---

## Event envelope + proto layout

| Option | Description | Selected |
|--------|-------------|----------|
| proto Envelope + `Any` payload | | ✓ |
| No envelope, metadata in headers | | |
| Envelope + bytes + registry | | |

| Option | Description | Selected |
|--------|-------------|----------|
| `proto/events/<svc>/v1` + `proto/services/<svc>/v1` | | ✓ |
| `proto/<svc>/v1` merged | | |

| Option | Description | Selected |
|--------|-------------|----------|
| key = aggregate_id; headers traceparent/event_type/event_id | | ✓ |
| key = aggregate_id; traceparent only | | |

| Option | Description | Selected |
|--------|-------------|----------|
| buf lint + breaking + gen diff | | ✓ |
| buf lint + gen diff | | |

**User's choice:** All recommended.

---

## Outbox relay + DLQ behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Poll 500ms + in-process nudge | | ✓ |
| Poll 200ms only | | |
| LISTEN/NOTIFY + poll | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Batch 100, SKIP LOCKED, mark, 7-day retention | | ✓ |
| Delete immediately | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Retry 3× backoff 1s/5s/25s → DLQ + ERROR + `make dlq-list` | | ✓ |
| Retry 3× no backoff | | |

| Option | Description | Selected |
|--------|-------------|----------|
| pkg/kafka enforces processed_events in handler tx | | ✓ |
| Service calls helper | | |

**User's choice:** All recommended.

---

## internal/ structure + sqlc + config

| Option | Description | Selected |
|--------|-------------|----------|
| Thin layers, no interfaces/mocks | | ✓ |
| Full hexagonal + mocks | | |

| Option | Description | Selected |
|--------|-------------|----------|
| sqlc per service under adapters/postgres | | ✓ |
| Root sqlc.yaml | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Env only, small helpers | | ✓ |
| koanf/envconfig | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Single root `.env` | | ✓ |
| `.env.example` per service | | |

**User's choice:** All recommended.

---

## Redpanda topics + Postgres DB provisioning

| Option | Description | Selected |
|--------|-------------|----------|
| rpk script on `make up`, auto-create off | | ✓ |
| Broker auto-create | | |
| Service creates on boot | | |

| Option | Description | Selected |
|--------|-------------|----------|
| init SQL: role + DB per service | | ✓ |
| Single superuser | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Service auto-migrates on start | | |
| Separate migrate step (Compose/CI) before start | | ✓ |

**User's choice:** Recommended except migrations run as a separate step.

---

## Run everything locally via Docker

| Option | Description | Selected |
|--------|-------------|----------|
| Web in Compose (next dev + bind mount) and host `npm run dev` | | ✓ |
| Docker only | | |
| Host only | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Separate `deploy/ci/docker-compose.yml` (Jenkins + Harbor) | | ✓ |
| Profile inside `make up` | | |
| EC2 only | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Single root Dockerfile with ARG SERVICE | | ✓ |
| Dockerfile per service | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Expose Kong/Grafana/web + infra only | | ✓ |
| Expose every service | | |

**User's choice:** All recommended.

---

## Health/ready + graceful shutdown

| Option | Description | Selected |
|--------|-------------|----------|
| readyz = DB + Kafka (+Valkey where configured) | | ✓ |
| DB only | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Ordered shutdown, 15s budget | | ✓ |
| Parallel 5s | | |

| Option | Description | Selected |
|--------|-------------|----------|
| healthcheck /readyz + full depends_on | | ✓ |
| /healthz only | | |

| Option | Description | Selected |
|--------|-------------|----------|
| distroless + `-healthcheck` subcommand | | ✓ |
| alpine + curl | | |

**User's choice:** All recommended.

---

## Shared helpers in pkg

| Option | Description | Selected |
|--------|-------------|----------|
| `pkg/clock` in Phase 1 | | ✓ |
| Wait for Phase 3 | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Wait for Phase 2 (rule only) | | |
| `pkg/money` in Phase 1 | | ✓ |

| Option | Description | Selected |
|--------|-------------|----------|
| uuid v7 + connect error codes | | ✓ |
| `pkg/errs` custom | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Convention + no body logging + span allowlist | | ✓ |
| Redacting slog handler | | |

**User's choice:** Recommended except `pkg/money` built now.

---

## Lint + code quality gates

| Option | Description | Selected |
|--------|-------------|----------|
| Defaults + errcheck/govet/staticcheck/gosec/sqlclosecheck + forbidigo | | ✓ |
| Defaults only | | |

| Option | Description | Selected |
|--------|-------------|----------|
| ESLint + Prettier + tsc strict | | ✓ |
| Biome | | |

| Option | Description | Selected |
|--------|-------------|----------|
| CI only | | |
| lefthook pre-commit on changed files | | ✓ |

| Option | Description | Selected |
|--------|-------------|----------|
| Scopes pkg/proto/deploy/web/ci/template | | ✓ |
| Single platform scope | | |

**User's choice:** Recommended except lefthook pre-commit.

---

## Grafana/metrics + log rules

| Option | Description | Selected |
|--------|-------------|----------|
| Full platform metrics via OTel | | ✓ |
| HTTP only | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Provision datasources + 1 dashboard JSON | | ✓ |
| Datasources only | | |

| Option | Description | Selected |
|--------|-------------|----------|
| slog JSON + mandatory fields + text dev mode | | ✓ |
| JSON only | | |

| Option | Description | Selected |
|--------|-------------|----------|
| OTel logs bridge → Collector → Loki | | ✓ |
| stdout + filelog receiver | | |

**User's choice:** All recommended.

---

## Claude's Discretion

Middleware/interceptor order, otel bootstrap code, Makefile internals, Compose naming, Grafana panel layout, Redpanda single-node flags, `kong.yml` structure beyond spike criteria, `_template` sample endpoint.

## Deferred Ideas

- SEO/server-rendered pier/route pages — Phase 7
- Refresh-token endpoint + rotation — Phase 2
- DLQ viewer + replay — Phase 6
- LISTEN/NOTIFY relay / separate relay process — Growth
- Kubernetes — Growth (Harbor chosen to ease it)
- Prod Redpanda durability/retention/backup — Phase 5
