#!/usr/bin/env bash
# Run assert-released-version.sh against fixtures, both directions.
#
# WHY THIS EXISTS. assert-released-version.sh is called from release.yml, which
# is dispatch-only and has never been dispatched -- so until this guard, nothing
# in CI executed it. Three separate portability bugs shipped in it as a result,
# each found by a reviewer reading rather than by a run: `mapfile` (bash 4, not
# on the macOS runner it is routed to), a perl `alarm` cap that Go binaries
# ignore entirely, and `mktemp` with no template (GNU-only). A guard that reads
# is not a guard. This one runs it.
#
# AGENTS.md: "A guard that passes proves nothing until you have seen it fail on
# the defect it is for." So this asserts BOTH directions -- a dead -X must fail,
# a good one must pass -- and fails if either comes out the other way.
set -euo pipefail
cd "$(dirname "$0")/../.."

script=.github/scripts/assert-released-version.sh
tmp=$(mktemp -d "${TMPDIR:-/tmp}/check-assert.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

svc=$tmp/fixture
mkdir -p "$svc/pkg/build" "$svc/cmd" "$svc/dist/fixture_$(go env GOHOSTOS)_$(go env GOHOSTARCH)"
bin=$svc/dist/fixture_$(go env GOHOSTOS)_$(go env GOHOSTARCH)/fixture

cat > "$svc/go.mod" <<'EOF'
module example.com/fixture

go 1.24
EOF
cat > "$svc/version.json" <<'EOF'
{"version": "v7.7.7"}
EOF
cat > "$svc/pkg/build/version.go" <<'EOF'
package build

import (
	"encoding/json"
	"os"
)

var version string

const defaultVersion string = "v0.0.0"

func Version() string {
	if version == "" {
		if f, err := os.Open("version.json"); err == nil {
			defer f.Close()
			var j struct {
				Version string `json:"version"`
			}
			if json.NewDecoder(f).Decode(&j) == nil {
				return j.Version
			}
		}
		version = defaultVersion
	}
	return version
}
EOF
cat > "$svc/cmd/main.go" <<'EOF'
package main

import (
	"fmt"
	"os"

	"example.com/fixture/pkg/build"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println("version:", build.Version())
		return
	}
	os.Exit(1)
}
EOF

build_fixture() { # $1 = ldflag package path
  ( cd "$svc" && GOWORK=off GOFLAGS=-mod=mod \
      go build -ldflags="-X $1.version=v7.7.7" -o "$bin" ./cmd )
}

fail=0

# 1. A dead -X must FAIL. This is the defect the script exists for, and the one
#    a cwd-dependent version.json fallback used to hide.
build_fixture 'example.com/fixture/pkg/buildTYPO'
if ( cd "$svc" && bash "$OLDPWD/$script" dist v7.7.7 ) >/dev/null 2>&1; then
  echo "FAIL  a dead -X was accepted. The assertion is not asserting." >&2
  fail=1
else
  echo "ok    a dead -X is rejected"
fi

# 2. A good -X must PASS. A guard that rejects everything is equally useless.
build_fixture 'example.com/fixture/pkg/build'
if ( cd "$svc" && bash "$OLDPWD/$script" dist v7.7.7 ) >/dev/null 2>&1; then
  echo "ok    a correct -X is accepted"
else
  echo "FAIL  a correct -X was rejected. The assertion blocks good releases." >&2
  fail=1
fi

# 3. Releasing AT the compiled-in fallback must be refused, not passed: a match
#    there proves nothing, because an unstamped binary reports the same string.
if ( cd "$svc" && bash "$OLDPWD/$script" dist v0.0.0 ) >/dev/null 2>&1; then
  echo "FAIL  releasing at the fallback version was accepted; that is a tautology." >&2
  fail=1
else
  echo "ok    releasing at the fallback version is refused"
fi

# 4. A version that merely shares a prefix must not satisfy the check.
build_fixture 'example.com/fixture/pkg/buildTYPO'
sed -i.bak 's/v0.0.0/v7.7.77/' "$svc/pkg/build/version.go" && rm -f "$svc/pkg/build/version.go.bak"
build_fixture 'example.com/fixture/pkg/buildTYPO'
if ( cd "$svc" && bash "$OLDPWD/$script" dist v7.7.7 ) >/dev/null 2>&1; then
  echo "FAIL  v7.7.77 satisfied a want of v7.7.7; the match is not anchored." >&2
  fail=1
else
  echo "ok    a prefix-sharing version does not satisfy the check"
fi

exit $fail
