---
phase: 01-platform-foundation
fixed_at: 2026-09-26T09:10:28Z
review_path: .planning/phases/01-platform-foundation/01-REVIEW.md
iteration: 1
findings_in_scope: 6
fixed: 6
skipped: 0
status: all_fixed
---

# Phase 01: Code Review Fix Report

**Fixed at:** 2026-09-26T09:10:28Z
**Source review:** .planning/phases/01-platform-foundation/01-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 6 (CR-01, WR-01, WR-02, WR-03, IN-01, IN-02)
- Fixed: 6
- Skipped: 0

## Fixed Issues

### CR-01: Outbox relay discards already-published rows' progress on an unmarshal failure

**Files modified:** `pkg/outbox/outbox.go`, `pkg/outbox/outbox_integration_test.go`
**Commit:** `6707df5`
**Applied fix:** Changed the `events.Unmarshal` failure branch in `publishOnce` from `return fmt.Errorf(...)` (which rolled back the whole transaction, including `published_at` progress for earlier rows in the batch already sent to Kafka) to logging the poison row and marking it published, then `continue`-ing the loop — matching `pkg/kafka`'s consumer-side handling of the symmetric case. Added `TestRelaySkipsPoisonRowWithoutLosingEarlierProgress` (integration test, requires Docker) as a regression test: inserts a good row followed by a row with an unparseable payload, runs the relay, and asserts the good row's `published_at` is set (not rolled back) and the poison row is marked done. Verified: `go build ./...` clean, full `pkg/outbox` integration suite passes (including the new test) against real Postgres + Redpanda via testcontainers.

### WR-01: `Role` claim is carried end-to-end but never enforced

**Files modified:** `services/gateway/internal/adapters/http/bff.go`, `services/catalog/internal/adapters/http/routes.go`
**Commit:** `c795c14`
**Applied fix:** Did not add a role-gating check. `01-RESEARCH.md`'s "V4 Access Control" note states explicitly that Phase 1 has "no business-level roles yet" and identifies claim-header trust-boundary enforcement (network isolation + `X-Internal-Token`) as the actual Phase 1 access-control surface; `SKELETON.md` defers customer/staff roles to Phase 2. There is no documented policy in this phase's PLAN/CONTEXT for which roles may write catalog boats, so adding an enforcement check now would invent undocumented product policy — exactly the case this task's constraints call out for a skip-with-explanation. Per the review's own fallback suggestion, added `TODO(WR-01)` comments at both write-path call sites (`upsertBoatHandler` in gateway, `UpsertBoat` in catalog), each pointing at the other and at this finding, so a second role introduced in Phase 2 can't silently bypass this. Verified: both services build clean; gateway's `internal/adapters/http` unit tests pass.

### WR-02: `httpx.WriteError`'s status mapping is incomplete

**Files modified:** `pkg/httpx/errors.go`, `pkg/httpx/errors_test.go` (new)
**Commit:** `fced546`
**Applied fix:** Changed the message-hiding condition from comparing against `code == connect.CodeInternal || code == connect.CodeUnknown` to comparing against the resulting `status != http.StatusInternalServerError`, so it can never drift from the status-hiding switch again — any code that falls through to the generic 500, mapped or not, now hides its message. Added `errors_test.go` with `TestWriteErrorHidesMessageForUnmappedCode` (regression test using `CodeAborted`, an unmapped code) and `TestWriteErrorShowsMessageForMappedCode` (control case using `CodeNotFound`). Verified: full `pkg/httpx` test suite passes, `go vet ./...` clean.

### WR-03: `deploy/postgres/init.sh` interpolates `$SERVICE_DB_PASSWORD` unescaped

**Files modified:** `deploy/postgres/init.sh`
**Commit:** `1a3e99e`
**Applied fix:** Added `escaped_password=$(printf '%s' "$SERVICE_DB_PASSWORD" | sed "s/'/''/g")` before the `CREATE ROLE` heredoc and used `$escaped_password` in the SQL literal, doubling embedded single quotes per standard SQL literal escaping (Postgres's default `standard_conforming_strings=on` means backslash needs no separate handling). Verified: `sh -n deploy/postgres/init.sh` syntax-checks clean; manually confirmed the sed escaping doubles embedded `'` characters correctly on a password containing quotes and other special characters.

### IN-01: `Makefile`'s `push` target passes the Harbor password through `echo`

**Files modified:** `Makefile`
**Commit:** `eb1e7e8`
**Applied fix:** The review's suggested `printf` swap alone doesn't fix the described `ps`/`/proc` visibility, because Make substitutes `$(HARBOR_PASSWORD)` directly into the recipe text before the shell ever runs, so the secret ends up in the `sh -c "..."` argv regardless of `echo` vs `printf`. Applied the actual root-cause fix: since `Makefile:3` already has a bare `export` directive (exporting every Make variable, including `HARBOR_PASSWORD`, to each recipe's process environment), changed `echo "$(HARBOR_PASSWORD)"` to `printf '%s' "$$HARBOR_PASSWORD"` — a shell-side read of the already-exported environment variable instead of a Make-side textual substitution. The password now only ever exists in the process environment, never in argv. Verified: `make -n push` dry-run shows the recipe with `$HARBOR_PASSWORD` unexpanded (safe); reproduced the fix's mechanism standalone (a minimal Makefile with the same bare `export` directive) and confirmed the value reaches `printf` correctly while never appearing in `ps aux` output.

### IN-02: `pkg/httpx/otel.go`'s span-attribute allowlist has no compile-time link to emitted attributes

**Files modified:** `pkg/httpx/otel.go`
**Commit:** `9ae9e52`
**Applied fix:** Documentation-only, per the review's own assessment ("No change required for correctness/security — fails safe"). Added a short comment on `spanAttrAllowlist` (the single source of truth, rather than the one current `span.SetAttributes` call site in `pkg/kafka/consumer.go`, since future call sites in other services would need the same reminder) noting that unlisted attributes are silently dropped and pointing a future contributor debugging a missing trace attribute at the cause. Verified: `pkg/httpx` builds and tests pass.

## Skipped Issues

None — all findings in scope were addressed (4 with code/test changes, 2 — WR-01 and IN-02 — with documentation only, per the explicit "no invented policy" and "fails safe, no change required" guidance in the review itself).

---

_Fixed: 2026-09-26T09:10:28Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
