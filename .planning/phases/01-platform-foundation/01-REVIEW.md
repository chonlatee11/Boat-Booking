---
phase: 01-platform-foundation
reviewed: 2026-09-26T18:30:00Z
depth: standard
files_reviewed: 4
files_reviewed_list:
  - Makefile
  - deploy/observability/grafana/provisioning/datasources/datasources.yaml
  - pkg/httpx/otel.go
  - pkg/outbox/outbox.go
findings:
  critical: 0
  warning: 0
  info: 0
  total: 0
status: clean
---

# Phase 01: Code Review Report (incremental re-review)

**Reviewed:** 2026-09-26T18:30:00Z
**Depth:** standard
**Files Reviewed:** 4
**Status:** clean

## Summary

This is a second incremental re-review, scoped to the diff between
`8d068e2` and `HEAD` on the 4 files that changed: the three fixes for the
prior review's `WR-04`/`WR-05`/`IN-03` findings (`0b3df66`, `bd990fc`,
`425f926`), plus plan `01-14`'s unrelated Grafana trace-to-logs window
widening (`e54e24b`, gap `G-01-7`). Each change was traced against the full
current file, not just the patch hunk, and cross-checked against what the
prior `01-REVIEW.md`/`01-REVIEW-FIX.md` claimed was fixed:

- **WR-04** (`Makefile` `ci-up` leaking `HARBOR_ADMIN_PASSWORD` via `curl -u`
  argv) — **verified fixed, no regression**. The recipe now builds a
  netrc-style config line (`printf 'user = "admin:%s"\n' "$$HARBOR_ADMIN_PASSWORD"`)
  and feeds it to `curl -K -`, so the secret only ever reaches curl over its
  stdin, never as a literal argv token another local user could read via
  `ps`/`/proc`. `printf` here is a bash builtin (the Makefile sets
  `SHELL := bash`), so no separate process is spawned with the secret as an
  argument either. Confirmed `make -n ci-up` renders syntactically valid
  shell with the Make-level `$$` escaping intact, and `bash -n` on the
  extracted recipe body parses clean. `HARBOR_ADMIN_PASSWORD` is generated
  by `deploy/ci/harbor-prepare.sh` via `openssl rand -base64 32 | tr -dc
  'A-Za-z0-9' | head -c 24` — alphanumeric only, so no embedded `"`/`\` that
  could break the config-file quoting. The sibling `push` target's
  `--password-stdin` pattern this fix mirrors is unchanged and still correct.
- **WR-05** (`allowlistExporter.ExportSpans` mutating the SDK's shared,
  aliased attribute slice in place) — **verified fixed, no regression**.
  `kept := stub.Attributes[:0]` was replaced with `kept := make([]attribute.KeyValue,
  0, len(stub.Attributes))`, exactly the suggested fix — a fresh backing
  array is allocated before filtering, so the export path no longer writes
  through to `ReadOnlySpan.Attributes()`'s live storage. `go build`/`go vet`
  on `pkg/httpx` are clean.
- **IN-03** (outbox poison-row skip had no metric, only a log line) —
  **verified fixed, no regression**. `r.publishErrors.Add(ctx, 1)` was added
  in the unmarshal-failure branch of `publishOnce`, immediately mirroring
  the existing increment in the Kafka-publish-failure branch a few lines
  below. `ctx` is the enclosing `publishOnce(ctx context.Context)` parameter,
  correctly captured by the `bbpgx.WithTx` closure (no shadowing). Both
  "event never reached Kafka" cases now feed the same
  `outbox.publish_errors` counter. `go build`/`go vet` on `pkg/outbox` are
  clean.
- **Plan 01-14 Grafana change** (unrelated to the above three fixes) —
  reviewed against its own plan/summary for correctness. Added
  `spanStartTimeShift: '-1m'` and `spanEndTimeShift: '1m'` under the Tempo
  datasource's `tracesToLogsV2`, with `filterByTraceID: true` and
  `datasourceUid: loki` left untouched. YAML indentation and nesting are
  correct; the values match exactly what the plan specified and what its
  automated verify asserted against the live Grafana API. `filterByTraceID`
  staying `true` means the wider window can't leak logs from unrelated
  traces, addressing the plan's own threat register entry (T-01-14-01) for
  this exact concern.

No new bugs were introduced by any of the three review-fix commits, and the
Grafana config change is a minimal, correctly-scoped, already-verified
config-only edit. Quick-pattern scans (hardcoded secrets, dangerous
functions, debug artifacts, empty catches) across all 4 files returned no
hits. All reviewed files meet quality standards for this diff.

