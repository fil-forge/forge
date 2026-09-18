#!/usr/bin/env bash
# Finish the merge a `git subtree pull` could not follow.
#
# When we move a file out of a subtree prefix, git's rename detection stops
# following it -- it does not see outside the prefix -- so every later upstream
# change to that file arrives as `deleted by us, modified by them` at a path
# that exists nowhere in this layout. The resolution it invites, `git rm` on the
# resurrected file, discards upstream's change silently, because the moved copy
# is never flagged at all.
#
# Everything needed to merge it properly is already in the index: stage 1 is the
# merge base and stage 3 is upstream, both at the OLD path, and our side is the
# file at its new home. That is an ordinary three-way merge, so `git merge-file`
# performs it -- same engine, same markers, same conflict semantics.
#
# Afterwards the index is left the way git leaves a merge it did itself: a clean
# result is staged, a conflicted one gets stages 1/2/3 at the NEW path. From
# there `git status` reports UU, mergetool works, and `git add` resolves it.
# Nothing downstream has to be told this script was involved.
#
#   subtree-conflicts.sh <prefix>              merge, and write the results
#   subtree-conflicts.sh --dry-run <prefix>    print the diffs, change nothing
#
# stdout is diffs and nothing else, so --dry-run can be read, graded or piped.
# Notes and problems go to stderr. Exit 1 if anything needed a human.
set -euo pipefail

dry=0
while [ $# -gt 0 ]; do
  case "$1" in
    --dry-run|-n) dry=1; shift ;;
    --) shift; break ;;
    -*) echo "unknown flag: $1" >&2; exit 2 ;;
    *) break ;;
  esac
done

prefix=${1:?usage: subtree-conflicts.sh [--dry-run] <prefix>   (run during a conflicted subtree pull)}
prefix=${prefix%/}

git rev-parse -q --verify MERGE_HEAD >/dev/null 2>&1 || {
  echo "No merge in progress. Run this while a subtree pull is conflicted." >&2
  exit 2
}
cd "$(git rev-parse --show-toplevel)"

note() { printf '%s\n' "$*" >&2; }

# Where does <old path> live now? Follows our rename chain, verifying each hop
# against the index: a file can be moved more than once, and each hop is
# recorded against the path it had at the time, so reading only the first rename
# yields a path that no longer exists. Prints nothing; echoes the destination,
# or nothing if there is no verified one.
destination_of() {
  local old=$1 del dest hops=0
  del=$(git log -1 --format=%H --diff-filter=D -- "$old" || true)
  [ -n "$del" ] || return 0
  dest=$(git show -M20% --name-status --format= "$del" 2>/dev/null |
    awk -v p="$old" -F'\t' '$1 ~ /^R/ && $2 == p {print $3; exit}')
  while [ -n "$dest" ] && ! git ls-files --error-unmatch "$dest" >/dev/null 2>&1; do
    hops=$((hops + 1))
    [ "$hops" -gt 10 ] && return 0
    del=$(git log -1 --format=%H --diff-filter=D -- "$dest" || true)
    [ -n "$del" ] || return 0
    dest=$(git show -M20% --name-status --format= "$del" 2>/dev/null |
      awk -v p="$dest" -F'\t' '$1 ~ /^R/ && $2 == p {print $3; exit}')
  done
  printf '%s' "$dest"
}

unresolved=0
merged=0

while IFS= read -r conflicted; do
  # The conflict is reported at the path the file had UPSTREAM: <prefix>/<path>
  # when it stayed inside the prefix, bare <path> when we moved it out.
  case "$conflicted" in
    "$prefix"/*) old=$conflicted ;;
    *)           old=$prefix/$conflicted ;;
  esac

  # A rename/delete is reported at upstream's NEW path, which never existed in
  # our history, so our log knows nothing about it. MERGE_HEAD is in upstream's
  # own path space (no prefix), so its diff against the split point names the
  # rename and gives us back the path we actually had.
  if ! git log -1 --format=%H --diff-filter=D -- "$old" | grep -q .; then
    upold=$(git diff -M --name-status "$(git merge-base HEAD MERGE_HEAD)" MERGE_HEAD 2>/dev/null |
      awk -v n="${conflicted#"$prefix"/}" -F'\t' '$1 ~ /^R/ && $3 == n {print $2; exit}')
    [ -n "$upold" ] && old=$prefix/$upold
  fi

  dest=$(destination_of "$old")

  if [ -z "$dest" ]; then
    # No verified destination: a same-basename match is a guess, not grounds to
    # merge, and a true delete has no target at all. Both are a human's call.
    guess=$(git ls-files "*/$(basename "$conflicted")" | grep -v "^$prefix/" | head -3 || true)
    if [ -n "$guess" ]; then
      note "SKIP  $conflicted: no rename recorded; same-basename candidates (verify): $(echo $guess)"
    else
      note "SKIP  $conflicted: deleted, not moved -- upstream's change has no destination here"
    fi
    unresolved=$((unresolved + 1))
    continue
  fi

  if ! git diff --quiet -- "$dest" 2>/dev/null; then
    note "SKIP  $conflicted -> $dest: destination has uncommitted changes"
    unresolved=$((unresolved + 1)); continue
  fi
  # merge-file is a line-based text merge; on a binary it produces plausible
  # garbage rather than failing, which is the worst way to be wrong.
  if ! git show ":3:$conflicted" | grep -Iq . 2>/dev/null; then
    note "SKIP  $conflicted -> $dest: binary, merge by hand"
    unresolved=$((unresolved + 1)); continue
  fi

  tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
  mkdir -p "$tmp/$(dirname "$dest")"
  git show ":1:$conflicted" > "$tmp/base"
  git show ":3:$conflicted" > "$tmp/theirs"
  git show ":0:$dest"       > "$tmp/ours"
  rc=0
  git merge-file -p --diff3 \
    -L "ours: $dest" -L "base: $conflicted" -L "upstream: $conflicted" \
    "$tmp/ours" "$tmp/base" "$tmp/theirs" > "$tmp/$dest" || rc=$?
  if [ "$rc" -gt 127 ]; then
    note "SKIP  $conflicted -> $dest: git merge-file failed (rc=$rc)"
    unresolved=$((unresolved + 1)); rm -rf "$tmp"; continue
  fi

  if [ "$dry" = 1 ]; then
    # A real unified diff on stdout, not a rendering of one. --no-index is the
    # only way to diff a file against content that is not in the object store
    # yet; the b/ side naming a temp path is the honest cost of not writing it.
    git --no-pager diff --no-index -- "$dest" "$tmp/$dest" || true
    note "$([ "$rc" = 0 ] && echo "would merge cleanly" || echo "would merge with $rc conflict(s)")  $conflicted -> $dest"
    merged=$((merged + 1)); rm -rf "$tmp"; continue
  fi

  cat "$tmp/$dest" > "$dest"
  git rm -q --force "$conflicted"
  if [ "$rc" = 0 ]; then
    git add -- "$dest"
    note "merged  $conflicted -> $dest (clean, staged)"
  else
    # Re-create the conflict where it belongs, so git owns it from here.
    read -r mode _ _ <<<"$(git ls-files -s "$dest")"
    git update-index --force-remove "$dest"
    printf '%s %s %d\t%s\n' \
      "$mode" "$(git hash-object -w "$tmp/base")"   1 "$dest" \
      "$mode" "$(git hash-object -w "$tmp/ours")"   2 "$dest" \
      "$mode" "$(git hash-object -w "$tmp/theirs")" 3 "$dest" |
      git update-index --index-info
    note "merged  $conflicted -> $dest ($rc conflict(s); markers in the file, UU in git status)"
  fi
  merged=$((merged + 1)); rm -rf "$tmp"
done < <(git status --porcelain | awk '/^DU /{print substr($0,4)}')

if [ "$merged" = 0 ] && [ "$unresolved" = 0 ]; then
  note "No 'deleted by us, modified by them' conflicts. Anything else here is an"
  note "ordinary content conflict: resolve it normally."
  exit 0
fi
note "$merged merged, $unresolved left for a human."
[ "$unresolved" = 0 ]
