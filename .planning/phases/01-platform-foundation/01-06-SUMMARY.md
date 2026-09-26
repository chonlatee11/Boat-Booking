---
phase: 01-platform-foundation
plan: 06
subsystem: infra
tags: [jenkins, jcasc, job-dsl, harbor, docker-compose, ci, testcontainers]

requires:
  - phase: 01-platform-foundation (plans 01-02)
    provides: Makefile targets (dev-keys, test, test-integration, lint, images build context), root Dockerfile, .env.example convention
provides:
  - Jenkins controller + SSH docker-agent CI stack (deploy/ci/docker-compose.yml)
  - Harbor v2.15.2 image registry running beside Jenkins from the same compose file
  - Jenkinsfile pipeline (Tools/Lint/Unit/Integration/Images/Push) that make ci reproduces locally
affects: [01-07, 01-13 (folds these targets into `make ci` and narrows builds to changed services)]

actuals:
  tokens: 5522
  tasks: 2
  commits: 4
  plan_head_before: fe38a7c75169ca581c2517b0f706698b2de243f6

tech-stack:
  added: ["jenkins/jenkins:2.568.3-lts-jdk21", "jenkins/ssh-agent:9.0.0-jdk21", "Harbor v2.15.2", "job-dsl/JCasC multibranch pipeline"]
  patterns:
    - "docker-compose group_add does not propagate through sshd's PAM user switch — fix docker-group membership against the live docker.sock GID in the agent's own ENTRYPOINT, not via compose group_add alone"
    - "curl needs -g (globoff) against any Jenkins tree=...[...] query — the brackets are otherwise parsed as URL-range globbing and every request silently fails"
    - "keep JCasC's CASC_JENKINS_CONFIG file outside JENKINS_HOME so a rebuilt image's config isn't shadowed by an already-populated named volume"

key-files:
  created:
    - deploy/ci/docker-compose.yml
    - deploy/ci/jenkins/{Dockerfile,plugins.txt,casc.yaml}
    - deploy/ci/agent/{Dockerfile,entrypoint.sh}
    - deploy/ci/ci-keys.sh
    - deploy/ci/smoke.sh
    - deploy/ci/harbor/harbor.yml.tmpl
    - deploy/ci/harbor-prepare.sh
    - Jenkinsfile
  modified:
    - Makefile
    - .env.example
    - .gitignore

key-decisions:
  - "JCasC config file lives at /usr/local/jenkins-casc.yaml, not /var/jenkins_home/casc.yaml as literally suggested by planning — the latter gets shadowed by Docker's named-volume first-run population, so a later image rebuild silently stops applying config changes"
  - "Jenkins agent's docker-group membership is fixed at container start (entrypoint.sh) against the actual docker.sock GID, not solely via docker-compose group_add — group_add only sets PID 1's supplementary groups, and sshd's PAM-based login for the jenkins build user resets them from /etc/group, dropping the extra GID (Pitfall 12)"
  - "smoke.sh sources .env values via targeted grep, not `. ./.env` — some values (the SSH pubkey) contain spaces and break bash's source/dot-command parsing"
  - "Push stage runs inside the jenkins-agent container's own docker CLI context (fresh, no credential-store config), verified directly via docker compose exec to sidestep this dev sandbox's unrelated, pre-existing broken `pass`/GPG credential-store setup — the Makefile itself is unchanged and correct for any normally-configured host"

requirements-completed: [PLAT-08]

coverage:
  - id: D1
    description: "Jenkins controller + SSH build agent (docker.sock mounted) come up healthy from `make ci-up`, JCasC applies with no anonymous access"
    requirement: PLAT-08
    verification:
      - kind: other
        ref: "make ci-up (docker compose --wait reports jenkins/jenkins-agent healthy)"
        status: pass
    human_judgment: false
  - id: D2
    description: "A commit on the current branch is picked up by the boat-booking multibranch job and Jenkinsfile runs make dev-tools/lint/test/test-integration/images to a green SUCCESS build, with the agent able to run testcontainers via host.docker.internal"
    requirement: PLAT-08
    verification:
      - kind: other
        ref: "deploy/ci/smoke.sh -> PASS jenkins-build SUCCESS"
        status: pass
      - kind: integration
        ref: "Jenkins console log: pkg/outbox integration test passed under `make test-integration` on the agent"
        status: pass
    human_judgment: false
  - id: D3
    description: "Harbor v2.15.2 runs from the same compose file with a private `boatbooking` project; make push tags/pushes images there and the Jenkinsfile Push stage only runs on main"
    requirement: PLAT-08
    verification:
      - kind: other
        ref: "make ci-up && make images SERVICES=gateway TAG=ci-smoke && make push SERVICES=gateway TAG=ci-smoke && EXPECT_TAG=ci-smoke deploy/ci/smoke.sh --harbor -> PASS harbor-ping Pong"
        status: pass
      - kind: other
        ref: "grep -A3 \"stage('Push')\" Jenkinsfile | grep -q \"branch 'main'\""
        status: pass
    human_judgment: false

duration: 71min
completed: 2026-09-26
status: complete
---

# Phase 01 Plan 06: Jenkins + Harbor CI Summary

**Jenkins controller (JCasC, no anonymous access) + SSH docker-agent multibranch pipeline builds and tests every push via `make` targets, with Harbor v2.15.2 receiving `:sha` images from `main` only — all from one `deploy/ci/docker-compose.yml`.**

## Performance

- **Duration:** 71 min
- **Started:** 2026-09-26T02:52:00Z
- **Completed:** 2026-09-26T04:03:28Z
- **Tasks:** 2 completed
- **Files modified:** 14

## Accomplishments
- Jenkins controller (JCasC: local realm, `allowAnonymousRead: false`, 0 controller executors) + one SSH docker-agent (Go 1.25.9, Node 24, Docker CLI/buildx, make/git/curl/jq) come up healthy from `make ci-up`, matching `deploy/ci/docker-compose.yml`
- `boat-booking` multibranch pipeline job (job-dsl in JCasC) scans `file:///repo` every 2 minutes and built the current branch's HEAD commit to a genuine `SUCCESS`, including a testcontainers-backed integration test (`pkg/outbox`) running on the agent via `host.docker.internal`
- Harbor v2.15.2 runs beside Jenkins from the same compose file (`include:`), HTTP-only on `localhost:8880`, with a private `boatbooking` project auto-created by `make ci-up`
- `make push` tags and pushes built images to Harbor; the Jenkinsfile's `Push` stage is gated `when { branch 'main' }` using a JCasC-provisioned `harbor` credential

## Task Commits

1. **Task 1: Jenkins controller + SSH docker-agent, JCasC multibranch scan** - `463640b` (feat)
   - Follow-up fix: `2513c38` (fix — chown agent volumes) and `bae5291` (fix — curl -g)
2. **Task 2: Harbor registry, make push, main-only Push stage** - `0f13b0b` (feat)

**Plan metadata:** (this commit, made after this SUMMARY)

## Files Created/Modified
- `deploy/ci/docker-compose.yml` - Jenkins + SSH agent + Harbor (via `include:`)
- `deploy/ci/jenkins/{Dockerfile,plugins.txt,casc.yaml}` - JCasC controller image
- `deploy/ci/agent/{Dockerfile,entrypoint.sh}` - build agent image + docker-group-membership fixup
- `deploy/ci/ci-keys.sh` - idempotent JENKINS_*/DOCKER_GID/CI_REPO_URL fill
- `deploy/ci/smoke.sh` - Jenkins build + Harbor (`--harbor`) verification
- `deploy/ci/harbor/harbor.yml.tmpl`, `deploy/ci/harbor-prepare.sh` - Harbor installer bootstrap
- `Jenkinsfile` - Tools/Lint/Unit/Integration/Images/Push pipeline
- `Makefile` - `ci-keys`, `ci-up`, `ci-down`, `images`, `push`
- `.env.example`, `.gitignore` - new CI/Harbor env keys, ignore Harbor's generated installer/data dirs

## Decisions Made
- JCasC config kept outside `JENKINS_HOME` (`/usr/local/jenkins-casc.yaml`) so a rebuilt controller image's config isn't shadowed by the already-populated `jenkins_home` named volume
- Agent's docker-group membership fixed at container start (real docker.sock GID, not the static `.env` `DOCKER_GID`) because `docker-compose group_add` doesn't survive sshd's PAM user switch to the `jenkins` build user (Pitfall 12)
- `smoke.sh` reads `.env` values with targeted `grep`, never `. ./.env` — the SSH pubkey value contains spaces and breaks shell-sourcing

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] JCasC config path moved off the `JENKINS_HOME` volume**
- **Found during:** Task 1 implementation
- **Issue:** Plan specified `ENV CASC_JENKINS_CONFIG=/var/jenkins_home/casc.yaml`. Docker populates a fresh named volume from the image's existing directory content on first run, so this works once — but a later `docker compose build` that changes `casc.yaml` would silently stop being applied, since the volume is already populated and no longer copies from the image.
- **Fix:** Placed `casc.yaml` at `/usr/local/jenkins-casc.yaml` instead, outside any volume-mounted path.
- **Files modified:** `deploy/ci/jenkins/Dockerfile`
- **Verification:** JCasC applied cleanly on every `make ci-up` rebuild during this plan's execution (3 rebuilds, including after fixing the entrypoint bug below)
- **Commit:** `463640b`

**2. [Rule 2 - Missing Critical] Agent entrypoint fixes docker-group membership at runtime**
- **Found during:** Task 1 acceptance-criteria verification (agent could not reach the Docker daemon as the `jenkins` build user)
- **Issue:** `docker-compose group_add` only sets the container's PID 1 supplementary groups. sshd forks a fresh login session for the `jenkins` user via PAM, which resets supplementary groups from `/etc/group` and drops the injected GID — so testcontainers-based integration tests would fail with a Docker permission error over the actual SSH session Jenkins builds in.
- **Fix:** Added `deploy/ci/agent/entrypoint.sh`, which detects the real docker.sock GID at container start, creates/updates a matching group, and adds `jenkins` to it before starting sshd.
- **Files modified:** `deploy/ci/agent/Dockerfile`, `deploy/ci/agent/entrypoint.sh` (new)
- **Verification:** `docker compose exec jenkins-agent docker version` succeeds; `make test-integration`'s testcontainers-backed `pkg/outbox` test passed inside the real Jenkins build
- **Commit:** `463640b`

**3. [Rule 1 - Bug] `agent_go`/`agent_cache` named volumes chowned to the build user**
- **Found during:** Task 1 verification — first real Jenkins build failed at the `Tools` stage (`mkdir /home/jenkins/go/pkg: permission denied`)
- **Issue:** Named volumes with no matching path in the base image are created root-owned on first mount; the SSH-connected build user is `jenkins` (uid 1000), so `go install` writes into the module cache failed.
- **Fix:** `entrypoint.sh` now `chown`s `/home/jenkins/go` and `/home/jenkins/.cache` to `jenkins:jenkins` before starting sshd.
- **Files modified:** `deploy/ci/agent/entrypoint.sh`
- **Verification:** Re-ran the pipeline; `Tools`/`Lint`/`Unit`/`Integration`/`Images` all passed
- **Commit:** `2513c38`

**4. [Rule 1 - Bug] `smoke.sh` curl calls need `-g` (globoff)**
- **Found during:** Task 1 verification — `deploy/ci/smoke.sh` ran to its full poll budget without ever detecting an already-`SUCCESS` build
- **Issue:** The Jenkins `tree=result,building,actions[lastBuiltRevision[SHA1]]` query parameter contains literal `[`/`]`, which curl's default URL-globbing parses as a range/list and rejects with "bad range in URL" — every poll silently returned empty.
- **Fix:** Added `-g` to the shared `auth_curl` helper.
- **Files modified:** `deploy/ci/smoke.sh`
- **Verification:** `deploy/ci/smoke.sh` now returns `PASS jenkins-build SUCCESS` immediately when a matching build already exists
- **Commit:** `bae5291`

---

**Total deviations:** 4 auto-fixed (2 bug, 1 missing-critical, 1 config-path hardening)
**Impact on plan:** All four were necessary for the plan's own acceptance criteria (a genuinely green, testcontainers-capable Jenkins build) to actually hold up under real execution rather than only on paper. No scope creep — no files outside the plan's declared CI surface were touched.

## Issues Encountered
- This sandbox's Docker Desktop-for-Linux install has a pre-existing, unrelated broken `pass`/GPG credential store, which makes a bare `docker login` on the *host* fail to persist credentials. This is a local-machine artifact, not a project bug: the actual `Jenkinsfile` `Push` stage runs `make push` **inside** the `jenkins-agent` container, which has its own fresh Docker CLI config (no credential-store configured) and is unaffected. Verified `make push` end-to-end via `docker compose exec jenkins-agent ... make push`, which succeeded and is representative of real CI execution.

## User Setup Required
None - no external service configuration required beyond what `make ci-keys`/`make ci-up` already automate. Real EC2 deployment reuses the same `deploy/ci/docker-compose.yml`; only `.env` needs its CI secrets generated on that host the same way.

## Next Phase Readiness
- `make ci-up` brings up a working Jenkins + Harbor CI stack locally and (unchanged) on the single EC2 target; `deploy/ci/smoke.sh` / `--harbor` give a scriptable green/red signal
- Plan 13 (per this plan's own objective note) still needs to: narrow builds to changed services (D-22's path-based rebuild rule), add web/proto lint gates, and fold everything into one `make ci` target
- No blockers for proceeding to the next plan in this phase

## Self-Check: PASSED

All 11 key files confirmed present on disk; all 4 task/fix commits confirmed in `git log`.

---
*Phase: 01-platform-foundation*
*Completed: 2026-09-26*
