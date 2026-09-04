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

For `go-ipni-tools` specifically, "migrate the module" overstates the job.
Its `pkg/advertisement` (109 LOC, one function importing `commands/assert`,
one importing `digestutil`) is a Forge→IPNI adapter — translating Storacha
location claims into IPNI shard CIDs — and a leaf inside the library: nothing
else in `go-ipni-tools` imports it. Originally written down as having one
consumer (`piri`); the `indexing-service` recon (`build-readiness.md`) found
a second, independent one — `indexing-service/pkg/service/service.go` calls
both of its exported functions directly, against its own
`libforge/commands/assert` import. The other
eight packages (~3,300 LOC) have no Forge content once it is gone, and are
independently the plan's clearest "stays out" case: an externally-specified
protocol (IPNI), a deliberately narrow-interface library shape, an unbounded
third-party audience. So the unit that actually needs to migrate is one
package, not one module — the same correction S12 makes about `libforge` not
being monolithic, found here by applying it to a *consumer* instead of a
*subject*. `commands-move.md` §5 has the detail and the resulting package-deal
consequence: deleting `commands/**` from `libforge` (as opposed to this
branch's copy, which sits alongside the original and breaks nothing) takes
`go-ipni-tools/pkg/advertisement` with it in the same change, or `go-ipni-tools`
stops building for its one consumer. `package-inventory.md` now tracks
`go-ipni-tools` as a consumer (added mid-session): `commands`, `commands/assert`,
`digestutil`, `jobqueue`, `testutil` and ucantone's `did` package all show it
as an importer.

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

### S13. Experiments C and D contradicted each other the moment they met

Experiment C taught `smelt/pkg/workspace.Detect` to treat the in-repo
`commands` and `internal` modules like `libforge`: "shared dir in the
use-list, rebuild every service". Experiment D, built in a separate worktree
on `f60dd59`, added `TestServiceTable`, which asserts that the set of services
the workspace would build equals the compat table. Cherry-picked together, the
test failed immediately: the monorepo's `go.work` always lists `commands` and
`internal`, so `Detect` selected all eight services in its table, including
`indexer`, `delegator`, `guppy` and `signing-service`, whose module dirs do not
exist in forge, and `BuildBinary` would have failed before any compat stack
started. The same defect was latent at `f60dd59` for any `go.work` that
listed `libforge` (D's latent bug 5) — C merely made it unconditional. Fixed in
`8bcbddc`: `Detect` rebuilds exactly the services whose module is in the
use-list, which is the only set `go build` can build in workspace mode anyway,
pinned by `TestDetectSelectsOnlyWorkspaceModules`.

Two readings. First, D's daemon-free test did its job: it turned a would-be
first-real-run failure on GitHub into a local test failure. Second, this is
what "a change to a shared module can alter any service" looks like in
practice even inside one repository: the two experiments touched one file's
*semantics* from different directions, and only a test that encoded the
contract caught it. The monorepo's advantage here was that both changes and
the test lived in one tree and one CI run, so the contradiction had nowhere to
hide for five weeks.

### S14. Bumps are not "to the tip" a third of the time, and four pin series point off `main`

`divergence-august-2026.md` (generated by `tools/divergence`, reproducible
byte for byte): of the 105 consumer bump events since 2026-07-01, 66 land on
the library tip; outside forge it is 43 of 58. The rest are subtree imports,
feature and merge commits that carried a pin already six to seven days and up
to ten first-parent commits behind `main` (the 08-05/06 bumps that absorbed
the July blob-removal work), two pins of *unmerged PR heads* that were ahead of
`main`, and one regression (piri's ucantone pin moved backwards on 07-22 when
the Curio PDP branch merged). Four of the fifteen pin series contain SHAs not
on the library's `main`: forge → libforge `928cf2a` (PR #52, S1), piri →
libforge `3e5e6ba` (an earlier head of PR #49), sprue → ucantone `2662bdd`,
and hilt and ingot → libforge `c9252ac` (the PR #64 head, pinned by hilt 26
minutes before the PR merged) — `go-ipni-tools`'s pin is not among them,
having landed exactly on both libraries' tips the one time it was set
(2026-07-02) and never moved since (S7). On no day in August did the ten
non-forge consumers share a libforge pin or a ucantone pin (up to 7 distinct
libforge pins on one day; widest spread 57.1 days, set by `go-ipni-tools`'s
frozen pin against a caught-up live/hilt — 43.0 days without it). Eleven
straddle intervals of breaking or additive-required library changes were open
between peers that import the changed package; seven are still open today,
five of them held by smelt (in forge, pinned at libforge 07-27 / ucantone
07-06), the others by delegator, `go-ipni-tools` and piri-signing-service with
indexing-service, all outside forge. The RFC's "explicit, tested PR in the
consuming repo" is routinely short-circuited by pinning the library branch
before it merges, and four of July's twelve human library code commits were
pushed straight to `main`, two of them wire breaking.

### S15. The compat suite's first two runs caught a wire break the Go API hid

`compat.yml` runs [35](https://github.com/fil-forge/forge/actions/runs/33834794514)
and [36](https://github.com/fil-forge/forge/actions/runs/33834923448) on the
branch (HEAD vs the `sha-96a672e` and `sha-f60dd59` images the network ran;
full account in `compat-validation.md`): seven of eight real stacks failed,
every one on ucantone `bfc05d9` (#49) — a post-#49 component cannot read a
receipt issued by a pre-#49 executor, while old readers accept new receipts.
The break showed through four component pairs (HEAD sprue ↔ old piri,
today's guppy ↔ old sprue, July's guppy ↔ HEAD sprue, today's indexer ↔ old
piri); the two stacks that passed are the two in which every reader was at
least as new as its writer. Experiment A had bumped that library with zero
compile errors and green unit tests (the cost report flagged wire
compatibility as the open question); nothing short of booting the old images
against HEAD showed it. The provenance guard passed in all eight stacks and
fired in the negative test, so none of this was vacuous. Each job took under
nine minutes.

Two things the runs add beyond confirming the suite's purpose. First, the
suite is not hermetic: `guppy:main-dev`, `indexing-service:main`,
`delegator:main` and `piri-signing-service:main` float, the two runs differ
by one of them and disagree on two of four stacks — the polyrepo's floating
images sit inside the monorepo's own test harness (latent bug 14). Second,
the failure never names its cause: clients report `missing receipt for task`
against servers that logged a 200, because ucantone's container decoder
discards the `audience must be omitted` error (latent bug 15). Experiment E's
reading of the 08-20/21 straddle had the risk on the wrong side and is
corrected in `divergence-august-2026.md`.

### S16. Two decisions from the human round collide: archiving guppy breaks the compat suite's own driver

Decision 3 archives the guppy CLI. Decision 6/7 (own the nightly, deepen the
suite) assume the suite keeps working. Checked directly: `assertUploadRetrieve`
— the one assertion every compat test runs — goes through
`smelt/pkg/clients/guppy.ContainerClient`, which execs the CLI *inside the
running container* (`c.stack.Exec(ctx, "guppy", args...)`; `login`, `upload`,
`retrieve` subcommands). Not the client library forge is keeping — the binary
forge is archiving. Nothing about this was flagged anywhere in this POC before
this pass, because nothing here previously asked "what does the compat driver
actually depend on."

The plan already designed the fix (Phase 5, unbuilt): make the S3-path
assertion (`smelt/pkg/s3glue`, already proven by
`smelt/tests/s3/forge_eviction_test.go`'s force-a-real-retrieve pattern) the
primary driver, add a protocol-path driver using the new shared client (once
Phase 4's `forge/forgeclient` exists) for what S3 can't reach. The point of
this note is sequencing, not novelty: this has to land in the same change as
archiving guppy's CLI, or the suite this POC spent a day validating goes from
green to unable to boot a driver.

### S17. This branch's `commands`/`internal` split doesn't match the plan's per-package classification

Experiment C moved `commands/**` wholesale plus a three-package "control
group" into `internal/`, to demonstrate the mechanics cheaply. The plan has a
destination per package (`build-readiness.md` has the full table), and this
branch diverges from it in one concrete way worth fixing before a real
attempt: `commands/ucan/attest` is inside `forge/commands` today; the plan
puts it with `attestation` in a new, standalone extension module instead
(reasoning: it's Forge-independent once `commands.Unit` is redeclared
locally, and bundling it with `attestation` gives that extension a `go.mod`
requiring only `ucantone` and multiformats — compiler-enforced independence).
Six more libforge packages the plan places (`ucan`/`ucanlib` into `ucantone`
itself, renamed; `attestation`+`attestation/didmailto`; `identity`; `piece`;
`testutil`; `ucan/retrieval`; `blobindex`) are simply unmoved on this branch —
still imported from libforge directly. `receipt` is the one package the plan
itself leaves "open — see questions", not resolved by anything in this round.

Update: `ucan`/`ucanlib` now has a real demonstration, just not on this
branch — see `build-readiness.md`'s Phase 3 table and the
`claude/forge-monorepo-poc-p9w0yr` branch on
[`fil-forge/ucantone`](https://github.com/fil-forge/ucantone), which ports
exactly the two ucantone-destined files (`proof_chain.go`, `proof_store.go`,
176 LOC) and excludes `ucan/retrieval` (the plan's rationale already routes it
to the monorepo instead, not `ucantone` — it's an HTTP transport binding, not
protocol logic) and `ucan/zapucan` (undocumented in the plan's own table; see
S18 for why it can't follow `ucan` into `ucantone` either).

### S18. The plan's "534 LOC" for `ucan`→`ucantone` silently includes a package that isn't going there, and omits one entirely

Re-deriving the plan's Phase 3 LOC figures from `libforge` source directly
(not trusting the plan's table) turned up an arithmetic tell: `ucan` (root)
is 176 non-test LOC (`proof_chain.go` 123 + `proof_store.go` 53);
`ucan/retrieval` is 358 (`client.go` 88 + `transport.go` 135 + `server.go`
135). `176 + 358 = 534` — exactly the plan's stated LOC for the `ucan`→
`ucantone` row. But the plan gives `ucan/retrieval` its *own* row, with its
*own*, different destination (monorepo, not `ucantone` — it's a UCAN
transport binding consumed by `piri`/`ingot`, not proof-assembly logic). So
the 534 figure that labels the `ucantone`-bound row already double-counts
retrieval's LOC into a package that isn't going to `ucantone` at all. Checked
whether this is a staleness artifact (LOC counted before `retrieval`/`zapucan`
existed): no — `git ls-tree` at the plan's own reference commit (`928cf2a`)
shows both subpackages already present, so this is a table-arithmetic slip,
not a stale snapshot. Separately, `ucan/zapucan` (a `go.uber.org/zap`
structured-logging helper for `ucan.Invocation`, 54 LOC + 42 test) appears in
neither the `ucan` row nor its own row — it's simply absent from the plan's
Phase 3 table, at a commit where it already existed. It can't quietly ride
along with `ucan` into `ucantone` either: `ucantone`'s own `AGENTS.md` states
its dependencies are "third-party modules (multiformats, go-cid, cbor-gen,
dag-json-gen, secp256k1). Keep it that way" — `zap` isn't on that list, and
adding a structured-logging dependency to core UCAN primitives is exactly the
kind of scope creep that policy exists to block. `zapucan` most likely belongs
with the other small, generic, monorepo-`internal/`-bound packages (`bytemap`,
`digestutil`, `identity`, `piece`) — nothing about it is Forge-specific,
either, but nothing makes it `ucantone`'s problem — not decided here, flagging
it as unaddressed by the plan rather than guessing.

### S19. `libforge/jobqueue` and `libforge/testutil` have real external consumers the inventory tool doesn't surface, and neither has a clean home yet

Follow-up to S7/S12's "package, not module" methodology, applied this time to
what's left in `go-ipni-tools` once `pkg/advertisement` moves out
(`commands-move.md` §5 has the full derivation; summary here). Direct grep
across every locally-cloned repo — not `package-inventory.md`, which turns
out not to record full import edges for `-consumer`-mode modules at all (a
real tooling gap, detailed in `commands-move.md` §5) — finds `libforge/jobqueue`
has exactly two external consumers today: `go-ipni-tools/pkg/queue/poller.go`
and `indexing-service/pkg/construct/construct.go:25` (wiring a provider-caching
job queue). Both would lose access outright the moment `jobqueue` becomes a
forge-`internal/`-only package, per the plan's own Phase 3 table — an
unconditional compile break for whichever of the two stays outside the
monorepo, not a style question. `libforge/testutil` turns out to be almost
entirely a re-export of `ucantone/testutil` already (nine of its symbols are
verbatim aliases, `Must` duplicates `ucantone/testutil.Must` symbol for
symbol); its only genuine content is `Must2` and six named test identities
(`Alice`/`Bob`/`Carol`/`Mallory`/`Service`/`WebService`), which fit naturally
into `ucantone/testutil` — a package whose own `AGENTS.md` already says it's
meant for "tests here and in dependents." Recommendation (not yet executed
anywhere but demonstrated in part on the `ucantone` POC branch, see S17's
update above): fold `testutil`'s small delta into `ucantone/testutil` and
delete `libforge/testutil` outright rather than relocate it; for `jobqueue`,
vendor a private copy into `go-ipni-tools` if `indexing-service` joins the
monorepo (leaving `go-ipni-tools` as the only permanent external consumer), or
give it one small shared home outside forge's `internal/` if `indexing-service`
stays out (it has zero dependencies of its own, so this costs nothing
technically). Either way, "internal/, full stop" — the plan's current
Phase 3 answer — is incomplete for both packages as written.

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
  consequence. The re-dispatch
  ([33813866714](https://github.com/fil-forge/forge/actions/runs/33813866714))
  was **green end to end: 18 min 28 s** from dispatch to a green `stack`. The
  first dispatch
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
- **A + C + D + E combined, on the branch head:** `ci.yml` run
  [33835038235](https://github.com/fil-forge/forge/actions/runs/33835038235)
  (dispatched on `d889fc1`, 2026-09-04 03:57 UTC) **green end to end in
  10 min 50 s**: seven `unit` jobs (five services plus `commands` with its
  codegen gate and `internal`) and the `stack` job — four root-context image
  builds in 6–10 s each from the BuildKit cache, the e2e smoke suite in
  2 min 37 s, the S3 system suite in 5 min 19 s. The 18-minute figures above
  were cold caches; this is what a warm rerun costs.
- **D — `compat.yml` made runnable, then run twice:** run 35 (HEAD vs old
  images, today's guppy, guard enabled) 8 min 27 s for the job, 411 s for the
  suite, five stacks; run 36 (guppy pinned to its 2026-07-31 build) 7 min
  30 s, 358 s, four stacks. Time to a ready stack 40–120 s depending on how
  many HEAD services had to be built; the six or seven pinned images
  pre-pulled in 11–13 s. Against the D agent's 20–40 min estimate. Both runs
  red for real reasons (S15). See `compat-validation.md`.
- **B — inventory:** 64 libforge packages, 70 guppy, 82 ucantone; tool runs
  in 7 s warm (1 min 23 s cold, network for `go list`); see
  `package-inventory.md` and the notes.
- **E — August divergence:** the scripted measurement
  (`divergence-august-2026.md`, `tools/divergence`, 18 s per run, reproducible
  byte for byte) counts, for 2026-08: libforge 11 commits (7 dependabot, 2
  human commits touching non-test Go, 1 wire-visible: `2585ed1`); ucantone 13
  (3 dependabot, 8 human code, 6 wire-visible); the seven live service repos
  215 (108 dependabot); forge 0. Changes that needed a coordinated
  service-side move: 4 of the 10 human library code commits in August, 6 of
  12 in July. Today forge's five modules sit 27.8 days / 12 first-parent
  commits behind libforge `main` (by merge-base, since the pin is off `main`)
  and 31.8 days / 14 commits behind ucantone; live smelt is 31.6 / 13 and
  53.2 / 18; live piri, sprue, guppy and indexing-service 20.6 days / 2 behind
  libforge; `go-ipni-tools` — added as a tracked consumer mid-session, S7 —
  is furthest of all at 57.8 / 18 (libforge) and 59.2 / 19 (ucantone), on a
  pin it set once on 2026-07-02 and never touched again. Uptake of the one
  real breaking August change (`bfc05d9`,
  receipts) took 3.0–3.9 days for seven consumers in a single sweep, 10.2 for
  delegator, and never for smelt and forge; the July `b13386b` blob-release
  change took 7 days to reach piri and sprue and 21–22 to reach hilt, ingot
  and guppy. The qualitative companion (`polyrepo-august-casestudy.md`)
  reads the same history commit by commit. Numbers the two disagree on are
  labelled in each. See S14.

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
9. **`workspace.Detect` selected services that cannot be built (S13).** With
   `libforge` in `go.work` at `f60dd59`, and with `commands`/`internal` there
   after Experiment C, it selected all eight services in its table, four of
   which have no module in forge. Fixed in `8bcbddc`. The fix itself is
   layout-independent (rebuild exactly the services whose module is in the
   use-list, plus a test), but the commit does *not* cherry-pick onto `main`
   (`cherry-pick -n` on `f60dd59` conflicts in `workspace.go`, because the
   commit also removes Experiment C's `sharedDirs`); on `main` it is the same
   three-line simplification of `Detect` applied by hand.
10. **`compat.yml`'s `Resolve supported window` dies of SIGPIPE at about 300
    tags per service.** `git tag --list … | head -n "$N"` inside `$(…)` under
    `pipefail` exits 141 once git's output exceeds one stdio buffer; measured
    at 600 tags for `f60dd59`'s script and the POC's first version alike.
    Fixed in `d57234a` with `awk 'NR <= n'`. Not reachable today (zero tags),
    so a real defect that could only ever have appeared after the release
    process started working.
11. **The bind-mount path `compat.yml` depends on had never run in CI** until
    runs 35 and 36 on this branch. `ci.yml`'s stack job runs prebuilt images
    with `SMELT_STACK_PREBUILT=1`; e2e mounts binaries only when
    `SMELT_WORKSPACE` is set locally; the nightly boots nothing (S3). On the
    runner it worked first time: nine stacks built and mounted, provenance
    verified in all of them (`compat-validation.md`).
12. **`workspace.BuildBinary` hardcodes `GOARCH=amd64`** while
    `smelt/tests/s3` uses `runtime.GOARCH`; correct on GitHub's amd64 runners,
    wrong on an arm64 laptop (D's latent bug 10; not fixed).
13. **The compat suite inherits the smoke test's depth.** `TestPinnedPeer/ingot`
    checks only `/health`; `TestRollingUpgrade` boots each service at HEAD
    against a baseline rather than upgrading in place; `assertUploadRetrieve`
    never removes a blob, so the one `feat!` between the two available pins
    (libforge `b13386b`, `/blob/release` cause) is off the tested path (D's
    latent bugs 6–8; plan question 7).
14. **Four components of every compat and e2e stack float** (`guppy:main-dev`,
    `indexing-service:main`, `delegator:main`, `piri-signing-service:main`,
    the compose defaults in `smelt/systems/*/compose.yml`). A compat verdict
    therefore depends on the day's builds of three repositories outside
    forge; runs 35 and 36 disagree on two stacks for this reason alone. Pin
    them by `sha-*` tag or digest (only guppy has an input today).
15. **ucantone's container decoder hides why a token was not a receipt.**
    `container.decodeTokens` discards the `receipt.Decode` error, so every
    failure in S15 surfaced as `missing receipt for task` or `conclusion
    receipt not found` against a server that logged a 200. Cross-repo; worth
    an issue on ucantone whatever the layout decision.
16. **The suite's log timestamps are flush times.** `group-go-tests.awk`
    writes through a 4 KiB buffer (also in `ci.yml`); per-step durations
    cannot be read from the log. `fflush()` per line fixes it.

## Decisions needed from humans

From the plan, with what this POC adds. **All nine now have an answer** (a
human round on 2026-09-04); each item below keeps the original open question
for context and adds the resolution. Executing on several of them —
delegator/signing-service in particular — needs `build-readiness.md`'s recon,
not just the decision.

1. **Do partners compile against `commands/**`, or only run images?** Still
   unanswerable from code. New evidence: the only cross-module Go consumers of
   libforge's wire packages found in this session are first-party (forge's
   services, the live services, guppy, indexing-service, delegator).
   → **Resolved: no third-party code compiles against it.** Removes the need
   for `forge/commands` to ever be a stable, tagged, publishable module — the
   dilemma `commands-move.md` §1 posed. See S7.
2. **`delegator` and `signing-service` are live services outside forge**
   (`fil-forge/delegator` 15 commits in August; `fil-forge/piri-signing-service`
   4). piri pins `delegator` as a Go dependency. Scoping question stands.
   → **Resolved: both join the monorepo as new service modules.** This POC
   never investigated either repo's own structure; `build-readiness.md` has
   the recon.
3. **guppy's status** — answered in `guppy-status.md` and the appendix (Q6):
   functional for the smoke path with a version-pair qualifier; idle, not
   broken. The decision left is whether guppy is a product or a test client.
   → **Resolved: not a product.** The CLI is archived entirely; only
   `pkg/client` (+ `pkg/tokenstore`) survives, unpublished, for services in
   the monorepo. Surfaces a real, urgent dependency: the compat suite's own
   driver execs the guppy *CLI*, not the client library — `build-readiness.md`
   Phase 5.
4. **fil-one RFC 7** — checked `fil-one/RFC` (`59438b5`, 2026-09-01): it holds
   two documents (`rfcs/2026-05-filone-forge-deployment-proposal.md`,
   `rfcs/2026-07-review-time-and-stacked-prs.md`) and nothing numbered 7,
   nothing mentioning varsig or `0x300001`. The citation in
   `libforge/attestation/varsig.go` points at a document not in that repo.
   Still open where it lives (Notion is the likely place; not checked).
   → **Resolved: it exists, unmerged — `fil-one/RFC#7`.** Should have merged.
5. **Varsig `0x300001`** — unchanged; independent of layout.
   → **Not actually a live question** per the human round; nothing to resolve.
6. **Who owns a red nightly** — sharpened by S3: today nobody can own it
   because it cannot go red. After Experiment D it can: a run with no tags is
   a visible no-op, a run in which every test skipped fails, and a pinned
   service that turns out to run HEAD fails. And it would be red tonight:
   against the images the network runs, HEAD fails seven of eight stacks on
   the receipt rule (S15). Someone has to own that before `compat.yml` is
   pointed at real pins on `main`.
   → **Direction chosen, not built:** gate a release on the full skew suite
   via a release PR that can sit accumulating commits, merged on demand;
   `main` keeps only the fast HEAD-only check. Maps onto the plan's own
   unbuilt Phase 1 item 4 ("wire compat as a pre-release gate") —
   `build-readiness.md` has the mechanics and two open sub-decisions
   (per-service vs. fleet scope; fixed snapshot vs. living PR).
7. **Suite depth** — sharpened by D (latent bug 13): the suite proves
   provenance and the smoke path, and would not have caught the one `feat!`
   between the two image pins. Deepening it is independent of layout.
   → **Resolved: deferred as tracked follow-up work**, recorded above under
   "What a real migration would need."
8. **New: is forge meant to be the source of truth?** Five weeks of divergence
   between forge and the per-service repos (S2) means a real migration starts
   with a re-import, not with this snapshot. Someone has to decide which side
   is canonical before any of this is more than a measurement.
   → **Resolved: the polyrepo is canonical.** Advance the monorepo's modules
   to the polyrepo's current versions, or recreate the monorepo from them.
   No behavioral drift expected in forge's own history beyond infrastructure
   tweaks (confirmed: PR #3's replace-directive removal is the only thing
   found, matching that expectation) — but "advance to current" is bigger
   than a version bump once decision 2 (delegator/signing-service) and F3/S14
   (new module edges the live fleet grew that forge's `go.work` doesn't have
   at all: `swarf`, `smelt`-as-a-module, `ingot`→`hilt`) are counted.
9. **New: libforge PR #52.** Merge it (after addressing the review), or move
   the four packages into forge (Experiment C shows how), or leave forge
   pinned to a draft PR. The third option is the status quo.
   → **Resolved: obsolete as a libforge merge target.** Its packages land in
   forge instead (already prototyped by `internal/`/`commands/`); its four
   review findings are not obsoleted and still need addressing wherever the
   code lands.

## What a real migration would need that this POC skipped

- Re-importing the services from their live repos (S2) — this branch moves
  code that is five weeks stale.
- Resolving PR #52's four review findings (authorization boundaries,
  presigned-URL host binding, default-port `Presign`) — the POC rebased the
  PR as-is; it only fixed formatting.
- The protocol gates the consolidation plan puts *before* dissolving
  libforge (apidiff on `commands/**`, greasing tests, test vectors, writing
  the protocol down, a narrowed PR skew job, a published skew window). One
  exception, previously uncredited here: the codegen-freshness gate exists
  for `commands` (`ci.yml`'s `codegen gate` step). Full phase-by-phase status
  in `build-readiness.md`.
- `commands/ucan/attest` vs `attestation` is not an open decision — the plan
  already specifies both move together into a new, standalone extension
  module (S17). This branch has `commands/ucan/attest` inside `forge/commands`
  instead, the wrong split; fixing it is execution, not a question for a
  human.
- Rebuilding the Docker build contexts and caches (S4) properly, not just
  enough for the stack job to pass.
- Migrating the four first-party modules that compile against
  `libforge/commands` (S7) — `go-ipni-tools`, `piri-signing-service`,
  `indexing-service`, `delegator` — to whatever the canonical wire module
  becomes, and versioning that module for them. The branch leaves two seams
  in piri on libforge's types instead. Now sharper for three of the four:
  `go-ipni-tools` is scoped to moving `pkg/advertisement` (109 LOC), not the
  module (`commands-move.md` §5); `delegator` and `piri-signing-service` are
  joining the monorepo outright (decision 2), which removes their seams
  entirely rather than just relocating them — recon in `build-readiness.md`.
  `indexing-service` remains the one case with no scoping answer yet.
- Deciding the fate of `identity`, `digestutil`, `bytemap`, `blobindex`,
  `receipt`, `testutil`, `ucan`/`ucanlib` and `ucan/retrieval`, which the
  inventory shows have consumers outside forge (S12); the branch copies three
  of the first six into `forge/internal` as the plan's control group, which
  is only reversible because libforge still carries the originals. The
  plan's own per-package destinations for all of these are in `build-readiness.md`
  (Phase 3); `receipt` is the one the plan itself leaves open.
- Tag scheme, release workflow and compat wiring changes (consolidation plan
  Phases 1 and 6) — Phase 1's remaining items (cutting tags, wiring compat as
  a pre-release gate, the sliding-window tag-eviction bug, rollback-direction
  testing) are detailed in `build-readiness.md`; Phase 6 unexamined.
- Deepening the compat suite past its smoke path (decision 7, D's latent bug
  13): `assertUploadRetrieve` — the one assertion every compat test runs —
  logs in, uploads, and retrieves; it never calls `/blob/remove` or
  `/blob/release`. The two image pins the suite has to test against
  (`sha-96a672e`, `sha-f60dd59`) straddle exactly one wire-breaking libforge
  commit, `b13386b` ("feat!: add cause to blob release arguments"), which
  changes `/blob/release`'s arguments — the textbook case the suite exists to
  catch, on a path it doesn't exercise. Needs at least one assertion that
  removes an uploaded blob. Independent of the in/out decision.

## Appendix — evidence-answerable questions from the plan

| # | question | answer | evidence |
|---|---|---|---|
| 5 | where did `sigv4`, `s3perm`, `client/hilt`, `ucan/zapucan`, `commands/s3` go? | never merged: four live only on libforge PR #52; `commands/s3` is on `main` but PR #52 adds the symbols forge uses; origin was hilt | S1 |
| 6 | is guppy functional against the current stack? | yes for the smoke path (login, space, 10 MB upload with one replica, retrieve), with a version-pair qualifier: today's floating `main-dev` image is post-ucantone-#49 and forge `main` is pre-#49 (S9). guppy is idle, not broken: no feature commit since July, main HEAD never CI-tested or published (S10). Retrieved bytes are never compared; most guppy commands are untested by any stack path | `guppy-status.md` |
| 7 | does `DESIGN_NOTES.md` §B substantiate the guppy→ingot cycle? | there is no §B; the cycle is asserted in "Known gaps" and contradicted by guppy's code and history | S5 |
| 8 | how divergent is `ingot/forgeclient` from `guppy/pkg/client`? | planning numbers all reproduced exactly. `tokenstore`: zero divergence (one header comment per file). `forgeclient`: four substantive items, all in `blobadd.go`/`indexadd.go` (per-call `WithProofStore`, explicit `Content-Length` on the PUT, dropped `/pdp/accept` requirement, parameter order + options); everything else is deletion of unused surface, the logger swap, renames. Upstream fixes were hand-applied 27–32 days late; guppy `pkg/client` has had no commits since 2026-06-19, and the *live* ingot has since grown the copy further (`BlobConclude`/`BlobAbort`/`BlobRemove`) — the flow has reversed | `forgeclient-divergence.md` |
| 1, 2, 3, 4 | see `cost-report-libforge-bump.md`, `divergence-august-2026.md`, `commands-move.md`, `compat-validation.md` | | |
