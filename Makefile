SHELL := bash
-include .env
export

M := github.com/chonlatee11/boat-booking
COMPOSE := docker compose --env-file .env -f deploy/docker-compose.yml

.PHONY: dev-keys dev-token up down kong-roundtrip test test-integration dev-tools lint hooks proto-gen proto-check

dev-keys:
	go run $(M)/pkg/auth/cmd/devtoken keys

deploy/kong/kong.yml: deploy/kong/kong.yml.tmpl .env
	go run $(M)/pkg/auth/cmd/devtoken kong -in deploy/kong/kong.yml.tmpl -out deploy/kong/kong.yml

dev-token:
	go run $(M)/pkg/auth/cmd/devtoken token $(ARGS)

up: dev-keys deploy/kong/kong.yml
	$(COMPOSE) --profile app --profile web up -d --build --wait

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
