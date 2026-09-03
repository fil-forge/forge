# Coordinated changes in the live polyrepo, August 2026 — case study

Evidence for the plan's central question: how often does a library change force a
service-side change, and what does the "two-step rollout" (library PR, then a pin
bump PR in each consumer) cost in practice. Source: `git log`/`git show` in the
read-only checkouts `/home/user/fil-forge/{piri,hilt,sprue,ingot,smelt,delegator,piri-signing-service}`
(shallow since 2026-06-15), `/home/user/libforge`, `/home/user/ucantone`,
`/home/user/guppy`, `/home/user/indexing-service`, plus PR metadata for
`fil-forge/libforge` and `fil-forge/ucantone` via the GitHub API (read only).
Dates are the commit dates shown by `git log --date=iso-strict`; "lag" is measured
from the library commit's merge to `main`. Nothing was written to any checkout.

Reproduce the pin history behind every table with (per repo):

```
git log --date=short --format='@@@ %h %ad %an | %s' -p -- go.mod \
 | awk '/^@@@ /{hdr=$0;shown=0;next} /^[-+][[:space:]]+github.com\/fil-forge\/(libforge|ucantone|hilt|smelt|swarf)[[:space:]]/{if(!shown){print hdr;shown=1}print}'
```

Author notes: `ash` and `Alan Shaw` are the same person (GitHub `alanshaw`);
commits authored `Claude` are agent-authored on a human's behalf; `dependabot[bot]`
is the bot. **No dependabot commit in any repo ever changed a `fil-forge/*` pin** —
every pin bump in the tables below is a human (or human-driven agent) commit.

---

## Case 1 — tenant DID in the S3 authorize response (libforge #64 ↔ hilt #50 ↔ ingot #112)

Wire change: `commands/s3/request/types.go` adds `Tenant did.DID` (`cborgen:"tenant"`,
not `omitempty`) to `AuthorizeOK`, the result of `/s3/request/authorize`, which hilt
executes and ingot invokes. PR #64 body: "Ingot needs to know the tenant ID so that it
can retrieve the key agreement key and encrypt the CEK for the COSE encryption header."

| date (UTC) | repo | sha | what | lag from libforge merge |
|---|---|---|---|---|
| 2026-08-24 | ingot | `fc5be24` (Petra, #15) | FEE wrap material on `blob_encryption_params` (FIL-480) — motivating feature | -4 d |
| 2026-08-27 | hilt | `059df06` (Petra, #14) | per-tenant X25519 wrap-key registry; also bumps ucantone `3a20cd5`→`25cf834` — prerequisite: the tenant wrap key hilt will hand out | -1 d |
| 2026-08-27 18:01 | libforge | `0f0c46f` (PR #64 c1) | `feat: add tenant DID to authorization response` | PR opened 18:03 |
| 2026-08-27 18:08 | libforge | `c9252ac` (PR #64 head, "chore: regen") | PR head; **not a commit on `main`** | |
| 2026-08-28 11:49 | hilt | `cb1bc0b` (ash, #50) | `feat: add tenant DID to authorize response`: `pkg/rpc/authorize.go` +1 line (`Tenant: authz.Tenant.ID`), test +1, `go.mod` libforge `3e6895b`→**`c9252ac`** | **-26 min** (merged before the library PR) |
| 2026-08-28 12:15 | libforge | `2585ed1` (squash merge of #64) | on `main`; module tree **byte-identical** to `c9252ac` (see below) | 0 |
| 2026-08-28 12:32 | ingot | `1365193` (ash, #112) | `feat: add tenant key to encryption envelope`: 31 files; `go.mod` libforge `3e6895b`→`c9252ac`, hilt `0928688`→`cb1bc0b`; `iam/service.go` now **requires** `ok.Tenant` | +17 min (+43 min after hilt) |
| 2026-08-28 | ingot | `0ed4bd8` (#107), `a664171` (#110) | encrypted write; refuse to start without hilt proofs — same-day follow-ups | same day |

Verification that the PR-head pin equals the merge commit: both pseudo-versions were
downloaded from the module proxy into `/root/go/pkg/mod` and `diff -rq` over the two
trees printed nothing (`IDENTICAL TREES`); sums `h1:2na0I6…` vs `h1:+tzowz…` differ only
because the version string is part of the zip. So hilt and ingot pinning
`v0.0.0-20260827180828-c9252ac89b0e` is benign today, but it is exactly the pattern
that produced forge's `928cf2a` pin (F2): a consumer pinned to a PR head, with no
guarantee the head ever reaches `main`.

Who else sees the new wire shape? Importers of `libforge/commands/s3/request` across
all live repos: `hilt/pkg/{client,rpc,rpc/service/bucket}` (6 files),
`ingot/iam/service*.go` (3 files), `smelt/pkg/stack/proofs.go` (uses only the
`s3req.Authorize.Command` constant, never decodes `AuthorizeOK`). Consumers still on
older libforge — piri, sprue, piri-signing-service, guppy, indexing-service (`3e6895b`,
2026-08-07), smelt (`5e299c4`, 2026-07-27), delegator (`7fc3b2c`, 2026-07-24) — do not
touch this type, so the shape divergence is confined to the hilt↔ingot pair, which
moved together within 43 minutes.

Mixed-version behaviour, from the code (not observed at runtime):

- old ingot (e.g. forge monorepo at `f60dd59`, libforge `928cf2a`, no `Tenant`) ←
  new hilt: cbor-gen's map decoder ignores unknown keys (`// Field doesn't exist on
  this type, so ignore it` + `cbg.ScanForLinks`, present in
  `commands/s3/request/cbor_gen.go` at both `3e6895b` and `main`) → works.
- new ingot (`1365193`+) ← old hilt (forge monorepo hilt, or any hilt < `cb1bc0b`):
  `tenant` absent → zero `did.DID` → `ingot/iam/service.go:204`
  `if !ok.Tenant.Defined() { return ..., fmt.Errorf("hilt/iam: no tenant for %s in authorize result", ...) }`
  → **every authorize fails**. This is a hard forward-compatibility break introduced
  deliberately, which is why hilt had to ship first (and did, by 43 min).

Side observation: `2585ed1` also moved libforge's *own* ucantone pin from `7985ec0`
(2026-06-19) to `25cf834` (2026-08-27) — libforge's own CI had been testing against a
69-day-old ucantone while all its consumers were on 2026-08-17 or later. MVS makes
this harmless for consumers, but the library was not testing what the services ran.
The 18 `json_gen.go` files regenerated in `2585ed1` are plausibly the result of that
bump (ucantone #52 "adopt CBOR/JSON codegen method from libforge", or dag-json-gen
0.0.8 from dependabot #55) — **speculation, not verified**.

## Case 2 — attestation idempotency (libforge #51)

`f54066c` (2026-08-03, ash): `attestation/signer.go` +7 lines — `Sign()` now uses
`invocation.WithNoNonce()` and `WithNoExpiration()` so an attestation for the same
input is the same invocation (same CID). PR opened 2026-07-31, merged 2026-08-03 15:42Z.
Not a schema change, but it changes the bytes/CID of every `/ucan/attest/proof`
invocation on the wire.

Consumers of `libforge/attestation` (grep over all checkouts): **sprue** (20 files;
non-test: `pkg/lib/ucan_server/email_auth.go`, `pkg/service/handlers/{access_confirm,access_request,provider_add}.go`,
`pkg/service/service.go`), **guppy** (`cmd/login.go`, `pkg/didmailto/didmailto.go`),
ingot (`forgeclient/accounts_test.go`, test only). Not piri, hilt, smelt, delegator,
indexing-service, piri-signing-service.

| date | repo | sha | what | lag from `f54066c` |
|---|---|---|---|---|
| 2026-08-03 | libforge | `f54066c` | fix merged | 0 |
| 2026-08-06 | sprue | `0fa0743` (ash, #50) | `fix: blob release cause` — bumps libforge `5e299c4`→**`850148f`** (2026-08-01), i.e. **two commits short of the fix**; the fix was not picked up | +3 d, missed |
| 2026-08-20 | sprue | `b403a68` (ash, #70) | `chore: upgrade forge deps` → `3e6895b`; **no Go files changed** | **+17 d** |
| 2026-08-21 | guppy | `d74fd06` (ash, #47) | `chore: update deps` → `3e6895b` | +18 d |
| 2026-08-21 | ingot | `de3ca05` (ash, #70) | inside the revocation feature PR → `3e6895b` (test-only importer) | +18 d |

No consumer needed a source change; the cost of the two-step here was purely latency
(17–18 days for the only production consumer) and the near-miss on 08-06, where a
bump made three days after the fix landed on an older commit. Thematic neighbour, not
a dependency: piri `b9309b6` 2026-08-04 (ash) "fix: `/pdp/accept` task link mismatch by
invoking with no nonce (#54)" — same author, next day, same `WithNoNonce` idea, no pin
change (**speculation** that it is the same investigation).

## Case 3 — ucantone receipt spec alignment (#49) and issued-at option (#50), 2026-08-17

`bfc05d9` (#49, 10:07Z): receipts no longer set `aud` and `receipt.Decode` **rejects**
invocations that carry an audience; receipts default to `exp: null`; new
`receipt.WithExpiration`, `Expiration()`. `e926fd5` (#50, 15:37Z): `iat` only set when
requested; fixes `WithReceiptTimestamps`. Same day `3a20cd5` (#43, 17:06Z): validator
fix for empty `capabilityInvocation`. Consumers bumped straight to `3a20cd5`, which
contains all three. Wire-visible: every receipt issued by a new executor lacks `aud`;
every receipt issued by an old executor carries `aud` and is **rejected** by a new
`receipt.Decode`.

| date | repo | sha | what | lag from `bfc05d9` |
|---|---|---|---|---|
| 2026-08-17 | ucantone | `bfc05d9`, `e926fd5`, `3a20cd5` | three fixes merged | 0 |
| 2026-08-20 | hilt | `80f80f3` (ash, #42) | `chore: upgrade forge deps` — libforge `7fc3b2c`→`3e6895b`, ucantone `79141c5`→`3a20cd5`; **no Go files** | +3 d |
| 2026-08-20 | sprue | `b403a68` (ash, #70) | same; ucantone `2662bdd`→`3a20cd5`; **no Go files** | +3 d |
| 2026-08-20 | piri | `b3b91be` (ash, #86) | `chore: upgrade forge deps and fix builds on darwin` — ucantone `ccb7705`→`3a20cd5`; only `Makefile`, `.goreleaser.yaml`, release workflow | +3 d |
| 2026-08-20 | piri-signing-service | `bacd87b` (ash, #18) | libforge `eb26d87`(06-19)→`3e6895b`, ucantone `7985ec0`(06-19)→`3a20cd5` | +3 d |
| 2026-08-20 | indexing-service | `9eb6204` (ash, #51) | libforge `2b55dbc`→`3e6895b`, ucantone `a8f24fe`→`3a20cd5` | +3 d |
| 2026-08-21 | guppy | `d74fd06` (ash, #47) | libforge `2b55dbc`→`3e6895b`, ucantone `7985ec0`→`3a20cd5` | +4 d |
| 2026-08-21 | ingot | `de3ca05` (ash, #70) | inside feature PR: ucantone `ccb7705`→`3a20cd5` (+ libforge, hilt, smelt, indexing-service, swarf) | +4 d |
| 2026-08-27 | delegator | `b5552f1` (ash, #33) | ucantone `79141c5`→`25cf834` (skips `3a20cd5`); libforge left at `7fc3b2c` | +10 d |
| 2026-08-28 | libforge | `2585ed1` (#64) | ucantone `7985ec0`→`25cf834` | +11 d |
| never | smelt | — | still ucantone `79141c5` (2026-07-06), libforge `5e299c4` (2026-07-27) | — |

Seven repos were bumped by one person within two days (08-20/21) — the polyrepo's
equivalent of a coordinated release, executed as seven PRs. None of the pure-bump
PRs changed Go code, so the ucantone receipt change was not *source*-breaking for
any service. Between 08-17 and 08-20/21 a new-ucantone consumer decoding a receipt
from an old-ucantone executor would have hit the `aud` rejection; whether that
happened in any deployed environment is not recoverable from git (**unknown, flagged**).

## Case 4 — how ingot gets hilt's S3 client in the live polyrepo

Live `ingot/go.mod` (line 12): `github.com/fil-forge/hilt v0.0.1-0.20260828114936-cb1bc0b84e7b` —
a service importing another service's module. Pseudo-version base `v0.0.1-0` comes
from hilt's single tag `v0.0.0` at `6758d7b` (2026-07-10, "feat: add build info (#30)").
Packages imported (files): `hilt/pkg/client` 5, `hilt/pkg/sigv4` 3,
`hilt/pkg/rpc/service/auth` 2, `hilt/pkg/s3perm` 1, `hilt/pkg/rpc/service/bucket` 1.
Live ingot's `go.mod` history contains **no `replace` directive, ever**.

| date | ingot sha | hilt pin (hilt commit date) | trigger / lag |
|---|---|---|---|
| 2026-07-21 | `02af9fc` (ash, #35) `feat: add Hilt client and wire with FX` | `7ddddf0` (07-16) | first import; PR body: "moved `hiltclient` into a `hilt` package"; depends on versitygw fork PR #1 and piri #32 |
| 2026-07-28 | `a7684d7` (Forrest, #44) | `ba71f84` (07-24) | +4 d |
| 2026-08-06 | `433b242` (ash, #41) `chore: update hilt client` — `module.go` 2 lines | `c6afc4f` (08-06, hilt #32 `refactor(client): optional base proofs`, 12 files) | **0 d**, same author |
| 2026-08-21 | `de3ca05` (ash, #70) | `7792ca5` (08-20) | +1 d |
| 2026-08-21 | `3e9b741` (ash, #91) `chore: update deps` | `0928688` (08-21) | 0 d |
| 2026-08-28 | `1365193` (ash, #112) | `cb1bc0b` (08-28) | 0 d (43 min) — Case 1 |

Other service→service module edges in the live repos (`go.mod` direct requires):
ingot→smelt, hilt→smelt (both import only `smelt/pkg/stack`, 6 files, the test stack),
hilt→swarf and ingot→swarf (`swarf/pkg/client` 10 files, `pkg/api` 2, `pkg/store` 1 —
the revocation service), piri→delegator. In August hilt bumped smelt 4× and ingot
bumped smelt 5×.

The monorepo side: forge `ingot` at `f60dd59` imports the same code from libforge PR
#52's paths (`libforge/client/hilt` 4 files, `libforge/sigv4` 3, `libforge/s3perm` 1)
pinned at `928cf2a`. Before forge `8d55284` (Forrest, 2026-07-31, "Refactor/drop
replaces (#3)") monorepo ingot carried `replace github.com/fil-forge/forge/hilt => ../hilt`
and `=> ../smelt`. PR #52's body ("forcing ingot to carry a replace directive that
breaks external consumers of the ingot library") therefore describes the monorepo
ingot, not live ingot. PR #52 (opened 2026-07-31, draft, `mergeable_state: dirty`,
CHANGES_REQUESTED 2026-08-19 — F2) never merged, so libforge never published the
client; live hilt kept `pkg/client` and refactored it (`c6afc4f`, 08-06), and libforge
#64 changed `commands/s3/request` on `main`, which #52 also touches. Two copies of the
hilt client now exist: live `hilt/pkg/client` (moving) and libforge PR #52
`client/hilt` (frozen at 07-31). Their divergence was not measured here.

## Case 5 — other August cross-references

| # | library / origin commit | service commits | lag | kind |
|---|---|---|---|---|
| 5a | libforge `b13386b` 2026-07-30 (ash, #50) `feat!: add cause to blob release arguments` — new required `Cause cid.Cid` (`cborgen:"cause"`, no omitempty) in `ReleaseArguments`/`AbortArguments` | piri `8509dfa` 2026-08-06 (ash, #68) `feat: validate release cause` — pins `b13386b`, +76 lines `pkg/ucanhandlers/blob/release.go`, missing/unknown cause → `ErrUnknownCause`; sprue `0fa0743` 2026-08-06 (ash, #50) `fix: blob release cause` — pins `850148f`, sets cause in `blob_remove.go` + `piriclient` | 7 d, both ends same day | wire-visible, coordinated both sides by one author. smelt `f0b69e6` 07-28 and ingot `a0250c3` 08-05 pinned `5e299c4` (pre-`cause`); smelt is still there |
| 5b | hilt `c6afc4f` 2026-08-06 (#32) client refactor | ingot `433b242` 2026-08-06 (#41) | 0 d | service→service (Case 4) |
| 5c | libforge `aba2bd2` 2026-07-20 (#47) `/ucan/revoke` command | hilt `ef14123` 08-19 (#38/#39) revoke on key/bucket delete, adds swarf; smelt `1441914` 08-20 (#27) revocation service; ingot `de3ca05` 08-21 (#70) revocations clear caches, adds swarf, bumps hilt+smelt | 30–32 d from libforge; 0–2 d between services | three services + a new module in 3 days, one author |
| 5d | ucantone `dacea7a` 2026-08-27 13:44Z (#53) validator: skip unsupported verification methods | same day: piri `b9ed572` (#96), sprue `f99dda9` (#78), delegator `b5552f1` (#33), hilt `059df06` (Petra, #14); next day: ingot `0ed4bd8` (#107), libforge `2585ed1` (#64) — all → `25cf834` | 0–1 d | 6-repo sweep; not swept: guppy, indexing-service, piri-signing-service (`3a20cd5`), smelt (`79141c5`) |
| 5e | smelt `cdb15d0` 08-25 (#34) OpenBao vault; `a1f34a7` 08-26 (#36) OpenBao for ingot; `efe09f1`/`640b11b` 08-28 (#39/#38) ingot did:web / did:plc config | hilt `c6df91d` 08-25 (#46) switch to OpenBao, `ef7fc83` 08-26 (#47) bump smelt; ingot `b2e2094` 08-28 (#109) bump smelt, `30a7ff4` (#101) OpenBao provider, `8fed649` (#113) did:web identity → smelt `d081722` | 0–1 d | test-stack ↔ service coupling |
| 5f | smelt `31223a2` 08-21 (#30) `HiltEndpoint`/`HiltPartnerKey` in Stack | hilt `0928688` 08-21 (#44) use Smelt for integration tests; hilt `cf9343c` 08-21 (#45) bump smelt → `defc6a7` | 0 d | three PRs, one day |
| 5g | libforge `3e6895b` 2026-08-07 (dependabot #60; last human content `f54066c` 08-03) + ucantone `3a20cd5` 08-17 | the 08-20/21 sweep: hilt #42, sprue #70, piri #86, piri-signing-service #18, indexing-service #51, guppy #47, ingot #70 | 13 d / 3 d | Case 3 |
| 5h | libforge `7fc3b2c` 2026-07-24 (#48) DID verification method fix | same day, authored `Claude`: hilt `b9ad649`, sprue `2f89e90`, delegator `626fc76` (all → `7fc3b2c`) | 0 d | July, agent-driven three-repo sweep (pattern evidence only) |

Pure dependency-bump PRs merged in August across the nine consumer repos (subject is
"chore: upgrade/update deps" or equivalent, no feature): hilt #42, #45, #47; sprue #70,
#78; piri #86, #96; piri-signing-service #18; ingot #41, #91, #109; delegator #33;
guppy #47; indexing-service #51 — **14**. Bumps folded into feature PRs: ingot #70,
#107, #112, #113; hilt #14, #50; sprue #50; piri #68 — 8. Total human `fil-forge/*`
pin-bump commits in August: 22 across 9 repos; dependabot: 0.

Pins never moved in August: smelt (libforge `5e299c4` 07-27, ucantone `79141c5` 07-06),
delegator's libforge (`7fc3b2c` 07-24). Both are consumed as modules by other services
(hilt/ingot→smelt, piri→delegator), so MVS lifts their transitive versions in the
importers and the staleness is invisible there; it only affects their own builds/tests.

---

## What this says about the two-step rollout (evidence, with speculation flagged)

1. **Every wire-visible libforge change in the window was shipped as a same-author,
   same-day, multi-repo change.** `blob/release` `cause` (libforge 07-30 → piri + sprue
   both 08-06, same author, both ends of the wire); tenant DID (libforge PR 08-27 →
   hilt 08-28 11:49Z → libforge merge 12:15Z → ingot 12:32Z). The "explicit, tested PR
   in the consuming repo" exists, but it is written by the library author as part of
   the same piece of work, not by a consumer team later. For #64 the consumer PR
   merged **before** the library PR, pinning the PR-head pseudo-version `c9252ac`, which
   is not on libforge `main` — the same shape of pin that left forge stranded on
   `928cf2a` (F2). Here it is harmless only because the squash merge produced an
   identical tree (verified by module-zip diff).

2. **Non-wire library fixes propagate on the sweep schedule, not on their own.** The
   attestation fix (08-03) reached its only production consumer (sprue) on 08-20,
   17 days later, via a generic "upgrade forge deps" PR; a sprue bump on 08-06 landed
   two commits short of it. Nothing in the consumer's code told anyone the fix was
   missing. Cost: latency and a silent near-miss; zero lines of adaptation.

3. **The sweeps are the real mechanism.** Two observed: 08-20/21 (7 repos → libforge
   `3e6895b` + ucantone `3a20cd5`) and 08-27/28 (6 repos → ucantone `25cf834`). Each
   sweep is one PR per repo by one person (ash), with review/CI per PR. 14 pure-bump
   PRs in August, none of which changed Go source in hilt, sprue, delegator, guppy,
   indexing-service or piri-signing-service — i.e. the library changes in August were
   API-compatible for every service; the two-step's cost was PR count and lag, not
   code. Speculation: the sweeps coincide with the 08-17 receipt change and the 08-27
   validator fix, suggesting they were triggered by specific library fixes rather
   than by a cadence.

4. **Mixed-version windows are real and unmeasured.** ucantone #49 makes new decoders
   reject receipts from old executors; consumers were split across old/new ucantone
   from 08-17 until 08-20/21, and smelt is still on the 07-06 ucantone. Whether any
   environment ran mixed versions is not in git (**unknown**). For the tenant DID, the
   new ingot hard-fails against an old hilt by design (`iam/service.go:204`), which is
   why the order hilt-then-ingot mattered and was honoured by 43 minutes.

5. **Service→service module dependencies double the two-step.** ingot bumped its hilt
   pin 5 times in 5 weeks (3 of them the same day as the hilt change) and smelt 5
   times; hilt bumped smelt 4 times. These are the same bump PRs the RFC counts as
   "the normal Go workflow", but between services rather than library→service, and
   they exist precisely because the shared code (hilt's S3 client) was never moved to
   libforge — PR #52 tried and stalled on review; the live repos routed around it by
   importing the hilt module directly.

6. **Some pins simply do not move.** smelt and delegator went the whole month without a
   libforge bump; nobody noticed because their importers' MVS resolution hides it. In
   a monorepo with one pin this class of drift cannot exist; in the polyrepo it is
   invisible until a build of the stale module fails.

Net: in August the two-step was exercised for 2 wire-visible libforge changes, 3
wire-visible ucantone changes (#49, #50, #53) and ~3 service→service contract changes,
all handled by one person doing same-day fan-out. It worked, but only because a single
author owned both ends of every change; the process left three non-main pins
(`c9252ac` ×2 in hilt/ingot; `928cf2a` ×5 in forge) and two stale modules behind.
