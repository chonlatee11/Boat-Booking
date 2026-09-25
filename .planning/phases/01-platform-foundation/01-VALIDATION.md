---
phase: "1"
slug: "platform-foundation"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-26"
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) + testcontainers-go v0.44 (`//go:build integration`); web: eslint + tsc + next build |
| **Config file** | none — Wave 0 installs (go.work, per-module go.mod, apps/web/package.json) |
| **Quick run command** | `make test` (`go test $(go list -m -f '{{.Path}}/...')`, no Docker — `./...` fails at a go.work root and skips `_template`) |
| **Full suite command** | `make test-integration && make lint && npm --prefix apps/web run build` |
| **Estimated runtime** | ~20s quick / ~180s full (container startup dominates) |

---

## Sampling Rate

- **After every task commit:** Run `make test`
- **After every plan wave:** Run `make test-integration` (+ web lint/typecheck/build when apps/web touched)
- **Before `/gsd-verify-work`:** Full suite must be green + Kong round-trip script + manual Tempo trace check
- **Max feedback latency:** 180 seconds

---

## Per-Task Verification Map

Filled by planner/executor per task. Requirement → test anchors from RESEARCH.md:

| Requirement | Test Type | Automated Command | File Exists | Status |
|-------------|-----------|-------------------|-------------|--------|
| PLAT-01 | build + smoke | `make template-smoke` (01-10) and `go test -tags=integration 'github.com/chonlatee11/boat-booking/services/__NAME__/...'` (01-09) | ❌ W0 | ⬜ pending |
| PLAT-02 | unit + integration | `make test && make test-integration` (pkg/* tests from 01-01, 01-03, 01-04, 01-07, 01-09) | ❌ W0 | ⬜ pending |
| PLAT-03 | smoke | `make up && make proof` (01-12); `deploy/postgres/isolation-check.sh` | ❌ W0 | ⬜ pending |
| PLAT-04 | CI gate | `make proto-gen && make proto-check` (01-02) | ❌ W0 | ⬜ pending |
| PLAT-05 | integration + e2e | `go test -tags=integration -run TestBoatUpsertedAppliedOnce github.com/chonlatee11/boat-booking/services/schedule/...` (01-11); `make proof` PASS exactly-once (01-12) | ❌ W0 | ⬜ pending |
| PLAT-06 | integration + e2e + manual | `go test -tags=integration -run TestTraceparentSurvivesRelay github.com/chonlatee11/boat-booking/pkg/outbox/...` (01-04); `make proof` PASS single-trace / logs-correlated (01-12) | ❌ W0 | ⬜ pending |
| PLAT-07 | integration | `go test -tags=integration -run 'TestFailedHandlerLandsInDLQAfter3Retries|TestUncommittedRecordIsRedelivered' github.com/chonlatee11/boat-booking/pkg/kafka/...` (01-07) | ❌ W0 | ⬜ pending |
| PLAT-08 | CI self-test + manual | `make ci` (01-13); `make ci-up && deploy/ci/smoke.sh` (01-06, 01-13) | ❌ W0 | ⬜ pending |
| PLAT-09 | build gate + manual | `npm --prefix apps/web run lint && npm --prefix apps/web run typecheck && npm --prefix apps/web run build` (01-05) | ❌ W0 | ⬜ pending |
| PLAT-10 | script | `make up && make kong-roundtrip` (01-01, extended in 01-12) | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `go.work` + `pkg/*` modules — shared packages
- [ ] `services/_template/**` — template service
- [ ] `proto/**` + `buf.gen.yaml` — proto-gen inputs
- [ ] `deploy/docker-compose.yml` + init scripts — `make up`
- [ ] `Jenkinsfile` + `deploy/ci/docker-compose.yml` — CI
- [ ] `apps/web` — Next.js skeleton
- [ ] `deploy/kong/kong.yml` + dev-token target — Kong spike
- [ ] Integration tests `TestBoatUpsertedAppliedOnce`, `TestFailedHandlerLandsInDLQAfter3Retries`, `TestTraceparentSurvivesRelay`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Single trace spans HTTP + Kafka in Tempo | PLAT-06 | Tempo UI inspection | `make up`, POST a boat via Kong, open Grafana → Tempo, find trace, confirm consumer span shares trace_id |
| Jenkins rebuild scoping | PLAT-08 | Needs live Jenkins | Push change under `pkg/` → all services rebuild; push under one `services/<name>/` → only that one |
| Thai font + i18n + mobile layout | PLAT-09 | Visual | Open `/th` and `/en` at 375px width; Thai glyphs render in IBM Plex Sans Thai; network tab shows only Kong origin |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 180s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
