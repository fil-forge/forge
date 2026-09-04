# Build readiness — status against the original plan, before a real attempt

Written 2026-09-04, after the POC's six experiments and a round of human
decisions on the open questions in `findings.md`. Purpose: before attempting
to actually build the monorepo (not another reversible experiment — a real
branch meant to be looked at and, if it looks good, used to replace `main`),
check what the original plan (`forge-consolidation-plan.md`, kept outside
this repo) calls for against what has and hasn't been done, and what's new
since the POC started. Where a claim is unverified this session it says so.

## Decisions now settled (from the human answers)

| # | question | decision |
|---|---|---|
| 1 | Do partners compile against `commands/**`? | No third-party code does. Only first-party repos. |
| 2 | `delegator` and `piri-signing-service` scope | Both join the monorepo as new service modules. |
| 3 | `guppy`'s status | Not a product. The CLI is archived. Only `pkg/client` (+ `pkg/tokenstore`) survives, for internal use by services in the monorepo — not published. |
| 4 | fil-one RFC 7 | Real, unmerged: `fil-one/RFC#7`. Should have merged. |
| 5 | Varsig `0x300001` | Not a live question; independent of layout either way. |
| 6 | Who owns a red nightly | Direction: gate a release on the full skew suite via a release PR that can sit accumulating commits, merged on demand; `main` keeps only the fast HEAD-only check. Not built — see Phase 1 below. |
| 7 | Suite depth | Deferred as tracked follow-up work (`findings.md`, "What a real migration would need"). |
| 8 | Is forge the source of truth? | No — the polyrepo is canonical. Advance the monorepo's modules to the polyrepo's current versions, or recreate the monorepo from them. No expected behavioral drift in forge's own `main`, only infrastructure tweaks (confirmed below). |
| 9 | libforge PR #52 | Obsolete as a libforge merge target. Its four packages (`sigv4`, `s3perm`, `client/hilt`, `ucan/zapucan`) and its `commands/s3` additions land in the monorepo instead — already prototyped by this POC's `internal/` and `commands/`. Its four unresolved review findings (authorization boundaries, presigned-signature host binding, `Presign` default ports) are not obsoleted — they need addressing wherever the code lands. |

Decision 2 opens the entirely new scope this section is mostly about: this
POC's every experiment was built and validated against the five services that
were already in forge. Nothing here has looked at `delegator` or
`piri-signing-service` themselves until now.

## Phase-by-phase status (against the plan's Phase 0–6)

The plan's own phase list, verified against the actual document (I had been
working from a partial recollection until this pass; the full phase text is
`c9d68f6a-forgeconsolidationplan.md`).

### Phase 0 — Unblock: forge builds against current libforge

**Done.** Experiment A (`cost-report-libforge-bump.md`): forge builds, vets,
and passes tests against current libforge/ucantone; CI green end to end
([run 33813866714](https://github.com/fil-forge/forge/actions/runs/33813866714)).
The plan's own instruction was "do not start Phase 1 until this holds" —
it now does.

### Phase 1 — Make `compat.yml` produce signal

| item | status |
|---|---|
| 1. Cut initial release tags | **Not done, deliberately.** POC rule: no tags. Real work, not a decision — `piri v0.2.4`, `sprue v0.0.6`, `hilt v0.0.0`, `ingot v0.0.0` per the plan; current values not reverified this session. |
| 2. Let the nightly run, expect failures | **Done, on the POC branch.** Runs [35](https://github.com/fil-forge/forge/actions/runs/33834794514)/[36](https://github.com/fil-forge/forge/actions/runs/33834923448): 7 of 8 real stacks failed on a genuine wire break (ucantone `bfc05d9`, receipt `aud`), exactly the kind of signal Phase 1 wants (`compat-validation.md`). |
| 3. Provenance assertion (pinned service really runs the pinned image) | **Done.** Experiment D's `checkProvenance`/`TestCheckProvenance`/`TestProvenanceGuardFires`, verified firing correctly in both directions on real runs. |
| 4. Wire compat as a pre-release gate | **Not built — direction chosen today** (decision 6). `compat.yml` still triggers only on `schedule` and `workflow_dispatch` (verified: no `pull_request` trigger exists). The release-PR design discussed today is the plan for this item; no code exists yet. |
| 5. Fix the sliding-window weakness | **Not built, not previously flagged by this POC.** `window_for()` (`.github/workflows/compat.yml`) is `git tag --list … --sort=-v:refname \| awk -v n="$N" 'NR <= n'` — still just "the newest N tags," even after this POC's SIGPIPE fix (`d57234a`), which changed *how* the list is read, not *what* it selects. A release still silently evicts the oldest tested version from the next nightly's window; nothing asserts the resolved window covers a declared support policy (grepped for "support polic"/"declared coverage" in `compat.yml`: no hits). |
| 6. Add the rollback direction | **Not built.** `TestRollingUpgrade` (`smelt/tests/compat/compat_test.go:278`) only goes old-fleet→HEAD-service; nothing tests HEAD-fleet→old-service (the revert case). Distinct from D's latent bug 8 ("does not upgrade in place"), which is about the same test's realism, not this specific missing direction. |

### Phase 2 — Protocol gates (the plan says land these *before* Phase 3)

| item | status |
|---|---|
| 1. `apidiff`/`gorelease` PR gate | **Not built.** (A grep for "gorelease" in `release.yml` is `goreleaser`, the binary-release tool — unrelated; false lead, checked and ruled out.) |
| 2. Greasing tests (unknown field ignored, unknown command → `HandlerNotFound`) | **Not built.** No test asserting either behavior found in `commands/` or `smelt/tests/`. |
| 3. Codegen freshness gate | **Partially built, previously uncredited.** `ci.yml`'s `codegen gate` step (`if: matrix.service == 'commands'`, `run: make codegen-build gen-check`) is exactly this, for the `commands` module. Not extended to any other module, and not by design for that reason — `commands` is the only one with a `gen/` step in this repo today. |
| 4. Protocol test vectors | **Not built.** (`ingot/fee/cose/vectors_test.go` is COSE fee vectors, unrelated; false lead, checked and ruled out.) |
| 5. Write the protocol down | **Not built.** |
| 6. Narrowed PR skew job on `commands/**` paths | **Not built.** No such job or path filter exists in `compat.yml` or `ci.yml`. |
| 7. Published skew window | **Not built.** |

The plan's own rationale for this phase's ordering: land it before Phase 3
(dissolving libforge) or there's a window where atomic cross-service protocol
changes are frictionless and nothing catches a break against a deployed peer.
A real build attempt that includes Phase 3's moves inherits that window as-is
unless this phase is addressed first or in the same branch.

### Phase 3 — Dissolve `libforge`

The plan has a **per-package** destination table, more precise than what this
POC actually built. Checked against the current POC branch:

| package | plan's destination | this branch |
|---|---|---|
| `commands/**` minus `commands/ucan/attest` | monorepo, protocol | **Wrong split**: `commands/ucan/attest` is inside `forge/commands` (`commands/ucan/attest/{proof,types}.go` present) — the plan wants it out, with `attestation` |
| `blobindex` | monorepo, protocol | not moved; still imported from libforge (5 files, per `package-inventory.md`) |
| `ucan` (package `ucanlib`, collides with `ucantone/ucan`) | rename into `ucantone` | not moved on **this** branch (still imported from libforge directly — `hilt/integration/network.go`, `hilt/pkg/api/tenants_test.go`, `hilt/pkg/api/service/tenant/service_test.go`, `hilt/pkg/fx/upload.go`, `hilt/pkg/store/delegation/memory/store.go`, `hilt/pkg/client/{upload.go,upload_test.go}`, `ingot/uploader/{forge.go,forge_test.go}`, verified by grep) — but now demonstrated for real on a sibling branch: `claude/forge-monorepo-poc-p9w0yr` on [`fil-forge/ucantone`](https://github.com/fil-forge/ucantone) (commit `b0f9cf6`) adds `ucanlib/` with exactly `proof_chain.go` + `proof_store.go` (176 LOC, the plan's own two ucantone-destined files — see "The go-ipni-tools seam, resolved" below for why the plan's 534 LOC figure for this row is overstated and what's excluded). Builds, vets, and tests clean standalone (`GOWORK=off go test ./...`); no other file in `ucantone` touched. This branch's own `commands/**`/`internal/` copy of `ucanlib` still needs the same move for forge's own importers (`hilt`, `ingot` above) to switch to it — not done here, this only proves the destination side. |
| `attestation` + `attestation/didmailto` | new standalone extension module | not moved; still imported from libforge directly (same file list above overlaps) |
| `commands/ucan/attest` | goes with `attestation`, not the protocol (its `ProofOK = commands.Unit` needs a locally-declared `Unit`) | inside `forge/commands` today — see above |
| `jobqueue`, `identity`, `piece`, `testutil`, `bytemap`, `digestutil` | monorepo `internal/` | **partially done**: `bytemap`, `digestutil`, `jobqueue` are in `forge/internal` (the POC's "control group"); `identity`, `piece`, `testutil` are not moved, still imported from libforge (`identity` 59 files, `testutil` 48, `piece` 9 per `package-inventory.md`) |
| `receipt` | **the plan itself marks this "open — see questions"** | not moved; 4 files still import it from libforge |
| `ucan/retrieval` | monorepo, non-standard UCAN transport | not moved; 6 files still import it from libforge |

None of this is a regression — the POC's `commands/`+`internal/` move was
scoped to demonstrate the mechanics (Docker context, path filters, workspace
detection, type-identity seams) cheaply, not to execute Phase 3's full
classification. But a real build attempt needs the *plan's* split, not the
POC's, and specifically needs `commands/ucan/attest` moved back out of
`commands/` and into whatever `attestation`'s new home is.

### Phase 4 — Consolidate the client, archive `guppy`

**Direction confirmed today (decision 3); zero of the extraction built.**
`ingot/forgeclient/` and `ingot/tokenstore/` — the forked, diverged copies
`forgeclient-divergence.md` measured — are still present and untouched (this
POC's rules forbade deleting them). None of the plan's concrete steps ran:
moving `pkg/client`, `pkg/client/locator`, `pkg/tokenstore`, `internal/ctxutil`
into a new `forge/forgeclient` module; parameterizing the package-level
logger; porting or deleting `pkg/client/spaceblobreplicate.go` (the sole
importer of the pre-`ucantone` `go-ucanto`/`go-libstoracha` stack in the
client's closure); deleting the ingot forks and wiring `replace` directives
in `ingot`/`smelt`.

### Phase 5 — Harness: drop `guppy` as the compat driver

**This is the one urgent, non-deferrable item this pass surfaced.** The
compat suite this POC built and validated depends on the guppy *CLI binary*,
not the client library. Verified directly:

```
smelt/tests/compat/compat_test.go:69:  "github.com/fil-forge/forge/smelt/pkg/clients/guppy"
smelt/tests/compat/compat_test.go:477: gup, err := guppy.NewContainerClient(s)
smelt/pkg/clients/guppy/container.go:76: return c.stack.Exec(ctx, "guppy", args...)
smelt/pkg/clients/guppy/container.go:100: c.guppyExec(ctx, "login", email)
smelt/pkg/clients/guppy/container.go:148: c.guppyExec(ctx, "upload", "source", "add", spaceDID, path)
```

`ContainerClient` execs `guppy login`/`upload`/`retrieve` *inside the running
guppy container* — it drives the CLI, not `pkg/client` as a Go dependency.
Decision 3 archives that CLI. If that happens before this phase is done, the
entire compat suite (`TestPinnedPeer`, `TestRollingUpgrade`, and everything
riding on top via `assertUploadRetrieve`) stops being runnable — not stale,
not degraded, unable to boot a driver at all. **This has to land in the same
branch as archiving guppy, not after.**

The plan's prescribed fix, unbuilt: make an S3-path driver
(`smelt/pkg/s3glue`, already used by `smelt/tests/s3/forge_eviction_test.go`,
which the plan calls a *better* assertion than today's read-after-write — it
forces a real `/content/retrieve` against `piri` by deleting the local spool
first) the primary compat assertion; add a protocol-path driver in `smelt`
using the new shared client (once Phase 4 exists) for what S3 can't express
(spaces, replication factor, `/provider/add`, `/access/delegate`,
`/blob/replicate`); confirm the S3 path exercises piri's write side as
thoroughly as today's 10MB multi-shard upload before retiring
`assertUploadRetrieve`. The plan's fourth item here — the `otherThan`
(`{"piri","ingot","upload","hilt"}`) vs. baseline-map (`{"piri","ingot","sprue","hilt"}`)
naming seam — **is already fixed**, by this POC's `d6eaf59`, which derives
both from one table keyed by compose name and image name; no further work
needed on that specific point.

### Phase 6 — Tag-scheme cleanup

Not investigated this pass; the plan itself marks it optional, low priority,
and independent of everything above.

## Cross-cutting corrections to this POC's own record

- `commands-move.md` and `findings.md` S7 described "a decision on
  `commands/ucan/attest` vs `attestation`" as open. The plan already designed
  the resolution (both move into a new, standalone extension module,
  importing only `ucantone` and multiformats) — this was a skipped
  *execution* item, not an open *design* question. Corrected here; `findings.md`
  should point at this document rather than restate it as undecided.
- The "protocol gates: none were built today" line in `findings.md`
  ("What a real migration would need") undersells the codegen gate, which
  partially exists for `commands`. Corrected above.

## Recon: `delegator`

Read-only pass over `/home/user/fil-forge/delegator` (`main`), compared
against forge's five existing services on this branch.

**What it does.** The network's storage-provider onboarding gatekeeper: it
validates a node's DID/endpoint/UCAN proofs and its on-chain contract
approval, records registration in DynamoDB, and mints the UCAN delegation
chains that let a registered node call `/claim/cache` on the indexing
service and `/space/egress/track` on the egress tracker (`AGENTS.md:6-17`).

**Module shape — a good fit.** One Go module, `go 1.27.0` — the same
directive forge's five services and shared modules carry today (Experiment
A's bump). ~2,248 lines of production Go across 14 files, ~1,005 lines of
test (all in one `test/system_test.go`, no per-package unit tests) — smaller
than `ingot`/`piri`/`smelt`, comparable to `hilt`/`sprue`. Uses `fx` and
`echo`, already used by most existing services; `cobra`+`viper`, universal
across all five. Its system test runs against an in-memory `mockStore` +
`httptest`, no Docker dependency — a clean unit-tier fit, though there's no
existing per-package test suite to model a `unit` CI job on the way the other
five have.

One real convention mismatch, cheap to fix: `main.go` sits at the module
root importing a `cmd` package that is *not* `package main` (`main.go:1-13`);
every forge service instead has `package main` inside `cmd/` itself. Forge's
shared `docker/Dockerfile` already takes the main package as a build arg
(default `./cmd`), so this needs a build-arg value, not a code change.

**`libforge` coupling — small and contained.** Five `commands/**` packages
(`commands/blob`, `commands/blob/replica`, `commands/claim`, `commands/pdp`,
`commands/space/egress`), from exactly 3 files (2 production —
`cmd/gen.go`, `internal/services/registrar/delegator.go` — 1 test). Also
`libforge/identity` (5 files) and `libforge/ucan` — the `ucanlib` package
Phase 3 wants renamed into `ucantone`, not moved to forge (1 file). All of
this sits under `internal/`, so **no type-identity seam**: delegator's only
externally-importable package, `client/`, exposes nothing but primitives and
one `*ucantone/did.Document` — no `libforge/commands` type crosses its public
API. Confirmed the practical consequence: rewriting delegator's imports to
`forge/commands` would not break any other repo's compile the way it did for
piri-signing-service.

**The one finding that actually matters for how this gets done.** `piri`
already pins `github.com/fil-forge/delegator` as a real Go module dependency
(`piri/go.mod:12`) and imports `delegator/client` in
`piri/cmd/cli/setup/register.go:36` (`delgclient.New`, `.IsRegisteredRequest`,
`.RegisterRequest`). No other forge service references delegator at all, and
delegator depends on none of the five. So today the edge is one-directional
and external: piri → delegator. **Folding delegator in as `forge/delegator`
while piri keeps importing its `client` package makes piri the monorepo's
first service that imports another service** — directly against `ci.yml`'s
own stated invariant, `# No service module imports another` (`ci.yml:31`).
Two ways to keep that invariant: keep piri's import pointed at the external,
still-published `github.com/fil-forge/delegator` module (which undercuts part
of the reason to fold delegator in at all), or carve `delegator/client` out
as a third shared module next to `commands/`/`internal/` that both `piri`
and `forge/delegator` import, neither importing the other. This needs a
decision before writing the module-move code, not after.

**Docker/CI/deploy — the most work of anything found so far.** Delegator has
its own Dockerfile (alpine runtime, not forge's debian-slim; no `ARG
SERVICE`; `CMD` baked in; binary at `/usr/bin/registrar` — a naming split
between "registrar" (binary/CLI/env prefix) and "delegator" (module/repo)
that its own `AGENTS.md:188-189` already calls a quirk). Its CI is ten
separate workflow files built on `ipdxco/unified-github-workflows` reusable
workflows — a completely different release mechanism from forge's home-grown
`release.yml`/GoReleaser-with-an-explicit-config pattern (delegator has no
`.goreleaser.yaml` anywhere in its history; it relies on GoReleaser's
defaults). Most consequentially: delegator has **real, working deploy
automation** — `deploy/app` + `deploy/shared` (storoku ECS Terraform
modules), a `.storoku.json` app manifest, applied against three live
environments (`warm-staging`, `forge-production`, `forge-test`), plus a
cross-repo GitHub App dispatch that notifies an `infra-central` repo on every
image bump (`README.md:239-251`). **None of forge's five services have any
CI-driven, storoku-style container deploy pipeline** — nothing to extend,
nothing to model this on. (Correction from the `indexing-service` recon
below: `forge/piri` does have its own `deploy/full-node/` Terraform —
bare-EC2 provisioning scripts, a different topology entirely, and not wired
into any GitHub workflow — so "no deploy automation at all" overstates it
slightly; the precise gap is the ECS/container pattern specifically.)
Folding delegator in means the monorepo inherits its first real
*CI-driven container* deploy pipeline, not just a fourth build.

No forked/duplicated code found (no "carried from"/"copy of"/"keep in sync"
comments anywhere) — delegator is not another `ingot/forgeclient`-shaped
problem.

## Recon: `piri-signing-service`

Read-only pass over `/home/user/fil-forge/piri-signing-service` (`main`,
`8472ad9`), cross-checked against `forge/piri` on this branch.

**What it does.** A signing oracle: it wraps a cold wallet and produces
EIP-712 signatures for PDP operations (create/delete data set, add/remove
scheduled pieces), so piri storage nodes ask it to co-sign instead of
holding the Storacha operator's private key themselves (`AGENTS.md:8-12`).

**Module shape.** One Go module, `go 1.25.3` — the oldest directive of
anything in scope (forge's five services and shared modules are `go 1.27.0`
on this branch; the live per-service repos' own `main` are 1.27.0 except
`smelt` at 1.25.9). `main.go` sits at the module root with no `cmd/`
directory at all — an even flatter layout than delegator's, same fix (build
arg or relocate). Small: 1,572 non-test lines across 15 files, 1,194 test —
comparable to one mid-sized package inside `piri`, not a whole service.

**`libforge` coupling — narrow and complete.** Exactly two `commands/**`
packages: `commands/pdp/sign` (8 files — touches nearly every non-legacy,
non-config file in the service) and `commands/access` (3 files). Plus
`libforge/identity` (`main.go`, `pkg/server/server.go`) — which is *not* part
of forge's `internal/` module and is already imported directly from libforge
by all five existing services too (`smelt/pkg/stack/proofs.go`,
`smelt/pkg/generate/keys.go`, four sites in `sprue`, `hilt/integration/network.go`),
so this needs no new handling, it's already the norm. No `did`, `attestation`,
`receipt`, `testutil`, `ucan` (ucanlib), `blobindex`, or `piece` imports.

**The `SignAddPieces`/`PieceProofs` seam — confirmed exactly as predicted,
nothing more complicated underneath.** `pkg/types.SigningService.SignAddPieces`
(`pkg/types/types.go:71-82`) is the *only* method, on the *only* interface,
anywhere in the service whose signature carries a `libforge/commands/**`
type (`[]sign.PieceProofs`); every other method — including all of the
sibling `OperationSigner` interface — takes only primitives and
`eip712.MetadataEntry`. Three implementations echo the same parameter
(`pkg/client/client.go:115`, `pkg/inprocess/signer.go:50`, and piri's own
`proofServiceSigner.SignAddPieces` at
`forge/piri/pkg/service/signer/proofservicesigner.go:88`). The workaround
already on this branch — `roots_add.go:14` and `proofservicesigner.go:10-12`
import `libforge/commands/pdp/sign` under an alias, specifically to match
this one interface, while every other `commands/pdp/sign` use in piri is the
in-repo `forge/commands/pdp/sign` — is confined to exactly those two files.
**If `piri-signing-service` joins the monorepo and 10 of its files
(`pkg/types/types.go`, `pkg/server/server.go`, `pkg/server/handlers/{sign,sign_test,access_grant,access_grant_test}.go`,
`pkg/inprocess/signer.go`, `pkg/client/{client,client_test}.go`) get their
import path switched to `forge/commands/**`, the seam disappears outright** —
`PieceProofs` becomes one type again, and both alias workarounds in `piri`
can be deleted. No other type-identity seam exists anywhere in the service
(checked every exported signature touching a libforge type). Only `piri`
depends on it as a Go module — no other fil-forge repo does.

**Docker/CI — the same shape as delegator, a second data point that this is
a real pattern, not a one-off.** Own Dockerfile (repo-root context, no
`MAIN_PKG`/`ARG SERVICE` — same mismatch class as delegator's). Nine of its
own workflow files, several wrapping the same `ipdxco/unified-github-workflows`
reusable workflows delegator uses (`go-check.yml`, `go-test.yml`,
`release-check.yml`, `releaser.yml`, `tagpush.yml`) — this is evidently a
standing fil-forge convention for standalone services, not specific to
either repo. `version.json` present, same shape forge's `release.yml`
expects. Its own storoku Terraform pipeline (`deploy/app`, `deploy/shared`,
`.storoku.json`) applies to `warm-staging`/`forge-production`/`forge-test` —
again, no equivalent for any of forge's five in-monorepo services.
`publish-ghcr.yml` also dispatches a `bump-deployed-image` event to a
`fil-forge/infra-central` repo on every push to `main`, the same cross-repo
mechanism delegator has.

**Concrete findings that change scope, beyond the seam:**
- **`forge/piri`'s pin on `piri-signing-service` is stale** — `2026-06-19`
  pseudo-version vs. the local clone's `2026-08-22` HEAD. Folding it in as
  source picks up two months of upstream change atomically; that range
  hasn't been diffed and should be, separately from the seam fix.
- **`smelt` already half-expects this service to be in-repo.**
  `smelt/CLAUDE.md`'s workspace table already lists `piri-signing-service` →
  service name `signing-service` → container binary `/usr/bin/signer`, so
  `SMELT_WORKSPACE=1` local-build tooling would pick it up the moment it's
  added to `go.work`. `smelt/systems/signing-service/compose.yml` currently
  pulls the floating `ghcr.io/fil-forge/piri-signing-service:main` — one of
  the four floating stack components S15/latent-bug-14 already flags as a
  source of compat-run irreproducibility.
- **Security-relevant, flagged unprompted per this org's key-handling
  rules**: this service holds a signing key (`config.go`'s
  `signing_key`/`signing_key_path`/`signing_keystore_path`) that authorizes
  on-chain PDP transactions via EIP-712 signatures. It already follows good
  practice (external secret, `.storoku.json` marks it `external: true`,
  never logged) — noted here so whoever reviews the actual migration treats
  this as key-custody code, not ordinary service code, since neither this
  POC nor the original plan called that out separately.
- **Legacy, unauthenticated `/sign/*` routes ship alongside the UCAN path**;
  the service's own `AGENTS.md` says assume deployed piri nodes may still use
  them. Not a reason to hold up the move, but not dead code to delete either.
- Heavy transitive dependencies for a ~2.8k-line service (`gnark-crypto`,
  `go-eth-kzg`, `blst`, a zkVM runtime, all via `go-ethereum`) — no action
  needed, but the post-move `go mod tidy` diff (`ci.yml`'s existing check)
  is worth reading, not just trusting to pass.
- Needs no `internal/` module — only a `commands/` replace, unlike `piri`
  itself (which also needs `internal/` for `sigv4`/`s3perm`/etc.).

## Recon: `indexing-service`

Read-only pass over `/home/user/indexing-service` (`ba73105`), cross-checked
against forge's five services, `delegator`, and `piri-signing-service`. Unlike
those two, this is **not yet a settled decision** — the user proposed it
("it's a service, so I believe it should be in the monorepo") and asked for
thoughts and concerns, not a recommendation; this recon exists to ground that
discussion in source rather than the two-line summary in its own `AGENTS.md`.

**What it does.** The network's content-routing index: bridges IPNI with
UCAN content claims, executing `assert/equals`/`assert/index`/`claim/cache`
invocations from storage/upload services and answering CID/multihash lookups
for clients and gateways (`AGENTS.md:7-13,83-88`). Deployed as a standalone
ECS container via storoku Terraform; `cmd/lambda/` is vestigial from a
retired architecture (`AGENTS.md:11-13`).

**Module shape — sized like one of forge's own services, not like the other
two candidates.** `go.mod` pins `go 1.25.7` exactly (needs the same bump to
`1.27.0` the others got). 8,215 production LOC across 70 files (excluding
tests, mockery mocks, and cbor/json codegen), plus 3,360 test LOC, 3,883 LOC
of generated mocks, 749 LOC of codegen — **~3.7× delegator's production LOC,
~5.2× piri-signing-service's**, reproduced fresh with the same counting
method both existing recons used. For scale, against forge's own five
services counted the same way: hilt 7,443, smelt 9,495, sprue/ingot ~15,600
each, piri 32,190 — indexing-service's 8,215 sits almost exactly between hilt
and smelt. It is not comparable in scale to delegator or piri-signing-service;
it is comparable to a forge service. One thing already in the right shape:
`cmd/main.go` is already `package main` inside `cmd/`, matching forge's
shared Dockerfile's default `MAIN_PKG=./cmd` — unlike both other candidates,
no build-arg override or file relocation needed on that axis.

**`libforge` coupling — the widest of any candidate examined.** A fresh
grep (import-path only, excluding tests and mockery-generated files) finds
**12 distinct `libforge` packages across 22 production files**: `blobindex`
(14 files), `commands/assert` (8), `identity` (4), `digestutil` (4),
`ucan/retrieval` (3), `bytemap` (3), `commands/content` (2), `commands/claim`
(2), `commands` base (2), `ucan`/`ucanlib` (1, `pkg/service/service.go:29`),
`testutil` (1), `jobqueue` (1). Cross-checked against
`package-inventory.json` and found consistent — the small gap is exactly the
two mockery-generated files the grep deliberately excluded and the inventory
tool didn't. Compare: delegator touches 3 `commands/**` packages + `identity`
+ `ucan`; piri-signing-service touches 2 `commands/**` packages + `identity`.
**Indexing-service touches more distinct `libforge` packages, in more files,
than delegator and piri-signing-service combined**, landing across every one
of the plan's Phase 3 disposition buckets simultaneously — including being
`blobindex`'s single heaviest consumer anywhere, heavier than forge's own
five services combined (`package-inventory-notes.md`).

**Type-identity seam — real, but structurally harmless.** `pkg/types.go`'s
`RetrievalRequest.Range *contentcaps.Range` carries a `libforge/commands/content`
type. Checked whether this creates a delegator/piri-signing-service-style
cross-repo seam: it does not — `package-inventory.json`'s 216-record
`packages` list has **zero** entries with an `importPath` under
`github.com/fil-forge/indexing-service`, meaning nothing in the fil-forge
fleet imports indexing-service as a Go module; it was only ever scanned as a
consumer. This is the mirror image of delegator's situation: delegator is
depended on *as a service* by `piri` (the "first service-imports-a-service
edge" finding above); indexing-service has zero reverse Go-module dependents,
so rewriting its imports to `forge/commands/**` cannot break anyone else's
build the way it would have for a hypothetical importer.

**A second, independent `go-ipni-tools/pkg/advertisement` seam.** Both
`commands-move.md` §5 and `findings.md` S7 originally described `piri` as
`pkg/advertisement`'s only consumer. It isn't:
`indexing-service/pkg/service/service.go:13,519,538` independently imports
`go-ipni-tools/pkg/advertisement` and calls both `EncodeContextID` and
`ShardCID` against its own `libforge/commands/assert.LocationArguments`. Both
documents are corrected. The fix is unchanged (move the 109-LOC leaf package
into `forge/commands` alongside `commands/**`, not all of `go-ipni-tools`),
but it now has two independent forcing functions — if `indexing-service`
joins the monorepo before `pkg/advertisement` moves, it hits the identical
compile error `piri` already did.

**`go-ipni-tools` coupling is far heavier here than for `piri`.** Fresh grep:
indexing-service imports 8 of `go-ipni-tools`'s 9 packages across **23
files** (`pkg/queue` 10, `pkg/metadata` 10, `pkg/store` 9, `pkg/publisher` 4,
`pkg/notifier` 3, `pkg/server`/`pkg/client`/`pkg/advertisement` 1 each); `piri`
imports it in exactly 9 files (verified identically in both the branch's copy
and the live clone). `findings.md`'s S7/S14 material calls `go-ipni-tools`
"the plan's clearest stays-out case" partly because only one first-party
service imports it — true of forge's five in-monorepo services today, but
**indexing-service is actually the heavier consumer of the two, by a wide
margin**. Doesn't change the "repo stays out, one leaf package moves"
conclusion, but the eight non-`advertisement` packages (~2,250 LOC of generic
IPNI plumbing) become a real, immediate external dependency `forge/indexing-service`
would lean on hard from day one, as a published module outside the monorepo.

**A previously untracked first-party dependency: `automobile`.**
`github.com/fil-forge/automobile` (a CAR encode/decode helper) is imported in
`pkg/server/server.go:305` and `pkg/service/queryresult/queryresult.go:50`.
Zero mentions anywhere in `docs/consolidation/*.md` before this pass. It
doesn't import `libforge/commands`, so it carries none of the seam risk
`go-ipni-tools` does — but it's a first-party repo this consolidation effort
has not looked at even once, and it happens to be one of the ten repos this
session already has a designated working branch for. Worth a look before,
not after, indexing-service actually moves.

**Deploy footprint — the biggest and most stateful of the three candidates,
by every measure checked.** `.storoku.json`: indexing-service declares 2
DynamoDB tables, 5 S3 buckets, 3 SQS queues, 4 Redis caches; delegator
declares 1 table and nothing else (plus 3 secrets); piri-signing-service
declares nothing but 1 secret (its signing key). `deploy/app/main.tf` is 143
lines vs. delegator's 82 and piri-signing-service's 71. `deploy.yml` wires
five environments (`staging`, `warm-staging`, `production`, `forge-production`,
`forge-test`) vs. exactly three for the other two — the two extras are a
legacy carryover from the retired Lambda architecture, consistent with its
own `AGENTS.md`. (This pass also corrected the "no forge service has any
deploy automation" line above: `forge/piri` has its own `deploy/full-node/`
Terraform, a bare-EC2 topology entirely unwired to CI — the precise gap is
the CI-driven ECS/container pattern specifically, not deploy automation as
such.)

**CI.** Same `ipdxco/unified-github-workflows` foundation as both other
candidates. One wrinkle: `workspace-test.yml` (a non-blocking job that tests
against a same-named branch in a libforge/indexing-service sibling repo pair,
explicitly documented upstream as never a required check) is present here and
in piri-signing-service, absent in delegator — a fil-forge convention adopted
inconsistently, and one that becomes redundant, not broken, the moment
indexing-service is tested via forge's own `compat.yml`/`ci.yml` instead.

**Docker.** Same runtime-base mismatch class as both other candidates
(`golang:1.25-bookworm` build → `alpine:latest` runtime, vs. forge's shared
`debian:bookworm-slim`) — not worse, not better. Easier to adopt forge's
shared Dockerfile on one specific axis, since the entry point is already
`package main` inside `cmd/`.

**Version/maturity.** `version.json` reads `v1.13.4`, against `v0.1.1`
(delegator) and `v0.1.0` (piri-signing-service) — a materially more mature,
already-multi-environment-deployed service, a different migration-risk
profile than two barely-released services independent of code size.

**Blast radius — largely orthogonal to the in/out question, but sharpens
Phase 2's priority specifically for this candidate.** Its own `AGENTS.md`
frames the blast radius as unbounded ("every client and gateway," "all
publishers and consumers... across the network") where delegator's and
piri-signing-service's own framings name specific, enumerable downstreams.
That's real, but moving the Go source into `forge/` doesn't change who
outside the repo must coordinate on a schema change — it only tightens the
loop between indexing-service and forge's other services once they share a
PR and a CI run. The concrete implication: indexing-service's hardest
consumers (external gateways) are structurally invisible to any CI this
monorepo could run, monorepo or not — which is exactly the gap Phase 2
("protocol gates," landed *before* Phase 3 by the plan's own sequencing) is
meant to cover. That argues for prioritizing Phase 2 before or alongside
folding indexing-service in specifically, not for keeping it out.

## What this recon changes about readiness

Both `delegator` and `piri-signing-service` are a **clean fold-in** for their
`libforge` coupling — smaller and more contained than piri's own two seams,
and both fully resolve (no residual workaround) once the service is in-repo
and its imports are rewritten. Neither depends on any of forge's five
services as a Go module.

`indexing-service` — not yet a settled decision, unlike the other two — is a
different shape entirely: sized like a sixth forge service rather than a
small satellite, with `libforge` coupling wider than the other two combined
and a deploy footprint bigger than both. Its own type-identity seam is
harmless (no reverse Go-module dependents), which is the one respect in which
it's an *easier* case than delegator. See "Recon: `indexing-service`" above
for the full picture; the items below fold in only what's new since that
recon, on top of the five delegator/piri-signing-service items already here.
The `piri`↔`delegator` edge is the one structural surprise: unlike
`piri-signing-service` (only `piri` depends on it, and that dependency
disappears once both share `forge/commands`), delegator is depended on by
`piri` as a **service**, through `delegator/client` — folding both in without
a decision here creates the monorepo's first service-imports-a-service edge,
against `ci.yml`'s own stated invariant.

New, concrete decisions this recon surfaced, none blocking a start but each
worth resolving before or during the actual module moves:

1. **Team to confirm; proceeding with a default so this doesn't block a full
   attempt.** Two options: keep `piri`'s import of `delegator/client` pointed
   at the external, published `github.com/fil-forge/delegator` module, or
   carve the client out as its own piece both `piri` and `forge/delegator`
   can import without importing each other. Within the second option, two
   further shapes: add `internal/client/delegator` next to the already-proven
   `internal/client/hilt` (same fix ingot/hilt already uses, and for a
   heavier case — ingot uses `client/hilt` continuously, piri only touches
   delegator's client once, at operator setup), or give `delegator/client`
   its own nested `go.mod` one directory under the service, replaced in by
   `piri`, keeping it physically inside `delegator/`'s own tree — unproven in
   this repo, `commands`/`internal` are siblings of the services, not nested
   inside one. **Default for the full attempt: `internal/client/delegator`**,
   for the "reuse what's already proven" reason. Revisit with the team once
   the branch is far enough along to look at together.
2. Diff `piri-signing-service`'s two months of undiffed upstream change
   before merging it as source, separately from the seam fix.
3. Decide whether either service's storoku Terraform deploy pipeline comes
   into the monorepo as-is, gets replaced by whatever mechanism forge's five
   services eventually use (none exists today), or stays external and just
   gets pointed at monorepo-built images.
4. Both services' entry points (`main.go` at module root, no `cmd/`) need
   either a build-arg override or a small relocation to fit forge's shared
   Dockerfile — mechanical, not a design question.
5. Both bring their own ten-workflow, `ipdxco/unified-github-workflows`-based
   CI, entirely separate from forge's home-grown `ci.yml`/`release.yml` —
   reconciling or replacing this is real work, not yet scoped in detail.

New items from the `indexing-service` recon, same status (not blocking,
worth resolving before or during a real move):

6. **`automobile` needs its own look before indexing-service moves.** A
   first-party dependency (`pkg/server/server.go:305`,
   `pkg/service/queryresult/queryresult.go:50`) with zero prior mention in
   this consolidation effort, despite already having a designated working
   branch in this same session's scope. No seam risk found (doesn't import
   `libforge/commands`), but "no seam risk" was asserted by one recon pass,
   not verified the way `libforge`/`go-ipni-tools` were — worth the same
   treatment before treating it as a non-issue.
7. **`go-ipni-tools/pkg/advertisement` now has two independent forcing
   functions, not one** — `commands-move.md` §5 and `findings.md` S7 are
   corrected. Sequencing consequence: if `indexing-service` joins the
   monorepo before `pkg/advertisement` moves into `forge/commands`, it hits
   the same compile break `piri` already did. The fix doesn't change, but the
   ordering now matters for a second, independent reason.
8. **If `indexing-service` joins, `go-ipni-tools`'s other eight packages
   become a real, immediate external dependency** — indexing-service imports
   8 of 9 `go-ipni-tools` packages across 23 files, more than twice `piri`'s
   9. Not a blocker (that's exactly what "stays out, cleanly" means for a
   library `go-ipni-tools`'s size), but worth knowing the monorepo would lean
   on it hard from day one, not lightly.
9. **`indexing-service`'s deploy footprint (2 DynamoDB tables, 5 S3 buckets,
   3 SQS queues, 4 Redis caches, 5 environments) is larger than delegator's
   and piri-signing-service's combined.** The item 3 question above ("decide
   whether either service's storoku pipeline comes in as-is, gets replaced,
   or stays external") gets harder, not easier, if indexing-service is also
   in scope — it's the one candidate where "stays external, monorepo just
   points at the image" is most likely to be the pragmatic near-term answer,
   given how much live infrastructure state (not just a container) already
   depends on this exact deploy pipeline in five environments today.

## The `go-ipni-tools` seam, resolved

`commands-move.md` §5 already established that `go-ipni-tools` splits at
package granularity: `pkg/advertisement` (109 LOC, Forge-specific) moves
alongside `commands`; the other eight packages (~3,300 LOC) are generic IPNI
plumbing that stays out, cleanly, forever. It left one thing open: two of
those eight packages (`pkg/queue`, `internal/testutil`) still import
`libforge/jobqueue` and `libforge/testutil`, both slated for forge's
`internal/` by the plan's own Phase 3 table — which is unreachable from
outside forge by construction. This pass closed that out (full derivation in
`commands-move.md` §5 and `findings.md` S18/S19):

- **`jobqueue`** is 265 LOC, zero non-stdlib dependencies, zero Forge content
  — a generic async worker pool. Direct grep (not `package-inventory.md`,
  which has a real gap for `-consumer`-mode modules — see below) finds
  exactly two external consumers: `go-ipni-tools/pkg/queue/poller.go` and
  **`indexing-service/pkg/construct/construct.go:25`** — a second consumer
  this pass found that neither `commands-move.md`'s first pass nor
  `findings.md` S7 had surfaced. This connects directly to the discussion
  below: if `indexing-service` joins the monorepo, `go-ipni-tools` becomes the
  *only* permanent external consumer, and vendoring the 265 lines into it
  (no dependency, cheap to own) is the simplest correct fix. If
  `indexing-service` stays out, there are two permanent external consumers and
  `jobqueue` deserves one small shared home outside `internal/` instead of two
  vendored copies.
- **`testutil`** (83 LOC) is mostly redundant already: nine of its symbols are
  verbatim re-exports of `ucantone/testutil`, and its `Must` duplicates
  `ucantone/testutil.Must` symbol for symbol. The only real content —
  `Must2` and six named fixed identities (`Alice`/`Bob`/`Carol`/`Mallory`/
  `Service`/`WebService`) — fits directly into `ucantone/testutil`, which its
  own `AGENTS.md` already says exists for "tests here and in dependents."
  Recommendation: fold the delta in, point every external consumer
  (`go-ipni-tools`, `indexing-service`, and `guppy/pkg/client`) at
  `ucantone/testutil`, and delete `libforge/testutil` rather than relocate
  it. The `ucanlib` branch above demonstrates the consumer side of this at
  small scale (its tests use `testutil.RandomIssuer(t)` in place of the fixed
  identities, since they only need distinct principals) without yet doing the
  fixture fold-in itself.
- **Tooling note**: `package-inventory.md`'s JSON only records full per-package
  import edges for modules walked as "subjects"; `-consumer`-mode modules
  (`go-ipni-tools` included) show up as scanned but contribute no package
  records, so a query against the JSON for "who imports `libforge/jobqueue`"
  silently returns nothing even though two real consumers exist. Every
  `-consumer` finding in this POC was cross-checked against a direct grep
  before being written down, so nothing already concluded is wrong — but the
  JSON itself should not be trusted as exhaustive here without that check,
  and `tools/inventory` should either walk full package records for
  `-consumer` modules too, or say in its own output that it doesn't.

Neither fix is executed anywhere yet (the `testutil` fold-in touches a
package other things already depend on; the `jobqueue` fix depends on the
still-open `indexing-service` decision below) — both are ready to act on once
that decision lands.
