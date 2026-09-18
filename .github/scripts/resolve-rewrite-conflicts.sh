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
# tidiness: the rewrite makes an import path longer, which can move it within
# its group, and gofmt sorts groups. Without normalising, a file whose only
# difference IS the rewrite compares unequal byte for byte and gets refused.
#
#   resolve-rewrite-conflicts.sh            resolve what qualifies, stage it
#   resolve-rewrite-conflicts.sh --dry-run  say what it would do, change nothing
#
# Exit 1 if anything was left for a human. Run it after finish-subtree-pull.sh,
# which handles a different class: files we moved out of the prefix, which git's
# rename detection cannot follow. The two do not overlap -- that one works on
# `deleted by us, modified by them`, this one on ordinary content conflicts --
# and neither sees the third class, a hunk that merged CLEANLY while carrying a
# polyrepo import path. Nothing reports that but a sweep of the tree afterwards.
set -uo pipefail

dry=0
case "${1:-}" in
  --dry-run|-n) dry=1 ;;
  "") ;;
  *) echo "usage: resolve-rewrite-conflicts.sh [--dry-run]" >&2; exit 2 ;;
esac

cd "$(git rev-parse --show-toplevel)"

git rev-parse -q --verify MERGE_HEAD >/dev/null 2>&1 || {
  echo "No merge in progress. Run this during a conflicted subtree pull." >&2
  exit 2
}

# The services are the top-level directories with a go.mod -- the same set
# go.work lists. Nothing here needs updating when one is added.
svcs=$(for d in */; do [ -f "$d/go.mod" ] && printf '%s\n' "${d%/}"; done | paste -sd'|')
[ -n "$svcs" ] || { echo "no modules found -- is this the repository root?" >&2; exit 2; }

# Two rules, in order. The URL one first, because the import rule would
# otherwise turn a repository URL into .../forge/<svc>, which is a 404 that
# services serve out of GET /. (?!forge/) makes it idempotent; libforge is
# untouched because it is not one of the prefixes.
rewrite() {
  perl -pe "
    s{https://github\\.com/fil-forge/(?:$svcs)\\b}{https://github.com/fil-forge/forge}g;
    s{github\\.com/fil-forge/(?!forge/)($svcs)\\b}{github.com/fil-forge/forge/\$1}g;
  "
}

took=0 left=0
while IFS= read -r f; do
  [ -n "$f" ] || continue
  t=$(mktemp -d)

  norm() { case "$f" in *.go) gofmt "$1" 2>/dev/null || cat "$1" ;; *) cat "$1" ;; esac; }

  if ! git show ":1:$f" > "$t/base" 2>/dev/null; then
    echo "LEFT  $f: no merge base (add/add conflict)"
    left=$((left + 1)); rm -rf "$t"; continue
  fi
  git show ":2:$f" > "$t/ours"   2>/dev/null || { echo "LEFT  $f: deleted on our side; finish-subtree-pull.sh reports these"; left=$((left+1)); rm -rf "$t"; continue; }
  git show ":3:$f" > "$t/theirs" 2>/dev/null || { echo "LEFT  $f: deleted upstream; resolve by hand"; left=$((left+1)); rm -rf "$t"; continue; }

  rewrite < "$t/base" > "$t/base_rw"
  norm "$t/base_rw" > "$t/base_n"
  norm "$t/ours"    > "$t/ours_n"

  if ! cmp -s "$t/base_n" "$t/ours_n"; then
    echo "LEFT  $f: ours differs from the base by more than the rewrite:"
    diff "$t/base_n" "$t/ours_n" | head -12 | sed 's/^/        /'
    left=$((left + 1)); rm -rf "$t"; continue
  fi

  rewrite < "$t/theirs" > "$t/out_raw"
  norm "$t/out_raw" > "$t/out"

  if [ "$dry" = 1 ]; then
    echo "TAKE  $f: ours is base+rewrite only; would take upstream and re-apply it"
  else
    cat "$t/out" > "$f"
    git add -- "$f"
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
