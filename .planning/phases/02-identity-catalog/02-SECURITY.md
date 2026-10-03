---
phase: "2"
slug: "identity-catalog"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-10-03"
---

# Phase 2 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Browser → Kong | Public internet into the gateway; CORS allow-list, rate-limit on `api-auth`, JWT plugin on admin routes | OTP destination/code, httpOnly session cookies |
| Kong → gateway BFF | Gateway strips client `X-*`/Cookie/Authorization and forwards verified claims only | JWT claims (role, operator_id, pier_ids) |
| gateway → identity / catalog | Internal connect-go RPC gated by `X-Internal-Token` (`httpx.RequireInternal`) | Trusted claim headers, catalog/user payloads |
| identity → Valkey / SMTP | OTP state (HMAC-hashed) and outbound mail | Hashed code/destination; plaintext code only in the mail body |
| Browser → SeaweedFS (presigned) | Direct pier-photo upload with signed Content-Type/Length, 10 min expiry | Image bytes (public marketing photos) |
| Services → Kafka | Outbox events | Ids/roles only — no PII |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-02-01-01 | Spoofing/EoP | pkg/httpx claims | high | mitigate | `ForwardClaims` deletes trusted headers before setting verified ones; constant-time `RequireInternal` (`pkg/httpx/claims.go`) | closed |
| T-02-01-02 | Tampering | pkg/auth JWT | high | mitigate | `Verify` pins RS256, issuer, exp required (`pkg/auth/auth.go`) | closed |
| T-02-01-03 | Tampering | pier_ids header | medium | mitigate | `parsePierIDs` uuid.Parse per part, 401 on failure | closed |
| T-02-01-04 | EoP | catalog scope | high | mitigate | `Scope.All()` bypass only for super_admin (`services/catalog/internal/app/scope.go`) | closed |
| T-02-01-SC | Tampering (supply chain) | deps | high | mitigate | Human legitimacy checkpoint approved; pinned versions match go.mod/package.json | closed |
| T-02-02-01 | EoP (OTP brute force) | identity otp | high | mitigate | Atomic `TxPipelined(HGetAll+HIncrBy)` attempt counter (CR-01 fix) | closed |
| T-02-02-02 | Info Disclosure | identity otp | high | mitigate | HMAC-SHA256 with `OTP_HASH_SECRET`; code never in Postgres | closed |
| T-02-02-03 | Info Disclosure | identity logs | high | mitigate | `TestOtpNeverLogged` captures stdout | closed |
| T-02-02-04 | Spoofing | otp generation | high | mitigate | `crypto/rand.Int` in `generateCode` | closed |
| T-02-02-05 | Info Disclosure (enumeration) | RequestOtp | medium | mitigate | No user lookup on request path, uniform result | closed |
| T-02-02-06 | DoS | RequestOtp | medium | mitigate | `SetNX` 60s cooldown + 5/hour cap | closed |
| T-02-02-07 | Info Disclosure | events | high | mitigate | `UserCreated` ids/role only; `proto/pii-check.sh` in `make proto-check` | closed |
| T-02-02-08 | Tampering/Replay | VerifyOtp | high | mitigate | `Del` winner check rejects concurrent reuse | closed |
| T-02-02-09 | Info Disclosure | refresh tokens | high | mitigate | Only sha256 hash stored (`session.go`) | closed |
| T-02-02-10 | DoS (SMS pumping) | notify | low | accept | See Accepted Risks | closed |
| T-02-02-SC | Tampering (supply chain) | go-redis | high | mitigate | v9.22.0 matches approved verdict | closed |
| T-02-03-01 | EoP/IDOR | catalog piers | high | mitigate | `GetPierForUpdateScoped`; NotFound out of scope | closed |
| T-02-03-02 | EoP | createPier | high | mitigate | Requires `scope.All()` | closed |
| T-02-03-03 | Tampering | updatePier | high | mitigate | operator_id taken from stored row | closed |
| T-02-03-04 | DoS (self) | scope | medium | mitigate | Explicit super_admin bypass | closed |
| T-02-03-05 | Info Disclosure | ListPiers | medium | mitigate | Separate `ListPiersPublic` query | closed |
| T-02-03-06 | Info Disclosure | PierUpserted event | low | mitigate | Event carries no address | closed |
| T-02-03-07 | Tampering | pier input | low | mitigate | `domain.Pier.Validate` + DB check constraints | closed |
| T-02-04-01 | Spoofing | gateway proxy | high | mitigate | `Rewrite` deletes Cookie/Authorization, calls `ForwardClaims` (`proxy.go`) | closed |
| T-02-04-02 | EoP | gateway proxy | high | mitigate | Role switch + downstream RPC re-checks | closed |
| T-02-04-03 | Tampering/SSRF | gateway proxy | high | mitigate | `adminMethodPattern` + exact upstream map | closed |
| T-02-04-04 | DoS | gateway proxy | medium | mitigate | 64 KiB `MaxBytesReader` | closed |
| T-02-04-05 | Info Disclosure | public proxy | high | mitigate | Fresh request with internal token only | closed |
| T-02-04-06 | EoP | gateway routing | medium | mitigate | AuthService only via explicit cookie routes | closed |
| T-02-04-07 | Repudiation | tracing | low | mitigate | otelhttp transport propagates traceparent | closed |
| T-02-05-01 | Info Disclosure | session cookies | high | mitigate | HttpOnly/Secure/SameSite=Lax; tokens never in JSON body | closed |
| T-02-05-02 | Spoofing/Replay | Refresh | high | mitigate | Reuse revokes the session family | closed |
| T-02-05-03 | EoP | Refresh | medium | mitigate | Re-reads user row each refresh | closed |
| T-02-05-04 | Tampering (CSRF) | auth routes | medium | mitigate | SameSite=Lax + Kong CORS allow-list + content-type enforcement | closed |
| T-02-05-05 | DoS/brute force | Kong api-auth | medium | mitigate | rate-limiting 20/min (`kong.yml.tmpl`) | closed |
| T-02-05-06 | EoP | super_admin bootstrap | medium | mitigate | `EnsureSuperAdmin` is the sole path | closed |
| T-02-05-07 | Info Disclosure | VerifyOtp responses | low | accept | See Accepted Risks | closed |
| T-02-06-01 | EoP/IDOR | routes | high | mitigate | `GetPierForShareScoped` / `GetRouteForUpdateScoped` | closed |
| T-02-06-02 | Tampering | routes | high | mitigate | operator_id derived from pier_from | closed |
| T-02-06-03 | Info Disclosure | archive error | medium | mitigate | Other operators' routes counted only | closed |
| T-02-06-04 | Tampering | prices | medium | mitigate | `effective_from >= today`; history kept | closed |
| T-02-06-05 | Tampering | cancellation policy | low | mitigate | Typed tiers marshalled, never raw JSON | closed |
| T-02-06-06 | Tampering (money) | prices | medium | mitigate | 0..10,000,000 satang bound, int64 end-to-end | closed |
| T-02-06-07 | DoS | row locks | low | mitigate | Share vs update locks | closed |
| T-02-07-01 | EoP | UserService | high | mitigate | super_admin guard on every method (`users.go`) | closed |
| T-02-07-02 | EoP | user roles | high | mitigate | `Validate` restricts to staff/pier_admin | closed |
| T-02-07-03 | Tampering | user piers | medium | mitigate | Synchronous catalog `ListPiers` ownership check | closed |
| T-02-07-04 | DoS | SetUserDisabled | low | mitigate | Self-disable rejected | closed |
| T-02-07-05 | Info Disclosure | ListUsers | medium | mitigate | No customer rows; events ids/role only | closed |
| T-02-07-06 | Repudiation/stale access | disable | medium | mitigate | `RevokeAllRefreshTokens` in same tx | closed |
| T-02-08-01 | EoP/IDOR | boats | high | mitigate | `GetPierForShareScoped` / `GetBoatForUpdateScoped` | closed |
| T-02-08-02 | Tampering | photo presign | high | mitigate | Signed Content-Type/Length; `photo-roundtrip.sh` proves 403 | closed |
| T-02-08-03 | Tampering | photo key | high | mitigate | Server uuid v7 key + `photoKeyPattern` | closed |
| T-02-08-04 | Info Disclosure | S3 creds | high | mitigate | Env-only, never logged | closed |
| T-02-08-05 | DoS | presign | medium | mitigate | 10 min expiry | closed |
| T-02-08-06 | Info Disclosure | pier photos | low | accept | See Accepted Risks | closed |
| T-02-08-SC | Tampering (supply chain) | minio-go | high | mitigate | v7.3.0 matches approved verdict | closed |
| T-02-09-01 | Info Disclosure | admin api.ts | high | mitigate | No web storage; never reads token | closed |
| T-02-09-02 | EoP | admin UI gating | high | transfer | Enforced by catalog/identity scope checks (re-verified) | closed |
| T-02-09-03 | Tampering (XSS) | admin | medium | mitigate | No `dangerouslySetInnerHTML` | closed |
| T-02-09-04 | Info Disclosure | Kong CORS | high | mitigate | Explicit origins, never `*` | closed |
| T-02-09-05 | Tampering (supply chain) | admin deps | medium | mitigate | Exact pins + lockfile | closed |
| T-02-10-01 | Info Disclosure | web api.ts | high | mitigate | No token read/store; httpOnly cookies | closed |
| T-02-10-02 | Tampering (XSS) | web | medium | mitigate | No `dangerouslySetInnerHTML` | closed |
| T-02-10-03 | DoS | refresh | low | mitigate | Single shared `refreshPromise` | closed |
| T-02-10-04 | Tampering (supply chain) | input-otp | medium | mitigate | 1.5.0 pinned | closed |
| T-02-11-01 | Tampering | photo upload | high | mitigate | `put-wrong-length-403` check | closed |
| T-02-11-02 | Info Disclosure | bucket | medium | mitigate | Anonymous Read only, no List | closed |
| T-02-11-03 | Tampering (CORS) | storage | medium | mitigate | Exact origin asserted in `photo-roundtrip.sh` | closed |
| T-02-11-04 | Info Disclosure | S3 creds | high | mitigate | From `.env` only | closed |
| T-02-11-05 | Info Disclosure | map tiles | low | accept | See Accepted Risks | closed |
| T-02-11-SC | Tampering (supply chain) | maplibre-gl | high | mitigate | 6.11.2 matches approved verdict | closed |
| T-02-12-01 | Tampering (money) | price UI | medium | mitigate | BigInt/string conversion + server bound | closed |
| T-02-12-02 | EoP | routes/boats UI | high | transfer | Catalog NotFound out of scope (re-verified) | closed |
| T-02-12-03 | Tampering (XSS) | catalog pages | medium | mitigate | No `dangerouslySetInnerHTML` | closed |
| T-02-12-04 | Tampering | prices | medium | mitigate | Backend forbids past effective_from | closed |
| T-02-13-01 | EoP | staff UI | high | transfer | identity rejects non-super_admin (re-verified) | closed |
| T-02-13-02 | Info Disclosure | staff page | medium | mitigate | No persistence in browser | closed |
| T-02-13-03 | Tampering (XSS) | staff pages | medium | mitigate | No `dangerouslySetInnerHTML` | closed |
| T-02-14-01 | Info Disclosure | otpMessage | medium | mitigate | Never logged; `TestOtpNeverLogged` | closed |
| T-02-14-02 | Tampering (header injection) | otpMessage | low | accept | See Accepted Risks | closed |
| T-02-15-01 | Info Disclosure | pier.go error | low | accept | See Accepted Risks | closed |
| T-02-15-02 | Tampering (torn write) | boats | medium | mitigate | FOR UPDATE + `TestConcurrentUpsertBoatSameID` | closed |
| T-02-16-01 | Tampering | pier sheet | low | accept | See Accepted Risks | closed |
| T-02-16-02 | DoS (client) | map-picker | medium | mitigate | `parseCoord` range guard before `setLngLat` | closed |
| T-02-16-03 | Info Disclosure | operator select | low | accept | See Accepted Risks | closed |
| T-02-17-01 | Tampering | policy editor | low | accept | See Accepted Risks | closed |
| T-02-17-02 | Info Disclosure | pierName | low | accept | See Accepted Risks | closed |
| T-02-17-03 | DoS | pierName | low | accept | See Accepted Risks | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-02-01 | T-02-02-10 | No real SMS provider in v1; prod builds no SMS sender (`ErrDeliveryUnavailable`) | plan 02-02 | 2026-10-03 |
| AR-02-02 | T-02-05-07 | Uniform VerifyOtp responses; residual timing signal negligible | plan 02-05 | 2026-10-03 |
| AR-02-03 | T-02-08-06 | Pier photos are public marketing assets by design (CAT-06) | plan 02-08 | 2026-10-03 |
| AR-02-04 | T-02-11-05 | OpenFreeMap keyless public tiles (D-20); style URL env-swappable | plan 02-11 | 2026-10-03 |
| AR-02-05 | T-02-14-02 | Subject is a compile-time const, RFC 2047 encoded; destination pre-normalized | plan 02-14 | 2026-10-03 |
| AR-02-06 | T-02-15-01 | "operator is archived" reachable only after `scope.All()` (super_admin) | plan 02-15 | 2026-10-03 |
| AR-02-07 | T-02-16-01 | UI-only change; backend `Pier.Validate` + D-08 archived-operator check unchanged | plan 02-16 | 2026-10-03 |
| AR-02-08 | T-02-16-03 | Archived state shown only to super_admin, who already sees it | plan 02-16 | 2026-10-03 |
| AR-02-09 | T-02-17-01 | UI `?? 0` default; backend `ValidateCancellationPolicy` re-checks every UpsertRoute | plan 02-17 | 2026-10-03 |
| AR-02-10 | T-02-17-02 | Pier id tail already public via `/api/v1/public/piers` | plan 02-17 | 2026-10-03 |
| AR-02-11 | T-02-17-03 | O(n) label pass over a few hundred piers; fine at v1 scale | plan 02-17 | 2026-10-03 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-10-03 | 89 | 89 | 0 | gsd-security-auditor (ASVS L1) |

Prior review findings CR-01 (OTP attempt race) and CR-02 (boat cross-operator operator_id) were fixed and re-verified before this audit (`02-REVIEW-FIX.md`, `02-VERIFICATION.md`). No `## Threat Flags` were raised in any SUMMARY.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-10-03
