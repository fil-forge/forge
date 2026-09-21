#!/usr/bin/env bash
# NOT A GUARD. The `guards` job names the check-*.sh scripts it runs and this is
# deliberately not one of them: every case builds throwaway upstream+monorepo
# repository pairs in a temp dir, which is too slow and too stateful for CI.
#
# It is the class-enumerating command rule 5 asks for. Three successive rounds
# fixed the anchoring one heuristic at a time -- "diff confined to the prefix",
# then "second parent contains the split", then "upstream has never heard of
# this monorepo", then "upstream's tree has no prefix directory" -- and each
# was verified on the fixture for the case it had just fixed. The fourth round
# found that two of them had never been right and that one verification had
# checked WHICH MERGE GOT ANCHORED rather than WHAT THE AUDIT THEN SAID.
#
# So every case here asserts on the script's OUTPUT, not its exit code. Run it
# before and after any change to the anchoring, the range derivation, the
# binary guard, the dry-run accounting or the trailer selection:
#
#     .github/scripts/subtree-class-probe.sh .github/scripts/finish-subtree-pull.sh
#
# Every row must read PASS, **and this script exits non-zero if any does not**.
# Enumerate the anchoring / range / dry-run / binary / trailer / destination
# class for finish-subtree-pull.sh. Eleven cases, each building a throwaway
# upstream+monorepo pair.
# A case passes when the script's OUTPUT is right, not merely its exit code --
# that distinction is the whole reason this exists.
set -uo pipefail
# Absolute: every case cds into its own temp repo, so a relative path would
# resolve against the wrong directory and the fixtures would silently run
# against no script at all.
S=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
[ -f "$S" ] || { echo "no such script: $1" >&2; exit 2; }
# Invoke it by the name it was GIVEN. Both call sites hardcoded
# `finish-subtree-pull.sh`, so pointing this at any other script -- a stub, an
# older revision kept beside it -- ran nothing at all and reported ten failures
# for "No such file or directory". Ten FAILs is the right verdict for a stub
# and the wrong reason, which is the distinction this file exists to make.
SN=$(basename "$S")
LAB=$(mktemp -d); trap 'rm -rf "$LAB"' EXIT
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
# The exit status is the point of a probe a caller can act on. This used to
# exit 0 whether it had run against the real script, against the round-three
# script with six FAILs, or against `echo "I refuse"; exit 2`.
fails=0
mk() { # $1 = name; echoes the mono dir. upstream has a.go + b.go; we move b.go out.
  local d=$LAB/$1; mkdir -p "$d/up" "$d/mono"
  ( cd "$d/up" || exit; git init -qb main; printf 'package s\n' >a.go; printf 'package s\n' >b.go
    git add -A; git commit -qm up-init ) >/dev/null 2>&1
  ( cd "$d/mono" || exit; git init -qb main; mkdir -p .github/scripts; cp "$S" .github/scripts/
    printf 'r\n' >README.md; git add -A; git commit -qm mono-init
    git remote add up ../up; git fetch -q up; git subtree add -q --prefix=svc up main
    mkdir -p shared; git mv svc/b.go shared/b.go; git commit -qm 'move b.go out' ) >/dev/null 2>&1
  echo "$d"
}
updel() { ( cd "$1/up" || exit; git rm -q b.go; printf 'package s\n//t\n' >a.go; git add -A
            git commit -qm 'up: delete b.go' ) >/dev/null 2>&1; }
pull()  { ( cd "$1/mono" || exit; git fetch -q up
            git subtree pull -q --prefix=svc up main -m 'subtree: pull svc' ) >/dev/null 2>&1; }
chk() { # $1 label, $2 mono dir, $3 = regex the OUTPUT must match
  local out rc
  out=$( cd "$2" && ".github/scripts/$SN" svc 2>&1 ); rc=$?
  if printf '%s' "$out" | grep -qE "$3"; then v="PASS"; else v="FAIL"; fails=$((fails+1)); fi
  printf '%s %-40s rc=%s\n' "$v" "$1" "$rc"
  [ "$v" = FAIL ] && printf '%s\n' "$out" | sed 's/^/        /' | head -4
  return 0
}

d=$(mk base);  updel "$d"; pull "$d"
chk "baseline: carried file is REPORTED" "$d/mono" 'STILL HERE.*shared/b\.go'
chk "baseline: counted as carried, not a lead" "$d/mono" '1 carried'

d=$(mk pfile)
( cd "$d/up" || exit; printf '#!/bin/sh\n' >svc; git add -A; git commit -qm 'up: a file named svc' ) >/dev/null 2>&1
updel "$d"; pull "$d"
chk "upstream has a FILE named like the prefix" "$d/mono" 'STILL HERE.*shared/b\.go'

d=$(mk pdir)
( cd "$d/up" || exit; mkdir -p svc; printf 'x\n' >svc/x; git add -A; git commit -qm 'up: a dir named svc' ) >/dev/null 2>&1
updel "$d"; pull "$d"
chk "upstream has a DIR named like the prefix" "$d/mono" 'STILL HERE.*shared/b\.go|path space'

d=$(mk fork)
( cd "$d/up" || exit; git remote add mono ../mono; git fetch -q mono
  git merge -q -s ours --allow-unrelated-histories -m 'up: fork-back' mono/main ) >/dev/null 2>&1
updel "$d"; pull "$d"
chk "fork-back: right answer OR loud refusal" "$d/mono" 'STILL HERE.*shared/b\.go|path space'

d=$(mk quote); updel "$d"; pull "$d"
( cd "$d/mono" || exit; git checkout -qb side; printf 'n\n' >n.md; git add -A; git commit -qm n
  git checkout -qm main; git merge -q --no-ff side -m 'Merge pull request #99

Explains that a subtree add records
git-subtree-dir: svc' ) >/dev/null 2>&1
chk "a commit QUOTING the trailer is not the add" "$d/mono" 'STILL HERE.*shared/b\.go'

# The same, quoting BOTH trailers at line start. Requiring both was the previous
# fix and it is not enough: the quoted split is captured, every real candidate
# then fails `--is-ancestor "$split"`, and the script exits 2 with "Found no
# merge of 'svc''s upstream in the last 200 merges" about a prefix whose file IS
# carried. A real subtree-add merges the commit its own split names, so that
# commit is an ancestor of its second parent; quoted text is not.
# The quoted split names a REAL commit that exists here -- the monorepo's own
# root -- not a string of zeros. A zeros sha is rejected by the existence check
# alone, so a fixture using one passes without the ancestry test and proves
# nothing about it.
d=$(mk quote2); updel "$d"; pull "$d"
( cd "$d/mono" || exit; git checkout -qb side2; printf 'm\n' >m.md; git add -A; git commit -qm m
  git checkout -qm main
  root=$(git rev-list --max-parents=0 HEAD | tail -1)
  git merge -q --no-ff side2 -m "Merge pull request #98

Explains that a subtree add records BOTH of
git-subtree-dir: svc
git-subtree-split: $root" ) >/dev/null 2>&1
chk "a commit quoting BOTH trailers is not the add" "$d/mono" 'STILL HERE.*shared/b\.go'

d=$(mk blank)
( cd "$d/up" || exit; printf '\n\n\n' >blank.txt; git add -A; git commit -qm 'up: blank-lines file' ) >/dev/null 2>&1
( cd "$d/mono" || exit; git fetch -q up; git subtree pull -q --prefix=svc up main -m 'subtree: pull svc'
  git mv svc/blank.txt shared/blank.txt; git commit -qm 'move blank.txt out' ) >/dev/null 2>&1
( cd "$d/up" || exit; printf '\n\n\n\n' >blank.txt; git add -A; git commit -qm 'up: touch blank' ) >/dev/null 2>&1
( cd "$d/mono" || exit; git fetch -q up
  git subtree pull -q --prefix=svc up main -m 'subtree: pull svc' ) >/dev/null 2>&1
# A POSITIVE string, not the absence of a negative one. `grep -q ... || PASS`
# passed against `echo "I refuse"; exit 2` -- a case that cannot fail is not a
# case, and this file's own header says no case here does that.
chk "blank-lines file merges, not 'binary'" "$d/mono" \
  'merged  svc/blank\.txt -> shared/blank\.txt \(clean, staged\)' 

d=$(mk dryc)
( cd "$d/up" || exit; printf 'package s\nL2\nL3\n' >c.txt; git add -A; git commit -qm 'up: c' ) >/dev/null 2>&1
( cd "$d/mono" || exit; git fetch -q up; git subtree pull -q --prefix=svc up main -m 'subtree: pull svc'
  git mv svc/c.txt shared/c.txt
  printf 'package s\nOURS\nL3\n' >shared/c.txt; git add -A; git commit -qm 'move+edit c' ) >/dev/null 2>&1
( cd "$d/up" || exit; printf 'package s\nTHEIRS\nL3\n' >c.txt; git add -A; git commit -qm 'up: edit c' ) >/dev/null 2>&1
( cd "$d/mono" || exit; git fetch -q up
  git subtree pull -q --prefix=svc up main -m 'subtree: pull svc' ) >/dev/null 2>&1
# Also a positive string. `rc != 0` alone passed against the do-nothing stub.
out=$( cd "$d/mono" && ".github/scripts/$SN" --dry-run svc 2>&1 ); dr=$?
if printf '%s' "$out" | grep -qE 'would merge with [0-9]+ conflict'; then v=PASS; else v=FAIL; fails=$((fails+1)); fi
printf '%s %-40s rc=%s\n' "$v" "dry-run NAMES the coming conflict" "$dr"
[ "$v" = FAIL ] && printf '%s\n' "$out" | sed 's/^/        /' | head -4

# TWO pulls from an upstream carrying a top-level FILE named like the prefix.
# The single-pull case above passes even with the `cat-file -e` spelling,
# because on a first pull the merge base is the subtree-add's split -- upstream
# BEFORE the blob existed. On the SECOND pull the base is an upstream commit
# that holds it, `-e` succeeds for a blob, and the path-space guard refused
# permanently with the carried file unreported. `-t` = tree is the fix.
d=$(mk pfile2)
( cd "$d/up" || exit; printf 'wrapper\n' >svc; git add -A; git commit -qm 'up: add a file named svc' ) >/dev/null 2>&1
pull "$d"
( cd "$d/up" || exit; printf 'package s\nC1\n' >c.go; git add -A; git commit -qm 'up: add c.go' ) >/dev/null 2>&1
pull "$d"
( cd "$d/mono" || exit; git mv svc/c.go shared/c.go; git commit -qm 'move c.go out' ) >/dev/null 2>&1
( cd "$d/up" || exit; git rm -q c.go; git commit -qm 'up: delete c.go' ) >/dev/null 2>&1
pull "$d"
chk "second pull past a prefix-named FILE" "$d/mono" 'STILL HERE.*shared/c\.go'

# A symlink destination must be REFUSED, not written through. `cat > "$dest"`
# follows the link: the merged text, conflict markers and all, landed in an
# unrelated tracked file while the link was untouched and the run said
# `merged ... (1 conflict(s))`. Unstaged, so `git status` shows it -- and
# `git add -A` commits it.
# The rename is recorded to a REGULAR file, which is later replaced by a
# symlink -- the only way the recorded destination can be a link, since a
# link's blob is its target string and rename detection will not match a file
# to one.
d=$(mk symln)
( cd "$d/up" || exit; printf 'package s\nL1\n' >d.go; git add -A; git commit -qm 'up: d.go' ) >/dev/null 2>&1
pull "$d"
( cd "$d/mono" || exit; git mv svc/d.go shared/d.go; git commit -qm 'move d.go out' ) >/dev/null 2>&1
( cd "$d/mono" || exit
  printf 'package s\nREAL\n' >shared/real.go
  rm shared/d.go; ln -s real.go shared/d.go
  git add -A; git commit -qm 'shared/d.go becomes a symlink to real.go' ) >/dev/null 2>&1
( cd "$d/up" || exit; printf 'package s\nTHEIRS\n' >d.go; git add -A; git commit -qm 'up: edit d.go' ) >/dev/null 2>&1
pull "$d"
chk "a symlink destination is refused" "$d/mono" 'SKIP.*symlink'


if [ "$fails" -gt 0 ]; then
  echo
  echo "$fails case(s) FAILED."
  exit 1
fi
echo
echo "every case passed."
