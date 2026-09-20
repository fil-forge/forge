#!/usr/bin/env bash
# Run this with every `git subtree pull` -- during one that stopped with
# conflicts, or straight after one that did not.
#
# It does two things, because a subtree pull can lose an upstream change in two
# different ways and only one of them is loud.
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
# THE SECOND THING, and the reason this is not called subtree-conflicts.sh any
# more: when upstream DELETES a file we had moved out of the prefix, both sides
# deleted that path, so git raises no conflict at all. The pull succeeds in
# silence and we go on carrying a file upstream removed. Nothing driven by
# conflicts can see that, so the audit below reads the merge itself -- upstream's
# own diff against the previous split point -- and reports deletions whose file
# we still have. It runs in both modes, including after a pull that had no
# conflicts whatsoever, which is exactly when it is the only thing looking.
#
#   finish-subtree-pull.sh <prefix>              merge, write the results, audit
#   finish-subtree-pull.sh --dry-run <prefix>    print the diffs, change nothing
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

if [ $# -eq 0 ]; then
  echo "usage: $(basename "$0") [--dry-run] <prefix>" >&2
  echo "exit 2 means it refused to run; exit 1 means it ran and found something." >&2
  exit 2
fi
prefix=$1
prefix=${prefix%/}
shift
# Refuse trailing arguments rather than ignoring them. The option loop stops at
# the first positional, so `finish-subtree-pull.sh svcA --dry-run` left dry=0
# and the script then rewrote files and the index for someone who asked for a
# preview -- and index surgery has no undo short of redoing the pull.
if [ $# -gt 0 ]; then
  echo "unexpected argument(s): $*" >&2
  echo "usage: finish-subtree-pull.sh [--dry-run] <prefix>   (flags before the prefix)" >&2
  exit 2
fi

note() { printf '%s\n' "$*" >&2; }

# Which mode we are in is detected, not flagged: a conflicted pull leaves
# MERGE_HEAD, and a finished one leaves a merge commit at HEAD whose second
# parent is the subtree split. Asking the caller to say which would be one more
# thing to get wrong on a day that is already going badly.
if git rev-parse -q --verify MERGE_HEAD >/dev/null 2>&1; then
  pull_mode=conflicted
  upstream=MERGE_HEAD
  base=$(git merge-base HEAD MERGE_HEAD)
elif git rev-parse -q --verify 'HEAD^2' >/dev/null 2>&1; then
  pull_mode=merged
  # WHICH merge, though. Binding blindly to HEAD^2 made this audit read the
  # wrong pull whenever more than one prefix had been pulled -- and a resync
  # pulls eight, so seven of eight audits looked at a merge that had nothing to
  # do with their prefix, found nothing, and exited 0 while a file upstream had
  # deleted sat in the tree. A mistyped prefix passed for the same reason.
  #
  # Anchor on the prefix's own subtree-ADD. `git subtree pull` records no
  # trailer -- only `add` does -- and the add's `git-subtree-split` names the
  # upstream root. Every later pull of that prefix has a second parent
  # descending from that root, and no other prefix's does. That is exact, and it
  # needs nothing from the pull commits themselves.
  #
  # NOT "the most recent merge whose diff is confined to the prefix". That was
  # the first attempt and it is wrong in the worst possible way: when upstream
  # deletes a file we had already moved out of the prefix, the merge changes
  # NOTHING on our side, so its diff is empty -- and an empty diff is exactly
  # the case this audit exists for. Caught on a fixture.
  add=$(git rev-list --merges HEAD --grep="git-subtree-dir: $prefix\$" --max-count=1 || true)
  if [ -z "$add" ]; then
    echo "Found no 'git subtree add' for '$prefix' in this history." >&2
    echo "Refusing rather than guessing which merge to audit. Check the prefix." >&2
    exit 2
  fi
  split=$(git log -1 --format=%B "$add" | sed -n 's/^[[:space:]]*git-subtree-split:[[:space:]]*//p' | head -1)
  if [ -z "$split" ]; then
    echo "'$prefix' has a subtree-add with no git-subtree-split trailer." >&2
    exit 2
  fi

  # Two conditions, and the second is the one that makes the first mean
  # anything. "$c^2 contains $split" alone is satisfied by ANY merge whose
  # second parent descends from the subtree add -- which is every ordinary
  # "Merge pull request #N" on main, because a feature branch cut after the add
  # contains the add, and the add contains upstream's history. Measured: with
  # only the first test, ingot, piri, hilt, smelt, sprue and delegator ALL
  # anchored on 0d8fb04c, PR #10's merge, which touches two files under
  # .github/. The prefix argument was inert. That is the same fault as the
  # "diff confined to the prefix" attempt above, arrived at from the other
  # direction, and it survived a review round.
  #
  # A real `git subtree pull` merge has upstream's tip as its second parent,
  # and upstream has never heard of this monorepo, so $add is NOT an ancestor
  # of it. A monorepo feature branch always is. That one test separates them.
  # Verified against origin/main: all ten prefixes resolve to their own pull
  # merge, or to their own add when they have never been pulled.
  merge=
  for c in $(git rev-list --merges --max-count=200 HEAD); do
    git rev-parse -q --verify "$c^2" >/dev/null 2>&1 || continue
    git merge-base --is-ancestor "$split" "$c^2" 2>/dev/null || continue
    git merge-base --is-ancestor "$add"   "$c^2" 2>/dev/null && continue
    merge=$c; break
  done
  if [ -z "$merge" ]; then
    echo "Found no merge of '$prefix''s upstream in the last 200 merges." >&2
    exit 2
  fi
  if [ "$merge" = "$add" ]; then
    note "'$prefix' has only ever been added, never pulled -- nothing to audit."
    exit 0
  fi
  if [ "$merge" != "$(git rev-parse HEAD)" ]; then
    note "Auditing $prefix's pull at $(git rev-parse --short "$merge"); later pulls sit on top."
  fi
  upstream="$merge^2"
  # Guarded: unrelated histories give an empty merge-base, and `set -e` then
  # kills the script with no message at all.
  base=$(git merge-base "$merge^1" "$merge^2" || true)
  if [ -z "$base" ]; then
    echo "No merge base between '$prefix''s pull and its upstream -- cannot diff" >&2
    echo "the upstream range, so there is nothing this audit can honestly say." >&2
    exit 2
  fi
else
  echo "No subtree pull to finish: no merge in progress, and HEAD is not a merge." >&2
  echo "Run this during a conflicted pull, or immediately after any pull." >&2
  exit 2
fi
cd "$(git rev-parse --show-toplevel)"

rename_warn=$(mktemp "${TMPDIR:-/tmp}/finish-subtree-rename.XXXXXX")
# One trap for both, set once. Two traps means the second REPLACES the first,
# so the per-file `trap 'rm -rf "$tmp"'` inside the merge loop used to discard
# this one and leak $rename_warn on every run that merged anything.
tmp=""
trap 'rm -f "$rename_warn"; [ -n "$tmp" ] && rm -rf "$tmp"' EXIT

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
  # Process substitution, not a pipe. `git show … | grep -Iq .` under pipefail
  # returns 141 for any text file bigger than the pipe buffer: grep -q exits on
  # the first match, git dies of SIGPIPE, and pipefail propagates it -- so every
  # large TEXT file was declared binary and refused. go.sum and generated
  # cbor_gen.go routinely exceed 64 KiB. Measured: 164 KB of text gives 141
  # through the pipe and 0 through process substitution.
  # `grep -Iq .` needs a non-empty LINE, so an empty file and a blank-lines-only
  # file both failed it and were refused as binary. Test emptiness first, then
  # look for a NUL the way grep -I decides.
  if [ -n "$(git show ":3:$conflicted" 2>/dev/null | head -c 1)" ] \
     && ! grep -Iq . < <(git show ":3:$conflicted" 2>/dev/null); then
    note "SKIP  $conflicted -> $dest: binary, merge by hand"
    unresolved=$((unresolved + 1)); continue
  fi

  tmp=$(mktemp -d)
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
done < <(
  if [ "$pull_mode" = conflicted ]; then
    git status --porcelain | awk '/^DU /{print substr($0,4)}'
  fi
)

if [ "$pull_mode" = conflicted ] && [ "$merged" = 0 ] && [ "$unresolved" = 0 ]; then
  note "No 'deleted by us, modified by them' conflicts. Anything else here is an"
  note "ordinary content conflict: resolve it normally."
fi

# THE AUDIT. Everything above reacts to a conflict; this reads the merge.
#
# When upstream deletes a file we had moved out of the prefix, BOTH sides
# deleted that path, so git raises nothing -- no conflict, no status entry,
# nothing for a conflict-driven tool to react to. The pull succeeds and we keep
# carrying a file upstream removed. This is the only thing that looks.
#
# Upstream's own history is in its own path space (no prefix), so its diff
# against the previous split point names deletions as upstream saw them, and we
# map each back through our prefix. -M matters: without it a rename upstream
# reads as a delete and every one would be a false positive.
stale=0
while IFS= read -r gone; do
  [ -n "$gone" ] || continue
  old=$prefix/$gone
  dest=$(destination_of "$old")
  if [ -n "$dest" ]; then
    note "STILL HERE  upstream deleted $gone; we moved it to $dest and still have it"
    stale=$((stale + 1))
    continue
  fi
  # No recorded rename does NOT mean no move. Move a file out of the prefix and
  # substantially rewrite it in the same commit and git records D+A even at
  # -M20%, so destination_of comes back empty -- and this loop used to say
  # nothing at all, on the one failure mode the whole script exists for. The
  # loud path already probes for this case and prints SKIP; the audit did not.
  # Same probe here, so a genuine delete stays quiet (nothing outside the
  # prefix has that basename) and a rewritten move is named.
  cands=$(git ls-files "*/$(basename "$gone")" | grep -v "^$prefix/" || true)
  if [ -n "$cands" ]; then
    note "VERIFY      upstream deleted $gone; no rename was recorded, but these exist"
    note "            outside the prefix: $(echo "$cands" | tr '\n' ' ')"
    stale=$((stale + 1))
  fi
done < <(git diff -M --diff-filter=D --name-only "$base" "$upstream" 2>"$rename_warn" || true)

# git prints "exhaustive rename detection was skipped" to stderr when the range
# exceeds diff.renameLimit, and -M then silently stops working -- so every
# upstream RENAME reads as a delete and the audit emits a wave of false STILL
# HERE lines, inviting an agent to delete live files. That warning used to go to
# /dev/null. It is the one message that invalidates everything above.
if [ -s "$rename_warn" ] && grep -qi 'rename' "$rename_warn"; then
  note ""
  note "RENAME DETECTION WAS SKIPPED by git, so -M did not work and every upstream"
  note "rename above reads as a delete. Do NOT act on these until you re-run with a"
  note "higher limit -- git's own message follows:"
  sed 's/^/    /' "$rename_warn" >&2
  stale=$((stale + 1))
fi

if [ "$stale" -gt 0 ]; then
  note ""
  note "$stale file(s) upstream deleted are still in this tree because we had moved"
  note "them. git raised no conflict for these -- both sides deleted the old path --"
  note "so nothing else would have mentioned them. Decide each one: upstream may have"
  note "deleted dead code we are still carrying, or may have moved it somewhere this"
  note "audit cannot see."
fi

unmerged=0
if [ "$pull_mode" = conflicted ]; then
  unmerged=$(git ls-files -u | awk '{print $4}' | sort -u | wc -l | tr -d ' ')
fi

if [ "$pull_mode" = merged ]; then
  note "Audited a completed pull: $stale upstream deletion(s) we still carry."
else
  note "$merged merged, $unresolved left for a human, $unmerged path(s) still unmerged, $stale upstream deletion(s) we still carry."
fi

# $unmerged, not just $unresolved. The status used to count only the DU class,
# so a pull whose conflicts were ordinary content conflicts -- the common case,
# fourteen of them in one resync -- exited 0 with the merge unresolved, and a
# caller acting on the status per AGENTS.md committed it.
[ "$unresolved" = 0 ] && [ "$stale" = 0 ] && [ "$unmerged" = 0 ]
