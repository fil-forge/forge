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
| `ucan` (package `ucanlib`, collides with `ucantone/ucan`) | rename into `ucantone` | not moved; still imported from libforge directly — `hilt/integration/network.go`, `hilt/pkg/api/tenants_test.go`, `hilt/pkg/api/service/tenant/service_test.go`, `hilt/pkg/fx/upload.go`, `hilt/pkg/store/delegation/memory/store.go`, `hilt/pkg/client/{upload.go,upload_test.go}`, `ingot/uploader/{forge.go,forge_test.go}` (verified by grep) |
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

## Recon: `delegator` and `piri-signing-service`

Pending — two read-only recon passes are running in parallel (this
document's structure follows the same pattern Experiment B used for the
original five services: module shape, `libforge` dependency graph, seam
candidates, Docker/CI fit). Appended below once both report.
