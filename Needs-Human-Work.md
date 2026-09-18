# Needs human work

**Updated 2026-09-17 20:35Z.** Everything on this page is waiting on a person —
either because it is a judgement call, or because the agent cannot perform the
action. Work that is merely unfinished does not belong here; see
[[Current State]] for the broad picture and [[Consolidation Findings]] for why
each item exists.

Kept current as things move. Items leave via **Recently cleared**, which is
pruned once it stops being useful.

## Blocking

**`main` is RED, and it is root-caused: the `ingot` itest suite does not
reliably fit in its 25-minute timeout.** Petra supplied the job log, which the
agent could not reach. The sharpest evidence is not in the log at all — it is
that **the identical tree passed at 21m12s and timed out at 25m09s, six minutes
apart.**

`itest ingot` on `991633b0` ended with `panic: test timed out after 25m0s` —
Go's whole-binary timeout from `.github/workflows/itest.yml:251`,
`go test -count=1 -v -timeout 25m ./...`.

**Nothing hung.** At the alarm, twelve top-level tests had run; eleven passed
in sequence over 19 minutes (14:27:06 → 14:46:06), and `TestForgeVersity` was
6 minutes into its own subtests, each completing in 0.2–3s. The subtest named
in the panic, `DeleteObject_nested_dir_object`, had been running **0s**. The
suite was making progress the whole way; it simply ran out of budget.

**The numbers, and they are close.** Three measured runs of the same step,
against a 25-minute budget:

| commit | tree | `itest` step began | duration | |
|---|---|---|---|---|
| `24b18ee5` (`main`) | | 09-17 20:34 | **22m53s** | green |
| `8897b972` (#9 head) | `051ff34b` | 09-18 **14:20:58** | **21m12s** | green |
| `991633b0` (`main`) | `051ff34b` | 09-18 **14:26:57** | **25m09s** | **red** |
| `a032558f` (#10 head) | | 09-18 15:03 | **21m54s** | green |

**The two middle rows are the same tree.** `991633b0` is the merge commit of
#9, and `git rev-parse` gives both the same tree object,
`051ff34b9dc6539fb31b16805c0a0a468c2d8136` — byte-identical code, identical
workflow, six minutes apart, different runners. One finished in 21m12s. The
other did not finish at all.

So this is **not** "the suite grew past 25 minutes." It is: the same work
costs anywhere from 21m12s to over 25m, and **25m sits inside that band**. The
workflow's own comment says it was ~21m30s when 25m was chosen; the floor has
barely moved, but the spread now crosses the line.

It also clears both #9 and #10: the same tree as the red run went green, and
#10, which is that tree *plus* its own diff, went green too.

**What causes the 3m57s swing is NOT established**, and an earlier version of
this page said "depending on which runner it lands on" as though it were. That
was shorthand for the variable that has not been identified, and it should not
have been written as a finding. What is actually known:

- Each job gets a fresh ephemeral VM. The four `itest ingot` jobs ran on four
  distinct runner IDs (`1000031610`, `1000032094`, `1000032112`, `1000032209`),
  all labelled `ubuntu-24.04`.
- **On paper they are identical.** The red job's own log reports runner image
  `ubuntu24/20260907.300`, `Ubuntu 24.04.5 LTS`, `CPUs: 4`,
  `Total Memory: 15988 MB` — the documented standard public-repo shape.
- **Ruled out:** image pulls during the test window. Zero pull/extract lines
  between 14:26:57 and 14:52:06.
- **Also ruled out: a retry or health-check cycle.** This page briefly floated
  it as "bug-shaped and fixable". It is not, and the check is cheap. The
  readiness waits are `wait.ForHTTP` / `wait.ForListeningPort` with
  `WithStartupTimeout(2*time.Minute)` and **no `WithPollInterval`**
  (`smelt/pkg/stack/stack.go:280-293`), and testcontainers-go v0.44.0 defaults
  that interval to **100 ms** (`wait/wait.go:63-65`). A missed window therefore
  costs 0.1s. Across 11 boots × 9 services that is a couple of seconds at the
  outside — it **cannot** account for 4 minutes. Nothing changed between the
  two runs, and nothing needed to.

**What the boot actually spends its time on.** From the red log, the first
test is 24.8s to compile and mount the ingot binary, then **70.1 seconds with
nothing logged at all** (14:27:31.04 "Connected to docker" → 14:28:41.13
"Creating container"), then the S3 endpoint is live 0.8s later. That silence is
`docker compose` bringing ~9 containers up with its output not forwarded —
confirmed by the window being literally empty, and by the only compose lines
anywhere in the job being `docker version` plugin listings from the build
steps. So it is real CPU and disk work, just invisible.

**What is left is the unexciting candidate: the machine.** Roughly two fifths
of the run is container startup, it is the most host-sensitive thing in the
suite, and it runs on a shared cloud VM. That is still a *candidate*, not a
finding — a green `itest ingot` log would confirm it, since machine speed
predicts a **uniform** ~19% slowdown across every test. The agent cannot fetch
one: the API returns only ~14.5 kB of trailing log and the artifact blob host
is blocked by the egress proxy, so it needs the zip from the run page, the way
the red one arrived. It would confirm rather than change the picture, so it is
worth a minute of Petra's time, not an hour.

**Worth naming, because it is the opposite case.** versitygw's self-imposed 3s
`lockWaitTime` (`tests/integration/utils.go:2654`, 38 call sites ≈ **114s of
pure sleep**) sits inside `TestForgeVersity` — the test that ran out of budget.
That cost is *fixed*, so it explains why the suite is long, not why it varies.
3s → 1s takes ~76s off **every** run. It lands in versitygw, outside the
agent's repo scope, and arrives here as a pin bump — a third lever, independent
of the two options below.

### Where the 25 minutes go

Read out of the same log, so it is measurement rather than estimate — but
note it is the **slow** run, so every absolute number below is the top of the
21–25 minute band, not the middle. The suite is **strictly sequential** — `grep -c 't.Parallel()' ingot/itest/*.go` is **0**
— and each top-level test calls `forgeStack(t, …)`, which boots the whole smelt
Forge stack (sprue + piri + indexer + postgres + …) in Docker.

| test | wall clock | breakdown |
|---|---|---|
| `TestForgeVersity` | **≥5m59s** | unfinished — ran last, out of budget |
| `TestForgeEncryption` | 349.5s | 298.4s in subtests, 51.1s not |
| `TestForgeDeleteReleasesNetworkBlob` | 171.4s | no subtests |
| `TestForgeAWSCLI` | 125.0s | no subtests |
| `TestForgeScenarios` | 118.5s | 66.9s in subtests, 51.6s not |
| `TestForgeMultipartExpiryShred` | 93.9s | no subtests |
| `TestForgeReadAfterCatalogRetention` | 59.4s | no subtests |
| `TestForgeDeferredMultipart` | 58.3s | **2.2s in subtests, 56.1s not** |
| `TestForgeCopyAuthorization` | 57.3s | no subtests |
| `TestForgeReadAfterEviction` | 55.4s | no subtests |
| `TestForgeNativeProvision` | 52.0s | no subtests |
| `TestForgeMaxSizePart`, `TestForgeS3Compat` | skipped | |

Ten completed tests = **19m01s**. Plus Versity's 5m59s = **24m59s** against a
25m00s budget. It did not overshoot by a lot; it overshot by a second, on the
last test.

**Three things follow, and they bear on the choice:**

1. **Roughly two fifths of the run is stack boot.** The three tests whose work is in
   subtests each carry 51.1s, 51.6s and 56.1s that no subtest accounts for —
   the same number three times, which is the fixed cost of booting and tearing
   down a stack. `TestForgeDeferredMultipart` is the clean case: **58.3s to run
   2.2s of subtests.** Extrapolated across the 11 tests that boot one, that is
   roughly **9.5 minutes, ~39% of the suite**, spent starting Docker stacks.
   (Measured three times; extrapolated to the other eight, which have no
   subtests to separate boot from work.)
2. **`t.Parallel()` is not the cheap way out.** Parallel top-level tests would
   mean several full Forge stacks on one runner at once. Worth knowing before
   anyone suggests it as a one-line alternative.
3. **A 2-way shard falls out almost balanced.** `{Versity, Encryption}` is
   11.8 min and everything else is 13.2 min — the two longest tests are 47% of
   the suite. That is the split, if sharding is the answer.

**Two things in the repo point the same way:**

- **`ingot/Makefile:28` uses `-timeout 30m`.** CI is five minutes stricter than
  the repo's own local target, so a contributor running `make` would never see
  this.
- **The workflow forbids retrying, deliberately:** *"Never retried,
  deliberately: a retry here would mask a flaky suite, which is the one thing
  this job exists to see."* So the re-run the agent was holding would have been
  against stated repo intent — and would not have been a diagnosis anyway.

**Not #9's doing.** Its diff was four `.goreleaser.yaml` files, a shell script,
two lines of `ci.yml` and `MONOREPO_TODO.md`; none is read by `itest.yml`.

### The decision, which is Petra's

The 25m is a *reasoned* number, not an accident — the same comment explains it
is deliberately below the job's `timeout-minutes: 45` so the Go timeout fires
first and dumps goroutines, "where a runner kill gives nothing". So changing it
is a decision, not a mechanical fix.

What the same-tree measurement changes: the budget has to clear the **slowest**
run, not the typical one. Judged that way —

- **Raise it to 30m**, matching `ingot/Makefile`. One line. Preserves the
  fire-before-the-job ordering (30 ≪ 45). Against the worst run measured
  (25m09s) that is ~19% headroom; against the best (21m12s), ~42%. Papers over
  the variance rather than removing it, and the next slow runner eats into it
  again.
- **Shard the suite** — which is exactly what the closed #21 was. `{Versity,
  Encryption}` against everything else splits 11.8 / 13.2 min at *this* run's
  speed, so even a slow runner leaves each shard around 9–11 minutes clear of
  25m. Removes the variance problem rather than out-running it, and costs more
  runner-minutes — the trade Petra already weighed when she closed #21.

Neither touches the real cost, which is that **two fifths of the run is
booting Docker stacks** (below). That is a third option, and a much larger
one: share a stack across the tests that do not need isolation.

`main` stays red until one of them lands. The patch for the first is ready to
push on request.

## Open pull requests

One: **[#10](https://github.com/fil-forge/forge/pull/10)** — the
`subtree-conflicts.sh` conflict reader and its `AGENTS.md` procedure, asked for
after the dead-file approach was rejected, then redirected from predicting
conflicts to reading them. **Green on all four workflows** (22 checks) at
`a032558f`; waiting on review. `swarf` has none.

Two things were fixed on it after that green run, neither of which CI could
see:

- **`AGENTS.md` said `## Conventions## Conventions`** on one line (`62af49c7`).
  The heading stopped being a heading, so every convention under it read as
  part of the subtree section. **No CI job reads a markdown file** — `guards`
  runs the shell scripts under `.github/scripts/`, and that is the closest
  thing this repo has to a linter for prose. It was caught by reading the PR's
  own rendered diff.
- **The description still described the design that was replaced**, including
  a claim the second commit disproved: *"git does follow a pure move,
  including a file hoisted out of a prefix."* It does not — that came from a
  toy repository whose move emptied the prefix. Rewritten, with the wrong
  sentence struck rather than deleted, since it is the reason the first design
  looked reasonable.

Both of the last two merged on 2026-09-18:

- **[#9](https://github.com/fil-forge/forge/pull/9)** — merged 14:20Z as
  `991633b0`. Three commits: the goreleaser `-X` ldflag fix and its guard, the
  corrected rationale, and the Phase 1 note in `MONOREPO_TODO.md` on replacing
  the lint with a release-time assertion. Merged while its CI was still
  running, which the PR's own skippable-checks block had called: only `guards`
  was load-bearing, and it was green. **`main`'s post-merge run is the thing to
  watch.**
- **[`swarf` #17](https://github.com/fil-forge/swarf/pull/17)** — merged 13:51Z
  as `f286fb0`, carrying `a50b142` (the firehose fix) and `406cbe0` (the
  `internal/sse` extraction).

**Two consequences of the swarf merge are still live work for a person:**

- **The subtree pull's one ordering constraint is satisfied** — it can now
  bring the firehose fix in. Still Petra's to authorize; a subtree pull is a
  rule 7 operation.
- **The pin bumps for upstream `hilt` and `ingot`.** The version they move to,
  derived and then **confirmed against the module proxy** rather than
  hand-computed:
  `github.com/fil-forge/swarf v0.0.1-0.20260918135142-f286fb01aa10`, replacing
  `v0.0.1-0.20260821142121-d5d1a0a56f00`. `go get github.com/fil-forge/swarf@main`
  resolves to exactly that. Both repos are outside this session's scope.

**What #9's review settled, worth keeping:** a release workflow would *not*
have caught the stale ldflags loudly. `addstrdata` in
`src/cmd/link/internal/ld/data.go` returns early on a missing symbol, on absent
type info and on an unreachable one, and only `Errorf`s when the symbol exists
but is not a string; `go version -m` records the flag either way; and
`pkg/build` reads `version.json` by *relative* path at runtime, so the same
stale-ldflag `sprue` binary reports `v0.0.6` from a checkout and `v0.0.0` from
a container. All measured. The replacement — run the released binary and
require it to report the tag — is recorded in `MONOREPO_TODO.md`, along with
the fact that only `piri` and `ingot` can be asked their version today.

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
