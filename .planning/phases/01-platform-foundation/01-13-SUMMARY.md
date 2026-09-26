---
phase: 01-platform-foundation
plan: 13
subsystem: infra
tags: [ci, jenkins, make, changed-services, harbor, testcontainers]

requires:
  - phase: 01-platform-foundation (plans 01-12)
    provides: "Makefile targets (lint, test, test-integration, template-smoke, images, push), Jenkins+Harbor CI stack (deploy/ci/docker-compose.yml), root Dockerfile, all 4 service directories (catalog, gateway, schedule, _template)"
provides:
  - "deploy/ci/changed-services.sh: base-ref or --stdin path selector; pkg/proto/gen/_template/go.work(.sum)/Makefile/Dockerfile select ALL services, services/<name>/ selects only <name> (exact segment)"
  - "deploy/ci/changed-services_test.sh: table-driven coverage of every PLAT-08 edge case (adjacency, dedup, multi-service sort, docs-only empty, web-only empty)"
  - "Makefile `ci` target: one command reproduces the full Jenkins pipeline order (lint, proto-check, test, test-integration, migrate-validate, template-smoke, web-check, images), never skippable on test-integration"
  - "Makefile gains BASE (origin/main, falls back to main), SERVICES now defaults from changed-services.sh(BASE), migrate-validate (goose validate per service), web-check (format:check + next build), lint gains web eslint+tsc"
  - "Final Jenkinsfile: Prepare stage computes BASE, stages Tools/Lint/Proto/Unit/Integration/Migrations/Template/Web/Images/Push exactly mirror make ci, only Push is conditional (branch 'main')"
  - "deploy/ci/smoke.sh asserts the actual Jenkins build console contains \"make test-integration\" and \"PASS template-smoke\" before reporting green, so a passing build can't have silently skipped either gate"
affects: ["phase-02-identity-catalog (inherits make ci as the standing quality gate for every future service)"]

actuals:
  tokens: 3376
  tasks: 2
  commits: 3
  plan_head_before: 5177395fc31bae85c157d1be6863fc60d4b0ca0f

tech-stack:
  added: []
  patterns:
    - "changed-services.sh pins LC_ALL=C on every sort call so service-name ordering is byte-identical between a dev host's en_US.UTF-8 locale and the CI agent's likely C/POSIX locale -- without it '_template' sorts before or after 'catalog' depending on the machine"
    - "Makefile SERVICES now defaults from `deploy/ci/changed-services.sh $(BASE)` instead of an unconditional 'every service with cmd/main.go' scan, so `make images`/`make push` narrow to the actual diff by default while still being fully overridable (SERVICES=foo)"
    - "goose CLI pinned to the same v3.27.3 already used by deploy/migrate/Dockerfile and pkg/go.mod (v3.28.0 needs go1.26, breaking this repo's go1.25.x pin) -- migrate-validate reuses the identical version, not a second independent pin"

key-files:
  created:
    - deploy/ci/changed-services.sh
    - deploy/ci/changed-services_test.sh
  modified:
    - Makefile
    - Jenkinsfile
    - deploy/ci/smoke.sh

key-decisions:
  - "LC_ALL=C on every sort in changed-services.sh -- locale-dependent collation would otherwise make 'make ci' reproduce a different service order on the Jenkins agent than on a developer's host, defeating the plan's own 'developer gets the exact verdict Jenkins will give' objective"
  - "goose validate pinned to v3.27.3 (matches deploy/migrate/Dockerfile and pkg/go.mod), not the plan's illustrative v3.28.0 -- v3.28.0 requires go1.26.0, which would break the repo's go1.25.x toolchain pin (same class of issue already hit in 01-04/01-06/01-12)"
  - "smoke.sh's RESULT/SHA/BUILDING captures fixed to survive `set -o pipefail`: grep finding no match on an in-progress build's null result field failed the whole pipe (even though head/cut succeeded), silently killing the script via set -e on the very first poll -- every real Jenkins run hit this, not an edge case"
  - "smoke.sh's new console-text assertion (T-13-02) retries up to 5x/3s: Jenkins can flip a build's `result` to a terminal state via the API a beat before /consoleText reflects the fully-flushed log, so a single fetch right at the SUCCESS transition can report a false 'missing test-integration/template-smoke' on a build that plainly ran both"

patterns-established: []

requirements-completed: [PLAT-08, PLAT-04, PLAT-09]

coverage:
  - id: D1
    description: "deploy/ci/changed-services.sh selects services by exact path segment (services/catalogx/ never selects catalog) and treats pkg/proto/gen/_template/go.work(.sum)/Makefile/Dockerfile changes as select-all; docs-only and web-only changes select nothing"
    requirement: PLAT-08
    verification:
      - kind: other
        ref: "bash deploy/ci/changed-services_test.sh -> 13/13 PASS"
        status: pass
    human_judgment: false
  - id: D2
    description: "make ci reproduces the full Jenkins pipeline locally in order (lint incl. web eslint+tsc, proto-check, test, test-integration, migrate-validate, template-smoke, web-check, images), never skippable on test-integration"
    requirement: PLAT-08
    verification:
      - kind: other
        ref: "make ci (real run, no mocks) -> exit 0, stage order confirmed in log, zero FAIL lines"
        status: pass
    human_judgment: false
  - id: D3
    description: "Final Jenkinsfile stages mirror make ci exactly; only the Push stage is conditional and gated to branch 'main'"
    requirement: PLAT-08
    verification:
      - kind: other
        ref: "grep -c \"when {\" Jenkinsfile == 1; grep -A3 \"stage('Push')\" Jenkinsfile | grep -q \"branch 'main'\""
        status: pass
    human_judgment: false
  - id: D4
    description: "A real Jenkins build of the current branch's HEAD goes green (Push correctly skipped, not on main) and deploy/ci/smoke.sh confirms the console shows both make test-integration and PASS template-smoke actually ran"
    requirement: PLAT-08
    verification:
      - kind: integration
        ref: "make ci-up && deploy/ci/smoke.sh -> PASS jenkins-build SUCCESS (build #28, Finished: SUCCESS, Push stage skipped due to when conditional)"
        status: pass
    human_judgment: false
  - id: D5
    description: "Two Jenkins scan cycles prove path-scoped rebuilds in the real system: a pkg/ change rebuilds every service image, a services/schedule/-only change rebuilds only schedule, and neither push unless on main"
    human_judgment: true
    rationale: "Requires watching a live Jenkins react to two separate real pushes across scan cycles (the plan's own <human-check>); deferred to end-of-phase UAT per workflow.human_verify_mode=end-of-phase, per the plan's embedded <verify><human-check> block"

duration: 25min
completed: 2026-09-26
status: complete
---

# Phase 01 Plan 13: CI Completion — make ci + Final Jenkinsfile Summary

**One `make ci` now reproduces the exact Jenkins verdict locally (changed-services selector, full lint/proto/test/integration/migration/template/web/images gate chain), and the final Jenkinsfile mirrors it stage-for-stage with Push gated to main only — verified against a real green Jenkins build.**

## Performance

- **Duration:** 25 min
- **Started:** 2026-09-26T07:07:23Z (approx, continuing from 01-12 completion)
- **Completed:** 2026-09-26T07:31:41Z
- **Tasks:** 2 completed
- **Files modified:** 5 (2 created, 3 modified)

## Accomplishments
- `deploy/ci/changed-services.sh` selects affected services from `git diff --name-only <base>...HEAD` (or `--stdin` for tests): infra-wide paths (`pkg/`, `proto/`, `gen/`, `services/_template/`, `go.work(.sum)`, `Makefile`, `Dockerfile`) select every service; `services/<name>/...` selects only `<name>` by exact path segment (`catalogx` never matches `catalog`); output is deduped and `LC_ALL=C`-sorted for cross-machine determinism
- `deploy/ci/changed-services_test.sh` covers all 13 PLAT-08 edge cases table-driven, including the docs-only and web-only empty-selection edges
- `make ci` chains `changed-services_test.sh -> lint -> proto-check -> test -> test-integration -> migrate-validate -> template-smoke -> web-check -> images` with no skip flag anywhere near `test-integration`, and ran green end-to-end for real (not mocked) during this plan's own verification
- Final `Jenkinsfile`: a `Prepare` stage resolves `BASE` (prefers `GIT_PREVIOUS_SUCCESSFUL_COMMIT` on main, else `origin/main` after fetch, else a deliberately-unresolvable sentinel that makes `changed-services.sh` safely select all services), then stages `Tools/Lint/Proto/Unit/Integration/Migrations/Template/Web/Images/Push` call exactly one `make` target each; only `Push` carries a `when { branch 'main' }` guard
- `deploy/ci/smoke.sh` now also asserts the Jenkins build's console text actually contains `make test-integration` and `PASS template-smoke` before reporting green (T-13-02) — confirmed against a real triggered build (#28, `Finished: SUCCESS`, `Push` correctly `skipped due to when conditional` on this non-main branch)

## Task Commits

1. **Task 1: make ci end-to-end locally — changed-services.sh + table test + Makefile ci target** - `e70f05e` (feat)
2. **Task 2: Final Jenkinsfile mirrors make ci; smoke.sh console assertions** - `6fc4f53` (feat)
   - Follow-up fix (found verifying this same task): `f6aedcb` (fix — smoke.sh pipefail + console-race)

**Plan metadata:** (this commit, made after this SUMMARY)

## Files Created/Modified
- `deploy/ci/changed-services.sh` - path-based service selector (base-ref or `--stdin`)
- `deploy/ci/changed-services_test.sh` - table-driven selector coverage
- `Makefile` - `BASE`, `SERVICES` default, `lint` web checks, `migrate-validate`, `web-check`, `ci`, `images` empty-set message
- `Jenkinsfile` - `Prepare` (BASE resolution) + full stage chain mirroring `make ci`
- `deploy/ci/smoke.sh` - console-text assertions for `test-integration`/`template-smoke`, plus two bug fixes (below)

## Decisions Made
- `LC_ALL=C` pinned on every `sort` in `changed-services.sh` so output order is identical regardless of the running machine's locale
- `migrate-validate` uses goose `v3.27.3` (matching `deploy/migrate/Dockerfile` and `pkg/go.mod`), not the plan's illustrative `v3.28.0`, to stay on the repo's go1.25.x toolchain pin
- `SERVICES` now defaults from `changed-services.sh $(BASE)` instead of "every scaffolded service," narrowing `make images`/`make push` to the real diff by default (still fully overridable)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `smoke.sh` RESULT/SHA/BUILDING captures broke under `set -o pipefail` on every real (non-instant) build**
- **Found during:** Task 2's own verification (`make ci-up && deploy/ci/smoke.sh`) — first real poll of an in-progress build
- **Issue:** `RESULT=$(echo "$RESP" | grep -o '"result":"[A-Z]*"' | head -1 | cut -d'"' -f4)` fails as a whole pipeline under `pipefail` whenever `grep` finds no match (Jenkins reports `"result":null` while a build is still running) — even though `head`/`cut` succeed. Under `set -e`, that silently killed the script on the very first poll of any build that wasn't already finished, which is every real build.
- **Fix:** Appended `|| true` to each of the three captures; an empty value in the "still building" case is the correct, expected result, not an error.
- **Files modified:** `deploy/ci/smoke.sh`
- **Verification:** Re-ran against a real in-progress-then-SUCCESS Jenkins build (#28) — script now survives the poll loop and reports `PASS jenkins-build SUCCESS`
- **Committed in:** `f6aedcb`

**2. [Rule 1 - Bug] Console-text assertion raced Jenkins' own result-vs-log-flush timing**
- **Found during:** Task 2's own verification, immediately after fixing deviation 1 — the newly-added console assertion (from this same plan's Task 2 commit) reported `FAIL jenkins-build-console missing:make-test-integration PASS-template-smoke` on build #28, even though a manual re-fetch of the identical `/consoleText` endpoint moments later found both strings present exactly once
- **Issue:** Jenkins can flip a build's `result` field to a terminal value via the JSON API a beat before `/consoleText` reflects the fully-flushed console log; a single fetch made right at the SUCCESS transition can therefore race and return an incomplete log
- **Fix:** Retry the console-text fetch up to 5 times, 3 seconds apart, before concluding either assertion genuinely failed
- **Files modified:** `deploy/ci/smoke.sh`
- **Verification:** Re-ran `deploy/ci/smoke.sh` against the same already-finished build #28 — `PASS jenkins-build SUCCESS`
- **Committed in:** `f6aedcb`

---

**Total deviations:** 2 auto-fixed (both Rule 1 — bugs found while running this plan's own required verification against a real Jenkins instance, not hypothetical)
**Impact on plan:** Both fixes were necessary for `deploy/ci/smoke.sh` to ever report a truthful result against a real build; without them the script either crashes on the first in-progress poll or flakes with a false negative on a genuinely green build. No scope creep — both fixes are confined to `deploy/ci/smoke.sh`, the exact file this plan's Task 2 already modifies.

## Issues Encountered
None beyond the two deviations above (already fully resolved and verified).

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `make ci` is now the standing local quality gate every future phase's plans inherit; `Jenkinsfile` is Jenkins' own copy of the same gate chain
- The plan's embedded `<human-check>` (two live Jenkins scan cycles proving path-scoped rebuild scoping) is deferred to end-of-phase UAT per `workflow.human_verify_mode=end-of-phase` — not a blocker for this SUMMARY, but flagged for `/gsd-verify-work`
- Phase 01 (Platform Foundation) is now fully executed: 13/13 plans complete
- No blockers for proceeding to Phase 2 planning

## Self-Check: PASSED

All key files confirmed present on disk (`deploy/ci/changed-services.sh`, `deploy/ci/changed-services_test.sh`, `Makefile`, `Jenkinsfile`, `deploy/ci/smoke.sh`); all 3 commits (`e70f05e`, `6fc4f53`, `f6aedcb`) confirmed in `git log`; `bash deploy/ci/changed-services_test.sh` re-run clean (13/13 PASS); `make ci` re-verified green end-to-end against real infrastructure; `deploy/ci/smoke.sh` re-verified green against a real Jenkins build.

---
*Phase: 01-platform-foundation*
*Completed: 2026-09-26*
