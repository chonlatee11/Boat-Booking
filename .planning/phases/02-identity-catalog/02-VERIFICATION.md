---
phase: 02-identity-catalog
verified: 2026-09-27T00:00:00Z
status: gaps_found
score: 3/5 must-haves verified
covered_files:
  - ".planning/REQUIREMENTS.md"
  - ".planning/phases/02-identity-catalog/02-01-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-01-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-02-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-02-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-03-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-03-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-04-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-04-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-05-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-05-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-06-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-06-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-07-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-07-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-08-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-08-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-09-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-09-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-10-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-10-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-11-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-11-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-12-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-12-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-13-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-13-SUMMARY.md"
  - "services/catalog/internal/adapters/postgres/queries/boats.sql"
  - "services/catalog/internal/app/boat.go"
  - "services/catalog/internal/app/operator.go"
  - "services/catalog/internal/app/route.go"
  - "services/gateway/internal/adapters/http/auth.go"
  - "services/gateway/internal/adapters/http/proxy.go"
  - "services/identity/internal/app/otp.go"
covered_digest: "v1:sha256:e6fe9731be8df0bb17846f4b01ef3f91b90cd15f0a309986c1de45bd9ee0ef8e"
behavior_unverified: 0
overrides_applied: 0
gaps:
  - truth: "SC1/AUTH-01/D-03: A wrong code returns InvalidArgument with Attempts-Left, and the 5th wrong attempt deletes the code so the OTP brute-force lockout actually holds (02-02-PLAN must-have)"
    status: failed
    reason: "services/identity/internal/app/otp.go VerifyOtp is check-then-increment: it reads the stored hash (HGetAll), compares it, and only increments the attempt counter (HIncrBy) after a mismatch. Concurrent VerifyOtp calls for the same destination all read the pre-increment state, so N parallel guesses are not throttled by otpMaxAttempts — the real limit is concurrency-in-flight, not 5. This is CR-01 in 02-REVIEW.md, confirmed unfixed in the current otp.go (lines ~145-162) and confirmed by the absence of any concurrent-wrong-guess test (only TestOtpAttemptsLockout — sequential — and TestOtpSingleUseConcurrent — correct-code race — exist in services/identity/cmd/main_integration_test.go)."
    artifacts:
      - path: "services/identity/internal/app/otp.go"
        issue: "Attempt counter (HIncrBy) runs only inside the mismatch branch, after the hash comparison already happened against pre-increment state — no atomic check-and-increment"
    missing:
      - "Atomically increment the attempt counter before/with the comparison (e.g. pipelined HGetAll+HIncrBy) so concurrent guesses are throttled by real attempt count, not by request timing"
      - "A test with N parallel wrong guesses + 1 correct guess proving the lockout holds under concurrency"
  - truth: "SC3/SC4/CAT-04/AUTH-05: the boat's operator_id is always its home pier's operator, so tenant ownership and scoping remain correct after any edit (02-08-PLAN must-have)"
    status: failed
    reason: "UpsertBoat correctly derives b.OperatorID from the home pier on every call, but updateBoat() in services/catalog/internal/app/boat.go never writes operator_id — UpdateBoatParams only sets home_pier_id/name/default_capacity/status (confirmed in the current boat.go and boats.sql UpdateBoat query). Moving a boat to a pier owned by a different operator leaves the stored row with the old operator_id, breaking the documented invariant, hiding the boat from both operators' scoped admin views, and publishing a stale operator_id in catalog.BoatUpserted (schedule-service would attribute the boat to the wrong tenant). This is CR-02 in 02-REVIEW.md, confirmed unfixed. No test in boats_photo_integration_test.go covers a cross-operator home-pier move."
    artifacts:
      - path: "services/catalog/internal/app/boat.go"
        issue: "updateBoat() omits OperatorID from postgres.UpdateBoatParams"
      - path: "services/catalog/internal/adapters/postgres/queries/boats.sql"
        issue: "UpdateBoat query does not set operator_id"
    missing:
      - "Persist the derived operator_id on UpdateBoat (or reject cross-operator home-pier moves outright)"
      - "A test moving a boat's home_pier_id to another operator's pier and asserting operator_id is updated (or the move is rejected)"
deferred: []
advisory: []
human_verification:
  - test: "02-09 (admin scaffold): open http://localhost:3002 after `make up`, sign in with SUPER_ADMIN_EMAIL (code from Mailpit at :8025), create two operators, rename one, archive the empty one; resize to 768px and 360px"
    expected: "Thai UI in IBM Plex Sans Thai; OTP error copy matches the contract; nav shows all five super_admin entries; operator dialog/toast/skeleton/empty/archive-confirm behave per UI-SPEC; table scrolls inside its container on narrow widths; a long email wraps in the 'code sent to' line"
    why_human: "Typography, layout overflow and interaction feel need a real browser; no Playwright suite exists yet (deferred to end-of-phase per workflow.human_verify_mode)"
  - test: "02-10 (customer web OTP login): open http://localhost:3001/th/login at 360px, sign in with a 40+ char email (code from Mailpit), enter a wrong code twice, request again immediately, switch to /en/login, sign out"
    expected: "TH/EN contract copy; long email wraps without horizontal scroll; attempts-left counts down 4, 3; immediate re-request shows cooldown copy; header shows signed-in then signed-out; no token visible in devtools Local Storage"
    why_human: "Copy rendering, 360px wrapping and cookie-only storage need a real browser"
  - test: "02-11 (admin Piers page): as super_admin create a pier by clicking the map, drag the marker, upload a 4MB PNG then try a 6MB file and a GIF; as that pier's pier_admin edit it; create a route using the pier and try to archive it; block tile requests in devtools and reopen the Sheet"
    expected: "Marker follows click/drag and lat/lng inputs stay in sync; valid photo shows a thumbnail and appears in GET /api/v1/public/piers; oversize/GIF show validation copy; blocked archive shows the inline route list; blocked tiles show 'โหลดแผนที่ไม่สำเร็จ' with manual lat/lng still usable; Sheet scrolls with footer pinned"
    why_human: "Map interaction, real uploads and visual layout need a browser"
  - test: "02-12 (admin Routes/Boats pages): as a pier_admin create a route to another operator's pier, break then fix the cancellation policy, add adult 150.50 / child 80 prices effective today plus a future price, create the return route, archive a route; create/maintain/archive a boat; narrow to 768px"
    expected: "Derived route names; policy validation blocks save then passes; price history newest-first, current price shows ฿150.50; return route pre-fills swapped piers; boat status badges green/orange; tables scroll inside container; long names truncate with tooltip"
    why_human: "Form ergonomics and visual states need a browser"
  - test: "02-13 (admin Staff page): as super_admin create a pier_admin with two piers of operator A; sign in as that user in a private window (code from Mailpit) and open /piers /routes /boats; back as super_admin remove one pier and disable the user; in the private window let the access token expire (or clear the cookie) and navigate"
    expected: "New user sees only assigned piers/routes/boats; after the pier change, next refresh shows one pier; after disable, refresh fails and the user is sent to /login; pier badges wrap without misaligning rows"
    why_human: "Cross-session behaviour and visual wrapping need two browser sessions"
---

# Phase 02: Identity + Catalog Verification Report

**Phase Goal:** Customers and staff can authenticate with correct roles/scoping, and pier_admin can manage the catalog data (piers/routes/boats/prices) that customers browse publicly
**Verified:** 2026-09-27T00:00:00Z
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (Roadmap Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Customer requests OTP via email/phone and receives a session (JWT httpOnly cookie) without a password account | ✓ VERIFIED (with a security caveat) | `services/gateway/internal/adapters/http/auth.go` implements `/api/v1/auth/otp/request` and `/otp/verify`, setting `access_token`/`refresh_token` cookies; `services/identity/internal/app/otp.go` auto-creates a `role=customer` row with no password field in `services/identity/migrations/00002_users.sql`. Integration tests `TestEmailOtpLogin`, `TestPhoneOtpViaDevSms`, `TestOtpCooldownAndHourlyLimit` pass. **However** the OTP brute-force lockout (D-03, part of this truth's security contract) is bypassable under concurrent guessing — see Gap 1 (CR-01). |
| 2 | staff/pier_admin/super_admin log in and receive `role`+`operator_id` claims; Kong verifies JWT; BFF forwards claims as trusted headers; requests missing headers rejected | ✓ VERIFIED | `pkg/auth/auth.go` issues RS256 JWTs with role/operator_id/pier_ids; `httpx.ForwardClaims` (`pkg/httpx/claims.go`) strips any inbound spoofed `X-*` headers before setting verified ones; `httpx.RequireInternal` is wired via `pr.Use(httpx.RequireInternal(token))` in both `services/catalog/cmd/main.go:140` and `services/identity/cmd/main.go:189`, rejecting requests without the internal token (401). Code review corroborates: "the trusted-header boundary works... RequireInternal checks the token in constant time." |
| 3 | super_admin creates pier_admin/staff users assigned to operator+pier; every admin query scoped by `operator_id` so a pier_admin never sees another operator's data | ✗ FAILED (partial) | `services/identity/internal/app/users.go` UpsertUser validates piers against catalog before persisting (AUTH-04); catalog/identity scoped SQL queries AND `operator_id` with `pier_ids` throughout (confirmed by review: "Scoped SQL always ANDs operator_id with pier_ids"). **But** the boat cross-operator move bug (CR-02, Gap 2) breaks the operator_id invariant specifically for boats after an edit — see Gap 2. |
| 4 | super_admin creates/edits operators, creates piers; pier_admin edits/archives assigned piers (map picker), creates/edits/archives routes (tiered cancellation policy), boats, and per-route ticket prices (adult/child, integer satang) via admin UI | ✓ VERIFIED, UI polish pending human check | Backend: `services/catalog/internal/app/{operator,pier,route,price,boat}.go` + matching integration tests (`operators_piers_integration_test.go`, `routes_prices_integration_test.go`, `boats_photo_integration_test.go`) implement all CRUD/archive/pricing rules including D-13 tiered cancellation policy validation and integer-satang prices. Admin UI: `apps/admin/src/app/(admin)/{operators,piers,routes,boats}/*` wire these through the admin proxy (`rpc('catalog', 'UpsertPier'|'UpsertRoute'|'UpsertBoat'|'AddRoutePrice', ...)`). End-to-end visual/interaction behavior deferred to human verification (see below) per `workflow.human_verify_mode=end-of-phase`. |
| 5 | Public search lists piers and routes with coordinates for the map, without authentication | ✓ VERIFIED | `services/gateway/internal/adapters/http/bff.go:31` mounts `GET /api/v1/public/{resource}` outside `httpx.RequireInternal`; `publicHandler` (`proxy.go:125`) calls CatalogService with only the internal token, no claims. `ListPiersPublic` (`piers.sql:32-35`) selects `piers.*` (includes `lat`, `lng`) filtered to non-archived. `ListRoutesPublic` (`routes.sql:29-34`) returns non-archived routes referencing pier ids; the client resolves coordinates by joining with the public piers list. |

**Score:** 3/5 truths fully verified, 2/5 failed on a critical sub-invariant (0 present-but-behavior-unverified)

### Gaps Summary

Both gaps are the two **critical** findings from `02-REVIEW.md` (CR-01, CR-02), independently reproduced by reading the current code — neither was fixed by the post-review commit (`7c1e37b`, which only addressed gosec/gofmt lint issues unrelated to these findings):

1. **OTP attempt-limit race (CR-01).** `VerifyOtp` compares the code before counting the attempt, so the 5-guess lockout (D-03) does not hold against concurrent requests — a small botnet can brute-force a 6-digit OTP within its 5-minute TTL. This directly undermines the security contract of Success Criterion 1 (OTP login), a must-have in `02-02-PLAN.md`.
2. **Boat cross-operator move leaves stale `operator_id` (CR-02).** `UpdateBoat` never re-persists the derived `operator_id`, so moving a boat to another operator's pier breaks the "operator_id is always home pier's operator" invariant that AUTH-05/CAT-04 scoping depends on — a must-have in `02-08-PLAN.md`. The boat becomes invisible to both operators' scoped views and downstream `catalog.BoatUpserted` consumers get the wrong tenant.

Both are exploitable/incorrect-behavior bugs in security- and tenant-isolation-critical paths, not cosmetic issues — they block a clean phase-goal pass even though the large majority of Phase 2 (routing, scoping architecture, catalog CRUD, pricing, public reads, admin UI wiring) is solidly implemented and tested.

### Required Artifacts (spot-checked)

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `services/identity/internal/app/otp.go` | RequestOtp/VerifyOtp with atomic attempt limiting | ⚠️ STUB-LIKE (logic gap) | Present, wired, tested for sequential attempts and correct-code race, but the attempt-limit invariant fails under concurrency (Gap 1) |
| `services/catalog/internal/app/boat.go` | UpsertBoat/updateBoat preserving `operator_id = home pier's operator` | ⚠️ STUB-LIKE (logic gap) | Present, wired; `operator_id` correctly derived on create but not persisted on update (Gap 2) |
| `services/gateway/internal/adapters/http/auth.go` | Cookie-based OTP/refresh/logout routes | ✓ VERIFIED | Implements request/verify/refresh/logout with httpOnly cookies |
| `services/gateway/internal/adapters/http/proxy.go` | Admin proxy (claim-forwarding, allow-list) + public proxy (claim-less) | ✓ VERIFIED | `adminProxy`/`publicHandler` present and tested (`proxy_test.go`) |
| `pkg/auth/auth.go`, `pkg/httpx/claims.go` | Role/pier_ids claims, ForwardClaims, RequireInternal | ✓ VERIFIED | Confirmed by review and direct read; `pier_ids` claim round-trips |
| `services/catalog/internal/app/{operator,pier,route,price}.go` | Operator/pier/route/price CRUD with scope + tiered policy | ✓ VERIFIED | Present, wired, tested (`operators_piers_integration_test.go`, `routes_prices_integration_test.go`) |
| `apps/admin/src/app/(admin)/{operators,piers,routes,boats,staff}/*` | Admin UI CRUD pages | ✓ VERIFIED (wiring) | rpc() calls to the correct CatalogService/UserService methods confirmed; visual/UX correctness deferred to human verification |
| `apps/web/src/components/otp-login.tsx` | Customer OTP login (TH/EN) | ✓ VERIFIED (wiring) | Calls `/api/v1/auth/otp/request` and `/verify`; visual correctness deferred to human verification |

### Key Link Verification

| From | To | Via | Status |
|------|----|----|--------|
| `services/gateway/internal/adapters/http/bff.go` | `pkg/httpx/claims.go` | `ForwardClaims` before every outbound admin-proxy call | ✓ WIRED |
| `services/catalog/cmd/main.go`, `services/identity/cmd/main.go` | `pkg/httpx/claims.go` | `pr.Use(httpx.RequireInternal(token))` | ✓ WIRED |
| `services/identity/internal/app/users.go` | catalog `CatalogService.ListPiers` | forwarded claims + internal token, validates piers before UpsertUser | ✓ WIRED |
| `services/catalog/internal/app/boat.go` | `pkg/outbox` | `publishBoatUpserted` in the same tx | ✓ WIRED (but carries stale operator_id on cross-operator update — Gap 2) |
| `apps/admin/src/lib/api.ts`, `apps/web/src/lib/api.ts` | `/api/v1/auth/refresh` | one-shot refresh retry on 401 | ✓ WIRED |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|-------------|--------|----------|
| AUTH-01 | 02-02, 02-05, 02-10 | Customer OTP login, no password | ✓ SATISFIED (with security caveat, Gap 1) | otp.go, auth.go, apps/web otp-login.tsx |
| AUTH-02 | 02-02, 02-05, 02-07, 02-09, 02-13 | staff/pier_admin/super_admin login with role+operator_id claims | ✓ SATISFIED | session.go, bootstrap.go, users.go |
| AUTH-03 | 02-01, 02-04, 02-05 | Kong verifies JWT; BFF forwards trusted headers; missing headers rejected | ✓ SATISFIED | claims.go, proxy.go, RequireInternal wiring |
| AUTH-04 | 02-07, 02-13 | super_admin creates pier_admin/staff assigned to operator+pier | ✓ SATISFIED | users.go UpsertUser, staff-dialog.tsx |
| AUTH-05 | 02-01, 02-03, 02-06, 02-07, 02-08 | Every admin query scoped by operator_id | ✗ BLOCKED (partial — boats) | scope.go correct in general; boat.go breaks the invariant on update (Gap 2) |
| CAT-01 | 02-03, 02-09 | super_admin creates/edits operators | ✓ SATISFIED | operator.go, operator-dialog.tsx |
| CAT-02 | 02-03, 02-06, 02-08, 02-11 | pier_admin CRUD/archive piers (map picker) | ✓ SATISFIED | pier.go, map-picker.tsx |
| CAT-03 | 02-06, 02-12 | pier_admin CRUD/archive routes (tiered cancellation policy) | ✓ SATISFIED | route.go, policy-editor.tsx |
| CAT-04 | 02-08, 02-12 | pier_admin creates/edits boats | ✗ BLOCKED (partial) | boat.go — create path correct, update path breaks operator_id (Gap 2) |
| CAT-05 | 02-06, 02-12 | Per-route ticket prices, integer satang | ✓ SATISFIED | price.go, price-section.tsx, money.ts |
| CAT-06 | 02-03, 02-04, 02-06, 02-08, 02-11 | Public search lists piers/routes with coordinates, no auth | ✓ SATISFIED | publicHandler, ListPiersPublic/ListRoutesPublic |

No orphaned requirements — REQUIREMENTS.md's Phase 2 mapping (AUTH-01..05, CAT-01..06) exactly matches the union of `requirements:` fields across all 13 plans.

### Anti-Patterns Found

No unresolved `TBD`/`FIXME`/`XXX` markers found in phase-modified files. The two logic gaps above (CR-01, CR-02) are not marker-based debt — they are unaddressed critical findings from `02-REVIEW.md` with no corresponding code change since the review, and no override was recorded in this VERIFICATION.md's frontmatter.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| OTP integration tests exist and are named as expected | `grep -n "func Test" services/identity/cmd/main_integration_test.go` | `TestOtpAttemptsLockout`, `TestOtpSingleUseConcurrent`, `TestOtpCooldownAndHourlyLimit`, `TestEmailOtpLogin`, `TestPhoneOtpViaDevSms`, `TestOtpNeverLogged` found | ✓ PASS (existence) |
| No concurrent-wrong-guess test exists (confirms Gap 1 has no regression coverage) | `grep -n "concurrent" -i main_integration_test.go` | Only single-use-correct-code concurrency test found, no wrong-guess concurrency test | ✓ CONFIRMS GAP |
| No cross-operator boat move test exists (confirms Gap 2 has no regression coverage) | `grep -n "func Test" boats_photo_integration_test.go` | `TestBoatsScopedByHomePier`, `TestPierPhotoKeyAndUrl`, `TestPierPhotoUrlEmptyWithoutStorageConfig` — no cross-operator move test | ✓ CONFIRMS GAP |

Per orchestrator-provided facts (not re-run in this verification pass): `go build`/`go vet` clean, `make test` and `make lint` pass, integration tests (testcontainers) pass. These are consistent with — and do not contradict — the two logic gaps found above, since neither gap is currently covered by a failing test (that is precisely the problem: the gaps are real but untested).

### Human Verification Required

See the 5 items harvested from `<verify><human-check>` blocks deferred to end-of-phase in plans 02-09 through 02-13 (workflow.human_verify_mode=end-of-phase), listed in the frontmatter `human_verification` section above. These cover: admin scaffold visual/interaction QA, customer web OTP login visual QA, map/photo upload interaction QA, routes/boats form ergonomics QA, and cross-session staff scoping/disable QA.

---

_Verified: 2026-09-27T00:00:00Z_
_Verifier: Claude (gsd-verifier)_
