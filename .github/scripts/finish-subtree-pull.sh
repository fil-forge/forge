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
# Notes and problems go to stderr. Exit 1 if anything needed a human or was
# flagged by the audit; exit 2 if the script REFUSED TO ACT at all. A caller
# acting on the status wants "non-zero", not "1".
#
# The seven refusals, and this is the only copy of the list. A second copy in
# AGENTS.md went stale over the path-space refusal, which was added without it,
# and was born missing the 200-merge one -- two different ways for the same
# hand-maintained list to be wrong, which is why there is one copy now and it
# sits next to the code:
#
#   a bad argument, or a flag after the prefix
#   no merge to audit: no merge in progress and HEAD is not a merge
#   no `git subtree add` for the prefix: no merge carries both trailers naming
#     its own two parents
#   no merge of the prefix's upstream in the last 200 merges
#   the prefix has more than one subtree-add, so there is no pull range
#   no merge base between the pull and its upstream
#   the merge base is in THIS repository's path space, not upstream's
#
# The 200-merge cap on the anchor walk is load-bearing now that the walk passes
# over every ordinary PR merge: the worst correct anchor measured on the resync
# branch sits at position 50 of 108. Beyond the cap it exits 2 and says so.
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
  # descending from that root. Not exact on its own -- two prefixes taken from
  # ONE upstream repository share a root, and each would satisfy the other's
  # test; that is why the second condition below exists. Rule 1 forbids
  # importing another module without a human decision, and all ten upstream
  # roots here are unrelated histories, so it does not arise today.
  # It needs nothing from the pull commits themselves.
  #
  # NOT "the most recent merge whose diff is confined to the prefix". That was
  # the first attempt and it is wrong in the worst possible way: when upstream
  # deletes a file we had already moved out of the prefix, the merge changes
  # NOTHING on our side, so its diff is empty -- and an empty diff is exactly
  # the case this audit exists for. Caught on a fixture.
  # BOTH trailers, and the split is what selects the candidate rather than
  # merely being read off it. Anchoring the dir grep is not enough: a merge
  # commit whose BODY happens to contain the line `git-subtree-dir: svc` -- a
  # pull request about these very scripts is the likely carrier -- matched,
  # became $add as the most recent match, had no split trailer, and exited 2
  # with a message blaming the subtree-add. Every later pull of that prefix was
  # then unauditable, permanently. A real `git subtree add` records both.
  add=""; split=""
  while IFS= read -r c; do
    [ -n "$c" ] || continue
    cand=$(git log -1 --format=%B "$c" \
             | sed -n 's/^[[:space:]]*git-subtree-split:[[:space:]]*//p' | head -1)
    [ -n "$cand" ] || continue
    # The trailers must describe THIS merge. FOUR predicates were tried before
    # this one, each of which looked like a property, and each of which had a
    # fixture written for it that passed with the fix reverted:
    #
    #   both trailers present  -- a body quoting both at line start satisfies
    #                             it; a pull request about these scripts is the
    #                             obvious carrier.
    #   the quoted sha exists  -- defeated by quoting a real sha.
    #   it is an ANCESTOR of   -- defeated because everything in this
    #   the merge's ^2            repository is an ancestor of it. The fixture
    #                             written to prove this fix passed with the fix
    #                             reverted, which is how it was caught.
    #   the merge CREATES the  -- defeated by the `Merge pull request` that
    #   prefix                    LANDS the subtree-add branch: its first
    #                             parent is main before the import, its tree
    #                             has the prefix, and it is the single most
    #                             likely commit to quote those trailers.
    #
    # THE FOURTH IS NOT DISCARDED -- it is half of what is here. Two conditions
    # survive, and neither is sufficient alone, because each excludes an
    # impostor the other admits.
    #
    # (a) THE TRAILERS NAME THIS MERGE'S OWN PARENTS. `git subtree add` writes
    #     both: `git-subtree-split` is the commit it merged in (the second
    #     parent) and `git-subtree-mainline` is what it merged into (the
    #     first). Not inferred from the shape of the history -- the thing
    #     git-subtree literally did.
    #
    #     This is what excludes the `Merge pull request` that LANDS a
    #     subtree-add branch, which is the likeliest commit in any repository
    #     to quote those trailers. Both halves are needed: the landing merge's
    #     second parent IS the import branch tip, so quoting that tip satisfies
    #     the split half alone.
    #
    # (b) THE MERGE CREATES THE PREFIX -- absent in its first parent, present
    #     in the merge. This is what excludes `git subtree split --rejoin`,
    #     and that is not a hand-written message: git-subtree's own
    #     `rejoin_msg` writes the SAME three trailers, and a rejoin merge's
    #     parents are HEAD and the new split, so it satisfies (a) exactly.
    #     Measured -- a rejoin and a real add side by side:
    #
    #       Split 'svc/'  split==^2:YES mainline==^1:YES  ^1 has prefix: YES
    #       Add   'svc/'  split==^2:YES mainline==^1:YES  ^1 has prefix: no
    #
    #     Rule 7 forbids squashing; it says nothing about `--rejoin`, so this
    #     is reachable rather than out of policy.
    #
    # Verified on all ten real adds here, on fresh adds with and without `-m`,
    # and against a rejoin fixture.
    #
    # A `--squash` add does not reach this at all: its trailers land on the
    # non-merge `Squashed '<prefix>/' content` commit, so the `--merges` grep
    # finds nothing and the script exits 2 at the "no subtree-add" refusal.
    # Out of scope by rule 7, pre-existing, and loud.
    mline=$(git log -1 --format=%B "$c" \
              | sed -n 's/^[[:space:]]*git-subtree-mainline:[[:space:]]*//p' | head -1)
    [ "$cand"  = "$(git rev-parse -q --verify "$c^2" 2>/dev/null)" ] || continue
    [ "$mline" = "$(git rev-parse -q --verify "$c^1" 2>/dev/null)" ] || continue
    [ "$(git cat-file -t "$c^1:$prefix" 2>/dev/null)" = tree ] && continue
    [ "$(git cat-file -t "$c:$prefix"   2>/dev/null)" = tree ] || continue
    add=$c; split=$cand; break
  done < <(git rev-list --merges HEAD --grep="^git-subtree-dir: $prefix\$" || true)
  if [ -z "$add" ]; then
    echo "Found no 'git subtree add' for '$prefix' in this history -- no merge" >&2
    echo "carries BOTH a git-subtree-dir: $prefix and a git-subtree-split:" >&2
    echo "trailer. Refusing rather than guessing which merge to audit." >&2
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
  # The second test WAS "$add is not an ancestor of $c^2", on the reasoning that
  # upstream has never heard of this monorepo. That is an assumption, not a
  # fact, and it fails the moment upstream merges anything carrying our history
  # -- a fork-back, even an ancestry-only `merge -s ours` that changes no file.
  # Then the real pull merge is REJECTED, the loop falls through to the add
  # itself, and the script says "only ever been added, never pulled" and exits
  # 0, immediately after a pull, with the upstream-deleted file in the tree.
  # That is this script's own failure mode, rebuilt out of its own fix. Caught
  # on a fixture.
  #
  # Test the path space instead, which the repository already relies on
  # elsewhere: upstream's tree has no `$prefix/` directory -- that is what
  # makes it upstream -- and a monorepo branch tip always has one. It is a
  # property of the trees rather than a belief about who merged what.
  #
  # Verified: byte-identical anchors to the ancestry test on origin/main (10 of
  # 10 prefixes) and on the resync branch (10 of 10, worst anchor at position
  # 50 of 108 merges), AND correct on the fork-back fixture where the ancestry
  # test silently exits 0. Wrong only if an upstream repository carried a
  # top-level directory named exactly like its prefix; none of the ten does.
  merge=
  for c in $(git rev-list --merges --max-count=200 HEAD); do
    git rev-parse -q --verify "$c^2" >/dev/null 2>&1 || continue
    git merge-base --is-ancestor "$split" "$c^2" 2>/dev/null || continue
    # A `git subtree split --rejoin` is a split of OUR OWN prefix, never a pull
    # of upstream, and it must not anchor one. Nothing above rejects it: its ^2
    # is a fresh split commit, so it contains $split and carries no prefix tree
    # -- it fails BOTH rejection tests below -- and being newer than the real
    # pull it wins the scan outright. The audit then diffs upstream against
    # upstream and reports a clean pull. Measured on a fixture: a true
    # `1 carried` / exit 1 becomes `0 carried` / exit 0 with the carried file
    # still in the tree, which is this script's own failure mode rebuilt for the
    # second time (the fork-back comment below is the first).
    #
    # git-subtree writes the trailer pair naming a merge's own two parents for
    # exactly two things it makes: the add and the rejoin. `cmd_merge` and
    # `cmd_pull` write no trailers, and `--squash` puts them on a non-merge
    # commit. The add CREATES the prefix; a rejoin runs on one already there.
    # So trailers naming both parents AND a ^1 that already has the prefix is a
    # rejoin and nothing else is -- the same table the add selection above uses,
    # read the other way up. Deliberately not restricted to THIS prefix: a
    # rejoin of any prefix is a split of our tree, so none of them is ever the
    # merge wanted here.
    rj_s=$(git log -1 --format=%B "$c" \
             | sed -n 's/^[[:space:]]*git-subtree-split:[[:space:]]*//p' | head -1)
    rj_m=$(git log -1 --format=%B "$c" \
             | sed -n 's/^[[:space:]]*git-subtree-mainline:[[:space:]]*//p' | head -1)
    if [ -n "$rj_s" ] && [ -n "$rj_m" ] \
       && [ "$(git rev-parse -q --verify "${rj_s}^{commit}" 2>/dev/null)" \
          = "$(git rev-parse -q --verify "$c^2" 2>/dev/null)" ] \
       && [ "$(git rev-parse -q --verify "${rj_m}^{commit}" 2>/dev/null)" \
          = "$(git rev-parse -q --verify "$c^1" 2>/dev/null)" ] \
       && [ "$(git cat-file -t "$c^1:$prefix" 2>/dev/null)" = tree ]; then continue; fi
    # BOTH, not either. Each test alone has its own blind spot and they are
    # different ones, so a candidate is rejected only when they agree:
    #   ancestry alone   -- fails on a fork-back, where upstream's tip contains
    #                       our history and the real pull merge is rejected;
    #   path space alone -- fails when upstream carries a top-level entry named
    #                       like the prefix. `git cat-file -e` succeeds for a
    #                       BLOB as well as a tree, so an ordinary wrapper
    #                       script named `svc` is enough; the previous comment
    #                       said "directory" and it never was.
    # Neither is a property on its own, which is what the previous two
    # revisions each claimed of theirs. Requiring agreement is: a monorepo
    # branch tip satisfies both, upstream's tip satisfies at most one.
    if git merge-base --is-ancestor "$add" "$c^2" 2>/dev/null \
       && [ "$(git cat-file -t "$c^2:$prefix" 2>/dev/null)" = tree ]; then continue; fi
    merge=$c; break
  done
  if [ -z "$merge" ]; then
    echo "Found no merge of '$prefix''s upstream in the last 200 merges." >&2
    exit 2
  fi
  if [ "$merge" = "$add" ]; then
    # "Only ever been added" is only true if there is ONE add. Rule 7's rebuild
    # does a fresh `git subtree add` of a prefix that already has one, and then
    # $add is the newest of two: the audit has no range to diff (this add's
    # split is upstream's tip) but the prefix has a pull history the sentence
    # would deny. Refuse loudly rather than report nothing and exit 0 -- that
    # is the shape of failure this script exists to remove.
    # The SAME predicate the selection above uses, or the two disagree and this
    # refusal fires on a prefix the selection never treated as twice-added --
    # or, worse, fails to fire on one it did.
    #
    # Rule 7's rebuild is safe under it: `git subtree add` REFUSES an existing
    # prefix (`fatal: prefix 'svc' already exists.`), so a rebuild must
    # `git rm -r` first, and the add's first parent therefore does not carry
    # the prefix. A previous revision of this comment claimed the opposite and
    # used it to justify dropping condition (b); the claim was untested and
    # wrong in both directions -- a genuinely twice-added prefix counts 2
    # either way.
    adds=$(git rev-list --merges HEAD --grep="^git-subtree-dir: $prefix\$" \
             | while IFS= read -r c; do
                 body=$(git log -1 --format=%B "$c")
                 cand=$(printf '%s' "$body" \
                          | sed -n 's/^[[:space:]]*git-subtree-split:[[:space:]]*//p' | head -1)
                 mline=$(printf '%s' "$body" \
                          | sed -n 's/^[[:space:]]*git-subtree-mainline:[[:space:]]*//p' | head -1)
                 [ -n "$cand" ] || continue
                 [ "$cand"  = "$(git rev-parse -q --verify "$c^2" 2>/dev/null)" ] || continue
                 [ "$mline" = "$(git rev-parse -q --verify "$c^1" 2>/dev/null)" ] || continue
                 [ "$(git cat-file -t "$c^1:$prefix" 2>/dev/null)" = tree ] && continue
                 [ "$(git cat-file -t "$c:$prefix"   2>/dev/null)" = tree ] || continue
                 echo x
               done | wc -l | tr -d ' ')
    if [ "$adds" -gt 1 ]; then
      echo "'$prefix' has $adds subtree-adds, and the newest is the most recent" >&2
      echo "thing that happened to it, so there is no pull range to audit -- but" >&2
      echo "the prefix DOES have earlier history, so \"never pulled\" would be a lie." >&2
      echo "Re-adding at the SAME upstream commit (rule 7's rebuild) deletes nothing" >&2
      echo "upstream, so there is nothing to find; re-adding at a LATER one skips an" >&2
      echo "upstream range this audit cannot reconstruct. Check that range by hand." >&2
      exit 2
    fi
    # This sentence rests on an assumption the script cannot check, and says so
    # rather than asserting a fact. The anchor search rejects a candidate whose
    # second parent both descends from the add and carries a `$prefix/` tree,
    # because that is what a monorepo branch tip looks like and upstream's tip
    # is not supposed to. If upstream has merged this repository's history and
    # layout -- a hand-made fork-back; `git subtree push` cannot produce it,
    # since it pushes rewritten commits in upstream's path space -- then its tip
    # looks exactly like an ordinary `Merge pull request` second parent, by
    # ancestry and by path space alike, and those are every signal available
    # without a remote. Four review rounds each proposed a discriminator and
    # each was measured wrong; a counter of rejections false-positives on 15 to
    # 26 ordinary merges per prefix at origin/main. DECIDED, not open: we do not
    # expect a fork-back here, so it is left unhandled deliberately -- no fifth
    # heuristic, and no silent claim either. The note below is the whole of the
    # mitigation, and AGENTS.md says the same.
    note "'$prefix' has only ever been added, never pulled -- nothing to audit."
    note "  (That reads the history: of the last 200 merges, none has a second"
    note "   parent descending from the add's split WITHOUT also looking like a branch of"
    note "   this repository. An upstream that has merged this repository's own"
    note "   history and layout would be indistinguishable from one, and its pull"
    note "   would be missed here.)"
    exit 0
  fi
  if [ "$merge" != "$(git rev-parse HEAD)" ]; then
    note "Auditing $prefix's pull at $(git rev-parse --short "$merge"); it is not HEAD."
  fi
  upstream="$merge^2"
  # Guarded: unrelated histories give an empty merge-base, and `set -e` then
  # kills the script with no message at all.
  base=$(git merge-base "$merge^1" "$merge^2" || true)
  # The anchor being right does not make the RANGE right. After a fork-back the
  # merge base is a MONOREPO commit, so `git diff $base $upstream` compares two
  # path spaces: it names monorepo paths upstream never had and MISSES the file
  # actually carried. The previous round verified this case by checking which
  # merge got anchored and never read what the audit then said -- exit 1 was
  # taken for correctness while the answer was wrong in both directions.
  # Upstream's path space has no `$prefix/`; if the base does, the diff is
  # meaningless and saying so is the only honest option.
  # `cat-file -t` and compare to `tree`, NOT `cat-file -e`. This guard was added
  # one screen below a comment diagnosing exactly this: `-e` succeeds for a BLOB
  # as well as a tree, so an upstream carrying a top-level FILE named like the
  # prefix makes the merge base "inside our path space" on the SECOND pull --
  # the base is then upstream's own commit, which holds that blob -- and this
  # refuses permanently, with the carried file unreported. The diagnosis was in
  # the file and the new code did not apply it.
  if [ -n "$base" ] && [ "$(git cat-file -t "$base:$prefix" 2>/dev/null)" = tree ]; then
    echo "'$prefix''s merge base is inside this repository's path space, not" >&2
    echo "upstream's -- a fork-back, or an upstream carrying the prefix name." >&2
    echo "The audit would diff two different layouts and report nonsense in" >&2
    echo "both directions. Refusing rather than misreading; check by hand." >&2
    exit 2
  fi
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
# Counted apart from $merged, because a dry run that "would merge with N
# conflict(s)" is not a clean preview. $merged increments for rc>0 exactly as
# for rc=0, so the previous revision's summary said "0 path(s) still unmerged"
# about a run that will leave one, and exited 0 -- the mirror image of the bug
# it was fixing, in the preview whose whole purpose is to signal
# "this would all resolve".
would_conflict=0

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
  # A symlink destination is refused, not merged. The write below is
  # `cat "$tmp/$dest" > "$dest"`, and a redirect FOLLOWS a symlink: the merged
  # text -- conflict markers and all -- landed in whatever the link pointed at,
  # an unrelated tracked file, while the link itself was untouched and the run
  # reported `merged ... (1 conflict(s))`. It is unstaged, so `git status` shows
  # it, and `git add -A` commits it. Mode 120000 is a symlink in the index;
  # `git show ":0:$dest"` on one gives the link TARGET, so there is nothing
  # sensible to three-way merge here either.
  if [ "$(git ls-files -s -- "$dest" | cut -d' ' -f1)" = 120000 ]; then
    note "SKIP  $conflicted -> $dest: destination is a symlink, merge by hand"
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
  # look for a NUL the way grep -I decides -- `grep -Iq ''`, with an EMPTY
  # pattern. The previous revision fixed the emptiness half and left the probe
  # as `grep -Iq .`, so a blank-lines-only file went on being called binary: a
  # case that worked before that change and stopped working after it, under a
  # comment claiming it was fixed.
  #
  # Size, not the first byte. `$(... | head -c 1)` was the emptiness test, and
  # bash DROPS NUL bytes in command substitution -- so a file whose first byte
  # is \0, which is to say a great many binaries, substituted to the empty
  # string, the && short-circuited, and `grep -Iq` never ran. The binary guard
  # was bypassed by exactly the files it is for. Nothing broke only because
  # `git merge-file` refuses binaries on its own and rc > 127 catches it, which
  # is a second mechanism, not this one. `git cat-file -s` reads the size from
  # the object header and touches no content.
  if [ "$(git cat-file -s ":3:$conflicted" 2>/dev/null || echo 0)" -gt 0 ] \
     && ! grep -Iq '' < <(git show ":3:$conflicted" 2>/dev/null); then
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
    merged=$((merged + 1))
    [ "$rc" = 0 ] || would_conflict=$((would_conflict + 1))
    rm -rf "$tmp"; continue
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
# Two counters, because they are two different claims. $stale is "upstream
# deleted it, we moved it, we still have it" -- verified through our own rename
# chain. $guessed is "upstream deleted it and SOMETHING with that basename
# exists outside the prefix", which is a lead, not a fact. Folding the second
# into the first made the summary assert "because we had moved them" about
# files nobody moved. Measured collision rate of distinct basenames in a prefix
# that also exist outside it: delegator 81%, sprue 40%, ingot 23%, hilt 23%,
# piri 20%, smelt 16% -- so most genuine upstream deletions would have produced
# a false claim. Both still make the exit non-zero; only the wording differs.
guessed=0
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
  #
  # Both pathspecs: `*/x` does not match `x` at the repository root, and
  # consolidating a service file to the root is a thing this monorepo does.
  cands=$(git ls-files "*/$(basename "$gone")" "$(basename "$gone")" \
          | grep -v "^$prefix/" | sort -u || true)
  if [ -n "$cands" ]; then
    note "VERIFY      upstream deleted $gone; no rename was recorded. Same basename"
    note "            outside the prefix: $(echo "$cands" | tr '\n' ' ')"
    note "            -- a lead, not a match. Check whether it is the same file."
    guessed=$((guessed + 1))
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
if [ "$guessed" -gt 0 ]; then
  note ""
  note "$guessed further upstream deletion(s) have a same-basename file outside the"
  note "prefix. No rename was recorded for these, so this is a lead and not a"
  note "finding: it is equally the shape of a genuine delete next to an unrelated"
  note "file of the same name. Confirm each before acting on it."
fi

unmerged=0
# Not in dry-run mode. A dry run stages nothing by definition, so every path is
# still unmerged in the index and this term made --dry-run exit 1 even when it
# had just said everything would merge cleanly -- while the real run on the same
# tree exited 0. A caller under `set -e` aborted on the preview, and the preview
# could no longer signal "this would all resolve", which is the one thing it is
# for. The dry run's own counters already carry the answer.
if [ "$pull_mode" = conflicted ] && [ "$dry" != 1 ]; then
  unmerged=$(git ls-files -u | awk '{print $4}' | sort -u | wc -l | tr -d ' ')
fi

if [ "$pull_mode" = merged ]; then
  note "Audited a completed pull: $stale carried, $guessed to confirm."
else
  if [ "$dry" = 1 ]; then
    note "$merged would merge, $would_conflict of them WITH CONFLICTS, $unresolved left for a human, $stale carried, $guessed to confirm."
  else
    note "$merged merged, $unresolved left for a human, $unmerged path(s) still unmerged, $stale carried, $guessed to confirm."
  fi
fi

# $unmerged, not just $unresolved. The status used to count only the DU class,
# so a pull whose conflicts were ordinary content conflicts -- the common case,
# fourteen of them in one resync -- exited 0 with the merge unresolved, and a
# caller acting on the status per AGENTS.md committed it.
# $would_conflict is in the exit for the same reason $unmerged is: a preview
# that says "would merge with 1 conflict(s)" has not previewed a clean merge,
# and a caller acting on the status would commit one.
[ "$unresolved" = 0 ] && [ "$stale" = 0 ] && [ "$guessed" = 0 ] \
  && [ "$unmerged" = 0 ] && [ "$would_conflict" = 0 ]
