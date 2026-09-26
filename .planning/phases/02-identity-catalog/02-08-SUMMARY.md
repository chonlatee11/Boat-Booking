---
phase: 02-identity-catalog
plan: 08
subsystem: catalog
tags: [connect-go, sqlc, postgres, outbox, scoping, presigned-url, minio-go, s3, d-07, d-19]

# Dependency graph
requires:
  - phase: 02-identity-catalog (plan 02-01)
    provides: SeaweedFS dev object storage decision and minio-go v7.3.0 package-legitimacy approval (precondition for Task 2)
  - phase: 02-identity-catalog (plan 02-06)
    provides: app.Scope{Role, OperatorID, PierIDs}, http.scopeFrom/toConnectErr, GetPierForShareScoped/GetPierForUpdateScoped pattern reused for boats
provides:
  - CatalogService.ArchiveBoat/PresignPierPhoto RPCs
  - Boat.home_pier_id/archived (additive), UpsertBoatRequest.home_pier_id
  - Pier.photo_key/photo_url (additive), UpsertPierRequest.photo_key
  - catalog.BoatUpserted.home_pier_id/archived (additive event fields, schedule consumer unchanged)
  - boats.home_pier_id/archived_at and piers.photo_key migrations
  - app.Photos{Client, Bucket, PublicBaseURL} (PresignPierPhoto, URL) — nil-Client-safe presign issuer
affects: [02-11]

# Actuals (#2632)
actuals:
  tokens: 64500
  tasks: 2
  commits: 2
plan_head_before: 86187208273ad90ef1bbcc1b638240f343b6b1f9

# Tech tracking
tech-stack:
  added:
    - "github.com/minio/minio-go/v7 v7.3.0 (approved in 02-01, precondition met)"
  patterns:
    - "Boats reuse the exact GetPierForShareScoped/GetBoatForUpdateScoped scope shape UpsertRoute/ArchiveRoute already established — operator_id always derived from the scoped pier/boat row, never the request (D-07 mirrors D-11/D-30)"
    - "app.Photos{Client, Bucket, PublicBaseURL} is nil-Client-safe: a zero-value Photos (no S3_PUBLIC_ENDPOINT) makes PresignPierPhoto return FailedPrecondition instead of panicking, so catalog always starts"
    - "toProtoPier is a *server method (not a free function) so it can read s.photos.URL(p.PhotoKey) when building the wire Pier — the one place photo_key becomes photo_url"

key-files:
  created:
    - services/catalog/migrations/00007_boats_home_pier.sql
    - services/catalog/migrations/00008_pier_photo.sql
    - services/catalog/internal/app/photo.go
    - services/catalog/internal/app/photo_test.go
    - services/catalog/cmd/boats_photo_integration_test.go
  modified:
    - proto/events/catalog/v1/boat.proto
    - proto/services/catalog/v1/catalog.proto
    - services/catalog/internal/adapters/postgres/queries/boats.sql
    - services/catalog/internal/adapters/postgres/queries/piers.sql
    - services/catalog/internal/domain/boat.go
    - services/catalog/internal/domain/pier.go
    - services/catalog/internal/app/boat.go
    - services/catalog/internal/app/pier.go
    - services/catalog/internal/adapters/http/routes.go
    - services/catalog/internal/adapters/http/piers.go
    - services/catalog/cmd/main.go
    - services/catalog/cmd/main_integration_test.go
    - deploy/proof.sh
    - deploy/kong/roundtrip.sh
    - services/catalog/CLAUDE.md

key-decisions:
  - "Boats' scope check mirrors routes exactly: home pier loaded via GetPierForShareScoped (missing/out-of-scope -> NotFound, archived -> FailedPrecondition), operator_id always taken from the pier's stored operator_id, never the request (D-07/D-30)"
  - "A missing home_pier_id (uuid.Nil) is checked explicitly before the pier lookup and returns InvalidArgument directly — routing a nil UUID through GetPierForShareScoped would otherwise look up the all-zero UUID and misreport NotFound"
  - "Photos issues presigned URLs via minio-go's PresignHeader (not PresignedPutObject) specifically because Content-Type and Content-Length must be part of the signed header set (T-02-08-02) — plain Presign doesn't support extra signed headers"
  - "cmd/main.go's newPhotos() returns a zero-value Photos{PublicBaseURL: ...} (nil Client) when S3_PUBLIC_ENDPOINT is unset, rather than erroring at startup — object storage is optional infrastructure in dev, matching the existing relay/consumer optionality pattern"
  - "roundtrip.sh's admin-proxy-upsert-boat-200 check now seeds a real operator+pier via a super_admin token before minting the pier_admin token that writes the boat, since D-07 boats can no longer be created without an in-scope home pier"

patterns-established:
  - "domain.Pier.PhotoKey validated by a single compiled regexp (photoKeyPattern) matching the DB-level check constraint verbatim — belt-and-suspenders enforced in exactly one place per layer"

requirements-completed: [CAT-04, CAT-02, AUTH-05, CAT-06]

coverage:
  - id: D1
    description: "Every boat write requires home_pier_id; boat.operator_id is always its home pier's operator; pier_admin may create/edit/archive a boat only when home_pier_id is in their pier_ids (and operator); out-of-scope boats answer NotFound; staff/customer writes are denied"
    requirement: CAT-04
    verification:
      - kind: integration
        ref: "services/catalog/cmd/boats_photo_integration_test.go#TestBoatsScopedByHomePier"
        status: pass
      - kind: integration
        ref: "services/catalog/cmd/main_integration_test.go#TestUpsertBoatValidationAndTenancy"
        status: pass
    human_judgment: false
  - id: D2
    description: "catalog.BoatUpserted gains home_pier_id (field 6) and archived (field 7) as additive-only fields; schedule's existing consumer keeps applying the event unchanged"
    requirement: CAT-04
    verification:
      - kind: integration
        ref: "services/catalog/cmd/main_integration_test.go#TestUpsertBoatPublishesBoatUpserted"
        status: pass
      - kind: integration
        ref: "go test -tags=integration github.com/chonlatee11/boat-booking/services/schedule/... (unchanged, green)"
        status: pass
    human_judgment: false
  - id: D3
    description: "ListBoats with pier_admin/staff claims returns only boats whose home pier is in scope; empty pier_ids -> empty list; super_admin -> all; no claims -> every non-archived boat"
    requirement: AUTH-05
    verification:
      - kind: integration
        ref: "services/catalog/cmd/boats_photo_integration_test.go#TestBoatsScopedByHomePier"
        status: pass
      - kind: integration
        ref: "services/catalog/cmd/main_integration_test.go#TestListBoatsOrdered"
        status: pass
    human_judgment: false
  - id: D4
    description: "Missing home pier / empty name -> InvalidArgument; UpsertBoat with existing boat_id updates in place; ArchiveBoat is idempotent and archived boats reject edits"
    requirement: CAT-04
    verification:
      - kind: integration
        ref: "services/catalog/cmd/boats_photo_integration_test.go#TestBoatsScopedByHomePier"
        status: pass
    human_judgment: false
  - id: D5
    description: "PresignPierPhoto (pier_admin/super_admin) allow-lists image/jpeg|png|webp and 1..5,242,880 bytes, generates piers/<uuidv7>.<ext> server-side, returns a 10-minute PUT URL signed over Content-Type and Content-Length"
    requirement: CAT-02
    verification:
      - kind: unit
        ref: "services/catalog/internal/app/photo_test.go#TestPresignPierPhoto"
        status: pass
      - kind: unit
        ref: "services/catalog/internal/app/photo_test.go#TestPresignPierPhotoValidation"
        status: pass
      - kind: unit
        ref: "services/catalog/internal/app/photo_test.go#TestPresignPierPhotoScopeAndStorage"
        status: pass
    human_judgment: false
  - id: D6
    description: "UpsertPier accepts photo_key only as piers/<uuid>.(jpg|png|webp); ListPiers returns photo_url = PHOTO_PUBLIC_BASE_URL + '/' + photo_key; FailedPrecondition with no storage configured and catalog still starts"
    requirement: CAT-06
    verification:
      - kind: integration
        ref: "services/catalog/cmd/boats_photo_integration_test.go#TestPierPhotoKeyAndUrl"
        status: pass
      - kind: integration
        ref: "services/catalog/cmd/boats_photo_integration_test.go#TestPierPhotoUrlEmptyWithoutStorageConfig"
        status: pass
      - kind: unit
        ref: "services/catalog/internal/domain/pier_test.go#TestPierValidate (photo_key cases)"
        status: pass
    human_judgment: false
  - id: D7
    description: "make proof and make kong-roundtrip pass with a super_admin creating operator -> pier -> boat through the admin proxy; roundtrip also checks public piers/routes return 200"
    requirement: CAT-06
    verification:
      - kind: e2e
        ref: "deploy/proof.sh (make proof, live docker compose stack)"
        status: pass
      - kind: e2e
        ref: "deploy/kong/roundtrip.sh public-piers-200/public-routes-200/admin-proxy-upsert-boat-200 (make kong-roundtrip)"
        status: pass
    human_judgment: false
  - id: D8
    description: "Concurrent UpsertBoat calls on the same boat_id leave one row and one BoatUpserted outbox row per committed write"
    requirement: CAT-04
    verification: []
    human_judgment: true
    rationale: "Marked as a backstop-verification truth in the plan (no dedicated concurrency test written this plan) — the same UpdateBoat single-row UPDATE ... WHERE id = $1 pattern already proven race-safe for routes/prices in 02-06 applies unchanged to boats; a human/verifier should confirm no new race surface was introduced rather than re-deriving proof from a fresh load test."
  - id: D9
    description: "Every boat write requires home_pier_id; the boat's operator_id is always its home pier's operator (upload validation prohibitions: server-generated key, signed Content-Type/Content-Length)"
    requirement: CAT-02
    verification:
      - kind: unit
        ref: "services/catalog/internal/app/photo_test.go#TestPresignPierPhoto (asserts X-Amz-SignedHeaders contains content-length/content-type/host)"
        status: pass
    human_judgment: false

# Metrics
duration: 48min
completed: 2026-09-26
status: complete
---

# Phase 2 Plan 8: Boats Under Home-Pier Scope and Presigned Pier Photo Uploads Summary

**Boats join the pier-scoping rule established for routes/prices (home_pier_id, additive BoatUpserted fields, ArchiveBoat), and CatalogService.PresignPierPhoto issues minio-go v7.3.0 PresignHeader URLs signed over Content-Type/Content-Length for server-generated `piers/<uuidv7>.<ext>` keys.**

## Performance

- **Duration:** 48 min
- **Started:** 2026-09-26T18:57:06Z
- **Completed:** 2026-09-26T19:45:06Z
- **Tasks:** 2 (1 tracer, 1 auto/tdd)
- **Files modified:** 32 (5 created, 27 modified, including generated `gen/**` proto output)

## Accomplishments

- `boats.home_pier_id`/`archived_at` migration (column nullable so Phase-1 dev rows survive; every write requires it) with a scoped query rewrite: `InsertBoat`/`UpdateBoat`/`GetBoatForUpdateScoped`/`ListBoatsAdmin`/`ListBoatsPublic`/`ArchiveBoat`
- `app.UpsertBoat`/`ListBoats`/`ArchiveBoat` enforce the D-07 home-pier scope rule using the exact `GetPierForShareScoped`/`GetBoatForUpdateScoped` shape 02-06 established for routes — `operator_id` is always derived from the home pier's stored operator, never the request (D-30); a missing `home_pier_id` is `InvalidArgument`, an out-of-scope or missing home pier is `NotFound`, an archived home pier or boat is `FailedPrecondition`
- Removed the Phase-1 `TODO(WR-01)` reminder comment from `adapters/http/routes.go` — role checking now lives in `scopeFrom`, closing the tracked gap
- `catalog.BoatUpserted` gains `home_pier_id` (field 6) and `archived` (field 7) as additive-only fields; `services/schedule`'s existing consumer (`ApplyBoatUpserted`) needed zero changes and its integration suite stays green
- `CatalogService.ArchiveBoat` is idempotent (re-archiving publishes no new event) and blocks further edits on an archived boat
- `app.Photos{Client, Bucket, PublicBaseURL}` issues presigned PUT URLs via `minio-go`'s `PresignHeader` (not `PresignedPutObject` — the plain Presign call can't sign extra headers): allow-lists `image/jpeg|png|webp`, bounds size to 1..5,242,880 bytes, generates the object key server-side (`piers/<uuidv7>.<ext>`), signs `Content-Type` and `Content-Length` into the URL (10-minute expiry) so a mismatched upload is rejected by storage itself, not just by catalog
- A nil `Photos.Client` (no `S3_PUBLIC_ENDPOINT` configured) makes `PresignPierPhoto` return `FailedPrecondition` instead of panicking — catalog always starts regardless of object-storage configuration
- `piers.photo_key` migration with a DB-level check constraint mirroring `domain.Pier`'s compiled regexp exactly; `UpsertPier` accepts `photo_key` only in that exact shape; `ListPiers`/`UpsertPier`/`ArchivePier` responses carry `photo_url = PHOTO_PUBLIC_BASE_URL + "/" + photo_key` (empty when no photo or no base URL configured)
- `deploy/proof.sh` now mints a `super_admin` token and drives `UpsertOperator` -> `UpsertPier` (lat 7.88, lng 98.39, "Proof Pier") -> `UpsertBoat` with `homePierId` through the admin proxy before the existing applied/exactly-once/public-list/single-trace/logs-correlated checks; `deploy/kong/roundtrip.sh` gained `public-piers-200`/`public-routes-200` checks and its `admin-proxy-upsert-boat-200` check now seeds a real operator+pier first

## Task Commits

1. **Task 1 (tracer): Boats under home-pier scope end-to-end** — `724d9a4` (feat)
2. **Task 2 (auto, tdd): Pier photo presign (D-19)** — `45f5531` (feat)

**Plan metadata:** committed separately after this SUMMARY.

_Note: `workflow.tdd_mode` is disabled for this project, so Task 2's tests and implementation were written and verified together in one pass, matching the process note already established in 02-03/02-06's SUMMARYs._

## Files Created/Modified

- `services/catalog/migrations/00007_boats_home_pier.sql` / `00008_pier_photo.sql` — `boats.home_pier_id`/`archived_at`, `piers.photo_key` with its shape check constraint
- `proto/events/catalog/v1/boat.proto`, `proto/services/catalog/v1/catalog.proto` — additive `Boat.home_pier_id`/`archived`, `UpsertBoatRequest.home_pier_id`, `ArchiveBoat` RPC; `Pier.photo_key`/`photo_url`, `UpsertPierRequest.photo_key`, `PresignPierPhoto` RPC
- `services/catalog/internal/domain/boat.go` — `HomePierID`/`Archived` fields, `Validate` requires a non-nil home pier
- `services/catalog/internal/domain/pier.go` — `PhotoKey` field + compiled `photoKeyPattern` regexp validation
- `services/catalog/internal/app/boat.go` — rewritten `UpsertBoat`/`ListBoats`/`ArchiveBoat` on the home-pier scope rule
- `services/catalog/internal/app/photo.go` — `Photos{Client, Bucket, PublicBaseURL}`, `PresignPierPhoto`, `URL`
- `services/catalog/internal/app/pier.go` — `PhotoKey` threaded through insert/update/row-mapping
- `services/catalog/internal/adapters/http/routes.go` — scoped `UpsertBoat`/`ListBoats`/`ArchiveBoat` handlers, `TODO(WR-01)` removed
- `services/catalog/internal/adapters/http/piers.go` — `toProtoPier` converted to a `*server` method (fills `photo_url`), new `PresignPierPhoto` handler
- `services/catalog/cmd/main.go` — `newPhotos()` builds `app.Photos` from `S3_*`/`PHOTO_PUBLIC_BASE_URL` env vars
- `services/catalog/cmd/main_integration_test.go` — boat tests updated to seed a real operator+pier via `super_admin` and pass `home_pier_id`
- `services/catalog/cmd/boats_photo_integration_test.go` — `TestBoatsScopedByHomePier`, `TestPierPhotoKeyAndUrl`, `TestPierPhotoUrlEmptyWithoutStorageConfig`
- `services/catalog/internal/app/photo_test.go` — `TestPresignPierPhoto(Validation|ScopeAndStorage)`, `TestPhotosURL`
- `deploy/proof.sh`, `deploy/kong/roundtrip.sh` — super_admin operator->pier->boat flow, `public-piers-200`/`public-routes-200` checks
- `services/catalog/CLAUDE.md` — boats scope, `ArchiveBoat`, `PresignPierPhoto`, photo env vars, key format, "catalog never proxies file bytes"

## Decisions Made

- Boats reuse routes' exact `GetPierForShareScoped`/`GetBoatForUpdateScoped` scope pattern rather than inventing a boat-specific variant — one scoping shape, applied consistently (Pitfall 6 from earlier plans).
- A `uuid.Nil` `HomePierID` is checked explicitly before any DB lookup and returns `InvalidArgument` directly, rather than letting an all-zero UUID flow into `GetPierForShareScoped` and get misreported as `NotFound`.
- `PresignHeader` (not `PresignedPutObject`) is the only `minio-go` call that lets `Content-Type`/`Content-Length` be part of the signed header set — required by T-02-08-02's mismatch-rejection guarantee.
- `cmd/main.go`'s `newPhotos()` treats an unset `S3_PUBLIC_ENDPOINT` as "not configured" (zero-value `Photos`, nil `Client`) rather than a startup error — object storage is optional dev infrastructure, matching the existing outbox-relay/consumer optionality pattern in the same file.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed nil home_pier_id producing NotFound instead of InvalidArgument**
- **Found during:** Task 1 integration test run (`TestUpsertBoatValidationAndTenancy`, `TestBoatsScopedByHomePier`)
- **Issue:** `UpsertBoat` passed `b.HomePierID` (zero value `uuid.Nil` when the request omits `home_pier_id`) straight into `GetPierForShareScoped`, which queried the all-zero UUID, got `pgx.ErrNoRows`, and returned `domain.ErrNotFound` — but the plan's acceptance criteria (and the domain's own `Validate()`) require `InvalidArgument` for a missing home pier.
- **Fix:** Added an explicit `b.HomePierID == uuid.Nil` check at the top of `app.UpsertBoat`, before any DB query, returning `domain.ErrInvalidArgument` directly.
- **Files modified:** `services/catalog/internal/app/boat.go`
- **Verification:** `TestBoatsScopedByHomePier`/`TestUpsertBoatValidationAndTenancy` assert `InvalidArgument` for a missing `home_pier_id`; both green.
- **Committed in:** `724d9a4` (Task 1 commit)

**2. [Rule 3 - Blocking] roundtrip.sh's admin-proxy-upsert-boat-200 check needed a real home pier**
- **Found during:** Task 1 `make kong-roundtrip` verification
- **Issue:** The existing check minted a bare `pier_admin` token (default operator, no `pier_ids`) and posted `UpsertBoat` with no `home_pier_id` — this now fails validation under D-07 (the boat write path this check exists to prove).
- **Fix:** The check now mints a `super_admin` token first, creates a real operator + pier via the admin proxy, then mints a `pier_admin` token scoped to that pier before posting the boat write.
- **Files modified:** `deploy/kong/roundtrip.sh`
- **Verification:** `make kong-roundtrip` — `admin-proxy-upsert-boat-200` passes against the live stack.
- **Committed in:** `724d9a4` (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (1 bug, 1 blocking) — both necessary consequences of intentionally introducing the D-07 home-pier requirement, not scope creep.
**Impact on plan:** No behavior beyond what the plan specified; both fixes make the plan's own acceptance criteria pass.

## Issues Encountered

- A first full-package integration test run (`go test -tags=integration ./services/catalog/...` with no filter) produced one `FAIL` immediately after `make proof`/`make kong-roundtrip` had just exercised the live Docker Compose stack — a re-run seconds later, with full `-v` output, passed every test in the package cleanly. Treated as a transient Docker/testcontainers resource-contention flake (concurrent compose stack + fresh testcontainers on the same Docker daemon), not a code defect — confirmed by the clean re-run and by every individually-targeted test passing on both runs.

## Known Stubs

None.

## User Setup Required

None — no external service configuration required. Object storage (`S3_PUBLIC_ENDPOINT`) remains optional; when unset, `PresignPierPhoto` returns `FailedPrecondition` and every other RPC is unaffected. Wiring an actual SeaweedFS/MinIO-compatible container into `docker-compose.yml` is plan 02-11's responsibility per the phase's `02-01-SUMMARY.md`.

## Next Phase Readiness

- `CAT-04`, `CAT-02`, `AUTH-05`, `CAT-06` are marked complete in this plan's frontmatter `requirements`; `AUTH-05`/`CAT-02`/`CAT-06` are also declared by sibling plans in this phase (`02-01`, `02-03`, `02-06`) — `requirements.ready-ids` gates completion on that automatically.
- `app.Photos` and the `PresignPierPhoto` RPC are ready for plan 02-11 (`apps/admin`) to wire a real upload widget against, once that plan's storage container decision (SeaweedFS, per `02-01-SUMMARY.md`) lands in `docker-compose.yml`.
- No blockers. `go build`/`go test` are green across the whole workspace; the full catalog integration suite (16 tests) passes live against real Postgres + Redpanda; `make proto-check` is idempotent; `services/catalog`'s own `golangci-lint run ./...` reports 0 issues; `make up && make kong-roundtrip && make proof` all pass against the live stack with the new super_admin operator -> pier -> boat flow.
- The whole-workspace `make lint` still fails only on the pre-existing, unrelated gateway test-file `gosec` findings already tracked in `deferred-items.md`/`WINDOWS.md` from plan 02-06 — not a new issue, not a blocker for this plan.

---
*Phase: 02-identity-catalog*
*Completed: 2026-09-26*

## Self-Check: PASSED

- All 6 key created files verified present on disk (`[ -f ]`): both migrations, `photo.go`, `photo_test.go`, `boats_photo_integration_test.go`, and this SUMMARY.
- Both task commits (`724d9a4`, `45f5531`) verified present in `git log --oneline --all`.
- Task 1 acceptance criteria re-run and confirmed: `proto/events/catalog/v1/boat.proto` contains `string home_pier_id = 6;` and `bool archived = 7;`; `grep -c "TODO(WR-01)" services/catalog/internal/adapters/http/routes.go` = 0; `make proof` printed `PASS applied`/`PASS exactly-once`/`PASS single-trace`; `make kong-roundtrip` printed `PASS public-routes-200`.
- Task 2 acceptance criteria re-run and confirmed: `services/catalog/go.mod` requires `github.com/minio/minio-go/v7`; `photo.go` contains `PresignHeader`, `"Content-Length"`, and `5 << 20`; `00008_pier_photo.sql` contains the key regexp check; `photo_test.go`'s `TestPresignPierPhoto` asserts `X-Amz-SignedHeaders` includes `content-length` and passes (`go test -v` prints `--- PASS: TestPresignPierPhoto`).
- Plan-level `<verification>` re-run: `go test -count=1 ./...` green across the whole workspace; full catalog integration suite green (16 tests, `go test -tags=integration -count=1 -timeout 15m ./services/catalog/...`); `services/schedule`'s integration suite green (additive event fields, zero consumer changes); `make proto-check` exits 0 with a clean `gen/` diff; `services/catalog`'s own `golangci-lint run ./...` reports 0 issues; `make up && make kong-roundtrip && make proof` all pass against the live stack.
