# Current state

**Snapshot as of 2026-09-17 15:15Z.** Replace this page as things change; do not
append to it. For history and reasoning, see [[Consolidation Findings]].

Consolidating the Fil Forge polyrepo into a monorepo at
[`fil-forge/forge-2`](https://github.com/fil-forge/forge-2), which becomes
`forge` at the end. The plan is `forge-consolidation-plan.md` (Phases 0–6).

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

**`main` is at `3c3fe769`, and the import phase is closed.** All **ten**
in-scope modules are subtree-merged with their histories, module paths
rewritten, `go.work`, per-module CI, library pins unified, every subtree
resynced to its upstream head, images pinned by digest, the checks the
per-service `.github/` directories took with them restored, and the stack
booting in CI from images built at HEAD.

`git ls-tree main` now lists: `delegator`, `forgectl`, `hilt`,
`indexing-service`, `ingot`, `piri`, `piri-signing-service`, `smelt`,
`sprue`, `swarf`.

Merged since the last snapshot: **#12** (forgectl — the tenth and last,
`7ccafeab`), **#15** (`ci.yml`'s `permissions` block and the path-filtering
TODO entry, `ff2f794d`) and **#16** (26 base images pinned by index digest,
`check-base-images.sh`, and the `replaces` → `guards` rename, `3c3fe769`). Before those, in order: #3 (swarf),
#9 (dropped checks), #8 (`MONOREPO_TODO.md`), #10 (indexing-service),
#11 (the indexer from HEAD) and #14 (`ci.yml`'s `concurrency` block).

Two open, stacked, both follow-on tidying rather than migration:

| PR | branch | what |
|---|---|---|
| [#17](https://github.com/fil-forge/forge-2/pull/17) | `claude/pin-stragglers` | the 5 image references that arrived with swarf and indexing-service *after* #6 had finished pinning + `check-stack-images.sh` |
| [#19](https://github.com/fil-forge/forge-2/pull/19) | `claude/test-image-pins` | the four inline test image pins move into `testutil`, in piri's shape (named const + doc + env override). Answers a review question on #17; stacked on it because it moves the same lines |

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

1. **Merge #17, then #19.** Stacked in that order; neither carries a
   `git subtree add`, so they rebase rather than needing a rule 7 rebuild.
   #16 changes a check name (`replaces` → `guards`), so a branch protection
   rule naming the old one needs updating with it.
2. **Phase 1** — release tags, `compat.yml`, publishing. This is the next
   real phase now that the imports are done. `compat.yml` matters more than
   it did: moving `itest` to HEAD images removed the only thing that was
   accidentally testing compatibility against the deployed network.
3. **`libforge`'s dissolution** is what first exercises the audience rule
   (rule 1). Nothing currently in the repository is a pure library.
4. **The `forge-2` → `forge` rename**, whenever this path is judged correct.
   Waiting on a person; see [[Needs Human Work]].

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

- `Dockerfile.release` (hilt, ingot, sprue) has the build-context problem the
  main Dockerfiles had, and nothing builds it. Surfaces at the first release.
- ~~Base images float in our own Dockerfiles.~~ **PR open:
  [#16](https://github.com/fil-forge/forge-2/pull/16)** pins all 26 external
  `FROM` references by *index* digest (a per-arch digest would silently break
  the `--platform=$BUILDPLATFORM` builds), and adds
  `check-base-images.sh` so the 27th cannot arrive unnoticed.
- ~~`plc`, `storetheindex`, `filecoin-localdev` still float.~~ Not true as
  written: a sweep for 2026-09-17 found **one** unpinned compose image
  (`postgres:16-alpine` in swarf's) and four in Go, all of which arrived with
  swarf (#3) and indexing-service (#10) *after* #6 had finished pinning.
  **PR open: [#17](https://github.com/fil-forge/forge-2/pull/17)**, which also
  adds `check-stack-images.sh` for compose.
  It deliberately adds **no Go guard**: a string shaped like an image
  reference matches 367 times here, almost all `s3:GetObject` IAM actions and
  `host:port` pairs, and narrowing to testcontainers call sites drops to 8 but
  then misses two of the real ones. A guard over part of a class reads exactly
  like a guard over the class (L12), so there is none rather than a partial
  one. That population remains unguarded, on purpose and in writing.
- Old `fil-forge/forge` still references the dead MinIO image. Superseded;
  left alone deliberately.
- **This repository has two guard scripts**, `check-replaces.sh` and
  `retry.sh`. The `check-image-lists.sh`, `check-setup-go-cache.sh` and
  `check-dockerfile-retry.sh` written during the first attempt live in the
  old `fil-forge/forge` and were never carried across. Worth porting the
  ones whose defect can recur here.
- **`itest ingot` is drifting toward its 45-minute cap, one image build at a
  time.** 21m30s standalone before the peers came from HEAD; 29m03s once six
  images were built in-job; and on #12's final head, with eight images and
  forgectl in the tree, **26m37s** — the fastest of the recent runs, against
  30m18s and 29m43s on its two predecessors. The trend is noisy enough that
  three points do not make a line; what is not noisy is that each service
  brought in-repo adds a build to this job. Each service brought
  in-repo adds a build to this job, so the margin shrinks as the monorepo
  grows rather than staying put. About fifteen minutes left.
  The cap was 30 and was raised to 45 on an estimate; the first full run after
  the peers moved to HEAD came in with **57 seconds** to spare against the old
  number, so the raise was load-bearing rather than precautionary.
  The ordering holds and must keep holding: Go's `-timeout 25m` covers the
  test portion only (~22m30s of the 29m) and fires first on a hang, dumping
  every goroutine. A runner kill at the job cap gives nothing, which is why
  the two numbers must not be levelled.

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
