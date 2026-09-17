# Needs human work

**Updated 2026-09-17 02:55Z.** Everything on this page is waiting on a person —
either because it is a judgement call, or because the agent cannot perform the
action. Work that is merely unfinished does not belong here; see
[[Current State]] for the broad picture and [[Consolidation Findings]] for why
each item exists.

Kept current as things move. Items leave via **Recently cleared**, which is
pruned once it stops being useful.

## Blocking

Nothing else proceeds until these do.

- **[#12](https://github.com/fil-forge/forge-2/pull/12) is green and ready and
  the agent cannot merge it.** 22/22 checks, `mergeable_state: clean`, base
  `main`. GitHub has it registered as a **stacked** pull request — left over
  from when its base was `claude/indexer-from-head` — and that survived #11
  merging and the retarget to `main`. Every route the agent has refuses:

      REST merge        403  Merging stacked PRs via this endpoint is not supported
      auto-merge        Auto-merge is not supported for stacked pull requests
      change the base   Cannot change the base branch because the PR is part of a stack

  The web UI's merge button uses the endpoint the first error points at, so
  this is **one click for you** and nothing is wrong with the PR. Merging it
  closes the import phase: all ten in-scope modules in.

  (The earlier blocker on this page — the force-push needed to rebuild #12 —
  is cleared. Petra authorised it, the rebuild is pushed, and it is what is
  green now.)

## Open pull requests

Four. #12 is the last import; #15 → #16 → #17 are follow-on tidying and stack
on each other. None of the three carries a `git subtree add`, so they rebase
rather than needing a rule 7 rebuild.

| | what | state |
|---|---|---|
| [#12](https://github.com/fil-forge/forge-2/pull/12) | **Bring `forgectl` in** — the tenth and last in-scope module. Widens delegator's Docker context to the repository root, because delegator links it in non-test code. | **22/22 green.** Blocked on the merge above, not on anything technical. |
| [#15](https://github.com/fil-forge/forge-2/pull/15) | **`ci.yml` gains `permissions: contents: read`** — it was the only workflow without one — plus the CI path-filtering question recorded in `MONOREPO_TODO.md`. | On `main`. Two changes in one PR *deliberately*: unfiltered CI charges a full ~26-minute `itest` run for a Markdown file, so two PRs would have cost two of them. Split it if you would rather. |
| [#16](https://github.com/fil-forge/forge-2/pull/16) | **Pin the base images our Dockerfiles build `FROM`** — 26 references, by index digest — plus `check-base-images.sh`. Rule 2 applied one layer below where #6 stopped. | On #15. |
| [#17](https://github.com/fil-forge/forge-2/pull/17) | **Pin the 5 references that arrived after #6 had finished pinning**, carried in by swarf (#3) and indexing-service (#10) — plus `check-stack-images.sh` for compose. | On #16. |

**Nothing is left to import.** `MAJOR_DECISIONS.md` named indexing-service and
forgectl as the last two pending; indexing-service landed with #10 and
forgectl is #12. Do not start another module without saying so.

## Waiting on Petra

Decisions taken while you were away, all reversible, all flagged on the PR
that made them. None needs undoing to keep going; they need confirming.

- **#15 bundles two unrelated changes** — a workflow permissions block and a
  TODO entry — to spend one CI run instead of two. Ordinarily they would be
  separate.
- **#16 leaves the `replaces` job named `replaces`** though it now runs three
  guards (`check-replaces.sh`, `gofmt -s`, `check-base-images.sh`). Renaming it
  changes a check name and could break required-status-check configuration, so
  it was not done unattended. It wants to be `guards`.
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
- **A CI guard added on the agent's own initiative, easy to drop.** You
  declined a squashed-subtree guard, so this one is named rather than assumed
  welcome: `.github/scripts/check-replaces.sh` also fails when a `go.mod`
  requires a service this repository contains under its old upstream path. It
  has earned its keep twice — `hilt/itest/go.mod` on #3, `ingot/itest/go.mod`
  on #10, both nested modules a sweep over the service's own `go.mod` misses.
  One commit (`3b4c4d8e`), reverts cleanly. #16 and #17 each add one more
  guard in the same spirit.

## Needs access the agent does not have

- **Branch deletion returns HTTP 403** from the git proxy — both
  `git push origin --delete` and the `:refs/heads/…` refspec form — while
  ordinary pushes to the same remote succeed. Left for a human rather than
  routed around.

  **Verified fully contained in `main` (`da029c51`) as of 02:55Z, safe to
  delete now:** `claude/bring-in-swarf`, `claude/ci-concurrency`,
  `claude/indexer-from-head`, `claude/monorepo-todo`,
  `claude/upstream-findings`.

  **Not contained, so look before deleting:** `claude/major-decisions` and
  `claude/subtree-resync` (each may hold commits that reached `main` only as
  content), and `claude/pin-guppy`, which is
  [#2](https://github.com/fil-forge/forge-2/pull/2), closed unmerged.

  **Live, must stay:** `claude/bring-in-forgectl` (#12),
  `claude/ci-permissions` (#15), `claude/pin-base-images` (#16),
  `claude/pin-stragglers` (#17).

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
