# Needs human work

**Updated 2026-09-16 12:30Z.** Everything on this page is waiting on a person —
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
| 1 | **Merge [#4](https://github.com/fil-forge/forge-2/pull/4)** `claude/itest-modules` — each itest suite its own module, and actually run in CI. Carries the six-service subtree resync. | CI re-running on `8efa2335`. Was 17/17 green on `3c1a04b3`, whose tree is identical. |
| 2 | **Merge [#6](https://github.com/fil-forge/forge-2/pull/6)** `claude/pin-external-images` — 18 images pinned by digest across 35 references, plus a `renovate.json` that would keep them moving. | 15/15 green at `ef607dea`. Based on `main`, so independent of #4. |
| 3 | **Decide on [#3](https://github.com/fil-forge/forge-2/pull/3)** `claude/bring-in-swarf`. Frozen at `1ede102`; its base `claude/images-from-head` merged as #1 on 2026-09-15. | Needs a **rebuild**, not a base repoint — it carries a `git subtree add` merge that a plain rebase would flatten (approach rule 6). Recipe below. Awaiting go-ahead. |

#3's rebuild, verified twice: `git subtree add -P swarf c43af97be79883bd74b20a1df2dab46e09605b0a`
onto the new base, cherry-pick the ten commits, check `git diff <old head> HEAD -- swarf/`
is empty and the upstream root is still an ancestor, `push --force-with-lease`,
then repoint the PR base to `main`.

## Decisions waiting

Flagged and deliberately not acted on. Each is a judgement call, not a task.

- **Subtree history: unsquashed or `--squash`?** `main` is unsquashed by
  construction — seven `Add '<svc>/' from commit '<sha>'` adds, 1010 commits,
  59 merges — and `git subtree pull` continues that, so #4 carries 53 upstream
  commits behind 6 merges. Raised over #4 on 2026-09-16 and settled *for #4
  only*: one empty merge dropped, the mechanism left alone. The repo-wide
  question is open and belongs to `main`, not to a branch. Mixing the two
  modes on one prefix is the thing to avoid.
- **`itest.yml`'s timeout budget.** `-timeout 25m` sits inside
  `timeout-minutes: 30` so Go fires first and dumps goroutines rather than the
  runner killing the job blind. Observed run: 21m04s. The margin is thin, and
  widening it is a CI spend question.
- **Restore the s3-compat report pipeline.** ingot's `2365944c`
  ([ingot#131](https://github.com/fil-forge/ingot/pull/131), publish the
  compatibility report to GitHub Pages) arrived with the resync; no equivalent
  job exists here.
- **Wire `INGOT_ITEST_BIG` into CI**, or leave the large-object cases to
  manual runs.
- **Does `swarf` stay pinned** once #3 lands, or does moving in retire its pin
  the way approach rule 2 says it should?
- **Turn Renovate on.** #6 ships the config as a demonstration; installing the
  GitHub App on the org is an admin action, deliberately not attempted.

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

- **`claude/itest-image-pin-probe` is gone** — the scratch branch the 403
  above blocked. Deleted since; no longer outstanding.
- **The three MinIO repoints all merged**:
  [indexing-service#96](https://github.com/fil-forge/indexing-service/pull/96)
  (2026-09-14),
  [piri#123](https://github.com/fil-forge/piri/pull/123) and
  [sprue#97](https://github.com/fil-forge/sprue/pull/97) (both now in our
  subtrees as `ca232743` and `7fe2dad8`). [[Current State]] still lists the
  last two as open; that page is a 2026-09-14 snapshot and has drifted.
