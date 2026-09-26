---
phase: "01"
slug: "platform-foundation"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-26"
---

# Phase 01 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| internet/browser → Kong :8000 | Untrusted requests and JWT cookies | JWT (auth credential) |
| Kong → gateway | JWT-verified `/api`; public prefix unauthenticated | Tokens, request bodies |
| gateway → downstream services | Claim headers + X-Internal-Token; services trust nothing else | Verified claims |
| developer machine → repo | Secrets stay in git-ignored `.env` | RSA private key, passwords |
| proto schema → every consumer | Wire contract reaching every log/DLQ | Event payloads (ids only) |
| buf remote plugins / npm registry → repo | Third-party code generators and packages | Generated/installed code |
| service DB → Kafka (outbox) | Outbox rows become broker records | Event envelopes |
| Kafka → consumer DB | Other services' events mutate local state | Event envelopes |
| services → Collector → Grafana | Telemetry storage and read access | Traces/logs/metrics |
| git repo → Jenkins agent (docker.sock) | Pipeline code runs with host-root-equivalent socket | Build code, CI secrets |
| Jenkins → Harbor | Credentialed image pushes (main only) | Images, registry creds |
| service → other service's DB | Must be impossible (database-per-service) | — |
| developer input → `make new-service` | Name flows into sed, paths, SQL, topics | Service name |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-01-01 | Spoofing | Kong jwt / gateway Verify | high | mitigate | `pkg/auth/auth.go:99-102` RS256 + issuer + exp; `deploy/kong/kong.yml.tmpl`; tests `pkg/auth/auth_test.go` | closed |
| T-01-02 | Spoofing | X-* claim headers | high | mitigate | `services/gateway/internal/adapters/http/bff.go:159-169` ForwardClaims; `pkg/httpx/claims.go:46-65` ConstantTimeCompare; `pkg/httpx/claims_test.go` | closed |
| T-01-03 | EoP | refresh token as access | medium | mitigate | `pkg/auth/auth.go:106-108` kind check; `auth_test.go:159` | closed |
| T-01-04 | Spoofing | alg confusion | high | mitigate | `pkg/auth/auth.go:99`; `auth_test.go:99` TestVerifyRejectsAlgConfusion | closed |
| T-01-05 | InfoDisc | RSA private key | high | mitigate | `pkg/auth/cmd/devtoken/main.go:91` 0600; `.gitignore` (.env, kong.yml); `deploy/docker-compose.yml:96-106` no env_file | closed |
| T-01-06 | DoS | Kong edge | medium | mitigate | `deploy/kong/kong.yml:39-42` rate-limiting 120/min; `roundtrip.sh:83-92` | closed |
| T-01-07 | InfoDisc | WriteError | medium | mitigate | `pkg/httpx/errors.go:37-48` fixed "internal error" | closed |
| T-01-08 | Tampering | gateway image | medium | mitigate | `deploy/docker-compose.yml:118` kong:3.9.1 | closed |
| T-01-SC | Tampering | Go modules | low | mitigate | `pkg/go.sum` committed | closed |
| T-02-01 | InfoDisc | proto/events/** | high | mitigate | `proto/pii-check.sh` via `Makefile:225` proto-check | closed |
| T-02-02 | Tampering | remote plugin output | medium | mitigate | `buf.gen.yaml` pinned; `Makefile:227-231` drift check; `gen/` committed | closed |
| T-02-03 | Tampering | breaking wire change | medium | mitigate | `Makefile:220-224` buf breaking; `envelope.proto:16` version | closed |
| T-02-SC | Tampering | buf CLI | low | mitigate | `Makefile:132` buf@v1.73.0 | closed |
| T-03-01 | Tampering | money.PercentOf | medium | mitigate | `pkg/money/money.go:22-28`; `money_test.go` | closed |
| T-03-02 | Tampering | clock.LocalDate | medium | mitigate | `pkg/clock/clock.go:16-19` time/tzdata; `clock_test.go` | closed |
| T-03-SC | Tampering | dependencies | low | accept | Stdlib only (AR-01) | closed |
| T-04-01 | Tampering | outbox bypass | medium | mitigate | `.golangci.yml:22-23` forbidigo kgo.NewClient | closed |
| T-04-02 | InfoDisc | relay/producer logs | high | mitigate | `pkg/kafka/consumer.go` ids-only log attrs | closed |
| T-04-03 | DoS | outbox growth | medium | mitigate | `pkg/outbox/outbox.go:88-130` gauges; Sweep 181-187 | closed |
| T-04-04 | Tampering | SQL in outbox | low | mitigate | Parameterised pgx in `pkg/outbox/outbox.go` | closed |
| T-04-SC | Tampering | Go modules | low | mitigate | `pkg/go.sum` | closed |
| T-05-SC | Tampering | npm installs | high | mitigate | `apps/web/package.json` exact pins; `package-lock.json` committed | closed |
| T-05-01 | InfoDisc | browser tokens | high | mitigate | `apps/web/src/lib/api.ts:8` credentials:'include'; no token storage | closed |
| T-05-02 | Tampering (XSS) | API-rendered names | medium | mitigate | No `dangerouslySetInnerHTML` in apps/web | closed |
| T-05-03 | Spoofing | direct service calls | medium | mitigate | Single `apiFetch()`; no service host ports | closed |
| T-05-04 | InfoDisc | CORS credentials | high | mitigate | `deploy/kong/kong.yml:33-38` explicit origin | closed |
| T-06-01 | EoP | docker.sock on agent | high | mitigate | `deploy/ci/docker-compose.yml:29-30`; `casc.yaml:2` 0 executors; `casc.yaml:47`; Jenkins 127.0.0.1; residual risk AR-09 | closed |
| T-06-02 | Spoofing | Jenkins UI/API | high | mitigate | `casc.yaml:4-11`; `deploy/ci/ci-keys.sh:20-22`; crumb in `smoke.sh:83-85` | closed |
| T-06-03 | Spoofing | Harbor admin | high | mitigate | `deploy/ci/harbor-prepare.sh:23-24`; `Makefile:253-258` private project | closed |
| T-06-04 | InfoDisc | CI secrets | medium | mitigate | `Jenkinsfile:83-88` withCredentials | closed |
| T-06-05 | Tampering | plugins/images/installer | medium | mitigate | `plugins.txt` pins; `harbor-prepare.sh:44` v2.15.2 HTTPS | closed |
| T-06-06 | Tampering | branch image pushes | medium | mitigate | `Jenkinsfile:82` branch 'main' | closed |
| T-06-SC | Tampering | package installs | low | accept | No npm/pip/cargo (AR-02) | closed |
| T-07-01 | Tampering | replayed events | high | mitigate | `pkg/kafka/consumer.go:261-279`; TestHandleAppliesOnce | closed |
| T-07-02 | DoS | poison message | high | mitigate | `consumer.go:226-254` retries→DLQ+commit; integration tests | closed |
| T-07-03 | Repudiation | silent drops | medium | mitigate | `consumer.go:324-330` ERROR log + dlq metric | closed |
| T-07-04 | InfoDisc | DLQ payload retention | low | accept | PII-free envelopes (AR-03) | closed |
| T-07-05 | InfoDisc | consumer logs | medium | mitigate | `consumer.go` ids-only log attrs | closed |
| T-08-01 | InfoDisc | Grafana UI | medium | mitigate | `deploy/docker-compose.yml:183-189` anon off, 127.0.0.1 | closed |
| T-08-02 | Spoofing/Tampering | OTLP endpoints | medium | mitigate | `docker-compose.yml:148-176` no host ports | closed |
| T-08-03 | InfoDisc | PII in telemetry | medium | transfer | Enforced at source (pkg/httpx, pkg/kafka — T-09-02, T-07-05) | closed |
| T-08-04 | DoS | telemetry storage | low | mitigate | 72h/3d retention; `otel-collector.yaml:10` memory_limiter | closed |
| T-08-05 | Spoofing | dev Grafana password | low | accept | Dev-only, localhost (AR-04) | closed |
| T-09-01 | Spoofing/EoP | service claim routes | high | mitigate | `services/catalog/cmd/main.go:131-134` RequireInternal; `routes.go:49-56` | closed |
| T-09-02 | InfoDisc | logs/spans | high | mitigate | `pkg/httpx/middleware.go:38-42`; `pkg/httpx/otel.go:97-121` allowlistExporter | closed |
| T-09-03 | DoS | request bodies | medium | mitigate | 4 KiB MaxBytesReader; ReadHeaderTimeout 5s; `00002_pings.sql:8` | closed |
| T-09-04 | DoS | readyz | low | mitigate | `pkg/httpx/health.go:14-15` | closed |
| T-09-05 | InfoDisc | health endpoints | low | accept | Status only, no host ports (AR-05) | closed |
| T-09-SC | Tampering | modules / sqlc image | low | mitigate | `Makefile:215` sqlc:1.31.1; go.sum | closed |
| T-10-01 | EoP | UpsertBoat scoping | high | mitigate | `queries/boats.sql:5`; `app/boat.go:33-38`; TestUpsertBoatValidationAndTenancy | closed |
| T-10-02 | Spoofing | catalog API | high | mitigate | `services/catalog/cmd/main.go:131-134`; `routes.go:49-51` | closed |
| T-10-03 | Tampering | input validation | medium | mitigate | `domain/boat.go:29-42`; `00002_boats.sql` checks | closed |
| T-10-04 | Tampering (injection) | new-service name | medium | mitigate | `Makefile:145-148` regex guard | closed |
| T-10-05 | InfoDisc | BoatUpserted payload | low | mitigate | `app/boat.go:63-69` | closed |
| T-11-01 | Spoofing | spoofed X-* headers | high | mitigate | `bff.go:159-169`; `bff_test.go:89` | closed |
| T-11-02 | Spoofing | forged/refresh tokens | high | mitigate | `bff.go:78-89`; `bff_test.go:143` | closed |
| T-11-03 | Tampering | oversized/malformed bodies | medium | mitigate | `bff.go:24,97` 16 KiB; strict protojson | closed |
| T-11-04 | Tampering | replayed BoatUpserted | high | mitigate | processed_events; `services/schedule/cmd/main_integration_test.go:183` | closed |
| T-11-05 | InfoDisc | public boats listing | low | accept | Public catalog data (AR-06) | closed |
| T-11-06 | DoS | slow catalog | medium | mitigate | `services/gateway/cmd/main.go:144` 5s timeout | closed |
| T-12-01 | Spoofing/EoP | services from host | high | mitigate | `deploy/compose/service.yml.tmpl` no ports | closed |
| T-12-02 | InfoDisc/Tampering | cross-service DB | high | mitigate | `deploy/postgres/init.sh:47-48`; `isolation-check.sh` | closed |
| T-12-03 | InfoDisc | dev infra ports | medium | mitigate | All ports 127.0.0.1; `.env` git-ignored | closed |
| T-12-04 | Tampering (SQLi) | init.sh names | medium | mitigate | `deploy/postgres/init.sh:34-37` | closed |
| T-12-05 | DoS | Redpanda dev mode | low | accept | Dev-only flags (AR-07) | closed |
| T-12-SC | Tampering | goose, images | low | mitigate | `deploy/migrate/Dockerfile:10` goose v3.27.3 pinned; image tags pinned | closed |
| T-13-01 | Tampering | feature-branch images | medium | mitigate | `Jenkinsfile:82` only conditional stage | closed |
| T-13-02 | Repudiation | skipped test gates | medium | mitigate | `deploy/ci/smoke.sh:133-134` | closed |
| T-13-03 | EoP | docker.sock on agent | high | mitigate | Inherits T-06-01; residual risk AR-09 | closed |
| T-13-04 | Tampering (injection) | changed-services.sh | low | mitigate | `deploy/ci/changed-services.sh` segment-only extraction | closed |
| T-01-14-01 | InfoDisc | tracesToLogsV2 | low | mitigate | `datasources.yaml:12` filterByTraceID: true | closed |
| T-01-14-02 | InfoDisc | Grafana verify credential | low | mitigate | `curl -K -` stdin pattern (`01-14-PLAN.md:76`, `Makefile:253-258`) | closed |
| T-01-14-03 | DoS | Loki query cost | low | accept | Dev-only single user (AR-08) | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-01 | T-03-SC | money/clock packages use stdlib only — no new modules | plan 01-03 | 2026-09-26 |
| AR-02 | T-06-SC | No npm/pip/cargo installs; agent apt packages from Debian mirrors | plan 01-06 | 2026-09-26 |
| AR-03 | T-07-04 | DLQ envelopes PII-free by construction (proto gate); retention = topic retention (3d dev) | plan 01-07 | 2026-09-26 |
| AR-04 | T-08-05 | Static dev Grafana password in .env.example, localhost-bound; prod creds in Phase 5 | plan 01-08 | 2026-09-26 |
| AR-05 | T-09-05 | Health endpoints return only ok/error; services publish no host ports | plan 01-09 | 2026-09-26 |
| AR-06 | T-11-05 | Public boats listing is public catalog data (name, capacity, status, operator uuid) | plan 01-11 | 2026-09-26 |
| AR-07 | T-12-05 | Redpanda dev-container flags (relaxed fsync) dev-only; prod durability in Phase 5 DEP-01 | plan 01-12 | 2026-09-26 |
| AR-08 | T-01-14-03 | ±1m extra Loki range per click, dev-only single user, bounded by trace-id filter | plan 01-14 | 2026-09-26 |
| AR-09 | T-06-01, T-13-03 | Residual host-root risk of docker.sock on jenkins-agent is inherent to D-21 (build images in CI). Controls: socket only on agent, controller 0 executors, repo-scoped job, Jenkins on 127.0.0.1. Not previously logged in 01-06-SUMMARY — recorded here | secure-phase audit | 2026-09-26 |

*Accepted risks do not resurface in future audit runs.*

**Transferred:** T-08-03 — PII filtering for telemetry is enforced at the source (pkg/httpx allowlistExporter, pkg/kafka log attrs); the observability stack stores what it receives.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-26 | 73 | 73 | 0 | gsd-security-auditor (64 mitigate verified) + orchestrator (8 accept, 1 transfer) |

### Notes
- T-12-SC: goose pinned at v3.27.3 (documented deviation from the plan's illustrative v3.28.0) — still pinned, closed.
- T-01-14-02: evidence is the documented verify command pattern (plan-only artifact) matching the in-code Harbor `-K -` pattern.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-26
