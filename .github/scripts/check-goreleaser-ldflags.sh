#!/usr/bin/env bash
# Every -X ldflag in a .goreleaser.yaml must name a package in the module that
# config builds.
#
# The linker does not check this. `go build -ldflags "-X wrong/path.Var=1.2.3"`
# exits 0, prints no warning, and leaves Var at its declared default --
# verified on go1.24.7. So a stale import path here does not fail a release, it
# ships one: the binary reports whatever its build package declares as the
# fallback (v0.0.0 for piri, sprue and indexing-service; "dev" for ingot) while
# the tag and the GitHub release say otherwise.
#
# That is not hypothetical. Consolidation rewrote every module from
# github.com/fil-forge/<svc> to github.com/fil-forge/forge/<svc>, including the
# comment in each pkg/build/version.go naming the ldflag -- but not the
# .goreleaser.yaml that comment points at. Nothing builds these files today,
# because there is no release workflow yet, so nothing caught it.
set -euo pipefail

cd "$(dirname "$0")/../.."

status=0
found=0
while IFS= read -r config; do
  dir=$(dirname "$config")
  if [ ! -f "$dir/go.mod" ]; then
    echo "FAIL $config: no go.mod beside it, so there is no module path to check against"
    status=1
    continue
  fi
  module=$(cd "$dir" && go mod edit -json | jq -r '.Module.Path')

  while IFS= read -r symbol; do
    found=$((found + 1))
    pkg=${symbol%.*}
    # package main is addressed as "main", not by import path.
    if [ "$pkg" = "main" ]; then
      echo "ok   $config: -X $symbol (package main)"
      continue
    fi
    if [ "$pkg" = "$module" ] || [ "${pkg#"$module"/}" != "$pkg" ]; then
      echo "ok   $config: -X $symbol"
    else
      echo "FAIL $config: -X $symbol names a package outside $module"
      echo "     this ldflag silently does nothing; the released binary keeps its default"
      status=1
    fi
  done < <(grep -o -- '-X [^ ="]*=' "$config" | sed 's/^-X //; s/=$//')
done < <(find . \( -name '.goreleaser.yaml' -o -name '.goreleaser.yml' \) -not -path './.git/*' | sort)

if [ "$found" -eq 0 ]; then
  echo "FAIL found no -X ldflags in any .goreleaser.yaml -- this guard is checking nothing"
  exit 1
fi

if [ "$status" -eq 0 ]; then
  echo "All $found -X ldflags name a package in the module that builds them."
fi
exit "$status"
