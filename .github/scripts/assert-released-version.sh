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
# Usage: assert-released-version.sh <dist-dir> <expected-version>
set -euo pipefail

dist=${1:?usage: assert-released-version.sh <dist-dir> <expected-version>}
want=${2:?usage: assert-released-version.sh <dist-dir> <expected-version>}

# Which binaries exist is DERIVED from what goreleaser produced, not listed.
# Only the host arch can be executed here; a cross-compiled artifact cannot be
# asked anything, and pretending otherwise would make this pass by looking away.
host_os=$(go env GOOS)
host_arch=$(go env GOARCH)

mapfile -t bins < <(
  find "$dist" -type f -perm -u+x \
    -path "*${host_os}*" \
    \( -path "*${host_arch}*" -o -path "*all*" \) \
    ! -name '*.txt' ! -name '*.json' ! -name '*.tar.gz' ! -name '*.zip' \
    2>/dev/null | sort -u
)

if [ "${#bins[@]}" -eq 0 ]; then
  echo "no ${host_os}/${host_arch} binary under $dist -- nothing to assert against." >&2
  echo "That is a failure, not a skip: the build produced nothing this runner can run." >&2
  exit 1
fi

asserted=0
unassertable=()

for bin in "${bins[@]}"; do
  name=$(basename "$bin")
  # Probe rather than consult a table of which services have a version command.
  # A service that gains one is picked up with no list to update; one that loses
  # it shows up as unassertable instead of silently passing.
  out=""
  for probe in version --version; do
    if out=$("$bin" "$probe" 2>&1) && [ -n "$out" ]; then
      break
    fi
    out=""
  done

  if [ -z "$out" ]; then
    unassertable+=("$name")
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
done

if [ "${#unassertable[@]}" -gt 0 ]; then
  echo
  echo "NOT ASSERTED: ${unassertable[*]}" >&2
  echo "  These binaries answered neither \`version\` nor \`--version\`, so nothing" >&2
  echo "  here checked what they report. Giving each one a version subcommand is" >&2
  echo "  ~15 lines modelled on piri/cmd/cli/version.go, and is worth having" >&2
  echo "  regardless: operators ask binaries their version." >&2
fi

if [ "$asserted" -eq 0 ]; then
  echo "Nothing was asserted. Treating that as a failure rather than a pass." >&2
  exit 1
fi
echo
echo "$asserted binary/binaries asserted, ${#unassertable[@]} unassertable."
