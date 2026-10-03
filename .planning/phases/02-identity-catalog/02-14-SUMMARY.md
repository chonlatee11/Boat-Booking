---
phase: 02-identity-catalog
plan: 14
subsystem: notifications
tags: [smtp, mime, rfc2047, otp, thai, utf-8, mailpit, tdd]

# Dependency graph
requires:
  - phase: 02-identity-catalog
    provides: "notify.Sender (SMTPSender, DevSMSSender, ResendSender) and sendSMTPMessage from plan 02-02"
provides:
  - "otpMessage(from, to, subject, code) []byte -- the one shared RFC 5322 message builder for both SMTPSender and DevSMSSender"
  - "MIME-Version, Content-Type: text/plain; charset=UTF-8, Content-Transfer-Encoding: 8bit headers on every SMTP-delivered OTP message"
  - "RFC 2047-encoded Subject header (ASCII on the wire, decodes back to the original Thai text)"
affects: [identity, phase-05-notification]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
actuals:
  tokens: 1452
  tasks: 1
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "SMTP message construction goes through one small pure builder function (otpMessage) that both SMTPSender and DevSMSSender call via sendSMTPMessage, so a header/encoding fix lands once for every SMTP-based transport"

key-files:
  created: []
  modified:
    - services/identity/internal/adapters/notify/notify.go
    - services/identity/internal/adapters/notify/notify_test.go

key-decisions:
  - "Content-Transfer-Encoding: 8bit (not quoted-printable/base64) since this sender only ever talks to Mailpit in dev (D-02); ResendSender (prod) sends JSON and is unaffected/out of scope"
  - "RED phase used a genuine Go build failure (undefined: otpMessage) as intentional RED for a brand-new function, matching the project's established greenfield-TDD precedent (Phase 01 decision log) -- no unrelated compile errors, single missing symbol"

patterns-established:
  - "Charset/MIME-header fixes for future SMTP-based transports in this service should extend otpMessage, not re-introduce a second raw fmt.Sprintf message builder"

requirements-completed: [AUTH-01, AUTH-02]

coverage:
  - id: D1
    description: "otpMessage builds an RFC 5322 message declaring UTF-8 (MIME-Version, Content-Type charset=UTF-8, Content-Transfer-Encoding: 8bit) with an RFC 2047-encoded Subject, used by both SMTPSender and DevSMSSender via sendSMTPMessage"
    requirement: "AUTH-01"
    verification:
      - kind: unit
        ref: "services/identity/internal/adapters/notify/notify_test.go#TestOtpMessageIsUTF8MIME"
        status: pass
      - kind: integration
        ref: "services/identity/cmd#TestEmailOtpLogin"
        status: pass
      - kind: integration
        ref: "services/identity/cmd#TestPhoneOtpViaDevSms"
        status: pass
      - kind: integration
        ref: "services/identity/cmd#TestOtpNeverLogged"
        status: pass
    human_judgment: false
  - id: D2
    description: "Visual confirmation that a live Mailpit-captured OTP email renders the Thai body correctly (not mojibake) and the subject still reads correctly"
    requirement: "AUTH-01"
    verification: []
    human_judgment: true
    rationale: "Requires a human to open the message in the Mailpit UI and visually confirm correct Thai rendering; deferred to end-of-phase UAT per workflow.human_verify_mode=end-of-phase -- the task's <verify><human-check> documents the exact steps for that harvest"

duration: 12min
completed: 2026-09-28
status: complete
---

# Phase 02 Plan 14: OTP SMTP MIME/UTF-8 Fix Summary

**Extracted `otpMessage()` as the single RFC 5322 builder for SMTP-delivered OTP messages, adding MIME-Version/Content-Type/Content-Transfer-Encoding headers and an RFC 2047-encoded Subject so Mailpit stops decoding Thai body text as Latin-1 mojibake (closes gap G-02-3).**

## Performance

- **Duration:** ~12 min
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments
- `otpMessage(from, to, subject, code) []byte` builds the raw message with `MIME-Version: 1.0`, `Content-Type: text/plain; charset=UTF-8`, `Content-Transfer-Encoding: 8bit`, and a `mime.BEncoding`-encoded Subject — used by both `SMTPSender` and `DevSMSSender` through the existing `sendSMTPMessage`
- `TestOtpMessageIsUTF8MIME` parses the built message with `net/mail` + `mime`, asserting every header, the RFC 2047 round-trip on the Subject, and the Thai body text
- Verified against a real Mailpit (unit test + `TestEmailOtpLogin`, `TestPhoneOtpViaDevSms`, `TestOtpNeverLogged` integration tests, all passing)

## Task Commits

Executed as a TDD task (`tdd="true"`):

1. **Task 1 RED: add failing test for OTP message UTF-8 MIME headers** - `4f86a9b` (test) — fails to compile (`undefined: otpMessage`), confirming the builder function did not exist yet
2. **Task 1 GREEN: declare UTF-8 MIME headers in OTP SMTP messages** - `037ad5a` (feat) — implements `otpMessage`, refactors `sendSMTPMessage` to use it; all tests pass

No REFACTOR commit — the GREEN implementation needed no further cleanup.

**Plan metadata:** commit created below for SUMMARY.md/STATE.md/ROADMAP.md/REQUIREMENTS.md.

## Files Created/Modified
- `services/identity/internal/adapters/notify/notify.go` — added `otpMessage`, refactored `sendSMTPMessage` to delegate to it, added `mime` import
- `services/identity/internal/adapters/notify/notify_test.go` — added `TestOtpMessageIsUTF8MIME`

## Decisions Made
- 8bit transfer encoding chosen over quoted-printable/base64 since this path only ever talks to Mailpit in dev (D-02); `ResendSender` (production, JSON API) is untouched and out of scope, matching the root-cause diagnosis in `.planning/debug/otp-email-thai-mojibake.md`
- RED phase used a genuine build failure (missing function) as intentional RED, consistent with the project's established greenfield-TDD precedent for a brand-new symbol with no unrelated errors

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Gap G-02-3 is closed at the code level: unit test proves the message format, and all three OTP integration tests pass against a real Mailpit container.
- End-of-phase UAT should re-run the documented human-check (request an OTP at http://localhost:3002/login — actually the web app's port depends on the current compose mapping — open it in Mailpit at http://localhost:8025, confirm the Thai body renders correctly) as part of the phase's consolidated verification pass.
- No blockers for subsequent Phase 02 plans.

---
*Phase: 02-identity-catalog*
*Completed: 2026-09-28*

## Self-Check: PASSED

- FOUND: services/identity/internal/adapters/notify/notify.go
- FOUND: services/identity/internal/adapters/notify/notify_test.go
- FOUND: commit 4f86a9b (test)
- FOUND: commit 037ad5a (feat)
- FOUND: `charset=UTF-8` in notify.go
- FOUND: `WordDecoder` in notify_test.go
