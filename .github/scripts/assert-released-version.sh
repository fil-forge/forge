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
if [ -n "$default" ] && [ "$default" = "$want" ]; then
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
  case "$rel" in
    *"$host_arch"*|*all*) ;;
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

# A portable 10-second cap. GNU `timeout` is not on macOS; perl is on both
# runners. Needed because the binary set is derived: a future service whose root
# command serves rather than erroring on an unknown argument would otherwise
# block until GitHub's six-hour limit, with a release half-published.
run_probe() {
  perl -e 'alarm shift @ARGV; exec @ARGV or exit 127' 10 "$@" 2>&1
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

  if printf '%s' "$out" | grep -qF -- "$want"; then
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

if [ -n "$unassertable" ]; then
  echo
  echo "NOT ASSERTED:$unassertable" >&2
  echo "  These binaries answered neither \`version\` nor \`--version\` with anything" >&2
  echo "  version-shaped, so nothing here checked what they report." >&2
fi

if [ "$asserted" -eq 0 ]; then
  echo "Nothing was asserted. Treating that as a failure rather than a pass," >&2
  echo "because a version check that checked no version would be worse than" >&2
  echo "none: it would report green over exactly the defect it exists for." >&2
  echo >&2
  echo "This currently blocks sprue and indexing-service, whose binaries answer" >&2
  echo "no version probe. Both have a subcommand in flight upstream --" >&2
  echo "sprue#106 and indexing-service#107 -- and the fix here is to take those" >&2
  echo "pulls, not to soften this into a skip." >&2
  exit 1
fi
echo
echo "$asserted binary/binaries asserted,$(printf '%s' "${unassertable:- none}") unassertable."
