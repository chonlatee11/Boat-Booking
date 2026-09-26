---
phase: 01-platform-foundation
plan: 02
subsystem: infra
tags: [buf, protobuf, connect-go, protoc-gen-es, kafka-envelope, catalog]

requires:
  - phase: 01-01
    provides: go.work, root Dockerfile/Makefile, pkg/httpx trust-boundary conventions
provides:
  - buf.yaml (single module at proto/, v2 schema) + buf.gen.yaml (pinned remote plugins)
  - platformv1.Envelope — the real Kafka value wire contract (D-06)
  - catalogv1.BoatUpserted event + BoatStatus enum, ids only, no PII (D-45)
  - catalogv1connect.CatalogService (UpsertBoat, ListBoats) generated Go handler/client + TS JSON types
  - gen/go module wired into go.work; committed gen/go and gen/ts output
  - make proto-gen / make proto-check (lint, breaking-vs-main, gen/ drift, PII gate)
  - proto/pii-check.sh — PII-shaped field-name gate reusable by any later event proto
affects: [01-05, 01-09, 01-10, 01-11, 01-13]

actuals:
  tokens: 15687
  tasks: 2
  commits: 2
  plan_head_before: 757e0430b052acfcc5d4b14b5ac26da31a90ce8b

tech-stack:
  added: ["buf v1.73.0", "protocolbuffers/go v1.36.12 (remote plugin)", "connectrpc/go v1.18.1 (remote plugin)", "bufbuild/es v2.15.0 (remote plugin)"]
  patterns:
    - "One buf module at proto/, packages boatbooking.<svc>.events.v1 / boatbooking.<svc>.v1, both mapped to the same Go package gen/go/<svc>/v1 — enums/messages shared across an events proto and its sync-API proto are defined once and imported, never redeclared (D-07)"
    - "buf.gen.yaml has no clean: — gen/go/go.mod is hand-written and must survive make proto-gen"
    - "make proto-check drift step = re-run proto-gen then git status --porcelain --untracked-files=all -- gen/; any output fails the gate"
    - "proto/pii-check.sh is the mechanical backstop for the 'no PII in proto/events/**' rule (D-45) — grep-based, reusable for every future event proto"

key-files:
  created:
    - buf.yaml
    - buf.gen.yaml
    - proto/events/platform/v1/envelope.proto
    - proto/events/catalog/v1/boat.proto
    - proto/services/catalog/v1/catalog.proto
    - proto/pii-check.sh
    - proto/README.md
    - gen/go/go.mod
    - gen/go/go.sum
    - gen/go/platform/v1/envelope.pb.go
    - gen/go/catalog/v1/boat.pb.go
    - gen/go/catalog/v1/catalog.pb.go
    - gen/go/catalog/v1/catalogv1connect/catalog.connect.go
    - gen/ts/events/platform/v1/envelope_pb.ts
    - gen/ts/events/catalog/v1/boat_pb.ts
    - gen/ts/services/catalog/v1/catalog_pb.ts
    - go.work.sum
  modified:
    - go.work
    - Dockerfile
    - Makefile

key-decisions:
  - "Pinned buf.build/protocolbuffers/go to v1.36.12 (latest v1.36.x tag on proxy.golang.org at execution time) rather than an illustrative version — this matches the pattern from 01-01's Docker base-image fix (verify exact tags before pinning)."
  - "go.work.sum committed alongside go.work/gen/go — not called out explicitly in the plan's files_modified list, but it is the workspace-mode checksum lockfile analogous to go.sum and is required for reproducible `go build` in workspace mode."
  - "proto-check's breaking-change step correctly no-ops with a printed notice (main has no buf.yaml yet) rather than failing — verified by direct execution, not just code inspection."

requirements-completed: [PLAT-04]

coverage:
  - id: D1
    description: "make proto-gen regenerates committed gen/go (protoc-gen-go + connect-go) and gen/ts (protoc-gen-es, JSON types) from proto/ in one command"
    requirement: "PLAT-04"
    verification:
      - kind: integration
        ref: "make proto-gen && go build github.com/chonlatee11/boat-booking/gen/go/... (re-run after commit, clean tree)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Envelope{event_id, event_type, aggregate_id, occurred_at, version, Any payload} and catalog.BoatUpserted{boat_id, operator_id, name, default_capacity, status} exist as the real wire contract"
    requirement: "PLAT-04"
    verification:
      - kind: other
        ref: "grep google.protobuf.Any payload proto/events/platform/v1/envelope.proto; grep message BoatUpserted / enum BoatStatus proto/events/catalog/v1/boat.proto"
        status: pass
    human_judgment: false
  - id: D3
    description: "CatalogService{UpsertBoat, ListBoats} generates a connect-go handler/client package and TS JSON types"
    requirement: "PLAT-04"
    verification:
      - kind: integration
        ref: "test -f gen/go/catalog/v1/catalogv1connect/catalog.connect.go; grep BoatUpsertedJson gen/ts/events/catalog/v1/boat_pb.ts"
        status: pass
    human_judgment: false
  - id: D4
    description: "make proto-check fails on buf lint errors, gen/ drift, breaking changes against main (skipped with notice while main has no buf.yaml), and PII-shaped field names under proto/events"
    requirement: "PLAT-04"
    verification:
      - kind: integration
        ref: "make proto-check (clean pass); proto/pii-check.sh temp-dir probe with a declared `email` field (exit 1, confirmed)"
        status: pass
    human_judgment: false
  - id: D5
    description: "Root Dockerfile copies gen/go so service images build with GOWORK=off"
    requirement: "PLAT-04"
    verification:
      - kind: other
        ref: "grep 'COPY gen/go' Dockerfile"
        status: pass
    human_judgment: false

duration: 18min
completed: 2026-09-26
status: complete
---

# Phase 1 Plan 2: Proto Toolchain Summary

**One buf module at proto/ producing the real committed Envelope + BoatUpserted event contract and CatalogService connect-go/TS API, gated by `make proto-check` (lint, breaking, drift, PII).**

## Performance

- **Duration:** ~18 min
- **Started:** 2026-09-26 (approx, first file write)
- **Completed:** 2026-09-26T01:57:54Z
- **Tasks:** 2 (1 tracer + 1 auto)
- **Files created:** 17, modified: 3

## Accomplishments

- `buf.yaml` (v2, single module at `proto/`, STANDARD lint minus `PACKAGE_DIRECTORY_MATCH`, `FILE` breaking rules) + `buf.gen.yaml` with pinned remote plugins (`protocolbuffers/go v1.36.12`, `connectrpc/go v1.18.1`, `bufbuild/es v2.15.0`)
- `platformv1.Envelope` — the real Kafka value wire contract with `google.protobuf.Any payload`; no trace fields (traceparent stays in Kafka headers per D-06)
- `catalogv1.BoatUpserted` + `BoatStatus` enum defined once in the events proto and imported (not redeclared) by `catalogv1.CatalogService`'s sync API proto — both share Go package `catalogv1`
- `CatalogService{UpsertBoat, ListBoats}` generates `catalogv1connect.CatalogServiceHandler`/`Client` plus TS `BoatUpsertedJson`/`BoatJson`/`UpsertBoatRequestJson`/`ListBoatsResponseJson` types
- `make proto-check`: buf lint → breaking-vs-main (correctly no-ops with a notice since `main` has no `buf.yaml` yet) → `proto/pii-check.sh proto/events` → gen/ drift check (re-run proto-gen, fail on any `git status --porcelain` output)
- `proto/pii-check.sh`: case-insensitive grep gate for PII-shaped field names (`email`, `phone`, `first_name`, `address`, `dob`, ...); verified to both pass the real tree and fail a deliberately-injected `email` field probe

## Task Commits

Each task was committed atomically:

1. **Task 1: Edit proto → make proto-gen → Go compiles and TS is emitted** - `b1605ad` (feat)
2. **Task 2: make proto-check — buf lint, breaking vs main, gen/ drift, PII field gate** - `f68ab94` (chore)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP/REQUIREMENTS update)

_Task 1 is `type="tracer"` — the tracer feedback gate (re-running `make proto-gen && go build ... && make test && make lint`) was re-executed after commit and passed before starting Task 2, per the `human_verify_mode: end-of-phase` + automated-only `<verify>` rule (no checkpoint needed on success)._

## Files Created/Modified

- `buf.yaml` — single buf module at `proto/`, v2 schema
- `buf.gen.yaml` — pinned remote plugins, `inputs: [{directory: proto}]`, no `clean:`
- `proto/events/platform/v1/envelope.proto` — `Envelope` Kafka wire contract
- `proto/events/catalog/v1/boat.proto` — `BoatStatus` enum + `BoatUpserted` event
- `proto/services/catalog/v1/catalog.proto` — `CatalogService`, imports `BoatStatus`
- `proto/pii-check.sh` — PII field-name gate (D-45)
- `proto/README.md` — proto review checklist (past-tense events, no PII, satang money, reserved fields, version bump rule)
- `gen/go/go.mod`, `gen/go/go.sum` — hand-written module, wired into `go.work`
- `gen/go/platform/v1/envelope.pb.go`, `gen/go/catalog/v1/boat.pb.go`, `gen/go/catalog/v1/catalog.pb.go`, `gen/go/catalog/v1/catalogv1connect/catalog.connect.go` — generated Go
- `gen/ts/events/platform/v1/envelope_pb.ts`, `gen/ts/events/catalog/v1/boat_pb.ts`, `gen/ts/services/catalog/v1/catalog_pb.ts` — generated TS
- `go.work` — added `./gen/go`; `go.work.sum` — new (workspace checksum lockfile)
- `Dockerfile` — added `COPY gen/go ./gen/go` before the service copy
- `Makefile` — `proto-gen`, `proto-check` targets; `dev-tools` installs `buf@v1.73.0`; `lint` extended with `buf lint` + `proto/pii-check.sh`

## Decisions Made

- Pinned `buf.build/protocolbuffers/go` to the exact current tag `v1.36.12` (verified against `proxy.golang.org`) rather than the plan's illustrative "v1.36.x" — same discipline as 01-01's Docker base-image fix.
- Committed `go.work.sum` (not in the plan's `files_modified` list) since it's the workspace-mode analog of `go.sum`, required for `go build` reproducibility once `gen/go` joined the workspace.

## Deviations from Plan

None - plan executed exactly as written. `go.work.sum` was an expected byproduct of adding a module to `go.work`, not a deviation from behavior — no fix was needed, just an additional file to commit alongside the intended change.

## Issues Encountered

None. `buf lint` passed on the first attempt for all three proto files; the PACKAGE_DIRECTORY_MATCH exception was already anticipated in the plan and configured in `buf.yaml` from the start.

## User Setup Required

None — no external service configuration required. `buf` is installed via `make dev-tools` (go-installable, no separate account/credentials).

## Next Phase Readiness

- The proto toolchain is the shared dependency for every later plan in this phase: outbox/consumer plans decode `platformv1.Envelope`, catalog-service implements `catalogv1connect.CatalogServiceHandler`, and the web skeleton imports `gen/ts` types directly.
- `make proto-check` is a real, verified gate (not just configured) — it will be wired into CI in plan 13.
- Flagged assumption carried forward unresolved: buf remote plugins on buf.build must stay reachable from the Jenkins agent for `proto-check`'s drift step to be CI-stable; if BSR rate-limits CI, switch to locally-installed plugins pinned to the same versions (noted in the plan, not yet needed).
- No blockers for 01-03 onward.

## Self-Check: PASSED

- All 17 created files verified present on disk (`[ -f ... ]` for each `key-files.created` entry, checked during Task 1/2 verification).
- Both task commit hashes (`b1605ad`, `f68ab94`) verified present in `git log --oneline`.
- Plan-level `<verification>` re-run clean at the end of this plan: `make proto-gen && make proto-check && make test && make lint` all exit 0; `git status --short` shows a clean tree for every plan-scoped path (only pre-existing unrelated files remain uncommitted, per the executor's explicit instructions not to touch them).

---
*Phase: 01-platform-foundation*
*Completed: 2026-09-26*
