SHELL := bash
-include .env
export

M := github.com/chonlatee11/boat-booking
COMPOSE := docker compose --env-file .env -f deploy/docker-compose.yml -f deploy/docker-compose.services.yml
COMPOSE_CI := docker compose --env-file .env -f deploy/ci/docker-compose.yml

# BASE is the diff base for changed-services.sh: prefer origin/main, fall
# back to local main when no origin remote/tracking ref exists (D-22).
BASE ?= $(shell git rev-parse --verify origin/main >/dev/null 2>&1 && echo origin/main || echo main)
# SERVICES defaults to whatever changed-services.sh selects for BASE, so
# `make images`/`make push` only touch services actually affected by the
# diff; override explicitly (e.g. SERVICES=gateway) when needed.
SERVICES ?= $(shell deploy/ci/changed-services.sh $(BASE))
TAG ?= $(shell git rev-parse --short HEAD)

PORT ?= 8080
CATALOG_PORT ?= 8090
topic ?= catalog.events

.PHONY: dev-keys dev-token up up-infra obs-check down kong-roundtrip proof auth-roundtrip compose-gen test test-integration dev-tools lint hooks proto-gen proto-check sqlc-gen ci-keys ci-up ci-down images new-service template-smoke migrate-% migrate-validate web-check ci run-% dlq-list

dev-keys:
	go run $(M)/pkg/auth/cmd/devtoken keys

deploy/kong/kong.yml: deploy/kong/kong.yml.tmpl .env
	go run $(M)/pkg/auth/cmd/devtoken kong -in deploy/kong/kong.yml.tmpl -out deploy/kong/kong.yml

dev-token:
	go run $(M)/pkg/auth/cmd/devtoken token $(ARGS)

# compose-gen renders deploy/docker-compose.services.yml (git-ignored) from
# deploy/services.txt + deploy/compose/service.yml.tmpl — one migrate-<svc> +
# <svc> block per listed service (D-24, D-26).
deploy/docker-compose.services.yml: deploy/services.txt deploy/compose/service.yml.tmpl
	@echo "services:" > $@
	@for svc in $$(grep -v '^[[:space:]]*#' deploy/services.txt | grep -v '^[[:space:]]*$$'); do \
		sed "s/__NAME__/$$svc/g" deploy/compose/service.yml.tmpl >> $@; \
	done

compose-gen: deploy/docker-compose.services.yml

# `docker compose up --wait` only accepts "running|healthy" as done, so a
# by-design one-shot container that exits 0 (postgres-init, redpanda-init,
# migrate-<svc>, D-26) makes --wait itself fail even though everything
# actually succeeded. wait_ready polls `ps -a` instead: exited-with-0 is
# treated as done, exited-nonzero or unhealthy fails fast, anything still
# starting is retried until WAIT_TIMEOUT (default 180s).
define wait_ready
	@deadline=$$(( $$(date +%s) + $${WAIT_TIMEOUT:-180} )); \
	while :; do \
		bad=""; unready=""; \
		while IFS= read -r row; do \
			[ -z "$$row" ] && continue; \
			state=$$(echo "$$row" | jq -r .State); \
			health=$$(echo "$$row" | jq -r .Health); \
			exitcode=$$(echo "$$row" | jq -r .ExitCode); \
			svc=$$(echo "$$row" | jq -r .Service); \
			if [ "$$state" = "running" ]; then \
				case "$$health" in ""|healthy) ;; *) unready="$$unready $$svc" ;; esac; \
			elif [ "$$state" = "exited" ]; then \
				[ "$$exitcode" != "0" ] && bad="$$bad $$svc(exit $$exitcode)"; \
			else \
				unready="$$unready $$svc($$state)"; \
			fi; \
		done < <($(COMPOSE) ps -a --format json); \
		if [ -n "$$bad" ]; then echo "wait_ready: failed:$$bad" >&2; $(COMPOSE) ps -a; exit 1; fi; \
		if [ -z "$$unready" ]; then echo "wait_ready: all services running/healthy or completed successfully"; break; fi; \
		if [ "$$(date +%s)" -ge "$$deadline" ]; then echo "wait_ready: timed out waiting for:$$unready" >&2; $(COMPOSE) ps -a; exit 1; fi; \
		sleep 2; \
	done
endef

up: dev-keys deploy/kong/kong.yml compose-gen
	$(COMPOSE) --profile app --profile web up -d --build
	$(call wait_ready)

up-infra: dev-keys compose-gen
	$(COMPOSE) up -d
	$(call wait_ready)

obs-check:
	deploy/observability/check.sh all

down: compose-gen
	$(COMPOSE) --profile app --profile web --profile tools down

kong-roundtrip:
	deploy/kong/roundtrip.sh

proof:
	deploy/proof.sh

auth-roundtrip:
	deploy/auth-roundtrip.sh

# run-% runs a service from the host (go run) against the up-infra stack,
# with hosts overridden to localhost (D-17, D-19). Services listed in
# deploy/services.txt are DB/Kafka-backed; every other service (gateway) has
# neither and instead needs CATALOG_URL. The Collector is not host-exposed
# (D-38), so OTEL_EXPORTER_OTLP_ENDPOINT is deliberately left unset.
run-%:
	@if grep -qx "$*" deploy/services.txt 2>/dev/null; then \
		DATABASE_URL="postgres://$*:$(SERVICE_DB_PASSWORD)@localhost:5432/$*?sslmode=disable" \
		KAFKA_BROKERS="localhost:19092" \
		LOG_FORMAT=text HTTP_ADDR=":$(PORT)" \
		go run $(M)/services/$*/cmd; \
	else \
		CATALOG_URL="http://localhost:$(CATALOG_PORT)" \
		LOG_FORMAT=text HTTP_ADDR=":$(PORT)" \
		go run $(M)/services/$*/cmd; \
	fi

# migrate-% runs that service's migrations manually against the compose
# infra (D-26) — the service binary never auto-migrates.
migrate-%: compose-gen
	$(COMPOSE) --profile app run --rm migrate-$*

# dlq-list prints (and exits after) whatever is on <topic>.dlq, including the
# error/consumer_group/attempts headers a failed handler attaches (D-12).
dlq-list:
	$(COMPOSE) exec -T redpanda rpk topic consume $(topic).dlq --offset :end

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

# new-service scaffolds services/$(name) from services/_template (D-03).
# Every guard below must exit 1 before anything touches the filesystem.
new-service:
	@if [ -z "$(name)" ]; then \
		echo "new-service: usage: make new-service name=<svc>" >&2; \
		exit 1; \
	fi
	@echo "$(name)" | grep -Eq '^[a-z][a-z0-9]*$$' || { \
		echo "new-service: name '$(name)' must be lowercase ASCII, start with a letter, letters/digits only" >&2; \
		exit 1; \
	}
	@if [ -e "services/$(name)" ]; then \
		echo "new-service: services/$(name) already exists" >&2; \
		exit 1; \
	fi
	@if [ -f deploy/services.txt ] && grep -qx "$(name)" deploy/services.txt; then \
		echo "new-service: '$(name)' is already listed in deploy/services.txt" >&2; \
		exit 1; \
	fi
	cp -r services/_template services/$(name)
	grep -rl __NAME__ services/$(name) | xargs sed -i 's/__NAME__/$(name)/g'
	go work use ./services/$(name)
	mkdir -p deploy
	echo "$(name)" >> deploy/services.txt
	$(MAKE) compose-gen
	@echo "new-service: services/$(name) is ready — cd services/$(name) and replace the sample slice (see CLAUDE.md)"

# template-smoke proves a freshly scaffolded service builds into a container
# image that reports Docker health status healthy (D-03, D-37), then removes
# every trace it left behind, success or failure.
template-smoke:
	@set -eu; \
	trap 'docker rm -f bb-tsmoke >/dev/null 2>&1 || true; \
	      rm -rf services/tsmoke; \
	      go work edit -dropuse=./services/tsmoke 2>/dev/null || true; \
	      [ -f deploy/services.txt ] && sed -i "/^tsmoke$$/d" deploy/services.txt || true' EXIT; \
	for bad in "" "Bad_Name" "9x" "catalog"; do \
		before=$$(git status --porcelain); \
		if [ -z "$$bad" ]; then \
			if $(MAKE) new-service >/dev/null 2>&1; then \
				echo "template-smoke: new-service with no name should have failed" >&2; \
				exit 1; \
			fi; \
		else \
			if $(MAKE) new-service name="$$bad" >/dev/null 2>&1; then \
				echo "template-smoke: new-service name=$$bad should have failed" >&2; \
				exit 1; \
			fi; \
		fi; \
		after=$$(git status --porcelain); \
		if [ "$$before" != "$$after" ]; then \
			echo "template-smoke: new-service name='$$bad' left the tree dirty" >&2; \
			exit 1; \
		fi; \
	done; \
	$(MAKE) new-service name=tsmoke; \
	go build github.com/chonlatee11/boat-booking/services/tsmoke/...; \
	docker build --build-arg SERVICE=tsmoke -t boatbooking/tsmoke:smoke .; \
	docker run -d --name bb-tsmoke --health-interval=2s \
		-e INTERNAL_TOKEN=smoke -e RELAY_ENABLED=false -e CONSUMER_ENABLED=false \
		boatbooking/tsmoke:smoke; \
	deadline=$$(( $$(date +%s) + 60 )); \
	status=""; \
	while [ $$(date +%s) -lt $$deadline ]; do \
		status=$$(docker inspect -f '{{.State.Health.Status}}' bb-tsmoke 2>/dev/null || echo ""); \
		[ "$$status" = "healthy" ] && break; \
		sleep 2; \
	done; \
	if [ "$$status" != "healthy" ]; then \
		echo "template-smoke: bb-tsmoke never reported healthy (last status: $$status)" >&2; \
		exit 1; \
	fi; \
	echo "PASS template-smoke"

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
	test -d apps/web/node_modules || npm --prefix apps/web ci
	npm --prefix apps/web run lint
	npm --prefix apps/web run typecheck

hooks:
	lefthook install

ci-keys: dev-keys
	deploy/ci/ci-keys.sh

ci-up: ci-keys
	deploy/ci/harbor-prepare.sh
	$(COMPOSE_CI) up -d --build --wait
	@HARBOR_ADMIN_PASSWORD=$$(grep -E '^HARBOR_ADMIN_PASSWORD=' .env | tail -1 | cut -d= -f2-); \
	STATUS=$$(printf 'user = "admin:%s"\n' "$$HARBOR_ADMIN_PASSWORD" | curl -s -o /dev/null -w '%{http_code}' -K - \
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
	@if [ -z "$(strip $(SERVICES))" ]; then \
		echo "no services changed"; \
	else \
		for s in $(SERVICES); do \
			name=$${s#_}; \
			echo "building services/$$s -> boatbooking/$$name:$(TAG)"; \
			docker build --build-arg SERVICE=$$s -t boatbooking/$$name:$(TAG) .; \
		done; \
	fi

# migrate-validate runs `goose validate` against every service's migrations/
# dir (D-26) using the same goose version pinned in deploy/migrate/Dockerfile
# and pkg/go.mod (v3.28.0 requires go1.26, breaking this repo's go1.25.x pin).
migrate-validate:
	@for d in services/*/; do \
		svc=$$(basename "$$d"); \
		if [ -d "$${d}migrations" ]; then \
			echo "goose validate services/$$svc"; \
			go run github.com/pressly/goose/v3/cmd/goose@v3.27.3 -dir "$${d}migrations" validate || exit 1; \
		fi; \
	done

web-check:
	npm --prefix apps/web run format:check
	npm --prefix apps/web run build

# ci reproduces the Jenkins pipeline locally in the same order, stopping at
# the first failure; no flag here ever skips test-integration (D-22).
ci:
	bash deploy/ci/changed-services_test.sh
	$(MAKE) lint
	$(MAKE) proto-check
	$(MAKE) test
	$(MAKE) test-integration
	$(MAKE) migrate-validate
	$(MAKE) template-smoke
	$(MAKE) web-check
	$(MAKE) images

REGISTRY ?= localhost:8880/boatbooking
HARBOR_USER ?= admin
HARBOR_PASSWORD ?= $(HARBOR_ADMIN_PASSWORD)

push:
	@printf '%s' "$$HARBOR_PASSWORD" | docker login localhost:8880 -u "$(HARBOR_USER)" --password-stdin
	@for s in $(SERVICES); do \
		name=$${s#_}; \
		docker tag boatbooking/$$name:$(TAG) $(REGISTRY)/$$name:$(TAG); \
		docker push $(REGISTRY)/$$name:$(TAG); \
	done
