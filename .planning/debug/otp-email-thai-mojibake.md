---
status: diagnosed
trigger: "OTP email body shows Thai correctly in Mailpit (UTF-8), not mojibake -- Test 3 UAT gap G-02-6. Subject renders Thai fine but text body Thai is mojibake ('à¸£à¸«à¸±à¸ª...'); English/code fine."
created: 2026-09-28T14:40:00Z
updated: 2026-09-28T14:55:00Z
---

## Current Focus

hypothesis: CONFIRMED — sendSMTPMessage() in services/identity/internal/adapters/notify/notify.go builds the raw RFC 5322 message with no MIME-Version, no Content-Type (so body defaults to a non-UTF-8 charset per RFC 2045), and no Content-Transfer-Encoding. Raw UTF-8 Thai bytes go straight into the body with no charset declaration; Mailpit falls back to a non-UTF-8 decode for the undeclared-charset body (matches classic "UTF-8 bytes read as Latin-1" mojibake), while its Subject-field JSON decoding path happens to render the raw (non-RFC2047-encoded) UTF-8 header bytes correctly regardless.
test: Fetched raw source of a captured OTP message via Mailpit API (GET /api/v1/message/{id}/raw) and confirmed headers are exactly: Message-ID, Return-Path, Received, From, To, Subject — no MIME-Version, no Content-Type, no Content-Transfer-Encoding. Body is raw UTF-8 Thai text with no encoding applied.
expecting: N/A -- root cause confirmed, goal is find_root_cause_only.
next_action: none (return ROOT CAUSE FOUND to caller)

## Symptoms

expected: OTP email body แสดงภาษาไทยถูกต้องใน Mailpit (UTF-8), ไม่ใช่ mojibake (UAT Test 3, gap G-02-6)
actual: Subject renders Thai correctly ("รหัสเข้าสู่ระบบ Boat Booking / Your Boat Booking login code"); text body Thai renders as mojibake, e.g. "à¸£à¸«à¸±à¸ª... / Your code: 082196" and "... 5 à¸™à¸²à¸—à¸µ / expires in 5 minutes." English text and the numeric code are unaffected.
errors: none
reproduction: Trigger any OTP request (email or dev-SMS-to-Mailpit path) via services/identity; open the captured message in Mailpit UI or via GET /api/v1/message/{id}/raw.
started: discovered during Phase 02 UAT (Test 3)

## Eliminated

(none — first hypothesis, from the caller's hint, confirmed directly on first check)

## Evidence

- timestamp: 2026-09-28T14:52:00Z
  checked: Mailpit API GET /api/v1/messages (list) for captured OTP emails
  found: JSON "Subject" field shows correct Thai ("รหัสเข้าสู่ระบบ Boat Booking / ..."); JSON "Snippet" field (body preview) shows mojibake matching the user report exactly: "à¸£à¸«à¸±à¸ªà¸‚à¸­à¸‡à¸„à¸¸à¸“ / Your code: 795051 à¸«à¸¡à¸”à¸­à¸²à¸¢à¸¸à¹ƒà¸™ 5 à¸™à¸²à¸—à¸µ / expires in 5 minutes." All affected messages are Size: 598 B (matches user report).
  implication: Confirms the split — subject decode path in Mailpit differs from body decode path; body is being decoded with the wrong charset.

- timestamp: 2026-09-28T14:53:00Z
  checked: Mailpit API GET /api/v1/message/{id}/raw (raw SMTP DATA payload as received by Mailpit)
  found: |
    Message-ID: <3IjvnJkj1mBDrsfyBXB5p1@mailpit>
    Return-Path: <no-reply@boatbooking.local>
    Received: from localhost ...
    From: Boat Booking <no-reply@boatbooking.local>
    To: admin@boatbooking.local
    Subject: รหัสเข้าสู่ระบบ Boat Booking / Your Boat Booking login code

    รหัสของคุณ / Your code: 795051
    หมดอายุใน 5 นาที / expires in 5 minutes.
  implication: No MIME-Version header, no Content-Type header, no Content-Transfer-Encoding header. Both Subject and body are literal raw UTF-8 bytes with zero MIME/charset declaration whatsoever -- this exactly matches services/identity/internal/adapters/notify/notify.go's sendSMTPMessage(), which builds the message as `fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", from, to, subject, body)` with no MIME headers at all, then hands the raw bytes to net/smtp.SendMail. Per RFC 2045 an SMTP body with no declared Content-Type defaults to text/plain; charset=us-ascii; Mailpit's lenient body-rendering path evidently falls back to a non-UTF-8 (Latin-1-style) byte-for-byte decode for that undeclared/default charset, producing the exact "UTF-8 bytes read as Latin-1" mojibake pattern seen (e.g. UTF-8 for "ร" = 0xE0 0xB8 0xA3 → read as three Latin-1 codepoints à¸£). Mailpit's Subject-field extraction (used for the JSON API / list view) apparently normalizes/display-decodes header text as UTF-8 regardless of the (missing, and technically non-compliant per RFC 2047 for non-ASCII) encoded-word declaration, which is why the subject happens to look right despite being an equally-undeclared raw byte sequence -- a lenient special-case for header display, not evidence the header format is actually correct.
  implication: Root cause fully localized to sendSMTPMessage() in notify.go — missing MIME-Version: 1.0, Content-Type: text/plain; charset=UTF-8, and Content-Transfer-Encoding headers on the constructed message.

- timestamp: 2026-09-28T14:54:00Z
  checked: services/identity/internal/adapters/notify/notify.go — SMTPSender.SendOtp and DevSMSSender.SendOtp
  found: Both SMTPSender.SendOtp (real email path) and DevSMSSender.SendOtp (dev SMS-to-Mailpit path, addr = "<E.164 digits>@sms.local") call the same shared sendSMTPMessage(addr, from, to, subject, code) helper. Only one code path builds the raw MIME message for both.
  implication: The bug is not email-specific — the dev SMS→Mailpit path (services/identity/internal/adapters/notify/notify.go, DevSMSSender) shares the exact same missing-charset defect and would show the same mojibake for Thai SMS OTP bodies once Thai text is used there too (currently that path's messages are ASCII-only templates, so it isn't yet visibly triggering, but the fix must land in the shared function, not per-caller).
  implication: ResendSender.SendOtp (production email path) is a separate code path — it sends via a JSON POST to the Resend API (`Text` field in a `json.Marshal`'d struct), which is UTF-8 by the JSON spec and carries no raw-SMTP-header risk; it is not affected by this defect and is out of scope for the fix.

## Resolution

root_cause: "services/identity/internal/adapters/notify/notify.go: sendSMTPMessage() constructs the raw SMTP message string with fmt.Sprintf and no MIME headers at all -- missing 'MIME-Version: 1.0', 'Content-Type: text/plain; charset=UTF-8', and 'Content-Transfer-Encoding' (8bit or quoted-printable/base64). The literal UTF-8 Thai bytes are sent as the body with no charset declared, so the receiving client (Mailpit) falls back to decoding the body with a non-UTF-8 (effectively Latin-1-style) charset per RFC 2045's default, producing byte-for-byte mojibake for every non-ASCII (Thai) character while ASCII text/digits pass through unaffected. The Subject header has the identical defect (raw UTF-8 bytes, not RFC 2047 encoded-word encoded) but happens to render correctly because Mailpit's header-display path decodes raw non-ASCII header bytes as UTF-8 leniently -- that is a Mailpit rendering quirk, not evidence the header is spec-compliant, and should not be relied on."
fix: ""
verification: ""
files_changed: []
