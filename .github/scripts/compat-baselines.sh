#!/usr/bin/env bash
# Print the baseline image each fleet service is pinned to for the compat suite.
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
# version-tagged image" for every service and the run went green having tested
# nothing. Now: any failure to read the registry aborts.
#
# # Two kinds of baseline, and the second is a fallback
#
# A RELEASE BASELINE is a version-shaped image tag -- the newest `X.Y.Z` the
# service publishes. Upstream pushes one only from a git tag `vX.Y.Z`, never
# from an ordinary push to main, so it names a cut release and does not move.
#
# A FLOATING BASELINE is `:main` resolved to its digest, used for a service
# that has cut no release at all. Three of the eight have never been tagged in
# git (sprue, delegator, piri-signing-service) and two carry a `v0.0.0` that
# predates their image workflow and published no image (hilt, swarf), so five
# of eight have no release to pin to and will not for as long as the release
# flow is unarmed.
#
# Skipping instead is what this used to do, and it made the gate VACUOUS: the
# job ran, the test found one baseline missing, skipped, exited 0, and the
# check went green in 0.126s having booted nothing. A green check that means
# "did not look" is worse than no check.
#
# The floating baseline is deliberately less than a release baseline, and the
# difference is worth stating rather than smoothing over:
#
#   - It is CURRENT CODE, not old code. `:main` tracks upstream's main, which
#     these prefixes are resynced against, so for those five the "old" fleet
#     is about the same code as HEAD and the skew they contribute is near
#     zero. That is the right outcome rather than a compromise -- we deploy
#     those five ourselves and can upgrade them together, so skew between
#     them is a scheduling problem. The skew the suite is actually about
#     comes from piri and ingot, which third parties run, and those two do
#     have release baselines.
#   - It is not this tree, either. `:main` is upstream's main, and a prefix
#     resynced some days ago is behind it, so a red can be caused by upstream
#     drift rather than by the pull request. Measured at the time of writing:
#     hilt 1 commit, piri 1, ingot 6, the rest 0. Small today, and it grows
#     between resyncs.
#   - It MOVES. Two runs of the same commit can face different code. Which is
#     why what goes out is the DIGEST and not the tag: the run still picks up
#     whatever `:main` is at the moment it asks, so the answer stays fresh,
#     but the digest is written into the run's own log and output, so a red is
#     reproducible afterwards. A floating tag would leave you unable to say
#     what a failed run even ran.
#
# This is scaffolding. Once every service cuts releases through the flow in
# RELEASE.md there is nothing left for the fallback to cover, the warning
# below stops firing on its own, and the fallback can come out.
#
# Output is `key=value` lines on stdout, suitable for appending to
# $GITHUB_OUTPUT, and readable on a terminal:
#
#   base_<PKG>=<version|sha256:...>  readable form; nothing consumes it
#   baselines={"PKG":"...",...}      ONE output carrying every baseline, so the
#                                    workflow needs no per-service `outputs:`
#                                    entry -- a hand-maintained list that fed
#                                    COMPAT_BASELINE_SPRUE to a test reading
#                                    COMPAT_BASELINE_UPLOAD, and silently
#                                    omitted four services when the fleet grew
#
# A value is either a version (`0.2.4`) or a digest (`sha256:...`); the test
# tells them apart by the prefix and needs no second output saying which is
# which. Never empty: a service this cannot resolve either way is a hard
# failure, because a fleet member with no image is a fleet that does not boot.
#
# The KEY is the package name upper-cased with `-` as `_`, which is what the
# test reads as COMPAT_BASELINE_<KEY>. Package, never smelt's service name:
# three of the nine differ, and the registry knows only the former.
#
# Usage: compat-baselines.sh
set -euo pipefail

if [ "$#" -gt 0 ]; then
  echo "usage: $0   (takes no arguments)" >&2
  exit 2
fi

api=${GHCR_API:-https://ghcr.io}

# Both list media types. THE INDEX DIGEST, NEVER A PER-ARCHITECTURE ONE
# (AGENTS.md rule 2): every one of these images is a 4-entry OCI index --
# amd64, arm64 and two attestation manifests -- and pinning a runner to one
# architecture's digest breaks the other silently. Measured: all eight answer
# 200 with `application/vnd.oci.image.index.v1+json` under this Accept.
manifest_list_types='application/vnd.oci.image.index.v1+json'
manifest_list_types="$manifest_list_types, application/vnd.docker.distribution.manifest.list.v2+json"

# Mint an anonymous pull token for a package.
#
# Exit: 0 token on stdout, 10 no such package, 1 the registry could not be read.
token_for() {
  local svc=$1 code body rc
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
      # version-tagged image" lines, exit 0, and the run green having asserted
      # nothing. That is the exact failure the `|| true` above it was removed
      # to close.
      return 10 ;;
    *) echo "$svc: token endpoint answered HTTP $code" >&2; return 1 ;;
  esac
  python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])' <"$body" || {
    echo "$svc: token endpoint returned something that is not a token" >&2; return 1; }
}

# Every tag, following pagination. `n=1000` fits all 117 of ingot's in one
# request today, but a page size is not a guarantee: the registry answers with
# `Link: <...?last=<tag>&n=...>; rel="next"` when it has truncated, so the loop
# follows that rather than trusting one request to be complete. Only `last` is
# taken from it, so the page size stays the one this script chose rather than
# whatever the registry echoes back. An earlier revision justified that with
# "GHCR's own next link carries `n=0`", which is not true of the request this
# script makes: measured, GHCR echoes the `n` you sent (`?n=100` comes back
# `n=100`, `?n=5` comes back `n=5`) and only answers `n=0` when you send none.
#
# Order is NOT lexicographic and not version order: ingot's first page of 100
# held exactly one version-shaped tag out of 117. Sorting happens below, over
# the whole set.
#
# Exit: 0 answered (tags on stdout, possibly none), 10 no such package,
#       1 the registry could not be read.
tags_for() {
  local svc=$1 tok code last url body rc
  set +e; tok=$(token_for "$svc"); rc=$?; set -e
  [ "$rc" -eq 0 ] || return "$rc"

  body=$(mktemp)
  # shellcheck disable=SC2064
  trap "rm -f '$body'" RETURN

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
    # NO `| head`, for the reason newest_version_from gives: an early-exit
    # reader under `set -o pipefail` can kill the writer with SIGPIPE and
    # abort the script. The header file is far below a pipe buffer so the
    # writer would in practice always finish first -- but "in practice always"
    # is what the other one looked like too, and taking the first line in the
    # shell costs nothing and removes the shape.
    last=$(sed -n 's/.*[?&]last=\([^&>]*\).*rel="next".*/\1/p' "$hdr")
    last=${last%%$'\n'*}
    rm -f "$hdr"
    if [ -n "$last" ]; then
      url="$api/v2/fil-forge/${svc}/tags/list?n=1000&last=$last"
    else
      url=
    fi
  done
}

# Resolve one tag to its index digest. Prints `sha256:...` on stdout.
#
# Read from the RESPONSE HEADER rather than computed from the body, because
# the digest has to match what the registry will serve it under and only the
# registry can say that -- a re-serialised body hashes to something else.
#
# Exit: 0 digest on stdout, 1 anything else. There is no "no such tag" exit,
# on purpose: this is only called for `:main`, and a fleet service whose
# `:main` cannot be resolved is a hard failure either way.
digest_for() {
  local svc=$1 tag=$2 tok code hdr rc
  set +e; tok=$(token_for "$svc"); rc=$?; set -e
  if [ "$rc" -ne 0 ]; then
    echo "$svc: could not mint a token to resolve :$tag" >&2; return 1
  fi

  hdr=$(mktemp)
  # shellcheck disable=SC2064
  trap "rm -f '$hdr'" RETURN

  code=$(curl -sS -I -D "$hdr" -o /dev/null -w '%{http_code}' \
         -H "Authorization: Bearer $tok" -H "Accept: $manifest_list_types" \
         "$api/v2/fil-forge/${svc}/manifests/${tag}") || {
    echo "$svc: could not read the manifest for :$tag" >&2; return 1; }
  if [ "$code" != 200 ]; then
    echo "$svc: manifest for :$tag answered HTTP $code" >&2; return 1
  fi

  local dig
  dig=$(tr -d '\r' <"$hdr" | sed -n 's/^[Dd]ocker-[Cc]ontent-[Dd]igest: //p')
  dig=${dig%%$'\n'*}
  # Checked rather than trusted: an empty value here would go out as a
  # baseline of the empty string and pull `<pkg>@`, which fails far from
  # here. The shape is checked too -- anything that is not a sha256 digest is
  # the registry answering a question this script did not ask.
  case "$dig" in
    sha256:[0-9a-f]*)
      [ "${#dig}" -eq 71 ] || {
        echo "$svc: :$tag resolved to a malformed digest ($dig)" >&2; return 1; }
      printf '%s\n' "$dig" ;;
    *)
      echo "$svc: :$tag carried no usable Docker-Content-Digest header" >&2
      return 1 ;;
  esac
}

# The NEWEST version-shaped tag, or nothing. Excludes main, main-dev, latest
# and sha-*, and also the per-architecture `0.0.0-amd64` / `0.0.0-arm64`
# variants ingot publishes, which are not runnable as a multi-arch reference.
#
# IT TAKES THE HEAD ITSELF rather than being piped into `head -n 1`, and that
# is a bug fix rather than tidiness. Under `set -o pipefail`, `head` closing
# the pipe after one line races python's write: the print takes
# BrokenPipeError, the pipeline reports non-zero, and `set -e` aborts the
# whole script with an empty stdout.
#
# WHEN IT FIRES: never idle, and often enough under CPU load to redden a job
# -- measured between 1% and 55% of runs across two machines and five load
# levels, so there is no single rate to quote and an earlier revision of this
# comment was wrong to quote one. It needs MORE THAN ONE version-shaped tag
# to be emitted, which no fleet service produces today (piri, ingot and
# indexing-service publish one each; the other five none). So it has never
# fired in a real run, and it arrives with the second release of any service.
#
# The signature is rc=1, empty stdout, and a BrokenPipeError traceback on
# stderr naming `<string>` line 4 -- which says a python block died but not
# which one or for which service, and in a CI log sits under whatever the
# caller was doing. Measured: 9/9 and 93/93 aborts carried it, none silent.
# check-compat-baselines.sh's tests 3, 10 and 11 are the ones whose fixtures
# emit two or more, and they reproduce it under load; the guard cannot catch a
# regression here deterministically, so the defence is that the pipeline is
# gone rather than that anything watches for it. How this was found, and the
# three wrong diagnoses before it, are in `git log` for this function.
newest_version_from() {
  python3 -c '
import re, sys
out = {t for t in sys.stdin.read().split() if re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", t)}
ordered = sorted(out, key=lambda v: [int(p) for p in v.split(".")], reverse=True)
print(ordered[0] if ordered else "")
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

baselines=
floating=
summary=
for svc in $services; do
  set +e
  raw=$(tags_for "$svc")
  rc=$?
  set -e
  case "$rc" in
    0) ;;
    10)
      # NOT "report none and carry on", which is what this did while a missing
      # baseline meant a skip. It no longer does: every fleet service is
      # pinned, so one with no package at all cannot be pinned to anything and
      # `:main` will not resolve either. Failing here says which service and
      # why; carrying on says `manifest unknown` from a container forty
      # minutes later.
      echo "::error::$svc has no package at $api, so the fleet has no image" \
           "to boot it from. Nothing to pin against." >&2
      exit 1 ;;
    *)
      echo "::error::could not read $svc's tags; refusing to report that as" \
           "'no releases'. A compat run pinned against nothing is green and" \
           "meaningless." >&2
      exit 1 ;;
  esac

  newest=$(printf '%s\n' "$raw" | newest_version_from)

  if [ -n "$newest" ]; then
    value=$newest
    summary="$summary| \`$svc\` | \`$newest\` | release |"$'\n'
  else
    value=$(digest_for "$svc" main) || {
      echo "::error::$svc publishes no version-tagged image and its :main" \
           "could not be resolved, so there is nothing to pin it to." >&2
      exit 1; }
    echo "$svc: no release to pin to; falling back to :main at $value" >&2
    floating="$floating $svc"
    summary="$summary| \`$svc\` | \`$value\` | **floating \`:main\`** |"$'\n'
  fi

  echo "base_$(key_for "$svc")=$value"
  baselines="$baselines$(key_for "$svc")\t$value\n"
done

if [ -n "$floating" ]; then
  # stderr, not stdout: stdout is $GITHUB_OUTPUT. GitHub reads workflow
  # commands from both streams, so the annotation still lands.
  echo "::warning::Compat baseline is a floating :main for${floating}." \
       "Those services have cut no release, so the 'old' half of the fleet is" \
       "upstream's current main rather than an older version, and the skew" \
       "this run exercises comes from the services that DO have releases." \
       "The digests are in the job summary; they are what makes a red" \
       "reproducible. This stops once every service releases (RELEASE.md)." >&2
fi

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  {
    echo "### Compat baselines"
    echo
    echo "| service | baseline | kind |"
    echo "|---|---|---|"
    printf '%s' "$summary"
    echo
    if [ -n "$floating" ]; then
      echo "A **floating \`:main\`** baseline is upstream's current main,"
      echo "pinned to the digest resolved just now. It is not an older"
      echo "version, so it contributes little skew; it is also not this"
      echo "tree, so upstream drift can redden this run. Both go away as"
      echo "services start cutting releases."
    else
      echo "Every service has a cut release. The \`:main\` fallback is no"
      echo "longer used and can come out of"
      echo "\`.github/scripts/compat-baselines.sh\`."
    fi
  } >> "$GITHUB_STEP_SUMMARY"
fi

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
