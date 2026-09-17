# Plan

**The one-page map**, kept short on purpose. [[Current State]] has the detail
and the evidence, [[Needs Human Work]] has what is waiting on a person,
[[Consolidation Findings]] has the reasoning. This page is only the shape.

**Goal:** the Fil Forge polyrepo becomes one monorepo at
[`fil-forge/forge`](https://github.com/fil-forge/forge), with each service
still released on its own cadence.

**Where we are:** Phase 0 is done, the move to `forge` is done, Phase 1 has
started.

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
- **The move to `fil-forge/forge` is complete** — `main` is `24b18ee5` there,
  the old `#6`/`#7`/`#8` chain is closed, and `forge-2` is archived (not
  deleted, so its PR links still resolve).

## In flight

| | what | state |
|---|---|---|
| [`swarf` #17](https://github.com/fil-forge/swarf/pull/17) | Firehose client hangs on an event over 64 KiB; fixed upstream, plus an `internal/sse` extraction | **Green, waiting on review** |
| [#9](https://github.com/fil-forge/forge/pull/9) | Every goreleaser `-X` ldflag names a pre-consolidation module path, so a release would ship binaries reporting `v0.0.0`. Fix + a guard | **Open** |
| the final `git subtree pull` | ~45 commits of upstream drift to take in one pull at the end | **Not started.** Must come *after* `swarf` #17 merges, or it brings the hang |

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
| `Dockerfile.release` | 4 — `hilt`, `ingot`, `sprue`, `swarf`, and **nothing builds them** |

The parts that need no further gate — writing the release workflow without
cutting tags, `compat.yml`, deciding the tag scheme — can go first.

## Not in scope

Decided, not deferred; `MAJOR_DECISIONS.md` carries the rule. Staying out:
**outward-facing libraries** (`ucantone`, `automobile`), **forks of upstream
software we patch** (`minio`, `storetheindex`, `did-method-plc`,
`filecoin-localdev`, `versitygw`, `filecoin-services`), and **`guppy`**, which
is being dismantled rather than moved.
