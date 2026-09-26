---
phase: 01-platform-foundation
reviewed: 2026-09-26T09:45:00Z
depth: standard
files_reviewed: 9
files_reviewed_list:
  - deploy/postgres/init.sh
  - Makefile
  - pkg/httpx/errors.go
  - pkg/httpx/errors_test.go
  - pkg/httpx/otel.go
  - pkg/outbox/outbox.go
  - pkg/outbox/outbox_integration_test.go
  - services/catalog/internal/adapters/http/routes.go
  - services/gateway/internal/adapters/http/bff.go
findings:
  critical: 0
  warning: 2
  info: 1
  total: 3
status: issues_found
---

# Phase 01: Code Review Report (incremental re-review)

**Reviewed:** 2026-09-26T09:45:00Z
**Depth:** standard
**Files Reviewed:** 9
**Status:** issues_found

## Summary

This is an incremental re-review of the 9 files touched since the prior
review (`2643ec3`) while fixing that review's 6 findings (`01-REVIEW-FIX.md`,
commits `6707df5`, `c795c14`, `fced546`, `1a3e99e`, `eb1e7e8`, `9ae9e52`).
Each fix was traced against its diff and re-verified against the current
full file content, not just the patch hunk:

- **CR-01** (outbox relay rollback-on-poison-row) — **verified fixed**.
  `Relay.publishOnce` now logs and marks a row poison'd instead of
  `return`ing an error that rolled back the whole transaction. Confirmed the
  `break` (Kafka-publish-failure) and `continue` (unmarshal-failure) paths
  both leave `published` correctly populated, re-read the new
  `TestRelaySkipsPoisonRowWithoutLosingEarlierProgress` test and confirmed
  it actually exercises the regression (good row committed via
  `bbpgx.WithTx`+`Insert`, poison row inserted directly with an invalid
  payload, asserts both `published_at` values). Builds clean
  (`go vet -tags=integration ./outbox/...`).
- **WR-02** (`httpx.WriteError` message leak on unmapped codes) — **verified
  fixed**. The hide-message condition now keys off `status !=
  http.StatusInternalServerError` instead of two specific codes, which by
  construction can never drift from the status-mapping switch again. Ran
  `go test ./pkg/httpx/...`: both the new regression test
  (`CodeAborted` → generic message) and the control test (`CodeNotFound` →
  real message) pass.
- **WR-03** (`init.sh` unescaped password in SQL literal) — **verified
  fixed**. `escaped_password` doubles embedded `'` before interpolation into
  the `CREATE ROLE ... PASSWORD '...'` literal; `sh -n` syntax-checks clean.
- **IN-01** (`Makefile push` password via `echo`) — **verified fixed, and
  correctly diagnosed the real root cause** (Make-side textual substitution
  into argv, not `echo` vs `printf`) rather than a cosmetic swap. See new
  WR-04 below though — the identical pattern is still present, unfixed, in
  `ci-up`, a few lines down in the same file.
- **WR-01** (missing role check) and **IN-02** (allowlist has no
  compile-time link to emitted attributes) — both fixed with comments only,
  per the original review's own "no invented policy" / "fails safe, no code
  change needed" guidance. Confirmed the TODO comments are present,
  symmetric, and correctly cross-reference both call sites.

No new bugs were introduced by any of the applied fixes. Two issues survived
this full-file re-read that were not part of the prior review's 6 findings
(both in files that were already in scope for a full review, one of them a
near-exact repeat of a pattern the fix itself just eliminated elsewhere in
the same file):

## Warnings

### WR-04: `Makefile`'s `ci-up` target leaks `HARBOR_ADMIN_PASSWORD` via `curl -u` argv — the exact pattern `push` was just fixed for

**File:** `Makefile:253-254`
**Issue:** `IN-01`'s fix (commit `eb1e7e8`) correctly root-caused and removed
the Harbor password from process argv in the `push` target (Make-side
substitution → shell reads the already-exported env var instead). A few
lines away, `ci-up` has the identical exposure, untouched by that fix:

```make
ci-up: ci-keys
	deploy/ci/harbor-prepare.sh
	$(COMPOSE_CI) up -d --build --wait
	@HARBOR_ADMIN_PASSWORD=$$(grep -E '^HARBOR_ADMIN_PASSWORD=' .env | tail -1 | cut -d= -f2-); \
	STATUS=$$(curl -s -o /dev/null -w '%{http_code}' -u "admin:$$HARBOR_ADMIN_PASSWORD" \
		-X POST http://localhost:8880/api/v2.0/projects \
		...
```

Here the password is read straight into a *shell* variable (not a Make
variable, so IN-01's specific "Make substitutes into argv" mechanism doesn't
apply) but it still ends up as a literal `-u admin:<password>` argument on
`curl`'s command line, visible to any other local user via `ps`/`/proc` for
the duration of the call — the same class of exposure `IN-01` flagged for
`docker login`, just via a different code path (`curl -u` instead of a
piped `echo`). This was in-scope in the original 133-file review (Makefile
was reviewed) but not caught there.
**Fix:** Use curl's `--netrc-file`/`-K` config-file form, or read the
password into an `Authorization: Basic` header built by curl's own
`--user-agent`-style stdin flag — simplest fix consistent with the sibling
target: pass credentials via a temp netrc (`curl -K -` with `--user` piped
via `-K` is more work than needed here). The smallest change that matches
what `push` now does is to avoid putting the secret on argv at all, e.g.:
```make
	@STATUS=$$(HARBOR_ADMIN_PASSWORD=$$(grep -E '^HARBOR_ADMIN_PASSWORD=' .env | tail -1 | cut -d= -f2-) \
		curl -s -o /dev/null -w '%{http_code}' --netrc-file <(printf 'machine localhost login admin password %s\n' "$$HARBOR_ADMIN_PASSWORD") \
		-X POST http://localhost:8880/api/v2.0/projects ...)
```
or, more simply, switch the auth header to be built from an env var curl
reads itself is not directly supported — the pragmatic fix is a short-lived
netrc file passed via `--netrc-file`. Low severity (single-tenant Jenkins
agent container, same threat model IN-01 already accepted as "minor"), but
worth closing since the sibling instance was just fixed for the same reason.

### WR-05: `allowlistExporter.ExportSpans` mutates the span's own shared attribute slice in place, silently corrupting data for any other consumer of the same span

**File:** `pkg/httpx/otel.go:129-143`
**Issue:**
```go
func (e *allowlistExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	filtered := make([]sdktrace.ReadOnlySpan, len(spans))
	for i, s := range spans {
		stub := tracetest.SpanStubFromReadOnlySpan(s)
		kept := stub.Attributes[:0]
		for _, a := range stub.Attributes {
			if allowedSpanAttr(string(a.Key)) {
				kept = append(kept, a)
			}
		}
		stub.Attributes = kept
		filtered[i] = stub.Snapshot()
	}
	return e.next.ExportSpans(ctx, filtered)
}
```
`tracetest.SpanStubFromReadOnlySpan` sets `stub.Attributes = ro.Attributes()`
— it does **not** copy the slice. For the SDK's own `recordingSpan`,
`Attributes()` returns the span's live internal `s.attributes` slice
directly (`otel/sdk/trace/span.go:615-621`, which even sets
`s.attributesShared = true` specifically to flag that the returned slice is
aliased). `kept := stub.Attributes[:0]` followed by `append` performs the
classic in-place filter *on that same backing array* — this doesn't just
filter the copy `ExportSpans` was handed, it overwrites the span's own
attribute storage as a side effect of exporting it.

Today this is silent because `SetupOTel` registers exactly one span
processor (`sdktrace.WithBatcher(&allowlistExporter{...})`), so nothing else
ever reads the span's attributes after this runs. But it violates the
`ReadOnlySpan` contract callers are entitled to rely on ("read-only"), and
it is a latent trap: if a second span processor is ever added (e.g. a
console/debug exporter registered alongside the OTLP one for local
debugging — a very normal thing to reach for), that processor would
silently see the *filtered* attribute set instead of the full one, with no
error, and depending on `BatchSpanProcessor` internals/registration order
could just as easily see attributes it should have seen dropped out from
under it. This is exactly the kind of "spooky action at a distance" bug
that's expensive to debug precisely because there is no error, no test
failure, and no log line pointing at the cause.
**Fix:** Copy before filtering instead of reusing the aliased backing array:
```go
kept := make([]attribute.KeyValue, 0, len(stub.Attributes))
for _, a := range stub.Attributes {
	if allowedSpanAttr(string(a.Key)) {
		kept = append(kept, a)
	}
}
stub.Attributes = kept
```
One extra allocation per exported span batch; correctness over the
zero-copy trick, and it removes the dependency on "this exporter happens to
be the only span consumer today" holding forever.

## Info

### IN-03: Outbox poison-row skip has no metric, only a log line — a permanently-lost event is invisible to dashboards/alerts

**File:** `pkg/outbox/outbox.go:238-249`
**Issue:** The CR-01 fix correctly stops the poison row from wedging the
batch or rolling back earlier progress, but the only signal it emits is
`r.log.Error(...)`. Unlike the Kafka-publish-failure branch a few lines
below, which increments `r.publishErrors` (a registered
`outbox.publish_errors` counter feeding dashboards/alerts), a skipped
poison row — which means an event is **permanently and silently never
published to Kafka** — increments nothing. An operator without live log
tailing/alerting on this exact log line would have no way to notice a
business event was dropped. This mirrors the symmetric, already-accepted
design on the consumer side (`pkg/kafka/consumer.go`'s `processRecord`,
which also only logs on unmarshal failure), so it's a consistent choice
rather than a regression — flagging as a maintainability/observability gap
for both sides, not a defect introduced by this fix.
**Fix:** Reuse (or add a sibling to) `r.publishErrors.Add(ctx, 1)` in the
poison-row branch so both classes of "an event never made it to Kafka" are
visible on the same metric/alert.

---

_Reviewed: 2026-09-26T09:45:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
