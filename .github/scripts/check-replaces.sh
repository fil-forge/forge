#!/usr/bin/env bash
# Every github.com/fil-forge/forge/* module that an in-repo go.mod requires must
# be resolved by a replace directive pointing at the sibling directory that
# holds it. Without one, Go resolves the path through the module proxy — which
# finds the repository root, not a module — and the failure is either
# confusing or, worse, absent. This check runs before anything else in CI.
set -euo pipefail

fail=0
while IFS= read -r gm; do
  dir=$(dirname "$gm")
  json=$(cd "$dir" && go mod edit -json)
  for req in $(jq -r '.Require[]?.Path | select(startswith("github.com/fil-forge/forge/"))' <<<"$json"); do
    target=$(jq -r --arg p "$req" '.Replace[]? | select(.Old.Path == $p) | .New.Path' <<<"$json")
    if [ -z "$target" ]; then
      echo "$gm: requires $req with no replace directive"
      fail=1
      continue
    fi
    if [ ! -f "$dir/$target/go.mod" ]; then
      echo "$gm: replace for $req points at $target, which has no go.mod"
      fail=1
      continue
    fi
    got=$(cd "$dir/$target" && go mod edit -json | jq -r .Module.Path)
    if [ "$got" != "$req" ]; then
      echo "$gm: $target is module $got, not $req"
      fail=1
    fi
  done
done < <(git ls-files '*/go.mod')

if [ "$fail" = 0 ]; then
  echo "every in-repo require has a matching replace"
fi
exit "$fail"
