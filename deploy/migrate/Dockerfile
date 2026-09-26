# syntax=docker/dockerfile:1
# Small goose CLI image for migrate-<svc> one-shot containers (D-26): the
# service binary never auto-migrates, goose runs as a separate step before
# the service starts.
# goose pinned to v3.27.3, not the plan's illustrative v3.28.0: v3.28.0's own
# go.mod requires go1.26.0, breaking this repo's go1.25.x toolchain pin —
# same class of issue 01-04 hit with franz-go/goose (see STATE.md decisions).
FROM golang:1.25.9-alpine AS build
ENV CGO_ENABLED=0
RUN go install github.com/pressly/goose/v3/cmd/goose@v3.27.3

FROM alpine:3.22
COPY --from=build /go/bin/goose /usr/local/bin/goose
