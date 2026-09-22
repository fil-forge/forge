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
# and the run went green having tested nothing. Now: a package the registry
# declines to mint a token for reports none, and any OTHER failure aborts.
#
# Output is `key=value` lines on stdout, suitable for appending to
# $GITHUB_OUTPUT, and readable on a terminal:
#
#   base_<PKG>=<newest version>     empty when the service publishes none.
#                                   Readable form; nothing consumes it
#   <svc>=<newest N, comma sep>     likewise, piri and ingot only
#   baselines={"PKG":"ver",...}     ONE output carrying every baseline, so the
#                                   workflow needs no per-service `outputs:`
#                                   entry -- a hand-maintained list that fed
#                                   COMPAT_BASELINE_SPRUE to a test reading
#                                   COMPAT_BASELINE_UPLOAD, and silently omitted
#                                   four services when the fleet grew
#   windows={"PKG":"v1,v2",...}     the same, for the pinned-peer window
#   pinnable=true|false             false when NOTHING can be pinned. Since the
#                                   registry failing is now fatal, that can only
#                                   mean piri and ingot both publish no
#                                   version-tagged image -- the bootstrap case,
#                                   which should not give this a permanent
#                                   red.
#
# The KEY is the package name upper-cased with `-` as `_`, which is what the
# test reads as COMPAT_BASELINE_<KEY>. Package, never smelt's service name:
# three of the nine differ, and the registry knows only the former.
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
    403)
      # 403 ONLY, because 403 is the only one measured: a package that does not
      # exist is refused HERE, at the token endpoint, with 403 DENIED -- not at
      # tags/list with a 404. 401 and 404 were in this list as a guess, and a
      # guess in this position fails open: simulated, a 401 gave four "no
      # version-tagged image" lines, exit 0, pinnable=false, the compat job
      # skipped and the run green having asserted nothing. That is the exact
      # failure the `|| true` above it was removed to close.
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

# THE SERVICE LIST IS DERIVED, from the one table the Go side already keys on:
# `publishedImages` in smelt/pkg/stack/options.go names a ghcr.io reference per
# service this repository builds. A service added there needs no edit here, and
# the two cannot disagree about who is in the fleet.
#
# These are PACKAGE names, which is what the registry knows and what the test
# reads as COMPAT_BASELINE_<NAME>. They are not smelt's service names: three
# differ, and asking ghcr.io for fil-forge/upload, fil-forge/indexer or
# fil-forge/signing-service gets a 403.
here=$(cd "$(dirname "$0")" && pwd)
services=$(sed -n 's|.*"ghcr\.io/fil-forge/\([a-z0-9-]*\):main".*|\1|p' \
           "$here/../../smelt/pkg/stack/options.go")
if [ -z "$services" ]; then
  echo "::error::could not derive the service list from" \
       "smelt/pkg/stack/options.go. Refusing to resolve a guessed list." >&2
  exit 2
fi

key_for() { printf '%s' "$1" | tr 'a-z-' 'A-Z_'; }

pinnable=false
baselines=
windows=
for svc in $services; do
  set +e
  raw=$(tags_for "$svc")
  rc=$?
  set -e
  case "$rc" in
    0) ;;
    10)
      echo "$svc: no such package at $api -- reporting none" >&2
      echo "base_$(key_for "$svc")="
      baselines="$baselines$(key_for "$svc")\t\n"
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
    echo "base_$(key_for "$svc")="
    baselines="$baselines$(key_for "$svc")\t\n"
    continue
  fi

  newest=$(printf '%s\n' "$all" | head -n 1)
  echo "base_$(key_for "$svc")=$newest"
  baselines="$baselines$(key_for "$svc")\t$newest\n"

  # THE WINDOW IS piri AND ingot ONLY, and that is a decision rather than an
  # omission: they are the services third parties run, and therefore the ones
  # that can lag behind us. sprue and hilt we deploy ourselves and can upgrade
  # together, so a skew between them is our scheduling problem rather than a
  # compatibility contract; everything else here is ours too. TestPinnedPeer
  # pins exactly this pair.
  case "$svc" in
    piri|ingot)
      window=$(printf '%s\n' "$all" | head -n "$n" | paste -sd, -)
      echo "${svc}=$window"
      windows="$windows$(key_for "$svc")\t$window\n"
      pinnable=true
      ;;
  esac
done

as_json() {
  printf '%b' "$1" | python3 -c '
import json, sys
out = {}
for line in sys.stdin:
    line = line.rstrip("\n")
    if not line:
        continue
    k, _, v = line.partition("\t")
    out[k] = v
print(json.dumps(out, sort_keys=True))
'
}
echo "baselines=$(as_json "$baselines")"
echo "windows=$(as_json "$windows")"
echo "pinnable=${pinnable}"
