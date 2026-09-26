#!/usr/bin/env bash
# `docker compose group_add` only affects this container's PID 1 credentials;
# sshd forks a fresh login session for the "jenkins" user via PAM, which
# resets supplementary groups from /etc/group and drops that GID (Pitfall
# 12). Fix membership against the real docker.sock GID before sshd starts so
# testcontainers works over the SSH session Jenkins actually builds in.
set -euo pipefail

# Named volumes with no matching path in the image are created root-owned;
# the SSH build user is "jenkins" (uid 1000), so fix ownership before sshd
# starts or `go install`/module-cache writes fail with permission denied.
mkdir -p /home/jenkins/go /home/jenkins/.cache
chown jenkins:jenkins /home/jenkins/go /home/jenkins/.cache

DOCKER_SOCK=/var/run/docker.sock
if [ -S "$DOCKER_SOCK" ]; then
  SOCK_GID=$(stat -c %g "$DOCKER_SOCK")
  if getent group "$SOCK_GID" >/dev/null 2>&1; then
    DOCKER_GROUP=$(getent group "$SOCK_GID" | cut -d: -f1)
  else
    DOCKER_GROUP=docker
    if getent group docker >/dev/null 2>&1; then
      groupmod -g "$SOCK_GID" docker
    else
      groupadd -g "$SOCK_GID" docker
    fi
  fi
  usermod -aG "$DOCKER_GROUP" jenkins
fi

exec setup-sshd "$@"
