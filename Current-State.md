# Current state

**Snapshot as of 2026-09-16 16:00Z.** Replace this page as things change; do not
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

   In: `piri`, `hilt`, `ingot`, `sprue`, `smelt`, `delegator`,
   `piri-signing-service` (all landed), `swarf` (landed), `indexing-service`
   and `forgectl` (pending).

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

**`main` is at `edf25236`.** Phase 0 complete and then some: 7 services
subtree-merged with history, module paths rewritten, `go.work`, per-module CI,
library pins unified across the nine original modules, every subtree resynced
to its upstream head, 18 images
pinned by digest, and the stack booting in CI from images built at HEAD.

Merged since the last snapshot: **#1** (stack from HEAD images), **#5** (the
six-subtree resync, into #4's branch), **#6** (image pins + `renovate.json`),
**#7** (`MONOREPO_TODO.md`) and **#4** (itest modules, the resync, HEAD peers,
`MAJOR_DECISIONS.md`). **#2** was closed unmerged — guppy is being
archived, so its tag cannot move under us in the window that mattered.

Two open, as a stack:

| PR | branch | what |
|---|---|---|
| [#4](https://github.com/fil-forge/forge-2/pull/4) | `claude/itest-modules` | each `itest/` its own module and actually run in CI; carries #5's resync; `itest` peers now built from HEAD; `MAJOR_DECISIONS.md` |
| [#3](https://github.com/fil-forge/forge-2/pull/3) | `claude/bring-in-swarf` | swarf as the 8th module, stacked on #4 |

#3 is **rebuilt** onto #4 rather than merging it (rule 7), and has been twice
now — each #4 push costs a #3 rebuild, which is the price of the stack.

**One interaction to watch.** #4 narrows hilt's Docker context to `hilt/`
because its only sibling dependency was test-only. #3 gives hilt a *linked*
dependency on swarf, widening it back to the repository root — hilt plus
swarf, and still not smelt. #4's own "2.1 MB" figure is true of #4 and does
not survive #3.

**Polyrepo MinIO repoints are all merged**: `smelt`,
[indexing-service#96](https://github.com/fil-forge/indexing-service/pull/96),
[piri#123](https://github.com/fil-forge/piri/pull/123),
[sprue#97](https://github.com/fil-forge/sprue/pull/97).

## Next

1. **Merge #4, then #3.** Everything else stacks behind them.
2. **Bring in `indexing-service`.** One question the plan left open is now
   answered: **its client is not test-only.** Derived rather than assumed —
   `GOWORK=off go list -deps ./cmd/...` in `ingot` reaches
   `indexing-service/pkg/{client,types,service/queryresult}`, and the imports
   live in `blockstore/forge.go` and `blockstore/locator/indexlocator.go`,
   with no `_test.go` importing it at all. So ingot's Docker build needs
   indexing-service **source**, not a `go.mod` stub. Cheaper than it sounds:
   ingot already builds with the repository root as context for its `hilt`
   edge, so this adds a `COPY indexing-service/` line rather than changing the
   context. ingot is its only consumer.
3. **`forgectl`** — in scope, a CLI, touches none of the image machinery.
4. **Phase 1** — release tags, `compat.yml`, publishing. `compat.yml` matters
   more than it did: moving `itest` to HEAD images removed the only thing that
   was accidentally testing compatibility against the deployed network.
5. **`libforge`'s dissolution** is what first exercises the audience rule
   (rule 1). Nothing currently in the repository is a pure library.

## Known debt

- **The unit suites run only under `-race`, not twice.** Upstream ran the
  suite plain and then again under the race detector; #9 runs it once, with
  `-race -shuffle=on`. Measured on this repository the second run is 3.2x the
  first (49s against 2m38s for sprue, 64% of the job), and what it uniquely
  covers is the uninstrumented binary, which `go build` and `go vet` already
  compile. The cost, which is real: piri's matrix sets `CGO_ENABLED=0` for
  its skiff build and `-race` requires cgo, so piri's tests now always run
  with cgo enabled, which its shipped binary does not.

- **Five per-module checks were lost with the per-service `.github/`
  directories, and nothing replaced them.** Every service called
  `ipdxco/unified-github-workflows`' `go-check` and `go-test`; seven had those
  callers deleted on `main` in `7321ee6a`, swarf's went in `52a5979b` on #3.
  Read against `Peeja/unified-github-workflows` at `d9b156f4` (a fork of
  `main` — the services pinned `@v1.0`, and the fork carries no tags, so the
  two could not be diffed), what is gone is: **`staticcheck ./...`**;
  **`gofmt -s`** (the `replaces` job runs plain `gofmt -l .`, so the
  simplifications are unchecked); **`go test -race ./...`** on ubuntu;
  the **macOS** run; and **`-shuffle=on`**.
  What is *not* lost, because it was gated off upstream too: the 32-bit and
  Windows runs (`skip32bit`, `skipOSes`), the `go generate` drift check
  (needs `gogenerate: true`, and no service sets it) and `golangci-lint`
  (needs a `.golangci.*`, which no service has). `go mod tidy` + go.sum diff
  and `go vet` are covered by the root `unit` job.
  **Codecov is not on this list, though an earlier version of this page had
  it.** The upload step is gated on `steps.secrets.outputs.CODECOV_TOKEN ==
  'true'`, computed as `if ($s[$k] // "") == "" then "false" else "true"`, so
  an absent, empty or missing-entirely secret skips it. No service carries a
  `codecov.yml` or mentions codecov in any `.md`/`.yml`/`.yaml`, and Petra's
  recollection (2026-09-16) is that it was not running on the polyrepo. The
  `-cover -coverprofile -coverpkg=./...` flags did run, but the profile went
  only to the skipped upload, so nothing consumed it. Adding Codecov would be
  new work, not restoration.
  **macOS is a TODO, not a restore** (Petra, 2026-09-16): the runners have no
  Docker daemon and several modules' tests need one, so whether those jobs
  were ever green upstream has to be established first — reproducing a job
  that was already red buys nothing.
  Belongs in `MONOREPO_TODO.md`; see **Next**.
- **hilt's image builds from the repository root, and that is meant to be
  temporary.** hilt links swarf through a sibling `replace`, and Go resolves
  replace targets before downloading, so the context must contain both.
  Narrowing it is not a Dockerfile change: an in-repo module reached by a
  `replace` always lives outside `hilt/`, so only consuming swarf as a
  published tagged module narrows it — Phase 1 work, which gives up
  same-commit co-development in exchange. Petra's call (2026-09-16): keep it,
  resolve before the consolidation finishes. Recorded in `hilt/Dockerfile`
  (`5177695b`); `MONOREPO_TODO.md` entry owed.

- `Dockerfile.release` (hilt, ingot, sprue) has the build-context problem the
  main Dockerfiles had, and nothing builds it. Surfaces at the first release.
- Base images float in our own Dockerfiles (`alpine:latest`,
  `debian:bookworm-slim`, `golang:1.27-bookworm`).
- `plc`, `storetheindex`, `filecoin-localdev` still float, but barely move —
  pin when convenient.
- Old `fil-forge/forge` still references the dead MinIO image. Superseded;
  left alone deliberately.
- **This repository has two guard scripts**, `check-replaces.sh` and
  `retry.sh`. The `check-image-lists.sh`, `check-setup-go-cache.sh` and
  `check-dockerfile-retry.sh` written during the first attempt live in the
  old `fil-forge/forge` and were never carried across. Worth porting the
  ones whose defect can recur here.
- **`itest ingot` now runs 29m03s and the job cap is 45 minutes.** Taking the
  peers from HEAD added a six-image build to the job — measured at 6m27s and
  8m12s on two runs — and the total went from ~21m30s to **29m03s**. The cap
  was 30. It was raised to 45 in the same change, on an estimate; the first
  full run came in with **57 seconds** to spare against the old number, so the
  raise was load-bearing rather than precautionary.
  The ordering still holds and still matters: Go's `-timeout 25m` covers the
  test only (~22m30s of that 29m) and fires first on a hang, dumping every
  goroutine. A runner kill at the job cap gives nothing, which is why the two
  numbers must not be levelled.
  Worth watching rather than acting on: the test portion is a little above the
  ~21m30s it used to take standalone. One observation, so not yet a fact.
- No **image-age check** anywhere. Every image failure so far would have been
  visible months earlier from "when was this tag last pushed".
- Per-service `CLAUDE.md`/`AGENTS.md` still describe polyrepo reality; 13
  stale module paths in docs. Best swept at the `forge-2` → `forge` rename.
