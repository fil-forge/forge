# Needs human work

**Updated 2026-09-17 20:35Z.** Everything on this page is waiting on a person —
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

One that matters: **[`swarf` #17](https://github.com/fil-forge/swarf/pull/17)**,
upstream. `forge` has no open PRs at all, and
[#28](https://github.com/fil-forge/forge-2/pull/28) on `forge-2` is superseded
(see below) — it can be closed.

**#17 took a review and grew a second commit** (`406cbe0`). Petra's note: the
duplicated buffer limits and the comment explaining the duplication were the
wrong shape. The duplication ran deeper than the constants — `cmd/swarf` and
`pkg/client` had the *same* SSE loop — so the framing moved to a new
`internal/sse`, whose `Scanner` reads events the way `bufio.Scanner` reads
lines. Both call sites collapse to the one step that differs. Two behaviours
of the old loop were checked rather than assumed and written down there: an
event carrying no data is not dispatched (which the event stream format
requires, not an accident), and fields other than `event`/`data` are ignored —
including the `id:` line `pkg/fx` writes before every revocation. #27 merged as `24b18ee5`, so the postgres healthcheck fix and
its guard are live, and `e2e` has now passed twice on the fix (still weak
evidence: two passes had a ~90% chance even unfixed). #25 and #26
merged, so the layer cache is live on `main` (`95e83665`) and its numbers are
recorded: **23s warm** against a 7m34s baseline, the `e2e` job 13m10s → 5m29s.
Three caveats travel with that, on [[Current State]] and in #26: the warm run was
a same-commit re-run so it is the **ceiling not the average**, the cold path is
~3 min *worse*, and **the cache size has not been checked against GitHub's 10 GB
limit** — that one needs `gh cache list` or the Actions cache API, neither of
which the agent can reach from here.

**Each one now opens with a block naming the checks that are safe to merge
without waiting for** — see [[Current State]] for why, and what makes it more
than a feeling.

| | what | state |
|---|---|---|
| [#28](https://github.com/fil-forge/forge-2/pull/28) | **swarf's firehose client dropped oversized events and then hung** — default 64 KiB scanner cap, `ErrTooLong` discarded, so `Stream` reconnected at the same cursor forever. hilt and ingot both link it in production. | `52648c29`, **22/22 green**. **Now also open upstream** as [`fil-forge/swarf` #17](https://github.com/fil-forge/swarf/pull/17) (`a50b142`, **5/5 green**) — the identical patch, re-verified in both directions against upstream's own tree. That was the part that mattered: upstream `hilt` and `ingot` pin a swarf version that has the bug, so #28 alone fixes the copy nobody deploys. **Still yours:** review and merge swarf #17, then a pin bump in each consumer — the same shape as the versitygw `lockWaitTime` item. **Three sibling findings are left deliberately**: the `immutable` cache on a mutable route, the memory-vs-PostgreSQL `Get` divergence, and the 10s settle window. Each is a **contract decision** — I would rather you chose than have me encode one. |

**No check-name change is outstanding.** #16's `replaces` → `guards` is merged,
live, and confirmed resolved (2026-09-17). #21 would have added a second
(`itest ingot` → `itest ingot 1/3`…) but is closed, so that one returns only if
the branch is revived. #27 adds a *step* to the existing `guards` job, which
changes no check name. Worth re-reading this line before enabling required
checks.

**Nothing is left to import**, and nothing should be started. `MAJOR_DECISIONS.md`
records what is deliberately out.

## Waiting on Petra

**Two decisions, both created by the move to `forge` (2026-09-17 20:21Z).**

- **Archive `fil-forge/forge-2`, or delete it?** *Recommend archive.* This
  wiki links 11 distinct `forge-2` PRs (#2, #15–#17, #19–#22, #25, #27, #28)
  in 16 places, and `main`'s own merge commits name those numbers in their
  messages. The links cannot be repointed at `forge`: its PR numbering is
  separate, **#2 already collides**, and `forge` will climb into the rest.
  Deleting `forge-2` turns the whole record into 404s and leaves the merge
  messages pointing at other people's PRs; archiving costs nothing and keeps
  every link live.
- ~~Where does #28 land?~~ **Decided: it does not.** The swarf fix arrives
  with the final subtree pull once
  [`swarf` #17](https://github.com/fil-forge/swarf/pull/17) has merged, so
  nothing local diverges inside `swarf/`. #28 is superseded and can be closed
  whenever convenient. **The ordering is the thing to remember: #17 must merge
  before the pull**, or the pull brings the hang and a second pull is needed.

**All five earlier items were approved 2026-09-17** — the
`replaces` → `guards` rename confirmed resolved, #17's digest reuse, #17's edits
inside subtree prefixes, #17 shipping no Go image guard, and the four guard
scripts that were the agent's own initiative.

The reasoning that outlives the approval has moved to [[Current State]] rather
than sitting in a list that gets pruned: the digest-reuse practice is now
recorded under rule 2 as its worked example, and Known debt carries both the
standing cost of pinning inside subtree prefixes and the guards' provenance.
The measured case against a Go image guard — 367 matches broadly, 8 when
narrowed and still missing two real references — was already there.

**New decisions land here as they are taken.** The pattern that has worked:
take the reversible one, flag it on the PR that makes it, and list it here to be
confirmed rather than assumed.

## Subtree drift, and what to pull early

**The polyrepo is not frozen** — ~45 commits across six services since the
imports; `sprue`, `libforge` and `ucantone` all committed today. Full table on
[[Current State]]. Policy (Petra, 2026-09-17): no regular pulls, **one final
pull at the end**, but pull early where upstream fixes something we have hit.

**One row meets that bar now: `smelt` `96fc212`, "postgres and openbao boot
issues".** It is the same bug #27 fixed, found upstream two days earlier, and it
fixes a *second* boot race we have not hit yet — `ingot-openbao-init` writing
before raft elects a leader. The two fixes are complementary and do not
conflict. **Pulling smelt (4 commits) is a recommendation awaiting your call**;
an agent-initiated `git subtree pull` is a rule 7 operation and not something to
start unasked.

`ingot` is **25 behind**, including #166 — CI itest sharding, the same work as
closed #21. It arrives with the final pull regardless.

**The final pull will not silently regress the image pins** — an earlier note
here said it would, and that was wrong. Tested with `git merge-file` on the real
three versions: all three files conflict loudly (piri 1, sprue 1, smelt 2), the
digest-bearing lines survive the merge, and even resolving to *theirs* is caught
by `staticcheck` U1000 on the orphaned const and helper. Nothing to do here.

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
  `claude/ignorable-checks` (#22).

  **Closed, but do not delete:** `claude/shard-itest` at `e0205346` — #21,
  closed unmerged for simplicity. It holds a green, measured sharding
  implementation and the only copy in git of the measurement behind it, so
  reviving it is a reopen rather than a rebuild. Deleting the branch is what
  would turn that into a rebuild.

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

- **⚠️ forgectl's mainnet metrics need a `mainnet` environment and two secrets
  on this repository, before `fil-forge/forgectl` is archived.**
  `metrics-payments.yaml` (every 30 minutes) and `metrics-faults.yaml` (every
  12 hours) run `forgectl metrics payments`/`faults` against
  `environment: mainnet` and push to an OTLP endpoint, taking the payer address
  and endpoint from repository secrets. **Nothing is broken now** — they run in
  the polyrepo, which still exists; the copies imported here never ran and #24
  deletes them. **The hazard is the archival step**: whenever that repository is
  archived or its workflows disabled, mainnet fault and payment metrics stop
  silently — a dashboard goes flat and nothing fails. Configuring an environment
  and secrets is a person's job, so this is here rather than in the TODO alone.
  Sequence it *with* the archival. (No equivalent risk among the seven services
  pruned earlier: checked every file `7321ee6a` deleted, and the only
  `schedule:` keys were dependabot intervals.)

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

- **All five "Waiting on Petra" items approved** (2026-09-17): the
  `replaces` → `guards` rename resolved, #17's reuse of digests already in the
  tree, #17's edits inside subtree prefixes, #17 shipping no Go image guard,
  and the guard scripts that were the agent's own initiative. The durable
  reasoning moved to [[Current State]]; the approval itself is what is recorded
  here.
- **#21 closed unmerged** (16:21Z), sharding `itest ingot`, on Petra's call:
  "closed it for simplicity. We can revive it later if we want." It was green
  and measured — 30m43s → 23m07s, ~25% — but the trade was ~43% more
  runner-minutes, four `itest` jobs of flake surface instead of two, and a
  check-name change to remember. I had recommended keeping it, on the argument
  that the rename is free only while no check is required; that argument lost
  to simplicity, which is a reasonable place for it to lose. Branch survives at
  `e0205346`.

  **The measurement outlived the PR and is on [[Current State]]**, including the
  two levers it turned up that need no sharding at all — the missing buildx
  layer cache in `itest`/`e2e`, and the self-imposed 3-second `lockWaitTime` in
  our versitygw fork. Those two are the live work now, and **they are not yet in
  `MONOREPO_TODO.md` on `main`** — they were born on the closed branch.
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
