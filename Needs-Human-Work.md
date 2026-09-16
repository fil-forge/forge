# Needs human work

**Updated 2026-09-16 21:25Z.** Everything on this page is waiting on a person —
either because it is a judgement call, or because the agent cannot perform the
action. Work that is merely unfinished does not belong here; see
[[Current State]] for the broad picture and [[Consolidation Findings]] for why
each item exists.

Kept current as things move. Items leave via **Recently cleared**, which is
pruned once it stops being useful.

## Blocking

Nothing else proceeds until these do.

*Nothing blocking.* The eight-module monorepo is built: #3 merged
2026-09-16 20:47Z as `433cd628`, and `main` now carries all eight services
with their histories.

## Open pull requests

None blocks anything. #10 is stacked on #9.

| | what | state |
|---|---|---|
| [#10](https://github.com/fil-forge/forge-2/pull/10) | **Bring `indexing-service` into the monorepo** — the ninth module and the last service. Stacked on #9. | Rebuilt (not rebased) when #9 changed, per rule 7; trees verified byte-identical. CI running. |
| [#9](https://github.com/fil-forge/forge-2/pull/9) | **Restore the dropped checks**: `staticcheck`, `gofmt -s`, `-race`, `-shuffle=on`. | Rebuilt to Petra's 20:57Z review — comments cut, `stress-tester` moved to the TODO, tests are one run not two. All five threads answered. CI running. |
| [#8](https://github.com/fil-forge/forge-2/pull/8) | **`MONOREPO_TODO.md`** — four swarf findings, plus hilt's build context, the macOS run and `stress-tester`'s coverage. Docs only. | Asks one question: should upstream findings and whole-repo questions share the file? |

## Decisions waiting

Flagged and deliberately not acted on. Each is a judgement call, not a task.

- **A CI guard I added on my own initiative, easy to drop.** You declined a
  squashed-subtree guard, so this one should be named rather than assumed
  welcome: `.github/scripts/check-replaces.sh` (which already ran as the
  `replaces` job) now also fails when any `go.mod` requires a service this
  repository contains under its old upstream path. That is the bug that just
  took `itest hilt` red — `hilt/itest/go.mod` still required
  `github.com/fil-forge/swarf` while its `.go` files imported
  `github.com/fil-forge/forge/swarf`. It is module wiring, not commit history,
  and the same shape recurs on every import: a service arrives by subtree with
  the `go.mod` it had upstream, and a nested module (`hilt/itest`,
  `ingot/itest`) is not reached by a sweep over the service's own `go.mod`.
  `indexing-service` is next in. One commit (`3b4c4d8e`), reverts cleanly.

## Needs access the agent does not have

- **Branch deletion returns HTTP 403** from the git proxy — both
  `git push origin --delete` and the `:refs/heads/…` refspec form — while
  ordinary pushes to the same remote succeed. Left for a human rather than
  routed around. Deletable now: `claude/e2e-stack-job` and
  `claude/images-from-head`, both fully contained in `main`. After #4 lands:
  `claude/subtree-resync` (contained in #4's branch). `claude/pin-guppy` is
  [#2](https://github.com/fil-forge/forge-2/pull/2), closed unmerged — keep or
  drop as you prefer.
- **The `forge-2` → `forge` rename**, whenever this path is judged correct.
  Module paths are already `github.com/fil-forge/forge/*` and are wrong only
  in the interim, so the rename fixes them by happening rather than needing a
  sweep of its own. What is broken meanwhile is narrow and worth knowing
  before someone "fixes" it: an external `go get
  github.com/fil-forge/forge/piri` resolves at the *old* repository and fails
  there. Nothing we build is affected — per-module `GOWORK=off` builds, the
  workspace build, and every `replace ../<svc>` edge are relative and never
  consult the network for an in-repo module.

## Recently cleared

- **`itest hilt` went red and is fixed** (`d8d1ef37`). Not one of the three
  suspects I had written down — the build contexts, swarf's libforge pin and
  the `${SWARF_IMAGE:?}` flip were all sound. `${SWARF_IMAGE:?}` in
  particular cannot fire from CI: `WithPublishedImages()` fills
  `c.swarfImage`, which the stack writes into the compose environment itself.
  The actual cause was `hilt/itest/go.mod` requiring the upstream
  `github.com/fil-forge/swarf`; nothing pointed at it until this branch,
  because hilt only started importing swarf here.
- **Two things that fell out of that fix.** `itest.yml` was building six
  images from HEAD and leaving swarf — now the seventh in-repo service — on
  the published `:main` tag, which is the mutable-peer ambiguity the rest of
  that `env` block exists to remove; `e2e.yml` already built it, and the two
  workflows now match. And `itest.yml`'s header still called the suite "a
  compatibility check" against the published network, contradicting its own
  `env` block sixty lines down. Both corrected on #3.

- **[#4](https://github.com/fil-forge/forge-2/pull/4) merged** 2026-09-16
  ~15:57Z as `edf25236` — itest suites as their own modules and actually run,
  the six-service subtree resync, `itest` peers from HEAD, and
  `MAJOR_DECISIONS.md`. Merged with a merge commit, so #3 already contains it
  and needed no fourth rebuild.
- **`itest`'s peers come from HEAD** — decided, implemented and pushed on #4.
  The suites append `pkg/stack.OptionsFromEnv` after `WithPublishedImages`, and
  `itest.yml` builds the six services from the commit under test, so a red
  means this service regressed rather than that somebody else merged.
  `WithPublishedImages` stays the local default. I had briefly reopened this
  here after finding my own justification for the old behaviour unsupported;
  it was settled the same hour and should not have stayed on the list.
- **`INGOT_ITEST_BIG` stays manual** — decided. The 5 GiB max-part case runs on
  demand, not in CI; enabling it is a three-part change (the variable, a
  40m/60m budget, and a step freeing 10–15 GiB of runner disk). Written up in
  `MAJOR_DECISIONS.md` rather than here, because "a gated test CI never
  enables" reads as an oversight to anyone who finds it cold.
- **#3's rebuild is done.** Replayed onto #4 per rule 7 — fresh
  `git subtree add` at `c43af97b`, then the ten commits. Three conflicts, all
  real rather than textual: hilt's `go.mod`/`Dockerfile` (see below), the
  workflow comments #4 had already reworded, and swarf's compose image, where
  #4's own comment said to flip the pin to `:?` once this branch landed —
  which is the rule-2 outcome already settled.
- **The round-1 review session has nothing left to post.**
  `session_01GXUttS5N775eQ7QXboRAxe` never got repo access, so it could not
  post to GitHub — but its findings did reach us, and both are in merged #6:
  the `matchPackageNames` collision in `renovate.json` (the offending rule was
  deleted; `main` now has no `excludePackageNames` and no rule combining `*`
  with another matcher), and the images its sweep counted as missing, which is
  what `36d3c5dd` fixed — `amazon/dynamodb-local:latest` is pinned by digest
  in `smelt/systems/common/compose.yml`, and `main` carries 18 distinct images
  across 37 pinned references. Its #4 review was a clean pass. An earlier
  version of this page said round 2 "re-derived" its findings; that was an
  assumption and it was wrong — they arrived directly, which is why
  `36d3c5dd` exists at all.
- **`itest.yml`'s timeout budget is a note, not a decision.** The failure mode
  is loud and self-labelling, so it moved to Known debt in [[Current State]]
  to be revisited on the first red rather than pre-emptively.
- **`swarf` will not stay pinned** — decided 2026-09-16. Rule 2 applies as
  written: moving into the monorepo retires the pin, no exception.
- **The s3-compat report pipeline and turning Renovate on** moved to
  `MONOREPO_TODO.md` at the repository root (branch `claude/monorepo-todo`,
  no PR opened). Both need the monorepo looked at whole, so they are not
  blocking anything and should not be answered early.
- **Subtree history is never squashed** — decided 2026-09-16, and now rule 7
  in [[Current State]] rather than an open question. `--squash` is off the
  table on every prefix, so the commit and merge counts `main` carries are
  the intended cost, not drift.
- **[#6](https://github.com/fil-forge/forge-2/pull/6) merged** 2026-09-16
  12:27Z — 18 images pinned by digest across 35 references, plus the Renovate
  config. It moved `main` under #4; see blocking item 1.
- **`claude/itest-image-pin-probe` is gone** — the scratch branch the 403
  above blocked. Deleted since; no longer outstanding.
- **The three MinIO repoints all merged**:
  [indexing-service#96](https://github.com/fil-forge/indexing-service/pull/96)
  (2026-09-14),
  [piri#123](https://github.com/fil-forge/piri/pull/123) and
  [sprue#97](https://github.com/fil-forge/sprue/pull/97) (both now in our
  subtrees as `ca232743` and `7fe2dad8`). [[Current State]] still lists the
  last two as open; that page is a 2026-09-14 snapshot and has drifted.
