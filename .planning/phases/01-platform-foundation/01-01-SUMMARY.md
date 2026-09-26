---
phase: 01-platform-foundation
plan: 01
subsystem: infra
tags: [kong, jwt, rs256, docker-compose, golangci-lint, lefthook, chi, go-workspace]

requires: []
provides:
  - Go workspace (go.work) with ./pkg and ./services/gateway modules (D-02)
  - pkg/auth: RS256 Issuer/Verifier, access/refresh token kinds, cookie builder
  - pkg/auth/cmd/devtoken: dev CLI (keys, kong, token) for local JWT testing
  - pkg/httpx: MustEnv/EnvOr, WriteError, RequireInternal/RequireClaims/FromContext trust-boundary middleware
  - services/gateway: thin BFF verifying access JWT, GET /api/v1/whoami, ForwardClaims
  - Root Dockerfile (ARG SERVICE, distroless nonroot) + deploy/docker-compose.yml (Kong 3.9.1 DB-less + gateway)
  - deploy/kong/kong.yml.tmpl + roundtrip.sh proving JWT verify/reject, routing, CORS, rate-limiting
  - Makefile targets (dev-keys, up, down, kong-roundtrip, test, test-integration, dev-tools, lint, hooks)
  - .golangci.yml (v2) + lefthook.yml pre-commit baseline
affects: [01-02, 01-03, 01-05, 01-09, 01-10, 01-11, 01-12, 01-13]

actuals:
  tokens: 12335
  tasks: 3
  commits: 6
  plan_head_before: a5e9d47b2d9d4b314b14feb8a6b2ec409eade07a

tech-stack:
  added: [golang-jwt/jwt/v5 v5.3.1, connectrpc.com/connect v1.20.0, go-chi/chi/v5 v5.3.2, golangci-lint v2.14.0, lefthook v2.1.14, goimports v0.50.0, kong:3.9.1]
  patterns:
    - "go.work + one ./pkg module + one module per services/<name>, each with replace ../../pkg (D-02)"
    - "Kong verifies JWT signature/expiry at the edge; BFF re-verifies and remaps claims to trusted X-* headers (Pattern 4, D-29)"
    - "pkg/httpx.RequireInternal constant-time-compares X-Internal-Token before trusting any X-User-Id/X-Operator-Id/X-Role header (Anti-Pattern 2, D-30)"
    - "Issue(claims, now) takes the clock as a parameter — never calls time.Now() internally (D-42 precursor); forbidigo lint-enforces this in app code"
    - "make test/test-integration/lint use go list -m -f '{{.Path}}/...' (module import paths), never ./... — go.work makes ./... from root fail"

key-files:
  created:
    - go.work
    - pkg/go.mod
    - pkg/auth/auth.go
    - pkg/auth/auth_test.go
    - pkg/auth/cmd/devtoken/main.go
    - pkg/httpx/env.go
    - pkg/httpx/errors.go
    - pkg/httpx/claims.go
    - pkg/httpx/claims_test.go
    - services/gateway/go.mod
    - services/gateway/cmd/main.go
    - services/gateway/internal/adapters/http/bff.go
    - services/gateway/internal/adapters/http/bff_test.go
    - Dockerfile
    - .dockerignore
    - deploy/docker-compose.yml
    - deploy/kong/kong.yml.tmpl
    - deploy/kong/roundtrip.sh
    - Makefile
    - .env.example
    - .gitignore
    - .golangci.yml
    - lefthook.yml
  modified: []

key-decisions:
  - "Kong 3.9.1 DB-less passed all D-28 spike criteria on the first attempt — no Traefik fallback needed"
  - "golang:1.25.14-bookworm used for the Dockerfile build stage instead of the plan's illustrative golang:1.25.9 (that patch tag does not exist on Docker Hub; 1.25.14 is the current 1.25.x patch)"
  - "go.work go directive set to 1.25.1 (matching the installed toolchain) rather than the plan's illustrative 1.25.0"

requirements-completed: [PLAT-10, PLAT-02, PLAT-03]

coverage:
  - id: D1
    description: "Kong 3.9.1 DB-less JWT edge spike: reject-bad-token (foreign key, expired, wrong kind), routing, CORS preflight, rate-limiting 429"
    requirement: "PLAT-10"
    verification:
      - kind: integration
        ref: "deploy/kong/roundtrip.sh (make kong-roundtrip)"
        status: pass
    human_judgment: false
  - id: D2
    description: "pkg/auth RS256 Issuer/Verifier: round-trip, TTLs (900s/2592000s), foreign-key/alg-confusion/expired/wrong-issuer/wrong-kind rejection, cookie attributes"
    requirement: "PLAT-02"
    verification:
      - kind: unit
        ref: "pkg/auth/auth_test.go (go test ./pkg/auth/...)"
        status: pass
    human_judgment: false
  - id: D3
    description: "pkg/httpx trust-boundary middleware: RequireInternal constant-time token check + claim propagation, RequireClaims gate, FromContext/WithClaims"
    requirement: "PLAT-02"
    verification:
      - kind: unit
        ref: "pkg/httpx/claims_test.go (go test ./pkg/httpx/...)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Gateway BFF: GET /api/v1/whoami verifies access cookie/bearer token; ForwardClaims deletes then sets trusted headers, closing the header-spoof vector"
    requirement: "PLAT-02"
    verification:
      - kind: unit
        ref: "services/gateway/internal/adapters/http/bff_test.go (go test ./services/gateway/...)"
        status: pass
      - kind: integration
        ref: "deploy/kong/roundtrip.sh valid-token-200 check"
        status: pass
    human_judgment: false
  - id: D5
    description: "Root Dockerfile (distroless nonroot) + docker-compose.yml: Kong pinned 3.9.1, gateway publishes no host port, private key never reaches gateway env"
    requirement: "PLAT-03"
    verification:
      - kind: other
        ref: "docker compose --profile app config --format json | jq (private key absent, no gateway ports, kong:3.9.1)"
        status: pass
    human_judgment: false
  - id: D6
    description: "golangci-lint v2 + lefthook baseline: forbidigo bans time.Now/fmt.Print*/kgo.NewClient/pgxpool.New in app code, proven to actually fire"
    requirement: "PLAT-03"
    verification:
      - kind: other
        ref: "make lint (0 issues) + deliberate time.Now() probe (make lint fails, forbidigo confirmed via isolated probe, then reverted)"
        status: pass
    human_judgment: false

duration: 40min
completed: 2026-09-26
status: complete
---

# Phase 1 Plan 1: Platform Foundation Summary

**Kong 3.9.1 DB-less RS256 JWT edge spike passes all criteria on the first attempt (no Traefik fallback), plus the repo spine: go.work, pkg/auth, pkg/httpx trust boundary, gateway BFF, root Dockerfile/compose, and a golangci-lint v2 + lefthook baseline that's proven to actually fire.**

## Performance

- **Duration:** ~40 min
- **Started:** 2026-09-26 (approx, first file write)
- **Completed:** 2026-09-26T08:46:56+07:00
- **Tasks:** 3 (1 tracer + 1 TDD + 1 auto)
- **Files created:** 23

## Accomplishments

- Kong 3.9.1 DB-less edge spike (D-28) passes all 8 acceptance checks via a re-runnable script: no-token/foreign-key/expired/refresh-kind all 401, valid token 200 with claims, CORS preflight, rate-limit header present, 429 after 120 requests/min
- `pkg/auth`: RS256 Issuer/Verifier with access (15 min) and refresh (30 day) token kinds, alg-confusion/wrong-issuer/wrong-kind/foreign-key rejection all unit-tested
- `pkg/httpx`: trust-boundary middleware (`RequireInternal`, `RequireClaims`, `FromContext`) enforcing that only the gateway's `X-Internal-Token` unlocks trusted claim headers (D-30, Anti-Pattern 2)
- Gateway BFF: `GET /api/v1/whoami` re-verifies the access JWT from Kong; `ForwardClaims` deletes any inbound spoofed claim headers before setting verified ones
- Root Dockerfile (distroless nonroot, `-healthcheck` subcommand) + `docker-compose.yml`: Kong pinned `3.9.1`, gateway publishes no host port and never receives the private key
- `.golangci.yml` (v2 schema) + `lefthook.yml`: forbidigo bans `time.Now`/`fmt.Print*`/`kgo.NewClient`/`pgxpool.New` in app code — confirmed to actually fire, not just configured

## Task Commits

Each task was committed atomically (Task 1 split into 3 scoped commits per D-49; Task 2 followed the TDD RED→GREEN pattern):

1. **Task 1a (pkg scope):** `6fed397` (feat) — RS256 auth issuer/verifier + devtoken CLI + httpx env/error helpers
2. **Task 1b (gateway scope):** `b610934` (feat) — thin BFF re-verifying access JWT
3. **Task 1c (deploy scope):** `ef751f1` (chore) — Kong 3.9.1 DB-less edge spike + root Dockerfile + Makefile
4. **Task 2 RED:** `ec1ff9e` (test) — trust-boundary tests for auth edge cases and claim headers
5. **Task 2 GREEN:** `f46f186` (feat) — RequireInternal/RequireClaims + BFF ForwardClaims
6. **Task 3:** `933c27c` (chore) — golangci-lint v2 baseline + lefthook pre-commit gate

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP/REQUIREMENTS update)

## Files Created/Modified

- `go.work` — workspace: `./pkg`, `./services/gateway` (D-02)
- `pkg/auth/auth.go` — RS256 Issuer/Verifier, `Claims`, `Cookie()`, PEM key parsing
- `pkg/auth/auth_test.go` — round-trip, TTL, and all rejection-path unit tests
- `pkg/auth/cmd/devtoken/main.go` — `keys`/`kong`/`token` dev CLI subcommands
- `pkg/httpx/env.go` — `MustEnv`/`EnvOr`
- `pkg/httpx/errors.go` — `WriteError` mapping connect codes to HTTP status + JSON body
- `pkg/httpx/claims.go` — `RequireInternal`/`RequireClaims`/`FromContext`/`WithClaims`
- `pkg/httpx/claims_test.go` — middleware unit tests
- `services/gateway/cmd/main.go` — chi router, healthz/readyz, `-healthcheck`, graceful shutdown
- `services/gateway/internal/adapters/http/bff.go` — `Routes`, `whoamiHandler`, `ForwardClaims`
- `services/gateway/internal/adapters/http/bff_test.go` — `ForwardClaims` unit tests
- `Dockerfile` — multi-stage `ARG SERVICE` build, distroless nonroot final stage
- `.dockerignore`
- `deploy/docker-compose.yml` — `gateway` + `kong` services, `app`/`web` profiles
- `deploy/kong/kong.yml.tmpl` — JWT RS256 consumer, `/api` + `/api/v1/public` routes, cors + rate-limiting
- `deploy/kong/roundtrip.sh` — the 8-check acceptance script
- `Makefile` — `dev-keys`, `up`, `down`, `kong-roundtrip`, `test`, `test-integration`, `dev-tools`, `lint`, `hooks`
- `.env.example`, `.gitignore`
- `.golangci.yml` — v2 schema, forbidigo bans
- `lefthook.yml` — pre-commit goimports/buf-lint/eslint (glob-gated, no-op until matching files exist)

## Decisions Made

- Kong 3.9.1 DB-less passed the spike on the first attempt (all 8 roundtrip checks green) — the Traefik fallback (D-28) was not needed.
- Dockerfile build-stage base image: `golang:1.25.14-bookworm` instead of the plan's illustrative `golang:1.25.9` — that exact patch tag does not exist on Docker Hub (verified against the registry; 1.25.14 is the current 1.25.x patch as of this session). Deviation Rule 3 (blocking issue — referenced image tag doesn't exist).
- `go.work`'s `go` directive set to `1.25.1` (the installed toolchain, matching every `go.mod`) rather than the plan's illustrative `1.25.0`. Deviation Rule 1 (minor correctness fix — consistency across all Go version directives).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `golang:1.25.9` Docker base image tag does not exist**
- **Found during:** Task 1 (root Dockerfile)
- **Issue:** The plan's illustrative Dockerfile example pins `golang:1.25.9` for the build stage; that exact patch tag was never published to Docker Hub (verified via the registry API — available 1.25.x tags start at `1.25.14`... down through earlier patches, `1.25.9` is not among them).
- **Fix:** Pinned `golang:1.25.14-bookworm` instead (current 1.25.x patch, Debian-based to match the `distroless/static-debian12` final stage).
- **Files modified:** `Dockerfile`
- **Verification:** `make up` builds the gateway image successfully; container reports healthy.
- **Committed in:** `ef751f1` (Task 1c commit)

**2. [Rule 1 - Bug] Two test-authoring bugs found while writing `pkg/auth/auth_test.go`**
- **Found during:** Task 2 (RED phase — running the newly-written tests against the already-implemented `auth.go`)
- **Issue:** (a) `TestIssueVerifyRoundTrip` and several rejection tests used a hardcoded historical `fixedNow` (2026-01-01), which made the issued token already expired by the time `Verify` ran with the real wall clock, causing round-trip to fail and making the wrong-issuer/wrong-kind tests pass for the wrong reason (coincidental expiry, not the intended rejection cause). (b) `TestCookieAttributes` asserted `SameSite != 3` for Lax, but `http.SameSiteLaxMode` is actually `2` (`SameSiteDefaultMode=1, SameSiteLaxMode=2, SameSiteStrictMode=3`) — a copy-paste constant error in the test, not a bug in `auth.go`.
- **Fix:** Replaced the historical `fixedNow` with a per-test `time.Now()` capture (still deterministic within a single test run, but valid relative to the real verification clock); replaced the magic-number `SameSite` comparison with `http.SameSiteLaxMode`.
- **Files modified:** `pkg/auth/auth_test.go`
- **Verification:** All 9 tests in `pkg/auth` pass; `auth.go` itself required no changes — Task 1's implementation was correct throughout.
- **Committed in:** `ec1ff9e` (Task 2 RED commit, fixed before commit)

**3. [Rule 2 - Missing Critical] gosec G304 findings on devtoken's file-path opens**
- **Found during:** Task 3 (`make lint` against Tasks 1-2 code, per the plan's explicit instruction to fix rather than exclude)
- **Issue:** `gosec` flagged `os.Open(path)` and `os.OpenFile(tmp, ...)` in `pkg/auth/cmd/devtoken/main.go` as G304 (potential file inclusion via variable) — both operate on local dev-CLI paths (`.env`, `.env.example`, template in/out flags), not untrusted network input.
- **Fix:** Added `//nolint:gosec` with an inline justification comment on each call site rather than excluding the whole file/linter in `.golangci.yml`, per the plan's explicit instruction.
- **Files modified:** `pkg/auth/cmd/devtoken/main.go`
- **Verification:** `make lint` exits 0 with 0 issues across both modules.
- **Committed in:** `933c27c` (Task 3 commit)

---

**Total deviations:** 3 auto-fixed (1 blocking image-tag fix, 1 test-authoring bug fix, 1 missing-critical lint suppression with justification).
**Impact on plan:** All three were necessary for correctness (build succeeds, tests test what they claim to, lint gate is real not decorative). No scope creep — no new features or architecture beyond what the plan specified.

## Issues Encountered

None beyond the deviations above — the Kong spike, the trust-boundary tests, and the lint baseline all worked as designed once the three fixes above were applied.

## User Setup Required

None — no external service configuration required. `make dev-keys` generates the local dev RSA keypair and `INTERNAL_TOKEN` into git-ignored `.env` automatically.

## Next Phase Readiness

- The gateway choice is settled: **Kong 3.9.1 DB-less**, proven end-to-end. Every later plan in this phase can build on it without revisiting the D-28 decision.
- `pkg/auth`, `pkg/httpx` (trust boundary half), the gateway BFF, root Dockerfile, dev compose file, Makefile, and lint/pre-commit baseline are all in place for plan 01-02 onward (proto/event envelope, `pkg/kafka`, `pkg/outbox`, `services/_template`, etc.).
- `make lint` is a real gate from this point forward — the `forbidigo` bans on `time.Now`/`kgo.NewClient`/`pgxpool.New` will fire the moment any later plan's code violates the D-02/D-42 ownership boundaries, which is the intended effect.
- No blockers for 01-02.

## Self-Check: PASSED

- All 23 created files verified present on disk (`[ -f ... ]` for each `key-files.created` entry).
- All 6 task commit hashes verified present in `git log --oneline --all`.
- Plan-level `<verification>` re-run clean at the end of this plan: `make up && make kong-roundtrip` (all 8 PASS), `make test` (3 packages ok), `make lint` (0 issues, both modules), compose config jq assertion (`true`).
- `docker compose ... down` run afterward — no containers left running.

---
*Phase: 01-platform-foundation*
*Completed: 2026-09-26*
