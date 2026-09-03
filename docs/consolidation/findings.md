# Findings — consolidation POC, 2026-09-03

Written as the work happened, not at the end. Every claim cites what produced
it. "Planning" refers to the numbers and assumptions in `forge-poc-day-plan.md`
and `forge-consolidation-plan.md`; where a planning assumption did not survive
contact with the repos, that is recorded under **Surprises**.

Reference commits, all re-verified on disk at the start of the day: forge
`f60dd59` (2026-07-31), libforge `2585ed1` (2026-08-28), guppy `e87812b`,
ucantone `8d7eb73`, indexing-service `ba73105`.

## Surprises

### S1. The "deleted" libforge packages were never on libforge `main`

Planning said `sigv4/`, `s3perm/`, `client/hilt/`, `ucan/zapucan/` and
`commands/s3/**` were *deleted* from libforge between forge's pin (`928cf2a`)
and `main`. `git log --diff-filter=D` on libforge finds no deletion of any of
them. Instead:

- `928cf2a` is not an ancestor of libforge `main`. It is the single commit of
  [libforge PR #52](https://github.com/fil-forge/libforge/pull/52) (branch
  `feat/hilt-s3-client`, opened 2026-07-31, still **open and draft** on
  2026-09-03). forge `main` has pinned an unmerged draft PR in all five
  service modules for five weeks.
- Four of the five packages exist only on that branch. The fifth,
  `commands/s3/**`, *is* on `main` (libforge PR #44, July); PR #52 adds
  `Operation`, `OperationFor`, `ClassifyRequest`, `Operation.Permission`,
  `Operation.AddressesExistingBucket` and the request/bucket error contract to
  it, and forge's hilt and ingot use those symbols (5 files use `Operation`,
  4 `Permission`, 1 each `OperationFor`, `ClassifyRequest`,
  `AddressesExistingBucket`).
- The packages came *from* hilt: forge `main` at `96a672e` (2026-07-31, before
  "Refactor/drop replaces" #3) still has `hilt/pkg/sigv4`, `hilt/pkg/s3perm`,
  `hilt/pkg/lib/zapucan`, `hilt/pkg/client`, and ingot reached them through a
  `replace github.com/fil-forge/hilt => ../hilt`. PR #3 removed that replace by
  moving the code to libforge and pinning the PR commit.
- PR #52's state: one comment from alanshaw (2026-08-03) — "I don't
  understand why this is necessary?" — and a `CHANGES_REQUESTED` review from
  bajtos (2026-08-19) with four inline findings (two on authorization
  boundaries, one on presigned signatures moving between hosts, one on
  `Presign` with explicit default ports) plus two unformatted files. The code
  under review is the code forge's published hilt and ingot images run.

So the plan's question "did they move or were they deleted" has a third
answer — **they were never merged** — and Experiment A's "reconcile forge with
libforge main" has no mechanical form: forge cannot depend on libforge `main`
until PR #52 merges, or until the four packages get another home. Both paths
were measured (see `cost-report-libforge-bump.md` and `commands-move.md`).

### S2. forge is a five-week-old snapshot; the polyrepo kept moving

`fil-forge/forge` has had no push since 2026-08-01 (last `main` commit
`f60dd59`, 2026-07-31). The per-service repositories still exist and were
active throughout August — commits after 2026-07-31 (shallow clones since
2026-06-15, `git rev-list --count --since=2026-08-01`):

| repo | commits after 2026-07-31 | last push |
|---|---|---|
| `fil-forge/ingot` | 46 | 2026-09-03 |
| `fil-forge/piri` | 45 | 2026-09-01 |
| `fil-forge/sprue` | 24 | 2026-08-28 |
| `fil-forge/hilt` | 18 | 2026-08-31 |
| `fil-forge/delegator` | 15 | 2026-08-28 |
| `fil-forge/smelt` | 12 | 2026-08-28 |
| `fil-forge/piri-signing-service` | 4 | 2026-08-29 |

Consequences: (a) anything measured on forge measures a snapshot, and the
August divergence data the plan wants lives in the per-service repos, which is
where Experiment E reads it; (b) the live polyrepo answered the S3-client
question differently from forge — live hilt still ships `pkg/sigv4`,
`pkg/s3perm`, `pkg/lib/zapucan`, `pkg/client`, and live ingot imports
`github.com/fil-forge/hilt v0.0.1-0.20260828114936-cb1bc0b84e7b` directly as a
Go module (a service depending on a service). `ci.yml`'s premise "No service
module imports another (shared surface lives in libforge)" is true of the
snapshot and false of the live code.

Speculation, flagged as such: I do not know whether the freeze was a decision
or a lapse; the RFC being `draft` with no recommendation is consistent with
either.

### S3. `compat.yml` has been green every night while running nothing

Planning said `compat.yml` "has never run". Precisely: it has run 34 times
(nightly since 2026-08-01) and every run concluded `success`. In each, job
`Resolve supported window` succeeds and job `Version skew` is `skipped`
(`ready=false`, zero tags); GitHub reports a workflow whose only test job was
skipped as a green check. Example:
[run 33751635321](https://github.com/fil-forge/forge/actions/runs/33751635321)
(2026-09-03). This is the vacuous-pass hazard the plan warns about, one level
up from the test suite: the nightly cannot go red until the day it can go red
for real. See `compat-validation.md` for the fix on this branch.

### S4. The Docker build context forbids a sibling module

`docker/Dockerfile` states "BUILD CONTEXT IS THE SERVICE DIRECTORY" as a design
goal (hermetic per-service layer cache), and `ci.yml`, `publish-ghcr.yml` and
every service `Makefile` build with `context: <svc>`. A `replace
github.com/fil-forge/forge/commands => ../commands` in a service `go.mod` is
therefore invisible to the image build: the sibling directory is outside the
context. Moving any shared code into the repo as its own module forces the
build context to the repo root (with a `.dockerignore` to keep the cache
scoped), touching the Dockerfile, both workflows and four Makefiles. This was
not in the plan's blast-radius list. Measured in `commands-move.md`.

### S5. The `guppy embeds ingot` cycle is asserted, not substantiated

`ingot/DESIGN_NOTES.md` has no "§B" (its headings are: Two ways to run it,
Write path, The two planes, Shipping, Read path, Identity & auth, State &
durability, Known gaps). "Known gaps" says the carried copies exist "to stay
cycle-free (ingot must never import guppy/sprue — guppy embeds ingot)". In
guppy: `grep -rn ingot` over the tree and `git log --all -S ingot` both return
nothing; `guppy/cmd/gateway` imports boxo's gateway, not ingot. "Two ways to
run it" lists guppy as a *possible* host of the ingot library, which reads as
the anticipated cycle. Nothing in code prevents ingot importing
`guppy/pkg/client` today; the dependency-surface argument (importing
`pkg/client` from the single guppy module drags ~100 modules into ingot's
build list) is the real one and is measured by the inventory tool.

### S5b. `refactor/drop-replaces` (forge PR #3) is where the two-step rollout began

Planning trap 7 asked what that branch was doing. At forge `96a672e` (before
PR #3) ingot's `go.mod` had `replace github.com/fil-forge/forge/hilt =>
../hilt` and `replace github.com/fil-forge/forge/smelt => ../smelt`: the
monorepo's first week already contained in-repo module dependencies through
`replace`, exactly the mechanism Experiment C uses. PR #3 ("Refactor/drop
replaces", merged 2026-07-31) removed them by moving hilt's S3 client, wire
contract, `sigv4`, `s3perm` and `zapucan` into libforge (PR #52) and splitting
ingot's itest into a nested module, then pinned the unmerged PR commit
everywhere. In other words the repo chose the library route over the in-repo
module route, hit the two-step rollout on the same day, and has been stuck at
step one since. The per-service Docker build context ("BUILD CONTEXT IS THE
SERVICE DIRECTORY") was made possible by that choice and is undone by
reversing it (S4).

### S6. A `forge/internal` module is compiler-enforced across sibling modules

Verified with a throwaway module set: a module at
`github.com/fil-forge/forge/internal` is importable from
`github.com/fil-forge/forge/<svc>` (Go's `internal` rule is an import-path
prefix check in module mode) and refused for any other module path (`use of
internal package … not allowed`). So "monorepo `internal/`" in the plan's
classification table can be a real, shared, compiler-enforced module — not
just a convention.

### S7. Four first-party modules outside forge compile against `libforge/commands`

Found by the compiler, not by grep. Moving `commands/**` into the repo and
rewriting piri's imports produced three type errors at two seams:

- `github.com/fil-forge/piri-signing-service/pkg/types.SigningService.SignAddPieces`
  takes `[]libforge/commands/pdp/sign.PieceProofs`
  (`piri/pkg/pdp/service/roots_add.go`, `piri/pkg/service/signer/proofservicesigner.go`);
- `github.com/fil-forge/go-ipni-tools/pkg/advertisement.ShardCID` takes
  `libforge/commands/assert.LocationArguments`
  (`piri/pkg/service/publisher/publisher.go`).

A wider grep over the module cache (the versions piri and ingot pin) shows
`go-ipni-tools`, `piri-signing-service`, `indexing-service` and `delegator`
together reference nine `libforge/commands/*` packages in their code
(`assert` 12 references, `pdp/sign` 7, `claim` 7, `space/egress` 3, `content` 3,
`access` 3, `pdp` 2, `blob/replica` 2, `blob` 2). So the plan's open question
"does anyone compile against `commands/**`?" has a partial answer: **yes, at
least four first-party libraries do, in their exported APIs.** Whether any
*partner* does remains a human question. On this branch the two seams keep
using libforge's copy of the type (`commands-move.md`); a real move would have
to migrate those four modules to `forge/commands` — making it a published
module after all — or keep `libforge/commands` canonical.

### S8. `go work sync` broke the `GOWORK=off` build, and only CI noticed

After the Experiment A bump, `go work sync` raised requirements in each
module's `go.mod` to the workspace-wide union (hilt: `golang.org/x/net v0.57.0
→ v0.58.0`) and recorded the new hashes only in `go.work.sum`, which
`.gitignore` excludes. Every module built and tested locally (workspace mode).
The first CI dispatch of the branch
([run 33795942498](https://github.com/fil-forge/forge/actions/runs/33795942498))
failed all five `unit` jobs in 76 seconds with `missing go.sum entry for go.mod
file`. `GOWORK=off go mod tidy` per module added 213 `go.sum` lines and nothing
else; `go work sync` was then a no-op. This is the consolidation plan's trap 3
observed in the wild, and the argument for its proposed `go work sync && git
diff --exit-code` gate — with the addition that the gate must run `go mod tidy`
under `GOWORK=off` afterwards, or it will pass locally and fail in CI.

### S9. forge `main` plus today's floating `guppy:main-dev` is predicted to fail its own smoke test

From `guppy-status.md` (G12–G13, code reading, not executed): ucantone #49
(2026-08-17) made receipt decoding reject a receipt that carries `aud`, which
every pre-#49 executor sets. `smelt/systems/guppy/compose.yml` runs
`ghcr.io/fil-forge/guppy:main-dev` (floating; today guppy `d74fd06`, ucantone
`3a20cd5`, post-#49). forge `main` at `f60dd59` runs ucantone `ccb7705`
(pre-#49). A new guppy against an old sprue fails at the first `/blob/add`
receipt with `missing receipt for task`. Nothing has booted that combination
since 2026-07-31 (no push to forge, and the nightly boots nothing — S3), so the
stack job on `main` is green only because it has not run. The POC branch is
on post-#49 ucantone and is not affected. Concrete fix regardless of layout:
pin `GUPPY_IMAGE` to a `sha-*-dev` tag in `ci.yml` and `compat.yml`.

### S10. Auto-merged Dependabot commits get no CI and no images (guppy; likely libforge and ucantone too)

`guppy-status.md` G5: guppy's `dependabot-auto-merge.yml` merges with
`secrets.GITHUB_TOKEN`, and pushes made with that token do not trigger `push`
workflows, so 25 of the last 40 commits on guppy `main` — including HEAD
`e87812b` — never ran `Test`, `Check` or `Container` on `main`; the newest
published image is `d74fd06` (2026-08-21). libforge (`c2b2789`) and ucantone
(`db4f8c0`) adopted the same auto-merge workflow on 2026-07-31; whether their
`main` commits are affected the same way was not checked (their PR checks still
run before merge, and neither publishes images). Speculation flagged as such.

### S11. In the polyrepo, the two-step rollout is one person doing same-day fan-out

From `polyrepo-august-casestudy.md`: every wire-visible libforge or ucantone
change in August was followed by consumer bumps authored by the same person
within 0–4 days (two "sweeps", 08-20/21 across seven repos and 08-27/28 across
six), 14 pure dependency-bump PRs, zero by Dependabot, no Go source changes in
any pure bump. hilt merged its consumer PR 26 minutes *before* libforge PR #64
merged, pinning the PR-head pseudo-version `c9252ac` — the same shape of pin
that stranded forge on `928cf2a`, harmless there only because the squash merge
produced a byte-identical module tree (verified). The attestation fix took 17
days to reach sprue, its only production consumer, and a sprue bump three days
after the fix landed two commits short of it. smelt and delegator never bumped
in August. Mixed-version windows (ucantone #49: new decoders reject old
executors' receipts) existed from 08-17 to 08-21 across the fleet's pins;
whether any environment ran mixed is not recoverable from git.

### S12. The inventory contradicts the plan's classification for three packages

`package-inventory-notes.md`: `identity` is imported by every first-party
module that uses libforge (14 of 14 scanned, including delegator,
indexing-service, piri-signing-service and guppy), `digestutil` by guppy (14
files) and indexing-service (4), `bytemap` by guppy (4) and indexing-service
(3). A compiler-enforced monorepo `internal/` for those three, as the plan
proposes, would break four modules outside forge. `blobindex`'s heaviest
consumer is indexing-service (16 files), which the recorded RFC vote keeps out
of the monorepo. And `commands/**` is 87% generated code (27,122 LOC, 3,446
hand-written); `commands/debug` has no importer anywhere. Every LOC figure the
plan quoted reproduced exactly; the plan's closure figures counted direct
imports (`guppy/pkg/client`: 15 direct, **45** transitive modules).

## Costs measured

Filled in from Experiments A, C and E as they complete; see the linked
documents for the raw logs.

- **A — deferred library bump (5 weeks, 12 libforge + 15 ucantone commits):**
  under 4 min to a building tree, under 8 min to local vet/tidy/test green,
  zero source changes; see `cost-report-libforge-bump.md`. On GitHub Actions
  (run [33796693902](https://github.com/fil-forge/forge/actions/runs/33796693902),
  head `a7b6449`): all five `unit` jobs green within 5 min of dispatch; its
  `stack` job was cancelled by the next dispatch on the same branch (latent
  bug 8). Re-run on a pointer branch
  ([33797690917](https://github.com/fil-forge/forge/actions/runs/33797690917)):
  four `unit` jobs green, `unit sprue` failed in `sprue/internal/fx`
  `TestWireApp/aws` (an fx wiring test against AWS-backed stores, 17 s), which
  passed locally and passed on the Experiment C head that contains the same
  commits ([33797489822](https://github.com/fil-forge/forge/actions/runs/33797489822)),
  so it is treated as a flake, not a bump regression; `stack` was skipped as a
  consequence and re-dispatched. The first dispatch
  ([33795942498](https://github.com/fil-forge/forge/actions/runs/33795942498))
  failed in 76 s on the `go work sync` go.sum gap (S8).
- **C — `commands/**` (+ the four PR #52 packages) into the repo as modules:**
  53 s of scripted mechanics, about 25 min including the two type-identity
  seams and the Docker/CI/workspace consequences; 154 service files with
  rewritten imports, five `go.mod` +7 −1, eight files for the Docker context.
  On GitHub Actions ([run 33797489822](https://github.com/fil-forge/forge/actions/runs/33797489822),
  head `1647cc2`): every `unit` job including the new `commands` and
  `internal` ones, and the full `stack` job (root-context images, e2e, S3
  suite) **green**, 17 min 40 s end to end. See `commands-move.md`.
- **B — inventory:** 64 libforge packages, 70 guppy, 82 ucantone; tool runs
  in 7 s warm (1 min 23 s cold, network for `go list`); see
  `package-inventory.md` and the notes.
- **E — August divergence:** the case study (`polyrepo-august-casestudy.md`)
  counts 2 wire-visible libforge changes, 3 wire-visible ucantone changes and
  about 3 service-to-service contract changes in August, 22 human pin-bump
  commits across 9 repos, and lags of 0–4 days for coordinated changes versus
  17–18 days for an uncoordinated fix. The scripted measurement is
  `divergence-august-2026.md`.

## Latent bugs found

1. **Path-filter silent-green (planning trap 1, confirmed).** `ci.yml` and
   `publish-ghcr.yml` filter on `<svc>/**`, `docker/**` and the workflow file
   only. A change touching only a new top-level directory selects no service;
   `unit` and `stack` are skipped and the run is green. The instance that
   exists on `main` today is `.github/scripts/**` (consumed by the stack job,
   in no filter). Fixed on this branch in commit `d59179d`, which cherry-picks
   cleanly onto `main` `f60dd59` (verified with `cherry-pick -n` in a clean
   worktree), as does `d5e8040` (the `.gitignore` comment correction). Those
   two are "the one thing worth merging to `main`"; everything else on the
   branch is a demonstration.
2. **Vacuous-green nightly (S3).** `compat.yml`'s skipped test job reports
   success. 34 green nightlies, zero tests.
3. **`otherThan` naming seam (plan, confirmed in code).**
   `smelt/tests/compat/compat_test.go`: `otherThan` uses
   `{"piri","ingot","upload","hilt"}` (smelt service names) while
   `TestRollingUpgrade`'s baseline map uses `{"piri","ingot","sprue","hilt"}`
   (image names). Both correct today; fixed on this branch by deriving both
   from one table.
4. **`.gitignore` comment is wrong (planning trap 6, confirmed).** It says
   `replace` directives in each service `go.mod` are the source of truth; the
   only `replace` in the tree is piri's `google.golang.org/genproto`. The
   `require` lines are the source of truth.
5. **`ci.yml` premise drift.** "No service module imports another" is false in
   the live polyrepo (ingot → hilt) and was false in forge before PR #3.
6. **`go work sync` vs `GOWORK=off` (S8).** Sync leaves per-module `go.sum`
   incomplete; the workspace hides it. Fixed on this branch by a per-module
   tidy; needs a gate.
7. **`compat.yml` skips on `workflow_dispatch` too.** The dispatch input is
   only `window`; there is no way to run the suite without tags. Fixed on this
   branch (Experiment D).
8. **Two `workflow_dispatch` runs on one branch cancel each other.** `ci.yml`'s
   concurrency group keys dispatches by ref with `cancel-in-progress: true`, so
   dispatching the branch again to test a later commit cancelled the earlier
   run mid-stack (run 33796693902). Correct for PRs; for dispatch it means one
   measurement per branch at a time. Worked around with a pointer branch at the
   earlier commit (`claude/forge-monorepo-poc-p9w0yr-expA`, no new commits).

## Decisions needed from humans

From the plan, with what this POC adds:

1. **Do partners compile against `commands/**`, or only run images?** Still
   unanswerable from code. New evidence: the only cross-module Go consumers of
   libforge's wire packages found in this session are first-party (forge's
   services, the live services, guppy, indexing-service, delegator).
2. **`delegator` and `signing-service` are live services outside forge**
   (`fil-forge/delegator` 15 commits in August; `fil-forge/piri-signing-service`
   4). piri pins `delegator` as a Go dependency. Scoping question stands.
3. **guppy's status** — see `inv-guppy-status` results in the findings
   appendix once available.
4. **fil-one RFC 7** — checked `fil-one/RFC` (`59438b5`, 2026-09-01): it holds
   two documents (`rfcs/2026-05-filone-forge-deployment-proposal.md`,
   `rfcs/2026-07-review-time-and-stacked-prs.md`) and nothing numbered 7,
   nothing mentioning varsig or `0x300001`. The citation in
   `libforge/attestation/varsig.go` points at a document not in that repo.
   Still open where it lives (Notion is the likely place; not checked).
5. **Varsig `0x300001`** — unchanged; independent of layout.
6. **Who owns a red nightly** — sharpened by S3: today nobody can own it
   because it cannot go red.
7. **Suite depth** — unchanged; `assertUploadRetrieve` mirrors the smoke test.
8. **New: is forge meant to be the source of truth?** Five weeks of divergence
   between forge and the per-service repos (S2) means a real migration starts
   with a re-import, not with this snapshot. Someone has to decide which side
   is canonical before any of this is more than a measurement.
9. **New: libforge PR #52.** Merge it (after addressing the review), or move
   the four packages into forge (Experiment C shows how), or leave forge
   pinned to a draft PR. The third option is the status quo.

## What a real migration would need that this POC skipped

- Re-importing the services from their live repos (S2) — this branch moves
  code that is five weeks stale.
- Resolving PR #52's four review findings (authorization boundaries,
  presigned-URL host binding, default-port `Presign`) — the POC rebased the
  PR as-is; it only fixed formatting.
- The protocol gates the consolidation plan puts *before* dissolving
  libforge (apidiff on `commands/**`, greasing tests, codegen-freshness gate,
  test vectors). None were built today.
- A decision on `commands/ucan/attest` vs `attestation` (the only libforge
  package importing `commands`) — moving `commands` out of libforge leaves
  `attestation` importing a package that now has two homes.
- Rebuilding the Docker build contexts and caches (S4) properly, not just
  enough for the stack job to pass.
- Migrating the four first-party modules that compile against
  `libforge/commands` (S7) — `go-ipni-tools`, `piri-signing-service`,
  `indexing-service`, `delegator` — to whatever the canonical wire module
  becomes, and versioning that module for them. The branch leaves two seams
  in piri on libforge's types instead.
- Deciding the fate of `identity`, `digestutil`, `bytemap`, `blobindex`,
  `receipt` and `testutil`, which the inventory shows have consumers outside
  forge (S12); the branch copies three of them into `forge/internal` as the
  plan's control group, which is only reversible because libforge still
  carries the originals.
- Tag scheme, release workflow and compat wiring changes (consolidation plan
  Phases 1 and 6).

## Appendix — evidence-answerable questions from the plan

| # | question | answer | evidence |
|---|---|---|---|
| 5 | where did `sigv4`, `s3perm`, `client/hilt`, `ucan/zapucan`, `commands/s3` go? | never merged: four live only on libforge PR #52; `commands/s3` is on `main` but PR #52 adds the symbols forge uses; origin was hilt | S1 |
| 6 | is guppy functional against the current stack? | yes for the smoke path (login, space, 10 MB upload with one replica, retrieve), with a version-pair qualifier: today's floating `main-dev` image is post-ucantone-#49 and forge `main` is pre-#49 (S9). guppy is idle, not broken: no feature commit since July, main HEAD never CI-tested or published (S10). Retrieved bytes are never compared; most guppy commands are untested by any stack path | `guppy-status.md` |
| 7 | does `DESIGN_NOTES.md` §B substantiate the guppy→ingot cycle? | there is no §B; the cycle is asserted in "Known gaps" and contradicted by guppy's code and history | S5 |
| 8 | how divergent is `ingot/forgeclient` from `guppy/pkg/client`? | planning numbers all reproduced exactly. `tokenstore`: zero divergence (one header comment per file). `forgeclient`: four substantive items, all in `blobadd.go`/`indexadd.go` (per-call `WithProofStore`, explicit `Content-Length` on the PUT, dropped `/pdp/accept` requirement, parameter order + options); everything else is deletion of unused surface, the logger swap, renames. Upstream fixes were hand-applied 27–32 days late; guppy `pkg/client` has had no commits since 2026-06-19, and the *live* ingot has since grown the copy further (`BlobConclude`/`BlobAbort`/`BlobRemove`) — the flow has reversed | `forgeclient-divergence.md` |
| 1, 2, 3, 4 | see `cost-report-libforge-bump.md`, `divergence-august-2026.md`, `commands-move.md`, `compat-validation.md` | | |
