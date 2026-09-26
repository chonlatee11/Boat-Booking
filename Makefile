SHELL := bash
-include .env
export

M := github.com/chonlatee11/boat-booking
COMPOSE := docker compose --env-file .env -f deploy/docker-compose.yml
COMPOSE_CI := docker compose --env-file .env -f deploy/ci/docker-compose.yml

# Every service directory that has been scaffolded with `make new-service`
# (has cmd/main.go). services/_template has no cmd/main.go and is excluded
# until D-03 gives it one.
SERVICES ?= $(shell for d in services/*/; do n=$$(basename "$$d"); [ -f "$${d}cmd/main.go" ] && echo "$$n"; done)
TAG ?= $(shell git rev-parse --short HEAD)

.PHONY: dev-keys dev-token up up-infra obs-check down kong-roundtrip test test-integration dev-tools lint hooks proto-gen proto-check sqlc-gen ci-keys ci-up ci-down images

dev-keys:
	go run $(M)/pkg/auth/cmd/devtoken keys

deploy/kong/kong.yml: deploy/kong/kong.yml.tmpl .env
	go run $(M)/pkg/auth/cmd/devtoken kong -in deploy/kong/kong.yml.tmpl -out deploy/kong/kong.yml

dev-token:
	go run $(M)/pkg/auth/cmd/devtoken token $(ARGS)

up: dev-keys deploy/kong/kong.yml
	$(COMPOSE) --profile app --profile web up -d --build --wait

up-infra: dev-keys
	$(COMPOSE) up -d --wait

obs-check:
	deploy/observability/check.sh all

down:
	$(COMPOSE) --profile app --profile web --profile tools down

kong-roundtrip:
	deploy/kong/roundtrip.sh

test:
	go test $$(go list -m -f '{{.Path}}/...')

test-integration:
	go test -tags=integration -count=1 -timeout 15m $$(go list -m -f '{{.Path}}/...')

dev-tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
	go install github.com/evilmartians/lefthook/v2@v2.1.14
	go install golang.org/x/tools/cmd/goimports@v0.50.0
	go install github.com/bufbuild/buf/cmd/buf@v1.73.0

proto-gen:
	buf generate
	cd gen/go && go mod tidy

sqlc-gen:
	@for f in services/*/internal/adapters/postgres/sqlc.yaml; do \
		echo "sqlc generate -f $$f"; \
		docker run --rm -u $$(id -u):$$(id -g) -v "$(CURDIR)":/src -w /src sqlc/sqlc:1.31.1 generate -f $$f; \
	done

proto-check:
	buf lint
	@if git cat-file -e main:buf.yaml 2>/dev/null; then \
		buf breaking --against '.git#branch=main'; \
	else \
		echo "proto-check: main has no buf.yaml yet, breaking check skipped"; \
	fi
	proto/pii-check.sh proto/events
	$(MAKE) proto-gen
	@if [ -n "$$(git status --porcelain --untracked-files=all -- gen/)" ]; then \
		echo "proto-check: gen/ is stale — run 'make proto-gen' and commit the result"; \
		git status --porcelain --untracked-files=all -- gen/; \
		exit 1; \
	fi

lint:
	for dir in $$(go list -m -f '{{.Dir}}'); do \
		case "$$dir" in */gen/go) continue ;; esac; \
		(cd "$$dir" && golangci-lint run ./...) || exit 1; \
	done
	buf lint
	proto/pii-check.sh proto/events

hooks:
	lefthook install

ci-keys: dev-keys
	deploy/ci/ci-keys.sh

ci-up: ci-keys
	deploy/ci/harbor-prepare.sh
	$(COMPOSE_CI) up -d --build --wait
	@HARBOR_ADMIN_PASSWORD=$$(grep -E '^HARBOR_ADMIN_PASSWORD=' .env | tail -1 | cut -d= -f2-); \
	STATUS=$$(curl -s -o /dev/null -w '%{http_code}' -u "admin:$$HARBOR_ADMIN_PASSWORD" \
		-X POST http://localhost:8880/api/v2.0/projects \
		-H 'Content-Type: application/json' \
		-d '{"project_name":"boatbooking","metadata":{"public":"false"}}'); \
	case "$$STATUS" in \
		201|409) ;; \
		*) echo "harbor project create failed: HTTP $$STATUS" >&2; exit 1 ;; \
	esac

ci-down:
	$(COMPOSE_CI) down

images:
	@for s in $(SERVICES); do \
		name=$${s#_}; \
		echo "building services/$$s -> boatbooking/$$name:$(TAG)"; \
		docker build --build-arg SERVICE=$$s -t boatbooking/$$name:$(TAG) .; \
	done

REGISTRY ?= localhost:8880/boatbooking
HARBOR_USER ?= admin
HARBOR_PASSWORD ?= $(HARBOR_ADMIN_PASSWORD)

push:
	@echo "$(HARBOR_PASSWORD)" | docker login localhost:8880 -u "$(HARBOR_USER)" --password-stdin
	@for s in $(SERVICES); do \
		name=$${s#_}; \
		docker tag boatbooking/$$name:$(TAG) $(REGISTRY)/$$name:$(TAG); \
		docker push $(REGISTRY)/$$name:$(TAG); \
	done
