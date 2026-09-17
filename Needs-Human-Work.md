# Needs human work

**Updated 2026-09-17 02:05Z.** Everything on this page is waiting on a person —
either because it is a judgement call, or because the agent cannot perform the
action. Work that is merely unfinished does not belong here; see
[[Current State]] for the broad picture and [[Consolidation Findings]] for why
each item exists.

Kept current as things move. Items leave via **Recently cleared**, which is
pruned once it stops being useful.

## Blocking

Nothing else proceeds until these do.

- **The `git push --force-with-lease` needed to rebuild #12 is denied.** The
  auto-mode classifier refuses it as `[Git Destructive]`, after four such
  pushes went through earlier the same night. There is no non-force route: a
  rule 7 rebuild *replaces* the branch's history by design, and GitHub will
  not let a PR's head branch be swapped for a different one. So #12 cannot be
  brought up to date until this is approved once, or a Bash permission rule
  for force-pushes to `claude/*` is added. The rebuild itself is done and
  verified, waiting in a worktree.

## Open pull requests

Two, and they are **not** a stack — both sit on `main` (`ecb70114`).

| | what | state |
|---|---|---|
| [#14](https://github.com/fil-forge/forge-2/pull/14) | **`ci.yml` gains a `concurrency` block**, matching the other three workflows, so its runs stop piling up on a re-push. One file, six lines. | Opened 01:50Z; Petra left a review 01:53Z that the agent **could not read** — see *Waiting on Petra* below. |
| [#12](https://github.com/fil-forge/forge-2/pull/12) | **Bring `forgectl` in** — the tenth and last in-scope module. Also widens delegator's Docker context to the repository root, because delegator links it in non-test code. | Reviewed and approved in substance ("This looks great. Just waiting for the stack."). **Stale and unrebuilt on purpose** — held until #14 merges so it is rebuilt once. Blocked on the force-push above. |

**Nothing is left to import.** `MAJOR_DECISIONS.md` named indexing-service and
forgectl as the last two pending; indexing-service landed with #10 and
forgectl is #12. Do not start another module without saying so.

## Waiting on Petra

- **The #14 review is unreadable from here.** A review in state `COMMENTED`
  was submitted at 01:53:14Z against `435e202e`, with no body — so the content
  is in inline comments, and that endpoint returns `API rate limit already
  exceeded for user ID 2407` (it has done this before; the limit is hourly).
  Recorded rather than guessed at. Pasting the comment is the quickest fix.
- **Two offers left open on #14**, both deliberately kept out of its diff:
  - **Fold in `permissions: contents: read`.** `ci.yml` is the only workflow
    without a `permissions:` block, so it inherits the repository default.
    Different concern from run cost, so it should be judged separately. Two
    lines.
  - **Record the CI path-filtering question in `MONOREPO_TODO.md`.** CI runs
    every job for every change, on purpose, and #8 — one Markdown file — still
    cost `itest` 26m27s. It needs a decision, not a quiet fix. See
    [[Current State]] Known debt for the shape of the argument.
- **A CI guard added on the agent's own initiative, easy to drop.** You
  declined a squashed-subtree guard, so this one is named rather than assumed
  welcome: `.github/scripts/check-replaces.sh` now also fails when any
  `go.mod` requires a service this repository contains under its old upstream
  path. It has since earned its keep twice — `hilt/itest/go.mod` on #3 and
  `ingot/itest/go.mod` on #10, both nested modules a sweep over the service's
  own `go.mod` does not reach. One commit (`3b4c4d8e`), reverts cleanly.

## Needs access the agent does not have

- **Branch deletion returns HTTP 403** from the git proxy — both
  `git push origin --delete` and the `:refs/heads/…` refspec form — while
  ordinary pushes to the same remote succeed. Left for a human rather than
  routed around.

  **Verified fully contained in `main`, safe to delete now:**
  `claude/bring-in-swarf`, `claude/indexer-from-head`, `claude/monorepo-todo`,
  `claude/upstream-findings`.

  **Not contained, so look before deleting:** `claude/major-decisions`,
  `claude/subtree-resync` (each may hold commits that reached `main` only as
  content), and `claude/pin-guppy`, which is
  [#2](https://github.com/fil-forge/forge-2/pull/2), closed unmerged — keep or
  drop as you prefer.

  Live and must stay: `claude/bring-in-forgectl` (#12),
  `claude/ci-concurrency` (#14).
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
