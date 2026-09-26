---
phase: 01-platform-foundation
plan: 08
subsystem: infra
tags: [opentelemetry, otel-collector, tempo, loki, prometheus, grafana, observability, docker-compose]

requires:
  - phase: 01-platform-foundation (plans 04, 07)
    provides: "pkg/outbox and pkg/kafka metric instrument names (outbox_backlog, outbox_oldest_unpublished_age_seconds, outbox_publish_errors_total, kafka_consumer_lag, kafka_consumer_processed_total, dlq_total) this plan's dashboard queries against"
provides:
  - "OTel Collector (OTLP gRPC 4317 / HTTP 4318, internal-only) fanning traces to Tempo, logs to Loki, metrics to Prometheus"
  - "Grafana provisioned from files: Tempo/Loki/Prometheus datasources with trace<->log derived-field links, published only on 127.0.0.1:3000"
  - "platform.json dashboard (uid platform) with HTTP rate/p95, consumer lag, processed events, outbox backlog/age/errors, DLQ panels"
  - "make up-infra / make obs-check and deploy/observability/check.sh synthetic OTLP round-trip (traces|logs|metrics|all)"
affects: [01-09-otel-sdk-wiring, 01-12-e2e-walking-skeleton]

actuals:
  tokens: 4031
  tasks: 2
  commits: 2
  plan_head_before: 945d22bf7b227c08526309b284ae1cab4eee5f6b

tech-stack:
  added:
    - "otel/opentelemetry-collector-contrib:0.161.0 — OTLP receiver, memory_limiter+batch, otlp/otlphttp exporters"
    - "grafana/tempo:2.9.5 — single-binary trace storage, 72h block retention"
    - "grafana/loki:3.7.8 — single-process log storage, tsdb v13, structured metadata"
    - "prom/prometheus:v3.15.0 — native OTLP receiver (--web.enable-otlp-receiver), 3d retention"
    - "grafana/grafana:12.4.9 — file-provisioned datasources + dashboard"
  patterns:
    - "Infra services (otel-collector, tempo, loki, prometheus, grafana) carry no compose profile so they're always started, matching D-19's make up-infra = infra only / make up = infra + app + web"
    - "check.sh sends OTLP/HTTP JSON from a throwaway curlimages/curl container on the compose network (Collector publishes no host port), then polls Grafana's own published :3000 datasource proxy from the host"

key-files:
  created:
    - deploy/observability/otel-collector.yaml
    - deploy/observability/tempo.yaml
    - deploy/observability/loki.yaml
    - deploy/observability/prometheus.yml
    - deploy/observability/check.sh
    - deploy/observability/grafana/provisioning/datasources/datasources.yaml
    - deploy/observability/grafana/provisioning/dashboards/dashboards.yaml
    - deploy/observability/grafana/dashboards/platform.json
  modified:
    - deploy/docker-compose.yml
    - Makefile
    - .env.example

key-decisions:
  - "Loki compactor requires compactor.delete_request_store set whenever limits_config.retention_period is non-zero (Loki 3.7's own config validator), even though the plan's action list didn't call this out explicitly — added delete_request_store: filesystem alongside the working_directory"
  - "Grafana's Loki derived field literally writes '$${__value.raw}' (double-dollar) in the YAML per the plan's own spec: Grafana provisioning expands single-$ as its own env-var syntax, so the derived-field template macro must be escaped to survive that pass before Loki's UI evaluates it"

requirements-completed: [PLAT-03, PLAT-06]

coverage:
  - id: D1
    description: "Synthetic OTLP span sent to the Collector is retrievable from Tempo through Grafana's Tempo datasource proxy by trace id (make up-infra, make obs-check traces)"
    requirement: PLAT-06
    verification:
      - kind: other
        ref: "make up-infra && deploy/observability/check.sh traces -> PASS traces"
        status: pass
    human_judgment: false
  - id: D2
    description: "Logs to Loki and metrics to Prometheus over OTLP, trace<->log derived-field links, platform.json dashboard provisioned with per-service HTTP/consumer-lag/outbox/DLQ panels"
    requirement: PLAT-06
    verification:
      - kind: other
        ref: "make obs-check -> PASS traces, PASS logs, PASS metrics"
        status: pass
      - kind: other
        ref: "jq -e dashboard panel-query assertion (kafka_consumer_lag, outbox_backlog, dlq_total, http_server_request_duration_seconds all present) -> true"
        status: pass
    human_judgment: false

duration: 32min
completed: 2026-09-26
status: complete
---

# Phase 01 Plan 08: Observability Stack (OTel Collector -> Tempo/Loki/Prometheus -> Grafana) Summary

**One OTel Collector fans synthetic traces/logs/metrics out to Tempo, Loki and Prometheus; Grafana is provisioned entirely from files with trace<->log links and a `platform.json` dashboard querying the outbox/Kafka metric names plans 04 and 07 already emit.**

## Performance

- **Duration:** 32 min
- **Started:** 2026-09-26T04:57:00Z
- **Completed:** 2026-09-26T05:25:09Z
- **Tasks:** 2 completed
- **Files modified:** 11 (8 created, 3 modified)

## Accomplishments
- `make up-infra` starts one OTel Collector (OTLP gRPC 4317 / HTTP 4318, no host ports) plus Tempo, Loki, Prometheus and Grafana (only Grafana published, on `127.0.0.1:3000`) — all with no compose profile so they're always up, per D-19/D-38
- A synthetic OTLP span sent to the Collector round-trips through Tempo and is queryable via Grafana's Tempo datasource proxy by trace id
- OTLP logs land in Loki (`service_name` label, `trace_id` structured metadata) and OTLP metrics land in Prometheus via its native `--web.enable-otlp-receiver` — both verified end-to-end by `make obs-check`
- Grafana is provisioned from files only: Tempo/Loki/Prometheus datasources with `tracesToLogsV2`/`derivedFields` trace<->log links on `trace_id`, and `platform.json` (uid `platform`) with HTTP rate/p95, consumer lag, processed events, outbox backlog/oldest-age/publish-errors, and DLQ panels

## Task Commits

1. **Task 1: Synthetic OTLP span -> Collector -> Tempo -> visible through Grafana's Tempo datasource** - `bfe6306` (feat)
2. **Task 2: Logs -> Loki, metrics -> Prometheus (OTLP), trace<->log links, platform.json dashboard, obs-check all** - `48fb795` (feat)

**Plan metadata:** (this commit, made after this SUMMARY)

## Files Created/Modified
- `deploy/observability/otel-collector.yaml` - OTLP receivers, memory_limiter+batch, otlp/tempo + otlphttp/loki + otlphttp/prometheus exporters, traces/logs/metrics pipelines
- `deploy/observability/tempo.yaml` - single-binary Tempo, local storage, 72h block retention
- `deploy/observability/loki.yaml` - single-process filesystem Loki, tsdb v13 schema, structured metadata, 72h retention
- `deploy/observability/prometheus.yml` - native OTLP receiver config, resource-attribute promotion
- `deploy/observability/check.sh` - synthetic OTLP round-trip check (`traces|logs|metrics|all`) via a throwaway curl container + Grafana datasource proxy polling
- `deploy/observability/grafana/provisioning/datasources/datasources.yaml` - Tempo/Loki/Prometheus datasources, trace<->log links
- `deploy/observability/grafana/provisioning/dashboards/dashboards.yaml` - file dashboard provider
- `deploy/observability/grafana/dashboards/platform.json` - platform dashboard (uid `platform`)
- `deploy/docker-compose.yml` - `otel-collector`, `tempo`, `loki`, `prometheus`, `grafana` services (no profile), `tempo_data`/`loki_data`/`prometheus_data` volumes
- `Makefile` - `up-infra`, `obs-check` targets
- `.env.example` - `GRAFANA_ADMIN_PASSWORD`

## Decisions Made
- Added `compactor.delete_request_store: filesystem` to `loki.yaml` — Loki 3.7's own startup validation rejects a non-zero `limits_config.retention_period` without it, which the plan's action text didn't call out
- Kept the plan's literal `'$${__value.raw}'` (double-dollar) in the Loki derived field — Grafana's provisioning-file env-var expansion would otherwise consume the single `$` before Loki's own derived-field macro ever sees it

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Loki compactor needs `delete_request_store` whenever retention is set**
- **Found during:** Task 2 (`make up-infra` failing to bring Loki healthy)
- **Issue:** `limits_config.retention_period: 72h` alone makes Loki's compactor consider retention "enabled", and Loki 3.7 refuses to start without `compactor.delete_request_store` configured for that case
- **Fix:** Added `delete_request_store: filesystem` under `compactor:` in `deploy/observability/loki.yaml`
- **Files modified:** `deploy/observability/loki.yaml`
- **Verification:** `make up-infra` brings Loki up healthy; `make obs-check` -> `PASS logs`
- **Commit:** `48fb795` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary for Loki to start at all with the plan's own specified 72h retention. No scope creep — the fix is a single added config key in a file the plan already owns.

## Issues Encountered
- This sandbox's Docker Desktop file-sharing layer served stale bind-mount content for `otel-collector.yaml`/`datasources.yaml`/`loki.yaml` across several of my own repeated `docker run -v <same path>` debugging invocations during this session (confirmed by testing the identical content at a fresh path, which read correctly) — resolved each time by forcing a new inode (delete + rewrite) before the next `docker compose ... up -d --force-recreate`. This is a local Docker Desktop artifact from rapid iterative debugging, not a defect in the committed config; a normal single edit-then-`make up-infra` dev workflow won't hit it.
- During the same debugging, one manual `docker compose -f deploy/docker-compose.yml up -d --force-recreate grafana` (run without `--env-file .env`, unlike the Makefile's own `$(COMPOSE)` variable) briefly left Grafana with a blank admin password — caught by checking the container's actual env and fixed by re-running `make up-infra`, which always threads `--env-file .env` correctly. No code or Makefile change needed.

## User Setup Required
None - `make dev-keys` (already run automatically by `up-infra`) syncs `GRAFANA_ADMIN_PASSWORD` from `.env.example` into `.env` without overwriting existing values.

## Next Phase Readiness
- `make up-infra && make obs-check` is green end-to-end (`PASS traces`, `PASS logs`, `PASS metrics`); the `platform.json` dashboard JSON assertion passes
- Backend is proven with synthetic telemetry only — plan 09 (OTel SDK wiring into `pkg/httpx`/`pkg/kafka`) and plan 12 (real end-to-end trace) are what prove a real service emits into this same pipeline
- No blockers for proceeding to the next plan in this phase

---
*Phase: 01-platform-foundation*
*Completed: 2026-09-26*

## Self-Check: PASSED

All 8 created files confirmed present on disk; both task commits (`bfe6306`, `48fb795`) confirmed in `git log`; `make up-infra && make obs-check` re-run green immediately before writing this summary.
