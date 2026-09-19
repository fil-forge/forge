# Wins

**For reporting out at the end.** Things the consolidation work found or fixed
that were broken before it started — most of them in the polyrepo itself, not
in the monorepo being built. Each one says plainly whether it is **solved** or
**not solved**, because a list where everything is a win is not worth reading.

Kept current as things land. The sibling pages are for doing the work:
[[Needs Human Work]] is what is waiting on a person, [[Consolidation Findings]]
is the running log of blind spots and latent issues.

**Updated 2026-09-19 17:45Z.**

## Why these count

None of these were on the plan. Every one surfaced because consolidating
forces the services to be built, run and tested *together*, and most of them
are invisible from inside any single repository. That is itself the finding
worth reporting: several of these had been live in production paths for weeks
and nothing in the polyrepo was positioned to notice.

---

## Solved

### 1. A live wire break between four services

**`/ucan/conclude` silently stopped working across a one-deploy skew.**
`libforge` renamed the argument and its CBOR key on 2026-09-17 — `Receipt
cid.Cid`/`receipt` became `Receipts []cid.Cid`/`receipts`. sprue, ingot and
guppy each adopted it the same day.

**A renamed CBOR key does not fail to decode. It decodes to nothing.** The old
key is simply absent, `Receipts` comes back empty, and a server one commit
ahead of its client concludes nothing, never calls `/blob/accept`, and the
client polls for a receipt that will never exist:

```
polling accept receipt: receipt for bafyrei… was not found after 6 attempts
```

Nothing errors. Nothing logs a mismatch.

**The only thing in the world that noticed was the monorepo's `e2e`**, because
it is the only place that runs all the participants against one `libforge`.
Fixed by re-pinning a stale image digest; `e2e` green on the fix.

> Worth raising with the team as a question rather than a trophy: four
> repositories changed a wire format on one day, and no repository's own tests
> could have caught the mismatch. That is the case for a compat suite, and it
> is a live risk today regardless of the monorepo.

### 2. sprue's container build was broken by its own new file

`go build ./cmd/main.go` names a **file**, and Go then compiles only that file
as the whole package. Adding `cmd/version.go` meant `versionCmd` was never
compiled, and the container build went red — while `go build ./...`,
`go vet`, `gofmt`, `go test ./...` and `go-check`/`go-test` on three OSes all
stayed **green on the broken tree**. Package-level checks cannot see a
file-level build.

Fixed in [sprue#106](https://github.com/fil-forge/sprue/pull/106). Released
goreleaser binaries were never affected — `.goreleaser.yaml` already said
`main: ./cmd`.

### 3. hilt had the identical bug, latent

Found only because sprue's taught us to look — and found late, see *Not solved*
below. `hilt`'s `cmd/` holds one `.go` file today, so both Dockerfile stages
building `./cmd/main.go` work by luck; the next file added to `package main`
would have broken its container build exactly as sprue's did. Reproduced with
a throwaway second file before fixing.

Fixed in [hilt#77](https://github.com/fil-forge/hilt/pull/77), green including
the Docker-backed `itest` suite.

### 4. Every indexing-service release shipped three of four build fields dead

`.goreleaser.yaml` passed `-X main.version -X main.commit -X main.date -X
main.builtBy`, and package `main` declares **none of them**.

**The Go linker accepts a `-X` for a symbol that does not exist and says
nothing** — exit 0, no diagnostic, and `go version -m` still lists the flag as
though it took. Measured three ways on a real binary:

```
-X .../pkg/build.Commit=deadbee   ->  commit: deadbee
-X main.commit=deadbee            ->  commit: unknown
-X main.doesnotexist=hello        ->  builds, exit 0, no output
```

One flag happened to name a real symbol, which is why versions looked fine.
Fixed in [indexing-service#107](https://github.com/fil-forge/indexing-service/pull/107),
along with the `version` subcommand that makes it checkable at all.

### 5. A month-old swarf pin kept three firehose fixes out of production

`hilt` and `ingot` both pinned swarf at `d5d1a0a5` (2026-08-21), nine commits
behind, missing [#17](https://github.com/fil-forge/swarf/pull/17),
[#18](https://github.com/fil-forge/swarf/pull/18) and
[#19](https://github.com/fil-forge/swarf/pull/19). Both import
`swarf/pkg/client` in non-test code.

**#19 matters most in ingot**, which consumes the revocation firehose directly.
Before it, a failed stream was swallowed rather than returned, so the consumer
could not tell a stream that had *ended* from one that had *broken* — and a
silently-dead revocation feed means revoked access keys keep working out of a
stale cache.

Fixed in [hilt#77](https://github.com/fil-forge/hilt/pull/77) and
[ingot#175](https://github.com/fil-forge/ingot/pull/175).

### 6. An intermittently-red test, root-caused rather than re-run

`TestCachingQueuePoller_BatchProcessing`. The first diagnosis — "`poller.Stop()`
races the final `Delete`" — was **wrong**, and the real mechanism is worth
knowing: `Stop()` is not a barrier at all. It cancels its context and then
hands that already-cancelled context to `jobqueue.Shutdown`, which returns
`ctx.Err()` immediately without waiting for workers. Probed directly: 0 of 11
deletes had happened when `Stop()` returned after 49µs; 11 of 11 by 300ms
later. The WaitGroup was the test's only synchronisation.

Fixed in [indexing-service#106](https://github.com/fil-forge/indexing-service/pull/106).
30 runs under `-race -shuffle=on`, 0 failures.

### 7. A subtree tool that could not see the failure mode that mattered

When upstream deletes a file we had moved out of a prefix, **both sides deleted
the path, so git raises no conflict and the pull succeeds in silence.** The
first tool only ran on conflicts, so it was blind exactly when it was the only
thing looking. Rewritten to read upstream's own diff against the previous split
point and run in both modes.

Also settled a measurement error: `git subtree pull` records **no**
`git-subtree-split` trailer — only `git subtree add` does — so measuring drift
by grepping commit messages is always wrong. It had the drift wrong by 2.5×
(91 across nine prefixes, actually 37 across eight). Ancestry is the only sound
derivation.

[forge#13](https://github.com/fil-forge/forge/pull/13).

### 8. The revocation lookup contract, settled against the spec

Three places in swarf answered "what does looking up a revocation by delegation
CID return?" and no two agreed. [UCAN Revocation
v1.0.0-rc.1](https://github.com/ucan-wg/revocation) settles it: revocations are
**immutable, irreversible, and a monotonically-growing set**, so a second
revocation of the same delegation is an *addition*, not a correction. Both
current behaviours — PostgreSQL returning the newest row, the memory store
overwriting — are wrong for the same reason.

Written up in [swarf#20](https://github.com/fil-forge/swarf/issues/20). The
API shape (newest record, all records, or a boolean) is still a choice; the
semantics are not.

---

## Not solved

### 9. Nothing can tell you an in-house dependency is stale

The biggest finding, and entirely open.

**35 of `ingot`'s last 100 pull requests are dependabot's. Zero bump a
`fil-forge/*` module.** All 35 are third-party. `hilt` has no dependabot config
at all.

The cause is tagging, not privacy — `swarf` is public. `swarf`, `hilt` and
`ingot` each carry exactly one tag, `v0.0.0`, which sorts *below* the
pseudo-version already pinned; `sprue`, `libforge`, `ucantone`, `smelt` and
`guppy` have no tags at all. So there is no newer version for dependabot to
offer, and it does not chase untagged commits on a default branch.

**Eight of ten repositories have in-house dependencies that no tooling will
ever flag as behind.** Item 5 above was the normal outcome of that, not
anyone's oversight.

This is an independent argument for cutting release tags — one that holds
whether or not the monorepo happens.

### 10. Containers report no build metadata, in six of seven services

A containerised sprue answers `v0.0.0-unknown` / `unknown` / `unknown` /
`unknown`. Three independent causes: no `-X` flags in either Dockerfile stage,
`.dockerignore` excluding `version.json` (killing the development fallback),
and `.dockerignore` excluding `.git` (so no `vcs.revision` is stamped).

**sprue is not the outlier.** `guppy` is the only service that injects build
metadata into its image at all; `piri`, `sprue`, `ingot`, `hilt`, `swarf` and
`indexing-service` all build with `-ldflags="-s -w"` and nothing else. Fixing
one would make it two of seven rather than fix the class.

Awaiting a decision: six PRs from a shared pattern copied off guppy's
Dockerfile.

---

## Process notes worth reporting honestly

Not wins. Included because a report that only lists successes invites the
question of what was left out.

- **A PR of mine sat red for eighteen hours.** sprue#106 broke the container
  build on push, and the check-in polling only the `forge` PRs never looked.
  The evidence was on [[Needs Human Work]] the whole time — the state column
  for the upstream rows was blank. A derived list would have had no blank to
  leave.
- **A survey came back clean because it could not see the repositories it
  claimed to cover.** "No other repo builds a single `.go` file, across eleven
  repositories" — `hilt` and `ingot` were not cloned at the time. The grep
  pattern had been validated, which made a never-tested negative feel proven.
  `hilt` had the identical bug (item 3).
- **A cross-repo diagnosis was wrong and published before it was checked.** I
  reported that guppy had not adopted the new `/ucan/conclude` schema, having
  read a stale local branch as guppy's `main`. Guppy was already fixed; only
  our image pin was old. The correction is in the PR thread above the original.

The common shape: each was a claim derived over a set that was not the set it
purported to be. That is the working principle these produced — **derive the
list, and check that the list covers the population.**
