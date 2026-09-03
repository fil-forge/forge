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

`smelt/pkg/workspace.Detect` rebuilt all services only when `libforge` was in
`go.work`'s use-list. `commands` and `internal` are the same kind of
dependency; they now count as shared dirs. Consequence: workspace mode always
rebuilds every service (they are always listed), which is the monorepo's
HEAD-vs-HEAD contract.

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
`forge/commands` is an internal directory or a library with a version.
