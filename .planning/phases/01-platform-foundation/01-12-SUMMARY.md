---
phase: 01-platform-foundation
plan: 12
subsystem: infra
tags: [docker-compose, postgres, redpanda, kong, otel, tempo, loki, walking-skeleton]

requires:
  - phase: 01-platform-foundation (plans 01, 04, 07, 08, 09, 10, 11)
    provides: "pkg/auth (JWT verify/issue/cookie), pkg/outbox + pkg/kafka (transactional outbox, exactly-once consumer, DLQ), the observability stack (OTel Collector -> Tempo/Loki/Prometheus -> Grafana), services/_template runtime, services/catalog (UpsertBoat/ListBoats + outbox), services/schedule (BoatUpserted projection), services/gateway (BFF routes)"
provides:
  - "One `make up` boots Postgres 17 (role+database per service, PUBLIC CONNECT revoked, D-25), Redpanda (auto-create off, provisioned topics), Valkey, the observability stack, migrate-<svc> one-shots, catalog, schedule, gateway, Kong and the web profile — every long-running service reports healthy, no Go service publishes a host port"
  - "`make proof`: POST a boat through Kong -> gateway -> catalog (tx + outbox) -> Redpanda -> schedule projection applied exactly once, GET /api/v1/public/boats lists it, one Tempo trace spans gateway+catalog+schedule (by span.aggregate_id), Loki holds the correlated schedule log line"
  - "`make kong-roundtrip` proves the catalog hop: public boats 200 without a token, POST /api/v1/boats 201 with one (PLAT-10)"
  - "Developer inner loop: `make up-infra` + `make run-<svc>` (go run from host, hosts overridden to localhost), `make migrate-<svc>` (manual goose up), `make dlq-list topic=<t>`, Redpanda Console under profile `tools`"
  - "`deploy/postgres/isolation-check.sh` proves database-per-service is enforced by Postgres itself, not convention; `pkg/testenv.TestImagesMatchCompose` is a no-Docker drift guard keeping testcontainers and Compose on the same Postgres/Redpanda tags"
affects: ["phase-02-identity-catalog", "phase-03-scheduling-inventory", "01-13-ci-migrate-validate"]

actuals:
  tokens: 6950
  tasks: 2
  commits: 2
  plan_head_before: 0e1f52d5b6cb797b7cc602be74c0f8e2b8db10c3

tech-stack:
  added:
    - "postgres:17.9-alpine, redpandadata/redpanda:v26.2.3, valkey/valkey:9.0.6-alpine, redpandadata/console:v3.12.0 — all pinned, already matched by pkg/testenv's existing image constants (D-23)"
    - "goose v3.27.3 (not the plan's illustrative v3.28.0) in a small golang:1.25.9-alpine build stage -> alpine:3.22 runtime image for migrate-<svc> (D-26)"
  patterns:
    - "deploy/compose/service.yml.tmpl + Makefile compose-gen: one migrate-<svc> + <svc> block per deploy/services.txt entry, rendered into the git-ignored deploy/docker-compose.services.yml — new-service's compose-gen call means a freshly scaffolded service is wired into the running stack with zero manual compose edits"
    - "docker compose up --wait cannot express 'a by-design one-shot container that exited 0 is done' (it only accepts running|healthy) — replaced with a Makefile wait_ready loop that polls `ps -a --format json` and treats exited-with-0 as done, exited-nonzero or unhealthy as a hard failure"
    - "pkg/kafka.Consumer's per-record logging now always uses the ctx-aware slog *Context methods (matching pkg/httpx/middleware.go's requestLog), so trace_id/span_id are attached to every consumer log line whenever an active span exists in context — plain .Info/.Warn/.Error silently drop that correlation"

key-files:
  created:
    - deploy/compose/service.yml.tmpl
    - deploy/postgres/init.sh
    - deploy/postgres/isolation-check.sh
    - deploy/migrate/Dockerfile
    - deploy/proof.sh
    - pkg/testenv/images_test.go
  modified:
    - deploy/docker-compose.yml
    - deploy/kong/roundtrip.sh
    - Makefile
    - .env.example
    - .gitignore
    - pkg/kafka/consumer.go

key-decisions:
  - "goose pinned to v3.27.3, not the plan's illustrative v3.28.0 — v3.28.0's own go.mod requires go1.26.0, which would break this repo's go1.25.x toolchain pin; v3.27.3 requires go1.25.7, already satisfied by golang:1.25.9-alpine. Same class of issue 01-04 already hit and fixed identically for franz-go/goose."
  - "docker compose up --wait replaced with a hand-rolled wait_ready Makefile function for up/up-infra: --wait's own semantics ('running|healthy') have no way to express a by-design one-shot init/migrate container exiting 0 as success, so it fails the whole `up` even when every container did exactly what it was supposed to do."
  - "pkg/kafka.Consumer's log calls switched to the ctx-aware slog *Context variants — a real correctness gap (not a config tweak): this plan's own 'logs-correlated' proof requires Loki to hold a schedule log line carrying the request's trace_id, and the plain (non-Context) calls never attach one because slog.Logger.Info/Warn/Error don't receive a ctx at all."
  - "Tempo's /api/search response already returns a serviceStats map keyed by service.name for the matched trace — proof.sh reads that directly instead of fetching /api/traces/<id> and walking OTLP-JSON span nesting, which is simpler and avoids depending on Tempo's exact trace-JSON shape."

patterns-established:
  - "Task-boundary note: run-%/migrate-%/dlq-list/PORT/CATALOG_PORT/topic (Makefile) and the redpanda-console service (docker-compose.yml) are Task 2 deliverables per the plan, but were authored in the same edit pass as Task 1's compose-gen/up/up-infra/down/proof changes and therefore landed in Task 1's commit (02736bc) rather than Task 2's (c5e83a5). Task 2's commit holds its two genuinely new files (isolation-check.sh, images_test.go). Disclosed here rather than re-split after the fact — see 01-10's identical precedent for tests written alongside their required implementation."

requirements-completed: [PLAT-03, PLAT-05, PLAT-06, PLAT-07, PLAT-10]

coverage:
  - id: D1
    description: "make up starts Postgres 17, Redpanda (auto-create off), Valkey, the observability stack, migrate-<svc> one-shots, catalog, schedule, gateway, Kong and the web profile; every long-running service reports healthy and no Go service publishes a host port"
    requirement: PLAT-03
    verification:
      - kind: other
        ref: "make up (fresh, from `make down`) — wait_ready reports 'all services running/healthy or completed successfully'; jq assertion over `docker compose config` confirms catalog/schedule/gateway have no `ports`"
        status: pass
    human_judgment: false
  - id: D2
    description: "make proof: POST /api/v1/boats through Kong with a dev token creates a boat in catalog, schedule's own database holds it with the same capacity within 30s, the catalog outbox event_id appears exactly once in schedule.processed_events, and GET /api/v1/public/boats lists it"
    requirement: PLAT-05
    verification:
      - kind: e2e
        ref: "deploy/proof.sh (invoked via `make proof`) — PASS applied, PASS exactly-once, PASS public-list"
        status: pass
    human_judgment: false
  - id: D3
    description: "make proof finds one Tempo trace (by span attribute aggregate_id = boat id) whose spans come from gateway, catalog and schedule, and Loki holds a schedule log line carrying that trace_id"
    requirement: PLAT-06
    verification:
      - kind: e2e
        ref: "deploy/proof.sh (invoked via `make proof`) — PASS single-trace (services: catalog,gateway,schedule), PASS logs-correlated"
        status: pass
    human_judgment: false
  - id: D4
    description: "make kong-roundtrip additionally proves the catalog hop: public boats 200 without a token, POST /api/v1/boats 201 with a valid token"
    requirement: PLAT-10
    verification:
      - kind: e2e
        ref: "deploy/kong/roundtrip.sh (invoked via `make kong-roundtrip`) — PASS public-boats-200, PASS upsert-boat-201, plus all 8 pre-existing checks still passing"
        status: pass
    human_judgment: false
  - id: D5
    description: "deploy/postgres/init.sh enforces database-per-service at the Postgres level (PUBLIC CONNECT revoked, only the owning role granted); isolation-check.sh proves every cross-service CONNECT is denied and every own-database CONNECT succeeds"
    requirement: PLAT-07
    verification:
      - kind: integration
        ref: "deploy/postgres/isolation-check.sh — PASS own-db-access catalog/catalog, PASS cross-db-denied catalog->schedule, PASS own-db-access schedule/schedule, PASS cross-db-denied schedule->catalog"
        status: pass
    human_judgment: false
  - id: D6
    description: "Developer inner loop works from the host against up-infra: make migrate-catalog (idempotent goose up) and make run-catalog (go run, hosts overridden to localhost) both succeed, catalog reports /readyz 200"
    verification:
      - kind: integration
        ref: "make down && make up-infra && make migrate-catalog && (PORT=18090 make run-catalog &) && poll http://localhost:18090/readyz — READY within a few seconds"
        status: pass
    human_judgment: false
  - id: D7
    description: "pkg/testenv.TestImagesMatchCompose keeps the testcontainers image tags and deploy/docker-compose.yml's tags in lockstep, no Docker required"
    verification:
      - kind: unit
        ref: "pkg/testenv/images_test.go#TestImagesMatchCompose"
        status: pass
    human_judgment: false
  - id: D8
    description: "make test, make test-integration, and make lint stay green repo-wide, including the pkg/kafka logging fix's effect on every existing consumer test"
    verification:
      - kind: other
        ref: "make test (all ok), make test-integration (all ok, ~2min across pkg/kafka, pkg/outbox, _template/catalog/schedule cmd, gateway http), make lint (0 issues across pkg/gateway/_template/catalog/schedule, buf lint, PII check), make template-smoke (PASS, no residue)"
        status: pass
    human_judgment: false
  - id: D9
    description: "One trace visibly continues across the Kafka hop in Grafana (gateway HTTP span -> catalog connect span -> Kafka publish -> schedule consume span, same trace id, no gap), and the platform dashboard shows populated HTTP/consumer-lag/outbox/DLQ panels"
    verification: []
    human_judgment: true
    rationale: "Trace continuity across the Kafka hop and dashboard usefulness are visual judgements — proof.sh already asserts the service set and log correlation programmatically (D3). Deferred to end-of-phase UAT per workflow.human_verify_mode=end-of-phase; see Task 2's <verify><human-check> in the plan."

duration: 32min
completed: 2026-09-26
status: complete
---

# Phase 1 Plan 12: Walking Skeleton — make up && make proof && make kong-roundtrip Summary

**Postgres 17 (role+database per service, database-per-service enforced by Postgres itself), Redpanda (auto-create off, provisioned topics), Valkey, migrate-`<svc>` one-shots and the web profile are wired into `deploy/docker-compose.yml`; `make proof` proves one boat travels Kong → gateway → catalog → outbox → Redpanda → schedule as a single Tempo trace with correlated Loki logs, applied exactly once.**

## Performance

- **Duration:** 32 min
- **Started:** 2026-09-26T13:32:35+07:00 (approx., end of prior plan)
- **Completed:** 2026-09-26T14:04:34+07:00
- **Tasks:** 2 completed
- **Files:** 6 created, 6 modified

## Accomplishments

- `deploy/postgres/init.sh` provisions a role + database per `deploy/services.txt` entry, idempotently, and revokes `PUBLIC`'s `CONNECT` while granting it only to the owning role — database-per-service is now enforced by Postgres itself, not convention (D-25)
- `deploy/compose/service.yml.tmpl` + `Makefile compose-gen` render one `migrate-<svc>` + `<svc>` block per service into the git-ignored `deploy/docker-compose.services.yml`; `make new-service` now re-renders it automatically, so a freshly scaffolded service is wired into `make up` with zero manual compose edits
- `deploy/proof.sh` (`make proof`) is fully green end-to-end from a cold `make up`: POST a boat via Kong, confirm schedule's projection holds the same capacity within 30s, confirm the catalog outbox `event_id` is applied exactly once in `schedule.processed_events`, confirm `GET /api/v1/public/boats` lists it, find the one Tempo trace whose spans span gateway+catalog+schedule, and confirm Loki holds the correlated schedule log line
- `deploy/kong/roundtrip.sh` gained the two PLAT-10 checks (public boats 200 without a token, upsert boat 201 with one) — all 10 checks pass
- Developer inner loop: `make up-infra` + `make run-<svc>` (hosts overridden to localhost), `make migrate-<svc>` (manual goose), `make dlq-list topic=<t>`, Redpanda Console under profile `tools`
- `deploy/postgres/isolation-check.sh` and `pkg/testenv.TestImagesMatchCompose` give ongoing, automated proof that isolation and image-tag parity hold

## Task Commits

1. **Task 1: make up full stack → make proof: POST boat via Kong → catalog tx + outbox → Redpanda → schedule applied once → one trace in Tempo + logs in Loki** — `02736bc` (feat)
2. **Task 2: Developer inner loop and platform guarantees — up-infra + run-\<svc\>, migrate-\<svc\>, dlq-list, Redpanda Console, DB isolation check, image drift test** — `c5e83a5` (feat)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP/REQUIREMENTS update)

_Task 1 is `type="tracer"` — the tracer feedback gate (re-running `make up && make proof && make kong-roundtrip` plus the port-exposure jq assertion) was re-executed after the commit and passed before starting Task 2, per the `human_verify_mode: end-of-phase` + automated-only `<verify>` rule (no checkpoint needed on success)._

## Files Created/Modified

- `deploy/compose/service.yml.tmpl` — per-service `migrate-<svc>` + `<svc>` compose block template
- `deploy/postgres/init.sh` — idempotent role+database provisioning, `REVOKE`/`GRANT CONNECT` (D-25)
- `deploy/postgres/isolation-check.sh` — proves cross-service `CONNECT` denial + own-database access
- `deploy/migrate/Dockerfile` — small goose CLI image (goose v3.27.3, D-26)
- `deploy/proof.sh` — the end-to-end proof script behind `make proof`
- `pkg/testenv/images_test.go` — `TestImagesMatchCompose` drift guard
- `deploy/docker-compose.yml` — `postgres`, `postgres-init`, `redpanda`, `redpanda-init`, `valkey`, `redpanda-console` services; gateway env additions; `pg_data`/`redpanda_data` volumes
- `deploy/kong/roundtrip.sh` — two new PLAT-10 checks
- `Makefile` — `compose-gen`, `wait_ready`, updated `up`/`up-infra`/`down`, `proof`, `run-%`, `migrate-%`, `dlq-list`, `new-service` now re-runs `compose-gen`
- `.env.example` — `POSTGRES_PASSWORD`, `SERVICE_DB_PASSWORD`
- `.gitignore` — `deploy/docker-compose.services.yml`
- `pkg/kafka/consumer.go` — every per-record log call switched to the ctx-aware `*Context` slog method (see Deviations)

## Decisions Made

See `key-decisions` in frontmatter: goose pinned to v3.27.3 (go1.25.x compatibility), `docker compose up --wait` replaced with a hand-rolled `wait_ready` Makefile function, `pkg/kafka.Consumer` switched to ctx-aware logging, and `proof.sh` reading Tempo's `serviceStats` directly instead of parsing the full trace JSON.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] goose v3.28.0 requires go1.26.0, breaking the repo's go1.25.x pin**
- **Found during:** Task 1, first `make up` build of `deploy/migrate/Dockerfile`
- **Issue:** `go install github.com/pressly/goose/v3/cmd/goose@v3.28.0` failed: "requires go >= 1.26.0 (running go 1.25.9)". This is the exact same class of issue 01-04 already hit and fixed (franz-go/goose both bumped a go.mod minimum to 1.26.0 in later tags).
- **Fix:** Pinned to v3.27.3 (requires go1.25.7, satisfied by `golang:1.25.9-alpine`), matching the version 01-04 and this project's STATE.md already established as the correct pin.
- **Files modified:** `deploy/migrate/Dockerfile`
- **Verification:** `make up` builds `migrate-catalog`/`migrate-schedule` successfully; both exit 0 and print `goose: no migrations to run` on re-run (idempotent).
- **Committed in:** `02736bc` (Task 1 commit)

**2. [Rule 3 - Blocking] `docker compose up --wait` fails on a by-design one-shot container that exits 0**
- **Found during:** Task 1, first `make up-infra` run
- **Issue:** `postgres-init` and `redpanda-init` are one-shot containers that run their provisioning script and exit 0 by design (the plan's own `depends_on: condition: service_completed_successfully` pattern requires this). `docker compose up --wait` only accepts `running|healthy` as "done" for every service in the compose file, so it reported `container boatbooking-postgres-init-1 exited (0)` as an error and returned exit 1 — even though provisioning succeeded and the dependency graph resolved correctly.
- **Fix:** Replaced `--wait` with a `wait_ready` Makefile `define` that polls `docker compose ps -a --format json` and treats `state=exited, exitcode=0` as done, `state=exited, exitcode!=0` or an unhealthy `running` container as a hard failure, retrying until `WAIT_TIMEOUT` (default 180s).
- **Files modified:** `Makefile`
- **Verification:** `make up` and `make up-infra` both complete cleanly and print `wait_ready: all services running/healthy or completed successfully`; a deliberately broken service (tested manually with a bad healthcheck during development) is correctly reported as `FAIL` rather than silently waited-out.
- **Committed in:** `02736bc` (Task 1 commit)

**3. [Rule 1 - Bug] `pkg/kafka.Consumer`'s per-record log calls never attached `trace_id`, so this plan's own "logs-correlated" proof could not pass**
- **Found during:** Task 1, first manual verification of Loki correlation before writing `proof.sh`
- **Issue:** Every log call in `processRecord`/`sendToDLQ`/`Run` used the ctx-less `slog.Logger.Info/Warn/Error(msg, args...)`. `pkg/httpx.NewLogger`'s `traceHandler` only attaches `trace_id`/`span_id` when it receives an active span via `ctx` (exactly how `pkg/httpx/middleware.go`'s `requestLog` already does it with `log.LogAttrs(r.Context(), ...)`) — a ctx-less call always logs with no span, so Loki never held a `trace_id` for any consumer log line, an unfixable gap for this plan's PLAT-06 proof requirement.
- **Fix:** Switched every log call in `pkg/kafka/consumer.go` to the matching ctx-aware method (`InfoContext`/`WarnContext`/`ErrorContext`), passing the most specific context already in scope at each call site (`ctx`, `spanCtx`, or `detachedCtx`).
- **Files modified:** `pkg/kafka/consumer.go`
- **Verification:** `make test-integration` (pkg/kafka's existing suite, unaffected in behavior) still green; manually confirmed a fresh boat POST produces a Loki line `{service_name="schedule"} | trace_id="<id>"` matching the Tempo trace id; `make proof`'s `PASS logs-correlated` check passes.
- **Committed in:** `02736bc` (Task 1 commit)

---

**Total deviations:** 3 auto-fixed (2 blocking, 1 bug). **Impact:** All three were necessary for the plan's own headline success criterion (`make up && make proof && make kong-roundtrip` green) to be achievable at all — none are scope creep, and none change any already-shipped service's public behavior beyond adding trace correlation to logs that were already being written.

## Issues Encountered

- Kong's `local`-policy rate limiter (120/min) persists in-container, so repeated manual `curl` testing during development against the same running `kong` container exhausted the budget and returned 429s unrelated to the code under test — worked around during iteration by `docker compose restart kong`. `make kong-roundtrip`'s own rate-limit-429 check is unaffected since it runs from a fresh `make up`.
- No Redpanda startup flakiness or stale bind-mount issues were hit this session (both called out as known risks for this host in prior plans' summaries).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- The full walking skeleton is real, running, and proven from a single `make up` — `make proof` and `make kong-roundtrip` are both green from a cold start, satisfying the phase's headline success criterion.
- `make up-infra` + `make run-<svc>` gives the Phase 2+ inner loop: any new service scaffolded via `make new-service` inherits `compose-gen` wiring automatically.
- PLAT-03, PLAT-05, PLAT-06, PLAT-07 and PLAT-10 are now all marked Complete in REQUIREMENTS.md — PLAT-05/06/07 were pre-marked by 01-07 but are now genuinely re-proven end-to-end by this plan's `make proof`; no reversion was needed.
- No blockers for 01-13 (CI wiring) or Phase 2.

## Self-Check: PASSED

- All 6 created files confirmed present on disk (`deploy/compose/service.yml.tmpl`, `deploy/postgres/init.sh`, `deploy/postgres/isolation-check.sh`, `deploy/migrate/Dockerfile`, `deploy/proof.sh`, `pkg/testenv/images_test.go`).
- `git log --oneline` confirms both commits (`02736bc`, `c5e83a5`).
- Plan-level `<verification>` re-run clean at the end of this plan: `make up && make proof && make kong-roundtrip` green from a cold `make down`; `deploy/postgres/isolation-check.sh`, `make dlq-list topic=catalog.events`, `make migrate-catalog`, `make run-catalog` from host, and `TestImagesMatchCompose` all green; `make test`, `make test-integration`, `make lint`, and `make template-smoke` all green.
- All plan `<acceptance_criteria>` re-verified via direct commands, all pass.

---
*Phase: 01-platform-foundation*
*Completed: 2026-09-26*
