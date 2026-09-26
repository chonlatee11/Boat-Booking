# syntax=docker/dockerfile:1
ARG SERVICE

FROM golang:1.25.14-bookworm AS build
ARG SERVICE
ENV CGO_ENABLED=0 GOWORK=off
WORKDIR /src
COPY gen/go ./gen/go
COPY pkg ./pkg
COPY services/${SERVICE} ./services/${SERVICE}
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd services/${SERVICE} && go build -trimpath -o /out/service ./cmd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/service /service
USER nonroot
ENTRYPOINT ["/service"]
HEALTHCHECK CMD ["/service", "-healthcheck"]
