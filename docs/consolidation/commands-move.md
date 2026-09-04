# Experiment C — `commands/**` into the monorepo as its own module

What breaks when the contested package group moves in, measured on this branch
(commits `bd2e556`..`c5cc15e`, on top of the Experiment A bump). "Can it
move" was never the question; "what does it drag with it" was.

## What moved, and where

| destination | contents | from | size |
|---|---|---|---|
| `commands/` (module `github.com/fil-forge/forge/commands`) | libforge `commands/**`, all 24 command packages and their `gen/` generators | libforge `f4b13f7` (main + PR #52) | 143 files, 27,122 non-test LOC of which 23,676 generated |
| `internal/` (module `github.com/fil-forge/forge/internal`) | `sigv4`, `s3perm`, `client/hilt`, `ucan/zapucan` (exist only on libforge PR #52) and the plan's control group `bytemap`, `digestutil`, `jobqueue` | same | 181 files, 1,545 non-test LOC |

Both are file-for-file copies with import paths rewritten; no other edit. The
copy and rewrite are scripted (`step1-commands-module.sh`,
`step2-internal-module.sh`, `step3-rewire-services.sh`, kept with the session
notes) and idempotent — the whole move re-ran from a clean tree in **53 s**.

`internal/` is a real module, not a directory: Go's `internal` rule is an
import-path prefix check in module mode, so any module under
`github.com/fil-forge/forge/` can import it and no other module can (verified;
`findings.md` S6). That is the compiler-enforced version of the plan's
"monorepo `internal/`" classification.

## The end state that matters

After the move, forge's five services pin **libforge `main` (`2585ed1`)** —
not the rebased PR #52 branch Experiment A had to use. Nothing in forge imports
a PR #52-only package any more. The move resolves the impasse described in
`findings.md` S1 without merging PR #52: the code hilt and ingot need lives
next to them, and alanshaw's "I don't understand why this is necessary?" on
that PR has a concrete answer — with an in-repo shared module, it isn't.

libforge packages forge still imports afterwards (files): `identity` 59,
`testutil` 48, `ucan` 27, `attestation/didmailto` 20, `piece` 9,
`ucan/retrieval` 6, `blobindex` 5, `receipt` 4, `attestation` 3 — and the two
seams below (`commands/pdp/sign` 2, `commands/assert` 1).

## What broke

### 1. Type identity across module boundaries — the compiler found external consumers

Rewriting piri's imports produced three errors:

```
pkg/pdp/service/roots_add.go:559: cannot use pieceProofs ([]forge/commands/pdp/sign.PieceProofs)
    as []libforge/commands/pdp/sign.PieceProofs in argument to p.signingService.SignAddPieces
pkg/service/signer/proofservicesigner.go:40: *proofServiceSigner does not implement
    piri-signing-service/pkg/types.SigningService (wrong type for method SignAddPieces)
pkg/service/publisher/publisher.go:89: cannot use lc (forge/commands/assert.LocationArguments)
    as libforge/commands/assert.LocationArguments in argument to advertisement.ShardCID
```

`github.com/fil-forge/piri-signing-service` and `github.com/fil-forge/go-ipni-tools`
declare libforge's wire types in their exported APIs. Two structs with
identical fields from different packages are different types; `PieceProofs`
would convert, `LocationArguments` (nested `commands.CborURL`, `*Range`) would
not. On this branch the two seams keep using libforge's copy (an aliased
import and a comment at each). A wider grep of the module cache shows four
first-party modules outside forge (`go-ipni-tools`, `piri-signing-service`,
`indexing-service`, `delegator`) referencing nine `libforge/commands/*`
packages. **This is the plan's open question 1 answered for first parties:
`commands/**` is a published Go API today.** A real move must either migrate
those modules to `forge/commands` (which then must be a stable, tagged,
publishable module — the very thing the plan wanted to avoid deciding) or keep
`libforge/commands` canonical and forge's copy a downstream fork. There is no
third option that compiles.

### 2. The Docker build context

`docker/Dockerfile`'s design goal was "BUILD CONTEXT IS THE SERVICE
DIRECTORY". `replace ../commands` cannot be seen from there. Changed: the
Dockerfile (root context, shared manifests copied first, build from
`/src/<svc>`), a root `.dockerignore` replacing four per-service ones, the
`image`/`image-dev` recipes in four Makefiles, and `context:` in both
workflows — 8 files. Layer caching is preserved (COPY hashes what it copies),
but every image build now uploads the whole repo minus the ignore list. **Not
in the plan's blast-radius list; not verifiable here (no Docker daemon);
the stack job on this branch is the first real build.**

### 3. The path-filter trap, as predicted

A change touching only `commands/**` matched no filter in `ci.yml` or
`publish-ghcr.yml`: `unit` and `stack` skipped, run green, nothing published.
Fixed by listing `commands/**` and `internal/**` in every service filter, and
giving both modules a `unit` job of their own (with libforge's
`codegen-build` + `gen-check` gate for `commands`). The pre-existing instance
of the same trap (`.github/scripts/**`, consumed by the stack job, in no
filter) is fixed in a separate, main-cherry-pickable commit `d59179d`.

### 4. Workspace detection

`smelt/pkg/workspace.Detect` rebuilt *all eight* services in its table
whenever `libforge` was in `go.work`'s use-list. The first version of this
experiment (`24ae12e`) treated `commands` and `internal` the same way, and
since the monorepo's `go.work` always lists them, that selected `indexer`,
`delegator`, `guppy` and `signing-service` too — modules that do not exist in
forge, so `BuildBinary` fails before any stack starts. Experiment D's
`TestServiceTable` caught it the moment the two branches met (measured: `the
workspace would build [delegator guppy hilt indexer ingot piri signing-service
upload] from HEAD; the compat table covers [hilt ingot piri upload]`). The
same failure was latent at `f60dd59` for any workspace that listed `libforge`
without every service module (`compat-validation.md`, latent bug 5).

The rule is now: rebuild exactly the services whose module is in the
use-list. Shared modules are compiled into those builds because the workspace
resolves them; they cannot widen the set, because `go build` refuses to build
a module that is not in the workspace. `TestDetectSelectsOnlyWorkspaceModules`
pins this with a `go.work` shaped like the monorepo's.

### 5. The `go-ipni-tools` seam is a package, not a whole module

Section 1's third compile error — `pkg/service/publisher/publisher.go:89:
cannot use lc (forge/commands/assert.LocationArguments) as
libforge/commands/assert.LocationArguments in argument to
advertisement.ShardCID` — is `github.com/fil-forge/go-ipni-tools`, the IPNI
advertisement library `piri` depends on. This branch resolved it the same way
as the `piri-signing-service` seam: keep libforge's type at the boundary.
That is the right call for *this* branch, but it understates what the seam
actually is, and a sibling conversation examining `go-ipni-tools` on its own
found the sharper version:

`go-ipni-tools` is nine packages (~3,300 LOC). Eight of them — `pkg/store`,
`pkg/publisher`, `pkg/metadata`, `pkg/notifier`, `pkg/server`, `pkg/client`,
`pkg/queue`, `internal/testutil` — are generic IPNI plumbing with no Forge
import (`pkg/queue`'s `jobqueue` and `internal/testutil`'s `libforge/testutil`
are borrowed infrastructure, not protocol coupling; both now show up as
`go-ipni-tools` importers in `package-inventory.md` after this session added
it as a tracked consumer). The ninth, `pkg/advertisement` (109 LOC), is not
generic: it imports `libforge/commands/assert` and `libforge/digestutil`, and
its two functions are a Forge→IPNI adapter —

```go
func EncodeContextID(space did.DID, digest mh.Multihash) ([]byte, error)
func ShardCID(provider peer.AddrInfo, caveats assert.LocationArguments) (*cid.Cid, error)
```

`ShardCID` converts a provider's multiaddrs to URLs, matches `{blob}`/
`{blobCID}` placeholders in them against each `location.URL()` in a Forge
location-claim's caveats, and recovers a shard CID. `space` and the location
claim are Forge/Storacha concepts end to end; nothing else in `go-ipni-tools`
imports `pkg/advertisement`, so it is a leaf — it can be lifted out without
touching the other eight packages. **Correction**: this was written up as
`piri/pkg/service/publisher/publisher.go` being its only consumer; the
`indexing-service` recon in `build-readiness.md` found a second, independent
one — `indexing-service/pkg/service/service.go:13,519,538` imports
`go-ipni-tools/pkg/advertisement` directly and calls both `EncodeContextID`
and `ShardCID` against its own `libforge/commands/assert.LocationArguments`.
The fix is unchanged (move the 109-LOC leaf package, not all of
`go-ipni-tools`), but it now has two independent forcing functions instead of
one, and if `indexing-service` joins the monorepo before this moves, it walks
into the exact same compile error piri already hit.

That makes this seam different in kind from `piri-signing-service`'s: there is
a smaller, correct fix than "keep libforge's copy at the boundary" forever.
Moving `pkg/advertisement` (not `go-ipni-tools`) into `forge/commands` (or
next to it) removes the seam rather than papering over it, and it removes the
only reason `libforge/commands/assert` has an external-ish Go consumer today
(`package-inventory.md`'s `commands/assert` row now lists `go-ipni-tools:1`
as an importer, confirming §1's "`commands/**` is a published Go API today"
with a name attached).

**The package-deal consequence.** This branch's copy of `commands/**` sits
*alongside* `libforge/commands`, which still exists, so nothing outside forge
is broken today. A real migration that deletes `commands/**` from `libforge`
— the end state Section "The end state that matters" describes — would break
`go-ipni-tools`'s build for everyone who depends on it, not just inside this
branch's `piri`, unless `pkg/advertisement` moves in the same change. Moving
`commands/**` and moving `go-ipni-tools/pkg/advertisement` are not independent
decisions; the second is a precondition of finishing the first, not an
optional follow-up.

**What doesn't move.** The other eight packages are, on their own reasoning,
the plan's clearest "stays out" case: `go-ipni-tools` implements IPNI, an
ecosystem standard with an unbounded third-party audience — a different
compatibility regime from `commands`, whose peers are Forge's own services.
Once `pkg/advertisement` is gone, it is a genuine IPNI toolkit with no Forge
content in it. `jobqueue`'s presence in both `go-ipni-tools` (via `pkg/queue`)
and forge/`internal` (Experiment C's control group, S12) means it needs a
neutral home reachable from both sides once `pkg/advertisement` moves in:
either `internal/` (unimportable by a module outside forge, so wrong once
`go-ipni-tools` is a consumer) or a small queue interface in `go-ipni-tools`
that forge's copy satisfies, or `jobqueue` gets its own tiny module.

**Methodological note.** This is the first case in the whole POC where
applying the in/out question at repo granularity gives a different answer
than applying it at package granularity — `go-ipni-tools` the repo says
"stays out" cleanly; `go-ipni-tools/pkg/advertisement` the package says "move
it, alongside `commands`." It sharpens the same point `package-inventory-notes.md`
made about `libforge` not being monolithic (S12): "true libraries stay out" is
a line to draw inside repos, not only between them.

**Follow-up: resolving `jobqueue` and `testutil`, the two dependencies left
once `pkg/advertisement` moves out.** The paragraph above left this as an open
menu ("either `internal/`... or a small queue interface... or `jobqueue` gets
its own tiny module"). Read both borrowed packages end to end
(`go-ipni-tools/pkg/queue/poller.go`, `internal/testutil/util.go`, and their
libforge originals) to close it out.

*`jobqueue` (265 LOC, `libforge/jobqueue/jobqueue.go`, one file).* Zero
non-stdlib imports (`context`, `errors`, `sync`, `time` only) and zero
Forge-specific content — a generic, type-parameterized async worker pool.
`go-ipni-tools/pkg/queue.QueuePoller` wraps exactly five of its symbols
(`JobQueue[T]`, `NewJobQueue`, `JobHandler`, `WithConcurrency`,
`WithErrorHandler`). The "also shows up in forge's `internal/`" fact from the
paragraph above undersells the actual conflict: `internal/` packages are
compiler-unimportable from outside the module they sit under (that's the
entire point of using `internal/` — see AGENTS.md's "Put the `internal/`
group under `internal/` specifically, so the compiler prevents the
classification being quietly re-litigated"), so **the moment `libforge/jobqueue`
is deleted in favor of a forge-internal copy, every external importer's build
breaks, unconditionally** — this was already true of the plan's own Phase 3
table (`jobqueue | 265 | monorepo internal/`), independent of whether
`go-ipni-tools` is in scope at all.

And `go-ipni-tools` is not the only one who'd break. A direct grep (not
`package-inventory.md` — see the tooling gap below) finds exactly two real
external consumers of `libforge/jobqueue` today:
`go-ipni-tools/pkg/queue/poller.go:10` and
`indexing-service/pkg/construct/construct.go:25` (wiring a
`providercacher.ProviderCachingJob` queue). Both are IPNI/content-routing
adjacent, which is probably not a coincidence. This matters for the fix:
- If `indexing-service` joins the monorepo (under discussion elsewhere in
  `build-readiness.md`), it stops being an external consumer, and
  `go-ipni-tools` is the only one left. At that point vendoring a private copy
  of the 265 lines into `go-ipni-tools` (no dependency, easy to own, cheap to
  keep in sync because it never needs to change) is the simplest correct fix.
- If `indexing-service` stays external instead, there are two permanent
  external consumers, and two independently-vendored copies of the same code
  is worse than giving `jobqueue` one small shared home outside forge's
  `internal/` — it has zero dependencies, so this costs nothing technically,
  it's purely a "whose repo hosts it" question.
Either way, "leave it as an ongoing dependency on a library that's otherwise
being dissolved" is not a real option — `jobqueue` needs to land somewhere
before `libforge` stops publishing it, not after.

*`testutil` (83 LOC, `libforge/testutil/`, three files).* Turns out to be
almost entirely redundant already. `fixtures.go` re-exports nine symbols
verbatim from `ucantone/testutil` (`RandomBytes`, `RandomCID`, `RandomDID`,
`RandomSigner`, `RandomIssuer`, `RandomMultikeySigner`, `RandomMultikeyIssuer`,
`RandomPrincipal`, and `RandomMultihash` — itself a deprecated alias for
`ucantone/testutil.RandomDigest`); `helpers.go`'s `Must` duplicates
`ucantone/testutil.Must` (`ucantone/testutil/helpers.go:47`) symbol for
symbol. `go-ipni-tools/internal/testutil/util.go` uses exactly three
symbols — `RandomMultihash`, `Must`, `Must2` — of which only `Must2` (a
15-line generic, no dependency of its own) and the six named fixed identities
in `fixtures.go` (`Alice`/`Bob`/`Carol`/`Mallory`/`Service`/`WebService`,
~30 lines) are actual net-new content over what `ucantone/testutil` already
has. `ucantone`'s own `AGENTS.md` already says its `testutil/` package is
meant for "tests here and in dependents" — it already intends to be exactly
the "deliberate conformance-fixtures package" the plan's Phase 3 table hedges
towards for `testutil` (`testutil | 83 | monorepo internal/, or a deliberate
conformance-fixtures package`). The cleanest resolution: fold `Must2` and the
six fixtures into `ucantone/testutil` (small, additive, matches its stated
intent), point every consumer (`go-ipni-tools`, `indexing-service`, and
`guppy/pkg/client` — the one piece of `guppy` the plan keeps alive — per the
grep below) at `ucantone/testutil` instead, and delete `libforge/testutil`
outright rather than relocate it. Demonstrated on a small scale in
`ucantone`'s own POC branch (`claude/forge-monorepo-poc-p9w0yr`,
[`fil-forge/ucantone`](https://github.com/fil-forge/ucantone)): the ported
`ucanlib` package's tests use `testutil.RandomIssuer(t)` in place of the fixed
`testutil.Alice`/`Bob`/`Carol`, since those tests only need distinct
principals — the fixture fold-in itself is not done there, to keep that
branch scoped to exactly the package it was asked to demonstrate.

**A tooling gap, found while checking the above.** `package-inventory.md`'s
JSON records full per-package `imports` lists only for modules walked as
"subjects" (`libforge`, `guppy`, `ucantone`, forge's own subtrees, `delegator`,
`piri-signing-service`, `indexing-service`); modules added with `-consumer`
(including `go-ipni-tools`) are recorded as scanned subjects (they appear with
a `Dir`/`Path`/`SHA` triple) but contribute **no package records at all** to
the `packages` list — so a query for "who imports `libforge/jobqueue`" against
that JSON silently returns nothing, even though `go-ipni-tools` and
`indexing-service` both really do (confirmed only by grepping the clones
directly). This isn't a correctness bug in anything this POC concluded from
the tool so far — every `-consumer` finding to date was checked against a
direct grep before being written down (S7, this section) — but it means the
JSON's importer edges should not be trusted as exhaustive for consumer-mode
modules without a direct-grep cross-check, and `tools/inventory`'s own
`-consumer` mode should either walk full package records for those modules
too, or say plainly in its output that it doesn't.

## The plan's checklist

| question | answer |
|---|---|
| Does `GOWORK=off` work for every module after the move? | Yes. All eight modules build, vet and pass unit tests under `GOWORK=off` (piri and smelt fail only their Docker-dependent packages, as at baseline). The `replace` directives in the binary modules are what make this work. |
| Does `go work sync` produce a clean diff? | After the move, yes — it is a no-op on the committed tree and per-module `GOWORK=off go mod tidy` is clean everywhere. After the Experiment A bump it was **not** (`go.work` go directive, `stress-tester` rewritten, and the go.sum gap that broke CI — `findings.md` S8). |
| How many `go.mod`/`go.sum` changed, how mechanical? | Five service `go.mod` (+7 −1 each: two `require`, two `replace`, libforge pin), five `go.sum` (+2 −2), `go.work` (+2), plus two new `go.mod`/`go.sum` pairs. Entirely mechanical — `go mod edit` + `go get` + `go mod tidy`. |
| Does the `stack` job still pass? | **Yes.** [Run 33797489822](https://github.com/fil-forge/forge/actions/runs/33797489822) on head `1647cc2`: `detect` + eight `unit` jobs (five services, `commands`, `internal`, all green; the `commands` codegen gate ran in 4 s) + `stack` (four images built from the root context, e2e smoke suite and S3 system suite green) — 17 min 40 s wall clock from dispatch, `stack` alone 13 min. The first real build of the root-context Dockerfile worked first time. |
| Control group (`bytemap`, `digestutil`, `jobqueue`)? | Trivial, as predicted: 22 files imported them (1 + 20 + 1), rewrite only, no seams. |

## Numbers

| item | count |
|---|---|
| Go files with rewritten imports in services | 154 (hilt 22, ingot 29, piri 41, smelt 2, sprue 60) |
| seams kept on libforge's types | 3 files, 2 external modules |
| files changed for the Docker context | 8 |
| workflow lines changed | +46 −9 |
| modules in the repo | 6 → 8 |
| `replace` directives | 1 → 11 (hilt 2, ingot 2, piri 3, smelt 2, sprue 2, internal 1, commands 0) |
| wall clock, mechanical steps (scripted, re-run) | 53 s |
| wall clock, first attempt to a building tree including the seam analysis | ≈ 8 min (19:21 → 19:29) |
| wall clock, whole experiment including Docker/CI/workspace consequences and commit messages | ≈ 25 min |

## Reading

Moving the package was cheap and mechanical. What was not mechanical was
everything the move exposed: the wire types have Go consumers outside the
repo, so the "internal protocol package" framing is already false for first
parties; the image build's per-service hermeticity was incompatible with any
in-repo shared module; and CI's path filters would have merged a wire-contract
change with no tests. None of those are arguments for or against the move —
they are the bill for it, and the first item is the one that decides whether
`forge/commands` is an internal directory or a library with a version. Section
5 sharpens that first item once more: the bill is not paid by moving
`commands/**` alone. `go-ipni-tools/pkg/advertisement` has to move with it, or
the move is incomplete for the one external consumer it has today.
