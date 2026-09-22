#!/usr/bin/env bash
# Print the released versions the compat suite can pin against, per service.
#
# THE SOURCE IS THE REGISTRY, NOT GIT TAGS, because the two disagree: the
# polyrepos tag git `v0.2.4` and publish the image `0.2.4`. Measured --
# ghcr.io/fil-forge/piri:0.2.4 answers 200 and :v0.2.4 answers 404 -- so a
# version read from a tag and used as an image tag pulls nothing, and the
# failure surfaces much later as a container that never became healthy.
# Reading the registry asks the question the suite actually has: what can I
# pin against?
#
# It reads the POLYREPOS' packages (ghcr.io/fil-forge/<svc>), which are what
# ships to the network today. This repository publishes no images and carries
# no service-prefixed tags; neither is a prerequisite.
#
# Anonymous pull tokens. These packages are public, and a token minted for
# THIS repository would not widen access to another repository's packages --
# logging in buys nothing here and could narrow it.
#
# Output is `key=value` lines on stdout, suitable for appending to
# $GITHUB_OUTPUT, and readable on a terminal:
#
#   base_<svc>=<newest version>     empty when the service publishes none
#   <svc>=<newest N, comma sep>     piri and ingot only -- the two services
#                                   TestPinnedPeer pins, because they are the
#                                   ones third parties run and so can lag
#   pinnable=true|false             false when NOTHING can be pinned, which is
#                                   not a failure: a service with no published
#                                   release should not give the nightly a
#                                   permanent red
#
# Usage: compat-window.sh [N]        N defaults to 2
set -euo pipefail

n=${1:-2}
case "$n" in
  ''|*[!0-9]*) echo "usage: $0 [N]  (N must be a positive integer)" >&2; exit 2 ;;
esac
[ "$n" -ge 1 ] || { echo "usage: $0 [N]  (N must be >= 1)" >&2; exit 2; }

# Version-shaped tags only, newest first. Excludes main, main-dev, latest and
# sha-*, and also the per-architecture `0.0.0-amd64` / `0.0.0-arm64` variants
# ingot publishes, which are not runnable as a multi-arch reference.
tags_for() {
  local svc=$1 tok
  tok=$(curl -fsS "https://ghcr.io/token?scope=repository:fil-forge/${svc}:pull&service=ghcr.io" \
        | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])')
  curl -fsS -H "Authorization: Bearer $tok" \
       "https://ghcr.io/v2/fil-forge/${svc}/tags/list" \
    | python3 -c '
import json, re, sys
tags = json.load(sys.stdin).get("tags") or []
out = [t for t in tags if re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", t)]
out.sort(key=lambda v: [int(p) for p in v.split(".")], reverse=True)
print("\n".join(out))
'
}

pinnable=false
for svc in piri ingot sprue hilt; do
  # `|| true`: a service whose package does not exist yet should report "none",
  # not abort the run for the services that do.
  all=$(tags_for "$svc" 2>/dev/null || true)

  if [ -z "$all" ]; then
    echo "$svc publishes no version-tagged image; anything needing it will skip" >&2
    echo "base_${svc}="
    continue
  fi

  echo "base_${svc}=$(printf '%s\n' "$all" | head -n 1)"

  case "$svc" in
    piri|ingot)
      echo "${svc}=$(printf '%s\n' "$all" | head -n "$n" | paste -sd, -)"
      pinnable=true
      ;;
  esac
done

echo "pinnable=${pinnable}"
