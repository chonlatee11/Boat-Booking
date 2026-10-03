---
phase: 02-identity-catalog
reviewed: 2026-09-28T16:20:31Z
depth: standard
files_reviewed: 21
files_reviewed_list:
  - apps/admin/src/app/(admin)/piers/pier-sheet.tsx
  - apps/admin/src/app/(admin)/routes/policy-editor.tsx
  - apps/admin/src/app/(admin)/routes/queries.ts
  - apps/admin/src/app/(admin)/routes/route-sheet.tsx
  - apps/admin/src/components/map-picker.tsx
  - deploy/proof.sh
  - services/catalog/cmd/boats_photo_integration_test.go
  - services/catalog/cmd/operators_piers_integration_test.go
  - services/catalog/internal/adapters/http/operators.go
  - services/catalog/internal/adapters/http/piers.go
  - services/catalog/internal/adapters/http/prices.go
  - services/catalog/internal/adapters/http/route_handlers.go
  - services/catalog/internal/adapters/http/routes.go
  - services/catalog/internal/adapters/http/scope.go
  - services/catalog/internal/adapters/http/scope_test.go
  - services/catalog/internal/app/pier.go
  - services/identity/internal/adapters/http/routes.go
  - services/identity/internal/adapters/http/routes_test.go
  - services/identity/internal/adapters/http/users.go
  - services/identity/internal/adapters/http/users_test.go
  - services/identity/internal/adapters/notify/notify.go
  - services/identity/internal/adapters/notify/notify_test.go
findings:
  critical: 0
  warning: 1
  info: 3
  total: 4
status: issues_found
---

# Phase 02: Code Review Report (incremental)

**Reviewed:** 2026-09-28T16:20:31Z
**Depth:** standard
**Files Reviewed:** 21
**Status:** issues_found

**Note:** This is an incremental re-review scoped to `git diff c9bfdb0..HEAD` for the 21 files listed above, run after the 02-14..02-17 gap-closure plans and prior review-fix commits (`8dda7a5`, `4f86a9b`, `037ad5a`, `2908240`, `3f9fbc8`, `fe41f8b`, `c34fcae`, `d27e0c6`, `0259f57`, `2a26bae`). It does not re-review code outside that diff range unless it directly interacts with the new changes.

## Summary

The diff since `c9bfdb0` covers nine gap-closure fixes: threading `ctx` into unmapped-error `slog` calls (catalog + identity), UTF-8 MIME headers for dev OTP email, a concurrent-`UpsertBoat` race-safety proof test, naming the archived-operator rejection reason, MapPicker lat/lng draft validation + tile-error latch fix, pier Sheet operator/hours validation, policy-tier delete-button visibility, pier-name disambiguation in route Sheet selects, and a run-unique pier name in `make proof`.

`go build`, `go vet`, and `go test ./internal/...` are clean for both `catalog` and `identity`; `tsc --noEmit` is clean for `apps/admin`. The changes are generally well-targeted, narrowly-scoped fixes with test coverage. One warning worth fixing before this is considered fully closed, plus a few minor robustness notes below.

## Warnings

### WR-01: PolicyEditor hides delete on the last array index, not on whichever tier is actually the required 0-hour tier

**File:** `apps/admin/src/app/(admin)/routes/policy-editor.tsx:99`
**Issue:** The gap-closure fix for "hide delete on last tier" changed the delete-button condition from `tiers.length > 1` to `index < tiers.length - 1`, i.e. it assumes the array's last element is always the mandatory 0-hour tier (`validatePolicy` requires the sequence to be strictly descending and to contain a 0, so a *valid* policy is guaranteed to have that shape — but the UI list itself doesn't enforce or verify it). The invariant holds today only because: (a) `DEFAULT_POLICY` ends in a 0-hour tier, (b) `addTier` always appends a new `{minHoursBefore: 0, ...}` tier to the end, and (c) any policy loaded from the backend already passed `ValidateCancellationPolicy` on a prior save. If any of those three assumptions is ever violated (e.g. a future edit adds reordering, or a legacy/imported route has an out-of-order policy that happens to still round-trip through `GetRoute`), the delete button would be hidden on a tier that is *not* the 0-hour tier, while the actual 0-hour tier — now not last — becomes deletable, silently defeating the safeguard this exact commit (`d27e0c6`) was written to add. `validatePolicy` will still catch the resulting invalid state and block save, so this is not silent data corruption, but it means the "can't delete the required tier" guarantee the fix claims is actually "can't delete whatever is currently last," which is weaker than intended.
**Fix:** Key the guard off content, not position — hide delete only on the tier that would remove the sole remaining `minHoursBefore === 0` entry:
```tsx
const zeroCount = tiers.filter((t) => (t.minHoursBefore ?? 0) === 0).length;
const canDelete = (tier: CancellationTierJson) =>
  tiers.length > 1 && !((tier.minHoursBefore ?? 0) === 0 && zeroCount === 1);
// ...
{canDelete(tier) && (
  <Button ... onClick={() => removeTier(index)}>
```

## Info

### IN-01: `make proof`'s pier-name suffix has second-level collision risk

**File:** `deploy/proof.sh:33`
**Issue:** `SUFFIX=$(date +%s)` gives second resolution. Two back-to-back invocations of `deploy/proof.sh` within the same wall-clock second (e.g. from a fast CI retry loop, or two developers running it near-simultaneously against a shared dev stack) would mint the operator and pier with identical names, reintroducing the exact same-name ambiguity this script's own fix (`2a26bae`) was written to avoid for the pier row it creates.
**Fix:** Append `$$` (PID) or use `date +%s%N` for sub-second uniqueness, e.g. `SUFFIX="$(date +%s)-$$"`.

### IN-02: `pierName`'s duplicate-name check runs against the full (unfiltered) pier lists, including archived piers

**File:** `apps/admin/src/app/(admin)/routes/queries.ts:61-73`, called from `apps/admin/src/app/(admin)/routes/route-sheet.tsx:162-166,189-193`
**Issue:** `route-sheet.tsx` passes `ownPiersData?.piers` / `publicPiersData?.piers` (the raw, unfiltered query results) into `pierName`, not the `ownPiers`/`publicPiers` arrays that are actually rendered as `<NativeSelectOption>`s (which exclude archived piers). If an archived pier happens to share a Thai name with a currently-selectable pier, the visible option gets an unnecessary `(id-tail)` suffix disambiguating it from a pier the admin can't even select. Cosmetic only — no data-correctness impact — but it doesn't match the stated intent ("so the two are distinguishable" implies two *selectable* piers).
**Fix:** Pass the already-filtered `ownPiers`/`publicPiers` arrays instead of the raw query data, so disambiguation only fires against genuinely competing visible options.

### IN-03: `hasDuplicate` in `pierName` is O(n) per call inside a `.map()`, and `flatMap` reallocates the combined array on every render

**File:** `apps/admin/src/app/(admin)/routes/queries.ts:66-71`
**Issue:** Not a correctness bug (out of v1 scope per review rules — noting only because it's a `.some()` inside a loop that itself is called inside `ownPiers.map(...)` and `publicPiers.map(...)`, each re-running the flatten + scan per option, per render). At current expected data volumes (piers per operator) this is a non-issue; flagged only for awareness if pier counts grow substantially.
**Fix:** No action needed at current scale; if it ever matters, hoist the flattened list and a name→count map to a single `useMemo` computed once per render instead of once per option.

---

_Reviewed: 2026-09-28T16:20:31Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
