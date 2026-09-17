# Needs human work

**Updated 2026-09-17 15:55Z.** Everything on this page is waiting on a person —
either because it is a judgement call, or because the agent cannot perform the
action. Work that is merely unfinished does not belong here; see
[[Current State]] for the broad picture and [[Consolidation Findings]] for why
each item exists.

Kept current as things move. Items leave via **Recently cleared**, which is
pruned once it stops being useful.

## Blocking

Nothing else proceeds until these do.

*Nothing blocking.* **The import phase is closed** — #12 merged as `7ccafeab`
and all ten in-scope modules are on `main`. The two open PRs are follow-on
tidying and block nothing.

## Open pull requests

Two. Both sit on `main` independently, touching disjoint files; neither carries
a `git subtree add`, so they rebase rather than needing a rule 7 rebuild.

| | what | state |
|---|---|---|
| [#19](https://github.com/fil-forge/forge-2/pull/19) | **Move the four inline test image pins into `testutil`**, in piri's shape — named const, doc comment, env override. | `6442c4c5`. Was stacked on #17; #17 merging retargeted it to `main` and Petra rebased it. Verified: the rebase brought in only #20's two files and left the six-file change intact. Answers Petra's review question on #17; she asked for it as its own PR. |
| [#21](https://github.com/fil-forge/forge-2/pull/21) | **Shard `itest ingot` across three runners.** **Measured, green:** 30m43s → 23m07s on the workflow, ~25% — not the half predicted. Shards derive their own tests, so a new test cannot fall out silently. | `6fb033cd`, based on `3c3fe769`. The last commit replaces the entry's estimates with the measurement and re-triggers CI; the code is unchanged from the green run. **Does not need rebasing**: it touches only `itest.yml` and `MONOREPO_TODO.md`, and `main` has touched neither since. **Changes check names** to `itest ingot 1/3`, `2/3`, `3/3` — a branch protection rule naming the old one will wait forever, same edge as `replaces` → `guards`. |

**Two check-name changes are outstanding for branch protection**: #16's
`replaces` → `guards`, already merged and live; and #21's `itest ingot` split,
which lands when #21 does. A rule requiring either old name stops being
satisfied. Both were done on Petra's say-so, 2026-09-17.

**Nothing is left to import**, and nothing should be started. `MAJOR_DECISIONS.md`
records what is deliberately out.

## Waiting on Petra

Decisions taken while she was away, all reversible, all flagged on the PR that
made them. None needs undoing; they need confirming.

- **#16 is merged, so the `replaces` → `guards` rename is live.** A branch
  protection rule still requiring `replaces` now waits on a check that will
  never report.
- **#17 reuses digests already in the tree** rather than resolving fresh —
  `postgres:16-alpine` is `cf78e766…` in four other places, the minio release
  `2c4349a1…` in piri's testutil. Resolving fresh would have put two builds of
  one tag in one repository, which is the *agreement* failure rule 2 exists
  for. Both were checked against the registry and are still current.
- **#17 edits inside subtree prefixes** (`swarf/`, `indexing-service/`), which
  is local divergence every future `git subtree pull` carries — the concern
  that moved `staticcheck.conf` to the root on #10. Judged acceptable here:
  #6 already pins images inside `piri/`, `hilt/` and `sprue/`, and unlike a
  lint config an image pin has no root-level alternative.
- **#17 ships no Go image guard, deliberately**, and the reason is measured: a
  string shaped like an image reference matches 367 times in this repository,
  almost all `s3:GetObject` IAM actions and `host:port` pairs; narrowing to
  testcontainers call sites drops that to 8 but then misses two of the real
  references. A guard over part of a class reads exactly like a guard over the
  class (L12), so there is none rather than a partial one. That population
  stays unguarded, on purpose and in writing.
- **Two CI guards now exist that were the agent's own initiative.** The
  squashed-subtree guard was declined, so these are named rather than assumed
  welcome: `check-replaces.sh`'s second pass (one commit, `3b4c4d8e`, which
  has since caught the same fault twice — `hilt/itest/go.mod` on #3 and
  `ingot/itest/go.mod` on #10), and now `check-base-images.sh` and
  `check-stack-images.sh` on #16 and #17. All revert cleanly.

*Cleared since the last update:* #15's two-changes-in-one-PR bundling (merged),
and the `replaces` → `guards` rename (approved and done).

## Needs access the agent does not have

- **Branch deletion returns HTTP 403** from the git proxy — both
  `git push origin --delete` and the `:refs/heads/…` refspec form — while
  ordinary pushes to the same remote succeed. Left for a human rather than
  routed around.

  **Verified fully contained in `main` (`586738ed`) as of 15:29Z — eighteen,
  safe to delete now.** Derived, not typed:

  ```sh
  for b in $(git branch -r --format='%(refname:short)' | grep '^origin/claude/'); do
    git merge-base --is-ancestor "$b" origin/main && echo "$b"
  done
  ```

  `claude/bring-in-forgectl`, `claude/bring-in-indexing-service`,
  `claude/bring-in-swarf`, `claude/ci-concurrency`, `claude/ci-permissions`,
  `claude/e2e-stack-job`, `claude/images-from-head`,
  `claude/indexer-from-head`, `claude/itest-modules`, `claude/monorepo-todo`,
  `claude/pin-base-images`, `claude/pin-external-images`,
  `claude/pin-stragglers`, `claude/prune-dead-workflows`,
  `claude/restore-dropped-checks`, `claude/root-agents-md`,
  `claude/unify-library-pins`, `claude/upstream-findings`.

  **Not contained, so look before deleting:** `claude/major-decisions` and
  `claude/subtree-resync` (each may hold commits that reached `main` only as
  content); `claude/itest-image-pin-probe`, a throwaway probe that never
  became a PR; and `claude/pin-guppy`, which is
  [#2](https://github.com/fil-forge/forge-2/pull/2), closed unmerged.

  **Live, must stay:** `claude/test-image-pins` (#19),
  `claude/shard-itest` (#21).

- **Why this page kept going stale, and what is proposed about it.** Every
  rule this repository runs on has been living in one session's scheduled
  check-in prompts — session-local, timer-driven, gone when the session ends.
  So these pages were updated when a check-in fired rather than when the thing
  they describe changed, and drifted in between; Petra noticed #19 missing
  before any check-in did. **[#20](https://github.com/fil-forge/forge-2/pull/20)
  is merged**, so the fix is in place rather than proposed: a root `AGENTS.md`,
  which loads at the start of every session, carrying the trigger as an
  *event* — opened, pushed, merged, closed, or decided → update the wiki before
  reporting — rather than as "keep it current", which is the phrasing that
  failed. Whether it works is now an observable thing rather than an argument.

- **This wiki lives in two places and the agent can only write one.** These
  pages are the `wiki` branch of `fil-forge/forge-2`, which it pushes, and the
  real GitHub wiki at `forge-2.wiki.git`, which it can read but not write —
  the git proxy returns `403 … not in this session's authorized repository
  set`, and `forge-2.wiki` cannot be added as a source because GitHub does not
  expose a wiki as a repository. **Petra is syncing the wiki herself**
  (2026-09-17), so this is not blocking; it is here so nobody assumes a push
  to the branch reaches both.

- **Archiving `session_01GXUttS5N775eQ7QXboRAxe`**, the round-1 review
  session. It has nothing left to post; see *Recently cleared*.
- **The `forge-2` → `forge` rename**, whenever this path is judged correct.
  Module paths are already `github.com/fil-forge/forge/*` and are wrong only
  in the interim, so the rename fixes them by happening rather than needing a
  sweep of its own. What is broken meanwhile is narrow and worth knowing
  before someone "fixes" it: an external `go get
  github.com/fil-forge/forge/piri` resolves at the *old* repository and fails
  there. Nothing we build is affected — per-module `GOWORK=off` builds, the
  workspace build, and every `replace ../<svc>` edge are relative and never
  consult the network for an in-repo module.

## A question about this wiki's own history

Nine commits here are **authored as `Peeja <petra@fil.org>` but carry a
`Co-Authored-By: Claude` trailer**: `9507e883`, `1621b192`, `b6f39672`,
`12fcb325`, `382046ce`, `e711aaa2`, `603ec97a`, `ec62b713` and `7525c169`.
The agent wrote them; the author field says you.

Nine older commits (`5ee5b2e5` through `7f982eac`) have no trailer either way
and cannot be told apart from here.

**Fixed going forward** — the worktree's identity is now
`Claude <noreply@anthropic.com>`, and every wiki commit since `fd012dd9` is
attributed correctly. The question is only whether the nine should be
rewritten. Rewriting them rewrites the wiki's history, which is cheap here and
still your call.

## Recently cleared

- **#20 and #17 merged**, 2026-09-17 15:19Z (`4db13774`) and 15:21Z
  (`586738ed`). The repository has a root `AGENTS.md` for the first time, and
  every image reference the stack pulls is pinned by digest with a guard over
  the compose half. Merging #17 also retargeted #19 to `main`; Petra rebased
  it, and the rebase was verified to bring in nothing but #20's two files.
- **#17's three flagged decisions are effectively confirmed** by its merge:
  reusing digests already in the tree rather than resolving fresh, editing
  inside subtree prefixes, and shipping no Go image guard. They are left
  written down above rather than deleted, because the reasoning is the part
  worth keeping.
- **The import phase is over.** #12 merged 2026-09-17 as `7ccafeab`, putting
  forgectl in and with it the tenth and last in-scope module. #15 followed as
  `ff2f794d`. `main` now carries delegator, forgectl, hilt, indexing-service,
  ingot, piri, piri-signing-service, smelt, sprue and swarf, each with its own
  history behind a subtree merge.
- **#12's merge could not be done by the agent**, and that is worth keeping
  rather than forgetting: GitHub had it registered as a *stacked* pull request
  from when its base was `claude/indexer-from-head`, and the registration
  outlived both #11 merging and the retarget to `main`. REST merge, auto-merge
  and changing the base all refused, each with a different message naming the
  stack. One click in the web UI, no API route.
- **The `replaces` job is now `guards`.** It had run `gofmt -s` as well as
  `check-replaces.sh` for some time, and #16 and #17 added two more guards —
  `check-base-images.sh` and `check-stack-images.sh`, both now on `main`.
- **#14 merged** 2026-09-17 ~01:5xZ as `da029c51` — `ci.yml` gained the
  `concurrency` block the other three workflows already had, so its runs stop
  piling up on a re-push. Amended in review to drop a comment that explained
  the standard idiom rather than this file.
- **#12's force-push blocker is gone.** The rebuild onto the post-#14 `main`
  is pushed and green; only the merge itself is stuck, and for an unrelated
  reason (see Blocking).
- **The eight-module monorepo became a nine-module one, then the import phase
  ended.** Merged in order: **#3** (swarf) 2026-09-16 20:47Z as `433cd628`;
  **#9** (the dropped checks restored) 23:06Z as `7435ba98`; **#8**
  (`MONOREPO_TODO.md`) 23:58Z as `f1746b3e`; **#10** (indexing-service)
  2026-09-17 00:45Z as `6c6cad31`; **#11** (the indexer built from HEAD)
  01:46Z as `ecb70114`. Only #12 is left.
- **#10 answered the question the plan left open about indexing-service's
  client, and it was not the expected answer.** Derived rather than assumed:
  `ingot` links `pkg/client` and `pkg/types` in non-test code, so ingot's
  Docker build needs indexing-service **source**, not a `go.mod` stub.
- **`SA4006` is off repo-wide and that is recorded, not hidden.** #10 adds a
  root `staticcheck.conf`; #8 carries the entry for turning it back on. The
  obvious fix — run the staticcheck the polyrepo ran — was tested and does not
  exist: both older versions fail on every package against Go 1.27. See
  [[Current State]] Known debt.
- **#12's own review is done.** Approved in substance at `9106da01`; the
  pending rebuild changes no file in the work itself, only the base under it.
- **The round-1 review session has nothing left to post.**
  `session_01GXUttS5N775eQ7QXboRAxe` never got repo access, so it could not
  post to GitHub — but its findings did reach us, and both are in merged #6:
  the `matchPackageNames` collision in `renovate.json`, and the images its
  sweep counted as missing, which is what `36d3c5dd` fixed. Its #4 review was
  a clean pass.
- **`itest hilt` went red and is fixed** (`d8d1ef37`). Not one of the three
  suspects written down at the time — the cause was `hilt/itest/go.mod`
  requiring the upstream `github.com/fil-forge/swarf`. It is why
  `check-replaces.sh` grew its second pass, which then caught the same shape
  on #10.
- **`INGOT_ITEST_BIG` stays manual** — decided. Written up in
  `MAJOR_DECISIONS.md` rather than here, because "a gated test CI never
  enables" reads as an oversight to anyone who finds it cold.
- **`swarf` will not stay pinned** — decided 2026-09-16. Rule 2 applies as
  written: moving into the monorepo retires the pin, no exception.
- **Subtree history is never squashed** — decided 2026-09-16, and now rule 7
  in [[Current State]] rather than an open question.
