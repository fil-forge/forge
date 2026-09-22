# forge — agent guide

The Fil Forge monorepo. Ten services, each brought in by `git subtree` with its
full history: `delegator`, `forgectl`, `hilt`, `indexing-service`, `ingot`,
`piri`, `piri-signing-service`, `smelt`, `sprue`, `swarf`. Each is its own Go
module; `go.work` ties them together.

> **This file is the short form.** The long form — why each rule exists, what
> was tried and failed, what is still open — lives on the wiki, which is the
> `wiki` branch of this repository. Read [[Current State]] before starting
> anything; it is a kept-current snapshot and it names what is in flight.

## This file is scaffolding, and should be replaced when the scaffolding comes down

Most of what follows exists because this repository is being *assembled*, not
because it is how Forge is worked on. Rules 7 and 8 are entirely about
`git subtree`; rule 1 is about what is still being moved in; the wiki and the
four-document split exist to carry decisions across a construction that spans
many sessions and one person's attention. None of that is a durable guide to a
finished monorepo, and left in place it will read as though it were.

**Replace it when the consolidation is finished** — when nothing is left to
import, no subtree pulls are pending, the `forge-2` → `forge` rename has
happened, and Phase 1's release machinery is real. That is a checkable set of
conditions rather than a feeling, on purpose; the same reason the wiki trigger
below is an event.

What a replacement probably keeps: rules 2 through 5, which are about the code
and its CI rather than about moving it, and the commands and conventions at the
end. What it probably drops: rules 1 and 6 through 9, the wiki, most of the
document table, and the per-pull-request **skippable-checks block** — by then
the wiki's content belongs in the repository or in issues, *Needs Human Work*
should be empty, and the blocks should have been replaced by real filtering
designed from what they turned out to say. Do not treat that split as settled;
decide it when you can see the finished shape.

## Where state lives, and what goes where

Four places, and putting something in the wrong one is how it gets lost:

| | holds | lifetime |
|---|---|---|
| **wiki → Current State** | what is true right now: `main`'s sha, open PRs, the rules, known debt | replaced as things change, never appended to |
| **wiki → Needs Human Work** | what is blocked on a person, and decisions taken on their behalf awaiting confirmation | items leave via *Recently cleared* |
| **`MAJOR_DECISIONS.md`** | decisions with a rationale someone will otherwise re-litigate — what is deliberately *out* of the monorepo, and why | permanent |
| **`MONOREPO_TODO.md`** | questions only answerable once the monorepo is whole, and bugs found in imported code that are not the migration's to fix | until answered |

Ordinary unfinished work belongs in issues, not in any of these.

## Update the wiki in the same turn as the thing it describes

**Not "keep it current".** That phrasing has failed here more than once: the
wiki is updated when someone happens to look, and drifts in between. The
trigger is an event, so it can be checked:

**Opened, pushed to, rebased, merged or closed a PR? Made a decision, or took
one on someone's behalf? Found something you are deliberately not fixing?**
→ update the wiki *before* you report what you did, not after.

A `main` sha or a PR list that is one merge stale is worse than no snapshot,
because it is believed. If you touched the repository and the wiki still
describes the previous state, you are not finished.

The wiki lives in two places that must stay in sync: the `wiki` branch here,
and the GitHub wiki at `forge-2.wiki.git`. Sessions can normally push only the
branch; check [[Needs Human Work]] for who is mirroring.

## Rules that have actually decided things

Numbered as the wiki numbers them; it carries the reasoning.

1. **What goes in: things that ship as the Forge network.** Outward-facing
   libraries, forks of upstream software, and things being retired stay out.
   `MAJOR_DECISIONS.md` has the list. **Do not import another module without a
   human decision** — every module in scope is already in.
2. **Build what we own; pin what we don't.** In-repo services are built from
   HEAD in CI; everything else is pinned by digest — compose images, Go
   testcontainers constants, and Dockerfile `FROM` lines alike. Pin to the
   **index** digest, never a per-architecture one, or cross-platform builds
   break silently. Reuse a digest the repository already names rather than
   resolving fresh: two builds of one tag in one repo is the failure this
   prevents.
3. **Derive dependency lists, never hand-maintain them.** `go list -deps` on
   the build target, not `go.mod`'s replace list. It is sometimes wider than
   the obvious guess — that is how `ingot → indexing-service` and
   `delegator → forgectl` were found, neither of which `go.mod` showed.
4. **Test what ships.** A bind-mounted binary in someone else's image is not
   the artifact that reaches production.
5. **A green check is a claim about what ran.** Ask what the job would have had
   to *do* to catch the fault.
   - When you fix one instance, run a command that enumerates the class before
     committing. Careful reading has missed the rest every time.
   - A guard over *part* of a chain reads exactly like a guard over the chain.
     Ship no guard rather than a partial one that looks total.
6. **Stacked branches rebase onto their base; they do not merge it.**
7. **Subtree history is never squashed, and a branch containing a
   `git subtree add` is never rebased.** A plain rebase silently flattens the
   merge that carries the imported history. When the base moves under such a
   branch, **rebuild** it: base tip, fresh `git subtree add` at the same
   upstream commit, then cherry-pick — then check the tree against the head it
   replaced and that the upstream root is still an ancestor.
8. **A PR containing a `git subtree add` links its non-subtree commits.** One
   `/pull/<n>/files/<sha>..<sha>` link per contiguous range, near the top of
   the body, refreshed on every push. The PR's own headline numbers describe
   the import, not the work.
9. **A branch meant to be reviewed and merged gets a PR when it is pushed.**
   Scratch branches do not.

## Commands

```
make -C deploy …          # per-service deploy tooling, where it exists
GOWORK=off go build ./...  # inside a module: what CI does, standalone
go build ./...             # workspace mode, across go.work
```

CI is four workflows — `ci` (per-module build/vet/staticcheck/tidy/test plus
the `guards` job), `images`, `e2e`, `itest`. **Nothing is filtered by path, on
purpose**: `ci.yml`'s header says why, and `MONOREPO_TODO.md` carries the
question of whether that should change. A documentation-only change therefore
costs a full run; that is known, not an oversight.

### Every pull request opens with which checks are safe to skip

Because nothing is path-filtered and `itest` alone is ~23 minutes, **the first
thing in a pull request body is a block naming the checks a reviewer can merge
without waiting for, and why.** Refresh it on every push, the way rule 8's
range link is refreshed.

This is the interim for the path-filtering question, and deliberately a manual
one. It avoids both faults the automatic version has: a `paths:` list goes
stale silently when a module gains a dependency, and a path-filtered job
reports *skipped*, which never satisfies a required status check. A sentence
written per push goes stale the moment it is written and is re-derived anyway,
and it can say things no filter can express — "this check already passed on
`<sha>` and nothing under `.github/` has changed since".

**Derive it; do not assert it.** The closure here is not obvious, which is the
whole reason CI is unfiltered: `ingot → indexing-service` and
`delegator → forgectl` were both invisible in `go.mod` and only appeared under
`go list -deps` (rule 3). So:

- **Markdown only** — everything is skippable; nothing compiles or reads it.
  Say which files, so the claim is checkable.
- **One workflow's own YAML** — only that workflow's checks matter. Prove no
  other workflow, script or test reads it, with the `grep` in the block.
- **Go code** — run `GOWORK=off go list -deps ./...` per module and name the
  jobs the closure reaches. Never eyeball it.
- **Already green at an earlier head** — name the run and show
  `git diff <green-sha> HEAD -- <paths>` is empty.

**Say what it costs if it is wrong**, in the block: the checks still run, we
just do not wait, so an ignored check that goes red leaves `main` red. That is
a real trade and the reviewer is the one making it.

Keep the blocks. When the real filtering is designed, they are the worked
examples of what it has to be able to express — and the ones that turned out
wrong are worth more than the ones that did not.

The `guards` job runs the `check-*.sh` scripts in `.github/scripts/`. **That
directory is the list** — this file deliberately does not enumerate them,
because a hand-maintained copy of a derivable list is the thing that goes
stale (rule 3). Each script's header says what it enforces and why: read the
directory, not this paragraph.

It used to end with a summary of what they cover between them. By the time a
sixth guard arrived, that sentence named three of five — which is rule 3
happening to the very paragraph that states it. Gone rather than extended.

Image references in **Go** are deliberately unguarded — every pattern narrow
enough to avoid hundreds of false positives also misses real references, and
rule 5 says ship none rather than a partial one. Keep them in a module's
`testutil` package with a named const and an env override, not inline in a
`_test.go`.

## Run these with every `git subtree pull`

```
.github/scripts/finish-subtree-pull.sh <prefix>              merge, write, audit
.github/scripts/finish-subtree-pull.sh --dry-run <prefix>    print the diffs only

.github/scripts/resolve-rewrite-conflicts.sh                 take upstream + rewrite
.github/scripts/resolve-rewrite-conflicts.sh --dry-run       say what it would take
```

**Flags come before the prefix.** `finish-subtree-pull.sh <prefix> --dry-run`
used to ignore the flag and write anyway; it now refuses the trailing argument.

The first runs after **every** pull, conflicted or not, and detects which it is
rather than being told. The audit runs in both modes and speaks in both; what is
true is narrower than an earlier revision of this line claimed — a pull with no
conflicts at all is the case where the audit is *the only thing looking*, not
the only case where it has something to say. The second is conflicted-pull-only, and exits 0
saying so when there is no merge in progress, so running the pair
unconditionally is safe.

A subtree pull can lose an upstream change in two ways, and only one of them is
loud, so the first script does two things.

**The loud one.** For every `deleted by us, modified by them` entry it finds
where that file lives now and performs the three-way merge git would have
performed if its rename detection had seen outside the prefix. Everything it
needs is already in the index: stage 1 is the merge base, stage 3 is upstream,
both at the old path, and our side is the file at its new home — an ordinary
three-way merge, so `git merge-file` does it.

**It hands the result back in git's own terms.** A clean merge is written and
staged. A conflicted one gets `--diff3` markers in the file *and* index stages
1/2/3 at the new path, so `git status` shows `UU` there, mergetool works, and
`git add` resolves it. After that git owns the conflict; nothing downstream
needs to know the script ran.

**stdout is diffs and nothing else**, so `--dry-run` can be read, graded or
piped — and a dry run that would resolve everything exits **0**, which it did
not until the index was stopped from being read as evidence about a run that
stages nothing. Notes and problems go to stderr. **Exit 1 if anything was left
for a human or flagged by the audit, 2 if the script refused to act at all**.
Test for non-zero, not for 1. **The seven refusals are enumerated in the
script's own header, not here** — this file carried a second copy that went
stale over the path-space refusal, which was added without updating it, and was
born already missing the 200-merge one. Two copies of a derivable list is the
failure rule 3 names, and the copy that is not next to the code is the one that
rots.

It applies only where a rename was recorded *and* the destination verified to
exist. It refuses a same-basename guess, a true delete, a destination with
uncommitted changes, a destination that is a symlink (the write is a redirect,
and a redirect follows the link — the merged text landed in an unrelated
tracked file and the run said `merged`), and a binary (where `merge-file`
yields plausible garbage rather than failing). Those are reported and skipped,
never guessed at.

**It cannot audit a pull from an upstream that carries this repository's own
history and layout** — a hand-made fork-back, not anything `git subtree push`
produces, since that pushes rewritten commits in upstream's path space. In that
state the pull merge is indistinguishable from an ordinary `Merge pull request`
by ancestry and by path space alike, which is every signal available locally.
Four successive review rounds each proposed a heuristic and each was measured
wrong. What the script does is refuse where it can see the problem (the merge
base lands in our path space) and say what "never pulled" rests on where it
cannot. **That is the decision, not an open question:** no fork-back is expected
against these ten upstreams, so the case is left unhandled on purpose. Do not
add a fifth heuristic. Reopen it only if an upstream actually merges this
repository's history — and then the fix is a remote, not another local signal.

**Why react to the conflict rather than predict it:** you cannot predict it.
Measured (matrix on the wiki):

| our change | prefix after | next pull |
|---|---|---|
| renamed **within** the prefix | non-empty | **clean**, rename followed |
| moved **out** of the prefix | non-empty | **conflicts — even byte-identical** |
| moved out, prefix left empty | empty | clean (degenerate; do not rely on it) |

Things that are true and worth not rediscovering:

- **Splitting the move and the rewrite into separate commits does not help.** A
  three-way merge compares the merge base to each tip, not the commits between.
- **`git rerere` records nothing here** — it only stores conflicts with markers,
  and `modify/delete` has none.
- **A rename/delete is reported at upstream's NEW path**, which never existed in
  our history. `MERGE_HEAD` is in upstream's path space (no prefix), so its own
  diff names the rename; the script uses that to map back.

**The quiet one, and the reason the name changed.** When upstream *deletes*
a file we had moved out of the prefix, **both sides deleted that path, so git
raises no conflict at all** — no status entry, nothing to react to. The pull
succeeds in silence and we keep carrying a file upstream removed. So the audit
does not wait to be asked: it reads upstream's own diff against the previous
split point and reports deletions whose file we still have. It runs in both
modes, including after a pull with no conflicts whatsoever, which is precisely
when it is the only thing looking. `-M` is load-bearing there — without it a
rename upstream reads as a delete and every one is a false positive.

Exit is non-zero if anything was left for a human *or* anything was flagged by
the audit, so a caller can act on the status rather than parse prose. The audit
reports two kinds and counts them apart: a **carried** file, where our own
rename chain proves we moved it, and one **to confirm**, where nothing outside
the prefix is proven to be the same file but something shares its basename.
The second is a lead — between 16% and 81% of a prefix's basenames also exist
outside it — so it is never described as a move.

## The other half of the conflicts: `resolve-rewrite-conflicts.sh`

Everything above is about files we moved out of a prefix. The far more common
conflict is the one this repository creates by existing: every service's
imports were rewritten from `github.com/fil-forge/<svc>` to
`github.com/fil-forge/forge/<svc>`, so a pull conflicts on every file where
upstream touched that import block. **Fourteen of them in one resync**, each
the same non-decision.

`resolve-rewrite-conflicts.sh` takes upstream's file and re-applies the
rewrite — but only where that is provably safe. What it writes for a `.go` file
is `gofmt(rewrite(theirs))`, not `rewrite(theirs)`: it reformats, so deliberate
spacing and CRLF do not survive it. Measured exposure today is zero — `gofmt -l`
over the tree reports nothing and there is no CRLF in it — but that is a fact
about the tree, not about the script.

**The check is the point, not the fix.** It is safe only if our side carries
nothing upstream could disagree with, so the script verifies per file that
`rewrite(base)` is *exactly* ours, and refuses with a diff when it is not.
`go.mod` and `go.sum` fail it every time, which is the check working: those
carry real decisions (siblings at `v0.0.0` with `replace ../<svc>`, a unified
libforge) that have to be re-applied by hand.

`gofmt` normalises both sides for `.go` files, and that is load-bearing, not
tidiness: the rewrite inserts `forge/` into the path, which can move the line
**within** its import group, because `gofmt` sorts each group lexicographically.
Not because the path got longer, and `gofmt` never reorders the groups
themselves — measured: `fil-forge/piri/…` sorts *after* `fil-forge/libforge/…`
and `fil-forge/forge/piri/…` sorts *before* it, since `forge/` < `libforge/`.
Without normalising, a file whose only difference *is* the rewrite compares
unequal and gets refused.

**Neither tool sees the third class.** A hunk can merge *cleanly* and still
carry a polyrepo import path, because it never touched a line we had rewritten
— true of a file upstream added and of an existing file that merely gained an
import. Nothing conflicts, so nothing reports it. So after every pull, before
trusting a build, sweep the prefix for them:

```sh
svcs=$(for d in */; do [ -f "$d/go.mod" ] && printf '%s|' "${d%/}"; done)
grep -rnE "github\.com/fil-forge/(${svcs%|})([^A-Za-z0-9_-]|$)" <prefix>/
```

Derived from the top-level directories with a `go.mod` — the same set
`resolve-rewrite-conflicts.sh` builds — rather than typed out, so a service
added or renamed needs no edit here (rule 3). Not the same set as `go.work`,
which also lists `hilt/itest` and `ingot/itest`; those are caught anyway by
their parent's alternative. Add `| grep -v 'https\?://'` if you only want
module paths: most of the hits are repository URLs the monorepo deliberately
left pointing at their own repositories.

**`check-module-paths.sh` is that sweep, and `guards` runs it**, so the grep
above is for looking at a single prefix mid-pull; CI covers the tree. It reads
`*.go` and `go.mod` only — its header names what it does not read (`Makefile`,
`*.yaml`, `*.json`, `*.sh`, `go.sum`), because a success line that reads as
total over a partial check is what rule 5 is about.

**Do not rely on the build to find these, and do not rely on `go mod tidy`
either — it goes both ways.** Most of what the sweep finds is an import, and on
the resync that produced this branch swarf's two failed `go build` outright and
piri's ten stopped `go mod tidy`. But an earlier resync had the opposite: tidy
*resolved* a polyrepo import instead of refusing it, adding
`github.com/fil-forge/sprue` to sprue's own `go.mod` pinned to the commit being
merged — which builds green against code downloaded from the polyrepo. Which of
the two you get depends only on whether the old path still resolves through the
proxy, so neither outcome is a check.

And some of them compile either way: piri's otel meter name
(`Meter("github.com/fil-forge/piri/pkg/service/publisher")`) is a string, so no
build or tidy anywhere would have objected, and it would have shipped a metric
attributed to a module path that does not exist here.

**Measure drift by ancestry, never by grepping commit messages.** Only
`git subtree add` records a `git-subtree-split` trailer; a pull records nothing,
so a grep counts imports and misses every pull since. Measured that way once and
got 91 commits across nine prefixes when the answer was 37 across eight.
Ancestry needs no metadata and cannot be fooled:

```sh
git merge-base --is-ancestor "up-$p/main" HEAD   # up to date?
git rev-list --count "up-$p/main" --not HEAD     # how far behind
```

## Conventions

- **Never hand-transcribe a digest or a sha.** Resolve and apply it with one
  script reading its own output, and read shas rather than completing a prefix.
- **Verify both directions.** A guard that passes proves nothing until you have
  seen it fail on the defect it is for.
- Commit messages say *why*, and record what was tried and rejected. Several of
  this repo's subtleties are only written down in commit messages.
