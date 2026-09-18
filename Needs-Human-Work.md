# Needs human work

**Updated 2026-09-17 20:35Z.** Everything on this page is waiting on a person —
either because it is a judgement call, or because the agent cannot perform the
action. Work that is merely unfinished does not belong here; see
[[Current State]] for the broad picture and [[Consolidation Findings]] for why
each item exists.

Kept current as things move. Items leave via **Recently cleared**, which is
pruned once it stops being useful.

## Blocking

**Nothing. `main` (`0d8fb04c`) is green on all four workflows** — `ci`, `e2e`,
`images` and `itest`, as of 19:20Z. Both PRs merged; the sharding is measured
and working.

**But the indexing-service race is NOT fixed.** `ci` passed because
`-test.shuffle` reorders every run and this one was kind. Petra's call was to
**fix it upstream**, on the principle that upstream is still the source of truth
for the services themselves — that work is delegated and in flight against
`fil-forge/indexing-service`, not this repo.

**Standing rule from that decision, worth keeping:** when a problem turns up in
imported code, ask *first* whether it belongs upstream, and fix it there
whenever it makes any sense. The bug here was byte-identical to upstream's
except the import path, so it was never the monorepo's to fix.

### What a correct diagnosis of that race needs

Recorded because the first one did not survive contact with a test. The claim
was "`poller.Stop()` races the final `Delete`". **Falsified:** the unfixed test
with a 50 ms sleep *inside* the `Delete` mock hook still passed — the mock
records a call on entry, so delaying inside `Delete` never stops it counting.
300 runs of the unfixed test under `-race -shuffle=on` also passed on a fast,
idle machine.

The window must be **between the cache call returning and the handler reaching
`queue.Delete(ctx, job.ID)`**, not inside `Delete`. The poller's handler
(`go-ipni-tools .../queue/poller.go`, ~line 239) runs `s.handler(...)` then
`queue.Delete(...)` sequentially per job. The experiment that was never run: in
the unfixed test, `wg.Done()` **first** and then sleep in the cache hook — the
original's `defer wg.Done()` delays the signal too, so the `defer` has to go.

## Open pull requests

**None.** Both merged today:

- **[#11](https://github.com/fil-forge/forge/pull/11)** 17:04Z as `9870d48a` —
  itest sharding.
- **[#10](https://github.com/fil-forge/forge/pull/10)** 19:08Z as `0d8fb04c` —
  `subtree-conflicts.sh`, which finishes the merge a subtree pull could not
  follow. Final interface, Petra's call: **merging is the default**,
  `--dry-run` prints real `git diff` to stdout and writes nothing, notes go to
  stderr, **exit 1 if anything was left for a human**. Built for an agent
  mid-pull, not a person reading a report.

**Still open on #10's subject, and not built:** the post-merge audit for
upstream deletions of files we moved. When upstream deletes a file we hoisted
out of a prefix, both sides deleted the path, so git raises no conflict at all
and the tool never sees it. Recoverable from `MERGE_HEAD`'s diff, but only by
something that runs when nothing conflicted.
## Waiting on Petra

- ~~Archive `fil-forge/forge-2`, or delete it?~~ **Done — archived 20:45Z.**
  Which is what the record needed: this wiki links 11 distinct `forge-2` PRs
  in 16 places and `main`'s merge commits name those numbers, and the links
  could not have been repointed (`forge`'s numbering is separate and **#2
  already collides**). Archiving keeps them all live. Side effect worth
  knowing: `forge-2` is read-only, so #28 stays frozen as *open* and cannot
  be closed — the archive banner is the signal, not its state.
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

## A second flake class: the testcontainers Ryuk reaper

**New on 2026-09-18 at 16:33Z**, on #10's `29972ce6`. `unit piri` went red:

```
run minio: generic container: create container:
  reaper: from container "5d98a414": wait for reaper 5d98a414: context deadline exceeded
```

It took down `TestObjectStore/minio` and panicked `TestMain` in
`piri/pkg/store/objectstore/minio`. Ryuk is testcontainers' own cleanup
sidecar; the failure is in starting it, before any test body ran.

**Established as not #10's, rather than assumed:**

- The diff from the green parent `c098486b` to the red `29972ce6` is **one
  file**, `.github/scripts/subtree-conflicts.sh` (+33/−5).
- That script is **referenced by no workflow at all** (`grep subtree-conflicts
  .github/workflows/` → nothing). It is a manual tool.
- `ci` run 47 on `c098486b` was **green**, eighteen minutes earlier, same job.

**The one re-run has been spent, and it came back GREEN** (`rerun_failed_jobs`
16:36Z, attempt 2 green at 16:38Z on the identical commit). That is the
confirmation the re-run existed to get: a flake, not a defect, and #10 is green
again at `29972ce6`. Note the "never retried, deliberately" rule is
`itest.yml`'s and does not cover `ci.yml`; and the reaper died before a test
body ran, which is the other case that justifies a re-run.

**Count this one separately from the postgres flake.** Same discipline, derive
rather than remember: `ci.yml` run 48 attempt 1 is the only `unit piri` failure
in 48 runs, and attempt 2 of that same run passed. One occurrence is not a rate.

**Worth connecting, since it is now a pattern rather than an incident.** Both
of today's non-code failures are container *startup*, not test logic: `itest
ingot`'s timeout was 11 full stack boots at ~52s each, and this is a single
sidecar failing to come up. The `TestForgeVersity` anomaly on #11 points the
same way — it ran far faster as the first test on a fresh runner than as the
thirteenth on a used one. **If container startup is the fragile part of this
CI, the shared-stack item in `MONOREPO_TODO` stops being an optimisation and
starts being a reliability fix.** That is a hypothesis with three
circumstantial supports and no direct measurement yet.

## Still counting: the `e2e` postgres flake

Not blocking anything — recorded so nobody declares it fixed from memory. **Count
it by derivation, not recall:** `e2e.yml`'s **run 1 in this repository is the
merge of the fix itself** (`24b18ee5`, #27, "Make `pg_isready` probe TCP"), so
*every* `e2e.yml` run in `fil-forge/forge` is a post-fix trial. List them and
count.

As of **15:45Z on 2026-09-18 that is 8 runs, all green.** At a 5% base rate,
eight straight passes still had a **~66%** chance with the bug untouched
(`0.95⁸`). That is not a verdict either. The earlier tallies on
[[Current State]] ("a fourth pass", "~81%") were correct when written and
counted runs from before the transplant; this section is the one to trust, and
it is cheap to recompute.

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
