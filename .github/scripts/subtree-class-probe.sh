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
# Every row must read PASS. It cleans up its own temp dir on exit.
# Enumerate the anchoring / range / dry-run / binary / trailer class for
# finish-subtree-pull.sh. Each case builds a throwaway upstream+monorepo pair.
# A case passes when the script's OUTPUT is right, not merely its exit code --
# that distinction is the whole reason this exists.
set -uo pipefail
# Absolute: every case cds into its own temp repo, so a relative path would
# resolve against the wrong directory and the fixtures would silently run
# against no script at all.
S=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
[ -f "$S" ] || { echo "no such script: $1" >&2; exit 2; }
LAB=$(mktemp -d); trap 'rm -rf "$LAB"' EXIT
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
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
  out=$( cd "$2" && .github/scripts/finish-subtree-pull.sh svc 2>&1 ); rc=$?
  if printf '%s' "$out" | grep -qE "$3"; then v="PASS"; else v="FAIL"; fi
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

d=$(mk blank)
( cd "$d/up" || exit; printf '\n\n\n' >blank.txt; git add -A; git commit -qm 'up: blank-lines file' ) >/dev/null 2>&1
( cd "$d/mono" || exit; git fetch -q up; git subtree pull -q --prefix=svc up main -m 'subtree: pull svc'
  git mv svc/blank.txt shared/blank.txt; git commit -qm 'move blank.txt out' ) >/dev/null 2>&1
( cd "$d/up" || exit; printf '\n\n\n\n' >blank.txt; git add -A; git commit -qm 'up: touch blank' ) >/dev/null 2>&1
( cd "$d/mono" || exit; git fetch -q up
  git subtree pull -q --prefix=svc up main -m 'subtree: pull svc' ) >/dev/null 2>&1
out=$( cd "$d/mono" && .github/scripts/finish-subtree-pull.sh svc 2>&1 )
if printf '%s' "$out" | grep -q 'blank.txt.*binary'; then echo "FAIL blank-lines file called binary"
else echo "PASS blank-lines file is not called binary"; fi

d=$(mk dryc)
( cd "$d/up" || exit; printf 'package s\nL2\nL3\n' >c.txt; git add -A; git commit -qm 'up: c' ) >/dev/null 2>&1
( cd "$d/mono" || exit; git fetch -q up; git subtree pull -q --prefix=svc up main -m 'subtree: pull svc'
  git mv svc/c.txt shared/c.txt
  printf 'package s\nOURS\nL3\n' >shared/c.txt; git add -A; git commit -qm 'move+edit c' ) >/dev/null 2>&1
( cd "$d/up" || exit; printf 'package s\nTHEIRS\nL3\n' >c.txt; git add -A; git commit -qm 'up: edit c' ) >/dev/null 2>&1
( cd "$d/mono" || exit; git fetch -q up
  git subtree pull -q --prefix=svc up main -m 'subtree: pull svc' ) >/dev/null 2>&1
( cd "$d/mono" && .github/scripts/finish-subtree-pull.sh --dry-run svc >/dev/null 2>&1 ); dr=$?
if [ "$dr" -ne 0 ]; then echo "PASS dry-run flags a merge that will conflict"
else echo "FAIL dry-run green on a merge that will conflict (rc=$dr)"; fi
