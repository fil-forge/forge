#!/usr/bin/env bash
# Every actions/setup-go step must set cache-dependency-path.
#
# setup-go defaults to cache: true, but resolves go.sum from the repository
# root. This repo has no root go.sum -- each module carries its own -- so a
# step without an explicit path logs
#
#   Restore cache failed: Dependencies file is not found in
#   /home/runner/work/forge-2/forge-2. Supported file pattern: go.sum
#
# as a WARNING and then runs with no module cache whatsoever. Nothing fails,
# nothing is annotated red, and every job re-downloads its entire dependency
# graph on every run -- which is what turned an occasional proxy.golang.org
# stream error into a recurring red build.
#
# The failure mode is a silently-skipped optimisation, so only an explicit
# check catches it. Same reasoning as check-replaces.sh and
# check-image-lists.sh.
set -euo pipefail

cd "$(dirname "$0")/../.."

status=0
for wf in .github/workflows/*.yml; do
  # Walk each setup-go step and look ahead for cache-dependency-path, stopping
  # at the next step (a line starting "      - ").
  while IFS=: read -r lineno _; do
    # awk note: a bare `exit N` inside a rule still runs END, whose own exit
    # would override it. Set a flag and decide once, in END.
    if ! awk -v start="$lineno" '
      NR > start {
        if ($0 ~ /^      - /) { exit }        # next step reached: not found
        if ($0 ~ /cache-dependency-path:/) { found = 1; exit }
      }
      END { exit found ? 0 : 1 }
    ' "$wf"; then
      echo "$wf:$lineno: setup-go step without cache-dependency-path" >&2
      status=1
    fi
  done < <(grep -n 'uses: actions/setup-go@' "$wf" || true)
done

if [ "$status" -ne 0 ]; then
  echo >&2
  echo "Add 'cache-dependency-path: <module>/go.sum' to each step above." >&2
  echo "Without it setup-go silently disables the module cache." >&2
fi
exit "$status"
