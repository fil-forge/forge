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
# Usage: subtree-conflicts.sh <prefix>
set -euo pipefail

prefix=${1:?usage: subtree-conflicts.sh <prefix>   (run during a conflicted subtree pull)}
prefix=${prefix%/}

git rev-parse -q --verify MERGE_HEAD >/dev/null 2>&1 || {
  echo "No merge in progress. Run this while a subtree pull is conflicted." >&2
  exit 2
}

cd "$(git rev-parse --show-toplevel)"

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
  fi

  if [ -n "$dest" ]; then
    echo "   RENAMED to  $dest"
    echo "   PORT INTO   $dest, then: git rm $conflicted"
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
  git diff ":1:$conflicted" ":3:$conflicted" 2>/dev/null |
    sed -n '5,40p' | sed 's/^/     /' || echo "     (could not diff index stages)"
  echo
done < <(git status --porcelain | awk '/^DU /{print substr($0,4)}')

if [ "$found" = 0 ]; then
  echo "No 'deleted by us, modified by them' conflicts. Any conflicts here are"
  echo "ordinary content conflicts: resolve them normally."
  exit 0
fi

echo "$found file(s) upstream changed have no file at their old path."
echo "Deleting the conflicted path alone resolves the merge and drops the change."
