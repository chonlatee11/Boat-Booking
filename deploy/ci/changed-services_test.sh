#!/usr/bin/env bash
# Table-driven test of changed-services.sh --stdin.
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

SELECTOR=deploy/ci/changed-services.sh
fail=0

# name | input paths (newline-separated) | expected output (newline-separated)
run_case() {
	local name="$1" input="$2" expected="$3"
	local actual
	actual=$(printf '%s' "$input" | "$SELECTOR" --stdin)
	if [ "$actual" = "$expected" ]; then
		echo "PASS $name"
	else
		echo "FAIL $name"
		echo "  expected: $(printf '%s' "$expected" | tr '\n' ',')"
		echo "  actual:   $(printf '%s' "$actual" | tr '\n' ',')"
		fail=1
	fi
}

ALL_SORTED=$(printf 'catalog\ngateway\nschedule\n_template' | LC_ALL=C sort)

run_case "single service file" \
	"services/catalog/a.go" \
	"catalog"

run_case "adjacency edge (catalogx != catalog)" \
	"services/catalogx/a.go" \
	"catalogx"

run_case "same service twice dedupes" \
	"$(printf 'services/catalog/a.go\nservices/catalog/b.go')" \
	"catalog"

run_case "two services sorted" \
	"$(printf 'services/schedule/x\nservices/catalog/y')" \
	"$(printf 'catalog\nschedule')"

run_case "pkg/ change selects all" \
	"pkg/kafka/consumer.go" \
	"$ALL_SORTED"

run_case "proto/ change selects all" \
	"proto/x.proto" \
	"$ALL_SORTED"

run_case "gen/ change selects all" \
	"gen/go/x.go" \
	"$ALL_SORTED"

run_case "_template change selects all" \
	"services/_template/cmd/main.go" \
	"$ALL_SORTED"

run_case "go.work change selects all" \
	"go.work" \
	"$ALL_SORTED"

run_case "Makefile change selects all" \
	"Makefile" \
	"$ALL_SORTED"

run_case "Dockerfile change selects all" \
	"Dockerfile" \
	"$ALL_SORTED"

run_case "docs-only change selects nothing (empty edge)" \
	".planning/notes.md" \
	""

run_case "web-only change selects nothing (checked, not imaged)" \
	"apps/web/src/x.tsx" \
	""

exit $fail
