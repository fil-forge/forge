# Cost report — reconciling `forge` against current `libforge` and `ucantone`

Experiment A of the consolidation POC (2026-09-03). The RFC's Option 3 accepts
a two-step rollout for library changes because "a library bump is an explicit,
tested PR in the consuming repo — the normal Go workflow". This measures what
that workflow cost after being deferred for five weeks.

## What was bumped

| module (all five service modules) | from | to |
|---|---|---|
| `github.com/fil-forge/libforge` | `v0.0.0-20260731172903-928cf2a21b7e` (2026-07-31) | `v0.0.0-20260903190243-f4b13f7e32f6` |
| `github.com/fil-forge/ucantone` | `v0.0.0-20260727203046-ccb77059de44` (2026-07-27) | `v0.0.0-20260828153820-8d7eb73066ce` (main, 2026-08-28) |

`f4b13f7` is not libforge `main`. It is libforge `main` (`2585ed1`, 2026-08-28)
with the single commit of the still-open draft
[PR #52](https://github.com/fil-forge/libforge/pull/52) cherry-picked on top,
pushed as branch `claude/forge-monorepo-poc-p9w0yr` on `fil-forge/libforge`.
That was necessary because forge's pin *is* PR #52's head: four packages forge
imports (`sigv4`, `s3perm`, `client/hilt`, `ucan/zapucan`) and five symbols in
`commands/s3` exist only on that PR (see `findings.md` S1). Reconciling forge
with libforge `main` proper is therefore not a bump; it is either merging PR #52
or relocating those packages, and the second path is Experiment C.

Delta absorbed: libforge 12 commits past PR #52's base (9 dependabot/CI,
`fix: attestation idempotency (#51)`, `feat: add tenant DID to authorization
response (#64)`); ucantone 15 commits (`fix: align receipts with the updated
receipt spec (#49)`, `fix: invocation issued at time and receipt timestamps
option (#50)`, `refactor: adopt CBOR/JSON codegen method from libforge (#52)`,
`fix(validator): skip unsupported verification methods (#53)`, ML-DSA-44
signer (#37), `feat(container): export the transport decode step (#55)`,
dependabot/CI).

## Wall clock (UTC, from `expA-timing.log`)

| step | time | elapsed from start |
|---|---|---|
| `GOWORK=off go get <libforge> <ucantone>` in five modules | 19:08:14 → 19:08:56 | 0:42 |
| first `go build ./...` of all five (piri `-tags skiff`), including downloads | → 19:12:07 | 3:53 |
| `go vet`, `go mod tidy`, `go test ./...` for all five | 19:12:30 → 19:15:33 | 7:19 |
| rebasing PR #52 on libforge `main` beforehand (cherry-pick, conflicts in `go.mod`/`go.sum` only, tidy, build, vet, test, gofmt two files) | ≈ 6 min | — |

To a building tree: **under four minutes.** To locally green vet/tidy/test:
**under eight minutes.** Compile errors encountered: **zero**, in every module.

CI-side figures, GitHub Actions on the bumped head `a7b6449`
([run 33813866714](https://github.com/fil-forge/forge/actions/runs/33813866714),
dispatched 22:36:32 UTC on the pointer branch `claude/forge-monorepo-poc-p9w0yr-expA`):
all five `unit` jobs and the `stack` job (four images, e2e smoke suite, S3
system suite) **green**, completed 22:55:00 — **18 min 28 s** from dispatch to
green, of which `stack` is about 12 min. Two earlier attempts on the same
commits are part of the honest cost: the first dispatch failed in 76 s on the
`go work sync` go.sum gap (`findings.md` S8) and the second lost `unit sprue`
to a `TestWireApp/aws` flake that passed on every other run.

## Files and modules touched

| module | `go.mod` | `go.sum` | source files |
|---|---|---|---|
| `hilt` | +44 −46 | +60 −104 | 0 |
| `ingot` | +20 −21 | +33 −42 | 0 |
| `piri` | +76 −73 | +110 −154 | 0 |
| `smelt` | +47 −51 | +49 −67 | 0 |
| `sprue` | +46 −51 | +65 −112 | 0 |
| `smelt/systems/stress-tester` (by `go work sync`, not bumped) | 52 lines | 106 lines | 0 |
| `go.work` | `go 1.26.5` → `go 1.27.0` | | |
| `docker/Dockerfile` | `golang:1.26-bookworm` → `golang:1.27-bookworm` | | |

Ten manifest files in the modules that were bumped, two more from the
workspace sync, one build file. **No Go source changed.** Most of the `go.mod`
churn is transitive: `golang.org/x/*`, `aws-sdk-go-v2` (hilt/sprue),
`dag-json-gen 0.0.8 → 0.0.9`, `cbor-gen 0.3.1 → 0.3.2-pre`, `go-cid 0.6.1 →
0.6.2`, `testify 1.11.1 → 1.12.1`, `docker 28.5.1 → 28.5.2` (piri),
`lib/pq` (smelt).

## Behaviour change vs import-path change

None of either in forge's source. What did change underneath:

- **Go toolchain.** Both libraries declare `go 1.27.0`. `go get` raised every
  service module's `go` directive from `1.26.5` to `1.27.0`, `GOTOOLCHAIN=auto`
  downloaded `go1.27.0`, and `go work sync` raised `go.work` to match. The
  Dockerfile's `golang:1.26-bookworm` would still build (the toolchain
  auto-downloads inside the image) but was bumped to `1.27-bookworm` so the
  image builds with the toolchain the modules declare. The branch
  `refactor/drop-replaces` had deliberately unified every module on 1.26.5
  ("build: one Go version across every module — 1.26.5"); a library bump
  silently undoes that unless every module bumps together, which the monorepo
  makes a single commit and the polyrepo makes five PRs.
- **Wire behaviour, potentially.** ucantone #49 aligns receipts with the
  updated receipt spec and #50 changes issued-at/timestamp handling. forge's
  code compiles and its unit tests pass against both, which says the Go API
  held, not that the bytes on the wire held. Whether a bumped sprue still
  interoperates with a piri image built at the old pin is exactly the question
  `compat.yml` exists for; see `compat-validation.md`.
- **Test attribution.** After the bump, piri fails 7 packages and smelt 3;
  every one is `rootless Docker not found` / `failed to connect to the docker
  API` from testcontainers, and the identical set fails at baseline `f60dd59`
  in a clean worktree. Not attributable to the bump. hilt (10 s), ingot (11 s)
  and sprue (14 s) pass.

## Unrecoverable

Nothing — *because the PR #52 branch still exists.* Had it been deleted, the
four packages (1,175 handwritten lines: `sigv4` 848, `client/hilt` 196,
`s3perm` 77, `ucan/zapucan` 54) and the `commands/s3` additions (386 lines)
would have had to be recovered from forge `main`'s history at `96a672e`, where
they lived under `hilt/pkg/`. Recoverable, but no longer a bump.

## What required judgment rather than mechanical translation

These are the items a two-step rollout would have needed a human for:

1. **What "current libforge" means when the pin is an unmerged PR.** No
   mechanical answer. Options: rebase the PR (done here, on a branch), merge
   it (blocked on a review with four security-relevant findings and an
   unanswered "why is this necessary?"), or relocate the packages
   (Experiment C). The bump itself was trivial once this was decided; deciding
   it is the cost.
2. **Whether to accept a toolchain major bump as part of a dependency bump.**
   `go get` does it silently; noticing it required reading the diff.
3. **Whether `go work sync`'s rewrite of `stress-tester` is wanted.** It
   changes a module nobody touched, to versions the workspace was already
   using. Committing it makes CI (`GOWORK=off`) match local; not committing it
   leaves the two resolving differently.
4. **Whether the receipt-spec change in ucantone is wire-compatible** with
   images already in the field. Nothing in the bump answers this; only a skew
   test does.

## Reading

The mechanical cost of a five-week deferral was small — minutes, no source
changes — for this particular delta, which contained no breaking Go API change
that forge's code exercised. The expensive part was not the bump but the
*state* the deferral produced: a service monorepo whose published images run a
library commit that no reviewer has approved and that its own library repo has
not merged. The two-step rollout did not cost a lot of engineering time here;
it cost the invariant that `main` depends on `main`.

Caveat: one delta, one repo, one day. Experiment E measures how typical this
delta was.
