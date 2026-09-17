# Current state

**Snapshot as of 2026-09-17 20:35Z.** Replace this page as things change; do not
append to it. For history and reasoning, see [[Consolidation Findings]].

Consolidating the Fil Forge polyrepo into a monorepo at
[`fil-forge/forge`](https://github.com/fil-forge/forge). The plan is
`forge-consolidation-plan.md` (Phases 0–6).

## The move to `forge` is done

**`fil-forge/forge`'s `main` is `24b18ee5`** — the monorepo. Petra pushed it at
20:21Z and closed forge [#6](https://github.com/fil-forge/forge/pull/6),
[#7](https://github.com/fil-forge/forge/pull/7) and
[#8](https://github.com/fil-forge/forge/pull/8), the pre-fork chain whose base
history `main` no longer contains. `fil-forge/forge` has **no open PRs**. Her
description — the rename "was really pushing commits back to the original repo"
— matches what the remotes show: `forge-2` was never renamed, it was copied
from, and both repositories still exist.

**This wiki now lives on `forge`.** Its `wiki` branch is `d9c8f3b2`,
fast-forwarded there at 20:31Z; it had been one commit behind, missing this
section's predecessor. `forge-2`'s copy is frozen at the same commit. Work from
`forge`.

What follows:

- **Module paths `github.com/fil-forge/forge/*` are correct at the repo they
  name.** The interim wrongness the plan flagged — an external `go get`
  resolving at a repo that does not have the code — is over.
- **The monorepo's CI ran on `forge` for the first time, cold**, and the cold
  prediction held. Actions caches are per-repository, so nothing `forge-2`
  warmed carried over. `ci` **9m14s green** (run 36, on top of old-forge's own
  35); `images` **3m04s green**; `e2e`'s eight image builds **10m42s** against
  the 11m21s cold figure measured on `forge-2`. The per-image split is the
  useful part: **piri 5m52s**, then hilt 10s, ingot 7s, sprue 5s, swarf 6s —
  they share piri's layers — and delegator 1m21s, piri-signing-service 1m10s,
  indexing-service 1m51s, which do not. The `e2e` suite itself was still
  running at 20:35Z; `itest` likewise. Do not read a flake verdict into
  whatever they say — see the counting rule below.
- **`compat.yml` is scheduled on `forge`** and last ran green at 12:16Z against
  the *old* `f60dd596`. Its next scheduled run is the first against the
  monorepo tree, and nobody has seen that happen.
- **Two things exist only on `forge-2`:** the branch-deletion list (including
  `claude/shard-itest`, which must **not** be deleted) and
  [#28](https://github.com/fil-forge/forge-2/pull/28), the swarf firehose fix.
  #28's branch applies to `forge` unchanged — the two `main`s are the same
  commit — but where it lands is Petra's call, and the same patch is already
  open upstream as [`swarf` #17](https://github.com/fil-forge/swarf/pull/17).
- **⚠️ Do not rewrite this wiki's `forge-2` PR links to point at `forge`.**
  They name 11 distinct PRs (#2, #15–#17, #19–#22, #25, #27, #28) across 16
  places, and those numbers are `forge-2`'s. `forge` has its own #1–#8, so
  **#2 already collides**, and `forge`'s numbering will climb into the rest.
  The links resolve only while `fil-forge/forge-2` exists: **archive it rather
  than delete it**, unless the record 404ing is acceptable.

## Approach

Seven rules that have actually decided things:

1. **What goes in: things that ship as the Forge network.** The services and
   the tools that operate them — deployed together, versioned together, and
   not built by anyone outside. Three categories stay out:

   - **Outward-facing libraries**, which have consumers beyond Forge and so
     want real semantic versions and their own cadence: `ucantone`,
     `automobile`. When `libforge` dissolves, it splits on this line — the
     parts that were only ever private to Forge come in; the parts that are
     externally useful become properly versioned libraries outside.
   - **Forks of upstream software** we patch or repackage: `minio`,
     `storetheindex`, `did-method-plc`, `filecoin-localdev`, `versitygw`,
     `filecoin-services`. Folding these in would destroy what makes them
     useful — upstream history, provenance, and the ability to take upstream
     changes.
   - **Things being retired**, which are not worth moving: `guppy` is
     being dismantled and archived now, not at some later phase.

   In, and **all ten are now on `main`**: `piri`, `hilt`, `ingot`, `sprue`,
   `smelt`, `delegator`, `piri-signing-service`, `swarf`, `indexing-service`,
   `forgectl`. Nothing is left to import.

   This rule has a useful side effect: anything moving in stops being an
   external dependency, so it needs no image pin — see rule 2.

2. **Build what we own; pin what we don't — this is about container images.**
   In-repo services are built from HEAD in CI; external images get digest
   pins. A module moving in retires its pin by construction, so don't pin
   something that is about to arrive (rule 1).

   Go modules follow the same principle, but Go already enforces it:
   `replace => ../<svc>` for in-repo (always the matching commit), and
   `go.mod` + `go.sum` for external (a digest pin by another name). The rule
   needs stating for images precisely because the ergonomics are inverted —
   Go makes the correct thing the default and will not let you depend on a
   moving external version, while Docker makes `:latest` and `:main` the
   default and nothing complains. The instinct "dependencies are pinned,
   that's handled" is true in Go and silently false in Docker.

   Go's separate problem is *agreement*, not reproducibility: modules can be
   pinned to reproducibly-different versions of the same library, which is
   how a six-week ucantone wire skew survived a green CI. That is what
   unifying the library pins fixed.

   **Applied: pinning an image the tree already pins means reusing that
   digest, not resolving a fresh one.** #17 took `cf78e766…` for
   `postgres:16-alpine` from the four places that already had it and
   `2c4349a1…` for the minio release from piri's testutil, rather than asking
   the registry today. Resolving fresh would have put two builds of one tag in
   one repository — the *agreement* failure, not the reproducibility one.
   Both were checked against the registry and were still current. Approved
   2026-09-17.
3. **Derive dependency lists, never hand-maintain them.** `go list -deps` on
   the build target, not `go.mod`'s replace list. It is sometimes narrower
   and sometimes wider than the obvious guess, and it cannot go stale.
4. **Test what ships.** A bind-mounted binary in someone else's image is not
   the artifact that reaches production.
5. **A green check is a claim about what ran.** Ask what the job would have
   had to *do* to catch the fault.
   - Corollary, learned three times in one afternoon: when you fix one
     instance, run a command that enumerates the class before committing.
     Careful reading missed the rest every time; `gofmt -l` and
     `check-dockerfile-retry.sh` found them instantly. See
     [[Consolidation Findings]] L11.
   - And the same question applies to the guards themselves: a guard over
     *part* of a chain reads exactly like a guard over the chain, so it
     converts "unchecked" into "checked" for free. `check-image-lists.sh`
     shipped that way (L12).
6. **Stacked branches rebase onto their base; they do not merge it.** A merge
   buries the branch's own commits under someone else's and makes the PR diff
   grow every time the base moves. One exception, and it is load-bearing:
   `git subtree add` produces a merge commit that *carries* the imported
   history, and a plain `git rebase` silently flattens it. Rebuild those
   branches instead — base tip, fresh `git subtree add` at the same upstream
   commit, then cherry-pick — and check afterwards that the tree is unchanged
   and the upstream root is still an ancestor.

7. **Subtree history is never squashed.** No `git subtree add --squash`, no
   `git subtree pull --squash`, on any prefix, ever. Same principle as rule
   6's exception, different mechanism: a rebase flattens imported history
   after the fact, `--squash` declines to import it in the first place, and
   both end with a monorepo that cannot say where its code came from. The
   cost is visible and is meant to be paid — `main` carries 1010 commits and
   59 merges because seven services' full histories are in it, and each
   `git subtree pull` adds that service's new commits behind a merge. That is
   the feature.

   When the base moves under such a branch, it is **rebuilt**: replay it onto
   the new base, re-running each `git subtree pull` so the merge is recreated
   rather than flattened or imported as content. Not merged — merging the base
   in buries the branch's own commits exactly as rule 6 says. Not plainly
   rebased either, which is rule 6's stated exception.

   **`git rebase --rebase-merges` does not do this**, and fails in a way that
   can look like success. It recreates the merge *topology* but re-runs a
   plain recursive merge, with no idea that the second parent's paths need the
   subtree prefix. Tried on the itest branch, it reported
   `pkg/generate/keys_test.go added in ... inside a directory that was
   renamed in HEAD, suggesting it should perhaps be moved to
   smelt/pkg/generate/keys_test.go` — rename detection *guessing* its way to
   the right place, which on a different set of changes guesses wrong and
   says nothing. A single `-Xsubtree=<prefix>` cannot rescue it either: one
   branch carries pulls at six different prefixes.

   So the rebuild is manual, and its cost is the sweep. Re-running a pull
   reproduces upstream's side faithfully and therefore reproduces its blind
   spot: files upstream *added* merge cleanly and arrive carrying old module
   paths, `//go:build` tags this repo has retired, and — the one that nearly
   escaped — pre-existing resolutions taken from the original pull commits,
   which predate whatever has landed on the base since. Finish with a sweep,
   then check the tree against the head being replaced. On the itest branch
   that check was the whole point: the rebuilt tree had to equal
   `d64a71eb`, and four separate classes of loss had to be fixed before it
   did.

8. **A PR containing a `git subtree add` links the non-subtree commits for
   review.** Near the top of the body, one link to the Files Changed view per
   contiguous range of commits that are not the import — because the PR's own
   headline numbers describe the wrong thing. #3 reads as 65 files and +4730;
   the work in it is 55 files and +236/−447, and the rest is swarf's source
   arriving with its history, which is the point of subtree and not something
   anyone should read as a diff.

   The form is the PR's own files view over a commit range,
   `/pull/<n>/files/<sha>..<sha>`, so the link keeps the review context rather
   than dropping into a bare compare. Ranges are read off the first-parent
   history: `git log --first-parent --oneline --reverse <merge-base>..HEAD`
   shows the subtree merges as single commits, and everything between them is
   a range. Where the subtree add is the branch's first commit, as on #3,
   there is exactly one.

   It pins the head sha, so it goes stale on every push and is **refreshed as
   part of pushing**, not left to rot. Policy set by Petra, 2026-09-16.

9. **A branch meant to be reviewed and merged gets a PR, opened when it is
   pushed.** Not left as a bare branch for someone to notice. Scratch branches
   — probes, experiments, anything not meant to survive — do not need one, and
   should not get one. Policy set by Petra, 2026-09-16, after two branches
   (`claude/monorepo-todo`, `claude/upstream-findings`) were pushed without
   PRs and had to be opened by hand.

## Where it stands

**`main` is at `24b18ee5`, and the import phase is closed.** All **ten**
in-scope modules are subtree-merged with their histories, module paths
rewritten, `go.work`, per-module CI, library pins unified, every subtree
resynced to its upstream head, images pinned by digest, the checks the
per-service `.github/` directories took with them restored, and the stack
booting in CI from images built at HEAD.

`git ls-tree main` now lists: `delegator`, `forgectl`, `hilt`,
`indexing-service`, `ingot`, `piri`, `piri-signing-service`, `smelt`,
`sprue`, `swarf`.

Merged since the last snapshot, in order: **#20** (a root `AGENTS.md` +
`CLAUDE.md`), **#17** (the 5 straggler image pins + `check-stack-images.sh`),
**#22** (the skippable-checks practice in `AGENTS.md`), **#19** (test image pins
into `testutil`), **#23** (three `MONOREPO_TODO.md` entries) and **#24** (the
last two per-service `.github/` directories) — `main` is `144b3162`. Before
those: #12 (forgectl, the tenth and last module), #15, #16, and earlier #3
(swarf), #9, #8, #10 (indexing-service), #11 and #14.

**The repository now has a root `AGENTS.md`**, which is where the rules below
also live in one-line form, and which says of itself that it is scaffolding for
the construction rather than a guide to the finished monorepo.

One open. **#27 merged** (`24b18ee5`), so the postgres healthcheck fix and its guard are live:

| PR | branch | what |
|---|---|---|
| [#28](https://github.com/fil-forge/forge-2/pull/28) | `claude/swarf-firehose-scanner` `52648c29`, **22/22 green** | swarf's firehose client dropped oversized events and hung; fixed with the limits `cmd/swarf` already used |
| [`swarf` #17](https://github.com/fil-forge/swarf/pull/17) | `claude/firehose-scanner-buffer` `a50b142`, **5/5 green** | **upstream**, the identical patch — see *The deferred import findings* below |

**The layer cache is live on `main`** (#25) and its numbers are recorded (#26):
23s warm against a 7m34s baseline, with the caveats below. **#27** is the
follow-on that the cache work turned up — chasing #25's `e2e` red to its root
found a real bug, not a flake.

**Pre-rename work, run while Petra is away** (2026-09-17 16:40–17:25Z): #23, #24,
#25, #26 and #27. Deliberately *not* attempted, with reasons: the release workflow itself
(unverifiable until tags can be cut, and an unrunnable workflow is the
silent-green shape this repo keeps deleting), `compat.yml` (needs published
images to test against), the tag scheme (long-lived and hard to reverse — a
recommendation, not a decision an agent should take), and building the
`Dockerfile.release` files (they are goreleaser-shaped and need the release flow
first).

**[#21](https://github.com/fil-forge/forge-2/pull/21) (sharding `itest ingot`)
was closed unmerged**, 16:21Z, for simplicity — it was green and measured, but
it bought ~25% of wall clock for ~43% more runner-minutes, four `itest` jobs of
flake surface instead of two, and a check-name change to remember. The branch
`claude/shard-itest` survives at `e0205346`, so reviving it is a reopen, not a
rebuild. **What it measured is the lasting part, and it is below.**

**Every pull request now opens with a block naming the checks a reviewer can
merge without waiting for, and why** — Petra's idea (2026-09-17), and the
interim for the path-filtering question. It is manual on purpose: a `paths:`
list goes stale silently when a module gains a dependency, and a path-filtered
job reports *skipped*, which never satisfies a required status check. A block
rewritten per push has neither fault, and it can say things no path pattern
can — the one on #21 (now closed) said "`itest` already passed on `f4c5c21f`
and `git diff f4c5c21f HEAD -- .github/` is empty", which is a fact about two
shas, not a path.

Two conditions make it work rather than just feel good. **Derive it, do not
assert it** — the closure here is not obvious, which is the whole reason CI is
unfiltered, and `ingot → indexing-service` and `delegator → forgectl` were both
invisible in `go.mod`. And **state what it costs when wrong**: the checks still
run, so an ignored check that goes red leaves `main` red, and the reviewer is
the one taking that trade. The blocks are kept — when real filtering is
designed they are the worked examples of what it must express, and the ones
that turn out wrong are worth more than the ones that do not.
[#22](https://github.com/fil-forge/forge-2/pull/22) puts it in `AGENTS.md`.

**#12's merge needed a human**, and the reason is worth keeping: GitHub had it
registered as a *stacked* pull request from when its base was
`claude/indexer-from-head`, and that registration outlived both #11 merging and
the retarget to `main`. REST merge, auto-merge and changing the base all
refused. The web UI's own button uses the endpoint the error points at, so it
was one click — but no API route the agent has could do it.

**Polyrepo MinIO repoints are all merged**: `smelt`,
[indexing-service#96](https://github.com/fil-forge/indexing-service/pull/96),
[piri#123](https://github.com/fil-forge/piri/pull/123),
[sprue#97](https://github.com/fil-forge/sprue/pull/97).

## Next

1. **Merge #19, #22, #23, #24 and #25** — none carries a `git subtree add` and
   they touch disjoint files, so any order. Only **#25** can fail; the other
   four are documentation or deletions nothing reads. One check-name change is
   outstanding for branch protection: #16's `replaces` → `guards`, already on
   `main`. (#21 would have added a second; it is closed.)
2. **The `forge-2` → `forge` rename — and it is no longer "whenever".** Phase 1
   is gated on it, which the plan does not say. Module paths are already
   `github.com/fil-forge/forge/*`, and a submodule tag has to be
   `<svc>/vX.Y.Z` in the repository the path names. Tags cut in `forge-2` sit
   at a repository no module path resolves to — `go get
   github.com/fil-forge/forge/piri@piri/v1.2.3` looks in `fil-forge/forge`,
   the old one, and finds nothing. **So any tag cut before the rename has to
   be cut again after it.** Still a person's call; see [[Needs Human Work]].
3. **Phase 1** — release tags, `compat.yml`, publishing. Bigger than the plan
   assumed, because **the machinery it says to use does not exist here.** The
   plan reads "cut initial release tags via the *existing* `release.yml`
   flow"; `main` has four workflows — `ci`, `e2e`, `images`, `itest` — and
   none of them tags, releases or publishes. What survives from the polyrepo
   is raw material, and uneven:

   | | have it |
   |---|---|
   | `version.json` | 8 of 10 — not `forgectl`, not `smelt` |
   | `.goreleaser.yaml` | 4 — `indexing-service`, `ingot`, `piri`, `sprue` |
   | `Dockerfile.release` | 4 — `hilt`, `ingot`, `sprue`, `swarf`, and **nothing builds them** |

   `images.yml` also deliberately takes no `packages: write`, so fork pull
   requests work — publishing needs its own workflow or a job split rather
   than a flag on that one. `compat.yml` matters more than it did: moving
   `itest` to HEAD images removed the only thing that was accidentally testing
   compatibility against the deployed network.

   **The parts that do not need the rename** — writing the release workflow
   without cutting tags, `compat.yml`, deciding the tag scheme — can go first.
4. **`libforge`'s dissolution** is what first exercises the audience rule
   (rule 1). Nothing currently in the repository is a pure library.
5. **Two stray per-service `.github/` directories survive**, `forgectl/` and
   `indexing-service/`. The original seven were pruned on import; these two
   came in later (#12, #10) and nothing looked again. They are inert — GitHub
   reads only the repository root — but they describe a per-repo release flow
   that does not apply, which is exactly the kind of thing someone reads and
   believes while building Phase 1.

**Do not import anything else without a decision.** `MAJOR_DECISIONS.md`
records what is deliberately out — outward-facing libraries, forks of upstream
software, and things being retired — and every module it listed as in is now
in.

`MONOREPO_TODO.md` carries **seven** whole-repo questions: the s3-compat
report pipeline, Renovate, hilt's build context, the macOS run,
`stress-tester` coverage, turning `SA4006` back on, and whether CI should run
only what a change affects. None is blocking; none should be answered early.

## Known debt

- **The unit suites run only under `-race`, not twice.** Upstream ran the
  suite plain and then again under the race detector; #9 runs it once, with
  `-race -shuffle=on`. Measured on this repository the second run is 3.2x the
  first (49s against 2m38s for sprue, 64% of the job), and what it uniquely
  covers is the uninstrumented binary, which `go build` and `go vet` already
  compile. The cost, which is real: piri's matrix sets `CGO_ENABLED=0` for
  its skiff build and `-race` requires cgo, so piri's tests now always run
  with cgo enabled, which its shipped binary does not.

- **Four of the five dropped per-module checks are restored; one is a
  question, not debt.** Every service called
  `ipdxco/unified-github-workflows`' `go-check` and `go-test`; those callers
  were deleted with the per-service `.github/` directories (seven in
  `7321ee6a`, swarf's in `52a5979b`). #9 brought back **`staticcheck ./...`**,
  **`gofmt -s`**, **`go test -race`** and **`-shuffle=on`**. The **macOS run**
  was deliberately not restored and is a `MONOREPO_TODO.md` question: those
  runners have no Docker daemon and several modules' tests need one, so
  whether the jobs were ever green upstream has to be established first.
  Measured when restored: staticcheck **0 findings across 11 modules, 344
  packages**; `gofmt -s -l .` **0 files** — both checked with planted controls
  so eleven zeros could not be a broken analyzer.
  **Codecov was never running** and is not on the list, though an earlier
  version of this page had it. Its upload is gated on a non-empty
  `CODECOV_TOKEN`, no service carries a `codecov.yml`, and Petra's
  recollection (2026-09-16) agrees. The `-cover` flags ran but the profile
  went only to the skipped upload. Adding it would be new work, not
  restoration.
  Also not lost, because gated off upstream too: the 32-bit and Windows runs,
  the `go generate` drift check and `golangci-lint`.

- **hilt's image builds from the repository root, and that is meant to be
  temporary.** hilt links swarf through a sibling `replace`, and Go resolves
  replace targets before downloading, so the context must contain both.
  Narrowing it is not a Dockerfile change: an in-repo module reached by a
  `replace` always lives outside `hilt/`, so only consuming swarf as a
  published tagged module narrows it — Phase 1 work, which gives up
  same-commit co-development in exchange. Petra's call (2026-09-16): keep it,
  resolve before the consolidation finishes. Recorded in `hilt/Dockerfile`
  (`5177695b`) and in `MONOREPO_TODO.md` as of #8.

- **The polyrepo is NOT frozen, and the subtrees have drifted — ~45 commits
  across six services.** Measured 2026-09-17 by comparing each
  `git-subtree-split` recorded in `main`'s history against that repository's
  live `origin/main`:

  | service | behind | newest upstream commit |
  |---|---|---|
  | `ingot` | **25** | 2026-09-16 `chore: CI itest sharding for faster results (#166)` |
  | `hilt` | 6 | 2026-09-14 |
  | `smelt` | 4 | 2026-09-15 `fix: postgres and openbao boot issues (#44)` |
  | `sprue` | 4 | 2026-09-17 (today) |
  | `delegator` | 4 | 2026-09-11 |
  | `piri` | 2 | 2026-09-14 `Point minio at our own build (#123)` |
  | `swarf` | 0 | `main` dormant since 2026-08-27, but **two open drafts** |
  | `indexing-service`, `piri-signing-service`, `forgectl` | 0 | at the import point |

  **"Behind" counts merged commits, and for `swarf` that undercounts what is
  coming.** Its `main` really has not moved since 2026-08-27, but `pyropy` has
  two drafts open from 2026-09-15:
  [#15](https://github.com/fil-forge/swarf/pull/15) (`WithNonce` on `Publish`)
  and [#16](https://github.com/fil-forge/swarf/pull/16) (memory store stamps
  `RecordedAt` under the lock). **#15 edits `pkg/client/client.go` and
  `pkg/client/client_test.go` — the same two files our fix touches.** Checked
  with `git merge-tree` rather than assumed: both merge cleanly into
  `claude/firehose-scanner-buffer` today. #16 is also adjacent to one of the
  three findings left for you, the memory-vs-PostgreSQL divergence — worth
  reading before deciding that one.

  **This page previously said every subtree was resynced to its upstream head.**
  That was true when it was written and is not true now; `sprue`, `libforge` and
  `ucantone` all had commits *today*.

  **Petra's policy (2026-09-17):** no regular pulls, one final pull at the end
  — but **pull early where upstream fixes something we have hit.**

  Two rows are worth reading closely:

  - **`smelt` `96fc212` is the same bug #27 just fixed, found upstream first.**
    Its message says `pg_isready` goes green against the temporary server the
    image runs for `initdb` — the mechanism #27 derived independently from the
    `plc-postgres` log, two days later. **The fixes are complementary, not
    duplicate**: upstream makes the *consumer* wait (a `postgres-init.sh` that
    waits for real sessions) and also fixes a second boot race we have not hit,
    `ingot-openbao-init` writing before raft elects a leader; #27 fixes the
    *signal* itself, which helps every dependent. They do not conflict — #27
    touches line 150, upstream lines 163–183 of the same file. **This is the
    strongest candidate for an early pull.**
  - **`ingot` #166 is CI itest sharding**, i.e. the same work as closed #21.
    It arrives with the final pull whether or not #21 is ever revived.

  **The drift is bidirectional, and that is the part that bites.** Checked file
  by file rather than assumed: on the MinIO work we are **ahead**, not behind.
  Upstream `piri` #123, `sprue` #97 and `smelt` #43 all point at
  `ghcr.io/fil-forge/minio:RELEASE.2025-10-15T17-29-55Z` — **without a digest**.
  Ours carries the same target **plus** the digest pin, a named const, a
  `MINIO_IMAGE` override and, in sprue, a `/minio/health/cluster` wait strategy
  upstream does not have. smelt's "move the healthcheck off `mc`" fix is already
  in our tree verbatim. **Nothing on `main` is made redundant by upstream.**

  **A subtree pull will NOT silently regress the pins — an earlier version of
  this entry said it would, and that was wrong.** Tested with `git merge-file`
  on the real base/ours/theirs for each file rather than reasoned about:

      piri/pkg/internal/testutil/minio.go   1 conflict
      sprue/internal/testutil/s3.go         1 conflict
      smelt/systems/common/compose.yml      2 conflicts

  Every one conflicts loudly, and in each the **digest-bearing line survives the
  merge** — the conflict is confined to the `minio.Run(…)` call, while the
  `const defaultMinioImage = "…@sha256:…"` block sits in a region upstream did
  not touch and merges cleanly.

  **And a careless resolution is caught too.** Simulated taking *theirs* at the
  conflict: it compiles and vets clean, but `staticcheck` reports
  `U1000 const defaultMinioImage is unused` and `U1000 func minioImage is
  unused`, and `ci.yml` runs staticcheck on every module. The compose case is
  additionally covered by `check-stack-images.sh`.

  **That is a property of the shape #19 introduced, and worth knowing.** A named
  const plus a `minioImage()` helper means dropping the pin *orphans them*, so
  the indirection is self-guarding under merge in a way a bare inline literal
  would not be. The deliberately-unguarded Go image population is not exposed
  here after all.

  The one residual risk is narrow and compound: resolving to *theirs* **and**
  deleting the orphaned const and helper to silence staticcheck. That is a
  deliberate act, not an oversight.

- **The deferred import findings are now fixable, and that is a category worth
  knowing about.** `MONOREPO_TODO.md`'s *Findings in the imported code* holds
  problems noticed while bringing a service in and deliberately left alone,
  because changing behaviour inside a commit whose job is to move code makes a
  regression and a migration fault indistinguishable. **The imports are settled,
  so that reason has expired** — these are ordinary work now.

  Of swarf's four, exactly one was a bug rather than a decision, and
  [#28](https://github.com/fil-forge/forge-2/pull/28) fixes it: the firehose
  client built a default `bufio.Scanner` (64 KiB token cap) and discarded
  `ErrTooLong`, so an event with a long delegation `Path` was never yielded and
  `Stream` reconnected at the same cursor forever — **a hang, not a failure**.
  `hilt/pkg/fx/revocation.go` and `ingot/revocation/consumer.go` both link it in
  production code. The fix already existed in `cmd/swarf/stream.go`; the library
  every service consumes did neither half of it.

  **#28 belongs upstream as well, and the argument is stronger than tidiness.**
  Verified rather than assumed: the fix and its test contain **no module paths**,
  the patch `git apply`s cleanly to `fil-forge/swarf` at its head, and there it
  builds, vets and passes both subtests under `github.com/fil-forge/swarf/pkg/client`.
  Nothing about it is monorepo-specific.

  What settles it is **who actually runs the code**. Upstream `hilt` and `ingot`
  both require `github.com/fil-forge/swarf` in their `go.mod`, pinned at
  `v0.0.1-0.20260821142121-d5d1a0a56f00` (2026-08-21) — and the bug predates that
  pin: `_ = scanner.Err()` arrived in `7520dac` on **2026-08-18**. Meanwhile the
  monorepo ships nothing, because Phase 1's release machinery does not exist yet.
  **So #28 as it stands fixes a hang in the copy nobody deploys, and leaves it in
  the copy everyone deploys.**

  Landing the *identical* patch in both places is also the cheapest outcome for
  the final pull: when ours and theirs are byte-identical, the three-way merge
  takes it with no conflict at all.

  **Opened upstream 2026-09-17 as
  [`fil-forge/swarf` #17](https://github.com/fil-forge/swarf/pull/17)**
  (`claude/firehose-scanner-buffer`, `a50b142`): the same patch, applied with
  `git apply -p2`, unmodified, and re-verified in both directions against
  upstream's own tree — unfixed, each subtest hangs to its 30s deadline; fixed,
  both pass in ~0.05s. **Green upstream: 5/5** — Go Checks, Go Test on ubuntu
  (including `-race`, and the shared workflow shuffles test order), macos and
  windows, and Container. The scope caveat is spent. Worth keeping from the
  exercise: the clone arrived **shallow (depth 1)**, where `git log -S` and
  `git merge-base` answer confidently and wrongly (`7520dac` reported as *not*
  an ancestor of a commit it plainly precedes). `git fetch --unshallow` is what
  made the history below checkable at all.

  **A sharper reading of that history, now that it is readable.** The 64 KiB cap
  is not four weeks old — `bufio.NewScanner` has been in `pkg/client/client.go`
  since the initial commit `3e14143` (2026-07-16). What `7520dac` changed is the
  *consequence*. Before it, `Stream` made one connection and **reported** the scan
  error (`yield(…, fmt.Errorf("reading revocation stream: %w", err))`), so an
  oversized event failed loudly on a stream that then ended. `7520dac` added the
  reconnect-with-backoff loop **and** replaced that branch with
  `_ = scanner.Err()`; the two together are the hang. So the hang is dated
  exactly to 2026-08-18 — three days before the pin every consumer still carries.

  **The remaining caveat stands:** fixing upstream swarf does not fix its
  consumers — `hilt` and `ingot` each need a pin bump to pick it up, the same
  shape as the versitygw `lockWaitTime` item.

  **The other three want deciding, not fixing**, and are still open: the
  revocation lookup served `immutable` for a year on a mutable route; the memory
  and PostgreSQL stores disagreeing about what `Get` returns (a test passing
  against memory can be wrong about production); and `streamSettleWindow = 10s`
  assuming a bound on transaction duration that PostgreSQL does not give.
- **Image pins live inside subtree prefixes, and that is a standing cost.**
  #6 pinned images inside `piri/`, `hilt/` and `sprue/`; #17 added `swarf/` and
  `indexing-service/`. Each is local divergence that every future
  `git subtree pull` has to carry — the same concern that moved
  `staticcheck.conf` out to the root on #10. Accepted deliberately and approved
  2026-09-17: unlike a lint config, an image pin **has no root-level
  alternative**, because it has to live where the reference is. Worth
  remembering at the next resync rather than rediscovering as a conflict.
- **The guard scripts were the agent's own initiative, and are approved**
  (2026-09-17). Named rather than assumed welcome, because an earlier proposal
  — a squashed-subtree guard — was declined: `check-replaces.sh`'s second pass
  (`3b4c4d8e`, which has since caught the same fault twice, `hilt/itest/go.mod`
  on #3 and `ingot/itest/go.mod` on #10), `check-base-images.sh` (#16),
  `check-stack-images.sh` (#17), and `check-pg-healthchecks.sh` (#27, open).
  All revert cleanly.
- `Dockerfile.release` (hilt, ingot, sprue) has the build-context problem the
  main Dockerfiles had, and nothing builds it. Surfaces at the first release.
- ~~Base images float in our own Dockerfiles.~~ **Fixed, merged
  ([#16](https://github.com/fil-forge/forge-2/pull/16))**: all 26 external
  `FROM` references pinned by *index* digest (a per-arch digest would silently
  break the `--platform=$BUILDPLATFORM` builds), with `check-base-images.sh` so
  the 27th cannot arrive unnoticed.
- ~~`plc`, `storetheindex`, `filecoin-localdev` still float.~~ Not true as
  written: a sweep for 2026-09-17 found **one** unpinned compose image
  (`postgres:16-alpine` in swarf's) and four in Go, all of which arrived with
  swarf (#3) and indexing-service (#10) *after* #6 had finished pinning.
  **Fixed, merged ([#17](https://github.com/fil-forge/forge-2/pull/17))**,
  which also adds `check-stack-images.sh` for compose.
  [#19](https://github.com/fil-forge/forge-2/pull/19), still open, moves the
  four Go references out of test bodies into `testutil` packages.
  It deliberately adds **no Go guard**: a string shaped like an image
  reference matches 367 times here, almost all `s3:GetObject` IAM actions and
  `host:port` pairs, and narrowing to testcontainers call sites drops to 8 but
  then misses two of the real ones. A guard over part of a class reads exactly
  like a guard over the class (L12), so there is none rather than a partial
  one. That population remains unguarded, on purpose and in writing.
- Old `fil-forge/forge` still references the dead MinIO image. Superseded;
  left alone deliberately.
- **`.github/scripts/` holds `check-replaces.sh`, `check-base-images.sh`,
  `check-stack-images.sh` and `retry.sh`** — the last a helper, not a guard.
  The `check-image-lists.sh`, `check-setup-go-cache.sh` and
  `check-dockerfile-retry.sh` written during the first attempt live in the
  old `fil-forge/forge` and were never carried across. Worth porting the
  ones whose defect can recur here — though not `check-image-lists.sh` as
  written, which is lesson L12 itself.
- **The image build is cacheable after all, and the warm number is 23 seconds.**
  [#25](https://github.com/fil-forge/forge-2/pull/25) is **22/22 green** and
  measured. `itest.yml` and `e2e.yml` used plain `docker build`, which cannot use
  `--cache-from type=gha` at all, so the absence of a cache was never a decision;
  they now use `docker/build-push-action@v6` with `type=gha` per service, and
  `images.yml` stays cold as the canary on the same events.

      all 8 images       baseline (plain docker build)   7m34s  itest ingot / 6m08s itest hilt
                         cold + cache write (run 1)     11m21s  e2e  -- ~3 min WORSE
                         warm (run 2)                      23s  e2e
      whole e2e job      baseline (#24)                 13m10s
                         warm                            5m29s

  Per service warm: piri 4s, hilt 5s, ingot 3s, sprue 2s, delegator 2s,
  piri-signing-service 2s, swarf 3s, indexing-service 2s.

  **A third data point, from a real branch push** (#27 `a8a6040b`, `e2e`): all 8
  images in **2m27s** — seven of them 3–7s each, and `indexing-service` alone
  1m55s. That lands exactly in the "minutes rather than seconds" range this
  entry predicted, so the caveat below was calibrated. The one cold image is
  worth knowing why: the previous push's run was cancelled mid-build by
  `cancel-in-progress`, so that scope's cache export never finished. **A rapid
  re-push can leave a partially warmed cache** — not a fault, but it means a
  single job's timing is not a clean measurement of anything.

  **A fourth point, `main` after #27 merged** (`24b18ee5`): the whole `e2e` job
  in **6m18s**, against the 13m10s baseline. The first run after a merge was
  expected to be partly cold and was not, which is the cache behaving as
  intended across the PR → `main` scope boundary.

  **Two things keep this honest.** The warm run is a **re-run of the same
  commit**, so every layer hit — that is the ceiling, not the average. A real
  pull request changes Go source, which invalidates the `go build` layer for the
  services it touches; the base image, apt and `go mod download` layers (the bulk)
  still hit, so expect minutes rather than 23 seconds, and **worth re-measuring on
  a real source change**. And the cold path costs ~3 minutes more than before,
  because `mode=max` exports every layer — so the first run on `main` after
  merging is *slower*, by construction.

  **Not yet checked: cache size against GitHub's 10 GB per-repository limit.**
  Eight images exported at `mode=max` is not small, and eviction is LRU, so a
  cache that overflows quietly degrades back to cold builds. Worth a look before
  treating 23 seconds as permanent.

- **~~`TestUploadAndRetrieve/filesystem` is flaky~~ — root cause found, fix open
  on [#27](https://github.com/fil-forge/forge-2/pull/27).** It was 2 failures in
  40 `e2e` runs naming a *different* container each time (`upload-1` on run 160,
  `main` `ff2f794d`; `plc-1` on run 182), which read like a generic startup race.
  It is one bug, and `plc-postgres`'s own log against `plc`'s crash pins it:

      16:55:27.593  temp server: listening on Unix socket ONLY
      16:55:27.633  temp server: ready to accept connections   <- healthcheck GREEN
      16:55:29.441  temp server: shut down
      16:55:29.887  real server: listening on IPv4 0.0.0.0:5432
      16:55:30.531  plc exits: ECONNREFUSED 172.18.0.6:5432

  **The healthcheck was green 2.25 seconds before the port existed.**
  `pg_isready` without `-h` probes the **Unix socket**, and the official postgres
  image runs `initdb` against a temporary server started with
  `listen_addresses=''` — socket up, TCP refused by design — so the probe passes
  *during* initialisation. `plc` already declared
  `depends_on: {condition: service_healthy}`: the ordering was never wrong, the
  **readiness signal** was. That is also why the victim differs run to run.

  Six sites fixed with `-h 127.0.0.1` (five compose files plus
  `smelt/pkg/generate/compose.go`, whose gitignored output was checked to
  actually emit it), and `check-pg-healthchecks.sh` added so the seventh cannot
  arrive unnoticed — list-free, covers Go as well as YAML, and fails if it ever
  finds nothing to check. Verified in both directions: 6 FAILs before, 6 oks
  after.

  **Postgres was the only class member**, checked: every other healthcheck is
  HTTP over localhost or `redis-cli ping`, TCP by construction.

  **The guard failed CI on its own step name**, first push: `ci.yml`'s
  `- name: every pg_isready healthcheck probes TCP` contains the literal word,
  and the grep matched a *mention* rather than an *invocation*. Anchored to a
  preceding quote in `a8a6040b`, which keeps `.github/` in scope — an Actions
  `services:` block can carry `--health-cmd "pg_isready …"`, so excluding the
  directory would have bought a real blind spot to dodge a false positive.

  **The process lesson is the durable one**: the guard was verified in both
  directions, *then* wired into `ci.yml`, and not run again — so what was
  verified was a tree that no longer existed at push time. **Verify a guard
  against the tree you are actually pushing.** Rule 5 says check both
  directions; it now also has to say check the final state.

  **The diagnosis was not novel, and the record should say so.** Upstream smelt
  `96fc212` (2026-09-15, ash) names the same mechanism two days earlier —
  `pg_isready` going green against the temporary `initdb` server. #27 derived it
  independently from the `plc-postgres` log, which is why the account holds, but
  it was already known in the polyrepo and nobody here had looked.

  **#27 merged as `24b18ee5`** (19:02Z) and `e2e` has now passed **three times** on
  the fix — `a8a6040b`, `main` post-merge, and #28's branch. **That is still weak
  evidence.** At a ~5% base rate three consecutive passes had a ~86% chance of
  happening even unfixed, so it rules out very little; it would take dozens to
  say anything statistically. The mechanism is what justifies the fix. Record
  passes as they accumulate, and **treat a recurrence as informative rather than
  as noise** — that is the observation that would actually falsify this.

  **Still unverified: that the flake is gone.** A 5% failure rate cannot be shown
  fixed by one green run. The mechanism is proven; the frequency is not.


- **`itest ingot` is ~30 minutes, unsharded, and that is now a measured choice
  rather than an unexamined one.**
  [#21](https://github.com/fil-forge/forge-2/pull/21) built the sharding, ran
  it green, and was **closed unmerged** (2026-09-17 16:21Z) once the numbers
  were in. Branch `claude/shard-itest` survives at `e0205346`; reviving it is a
  reopen. The measurement is the part worth keeping, because every remaining
  option is judged against it.

  A genuine A/B — #19's unsharded run finished four minutes after #21's sharded
  one, same runner pool, same hour:

      itest workflow      30m43s unsharded  ->  23m07s sharded
      slowest job         29m29s            ->  17m07s
      test binary         1267.853s         ->  628.841 + 462.731 + 204.654s
      runner-minutes      29m29s            ->  42m04s

  **Total test work is unchanged (+2.2%)** — nothing got cheaper, it got spread.
  The trade on offer was **~43% more runner-minutes for ~25% less wall clock**,
  plus four `itest` jobs of flake surface instead of two and a check-name change
  to remember. Declined for simplicity, revivable.

  **Why sharding only bought 25%**, all three of which outlive the decision:

  1. **Round-robin by test *name order* is unbalanced — 629s / 463s / 205s** —
     and the critical path is the slowest, not the mean. Shard 1 drew
     `TestForgeVersity` (hundreds of subtests, dozens of 3-second object-lock
     waits) plus two TTL-bound tests; shard 3 drew four cheap ones. Perfect
     balance would be 432s, so this alone cost **3m17s**. Any revival should
     bin-pack against a previous run's timings — never a hand-written grouping,
     which is a list a new test falls out of silently.
  2. **Queue wait rose from 42s to 2m25s–4m47s** — four concurrent jobs where
     there were two. That is ~4 of the 10.5 minutes handed straight back, and
     it gets worse with shard count.
  3. **The ~6 minute image build is per job** and did not move — sharding
     triplicated it.

  **It also falsified the boot arithmetic this page carried twice.** Shard 3 ran
  four tests — four boots — in 204.654s, so a boot is at most ~51s, not the ~80s
  from 1257/13. The "1040s booting, 217s working" split was wrong, so **the
  shared stack is worth less than the 3–4 minutes last estimated, not more.**
  Parked for good: it changes test isolation in the job that exists to catch
  flakiness, for less than was ever claimed.

  **Two levers survive the decision, and neither needs sharding.** Both found by
  reading source after the measurement pointed at them, and **both currently
  live only on the closed branch and this page** — they belong in
  `MONOREPO_TODO.md` on `main`:

  - **`itest` and `e2e` have no Docker layer cache, and never decided not to.**
    `itest.yml:143` and `e2e.yml:118` shell out to plain `docker build`, which
    cannot use `--cache-from type=gha` at all; only `images.yml` uses buildx.
    The "No caching, deliberately (2026-09-11)" comment is *in `images.yml`* and
    reasons about that workflow — "the point is to provoke build failures". That
    does not obviously carry where the build is a means to running tests, and
    `images.yml` already proves the cold build on the same commit every PR.
    **Keep `images.yml` uncached as the canary, cache the other two**: up to ~6
    min off each, for a handful of lines. **This is the cheapest thing
    available and it is now the top of the list.**
  - **`lockWaitTime` is 3s in our own versitygw fork and is self-imposed.**
    `tests/integration/utils.go:2654`; `cleanupLockedObjects` sets
    `RetainUntilDate: now + lockWaitTime` then sleeps that long waiting for the
    lock it just created. **38 call sites ≈ 114s of pure sleep**, inside
    `TestForgeVersity`, the test that bounds the job. 3s → 1s saves ~76s; 1s is
    the floor until someone checks sub-second retention round-trips. Lands in
    versitygw — **not** in the agent's repository scope — and arrives here as a
    pin bump.

  Then **build-once-and-load** (same ~6 min from the other side, 1–2 GB of
  unmeasured artifact round-trip — try the cache first), and the
  **path-filtering question**, whose interim answer is the skippable-checks
  block on every PR (#22).
  The ordering still holds and still matters: Go's `-timeout 25m` fires first
  on a hang and dumps every goroutine, where a runner kill at the 45-minute cap
  gives nothing. The two numbers must not be levelled.

- **`SA4006` is off for the whole repository**, via a root `staticcheck.conf`
  reading `checks = ["inherit", "-SA4006"]` (#10). It suppresses one true
  positive in one generated file —
  `indexing-service/pkg/service/queryresult/json_gen.go:231`, a comma counter
  incremented after the last field, which nothing reads. The fix belongs
  upstream in `alanshaw/dag-json-gen`, pinned at `v0.0.9`, which is also its
  latest release.
  **Pinning staticcheck back is not available**, which is the part worth
  knowing: indexing-service was `go 1.25.7`, so the shared workflow's
  version table gave it 2025.1.1 and its own checks were green at the exact
  commit we imported. Unifying the libforge pin moved it to `go 1.27.0`, and
  both older versions that table can produce — 2025.1.1 (`v0.6.1`) and 2026.1
  (`v0.7.0`) — fail on *every* package with `export data version 4 is greater
  than maximum supported version 2`. Verified by installing both.
  It fires exactly once across thirteen modules today, so nothing is lost
  yet; that stops being true the longer it stays. `MONOREPO_TODO.md` carries
  the entry and the three ways out.

- **CI does not filter jobs by what a PR changed, on purpose — and that is
  not free.** No `paths:`, `paths-ignore:` or changed-files detection
  anywhere in `.github/`. `ci.yml`'s header says why: one job per module and
  no filter list means a new shared module cannot fall out of one and go
  silently green. The cost, measured: **#8 was one Markdown file** and its
  last push still cost `itest` 26m27s, `e2e` 9m57s, `ci` 8m40s and `images`
  7m00s.
  Worth a decision rather than a quiet fix, because the obvious mechanism —
  a hand-written `paths:` list — is the same silent-green shape this
  consolidation keeps deleting, and a job skipped by a path filter reports as
  *skipped*, which never satisfies a required status check. If it is done, it
  should derive the affected set from `go list -deps` (rule 3) rather than
  from a typed list. **Recorded as a `MONOREPO_TODO.md` question by
  [#15](https://github.com/fil-forge/forge-2/pull/15)**, so this line is now a
  pointer rather than the record.

- ~~**`ci.yml` is the only workflow with no `permissions:` block.**~~ **PR
  open: [#15](https://github.com/fil-forge/forge-2/pull/15).** Originally:
  `images.yml`, `e2e.yml` and `itest.yml` each set `permissions: contents:
  read`; `ci.yml` inherits whatever the repository default is. Noticed while
  writing #14 and deliberately left out of that diff — different concern from
  run cost, and it should be judged on its own. Two lines.

- No **image-age check** anywhere. Every image failure so far would have been
  visible months earlier from "when was this tag last pushed".
- Per-service `CLAUDE.md`/`AGENTS.md` still describe polyrepo reality; 13
  stale module paths in docs. Best swept at the `forge-2` → `forge` rename.
  The **root** `AGENTS.md` (#20) is accurate but says of itself that it is
  construction scaffolding, and names the conditions for replacing it —
  nothing left to import, no subtree pulls pending, the rename done, Phase 1
  real. Three of those four already hold.
