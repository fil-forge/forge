#!/usr/bin/env bash
# Resolve the subtree-pull conflicts that are ONLY the monorepo's module-path
# rewrite, and refuse the rest.
#
# Every service arrived here with its import paths rewritten from
# github.com/fil-forge/<svc> to github.com/fil-forge/forge/<svc>. Upstream keeps
# editing those same import blocks, so a pull conflicts on every file where it
# touched one -- 14 of them across the eight prefixes of the last resync. Each
# is the same non-decision: our side says forge/<svc>, theirs says <svc>, and
# the actual change is somewhere else in the file.
#
# THE CHECK IS THE POINT, NOT THE FIX. Taking upstream's file and re-applying
# the rewrite is only safe if our side carries no judgement upstream could
# disagree with -- that is, if rewrite(base) is EXACTLY ours. This verifies that
# per file before touching any of them, and reports the ones that fail with what
# else is in there. go.mod and go.sum fail it every time, which is the check
# earning its keep: those carry real decisions (siblings pinned to v0.0.0 with
# `replace ../<svc>`, a unified libforge) that must be re-applied by hand.
#
# gofmt normalises both sides for .go files, and it is load-bearing rather than
# tidiness: the rewrite inserts `forge/` into the path, which can move the line
# WITHIN its import group, because gofmt sorts each group lexicographically.
# (Measured: `fil-forge/piri/...` sorts after `fil-forge/libforge/...`, and
# `fil-forge/forge/piri/...` sorts before it -- `forge/` < `libforge/`. Not
# because the path got longer, and gofmt never reorders the groups themselves.)
# Without normalising, a file whose only difference IS the rewrite compares
# unequal byte for byte and gets refused.
#
#   resolve-rewrite-conflicts.sh            resolve what qualifies, stage it
#   resolve-rewrite-conflicts.sh --dry-run  say what it would do, change nothing
#
# Exit 1 if anything was left for a human, 2 if it refused to act (a bad
# argument, a path outside the repository). Run it after finish-subtree-pull.sh,
# which handles a different class: files we moved out of the prefix, which git's
# rename detection cannot follow. The two do not overlap -- that one works on
# `deleted by us, modified by them`, this one on ordinary content conflicts --
# and neither sees the third class, a hunk that merged CLEANLY while carrying a
# polyrepo import path. Nothing reports that but a sweep of the tree afterwards.
set -uo pipefail

dry=0
case "${1:-}" in
  --dry-run|-n) dry=1; shift ;;
  "") ;;
  *) echo "usage: resolve-rewrite-conflicts.sh [--dry-run]" >&2; exit 2 ;;
esac
# Refused, not ignored. The sibling script had this exact fault -- a trailing
# argument silently dropped -- and it was fixed there in the round that left it
# here, in the other half of the same pull request.
if [ "$#" -gt 0 ]; then
  echo "unexpected argument(s): $*" >&2
  echo "usage: resolve-rewrite-conflicts.sh [--dry-run]   (it takes no prefix)" >&2
  exit 2
fi

cd "$(git rev-parse --show-toplevel)"

# Exit 0, not 2. AGENTS.md tells an agent to run both scripts with every pull,
# and a clean pull is the common case -- so "no merge in progress" is "nothing
# to do", not a finding. Exiting 2 made the documented pair return non-zero
# after every clean pull, in a section that also says a caller can act on the
# status rather than parse prose.
git rev-parse -q --verify MERGE_HEAD >/dev/null 2>&1 || {
  echo "No merge in progress -- nothing for this script to resolve." >&2
  exit 0
}

# Probed once, loudly. `norm` used to fall back to `cat` on any gofmt failure
# with the message discarded, so on a machine with no Go the normalisation this
# script calls load-bearing silently vanished -- and every .go file was then
# refused with a message asserting a judgement about its contents that had not
# been made.
command -v gofmt >/dev/null 2>&1 || {
  echo "gofmt is not on PATH, and the .go comparison depends on it: the rewrite" >&2
  echo "lengthens an import path, which can move it within its group, and gofmt" >&2
  echo "is what makes the two sides comparable. Refusing rather than silently" >&2
  echo "comparing unformatted bytes and blaming the file." >&2
  exit 2
}

# The services are the top-level directories with a go.mod. NOT "the same set
# go.work lists" -- go.work has twelve entries and this derivation yields ten,
# because hilt/itest and ingot/itest are nested modules rather than services.
# Ten is the right set here: they are the prefixes whose import paths were
# rewritten. Nothing here needs updating when a service is added.
# LONGEST FIRST. The hazard is real and the mechanism is simple: if `piri`
# precedes `piri-signing-service` in the alternation it matches inside it,
# rewriting it to `forge-signing-service`, a repository that does not exist.
# What is NOT established is that any locale actually orders them that way --
# the glob yields piri-signing-service first under C here, and an earlier
# revision of this comment asserted an en_US.UTF-8 flip that nobody has
# observed. So: sort by length rather than rely on a collation order this
# script does not control, and do not claim to know what that order is.
svcs=$(for d in */; do [ -f "$d/go.mod" ] && printf '%s\n' "${d%/}"; done \
  | awk '{ print length, $0 }' | sort -rn -k1,1 | cut -d' ' -f2- | paste -sd'|')
[ -n "$svcs" ] || { echo "no modules found -- is this the repository root?" >&2; exit 2; }

# ONE rule: an IMPORT PATH gains the forge/ prefix, never inside a URL.
#
# There used to be a second rule turning a bare `https://github.com/fil-forge/
# <svc>` into the forge repository URL, and it is gone because it cannot be
# made correct. It went through two rounds:
#
#   round 1 shipped it unbounded, and
#   `https://github.com/fil-forge/piri/releases/download/v1/piri.tar.gz`
#   became a 404 that the script wrote into the tree and reported success on;
#
#   round 2 added `(?![A-Za-z0-9_/-])`, which fixed that one suffix and not the
#   class -- `.` and end-of-line are not in the character class, so
#   `https://github.com/fil-forge/piri.git` still became
#   `https://github.com/fil-forge/forge.git`, the wrong repository, and a bare
#   URL at the end of a line still collapsed to the monorepo. Eleven tracked
#   lines are in that shape today: git clone commands in piri's deploy
#   scripts and piri-signing-service's and smelt's READMEs, and
#   piri/docs/mkdocs.yml's repo_url.
#
# The deeper problem is that the tree does not agree with itself. Commit
# 2b2bd5f2 ("Fix the repo URLs the module rewrite corrupted") rewrote exactly
# three Go build-info constants to .../forge and deliberately left the other
# eleven pointing at their own repositories -- and the two classes are
# indistinguishable by pattern. A rule that guesses is wrong on one of them.
#
# So: no rule. Rule 2's lookbehinds already leave every URL alone, so a file
# whose only difference is a URL is simply not "base plus the rewrite" and goes
# to a human with a diff. That is the refusal working, not a gap. Seventeen
# tracked files carry a fil-forge service URL in one form or the other; each is
# a decision the script cannot make.
rewrite() {
  perl -pe "
    s{(?<!https://)(?<!http://)github\\.com/fil-forge/(?!forge/)($svcs)(?![A-Za-z0-9_-])}{github.com/fil-forge/forge/\$1}g;
  "
}

took=0 left=0
while IFS= read -r f; do
  [ -n "$f" ] || continue
  t=$(mktemp -d)

  # gofmt's own failure on a file is meaningful (it does not parse), so it is
  # not swallowed here either -- that file goes to a human.
  norm() { case "$f" in *.go) gofmt "$1" ;; *) cat "$1" ;; esac; }

  if ! git show ":1:$f" > "$t/base" 2>/dev/null; then
    echo "LEFT  $f: no merge base (add/add conflict)"
    left=$((left + 1)); rm -rf "$t"; continue
  fi
  git show ":2:$f" > "$t/ours"   2>/dev/null || { echo "LEFT  $f: deleted on our side; finish-subtree-pull.sh reports these"; left=$((left+1)); rm -rf "$t"; continue; }
  git show ":3:$f" > "$t/theirs" 2>/dev/null || { echo "LEFT  $f: deleted upstream; resolve by hand"; left=$((left+1)); rm -rf "$t"; continue; }

  # gofmt's STATUS, not just its output. On a .go file it cannot parse it
  # writes nothing and fails -- and nothing here looked, so all three sides
  # normalised to EMPTY, `cmp` compared two empty files and passed, and the
  # write truncated a real file to zero bytes while printing TAKE and exiting
  # 0. The comment above claimed the opposite. Caught on a fixture carrying a
  # deliberate parser-error file.
  rewrite < "$t/base" > "$t/base_rw"
  if ! norm "$t/base_rw" > "$t/base_n" || ! norm "$t/ours" > "$t/ours_n"; then
    echo "LEFT  $f: gofmt could not parse it, so ours cannot be compared; merge by hand"
    left=$((left + 1)); rm -rf "$t"; continue
  fi

  if ! cmp -s "$t/base_n" "$t/ours_n"; then
    echo "LEFT  $f: ours differs from the base by more than the rewrite:"
    diff "$t/base_n" "$t/ours_n" | head -12 | sed 's/^/        /'
    left=$((left + 1)); rm -rf "$t"; continue
  fi

  rewrite < "$t/theirs" > "$t/out_raw"
  if ! norm "$t/out_raw" > "$t/out"; then
    echo "LEFT  $f: gofmt could not parse upstream's side; merge by hand"
    left=$((left + 1)); rm -rf "$t"; continue
  fi

  if [ "$dry" = 1 ]; then
    echo "TAKE  $f: ours is base+rewrite only; would take upstream and re-apply it"
  else
    # Checked. Neither was, under `set -uo pipefail` with no -e: a failed write
    # left the conflict-marked file in place and `git add` then staged content
    # containing <<<<<<< markers, while the script counted it resolved and
    # exited 0 -- so a caller acting on the status committed an unresolved
    # merge.
    if ! cat "$t/out" > "$f"; then
      echo "LEFT  $f: could not write the resolved content" >&2
      left=$((left + 1)); rm -rf "$t"; continue
    fi
    if ! git add -- "$f"; then
      echo "LEFT  $f: wrote the resolved content but could not stage it" >&2
      left=$((left + 1)); rm -rf "$t"; continue
    fi
    echo "TAKE  $f: ours is base+rewrite only; took upstream and re-applied it"
  fi
  took=$((took + 1)); rm -rf "$t"
done < <(git diff --name-only --diff-filter=U)

echo "--- $took resolved, $left left for a human ---"
if [ "$left" -gt 0 ]; then
  echo
  echo "Each one above carries something besides the rewrite; merge it by hand."
  echo "Then sweep the whole prefix for polyrepo import paths before trusting a"
  echo "build: a hunk that merged cleanly can carry one, and raises no conflict."
fi
[ "$left" = 0 ]
