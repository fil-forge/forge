#!/usr/bin/env bash
# Every in-repo require needs a matching replace, and no module may still
# require a service this repository now contains under its old upstream path.
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

# A module that came in by subtree keeps the go.mod it had upstream, and a
# nested one (hilt/itest, ingot/itest) is not reached by a sweep over the
# service's own go.mod. It then requires github.com/fil-forge/<svc> from the
# proxy while its .go files import github.com/fil-forge/forge/<svc>, and the
# check above cannot see it: the stale path is not in this repo's namespace,
# so there is nothing there to want a replace. It fails at `go vet` as a
# missing go.sum entry, which names the package and not the cause.
#
# Build the set of services this repository actually contains, then look for
# anyone still pointing at their upstream.
declare -A in_repo=()
for mod in $(find . -name go.mod -not -path './.git/*' | sort); do
  path=$(cd "$(dirname "$mod")" && go mod edit -json | jq -r '.Module.Path')
  case "$path" in
    github.com/fil-forge/forge/*)
      # Only the top segment: hilt/itest is github.com/fil-forge/forge/hilt/itest,
      # and there is no github.com/fil-forge/hilt/itest upstream to confuse it with.
      in_repo["${path#github.com/fil-forge/forge/}"]=1
      ;;
  esac
done

for mod in $(find . -name go.mod -not -path './.git/*' | sort); do
  dir=$(dirname "$mod")
  stale=$(cd "$dir" && go mod edit -json |
    jq -r '.Require // [] | .[] | .Path
           | select(startswith("github.com/fil-forge/"))
           | select(. != "github.com/fil-forge/forge")
           | select(startswith("github.com/fil-forge/forge/") | not)')

  for req in $stale; do
    svc="${req#github.com/fil-forge/}"
    if [ -n "${in_repo[$svc]:-}" ]; then
      echo "FAIL $dir: requires $req, but $svc is in this repo as github.com/fil-forge/forge/$svc"
      status=1
    fi
  done
done

if [ "$status" -eq 0 ]; then
  echo "All in-repo requires have a matching replace, and none name an upstream we now contain."
fi
exit "$status"
