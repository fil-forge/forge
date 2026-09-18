#!/usr/bin/env bash
# What is upstream about to change that has nowhere to land?
#
# Run BEFORE `git subtree pull`. A file we moved or deleted inside a subtree
# prefix cannot receive upstream's edit: git reports modify/delete at the
# UNPREFIXED path, and the obvious resolution drops the change silently. This
# lists exactly those files first, so the port is deliberate instead of a
# discovery under a conflict.
#
# Usage: subtree-orphans.sh <prefix> <remote-url> [ref]
set -euo pipefail

prefix=${1:?usage: subtree-orphans.sh <prefix> <remote-url> [ref]}
remote=${2:?usage: subtree-orphans.sh <prefix> <remote-url> [ref]}
ref=${3:-main}

# Where we last pulled this prefix from, per git subtree's own trailer. Derived,
# never recorded by hand.
# NB: no early `exit` in awk. Exiting mid-stream SIGPIPEs `git log`, and with
# `pipefail` that kills this script silently -- which a short test history is
# too small to expose.
split=$(git log --pretty=%B |
  awk -v p="$prefix" '/^git-subtree-dir:/{d=$2}
                      /^git-subtree-split:/{if(d==p && !seen){print $2; seen=1}}' |
  sed -n 1p)
[ -n "$split" ] || { echo "no git-subtree-split found for prefix '$prefix'" >&2; exit 1; }

git fetch -q "$remote" "$ref"
tip=$(git rev-parse FETCH_HEAD)

echo "prefix      $prefix"
echo "last pulled $split"
echo "upstream    $tip"
echo

orphans=0
changed=0
while IFS=$'\t' read -r status path rest; do
  # An orphan is a file that existed at the split point, that upstream has
  # since changed, and that our prefix no longer has. So:
  #   D — upstream deleted it: nothing to land.
  #   A — upstream ADDED it: it is absent from our prefix because it is new,
  #       which the pull is about to fix. Counting these was a false positive
  #       the small test history could not show; the real repo did.
  # R/C name the destination in the third field.
  case "$status" in
    D*|A*) continue ;;
    R*|C*) path=$rest ;;
  esac
  changed=$((changed + 1))
  [ -e "$prefix/$path" ] && continue          # we still have it there; the merge handles it
  orphans=$((orphans + 1))
  if [ "$orphans" = 1 ]; then
    echo "ORPHANED — upstream changed these, and $prefix/ no longer has them:"
    echo
  fi
  echo "  $prefix/$path"
  gone=$(git log -1 --format='%h %s' -- "$prefix/$path" || true)
  [ -n "$gone" ] && echo "      last seen in: $gone"
  echo "      to port:      git log -p $split..$tip -- $path"
  echo
done < <(git diff --name-status "$split..$tip")

echo "upstream touched $changed file(s) in this range; $orphans have nowhere to land."
if [ "$orphans" -gt 0 ]; then
  echo
  echo "Port each one AFTER the pull, then re-run to confirm the list is empty"
  echo "at the new split point. Resolving the conflict alone does not port them."
  exit 1
fi
