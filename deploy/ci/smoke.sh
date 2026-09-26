#!/usr/bin/env bash
# Verifies Jenkins built and tested the current branch's HEAD commit
# (SUCCESS), triggering a scan/build if needed. Prints "PASS jenkins-build
# SUCCESS" and exits 0 only on a genuine green build of HEAD.
#
# `deploy/ci/smoke.sh --harbor` instead verifies Harbor is reachable and the
# boatbooking project exists (D-21); set EXPECT_TAG to also require a
# gateway:<tag> artifact (D-22).
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

# .env values are plain KEY=value (not shell-quoted — some, like the SSH
# pubkey, contain spaces) so read only the keys we need instead of sourcing
# the whole file as a shell script.
env_get() { grep -E "^$1=" .env | tail -1 | cut -d= -f2-; }

if [ "${1:-}" = "--harbor" ]; then
  HARBOR_URL=http://localhost:8880
  HARBOR_ADMIN_PASSWORD=$(env_get HARBOR_ADMIN_PASSWORD)
  : "${HARBOR_ADMIN_PASSWORD:?HARBOR_ADMIN_PASSWORD is empty — run 'make ci-up' first}"

  PING=$(curl -fsS "$HARBOR_URL/api/v2.0/ping" || true)
  if [ "$PING" != "Pong" ]; then
    echo "FAIL harbor-ping got: ${PING:-<empty>}"
    exit 1
  fi

  PROJECT_CODE=$(curl -s -o /dev/null -w '%{http_code}' -u "admin:$HARBOR_ADMIN_PASSWORD" \
    "$HARBOR_URL/api/v2.0/projects?name=boatbooking")
  if [ "$PROJECT_CODE" != "200" ]; then
    echo "FAIL harbor-project boatbooking missing (HTTP $PROJECT_CODE)"
    exit 1
  fi

  if [ -n "${EXPECT_TAG:-}" ]; then
    ARTIFACTS=$(curl -fsS -u "admin:$HARBOR_ADMIN_PASSWORD" -g \
      "$HARBOR_URL/api/v2.0/projects/boatbooking/repositories/gateway/artifacts?with_tag=true")
    if ! echo "$ARTIFACTS" | grep -q "\"name\":\"$EXPECT_TAG\""; then
      echo "FAIL harbor-artifact gateway:$EXPECT_TAG not found"
      exit 1
    fi
  fi

  echo "PASS harbor-ping Pong"
  exit 0
fi

JENKINS_URL=http://localhost:8080
ADMIN_USER=admin
JENKINS_ADMIN_PASSWORD=$(env_get JENKINS_ADMIN_PASSWORD)
: "${JENKINS_ADMIN_PASSWORD:?JENKINS_ADMIN_PASSWORD is empty — run 'make ci-keys' first}"

COOKIE_JAR=$(mktemp)
trap 'rm -f "$COOKIE_JAR"' EXIT

auth_curl() {
  # -g (globoff): the Jenkins tree=... query params below contain literal
  # "[" "]", which curl otherwise parses as URL globbing syntax.
  curl -fsS -g -u "$ADMIN_USER:$JENKINS_ADMIN_PASSWORD" -c "$COOKIE_JAR" -b "$COOKIE_JAR" "$@"
}

# Multibranch encodes "/" in a branch name as "%2F"; nesting that inside
# another job path segment requires encoding it again ("%252F").
url_encode_branch() {
  local encoded=${1//\//%2F}
  printf '%s' "$encoded" | sed 's/%/%25/g'
}

# Wait for the Jenkins API to answer.
JENKINS_UP=false
for _ in $(seq 1 60); do
  if auth_curl -o /dev/null "$JENKINS_URL/api/json" 2>/dev/null; then
    JENKINS_UP=true
    break
  fi
  sleep 5
done
if [ "$JENKINS_UP" != true ]; then
  echo "FAIL jenkins-unreachable"
  exit 1
fi

CRUMB_JSON=$(auth_curl "$JENKINS_URL/crumbIssuer/api/json")
CRUMB=$(echo "$CRUMB_JSON" | grep -o '"crumb":"[^"]*"' | cut -d'"' -f4)
CRUMB_FIELD=$(echo "$CRUMB_JSON" | grep -o '"crumbRequestField":"[^"]*"' | cut -d'"' -f4)

# Trigger a repository scan so new commits/branches are picked up now,
# rather than waiting for the 2-minute periodicFolderTrigger.
auth_curl -X POST -H "$CRUMB_FIELD: $CRUMB" "$JENKINS_URL/job/boat-booking/build?delay=0" -o /dev/null

BRANCH=$(git rev-parse --abbrev-ref HEAD)
HEAD_SHA=$(git rev-parse HEAD)
ENCODED_BRANCH=$(url_encode_branch "$BRANCH")
JOB_URL="$JENKINS_URL/job/boat-booking/job/$ENCODED_BRANCH"

DEADLINE=$((SECONDS + 40 * 60))
TRIGGERED_BRANCH_BUILD=false
SHA=""
RESULT=""
while [ "$SECONDS" -lt "$DEADLINE" ]; do
  RESP=$(auth_curl "$JOB_URL/lastBuild/api/json?tree=result,building,actions[lastBuiltRevision[SHA1]]" 2>/dev/null || echo "")
  if [ -n "$RESP" ]; then
    SHA=$(echo "$RESP" | grep -o '"SHA1":"[a-f0-9]*"' | head -1 | cut -d'"' -f4)
    BUILDING=$(echo "$RESP" | grep -o '"building":[a-z]*' | head -1 | cut -d: -f2)
    RESULT=$(echo "$RESP" | grep -o '"result":"[A-Z]*"' | head -1 | cut -d'"' -f4)
    if [ "$SHA" = "$HEAD_SHA" ]; then
      if [ "$BUILDING" = "false" ] && [ -n "$RESULT" ]; then
        break
      fi
    elif [ "$TRIGGERED_BRANCH_BUILD" = false ]; then
      auth_curl -X POST -H "$CRUMB_FIELD: $CRUMB" "$JOB_URL/build?delay=0" -o /dev/null 2>/dev/null || true
      TRIGGERED_BRANCH_BUILD=true
    fi
  fi
  sleep 10
done

if [ "$RESULT" = "SUCCESS" ] && [ "$SHA" = "$HEAD_SHA" ]; then
  # A green build must have actually run the integration suite and the
  # template smoke test -- not just skipped straight to SUCCESS (T-13-02).
  CONSOLE=$(auth_curl "$JOB_URL/lastBuild/consoleText")
  MISSING=""
  echo "$CONSOLE" | grep -q "make test-integration" || MISSING="$MISSING make-test-integration"
  echo "$CONSOLE" | grep -q "PASS template-smoke" || MISSING="$MISSING PASS-template-smoke"
  if [ -n "$MISSING" ]; then
    echo "FAIL jenkins-build-console missing:$MISSING"
    exit 1
  fi
  echo "PASS jenkins-build SUCCESS"
  exit 0
fi
echo "FAIL jenkins-build result=${RESULT:-none} sha=${SHA:-none} expected=$HEAD_SHA"
exit 1
