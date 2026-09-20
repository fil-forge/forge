#!/usr/bin/env bash
# Assert that a freshly built binary REPORTS the version it was built for.
#
# The release flow's own check on itself. `check-goreleaser-ldflags.sh` checks
# that every `-X` names a package in the module being linked; that is a check on
# the path. This is the check on the effect, and it is the one that matters,
# because nothing else is loud: cmd/link's addstrdata gives up silently on a
# missing symbol, and `go version -m` records the -ldflags argument either way,
# so a released artifact's build info looks correct while the binary reports its
# fallback. It also catches what the lint cannot -- a correct -X against a symbol
# that was dead-code eliminated.
#
# PORTABLE ON PURPOSE. This runs on macos-14 as well as ubuntu, because the
# workflow routes piri there (darwin + cgo). GitHub's macOS image ships bash
# 3.2, which has no `mapfile`, and no GNU coreutils, which means no `timeout`.
# Neither is used below. Keep it that way.
#
# Usage: assert-released-version.sh <dist-dir> <expected-version>
set -euo pipefail

dist=${1:?usage: assert-released-version.sh <dist-dir> <expected-version>}
want=${2:?usage: assert-released-version.sh <dist-dir> <expected-version>}

# Resolve dist while the caller's cwd still applies, THEN pin our own.
dist_abs=$(cd "$(dirname "$dist")" && pwd)/$(basename "$dist")
svc_dir=$(dirname "$dist_abs")

# Pinning cwd to the repository root is load-bearing, not tidiness. Every
# service's pkg/build does, in init(): if the -X did nothing, fall back to
# reading ./version.json. Run this from inside <svc>/ -- which is where
# goreleaser itself runs, via `workdir:` -- and that fallback finds the file,
# the binary reports the right version for the wrong reason, and this script
# says ok to the exact defect it exists to catch. From the root there is no
# ./version.json, so a dead -X reports its default and is caught.
cd "$(dirname "$0")/../.."

# THE CHECK ONLY WORKS WHEN THE ANSWERS DIFFER. The whole method is "a dead -X
# makes the binary report its compiled-in default", so if the version being
# released IS that default, a match proves nothing: a correctly stamped binary
# and a completely unstamped one say the same thing. Five of the six services
# default to v0.0.0, and hilt, swarf and ingot carry v0.0.0 in version.json
# today -- so this is the live case, not a hypothetical.
#
# Derived from the service's own source rather than tabulated, so a service
# that changes its default moves itself.
# `|| true` is load-bearing: grep exits 2 (an error, not "no match") when a
# directory does not exist, and most services have no internal/build. Under
# `set -euo pipefail` that killed the script before it did anything, with no
# output and exit 2. Found by running it.
default=$(
  { grep -rhoE 'defaultVersion[[:space:]]+(string[[:space:]]+)?=[[:space:]]*"[^"]+"|Version[[:space:]]*=[[:space:]]*"dev"' \
      "$svc_dir/pkg/build" "$svc_dir/internal/build" 2>/dev/null || true; } \
    | grep -oE '"[^"]+"' | tr -d '"' | head -1 || true
)
if [ -z "$default" ]; then
  echo "CANNOT ASSERT: could not find $(basename "$svc_dir")'s compiled-in fallback" >&2
  echo "  version under pkg/build or internal/build, so there is no way to tell" >&2
  echo "  a correctly stamped binary from one whose -X did nothing." >&2
  echo >&2
  echo "  Refusing rather than proceeding: a guard that cannot find what it" >&2
  echo "  compares against is a guard that passes everything. delegator and" >&2
  echo "  piri-signing-service are in this state, and become dispatchable the" >&2
  echo "  day either gains a .goreleaser.yaml." >&2
  exit 1
fi
if [ "$default" = "$want" ]; then
  echo "CANNOT ASSERT: $(basename "$svc_dir") is being released at $want, which is" >&2
  echo "  also its compiled-in fallback ($default). A binary whose -X did nothing" >&2
  echo "  reports exactly that, so a match here would prove nothing and this" >&2
  echo "  check would be green over the defect it exists for." >&2
  echo >&2
  echo "  Release at a version that is not the fallback, or change the fallback." >&2
  exit 1
fi

# Which binaries exist is DERIVED from what goreleaser produced, not listed.
# Only the host arch can be executed here; a cross-compiled artifact cannot be
# asked anything, and pretending otherwise would make this pass by looking away.
#
# GOHOSTOS/GOHOSTARCH, not GOOS/GOARCH: the latter report the build *target*
# and are overridable from the environment, so a cross-build earlier in a job
# would point this at binaries the runner cannot execute.
host_os=$(go env GOHOSTOS)
host_arch=$(go env GOHOSTARCH)

# Matched against the path RELATIVE to dist. Against the absolute path, a
# checkout under any directory containing "linux", "all", "amd64" and so on --
# /srv/install/forge, ~/fallback/forge -- selects every cross-compiled artifact,
# which then all fail to exec and are reported as needing a version subcommand.
bins=""
while IFS= read -r rel; do
  [ -n "$rel" ] || continue
  case "$rel" in
    *"$host_os"*) ;;
    *) continue ;;
  esac
  # `*all*` unanchored matched any binary whose NAME contains "all" -- wallet,
  # install, smallfoo -- and selected cross-compiled artifacts that then failed
  # to exec and were reported as missing a version subcommand. goreleaser's
  # universal binaries live under `<id>_darwin_all`, so match that, not "all".
  case "$rel" in
    *"$host_arch"*|*_darwin_all|*_darwin_all/*) ;;
    *) continue ;;
  esac
  bins="$bins$dist_abs/$rel
"
done <<EOF
$(cd "$dist_abs" 2>/dev/null && find . -type f -perm -u+x \
    ! -name '*.txt' ! -name '*.json' ! -name '*.tar.gz' ! -name '*.zip' \
    2>/dev/null | sed 's|^\./||' | sort -u)
EOF

if [ -z "$bins" ]; then
  echo "no ${host_os}/${host_arch} binary under $dist -- nothing to assert against." >&2
  echo "That is a failure, not a skip: the build produced nothing this runner can run." >&2
  exit 1
fi

asserted=0
unassertable=""

# A portable 10-second cap, done the only way that works here.
#
# GNU `timeout` is not on macOS. perl's `alarm` across an `exec` is portable but
# USELESS against these binaries: Go's runtime registers SIGALRM as _SigNotify
# with no _SigKill, so with nobody calling signal.Notify the signal is swallowed
# and the process runs on. Measured -- `alarm 3` against a sleeping Go binary
# was still alive at 15s, while the identical call against /bin/sleep died at 3s
# with "Alarm clock". Every binary probed here is a Go binary, so the previous
# revision's cap bounded nothing at all.
#
# Background the probe, poll, and SIGKILL. No coreutils, no signal handling in
# the child, works in bash 3.2.
run_probe() {
  local out_file status pid waited
  # A template, because BSD mktemp (macos-14, which this script is routed to
  # for piri) rejects the bare GNU form. That would have made every probe
  # return nothing and the run blame the wrong services.
  out_file=$(mktemp "${TMPDIR:-/tmp}/assert-released.XXXXXX")
  # </dev/null is load-bearing. This loop is driven by a heredoc, so without it
  # the child inherits that heredoc on fd 0 -- and a binary that reads stdin
  # (a cobra command with no args, say) swallows the rest of the list. Measured
  # on three fixtures with the middle one doing `cat >/dev/null`: the third
  # binary, which carried the exact defect this script exists for, was never
  # probed and the script exited 0 saying "2 binary/binaries asserted".
  "$@" </dev/null >"$out_file" 2>&1 &
  pid=$!
  waited=0
  while kill -0 "$pid" 2>/dev/null; do
    if [ "$waited" -ge 10 ]; then
      kill -9 "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
      rm -f "$out_file"
      return 124
    fi
    sleep 1
    waited=$((waited + 1))
  done
  wait "$pid"; status=$?
  cat "$out_file"
  rm -f "$out_file"
  return $status
}

while IFS= read -r bin; do
  [ -n "$bin" ] || continue
  name=$(basename "$bin")
  # Probe rather than consult a table of which services have a version command.
  # A service that gains one is picked up with no list to update; one that loses
  # it shows up as unassertable instead of silently passing.
  #
  # Exit 0 with output is NOT enough. A cobra root with a Run function, or a
  # urfave/cli app with CommandNotFound set, prints usage and exits 0 on an
  # unknown argument -- and that output would then be compared against the
  # version and reported as a stale-ldflag build defect, which it is not. So the
  # output must also contain something version-shaped to count as an answer.
  out=""
  for probe in version --version; do
    if candidate=$(run_probe "$bin" "$probe") \
       && printf '%s' "$candidate" | grep -qE '[0-9]+\.[0-9]+'; then
      out=$candidate
      break
    fi
  done

  if [ -z "$out" ]; then
    unassertable="$unassertable $name"
    continue
  fi

  # Anchored, not a substring match. `grep -F v1.13.4` is satisfied by a
  # binary reporting v1.13.40, and v0.1.10 satisfies a want of v0.1.1 -- so
  # a stale -X can pass simply by sharing a prefix with the real version.
  #
  # The leading v is optional on the binary's side: goreleaser's {{.Version}}
  # carries none, and whether a config re-adds it is per-config. An earlier
  # revision normalised that with `sed 's/\bv\([0-9]\)/\1/g'`, which is a
  # GNU-ism -- a silent no-op on BSD sed, on the runner this file claims to
  # support -- and was unreachable anyway.
  want_bare=${want#v}
  # Every ERE metacharacter, not just the dot. release.yml deliberately admits
  # `+` in a version (its reject class is [^0-9A-Za-z.+-]), and `+` is a
  # quantifier: with only `.` escaped, want=v1.2.3+meta FAILED to match a
  # correct binary reporting v1.2.3+meta, and MATCHED a stale one reporting
  # v1.2.33meta. Wrong in both directions on the same input.
  want_re=$(printf '%s' "$want_bare" | sed 's/[][.^$*+?(){}|\\]/\\&/g')
  if printf '%s' "$out" | grep -qE "(^|[^0-9.])v?${want_re}([^0-9.]|\$)"; then
    echo "ok        $name reports $want"
    asserted=$((asserted + 1))
  else
    echo "FAIL      $name was built for $want but reports:" >&2
    printf '%s\n' "$out" | sed 's/^/            /' >&2
    echo "          This is the stale-ldflag failure: the build succeeded, the" >&2
    echo "          artifact's build info looks right, and the binary disagrees." >&2
    exit 1
  fi
done <<EOF
$bins
EOF

# FATAL, not a warning. It was a warning, and that left a hole big enough to
# drive the whole defect through: with one good binary and one whose -X was
# dead, the good one incremented `asserted`, the dead one fell into this bucket
# because its fallback ("dev", ingot's) is not version-shaped, and the script
# exited 0. A guard over part of a chain reads exactly like a guard over the
# chain -- rule 5. If a binary cannot be asked, this check cannot speak for the
# release, and says so.
if [ -n "$unassertable" ]; then
  echo >&2
  echo "NOT ASSERTED:$unassertable" >&2
  echo "  These binaries answered neither \`version\` nor \`--version\` with" >&2
  echo "  anything version-shaped, so nothing here checked what they report --" >&2
  echo "  which is indistinguishable from a build whose -X did nothing." >&2
  echo >&2
  echo "  The fix is to give each of those a version subcommand (~15 lines," >&2
  echo "  modelled on piri/cmd/cli/version.go), not to soften this to a skip." >&2
  exit 1
fi

if [ "$asserted" -eq 0 ]; then
  echo "Nothing was asserted. Treating that as a failure rather than a pass," >&2
  echo "because a version check that checked no version would be worse than" >&2
  echo "none: it would report green over exactly the defect it exists for." >&2
  exit 1
fi
echo
echo "$asserted binary/binaries asserted,$(printf '%s' "${unassertable:- none}") unassertable."
