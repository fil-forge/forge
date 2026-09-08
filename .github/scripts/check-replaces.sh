#!/usr/bin/env bash
# Every in-repo require needs a matching replace.
#
# Without one, github.com/fil-forge/forge/<svc> resolves through the module
# proxy to this repository's root instead of the sibling directory, and fails
# as "module found but does not contain package" — or, worse, silently
# resolves to whatever the proxy holds. See forge-consolidation-plan.md,
# Traps #4.
set -euo pipefail

cd "$(dirname "$0")/../.."

status=0
for mod in $(find . -name go.mod -not -path './.git/*' | sort); do
  dir=$(dirname "$mod")
  self=$(cd "$dir" && go mod edit -json | jq -r '.Module.Path')

  # Match this repo's own module namespace only. A bare prefix test would
  # also catch sibling repos like github.com/fil-forge/forgectl.
  requires=$(cd "$dir" && go mod edit -json |
    jq -r '.Require // [] | .[] | .Path
           | select(. == "github.com/fil-forge/forge"
                    or startswith("github.com/fil-forge/forge/"))')

  for req in $requires; do
    # A module requiring a path under its own is not an in-repo sibling edge.
    [ "$req" = "$self" ] && continue

    replaced=$(cd "$dir" && go mod edit -json |
      jq -r --arg r "$req" '.Replace // [] | .[] | select(.Old.Path == $r) | .New.Path')

    if [ -z "$replaced" ]; then
      echo "FAIL $dir: requires $req with no replace directive"
      status=1
    else
      echo "ok   $dir: $req => $replaced"
    fi
  done
done

if [ "$status" -eq 0 ]; then
  echo "All in-repo requires have a matching replace."
fi
exit "$status"
