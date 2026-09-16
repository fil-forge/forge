# Needs human work

**Updated 2026-09-16 13:45Z.** Everything on this page is waiting on a person —
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

- **Wire `INGOT_ITEST_BIG` into CI**, or leave the large-object cases to
  manual runs.

## Needs access the agent does not have

- **Branch deletion returns HTTP 403** from the git proxy — both
  `git push origin --delete` and the `:refs/heads/…` refspec form — while
  ordinary pushes to the same remote succeed. Left for a human rather than
  routed around. Deletable now: `claude/e2e-stack-job` and
  `claude/images-from-head`, both fully contained in `main`. After #4 lands:
  `claude/subtree-resync` (contained in #4's branch). `claude/pin-guppy` is
  [#2](https://github.com/fil-forge/forge-2/pull/2), closed unmerged — keep or
  drop as you prefer.
- **A stuck review session.** `session_01GXUttS5N775eQ7QXboRAxe` (round-1
  review of #4 and #6) is blocked on a permission prompt to post its comments
  and cannot be reached from the session doing the work. Its findings were
  independently re-derived in round 2, so nothing is lost by abandoning it.
- **The `forge-2` → `forge` rename**, whenever this path is judged correct.
  Module paths are already `github.com/fil-forge/forge/*` and are wrong only
  in the interim.

## Recently cleared

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
