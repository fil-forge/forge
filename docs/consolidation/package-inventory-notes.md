# Package inventory — reading notes and proposed destinations

Companion to the generated `package-inventory.md` / `package-inventory.json`
(Experiment B). The generated file is the evidence; this file is the argument.
Re-run the tool before trusting any number here:

```
cd tools/inventory && GOWORK=off go run . \
  -subject /home/user/libforge -subject /home/user/guppy -subject /home/user/ucantone \
  -consumer <forge main checkout>/hilt,…/ingot,…/piri,…/smelt,…/sprue,…/smelt/systems/stress-tester \
  -consumer /home/user/indexing-service \
  -consumer /home/user/fil-forge/piri,…/hilt,…/sprue,…/ingot,…/smelt,…/delegator,…/piri-signing-service \
  -out ../../docs/consolidation/package-inventory.md -json ../../docs/consolidation/package-inventory.json
```

## What was scanned, and why those checkouts

| role | checkout | why |
|---|---|---|
| subject | libforge `f4b13f7` (= `main` `2585ed1` + PR #52 rebased) | the only libforge state that contains every package forge imports (S1 in `findings.md`); `main` alone lacks four of them |
| subject | libforge `928cf2a` (separate run, `-no-reach`) | the commit the plan's reference numbers were taken at, for the cross-check below |
| subject | guppy `e87812b`, ucantone `8d7eb73` | the other two candidates the plan names |
| consumer | forge **`main` `f60dd59`** (a clean worktree), six modules | the POC branch rewrites `libforge/commands` imports to `forge/commands`, so it cannot be used to count libforge importers; `main` is what the decision is about |
| consumer | live `fil-forge/{piri,hilt,sprue,ingot,smelt,delegator,piri-signing-service}` | the services as they are actually developed (S2); shallow clones, HEADs of 2026-08-28..29 |
| consumer | indexing-service `ba73105`, guppy | the other first-party Go consumers |

Two runs of the tool over the same inputs produce byte-identical output
(checked with `cmp`). Reachability used `go list` in every module; no module
fell back to the direct-import approximation (the report's warning list is
empty).

Column definitions that matter for reading the tables:

- **external modules** is the number of distinct Go *modules* in the package's
  full transitive dependency closure, standard library excluded, own module
  excluded. This is the number that forces module boundaries: whatever module
  imports the package inherits all of them into its build list.
- **proto** means the closure reaches `libforge/commands/**` or
  `libforge/blobindex/**` from outside those trees.
- **reach** `shipped` means some non-example, non-codegen `package main` in a
  scanned module links the package; `library-only` means non-test code imports
  it but no scanned binary reaches it; `test-only` and `unreferenced` as named.

## Cross-check against the plan's reference measurements

Every LOC figure the plan quotes for libforge at `928cf2a` reproduces exactly,
both by the tool (`-subject` the `928cf2a` worktree) and independently by
`find … -name '*.go' ! -name '*_test.go' | xargs cat | wc -l` per directory:

| package | plan | tool @ `928cf2a` | `wc -l` | verdict |
|---|---:|---:|---:|---|
| `commands/` | 27,106 | 27,106 | 27,106 | reproduced (of which **3,442 hand-written**; the rest is `cbor_gen.go`/`json_gen.go`) |
| `blobindex` | 1,580 | 1,580 | 1,580 | reproduced (334 hand-written) |
| `sigv4` | 848 | 848 | 848 | reproduced |
| `ucan` | 588 | 588 | 588 | reproduced |
| `attestation` | 368 | 368 | 368 | reproduced (375 at `main`: the idempotency fix, #51) |
| `jobqueue` | 265 | 265 | 265 | reproduced |
| `receipt` | 229 | 229 | 229 | reproduced |
| `client` | 196 | 196 | 196 | reproduced |
| `identity` | 191 | 191 | 191 | reproduced |
| `piece` | 176 | 176 | 176 | reproduced |
| `testutil` | 83 | 83 | 83 | reproduced |
| `bytemap` | 80 | 80 | 80 | reproduced |
| `s3perm` | 77 | 77 | 77 | reproduced |
| `digestutil` | 25 | 25 | 25 | reproduced |
| total | ~31,000 | 31,812 | 31,812 | reproduced |

The plan's structural claims, checked against forge `main`:

| claim | measured | verdict |
|---|---|---|
| only `attestation` imports `libforge/commands` | `attestation` (via `commands/ucan/attest`), plus — on PR #52 — `client/hilt` and `s3perm` | true for `main`; PR #52 adds two more |
| `commands/` imports none of the other libforge packages | true for non-test code; one test file (`commands/blob/abort_test.go`) imports `testutil` | reproduced |
| `guppy/pkg/client` closure: 4 internal, ~15 external | **3 internal / 45 external modules** (`pkg/client/locator`, `pkg/tokenstore`, `internal/ctxutil`); 15 is the count of *direct* external imports (also measured: 15) | the plan counted direct imports; the transitive figure, which is what a consumer inherits, is three times larger |
| `guppy/cmd` closure: 57 internal, ~101 external | 54 internal / **159** external; the whole module (root package) 171 external; `go.mod` has 228 `require` lines (168 indirect) | same direct-vs-transitive difference; the ordering and the conclusion (`pkg/client` must be its own module to be importable) stand, the gap is larger than planned |
| `libforge/ucan` imported by 27 files across hilt and sprue | 27 files: hilt 5, **ingot 9**, sprue 8 (22 non-test + 5 test) | count reproduced; ingot was omitted |
| `libforge/receipt` imported by 4 files, all in ingot | forge: 4 files, all ingot; **guppy: 5 files** | reproduced for forge; guppy is a consumer outside forge |
| `libforge/attestation` has exactly one non-test importer (`sprue/…/access_confirm.go`) | **two**: `access_confirm.go` and `sprue/pkg/service/service.go`; plus 5 files import `attestation/didmailto` and 13 test files | not reproduced (off by one) |

## What the inventory adds that the plan did not have

1. **`commands/**` is 87% generated code.** 27,122 LOC, 3,446 hand-written.
   The "protocol package" that dominates libforge by volume is mostly
   `cbor-gen`/`dag-json-gen` output; the hand-written wire definitions are
   about the size of `blobindex` + `sigv4` + `ucan` together. Arguments from
   volume overstate it by 8×.
2. **`commands/debug` is unreferenced** — no consumer, no test importer, in any
   of the fourteen scanned modules. Dead protocol surface.
3. **External first-party consumers of the wire packages are concrete**, not
   hypothetical: `piri-signing-service` imports `commands/pdp/sign` (5 files),
   `commands/access` (2); `delegator` imports `commands/blob`, `blob/replica`,
   `claim` (2), `pdp`, `space/egress` (2); `indexing-service` imports `assert`
   (8), `claim` (2), `content` (2), `blobindex` (16); `guppy` imports twelve
   `commands/*` packages and `blobindex` (8). This is the same fact the
   compiler surfaced in Experiment C (`findings.md` S7), now with counts.
4. **`blobindex` is the most shared non-command package**: indexing-service 16
   files, guppy 8, ingot 4–5. It is wire-visible (IPLD-serialised, generated
   codecs) and every network participant that touches indexes decodes it.
5. **Three packages have a zero-module closure**: `sigv4` (848 LOC, standard
   library only), `bytemap`, `jobqueue`. They are free to place anywhere; the
   module boundary question does not arise for them.
6. **`testutil` is `library-only`**: imported by non-test code in hilt, sprue,
   guppy and indexing-service (test helpers that are not `_test.go` files) but
   linked by no shipped binary. Its 31 guppy test-file importers and 25 in
   piri make it the most-imported package in the repo by test count.
7. **ucantone's `init()` registrations are exactly what the plan expected**:
   `multikey/{ed25519,secp256k1,mldsa44}/verifier` call `multikey.Register`,
   `varsig` and `varsig/algorithm/{mldsa,nonstandard}` call
   `RegisterAlgorithmScheme`. `mldsa44/verifier` and `varsig/algorithm/mldsa`
   are `test-only` — no scanned binary links the post-quantum path yet — and
   `varsig/algorithm/nonstandard` is shipped only through libforge's
   `attestation`. libforge's `attestation` itself registers
   `varsig.RegisterAlgorithmScheme` in `init()` (the `0x300001` code), which
   is the link-time property the plan's question 9 worries about: only
   binaries that import `attestation` (sprue; guppy via `didmailto`) can decode
   attested signatures.
8. **Reachability differs between the frozen monorepo and the live services**
   in one telling way: live ingot imports `hilt/pkg/client`, `hilt/pkg/sigv4`,
   `hilt/pkg/s3perm` (S2), so the libforge copies of those packages have
   *zero* live consumers — their only importers are forge `main`'s hilt and
   ingot, pinned to PR #52.

## Proposed destinations (argued, not transcribed)

The plan's starting classification, re-examined against the columns. Where I
agree, the reason is the measured one; where I differ, it is marked.

| package | LOC (hand) | consumers outside forge | ext. modules | proposed | reason |
|---|---:|---|---:|---|---|
| `commands/**` except `ucan/attest`, `debug` | 26,600 (3,300) | guppy, indexing-service, delegator, piri-signing-service | 16–21 | **published module**, home undecided | The plan says "monorepo — the wire contract". Measured: four first-party modules outside forge compile against it, and two of them expose its types in their own APIs (S7). Wherever it lives it is a *library with consumers*, so the real choice is "libforge-as-is" vs "`forge/commands` published from the monorepo" — both need versions and a compatibility gate. The in-repo move is mechanically fine (Experiment C); the cost is that those four modules must follow, which makes `forge/commands` publishable by construction. **Differs from the plan**: not "monorepo-internal", because it never was internal. |
| `commands/ucan/attest` | 259 (22) | guppy, ingot | 19 | with `attestation` | Agrees with the plan. Its only non-attestation importers are the two clients that call `/ucan/attest/proof`; `attestation` needs it and nothing else in `commands` does. |
| `commands/debug` | 260 (17) | none | 19 | **delete** (or keep with `commands`, flagged dead) | Unreferenced everywhere scanned. Not in the plan's table at all. |
| `blobindex` (+ `datamodel`) | 1,543 (297) | indexing-service 16, guppy 8, live ingot 5 | 16–17 | **stays a library**, wherever `commands` goes | The plan says monorepo. Its heaviest consumer is indexing-service, a service the plan's own recorded vote keeps *out* of the monorepo. Moving it in-repo while indexing-service stays out recreates exactly the two-step the move is meant to remove. Travels with `commands`; same publishability requirement. **Differs from the plan.** |
| `ucan` (`ucanlib`) | 176 (176) + `retrieval` 358 | delegator, guppy, indexing-service, live hilt/ingot/sprue | 18 | `ucantone` for the proof-chain part; `ucan/retrieval` with `commands` | Agrees with the plan on the split. Measured support: `ucan` has six consumer modules, two of which (delegator, indexing-service) are not services in forge, so it is a genuine library; its closure is ucantone's plus nothing Forge-specific. `retrieval` (the `X-UCAN-Container` transport) has piri, ingot, guppy and indexing-service as consumers — again not forge-internal. |
| `attestation` + `didmailto` | 375 (375) | guppy (`didmailto` 2), live sprue | 19 | extension module (or `ucantone` subpackage — the plan's open question 7) | Agrees. `init()` registration of varsig `0x300001` means the package's placement decides which binaries can verify attested signatures; only sprue and guppy link it today. |
| `receipt` | 229 | guppy 5, ingot 4 | 23 | **open**, leaning library | The plan's open question stands. New fact: guppy is a consumer, so "4 files all in ingot" undersells it. Highest external-module count of the small packages (23: it drags the HTTP client stack). |
| `client/hilt`, `s3perm`, `sigv4`, `ucan/zapucan` | 1,175 | none live (live ingot uses hilt's copies) | 21 / 19 / **0** / 20 | monorepo `internal/` (done in Experiment C) or back into hilt | These exist only on PR #52 and have zero consumers outside forge `main`; the live polyrepo kept them in hilt and lets ingot import hilt. Either home works; libforge is the one place they demonstrably do not belong. |
| `jobqueue`, `piece`, `identity`, `bytemap`, `digestutil` | 737 | `identity`: delegator 4, guppy 4, indexing-service 4, piri-signing-service 2; `bytemap`: guppy 4, indexing-service 3; `digestutil`: guppy 14, indexing-service 4; `piece`: guppy 1; `jobqueue`: indexing-service 1 | 0–13 | **not `internal/`** for `identity`, `bytemap`, `digestutil`; `internal/` fine for `jobqueue`, `piece` | The plan puts all five in monorepo `internal/`. Measured: `identity` is imported by every first-party module scanned (14 of 14 modules that use libforge), `digestutil` by guppy 14 files and indexing-service 4, `bytemap` by guppy and indexing-service. A compiler-enforced `internal/` module would break all three consumers. **Differs from the plan** for those three; agrees for `jobqueue` (one external consumer, indexing-service, 1 file) and `piece` (guppy, 1 file) if those two are willing to copy 265 + 176 lines. |
| `testutil` | 83 | guppy 7+31, hilt, indexing-service, sprue | 20 | library (a conformance-fixtures package, as the plan's alternative suggests) | `library-only` and widely imported by tests; `internal/` would strand guppy's and indexing-service's test suites. |
| ucantone, indexing-service, versitygw fork | — | — | — | stay out | Agrees with the plan and the recorded vote. ucantone has 82 packages and 26,670 LOC of which 12,728 hand-written; nothing in the inventory suggests it is Forge-specific. |
| guppy `pkg/client` (+ `locator`, `tokenstore`, `internal/ctxutil`) | 2,700 | forge's ingot carries a copy (`forgeclient-divergence.md`) | 45 | own module (the consolidation plan's `forgeclient`), extracted from **live ingot**, not guppy | Agrees with the plan on the module boundary: 45 transitive modules vs 171 for the guppy module as a whole is the measured reason. Differs on the source: guppy's client has had no commits since 2026-06-19 while ingot's copy kept moving. |

## Things the tool cannot tell you

- Whether any **partner** compiles against these packages. Only first-party
  modules were scanned; the plan's human question 1 stands.
- Whether a wire type's *shape* changed — that is Experiment E's job, from
  history, not a snapshot.
- Reachability is per scanned module. A binary built outside these checkouts
  (an operator's fork, a partner's tool) is invisible.
