---
phase: 02-identity-catalog
plan: 04
subsystem: gateway
tags: [connect-go, reverse-proxy, httputil, otelhttp, trust-boundary, auth-03, cat-06]

# Dependency graph
requires:
  - phase: 02-identity-catalog (plan 02-01)
    provides: pier_ids signed claim, auth.Role* constants, httpx.Claims.PierIDs/HeaderPierIDs, httpx.ForwardClaims
  - phase: 02-identity-catalog (plan 02-03)
    provides: CatalogService.UpsertOperator/ListOperators/ArchiveOperator/UpsertPier/ListPiers RPCs the admin proxy now routes to
provides:
  - "POST /api/v1/admin/{service}/{method}: one allow-listed reverse proxy (boatbooking.catalog.v1.CatalogService, boatbooking.identity.v1.UserService) for every admin RPC — no per-RPC gateway handler needed"
  - "GET /api/v1/public/{boats|piers|routes}: one claim-less reverse proxy for every public catalog list"
  - "pkg/httpx.NewHTTPClient(timeout) — otelhttp-wrapped *http.Client shared by both proxies so traceparent crosses the hop"
affects: [02-06, 02-07, 02-08, 02-11]

# Actuals (#2632)
actuals:
  tokens: 12733
  tasks: 2
  commits: 2
plan_head_before: 81e1bcfb147825dbd0d70c5e6f5eac4cafe55d70

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "adminProxy/publicHandler are pure per-request closures with no shared mutable state — every claim/upstream lookup is a local variable computed fresh per call, so concurrent requests never cross-contaminate (AUTH-03 concurrency invariant, satisfied by construction, not by a lock)"
    - "The admin proxy reads the whole (MaxBytesReader-bounded) body into memory before building the reverse proxy, instead of streaming it through httputil.ReverseProxy — guarantees an oversized body is rejected before any upstream TCP connection, which a streaming body can't guarantee once headers are already in flight"
    - "publicHandler always builds a brand-new outbound *http.Request from scratch (method, URL, body, exactly two headers) — it never clones or copies anything from the inbound request, which is what makes 'no claim can leak into a public read' true by construction instead of by a header-stripping list"

key-files:
  created:
    - services/gateway/internal/adapters/http/proxy.go
    - services/gateway/internal/adapters/http/proxy_test.go
  modified:
    - pkg/httpx/otel.go
    - services/gateway/cmd/main.go
    - services/gateway/go.mod
    - services/gateway/internal/adapters/http/bff.go
    - services/gateway/internal/adapters/http/bff_test.go
    - services/gateway/CLAUDE.md
    - deploy/proof.sh
    - deploy/kong/roundtrip.sh

key-decisions:
  - "Routes takes one *http.Client instead of a typed catalog client + separate transport param — adminProxy derives its http.RoundTripper from client.Transport, publicHandler uses the client directly; one fewer parameter than the plan's literal Task 1 signature, same behavior"
  - "adminProxy buffers the request body upfront (io.ReadAll behind MaxBytesReader) rather than letting httputil.ReverseProxy stream it and catching http.MaxBytesError in ErrorHandler — a streaming body can't guarantee the upstream is never contacted once TLS/TCP handshake and headers are already sent, which the plan's own acceptance criterion requires (oversized body -> upstream never called); the ErrorHandler branch is kept as defensive depth for a Content-Length lie, not the primary enforcement path"
  - "proof.sh/roundtrip.sh now expect HTTP 200 for a successful UpsertBoat, not the old REST handler's 201 — connect unary success is always 200 regardless of the underlying operation being a create"

patterns-established:
  - "New admin/public catalog or identity RPCs need zero gateway code: add the RPC to the proto service, and it's reachable through the existing allow-listed proxy the moment it exists (D-18, D-21)"

requirements-completed: [AUTH-03, CAT-06]

coverage:
  - id: D1
    description: "POST /api/v1/admin/{service}/{method} allow-lists CatalogService/UserService by exact name, 404s any other service or malformed method, requires a valid staff/pier_admin/super_admin access token (401/403 otherwise), rejects non-JSON content-type and bodies over 64 KiB before any upstream contact, and passes the upstream's connect response (status + body) through unchanged"
    requirement: AUTH-03
    verification:
      - kind: unit
        ref: "services/gateway/internal/adapters/http/proxy_test.go#TestAdminProxyForwardsVerifiedClaimsAndStripsSpoofed"
        status: pass
      - kind: unit
        ref: "services/gateway/internal/adapters/http/proxy_test.go#TestAdminProxyRejects"
        status: pass
      - kind: unit
        ref: "services/gateway/internal/adapters/http/proxy_test.go#TestAdminProxyRoutesUserServiceToIdentity"
        status: pass
      - kind: e2e
        ref: "deploy/kong/roundtrip.sh checks admin-proxy-upsert-boat-200, admin-proxy-customer-403 (make kong-roundtrip)"
        status: pass
      - kind: e2e
        ref: "deploy/proof.sh upsert-200 step through Kong -> gateway -> catalog -> outbox -> schedule (make proof)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every client-supplied X-User-Id/X-Operator-Id/X-Role/X-Pier-Ids/X-Internal-Token, Cookie and Authorization is deleted before httpx.ForwardClaims sets verified values on the outbound admin-proxy request (Anti-Pattern 2)"
    requirement: AUTH-03
    verification:
      - kind: unit
        ref: "services/gateway/internal/adapters/http/proxy_test.go#TestAdminProxyForwardsVerifiedClaimsAndStripsSpoofed"
        status: pass
    human_judgment: false
  - id: D3
    description: "GET /api/v1/public/{boats|piers|routes} calls the mapped CatalogService RPC with body {} and only X-Internal-Token — never any claim header or cookie, even when the inbound request carries spoofed ones — and passes the upstream response through byte-for-byte; an unknown resource is 404 without contacting the upstream"
    requirement: CAT-06
    verification:
      - kind: unit
        ref: "services/gateway/internal/adapters/http/proxy_test.go#TestPublicProxyIsClaimLess"
        status: pass
      - kind: unit
        ref: "services/gateway/internal/adapters/http/proxy_test.go#TestPublicProxyUnknownResource404"
        status: pass
      - kind: unit
        ref: "services/gateway/internal/adapters/http/proxy_test.go#TestPublicProxyPassthrough"
        status: pass
      - kind: e2e
        ref: "deploy/kong/roundtrip.sh check public-boats-200 (make kong-roundtrip)"
        status: pass
      - kind: e2e
        ref: "deploy/proof.sh public-list step (make proof)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Legacy POST /api/v1/boats REST route and its typed catalogv1connect client are removed from the gateway; deploy/proof.sh, deploy/kong/roundtrip.sh and bruno/Boat-Booking/boats.bru all work against the new proxy paths, and make kong-roundtrip + make proof stay green"
    verification:
      - kind: e2e
        ref: "make kong-roundtrip (12/12 checks pass)"
        status: pass
      - kind: e2e
        ref: "make proof (all 5 checks pass, including single-trace across gateway/catalog/schedule)"
        status: pass
    human_judgment: false

# Metrics
duration: 21min
completed: 2026-09-26
status: complete
---

# Phase 2 Plan 4: Generic Admin + Public Catalog Proxies Summary

**Two small, allow-listed `httputil.ReverseProxy`-based handlers (`POST /api/v1/admin/{service}/{method}` and `GET /api/v1/public/{resource}`) replace the one-RPC-per-handler BFF pattern, so every current and future catalog/identity RPC is reachable through Kong with zero new gateway code.**

## Performance

- **Duration:** 21 min
- **Started:** 2026-09-26T17:30:54Z
- **Completed:** 2026-09-26T17:51:02Z
- **Tasks:** 2 (1 tracer, 1 auto)
- **Files modified:** 10 (2 created, 8 modified)

## Accomplishments

- `adminProxy` reverse-proxies `POST /api/v1/admin/{service}/{method}` to an exact-match service allow-list (`boatbooking.catalog.v1.CatalogService` -> `CATALOG_URL`, `boatbooking.identity.v1.UserService` -> `IDENTITY_URL`); unknown service or a method failing `^[A-Z][A-Za-z0-9]{0,63}$` is 404, non-JSON content-type or a missing/invalid token is 400/401, a non-staff/pier_admin/super_admin role is 403, and a body over 64 KiB is rejected before any upstream contact
- Every client-supplied trust-boundary header, `Cookie` and `Authorization` is deleted from the outbound request before `httpx.ForwardClaims` sets the caller's verified claims (Anti-Pattern 2) — proven end-to-end with a spoofed `X-Operator-Id`/`X-Role`/`X-Pier-Ids`/`X-Internal-Token`/`Cookie`/`Authorization` all replaced
- `publicHandler` serves `GET /api/v1/public/{boats|piers|routes}` by building a brand-new outbound request from scratch — method, URL, `{}` body, exactly `Content-Type` + `X-Internal-Token` — so a public list can never carry a claim regardless of what the inbound request sent
- `pkg/httpx.NewHTTPClient(timeout)` wraps `http.DefaultTransport` with `otelhttp.NewTransport` so traceparent survives the new proxy hop; `make proof`'s `single-trace` check still sees one trace spanning gateway, catalog and schedule
- Legacy `POST /api/v1/boats`, its typed `catalogv1connect.CatalogServiceClient` wiring, and the now-dead `writeProtoJSON`/protojson dependency are all removed from the gateway; `deploy/proof.sh` and `deploy/kong/roundtrip.sh` exercise the admin-proxy URL instead (expecting connect's `200`, not REST's `201`), plus a new `admin-proxy-customer-403` check

## Task Commits

1. **Task 1 (tracer): Admin RPC proxy end-to-end — curl -> Kong (JWT) -> gateway verify + spoof-strip + ForwardClaims -> catalog UpsertBoat -> 200 JSON** — `9003388` (feat)
2. **Task 2 (auto, tdd): Claim-less public read proxy — GET /api/v1/public/{boats|piers|routes} (D-21)** — `3b7c02c` (feat)

**Plan metadata:** committed separately after this SUMMARY.

## Files Created/Modified

- `services/gateway/internal/adapters/http/proxy.go` — `adminProxy`, `publicHandler`, `publicResourceMethods`
- `services/gateway/internal/adapters/http/proxy_test.go` — admin-proxy and public-proxy test suites, `newFakeUpstream`/`newAdminProxyServer`/`newPublicProxyServer` helpers
- `services/gateway/internal/adapters/http/bff.go` — `Routes` now takes `*http.Client` + `catalogURL`/`identityURL`; keeps only `whoamiHandler`
- `services/gateway/internal/adapters/http/bff_test.go` — whoami tests only; catalog-client test scaffolding moved to `proxy_test.go`
- `services/gateway/cmd/main.go` — parses `CATALOG_URL`/`IDENTITY_URL`, builds one shared `httpx.NewHTTPClient`, no more typed catalog client
- `services/gateway/go.mod` — `google.golang.org/protobuf` demoted to `// indirect` (no longer directly imported)
- `services/gateway/CLAUDE.md` — Sync API and Layout sections document both proxies
- `pkg/httpx/otel.go` — `NewHTTPClient(timeout)`
- `deploy/proof.sh`, `deploy/kong/roundtrip.sh` — admin-proxy URL, `200` not `201`, new `admin-proxy-customer-403` check

## Decisions Made

- `Routes(r, v, client *http.Client, catalogURL, identityURL *url.URL, internalToken string)` — one shared client instead of a typed connect client plus a separate `http.RoundTripper` parameter; `adminProxy` reads `client.Transport`, `publicHandler` uses `client` directly. Simpler signature than the plan's literal Task 1 text, same forwarding behavior, converges to the same shape by Task 2 anyway.
- The admin proxy buffers the (bounded) request body into memory with `io.ReadAll` before ever constructing the `httputil.ReverseProxy`, instead of relying on `http.MaxBytesReader` failing mid-stream inside `ReverseProxy`'s transport write. A streaming body can't guarantee "upstream never contacted" once TCP/TLS handshake and headers are already in flight — buffering upfront makes the plan's own acceptance criterion (oversized body -> upstream never called) true by construction. The `ErrorHandler`'s `*http.MaxBytesError` branch is kept as defense in depth, not the primary enforcement path.
- `deploy/proof.sh`/`deploy/kong/roundtrip.sh` now assert HTTP `200` for a successful `UpsertBoat`, not the old REST handler's `201` — connect unary success is always `200`, independent of the underlying operation.

## Deviations from Plan

None — plan executed as written (see "Decisions Made" above for two structural choices within the plan's stated design, not deviations from its `must_haves`).

## Issues Encountered

- `make proof` transiently returned `429` on the first post-Task-1 run because it ran immediately after `make kong-roundtrip`'s own rate-limit-exhaustion check (Kong's 120/minute local policy). Resolved by polling `/api/v1/whoami` until the per-minute window reset, then re-running `make proof` — not a code defect, an artifact of running both scripts back-to-back in the same terminal session.

## User Setup Required

None — no external service configuration required. `make up` rebuilt the gateway image with the new code twice (once per task) and all services report healthy.

## Next Phase Readiness

- `POST /api/v1/admin/{service}/{method}` is ready for 02-06 (routes/prices RPCs) and 02-07 (identity UserService) to land with zero gateway changes — the moment either proto service adds a new RPC, it is reachable through the existing allow-list.
- `GET /api/v1/public/piers` and `GET /api/v1/public/routes` will start returning real data the moment 02-06 implements `ListRoutes` (piers already work today via 02-03's `ListPiers`); no gateway change needed when that lands.
- `AUTH-03` and `CAT-06` are marked complete in this plan's frontmatter `requirements`.
- No blockers. `go test`/`go build` are green across every workspace module (workspace mode and standalone `GOWORK=off`), `make kong-roundtrip` passes all 12 checks, and `make proof` passes all 5 checks including the cross-service `single-trace` and `logs-correlated` assertions.

---
*Phase: 02-identity-catalog*
*Completed: 2026-09-26*

## Self-Check: PASSED

- Both created files (`services/gateway/internal/adapters/http/proxy.go`, `proxy_test.go`) verified present on disk (`[ -f ]`).
- Both task commits (`9003388`, `3b7c02c`) verified present in `git log --oneline --all`.
- All Task 1 and Task 2 acceptance criteria re-run and confirmed: `proxy.go` contains `httputil.ReverseProxy`, `MaxBytesReader`, `ForwardClaims`; `grep -c "/api/v1/boats\"" bff.go` = 0; `proxy.go` contains the `"boats"`/`"piers"`/`"routes"` map entries; `make kong-roundtrip` prints `PASS admin-proxy-customer-403` and `PASS public-boats-200`; `make proof` prints `PASS single-trace`; `TestAdminProxyForwardsVerifiedClaimsAndStripsSpoofed`, `TestAdminProxyRejects`, `TestPublicProxyIsClaimLess` all print `--- PASS`.
- Plan-level `<verification>` re-run clean: `go test -count=1` green across `pkg/httpx` and `services/gateway`, `go vet` clean, `make kong-roundtrip` all 12 checks pass, `make proof` all 5 checks pass.
