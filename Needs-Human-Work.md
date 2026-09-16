# Needs human work

**Updated 2026-09-16 16:15Z.** Everything on this page is waiting on a person —
either because it is a judgement call, or because the agent cannot perform the
action. Work that is merely unfinished does not belong here; see
[[Current State]] for the broad picture and [[Consolidation Findings]] for why
each item exists.

Kept current as things move. Items leave via **Recently cleared**, which is
pruned once it stops being useful.

## Blocking

Nothing else proceeds until these do.

| | what | state |
|---|---|---|
| 1 | **Merge [#4](https://github.com/fil-forge/forge-2/pull/4)** `claude/itest-modules` — each itest suite its own module, and actually run in CI. Carries the six-service subtree resync. | Head `0d455123`, CI running. Also moves `itest`'s peer images to HEAD, so a red means this service regressed rather than that somebody else merged. |
| 2 | **Merge [#3](https://github.com/fil-forge/forge-2/pull/3)** `claude/bring-in-swarf` — swarf as the 8th module. | **Rebuilt onto #4** and base repointed there; head `76204d40`. `swarf/` is byte-identical to the frozen `1ede102`, the upstream root is still an ancestor, and swarf/hilt/ingot all build, vet, tidy and gofmt clean. |

## Decisions waiting

Flagged and deliberately not acted on. Each is a judgement call, not a task.

*Nothing waiting.*

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
