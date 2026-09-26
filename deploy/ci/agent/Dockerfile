# Jenkins build agent: Go + Node + Docker CLI (BuildKit) + make/git/curl/jq,
# reachable over SSH from the controller. Runs with docker.sock mounted so
# testcontainers-based integration tests work (Pitfall 12, D-21).
FROM jenkins/ssh-agent:9.0.0-jdk21

RUN apt-get update && apt-get install -y --no-install-recommends make git curl jq \
    && rm -rf /var/lib/apt/lists/*

COPY --from=golang:1.25.9 /usr/local/go /usr/local/go
COPY --from=node:24-bookworm-slim /usr/local /usr/local
COPY --from=docker:28.1.1-cli /usr/local/bin/docker /usr/local/bin/docker
COPY --from=docker:28.1.1-cli /usr/local/libexec/docker/cli-plugins/docker-buildx /usr/local/libexec/docker/cli-plugins/docker-buildx

COPY entrypoint.sh /usr/local/bin/ci-agent-entrypoint.sh

ENV PATH="/usr/local/go/bin:${PATH}"
ENTRYPOINT ["/usr/local/bin/ci-agent-entrypoint.sh"]
