#!/usr/bin/env bash
# Run this WHEN a `git subtree pull` stops with conflicts.
#
# git has already worked out precisely which files upstream changed that we no
# longer have at their old path: they are the `DU` entries -- deleted by us,
# modified by them. No estimate needed, and no threshold to guess at, because
# the merge has already made that decision.
#
# What git cannot tell us is WHERE each file went. The resolution the conflict
# invites -- delete the resurrected file, it plainly belongs nowhere -- is the
# one that discards upstream's change silently, and nothing points at the file
# that actually needed it. This answers "where did it go", and prints the exact
# change to port, taken from the index rather than re-derived.
#
# With --apply it does not just report: for every file whose destination is a
# recorded rename, it performs the three-way merge git would have performed if
# its rename detection had seen outside the prefix, writing the result to the
# file's real home -- conflict markers and all where the two sides overlap --
# and leaving the index in the state git itself would have left it.
#
# Usage: subtree-conflicts.sh [--apply] <prefix>
set -euo pipefail

apply=0
while [ $# -gt 0 ]; do
  case "$1" in
    --apply) apply=1; shift ;;
    --) shift; break ;;
    -*) echo "unknown flag: $1" >&2; exit 2 ;;
    *) break ;;
  esac
done

prefix=${1:?usage: subtree-conflicts.sh [--apply] <prefix>   (run during a conflicted subtree pull)}
prefix=${prefix%/}

git rev-parse -q --verify MERGE_HEAD >/dev/null 2>&1 || {
  echo "No merge in progress. Run this while a subtree pull is conflicted." >&2
  exit 2
}

cd "$(git rev-parse --show-toplevel)"

# Perform the merge git would have performed had it followed the rename.
#
# The three inputs are already sitting in the index: stage 1 is the merge base
# and stage 3 is upstream, both at the OLD path, and our side is the file at its
# new home. That is an ordinary three-way merge, so `git merge-file` does it --
# same engine, same markers, and its exit status is the number of conflicts.
#
# Afterwards the index is left the way git leaves a merge it did itself: a clean
# result is staged, and a conflicted one gets stages 1/2/3 at the NEW path, so
# `git status` reports UU there, mergetool works on it, and `git add` resolves
# it. Nothing here is a private convention the rest of git has to be told about.
merge_into() {
  local conflicted=$1 dest=$2 mode ours rc tmp
  if ! git diff --quiet -- "$dest" 2>/dev/null; then
    echo "   NOT APPLIED — $dest has uncommitted changes; refusing to overwrite."
    return 1
  fi
  # merge-file is a line-based text merge; on a binary it produces plausible
  # garbage rather than failing, which is the worst way to be wrong.
  if ! git show ":3:$conflicted" | grep -Iq . 2>/dev/null; then
    echo "   NOT APPLIED — upstream's version is binary; merge by hand."
    return 1
  fi
  read -r mode ours _ <<<"$(git ls-files -s "$dest")"
  tmp=$(mktemp -d)
  git show ":1:$conflicted" > "$tmp/base"
  git show ":3:$conflicted" > "$tmp/theirs"
  git show ":0:$dest"       > "$tmp/ours"

  rc=0
  git merge-file -p --diff3 \
    -L "ours: $dest" -L "base: $conflicted" -L "upstream: $conflicted" \
    "$tmp/ours" "$tmp/base" "$tmp/theirs" > "$tmp/merged" || rc=$?

  if [ "$rc" -lt 0 ] 2>/dev/null || [ "$rc" -gt 127 ]; then
    echo "   NOT APPLIED — git merge-file failed (rc=$rc); merge by hand."
    rm -rf "$tmp"; return 1
  fi

  cat "$tmp/merged" > "$dest"
  git rm -q --force "$conflicted"

  if [ "$rc" = 0 ]; then
    git add -- "$dest"
    echo "   APPLIED CLEANLY into $dest, and staged. $conflicted removed."
  else
    # Re-create the conflict at the path it belongs to, so git owns it from here.
    git update-index --force-remove "$dest"
    printf '%s %s %d\t%s\n' "$mode" "$(git hash-object -w "$tmp/base")"   1 "$dest" \
                              "$mode" "$ours"                              2 "$dest" \
                              "$mode" "$(git hash-object -w "$tmp/theirs")" 3 "$dest" |
      git update-index --index-info
    echo "   APPLIED WITH $rc CONFLICT(S) into $dest — markers are in the file."
    echo "               git status now shows it as UU; resolve and git add it."
  fi
  rm -rf "$tmp"
  return 0
}

found=0
while IFS= read -r conflicted; do
  found=$((found + 1))
  # The conflict is reported at the path the file had upstream. That is
  # <prefix>/<path> when the file stayed inside the prefix, and the bare
  # <path> when we moved it out -- a path that matches nothing in this layout.
  case "$conflicted" in
    "$prefix"/*) old=$conflicted ;;
    *)           old=$prefix/$conflicted ;;
  esac

  echo "── $conflicted"
  [ "$old" != "$conflicted" ] &&
    echo "   (reported unprefixed; it was $old before we moved it out)"

  # Where did it go? The commit that removed it, asked with a permissive
  # threshold, since a move plus a rewrite scores low.
  del=$(git log -1 --format=%H --diff-filter=D -- "$old" || true)
  dest=""
  if [ -n "$del" ]; then
    echo "   removed by  $(git log -1 --format='%h %s' "$del")"
    dest=$(git show -M20% --name-status --format= "$del" 2>/dev/null |
      awk -v p="$old" -F'\t' '$1 ~ /^R/ && $2 == p {print $3; exit}')

    # A file can be moved more than once, and each hop is recorded against the
    # path it had at the time. Stopping at the first rename therefore reports a
    # path that no longer exists -- and reports it with no hedge, which is worse
    # than not answering. Follow the chain until the destination is a file we
    # actually have, or give up and fall through to the guess.
    hops=0
    while [ -n "$dest" ] && ! git ls-files --error-unmatch "$dest" >/dev/null 2>&1; do
      hops=$((hops + 1))
      if [ "$hops" -gt 10 ]; then dest=""; break; fi
      nextdel=$(git log -1 --format=%H --diff-filter=D -- "$dest" || true)
      if [ -z "$nextdel" ]; then dest=""; break; fi
      echo "   then by     $(git log -1 --format='%h %s' "$nextdel")"
      dest=$(git show -M20% --name-status --format= "$nextdel" 2>/dev/null |
        awk -v p="$dest" -F'\t' '$1 ~ /^R/ && $2 == p {print $3; exit}')
    done
  fi

  if [ -n "$dest" ]; then
    echo "   RENAMED to  $dest"
    if [ "$apply" = 1 ]; then
      merge_into "$conflicted" "$dest" || true
      continue
    fi
    echo "   PORT INTO   $dest, then: git rm $conflicted"
    echo "               or re-run with --apply to have this done for you"
  else
    guesses=$(git ls-files "*/$(basename "$conflicted")" | grep -v "^$prefix/" | head -3 || true)
    if [ -n "$guesses" ]; then
      echo "   no rename recorded. same-basename candidates (a GUESS, verify):"
      echo "$guesses" | sed 's/^/               /'
    else
      echo "   DELETED, not moved — nothing here has that basename."
      echo "               upstream's change has no home; confirm, then: git rm $conflicted"
    fi
  fi

  # Stage 1 is the merge base, stage 3 is upstream. Their diff IS the change
  # that needs porting -- no need to look up where we last pulled from.
  echo "   the change to port:"
  full=$(git diff ":1:$conflicted" ":3:$conflicted" 2>/dev/null || true)
  if [ -z "$full" ]; then
    echo "     (could not diff index stages)"
  else
    printf '%s\n' "$full" | sed -n '5,40p' | sed 's/^/     /'
    # This is a PREVIEW, not a patch: four header lines are dropped, the rest is
    # indented, and long changes are cut. Say how much was cut and how to get the
    # whole thing -- a truncated hunk reads exactly like a complete one.
    total=$(printf '%s\n' "$full" | wc -l)
    if [ "$total" -gt 40 ]; then
      echo "     ... $((total - 40)) more lines, not shown. The whole change:"
      echo "         git diff ':1:$conflicted' ':3:$conflicted'"
    fi
  fi
  echo
done < <(git status --porcelain | awk '/^DU /{print substr($0,4)}')

if [ "$found" = 0 ]; then
  echo "No 'deleted by us, modified by them' conflicts. Any conflicts here are"
  echo "ordinary content conflicts: resolve them normally."
  exit 0
fi

echo "$found file(s) upstream changed have no file at their old path."
if [ "$apply" = 1 ]; then
  echo "Applied where a rename was recorded. Anything left above needs a human:"
  echo "a guess is not good enough to merge on, and a true delete has no target."
else
  echo "Deleting the conflicted path alone resolves the merge and drops the change."
  echo "Re-run with --apply to merge the recorded renames automatically."
fi
