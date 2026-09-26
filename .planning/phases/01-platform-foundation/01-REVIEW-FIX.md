---
phase: 01-platform-foundation
fixed_at: 2026-09-26T09:52:46Z
review_path: .planning/phases/01-platform-foundation/01-REVIEW.md
iteration: 2
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 01: Code Review Fix Report

**Fixed at:** 2026-09-26T09:52:46Z
**Source review:** .planning/phases/01-platform-foundation/01-REVIEW.md
**Iteration:** 2

**Summary:**
- Findings in scope: 3 (WR-04, WR-05, IN-03)
- Fixed: 3
- Skipped: 0

**Verification environment:** all fixes applied and verified inside an isolated git
worktree (`.claude/worktrees/rf-01-*`, branch `gsd-reviewfix/01-*`), then
fast-forward-merged into `gsd/phase-01-platform-foundation`. `go build`/`go vet`/`go test`
ran in that worktree's checkout (same module graph and dependency versions as the main
checkout — no separate `node_modules`/vendor state involved for this Go-only fix set), so
the reported results are reproducible from the same commits in the main checkout after
worktree teardown.

## Fixed Issues

### WR-04: `Makefile`'s `ci-up` target leaks `HARBOR_ADMIN_PASSWORD` via `curl -u` argv

**Files modified:** `Makefile`
**Commit:** `0b3df66`
**Applied fix:** Replaced `curl -u "admin:$$HARBOR_ADMIN_PASSWORD"` with `printf 'user = "admin:%s"\n' "$$HARBOR_ADMIN_PASSWORD" | curl -K -`, i.e. feeding the credential to curl through its stdin config-file mechanism instead of putting it on the command line — the same "pipe the secret in, don't put it in argv" shape `IN-01`'s fix already applied to the sibling `push` target's `docker login --password-stdin`. Kept the existing `HARBOR_ADMIN_PASSWORD=$$(grep ... .env ...)` shell-side re-read as-is: it is not redundant — `deploy/ci/harbor-prepare.sh` (which runs earlier in the same recipe) can generate/rotate the password into `.env` *after* Make already parsed `-include .env` at startup, so the Make variable can be stale and the recipe must re-read the file at run time. Only the `curl -u` argv exposure was in scope for this finding. Verified: `make -n ci-up` dry-run shows the expected substituted recipe text with no syntax errors; `sh -n` on the extracted recipe body passes; `go build ./...` unaffected (Makefile-only change, no Go code touched).

### WR-05: `allowlistExporter.ExportSpans` mutates the span's own shared attribute slice in place

**Files modified:** `pkg/httpx/otel.go`
**Commit:** `bd990fc`
**Applied fix:** Changed `kept := stub.Attributes[:0]` (in-place filter reusing the SDK's aliased, shared backing array) to `kept := make([]attribute.KeyValue, 0, len(stub.Attributes))` (a fresh, appropriately-capacity-hinted slice), exactly as the review's suggested fix specified. `attribute` was already imported in the file, so no import changes were needed. Verified: `go build ./pkg/httpx/...` and `go vet ./pkg/httpx/...` clean, `go test ./pkg/httpx/...` passes (all existing tests, no regressions).

### IN-03: Outbox poison-row skip has no metric, only a log line

**Files modified:** `pkg/outbox/outbox.go`
**Commit:** `425f926`
**Applied fix:** Added `r.publishErrors.Add(ctx, 1)` in the unmarshal-failure ("poison row") branch of `publishOnce`, immediately after the existing `r.log.Error(...)` call — mirroring exactly how the Kafka-publish-failure branch a few lines below already increments the same counter. Both classes of "an event never made it to Kafka" (poison payload vs. publish error) now surface on the same `outbox.publish_errors` metric feeding dashboards/alerts. `ctx` was already in scope inside the transaction closure (same variable the sibling branch already uses), so this is a one-line addition with no signature changes. Verified: `go build ./pkg/outbox/...` and `go vet ./pkg/outbox/...` clean; ran the full `pkg/outbox` integration suite with `-tags=integration` against real Postgres + Redpanda via testcontainers (Docker was available) — `TestRelaySkipsPoisonRowWithoutLosingEarlierProgress` still passes, confirming the metric addition didn't disturb the poison-row-skip behavior it instruments.

## Skipped Issues

None — all three in-scope findings were fixed with code changes and verified.

---

_Fixed: 2026-09-26T09:52:46Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_
