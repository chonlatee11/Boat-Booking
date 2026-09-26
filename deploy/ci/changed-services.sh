#!/usr/bin/env bash
# changed-services.sh <base-ref>   -- lists changed paths via
#   git diff --name-only <base-ref>...HEAD
# changed-services.sh --stdin      -- reads changed paths from stdin (tests)
#
# Any path matching pkg/, proto/, gen/, services/_template/, or exactly
# go.work, go.work.sum, Makefile, Dockerfile selects ALL services (every
# directory under services/ with cmd/main.go, including _template, D-03).
# Otherwise a path matching services/<name>/... selects <name> (exact path
# segment -- services/catalogx/ never selects catalog, PLAT-08 adjacency
# edge). Prints selected names one per line, sorted and unique; nothing
# selected -> prints nothing, exit 0.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

# LC_ALL=C keeps sort order identical across dev-host locales (en_US.UTF-8)
# and the CI agent's likely C/POSIX locale -- without it "_template" sorts
# before or after "catalog" depending on the machine's collation rules.
all_services() {
	for d in services/*/; do
		n=$(basename "$d")
		[ -f "${d}cmd/main.go" ] && echo "$n"
	done
}

if [ "${1:-}" = "--stdin" ]; then
	paths=$(cat)
else
	base_ref="${1:?usage: changed-services.sh <base-ref>|--stdin}"
	if git rev-parse --verify "$base_ref" >/dev/null 2>&1; then
		paths=$(git diff --name-only "$base_ref"...HEAD)
	else
		all_services | LC_ALL=C sort -u
		exit 0
	fi
fi

select_all=false
declare -A selected=()

while IFS= read -r path; do
	[ -z "$path" ] && continue
	case "$path" in
	pkg/* | proto/* | gen/* | services/_template/*)
		select_all=true
		;;
	go.work | go.work.sum | Makefile | Dockerfile)
		select_all=true
		;;
	services/*/*)
		name=${path#services/}
		name=${name%%/*}
		selected["$name"]=1
		;;
	esac
done <<<"$paths"

if [ "$select_all" = true ]; then
	all_services | LC_ALL=C sort -u
else
	for name in "${!selected[@]}"; do echo "$name"; done | LC_ALL=C sort -u
fi
