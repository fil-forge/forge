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
# "NO RELEASES" AND "COULD NOT ASK" ARE DIFFERENT ANSWERS, and the whole value
# of this script is that it never confuses them. An earlier revision wrapped
# the registry read in `|| true`, so a network failure printed "publishes no
# version-tagged image" for all four services, exited 0 with pinnable=false,
# and the nightly went green having tested nothing. Now: a package the registry
# declines to mint a token for reports none, and any OTHER failure aborts.
#
# Output is `key=value` lines on stdout, suitable for appending to
# $GITHUB_OUTPUT, and readable on a terminal:
#
#   base_<svc>=<newest version>     empty when the service publishes none
#   <svc>=<newest N, comma sep>     piri and ingot only -- the two services
#                                   TestPinnedPeer pins, because they are the
#                                   ones third parties run and so can lag
#   pinnable=true|false             false when NOTHING can be pinned. Since the
#                                   registry failing is now fatal, that can only
#                                   mean piri and ingot both publish no
#                                   version-tagged image -- the bootstrap case,
#                                   which should not give the nightly a
#                                   permanent red.
#
# Usage: compat-window.sh [N]        N defaults to 2
set -euo pipefail

n=${1-2}
case "$n" in
  ''|*[!0-9]*) echo "usage: $0 [N]  (N must be a positive integer)" >&2; exit 2 ;;
esac
[ "$n" -ge 1 ] || { echo "usage: $0 [N]  (N must be >= 1)" >&2; exit 2; }

api=${GHCR_API:-https://ghcr.io}

# Every tag, following pagination. `n=1000` fits all 111 of ingot's in one
# request today, but a page size is not a guarantee: the registry answers with
# `Link: <...?last=<tag>&n=...>; rel="next"` when it has truncated, so the loop
# follows that rather than trusting one request to be complete. Only `last` is
# taken from it -- GHCR's own next link carries `n=0`, and replaying that asks
# for a page size this script did not choose.
#
# Order is NOT lexicographic and not version order: ingot's first page of 100
# held exactly one version-shaped tag out of 111. Sorting happens below, over
# the whole set.
#
# Exit: 0 answered (tags on stdout, possibly none), 10 no such package,
#       1 the registry could not be read.
tags_for() {
  local svc=$1 tok code last url body
  body=$(mktemp)
  # shellcheck disable=SC2064
  trap "rm -f '$body'" RETURN

  code=$(curl -sS -o "$body" -w '%{http_code}' \
         "$api/token?scope=repository:fil-forge/${svc}:pull&service=ghcr.io") || {
    echo "$svc: could not reach $api to mint a pull token" >&2; return 1; }
  case "$code" in
    200) ;;
    401|403|404)
      # Measured: a package that does not exist is refused HERE, at the token
      # endpoint, with 403 DENIED -- not at tags/list with a 404.
      return 10 ;;
    *) echo "$svc: token endpoint answered HTTP $code" >&2; return 1 ;;
  esac
  tok=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])' <"$body") || {
    echo "$svc: token endpoint returned something that is not a token" >&2; return 1; }

  url="$api/v2/fil-forge/${svc}/tags/list?n=1000"
  while [ -n "$url" ]; do
    local hdr; hdr=$(mktemp)
    code=$(curl -sS -D "$hdr" -o "$body" -w '%{http_code}' \
           -H "Authorization: Bearer $tok" "$url") || {
      rm -f "$hdr"; echo "$svc: could not read $url" >&2; return 1; }
    if [ "$code" != 200 ]; then
      rm -f "$hdr"; echo "$svc: tags/list answered HTTP $code" >&2; return 1
    fi
    python3 -c 'import json,sys; print("\n".join(json.load(sys.stdin).get("tags") or []))' <"$body" || {
      rm -f "$hdr"; echo "$svc: tags/list returned something that is not a tag list" >&2; return 1; }
    last=$(sed -n 's/.*[?&]last=\([^&>]*\).*rel="next".*/\1/p' "$hdr" | head -n 1)
    rm -f "$hdr"
    if [ -n "$last" ]; then
      url="$api/v2/fil-forge/${svc}/tags/list?n=1000&last=$last"
    else
      url=
    fi
  done
}

# Version-shaped tags only, newest first. Excludes main, main-dev, latest and
# sha-*, and also the per-architecture `0.0.0-amd64` / `0.0.0-arm64` variants
# ingot publishes, which are not runnable as a multi-arch reference.
versions_from() {
  python3 -c '
import re, sys
out = {t for t in sys.stdin.read().split() if re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", t)}
print("\n".join(sorted(out, key=lambda v: [int(p) for p in v.split(".")], reverse=True)))
'
}

pinnable=false
for svc in piri ingot sprue hilt; do
  set +e
  raw=$(tags_for "$svc")
  rc=$?
  set -e
  case "$rc" in
    0) ;;
    10)
      echo "$svc: no such package at $api -- reporting none" >&2
      echo "base_${svc}="
      continue ;;
    *)
      echo "::error::could not read $svc's tags; refusing to report that as" \
           "'no releases'. A compat run pinned against nothing is green and" \
           "meaningless." >&2
      exit 1 ;;
  esac

  all=$(printf '%s\n' "$raw" | versions_from)

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
