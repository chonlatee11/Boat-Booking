---
phase: "2"
slug: "identity-catalog"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: false
wave_0_complete: true
created: "2026-09-26"
validated: "2026-10-03"
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `go test` (unit) + `//go:build integration` testcontainers (Postgres, Redpanda, Valkey, Mailpit); frontend pure-TS logic via Node built-in `node --test` (no npm dependency); live-stack E2E shell scripts (`deploy/kong/roundtrip.sh`, `deploy/proof.sh`, `deploy/auth-roundtrip.sh`, `deploy/photo-roundtrip.sh`) |
| **Config file** | `go.work`; images pinned in `pkg/testenv` (checked by `TestImagesMatchCompose`); no JS runner config |
| **Quick run command** | `go test -count=1 github.com/chonlatee11/boat-booking/services/<svc>/...` · `cd apps/admin && node --test "src/app/(admin)/routes/money.test.ts" src/lib/api.test.ts` · `cd apps/web && node --test src/lib/api.test.ts` |
| **Full suite command** | `make test && make test-integration` (+ `make up && make kong-roundtrip proof auth-roundtrip photo-roundtrip` for E2E) |
| **Estimated runtime** | unit: seconds · integration: minutes (15m timeout) |

---

## Sampling Rate

- **After every task commit:** Run the per-service `go test` quick command (and the `node --test` files when touching `apps/*` logic covered by them)
- **After every plan wave:** Run `make test && make test-integration`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** unit < 10s; integration is a per-wave gate, not per-commit

---

## Per-Task Verification Map

Threat refs are declared per plan (range listed).

| Task ID | Plan | Wave | Requirement | Threat Ref | Test Type | Automated Command / Evidence | File Exists | Status |
|---------|------|------|-------------|------------|-----------|------------------------------|-------------|--------|
| 2-01-01 | 01 | 1 | AUTH-02/03/05 | T-02-SC | checkpoint:decision | manual (UAT #2) | n/a | ✅ green |
| 2-01-02 | 01 | 1 | AUTH-02/03/05 | T-02-SC | checkpoint:human-verify | manual (UAT #2) | n/a | ✅ green |
| 2-01-03 | 01 | 1 | AUTH-02/03/05 | T-02-01-01..04 | unit + e2e | TestIssueVerifyRoundTripWithPierIDs, TestRequireInternal*PierIDs*, TestForwardClaims*, TestWhoamiReturnsPierIDs; `make kong-roundtrip` | ✅ | ✅ green |
| 2-02-01 | 02 | 2 | AUTH-01/02/03 | T-02-02-01..10 | integration | TestEmailOtpLogin, TestExistingStaffLoginGetsClaims, TestRejectsMissingInternalToken | ✅ | ✅ green |
| 2-02-02 | 02 | 2 | AUTH-01/02/03 | T-02-02-01..10 | unit + integration | TestOtpCooldownAndHourlyLimit, TestOtpAttemptsLockout(+Concurrent), TestOtpSingleUseConcurrent, TestPhoneOtpViaDevSms, TestOtpNeverLogged, TestNormalizeDestination, TestResendSender* | ✅ | ✅ green |
| 2-02-03 | 02 | 2 | AUTH-01/02/03 | T-02-SC | unit + smoke | TestImagesMatchCompose; `make up` healthcheck | ✅ | ✅ green |
| 2-03-01 | 03 | 2 | CAT-01/02/06, AUTH-05 | T-02-03-01..07 | integration | TestOperatorsSuperAdminOnly, TestUpsertBoatPublishesBoatUpserted, TestListBoatsOrdered | ✅ | ✅ green |
| 2-03-02 | 03 | 2 | CAT-01/02/06, AUTH-05 | T-02-03-01..07 | unit + integration | TestPiersScoping, TestArchiveOperatorBlockedByPiers, TestPierValidate | ✅ | ✅ green |
| 2-04-01 | 04 | 2 | AUTH-03, CAT-06 | T-02-04-01..07 | unit + e2e | TestAdminProxyForwardsVerifiedClaimsAndStripsSpoofed, TestAdminProxyRejects; `make proof` | ✅ | ✅ green |
| 2-04-02 | 04 | 2 | AUTH-03, CAT-06 | T-02-04-01..07 | unit | TestPublicProxyIsClaimLess, TestPublicProxyUnknownResource404, TestPublicProxyPassthrough | ✅ | ✅ green |
| 2-05-01 | 05 | 3 | AUTH-01/02/03 | T-02-05-01..07 | unit + e2e | TestOtpVerifySetsCookiesAndBodyHasNoTokens, TestOtpRequestSpoofedClaimHeaderNeverReachesIdentity, TestWriteErrorMaps*; `make auth-roundtrip` | ✅ | ✅ green |
| 2-05-02 | 05 | 3 | AUTH-01/02/03 | T-02-05-01..07 | unit + integration | TestRefreshRotationAndReuseDetection, TestRefreshRereadsClaimsAndDisabled, TestRefreshExpired, TestLogoutIdempotent | ✅ | ✅ green |
| 2-05-03 | 05 | 3 | AUTH-01/02/03 | T-02-05-01..07 | integration + e2e | TestEnsureSuperAdminIdempotent, TestSuperAdminLoginViaOtp | ✅ | ✅ green |
| 2-06-01 | 06 | 3 | CAT-02/03/05/06, AUTH-05 | T-02-06-01..07 | unit + integration | TestRoutesScopingAndSharedPierTo, TestValidateCancellationPolicy, TestRouteValidate, TestUpdateRoute* | ✅ | ✅ green |
| 2-06-02 | 06 | 3 | CAT-02/03/05/06, AUTH-05 | T-02-06-01..07 | unit + integration | TestRoutePricesEffectiveDating, TestRoutePriceValidate, TestRoutePriceAmountIsSatang | ✅ | ✅ green |
| 2-06-03 | 06 | 3 | CAT-02/03/05/06, AUTH-05 | T-02-06-01..07 | integration | TestArchivePierBlockedByRoutes | ✅ | ✅ green |
| 2-07-01 | 07 | 4 | AUTH-04/02/05 | T-02-07-01..06 | integration | TestCreateStaffUserValidatesPiers, TestStaffLoginCarriesAssignedClaims, TestStaffUserInputValidate* | ✅ | ✅ green |
| 2-07-02 | 07 | 4 | AUTH-04/02/05 | T-02-07-01..06 | integration | TestUpsertUserIdempotencyAndConcurrency, TestUpdateAndDisableUser | ✅ | ✅ green |
| 2-08-01 | 08 | 4 | CAT-04/02/06, AUTH-05 | T-02-08-01..06 | integration + e2e | TestBoatsScopedByHomePier, TestUpsertBoatValidationAndTenancy, TestUpdateBoatMovedToAnotherOperatorPierUpdatesOperatorID, TestBoatUpsertedAppliedOnce; `make proof` | ✅ | ✅ green |
| 2-08-02 | 08 | 4 | CAT-04/02/06, AUTH-05 | T-02-08-01..06 | unit + integration | TestPresignPierPhoto*, TestPhotosURL, TestPierPhotoUrlEmptyWithoutStorageConfig | ✅ | ✅ green |
| 2-09-01 | 09 | 4 | AUTH-01/02, CAT-01 | T-02-09-01..05 | build + manual | admin lint/typecheck/build; UAT #3 | n/a | ✅ green |
| 2-09-02 | 09 | 4 | AUTH-01/02, CAT-01 | T-02-09-01..05 | build + unit + manual | `cd apps/admin && node --test src/lib/api.test.ts` (refresh-on-401); UAT #4 | ✅ | ✅ green |
| 2-09-03 | 09 | 4 | AUTH-01/02, CAT-01 | T-02-09-01..05 | config | `make lint && make web-check` | n/a | ✅ green |
| 2-10-01 | 10 | 4 | AUTH-01 | T-02-10-01..04 | build + manual | web lint/typecheck/build; UAT #5 | n/a | ✅ green |
| 2-10-02 | 10 | 4 | AUTH-01 | T-02-10-01..04 | unit + manual | `cd apps/web && node --test src/lib/api.test.ts` (refresh-on-401); UAT #72 | ✅ | ✅ green |
| 2-11-01 | 11 | 5 | CAT-02/06 | T-02-11-01..05 | build + manual | admin build; UAT #7 | n/a | ✅ green |
| 2-11-02 | 11 | 5 | CAT-02/06 | T-02-11-01..05 | e2e | `make up && make photo-roundtrip` | ✅ | ✅ green |
| 2-11-03 | 11 | 5 | CAT-02/06 | T-02-11-01..05 | build + manual | admin build; UAT #13 | n/a | ✅ green |
| 2-12-01 | 12 | 5 | CAT-03/04/05 | T-02-12-01..04 | build + manual | admin build; UAT #9 | n/a | ✅ green |
| 2-12-02 | 12 | 5 | CAT-03/04/05 | T-02-12-01..04 | unit + manual | `cd apps/admin && node --test "src/app/(admin)/routes/money.test.ts"`; UAT #10-12 | ✅ | ✅ green |
| 2-12-03 | 12 | 5 | CAT-03/04/05 | T-02-12-01..04 | build + manual | admin build; UAT #14 | n/a | ✅ green |
| 2-13-01 | 13 | 5 | AUTH-04/02 | T-02-13-01..03 | build + manual | admin build; UAT #15 | n/a | ✅ green |
| 2-13-02 | 13 | 5 | AUTH-04/02 | T-02-13-01..03 | build + manual | admin build; UAT #16-17 | n/a | ✅ green |
| 2-14-01 | 14 | 1 (gap) | AUTH-01/02 | T-02-14-01..02 | unit + integration | TestOtpMessageIsUTF8MIME, TestEmailOtpLogin, TestPhoneOtpViaDevSms | ✅ | ✅ green |
| 2-15-01 | 15 | 1 (gap) | CAT-02/04 | T-02-15-01..02 | integration | TestArchiveOperatorBlockedByPiers, TestArchiveOperatorRaceWithPierCreate | ✅ | ✅ green |
| 2-15-02 | 15 | 1 (gap) | CAT-02/04 | T-02-15-01..02 | integration | TestConcurrentUpsertBoatSameID | ✅ | ✅ green |
| 2-16-01 | 16 | 1 (gap) | CAT-02 | T-02-16-01..03 | build + manual | grep `parseCoord`/`sourcedata`; UAT #7 retest | n/a | ⚠️ manual-only |
| 2-16-02 | 16 | 1 (gap) | CAT-02 | T-02-16-01..03 | build + manual | grep `!op.archived`/HHMM; UAT #8 retest | n/a | ⚠️ manual-only |
| 2-17-01 | 17 | 1 (gap) | CAT-03 | T-02-17-01..03 | build + manual | grep `?? 0`, `tiers.length - 1`; UAT #12 retest | n/a | ⚠️ manual-only |
| 2-17-02 | 17 | 1 (gap) | CAT-03 | T-02-17-01..03 | build + manual | grep `pierName(`; UAT #9 retest | n/a | ⚠️ manual-only |
| 2-17-03 | 17 | 1 (gap) | CAT-03 | T-02-17-01..03 | e2e | `make up && make proof` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky / manual-only*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements (Go unit + testcontainers). Frontend pure-logic tests use Node's built-in `node --test` — no framework install. The `node --test` files are not yet wired into `make ci` / `web-check`.

---

## Manual-Only Verifications

All passed human UAT (`02-UAT.md`, 82/82).

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `validatePolicy` defaults absent tier fields to 0, Save enabled on load, no delete on last tier | CAT-03 | Function lives in `policy-editor.tsx` (JSX + `@/` aliases + `@gen` import) — not loadable by `node --test` without extracting to a plain `routes/policy.ts` | UAT #10, #12 |
| `pierName` id-tail suffix only on name collision | CAT-03 | Lives in `routes/queries.ts`, which imports `@tanstack/react-query` — needs extraction to `routes/pier-name.ts` | UAT #9 |
| `parseCoord` lat/lng range + partial-input guard | CAT-02 | Unexported in `map-picker.tsx` (imports `maplibre-gl` + JSX) — needs extraction to a plain module | UAT #7 |
| `HHMM` 24h hours validation | CAT-02 | Unexported const in `pier-sheet.tsx` — needs `export` + plain import | UAT #8 |
| Thai OTP email renders without mojibake in Mailpit | AUTH-01 | MIME bytes unit-tested (TestOtpMessageIsUTF8MIME); mail-client rendering is visual | UAT #3 |
| Admin/web login, operators/piers/routes/boats/staff UIs (dialogs, sheets, map drag, tile-failure copy, badges, responsive tables) | AUTH-01/02/04, CAT-01..05 | Interactive UI; no browser test runner in phase scope | UAT #3-17 |
| Storage server + package legitimacy decision | T-02-SC | Human decision checkpoint | UAT #2 |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies (frontend-only tasks verified by build + human UAT)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [ ] Feedback latency < 2s (integration suite is minutes; unit suite is seconds)
- [ ] `nyquist_compliant: true` set in frontmatter — 4 frontend logic items remain manual-only pending extraction

**Approval:** approved 2026-10-03 (partial)

## Validation Audit 2026-10-03
| Metric | Count |
|--------|-------|
| Gaps found | 5 |
| Resolved | 2 (CAT-05 money.ts, AUTH-01 refresh-on-401 ×2 apps) |
| Escalated | 3 (CAT-03 validatePolicy + pierName, CAT-02 parseCoord + HHMM) |
