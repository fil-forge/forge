# Needs human work

**Updated 2026-09-16 14:40Z.** Everything on this page is waiting on a person —
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
| 1 | **Merge [#4](https://github.com/fil-forge/forge-2/pull/4)** `claude/itest-modules` — each itest suite its own module, and actually run in CI. Carries the six-service subtree resync. | Head `3caffaa6`. #6 moved `main` under it; **rebuilt** onto the new base per rule 7 (the earlier merge-of-main is gone). Tree is identical to the head that went 17/17, CI re-running. |
| 2 | **Decide on [#3](https://github.com/fil-forge/forge-2/pull/3)** `claude/bring-in-swarf`. Frozen at `1ede102`; its base `claude/images-from-head` merged as #1 on 2026-09-15. | Needs a **rebuild**, not a base repoint — it carries a `git subtree add` merge that a plain rebase would flatten (approach rule 6). Recipe below. Awaiting go-ahead. |

#3's rebuild, verified twice: `git subtree add -P swarf c43af97be79883bd74b20a1df2dab46e09605b0a`
onto the new base, cherry-pick the ten commits, check `git diff <old head> HEAD -- swarf/`
is empty and the upstream root is still an ancestor, `push --force-with-lease`,
then repoint the PR base to `main`.

## Decisions waiting

Flagged and deliberately not acted on. Each is a judgement call, not a task.

- **Wire `INGOT_ITEST_BIG` into CI**, or leave the 5 GiB max-part case to
  manual runs. Note what this actually costs, which is more than an env var:
  upstream's `go-test.yml` sets it alongside `-timeout 40m` inside
  `timeout-minutes: 60`, *and* a "Free runner disk space" step that deletes
  dotnet, android and CodeQL because the test churns 10-15 GiB. Our
  `itest.yml` runs `-timeout 25m` inside `timeout-minutes: 30` and frees
  nothing — a budget sized for the suite *without* this test. So the three
  move together or not at all.

  The docs here that describe it were **correct upstream** and were
  invalidated by us: `itest/README.md` says "(CI sets it)", which was true of
  `fil-forge/ingot` and stopped being true when consolidation pruned the
  per-service workflows.

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
