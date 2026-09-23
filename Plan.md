# Plan

**The one-page map**, kept short on purpose. [[Current State]] has the detail
and the evidence, [[Needs Human Work]] has what is waiting on a person,
[[Consolidation Findings]] has the reasoning. This page is only the shape.

**Goal:** the Fil Forge polyrepo becomes one monorepo at
[`fil-forge/forge`](https://github.com/fil-forge/forge), with each service
still released on its own cadence.

**Where we are:** Phase 0 is done, the move to `forge` is done, and the final
subtree pull is done too — it is
[#14](https://github.com/fil-forge/forge/pull/14), green and open. **Phase 1 is
gated on that landing**, and #14 is gated on review. `main` is `0d8fb04c`;
**five PRs are open on `forge` and five upstream, all ten green.** Nothing is
blocked on the agent.

---

## Done

- **All ten modules are in**, by `git subtree add` with history intact —
  `piri`, `hilt`, `ingot`, `sprue`, `smelt`, `delegator`,
  `piri-signing-service`, `swarf`, `indexing-service`, `forgectl`. **Nothing
  left to import.**
- **Module paths rewritten** to `github.com/fil-forge/forge/<svc>`, sibling
  edges resolved with `replace ../<svc>`, one root `go.work`.
- **Phase 0's exit bar met:** every module builds, vets, is `go mod tidy`
  clean and passes its tests — enforced continuously by CI, not proven once.
- **CI exists:** `ci` (per-module unit jobs + `guards`), `itest`, `e2e`,
  `images`. Four guard scripts, each deriving what it checks rather than
  carrying a hand-maintained list.
- **Stack suites run on images built from HEAD**, not on a floating `:main`.
- **External images are digest-pinned; in-repo services build from HEAD.**
- **Layer cache measured**, not assumed: 23s warm against a 7m34s baseline,
  ~10m42s cold. Caches are per-repository, so `forge` started cold.
- **The move to `fil-forge/forge` is complete** — the old `#6`/`#7`/`#8` chain
  is closed and `forge-2` is archived (not deleted, so its PR links still
  resolve). `main` has moved on since: `991633b0` (#9), `9870d48a` (#11),
  and now **`0d8fb04c`** (#10).
- **Every subtree is resynced to its upstream `main`** — 37 commits across
  eight prefixes, measured by ancestry rather than by grepping commit messages.
  Done as work; awaiting review as #14.

## In flight

**Ten open pull requests.** On `forge` they are fully Open; upstream they stay
draft until Petra has passed on them, so nobody else spends time first.

| | what | state |
|---|---|---|
| [forge #12](https://github.com/fil-forge/forge/pull/12) | a release workflow: written, verified, **deliberately not armed** | Open, 24/24 |
| [forge #13](https://github.com/fil-forge/forge/pull/13) | `finish-subtree-pull.sh` (merge + the deletion audit) and `resolve-rewrite-conflicts.sh` | Open, 24/24 |
| **[forge #14](https://github.com/fil-forge/forge/pull/14)** | **the final subtree resync** — 37 commits across eight prefixes. Caught a live cross-repo wire break. **Phase 1 waits on this** | Open, 24/24 |
| [forge #15](https://github.com/fil-forge/forge/pull/15) | one `MONOREPO_TODO.md` entry: service images report no build metadata. Its own review found the entry wrong four ways; corrected on the branch | Open |
| [forge #16](https://github.com/fil-forge/forge/pull/16) | four Makefiles injected `-X` at a dead path and one at symbols nobody declared; the guard globbed `.goreleaser.y*ml` and never looked. **Stacked on #12, and not optionally** — it edits two files that exist only on #12's branch, so **#12 merges first** | Open |
| [sprue #106](https://github.com/fil-forge/sprue/pull/106) | a `version` subcommand, **plus the container build it broke** | draft, 8/8 |
| [indexing-service #106](https://github.com/fil-forge/indexing-service/pull/106) | the poller flake, root-caused not re-run | draft, 9/9 |
| [indexing-service #107](https://github.com/fil-forge/indexing-service/pull/107) | a `version` subcommand + **four dead `-X` ldflags** | draft, 9/9 |
| [hilt #77](https://github.com/fil-forge/hilt/pull/77) | swarf bumped 9 commits, **plus the same `./cmd/main.go` bug sprue had** | draft, on `6584213` after merging `main` |
| [ingot #175](https://github.com/fil-forge/ingot/pull/175) | swarf bumped 9 commits; ingot consumes the firehose directly | draft, 17/17 |

Merged since the last revision of this page: **#9** (goreleaser ldflags),
**#11** (sharding `itest ingot`), **#10** (the subtree merge tool — **#13
supersedes its interface**).

One issue, no PR: [swarf #20](https://github.com/fil-forge/swarf/issues/20),
the revocation lookup contract. The spec settles the semantics; the API shape
is still a choice. See [[Needs Human Work]].

## Ahead

Phase numbers are the plan's. **1, 3 and 4 are quoted from it; 2, 5 and 6 are
inferred from its own ordering** — treat those three names as approximate
until someone checks the plan text.

| | what | gated on |
|---|---|---|
| **1** | Release tags, `compat.yml`, publishing | started; see below |
| **2** | Protocol gates | Phase 1 |
| **3** | `libforge`'s dissolution — the first real test of the audience rule | — |
| **4** | "Consolidate the client, archive `guppy`" | — |
| **5** | Harness rework | — |
| **6** | Tag-scheme cleanup | — |

**Phase 1 is bigger than the plan assumed.** It says to cut initial tags "via
the *existing* `release.yml` flow" — there is no `release.yml`. `main` has
four workflows, none of which tags, releases or publishes. What survived the
import is uneven raw material:

| | have it |
|---|---|
| `version.json` | 8 of 10 — not `forgectl`, not `smelt` |
| `.goreleaser.yaml` | 4 — `indexing-service`, `ingot`, `piri`, `sprue` |
| `Dockerfile.release` | 4 — `hilt`, `ingot`, `sprue`, `swarf`. **Two are live**: `ingot`'s and `sprue`'s `dockers:` stanzas build them, packaging the goreleaser binary, so those released images *are* stamped. `hilt`'s and `swarf`'s are referenced by nothing |

The parts that need no further gate — writing the release workflow without
cutting tags, `compat.yml`, deciding the tag scheme — can go first.

**The shape of the flow is decided** (Petra, 2026-09-23, on
[#18](https://github.com/fil-forge/forge/pull/18)), which turns most of what is
left of Phase 1 from an open question into build work. **This is in scope for
the consolidation** — it is not a post-consolidation TODO — so the design lives
here:

- **A release pull request carries only the version bump.** One edit to
  `<service>/version.json` and nothing else. Merging it means exactly one
  thing: that module's current `main` is now that version. It therefore floats
  on `main` and sweeps up every change to the module since its last release —
  no cherry-picking, no release branch, and no second pull request for an
  ordinary change.
- **Merging it causes the tagging, automatically.** The merge already carries
  the whole decision, so a human retyping `<service>/vX.Y.Z` afterwards is a
  second chance to get it wrong rather than a second check. Today the tag is
  made by hand and `release.yml` asserts it exists at the built commit; that
  assertion is the interim, not the design.
- **Release pull requests are issued automatically.** A module needs one once
  it has been touched since its last release, so what has to be derived is
  which commits touch which module since `<service>/vX.Y.Z` — roughly
  `git log <service>/vX.Y.Z..main -- <service>/` being non-empty. Needs the tag
  to anchor from, and a rule for a module never released (no tag; the answer is
  its first commit). **Open:** one pull request per module needing one, or one
  bumping several. Per module keeps `compat.yml`'s `release/<service>`
  convention and per-service semver intact.

**Still gated on the two-sources-of-truth decision**, which is the same gate as
the rest of Phase 1: while the polyrepo still releases these services, a tag cut
here gives each of them two. Automatic tagging makes that worse rather than
better, so it is built after the decision, not before it. `MONOREPO_TODO.md`
carries that gate and the raw-material inventory; the design above is this
plan's.

### And the gate does not yet gate anything

Measured on #18, 2026-09-23: a dispatched `compat.yml` run went **green having
tested nothing**. `TestRollingUpgrade` needs a published baseline for every
service in the fleet and skips when any is missing — `delegator`, `hilt`,
`piri-signing-service`, `swarf` and `sprue` have none, so it skipped in 0.126s
and the check reported success.

The job's own condition asks a different question from the test: it runs when
`pinnable` is true, which `compat-window.sh` sets from the *window* (piri and
ingot have one), while the test needs *baselines for all eight*. So the job
runs and the test opts out.

**This is Phase 1 work, not a defect to file**: the gate cannot mean anything
until the fleet is published, which is the same gate above. What #18 should not
do is report green in the meantime.

## Not in scope

Decided, not deferred; `MAJOR_DECISIONS.md` carries the rule. Staying out:
**outward-facing libraries** (`ucantone`, `automobile`), **forks of upstream
software we patch** (`minio`, `storetheindex`, `did-method-plc`,
`filecoin-localdev`, `versitygw`, `filecoin-services`), and **`guppy`**, which
is being dismantled rather than moved.
