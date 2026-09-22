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
#
# EACH NEGATIVE TEST MATCHES ITS OWN MESSAGE and builds its own fixture state.
# Asserting `exit != 0` alone cannot tell "refused for the reason under test"
# from "failed for some other reason", and the difference is not theoretical:
# with the "released AT the fallback" refusal deleted, test 3 still printed its
# own name, because dist held an earlier test's binary and the script failed
# down the ordinary mismatch path instead. It passed for the wrong reason.
#
# THE FIXTURES ARE DELIBERATELY VARIED, not one well-behaved binary. A single
# good fixture leaves three branches unreachable -- the multi-binary NOT
# ASSERTED exit, the "cannot find the fallback" refusal, and run_probe's
# kill-after-timeout -- so any of them can be lost silently.
#
# FIXTURE ORDER MATTERS AS MUCH AS COUNT, and test 6 is the case. The
# stdin-eating binary has to sort BEFORE the binary it is meant to swallow, or
# there is nothing after it and the test passes with the bug present. `aaa`
# ahead of the goos does that, and the goarch stays in the name so the host-arch
# filter still selects it. It depends on the script under test doing `sort -u`
# on its dist listing: remove that and the order becomes filesystem order, and
# test 6 quietly reverts to a version that passes with the bug present.
#
# VERIFIED BOTH DIRECTIONS, by deleting each fix from the script under test and
# watching this go red:
#
#   the FATAL exit for an unassertable binary          -> FAIL (test 5)
#   the "released AT the fallback" refusal             -> FAIL (test 3)
#   the "cannot find the fallback" refusal             -> FAIL (test 7)
#   the `</dev/null` on the probe                      -> FAIL (test 6)
#   the full ERE escaping of $want                     -> FAIL (test 4b)
#   the "output must be version-shaped" test           -> FAIL (test 4c)
#   the `*_darwin_all` alternative in the dist filter  -> FAIL (test 9)
#   the host-ARCH `*) continue` arm of the dist filter -> FAIL (test 9b)
#
# THE LIST ABOVE IS THE CLAIM. A guard that names the fixes it covers is making
# one, and a list short by one row reads exactly like a complete list -- rule 5.
# Add the row in the same commit as the test.
#
# NOT COVERED, and listed so the next round starts from a set rather than a
# hunt. Each is a deliberate break that leaves this file green; all are inert
# in the tree today, which is why they are documented rather than tested:
#   dropping the LEADING anchor `(^|[^0-9.])`  -> v1.2.3 accepts v11.2.3
#   `--version` dropped from the probe list    -> a service answering only that
#                                                 becomes unassertable
#   `*all*` unanchored in the dist filter      -> a binary named `install`
#                                                 selects cross-compiled artifacts
#   GOHOSTOS/GOHOSTARCH -> GOOS/GOARCH         -> cross-compile picks the wrong host
#   `head -1` -> `tail -1` in the default read -> compares the wrong fallback
#   the host-OS `*) continue ;;` dist arm      -> a cross-OS artifact is probed
#                                                 rather than skipped; fails
#                                                 CLOSED (it cannot exec, so it
#                                                 comes back NOT ASSERTED), so
#                                                 it blocks good releases
#                                                 rather than passing bad ones
#
# COST: about 39s, most of it test 8 waiting out run_probe's own cap on a
# binary that never exits. That is the price of exercising the cap at all, and
# it is paid on every `guards` run; worth knowing before adding more.
set -euo pipefail
cd "$(dirname "$0")/../.."

# An absolute path, not $OLDPWD. Every invocation below runs inside a `( cd … )`
# subshell, and $OLDPWD is the repository root only while that cd is the first
# one the subshell performs -- load-bearing, invisible, and wrong the moment a
# test cds twice.
root=$(pwd)
script=$root/.github/scripts/assert-released-version.sh
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

# Run the assertion and require BOTH a non-zero exit and the message that names
# the reason. `want` is a grep -F pattern.
refuses_in() { # $1 = cwd, $2 = label, $3 = expected fragment, $4.. = script args
  local where=$1 label=$2 want=$3; shift 3
  local out rc
  set +e
  out=$( cd "$where" && bash "$script" "$@" 2>&1 ); rc=$?
  set -e
  if [ "$rc" -eq 0 ]; then
    echo "FAIL  $label: accepted it (exit 0)" >&2
    fail=1
  elif ! printf '%s' "$out" | grep -qF -- "$want"; then
    echo "FAIL  $label: refused, but not for the reason under test." >&2
    echo "      expected to see: $want" >&2
    printf '%s\n' "$out" | sed 's/^/      /' >&2
    fail=1
  else
    echo "ok    $label"
  fi
}

refuses() { local label=$1 want=$2; shift 2; refuses_in "$svc" "$label" "$want" "$@"; }

accepts() { # $1 = label, $2.. = script args
  local label=$1; shift
  if ( cd "$svc" && bash "$script" "$@" ) >/dev/null 2>&1; then
    echo "ok    $label"
  else
    echo "FAIL  $label: rejected it. The assertion blocks good releases." >&2
    # set +e: this diagnostic re-run is a failing pipeline, so under
    # `set -euo pipefail` errexit fired HERE and `fail=1` below was never
    # reached -- the suite stopped at the first failing positive test and
    # `exit $fail` was unreachable. refuses() has always guarded itself this
    # way; accepts() did not.
    set +e
    ( cd "$svc" && bash "$script" "$@" 2>&1 ) | sed 's/^/      /' >&2
    set -e
    fail=1
  fi
}

# 1. A dead -X must FAIL, naming the binary and what it reported. This is the
#    defect the script exists for, and the one a cwd-dependent version.json
#    fallback used to hide.
build_fixture 'example.com/fixture/pkg/buildTYPO'
refuses "a dead -X is rejected" "was built for v7.7.7 but reports" dist v7.7.7

# 2. A good -X must PASS. A guard that rejects everything is equally useless.
build_fixture 'example.com/fixture/pkg/build'
accepts "a correct -X is accepted" dist v7.7.7

# 3. Releasing AT the compiled-in fallback must be refused, not passed: a match
#    there proves nothing, because an unstamped binary reports the same string.
#    The binary is rebuilt UNSTAMPED first, so that if this refusal were
#    deleted the script would run on and PASS -- which is what makes the test
#    fail rather than quietly succeed down the mismatch path.
build_fixture 'example.com/fixture/pkg/buildTYPO'
refuses "releasing at the fallback version is refused" \
  "is being released at" dist v0.0.0

# 4. A version that merely shares a prefix must not satisfy the check.
sed -i.bak 's/v0.0.0/v7.7.77/' "$svc/pkg/build/version.go" && rm -f "$svc/pkg/build/version.go.bak"
build_fixture 'example.com/fixture/pkg/buildTYPO'
refuses "a prefix-sharing version does not satisfy the check" \
  "was built for v7.7.7 but reports" dist v7.7.7
sed -i.bak 's/v7.7.77/v0.0.0/' "$svc/pkg/build/version.go" && rm -f "$svc/pkg/build/version.go.bak"

# 4b. A VERSION CONTAINING AN ERE METACHARACTER. This is the test the round
#     that fixed the escaping was asked for and did not add -- and the ONE break
#     this guard did not catch: reverting `want_re`'s sed to `sed 's/\./\\./g'`
#     -- naming the variable rather than a line, because that line has moved in
#     three successive rounds and a line number is a hand-maintained fact -- left this
#     file printing nine ok lines and exit 0 while the assertion accepted a
#     stale binary. `release.yml` admits `+` deliberately (its charset class
#     lists it), so this is reachable, not hypothetical.
#
#     Both directions, because dot-only escaping is wrong in both: with `+`
#     unescaped, `1\.2\.3+meta` is an ERE meaning "one or more 3s", so it
#     ACCEPTS a binary reporting v1.2.33meta and it would REJECT the correct
#     v1.2.3+meta. Test the acceptance first so a regression cannot pass by
#     rejecting everything.
sed -i.bak 's/v0.0.0/v1.0.0/' "$svc/pkg/build/version.go" && rm -f "$svc/pkg/build/version.go.bak"
( cd "$svc" && GOWORK=off GOFLAGS=-mod=mod \
    go build -ldflags="-X example.com/fixture/pkg/build.version=v1.2.3+meta" -o "$bin" ./cmd )
accepts "a version containing a + is accepted when the binary reports it" dist 'v1.2.3+meta'

( cd "$svc" && GOWORK=off GOFLAGS=-mod=mod \
    go build -ldflags="-X example.com/fixture/pkg/build.version=v1.2.33meta" -o "$bin" ./cmd )
refuses "a + in the version is a literal, not an ERE quantifier" \
  "was built for v1.2.3+meta but reports" dist 'v1.2.3+meta'
sed -i.bak 's/v1.0.0/v0.0.0/' "$svc/pkg/build/version.go" && rm -f "$svc/pkg/build/version.go.bak"

# 4c. A BINARY THAT ANSWERS SOMETHING THAT IS NOT A VERSION must be reported as
#     unassertable, not blamed for a stale ldflag. Without the "output must be
#     version-shaped" test the script would print "was built for X but reports
#     <usage text>", which sends the reader after a flag that is fine.
notver=$svc/dist/fixture_$(go env GOHOSTOS)_$(go env GOHOSTARCH)_zzy
mkdir -p "$notver"
cat > "$tmp/notver.go" <<'EOF'
package main

import "fmt"

func main() { fmt.Println("usage: fixture [command]"); fmt.Println("see the docs") }
EOF
( cd "$tmp" && GOWORK=off GOFLAGS=-mod=mod go build -o "$notver/fixture" notver.go )
build_fixture 'example.com/fixture/pkg/build'
refuses "a binary that answers non-version text is unassertable, not stale" \
  "NOT ASSERTED" dist v7.7.7
rm -rf "$notver"

# 5. ONE BINARY THAT CANNOT BE ASKED CONDEMNS THE RELEASE, even alongside a
#    good one. This is the branch the old single-binary fixture could not
#    reach, and deleting its `exit 1` left the old guard green.
build_fixture 'example.com/fixture/pkg/build'
mute=$svc/dist/fixture_$(go env GOHOSTOS)_$(go env GOHOSTARCH)_v2
mkdir -p "$mute"
cat > "$tmp/mute.go" <<'EOF'
package main

func main() {}
EOF
( cd "$tmp" && GOWORK=off GOFLAGS=-mod=mod go build -o "$mute/fixture" mute.go )
refuses "a binary that answers nothing condemns the whole release" \
  "NOT ASSERTED" dist v7.7.7
rm -rf "$mute"

# 6. A BINARY THAT READS STDIN MUST NOT EAT THE LIST. run_probe is driven from
#    a heredoc, so without `</dev/null` the child inherits it and the binaries
#    after it are never probed -- silently, with exit 0.
build_fixture 'example.com/fixture/pkg/buildTYPO'
# The name has to SORT BEFORE the real binary's directory, or there is nothing
# left after it to swallow and this test passes with the bug present -- which
# it did, on the first version of it. `aaa` before the goos does that, and the
# goarch is still in the name so the host-arch filter still selects it.
hungry=$svc/dist/fixture_aaa_$(go env GOHOSTOS)_$(go env GOHOSTARCH)
mkdir -p "$hungry"
cat > "$tmp/hungry.go" <<'EOF'
package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	io.Copy(io.Discard, os.Stdin)
	fmt.Println("version: v7.7.7")
}
EOF
( cd "$tmp" && GOWORK=off GOFLAGS=-mod=mod go build -o "$hungry/fixture" hungry.go )
refuses "a stdin-reading binary does not swallow the ones after it" \
  "was built for v7.7.7 but reports" dist v7.7.7
rm -rf "$hungry"

# 7. A SERVICE WITH NO BUILD PACKAGE MUST BE REFUSED, not passed. Without a
#    fallback to compare against there is nothing to tell a stamped binary from
#    an unstamped one, and delegator and piri-signing-service are in exactly
#    this state today.
nobuild=$tmp/nobuild
mkdir -p "$nobuild/dist/nobuild_$(go env GOHOSTOS)_$(go env GOHOSTARCH)"
cp "$bin" "$nobuild/dist/nobuild_$(go env GOHOSTOS)_$(go env GOHOSTARCH)/nobuild"
refuses_in "$nobuild" "a service with no build package is refused" \
  "could not find" dist v7.7.7

# 8. A BINARY THAT NEVER EXITS MUST NOT HANG THE RELEASE. run_probe's cap is
#    the round-3 fix for a perl `alarm` that Go ignores; nothing exercised it.
build_fixture 'example.com/fixture/pkg/build'
stuck=$svc/dist/fixture_$(go env GOHOSTOS)_$(go env GOHOSTARCH)_zzz
mkdir -p "$stuck"
cat > "$tmp/stuck.go" <<'EOF'
package main

import "time"

func main() { time.Sleep(10 * time.Minute) }
EOF
( cd "$tmp" && GOWORK=off GOFLAGS=-mod=mod go build -o "$stuck/fixture" stuck.go )
started=$(date +%s)
refuses "a binary that never exits is bounded and condemns the release" \
  "NOT ASSERTED" dist v7.7.7
elapsed=$(( $(date +%s) - started ))
if [ "$elapsed" -gt 120 ]; then
  echo "FAIL  the probe cap did not bound it: ${elapsed}s" >&2
  fail=1
else
  echo "ok    the probe cap bounded it (${elapsed}s)"
fi
rm -rf "$stuck"

# 9. THE DIST FILTER'S "SKIP" DIRECTION. Tests 1-8 only ever exercise the arm
#    that SELECTS a binary; nothing reached the arms that skip one, so deleting
#    `*_darwin_all|*_darwin_all/*` from the filter left this file printing every
#    `ok` above and exit 0 -- while a stale UNIVERSAL binary sailed through on
#    macos-14, which is the only runner that arm exists for and the one piri
#    routes to. piri is also the one service in the fleet with a published tag.
#
#    Reachable on a linux runner by shimming `go env`, which is the only thing
#    the script under test asks go for. The binaries are this host's, named into
#    darwin directories: the filter selects on the PATH, never on the file.
shim=$tmp/shim; mkdir -p "$shim"
cat > "$shim/go" <<'SHIM'
#!/bin/sh
# Answer only what the script under test asks; delegate everything else.
if [ "$1" = env ]; then
  case "$2" in GOHOSTOS) echo darwin; exit 0 ;; GOHOSTARCH) echo arm64; exit 0 ;; esac
fi
exec "$REAL_GO" "$@"
SHIM
chmod +x "$shim/go"
REAL_GO=$(command -v go); export REAL_GO

# Clear the host-arch directory earlier tests built into FIRST, before the
# darwin fixtures exist. Doing it afterwards deleted `$arm` outright on a
# darwin/arm64 host -- the good fixture this test needs -- and the sabotage was
# then still caught, but through "refused, but not for the reason under test",
# which reads like a broken test rather than a caught defect. `guards` runs on
# ubuntu, so it only bit someone running this on the Mac the arm exists for.
rm -rf "${svc:?}/dist/fixture_$(go env GOHOSTOS)_$(go env GOHOSTARCH)"
uni=$svc/dist/fixture_darwin_all
arm=$svc/dist/fixture_darwin_arm64
amd=$svc/dist/fixture_darwin_amd64
mkdir -p "$uni" "$arm"
# The universal one is STALE (the -X names a package that does not exist, so the
# binary reports its default); the arch-specific one is correct. A filter that
# skips the universal binary sees only the good one and passes.
( cd "$svc" && GOWORK=off GOFLAGS=-mod=mod \
    go build -ldflags="-X example.com/fixture/pkg/buildTYPO.version=v7.7.7" -o "$uni/fixture" ./cmd )
( cd "$svc" && GOWORK=off GOFLAGS=-mod=mod \
    go build -ldflags="-X example.com/fixture/pkg/build.version=v7.7.7" -o "$arm/fixture" ./cmd )
# set +e around the assignment, the way refuses() does. Under `set -e` a
# command substitution that fails takes the assignment's exit status with it
# and errexit kills the script BEFORE the if -- which is how the first version
# of this test ran, produced no output at all, and looked like it had not run.
# Round seven found the same hole in accepts(); this is it a second time, in
# the test added to close round seven.
set +e
out=$( cd "$svc" && PATH="$shim:$PATH" bash "$script" dist v7.7.7 2>&1 ); rc=$?
set -e
if [ "$rc" -eq 0 ]; then
  echo "FAIL  a stale universal binary is not skipped by the dist filter: accepted it (exit 0)" >&2
  fail=1
elif ! printf '%s' "$out" | grep -qF -- "was built for v7.7.7 but reports"; then
  echo "FAIL  the universal binary was refused, but not for the reason under test." >&2
  printf '%s\n' "$out" | sed 's/^/      /' >&2
  fail=1
else
  echo "ok    a stale universal binary is selected and refused"
fi
rm -rf "$uni"

# 9b. THE ARM THAT SKIPS, which test 9 does not reach. Test 9 covers the
#     `*_darwin_all|*_darwin_all/*` SELECT alternative -- the defect round seven
#     named -- but instrumenting both `case` statements during round eight showed
#     the `*) continue ;;` arms still taken ZERO times across the whole suite:
#
#       14 arch-select   15 os-select   1 universal-select   0 os-skip   0 arch-skip
#
#     So deleting the host-OS filter entirely still gave 13 ok / exit 0.
#
#     9b reaches the host-ARCH one, and only that one: a cross-arch binary
#     alongside a good host-arch one means the release must PASS, on the arm64
#     binary, with the stale amd64 one skipped. Re-instrumented with 9b in
#     place: 17 os-select, 15 arch-select, 1 universal-select, 1 arch-skip,
#     **0 os-skip**. The host-OS skip arm is still unreached and deleting the
#     whole host-OS `case` still gives 14 ok / exit 0 -- it is in NOT COVERED
#     below, because a list of what a guard does not check is a claim like any
#     other and this one was missing an entry the same push measured.
#
#     It is also a tripwire on the shim, which is load-bearing and was otherwise
#     unasserted. The shim answers ONE variable per call and delegates the rest,
#     which is right for the script as written -- but if the script under test
#     ever asks for both in one call, the shim answers the FIRST and exits, so
#     both reads get the same answer: rewriting lines 88-89 to
#     `go env GOHOSTOS GOHOSTARCH | head -1` and `| tail -1` leaves `host_arch`
#     holding "darwin", not empty, and `*darwin*` then matches every path under
#     dist. (An earlier version of this comment said "empty" and `*""*`; the
#     effect is the same and the mechanism was not, which is the kind of claim
#     these reviews exist to catch.) Test 9 then passes WITH the `_darwin_all`
#     arm deleted. Under 9b the same degradation turns this red, because an
#     unfiltered amd64 binary is stale.
mkdir -p "$amd"
( cd "$svc" && GOWORK=off GOFLAGS=-mod=mod \
    go build -ldflags="-X example.com/fixture/pkg/buildTYPO.version=v7.7.7" -o "$amd/fixture" ./cmd )
set +e
out=$( cd "$svc" && PATH="$shim:$PATH" bash "$script" dist v7.7.7 2>&1 ); rc=$?
set -e
if [ "$rc" -ne 0 ]; then
  echo "FAIL  a cross-arch binary is not skipped: the release was refused over it" >&2
  printf '%s\n' "$out" | sed 's/^/      /' >&2
  fail=1
else
  echo "ok    a stale cross-arch binary is skipped"
fi
rm -rf "$arm" "$amd" "$shim"

exit $fail
