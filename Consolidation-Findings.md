# Consolidation findings

A running log of what the polyrepo→monorepo consolidation has turned up.

Its bias is deliberate: **things that were already broken but looked fine**,
and **places where a test was believed to cover something it did not**. A
consolidation is unusually good at finding these, because it forces every
implicit assumption about how the repositories relate to each other to become
explicit, and most of them turn out to be wrong in some small way nobody had
occasion to notice.

Entries cite `file:line` at the commit where they were found. Last updated 2026-09-11.

---

## Part 1 — Blind spots

Cases where CI was green and a human would reasonably have said the thing was
tested. These are the expensive ones: a latent bug costs a day, a blind spot
costs however long it takes to stop trusting the signal.

### B1. Nothing in the monorepo ever read a Dockerfile

`ci.yml` built, vetted, tidied and tested seven Go modules. None of that opens
a `Dockerfile`. The stack test (`e2e.yml`) booted containers, but under
`SMELT_WORKSPACE`, which compiles each service on the host and bind-mounts the
binary over the *published* image — so it tested HEAD's **code** inside the
polyrepo's **packaging**.

The consequence is that every packaging fault in this repository was invisible
by construction, and two of them were real (→ L1, L4).

Closed by `images.yml`, which builds all six in-repo images from HEAD, and by
the e2e job now running the stack on those images with `SMELT_WORKSPACE` off.
**Both halves matter**: leaving the workspace mount on alongside built images
would shadow the image's own binary and re-open exactly this hole.

### B2. A test opted out of the thing the job existed to test

`smelt/tests/e2e/snapshot_test.go` called
`MustNewStack(t, stack.WithEmbeddedSnapshot(...))` and nothing else. Image
overrides and workspace binaries are set *only* by explicit options —
`workspaceBinaries` has one writer, `WithWorkspaceBinaries()`
(`smelt/pkg/stack/options.go:192`), and there is no env-based default in the
harness.

So `TestStackFromSnapshot` ran the published `:main` images, start to finish,
inside a CI job whose entire purpose was to exercise HEAD. It passed. In the
log it is indistinguishable from a real pass, and it was reported as one.

The mechanism that let it drift: the env→option mapping was five hand-written
lines in `smoke_test.go` covering two services of eleven. A second test simply
didn't have them.

Closed by `stack.OptionsFromEnv()`, appended by both tests.

> **The general shape.** Opt-in coverage with no way to detect the opt-out.
> Worth looking for anywhere a test takes options that decide *what version of
> the system* it runs against — the failure is silent and the log looks right.

### B3. Re-importing upstream silently reverted a downstream fix

piri cannot be built `CGO_ENABLED=0` without `-tags skiff`, which selects
curio's FFI-free variants; without it the link fails on `ffi.GenerateSynthProofs`,
`ffi.SealCommitPhase1` and `supraffi.HasUsableCUDAGPU`.

Old `forge` knew this — `f60dd59` had added a `buildTags` field to
`pkg/workspace` for exactly this reason. The rebuild used `git subtree add` at
each service's current upstream tip, and upstream smelt had never carried that
field. **A fix that had already been made arrived undone**, and re-surfaced as
a build failure in the first stack run.

There is no general guard for this. The lesson is narrow and worth stating
plainly: when a monorepo is rebuilt from upstream tips, every fix that lived
only in the old monorepo is silently dropped, and the ones you notice are the
ones that break loudly. Assume there are quiet ones.

Pinned by regression tests: `TestPiriBuildsWithSkiff`,
`TestBuildArgsOmitsEmptyTags`.

### B4. Per-service agent docs describe a repository layout that no longer exists

`ingot/CLAUDE.md` explains that container builds need no `GOWORK=off` because
"CI checkouts and the Docker build context don't contain the parent `go.work`
(it lives above the repo)". In a monorepo `go.work` is at the repository root,
so that reasoning is void.

The conclusion survives, but for a different reason — the Dockerfiles' `COPY`
lists are narrow and never copy `go.work` — which is now stated in the
Dockerfiles themselves rather than left as an inherited assumption.

This is a class, not an instance: these files are read by agents and humans as
authority, and five of them still declare pre-rewrite module paths (→ L2).

### B5. Verdicts depend on other repositories' daily builds

Every first-party service in the stack is named by a mutable tag:
`ghcr.io/fil-forge/guppy:main-dev`, `…/indexing-service:main`, `…/swarf:main`,
`…/did-method-plc:main`, `…/filecoin-localdev:main`,
`…/storetheindex:latest`.

`guppy/.github/workflows/publish-ghcr.yml:125` republishes `:main-dev` on every
push to guppy's `main` — **39 commits in 90 days**, so that pointer moves
roughly every other day. CI runners are ephemeral and have no image cache, so
every run pulls whatever the tag means that morning.

Not theoretical: the earlier findings document records that *runs 35 and 36
disagreed on two stacks for this reason alone*.

Still open. See O1.

### B6. "The branch is green" meant one workflow of three

Three workflows run on every push here — `ci`, `e2e`, and now `images`. A
branch was reported green on the strength of its `e2e` run while `ci` had been
failing on it, and stayed failing across three further commits before anyone
looked.

The `ci` failure was not a code fault — `proxy.golang.org` returned
`stream error: stream ID 3; INTERNAL_ERROR` fetching
`github.com/labstack/echo/v4@v4.13.4`, so the job died during dependency
download, before a test body ran. But the failure being benign is luck, not a
process.

The recurrence was not luck either: see [L9](#l9-the-go-module-cache-was-never-on--fixed).
The module cache had never worked in this repository, so every job
re-downloaded its whole dependency graph every run — far more exposure to a
per-request failure than anyone intended.

Two habits fall out, and they are cheap:

- Read the **run list for the commit**, not the run you were waiting for.
- A module-download error is *not* a test result. It should be visibly
  distinguishable from a real failure, because it will be treated as one
  otherwise — or worse, dismissed as one.

---

## Part 2 — Latent issues found

Real defects that existed before the work started, or were introduced by it
and caught.

### L1. The module-path rewrite never reached non-Go files — **fixed**

Consolidation rewrote `github.com/fil-forge/<svc>` →
`github.com/fil-forge/forge/<svc>` across Go sources and `go.mod`. It did not
touch anything else.

Two Dockerfiles name their build target by absolute module path:

```
delegator/Dockerfile:18             go build … -o /app github.com/fil-forge/delegator
piri-signing-service/Dockerfile:18  go build … -o /app github.com/fil-forge/piri-signing-service
```

Both had been **unbuildable since consolidation**, under a green CI, for the
reason in B1.

Fixed by pointing both at `.`, not at the corrected absolute path — every
other service Dockerfile already uses a relative target, `smelt/pkg/workspace`
already uses `"."` for both services, and a relative target cannot break again
at the next path change, which is coming when `forge-2` takes the `forge`
name.

### L2. Thirteen further stale module paths — **open, inert**

Same root cause as L1, no build impact:

| where | what |
|---|---|
| `piri`, `hilt`, `ingot`, `delegator`, `piri-signing-service` `AGENTS.md`/`CLAUDE.md` | declared module path |
| four `README.md` | `git clone` / `go install` URLs |
| `piri/docs/mkdocs.yml` | `repo_url` |

Deliberately deferred: some describe repositories that still exist and will not
be wrong until the rename. Worth one sweep at rename time, not before.

### L3. `Dockerfile.release` has L1's problem and is untested — **open**

`hilt`, `ingot` and `sprue` each carry a `Dockerfile.release` driven by
goreleaser. They have the same build-context problem the main Dockerfiles had,
and nothing builds them, so nothing will say so until someone cuts a release.

Untouched because there is no way to verify a fix without running goreleaser.
This is a landmine with a lit fuse; it belongs to the Phase 1 release work.

### L4. Signer built on a Go toolchain older than its own module — **fixed**

`piri-signing-service/Dockerfile:2` specified `golang:1.25-bookworm` while
`piri-signing-service/go.mod` declares `go 1.27.0`. Introduced when the
pin-unification `go mod tidy` raised every module's go directive; caught only
once something built the Dockerfile (B1).

Depending on `GOTOOLCHAIN`, that is either a silent toolchain download inside
every image build or an outright failure.

### L5. Base images in our own Dockerfiles float — **open**

`alpine:latest`, `debian:bookworm-slim`, `golang:1.27-bookworm`. Same class as
B5 but inside code we own, and the cheapest of all of these to close.

### L6. Third-party stack images pinned inconsistently — **open, and now live**

Most are version-pinned (`postgres:16-alpine`, `openbao/openbao:2.6`,
`grafana/tempo:2.4.0`, `redis:7-alpine`). Two are not:
`amazon/dynamodb-local:latest` and `minio/minio:latest`
(`smelt/systems/common/compose.yml:14,47`). Arguably worse than B5, since
nobody here controls when they move.

**This stopped being hypothetical within hours of being written.** The first
CI run to boot the stack on HEAD-built images failed before a single
container started:

```
failed to create stack: compose up: Error response from daemon:
pull access denied for minio/minio, repository does not exist
or may require 'docker login'
```

All three e2e tests, same cause, `docker ps -a` empty. Cause not yet
established — a Docker Hub anonymous-pull limit consumed by base-image pulls
earlier in the same job is the leading hypothesis, but the wording is
401-shaped rather than the `toomanyrequests` a rate limit usually produces,
and Docker Hub could not be queried directly from the investigating
environment.

Whatever the cause, the exposure is the same: **an unpinned third-party tag
can take the entire stack offline, and did.** Pin both to real version tags.

### L7. ucantone hides why a token was not a receipt — **open, upstream**

`container.decodeTokens` discards the `receipt.Decode` error, so every
decode failure surfaces as `missing receipt for task` or `conclusion receipt
not found` against a server that logged a `200`. This is what made the
ucantone #49 straddle so expensive to diagnose. Cross-repo; wants an issue on
ucantone regardless of any layout decision.

### L8. smelt's Docker-dependent tests fail rather than skip — **open**

`smelt/pkg/stack/build_test.go` shells out to `docker build`. Without a
daemon it **fails**; hilt's stated convention is the opposite — its
`AGENTS.md` records that testcontainers-backed tests "skip when Docker is
unavailable", so `go test ./...` stays meaningful without one.

Consequence: a plain `go test ./pkg/...` in smelt is red for any developer
without Docker, which trains people to ignore a red suite. Same family as the
rest of this document — a result that says something other than it appears to.

### L9. The Go module cache was never on — **fixed**

`actions/setup-go` defaults to `cache: true`, and every job in this repository
had it on. None of them ever used it.

setup-go resolves `go.sum` from the **repository root**. This is a
multi-module repo: every module carries its own `go.sum` and there is no root
one. So every job logged

```
##[warning]Restore cache failed: Dependencies file is not found in
/home/runner/work/forge-2/forge-2. Supported file pattern: go.sum
```

and then ran with no module cache whatsoever. Ten jobs, two triggers, each
re-downloading its entire dependency graph on every run, since the repository
was created.

**Why it stayed invisible:** it is a *warning*. Nothing turns red, nothing is
annotated, and the job still passes — just slower, and with far more network
exposure than intended. The only symptom is one line in a log nobody reads
when the job is green.

**What it cost.** This is the cause behind [B6](#b6-the-branch-is-green-meant-one-workflow-of-three).
`proxy.golang.org` intermittently drops an HTTP/2 stream mid-transfer
(`INTERNAL_ERROR`), and the go command does not retry. Seven occurrences to
date across four modules and four jobs — `unit ingot` (build), `unit smelt`
(`go mod tidy`), `unit piri` twice, and `e2e`, which died at *compile* time
with `[setup failed]` before a container started. They kept landing on piri
because piri's graph (lotus, curio, specs-actors, boxo) is the largest here,
so it drew the most chances to lose.

Note the shape: the dead cache is not the *cause* of any single failure —
proxy.golang.org is — but it set the failure rate. A defect that changes only
a probability is the hardest kind to attribute, because every individual
instance has a complete and correct explanation that isn't it.

**Fixed** by `cache-dependency-path: <module>/go.sum` on each step, plus a
retry around dependency resolution only — `go mod download` up front and
`go mod tidy`, which reaches further because it walks the test dependencies
of dependencies. `go test` is deliberately never retried: that would mask a
flaky test, the one failure the job exists to show. Verified sufficient by
checking that a module builds with `GOPROXY=off` once `go mod download` has
run, so build, vet and test touch no network at all.

`.github/scripts/check-setup-go-cache.sh` asserts it stays on.

**Residual — which stopped being residual within the hour.** The first fix
left `go mod download` inside the Dockerfiles unretried, on the grounds that
it had never failed there. It failed there twenty minutes later, on #2's
`e2e`:

```
process "/bin/sh -c cd piri && go mod download" did not complete successfully
```

Now retried inline in the five Dockerfiles where the download is fatal. piri
is the one that broke and the most exposed of the seven: alone among them it
has neither a BuildKit module cache mount nor `|| true`. hilt and sprue
tolerate a failed download and refetch during `go build`, so they are left
alone and remain exposed at that later step.

The lesson is about the estimate, not the code. "Not yet observed" was
treated as "unlikely" on the same path that had just produced seven failures
in a day. On a failure mode you have just proved is live, an unfixed instance
is not residual risk; it is the next one.

### L10. The module-path rewrite silently unformatted 89 files — **fixed**

`gofmt` sorts imports within each blank-line-delimited block.
`github.com/fil-forge/forge/...` sorts before `github.com/fil-forge/libforge/...`
— `f` before `l` — and the two sit in one contiguous block, so rewriting the
module paths changed what `gofmt` wants in every file that imports both. Nothing
re-ran it.

`gofmt -l` reports **89 files**: 80 pre-existing on `main` from the original
seven-service rewrite (64 in `sprue`, 12 in `piri`), and 9 introduced by the
swarf migration (6 swarf, 3 hilt).

**CI cannot see this.** `ci.yml` runs build, vet, tidy and test; `go vet` does
not check formatting, and nothing else does. Unlike the other entries here,
this is not a job that ran and proved less than it appeared to — it is a check
that does not exist.

Copilot found the 9 in the migration's diff, exactly and with no false
positives, which is worth noting given how much of its other output on that
pull request was pre-existing upstream code rather than migration damage.

**Fixed:** all 89, together with a `gofmt` step in `ci.yml`. The two had to
land in one commit — adding the check first turns every branch red. The diff
is 134 insertions and 134 deletions, every changed line an import; all five
affected modules still compile.

This is the **fourth** thing the rewrite broke outside Go import paths, after
[L1](#l1-the-module-path-rewrite-never-reached-non-go-files--fixed)'s
Dockerfiles, a `Makefile`'s `-X` ldflags and a repository URL. The first three
were things the rewrite failed to reach. This one it reached and changed
correctly — the breakage is that a *correct* edit invalidated a derived
property nobody recomputed.

### L11. Fixing the instance, not the class — three times in one afternoon

Not a defect in the repository; a defect in how the repository was being
fixed. Recorded because it happened three times in a row, each time with the
remaining instances found by someone other than the person fixing it.

1. **Docker module retries.** piri's unretried `go mod download` failed in CI
   and was fixed. The same commit message observed that hilt and sprue
   "remain exposed at that later step" — and left them. A reviewer flagged
   hilt within the hour. A guard written in response then failed immediately
   on swarf, whose Dockerfile carried the same `|| true` and had never been
   looked at.
2. **Stale counts in workflow comments.** A review flagged `e2e.yml`'s header
   as saying six services when it built seven. The number was updated and the
   file was not grepped for other counts; the *same file* had a second stale
   count nine lines from the build commands. A later review flagged that one.
3. **The module-path rewrite**, across the whole project: Dockerfiles, then a
   `Makefile`'s ldflags, then a repository URL, then import ordering
   ([L1](#l1-the-module-path-rewrite-never-reached-non-go-files--fixed),
   [L10](#l10-the-module-path-rewrite-silently-unformatted-89-files--9-fixed-80-open)).
   Each was fixed as it surfaced; the class was never swept.

The shape is always the same, and it is not carelessness about the fix — each
individual fix was correct and tested. It is that *finding* a defect and
*characterising* it are different acts, and only the second one tells you
where else it lives. A fix applied without the second act is a coin flip on
whether the reviewer or the guard finds the rest.

What actually worked, both times it was tried: writing the check. `gofmt -l`
turned "nine files Copilot flagged" into "89, of which 80 predate this work".
`check-dockerfile-retry.sh` found swarf on its first run. In both cases the
mechanical sweep found instances that careful reading had missed, immediately.

### L12. A guard over part of a chain — **fixed**

`check-image-lists.sh` existed to prevent silent green, and had a silent-green
hole in it.

The path from "this commit's image" to "the stack ran it" has three links, and
the inventory is stated once per link: `images.yml`'s matrix, `e2e.yml`'s build
list, and `e2e.yml`'s job `env:` as `*_IMAGE` variables that
`pkg/stack.OptionsFromEnv` reads. The guard compared the first two and stopped.

Deleting `UPLOAD_IMAGE` therefore left the check green while compose fell back
to the published sprue image and the suite passed — *the exact failure the
script's own header describes*. The third link is also the easiest to break,
because the variable names are not derivable from the service name (`sprue` →
`UPLOAD_IMAGE`, `piri-signing-service` → `SIGNER_IMAGE`), so nothing about a
mismatch looks wrong on the page.

Fixed: the check now follows the chain to the end, including that each variable
is one `envImageOptions` actually reads, parsed out of `options.go` rather than
restated — a guard that hardcoded the list would be the same bug one level up.
Verified against all four ways to break the chain, not just the original two.

**The general form is worse than a missing guard.** A partial guard reads
exactly like a complete one: same name, same green tick, same sentence in the
PR description. Nobody re-derives its coverage afterwards, so it converts "we
have not checked this" into "we have checked this" at no cost and with no
signal. Ask of any guard what it would have to *observe* to catch the fault,
and then whether it observes that.

**Then the guards themselves were reviewed, and three more were hollow.** An
independent review session found that `retry.sh` exited 0 without running the
command at all when `RETRY_ATTEMPTS` was `0` or non-numeric (`seq 1 0` prints
nothing, the loop never runs, the script falls off the end) — which in the
tidy step leaves the following `git diff --exit-code` passing trivially;
that `check-dockerfile-retry.sh` tested for the substring `sleep` on one
physical line, so `RUN sleep 1 && go mod download` passed with no retry at all
while a correct retry written across continuations was *rejected*; and that
`check-setup-go-cache.sh`'s text scan was blind to a quoted `uses:` value and,
on a four-space `steps:` list, ran past the end of the step to be satisfied by
a **different job's** setting.

All three were guards written that same afternoon to prevent silent green,
and all three went green while observing nothing. The rewrites now parse the
YAML, join Dockerfile continuations and require the download to appear twice
(a retry is repetition, which is a property of the thing rather than its
spelling), validate the attempt count, and — generalising the sharpest of the
observations — **fail when they find zero subjects at all**, since in this
repository zero means the scan broke rather than the tree being clean.

Found in review. Two sibling findings in the same review were the same shape,
one level down — comments that had gone false:

- Both workflows' `concurrency` keys included `github.sha` for push events,
  making every run's group unique, so `cancel-in-progress` had nothing to
  cancel. `e2e.yml`'s comment directly above said "one stack at a time per ref
  … overlapping runs starve each other's healthchecks." The code had never done
  that.
- `ci.yml`'s header said "No Docker". Adding swarf made it false in the same
  commit that added a matrix comment *explaining* swarf's Docker dependency in
  detail, nine lines below. The local note was written; the global claim it
  contradicted was not read.

---

## Part 3 — Things that turned out better than expected

Worth recording, because they changed decisions.

### G1. Image builds are cheap — measured, not assumed

Caching was designed and then deliberately skipped, on the reasoning that a
cache hit means the build did not run, and the goal was to provoke failures
early. The measurement vindicated skipping it:

| image | time |
|---|---|
| piri (links curio — the feared one) | **2m14s** |
| ingot | 1m27s |
| hilt | 1m24s |
| sprue | 1m13s |
| delegator | 1m03s |
| piri-signing-service | 1m02s |

Six in parallel, under 2.5 minutes wall-clock. There is no caching problem to
solve. The design is preserved in O2 in case that changes.

### G2. `SMELT_WORKSPACE` collapsed a perceived prerequisite

The stack was believed to need Dockerfiles in the monorepo before it could
test HEAD at all. It does not: the workspace path builds binaries on the host
and mounts them over published images, so the stack could be brought up
against HEAD's code long before any image work. That is what made the first
green stack run possible.

Its limit is precisely B1 — which is why images replaced it in CI rather than
joining it.

### G3. The binary's dependency closure is narrower than `go.mod` says

`go.mod`'s `replace` list is not the set of modules an image needs.
`go list -deps` on the build target is:

```
piri  → delegator, piri-signing-service
ingot → hilt
hilt  → (nothing)
```

`smelt` appears in both hilt's and ingot's `go.mod` as a `replace`, but is
imported only from `itest/` — no smelt code is linked into either binary.
Copying just `smelt/go.mod` and `go.sum` is enough to resolve the module graph,
verified by building each service from a staged context with no smelt source
present.

**The lesson is not "derived is narrower" — it is "derived is right".** The
swarf migration proved the other direction. Bringing swarf in made hilt's and
ingot's closures *wider* than they had been:

```
hilt  -> hilt, swarf
ingot -> hilt, ingot, swarf
```

swarf is linked into both binaries (the revocation client, the SSE firehose
consumer), so both Dockerfiles needed its source and not just its go.mod.
Assuming instead of running `go list -deps` would have broken both images,
and the failure would have surfaced as a missing package deep inside a
dependency, naming nothing about swarf.

So: **derive dependency lists, never hand-maintain them.** A derived list is
sometimes narrower and sometimes wider than the one you would have written;
what matters is that it cannot go stale, in either direction. Re-derive on
every migration, not once.

---

### G4. A separate Claude session as reviewer — worked, with a known limit

Copilot's review quota ran out mid-afternoon, after 13 reviews that had caught
a great deal (L1's Makefile ldflags, L10's gofmt drift, the stale counts, the
`|| true` downloads). Replacement: a Claude Code session spawned into the same
environment with no access to this session's context, told only the repository
and the PR numbers, and instructed to post findings directly to the PRs.

**Seven findings, all verified against the code, zero false positives.** Four
were defects in guard scripts written that same afternoon (L12). One found a
year-long `immutable` cache header on a mutable revocation route. One
established that the e2e snapshot test now always restores three-week-old state
and says nothing about it.

What made it work was not cleverness but **where the output goes**. A subagent
reports back to the author, who then relays — and most of these findings were
criticisms of that author's own work. Posting straight to the PR removes the
filter. It also got a clean checkout, so it ran real commands: every mechanical
claim arrived with a reproduction rather than a suspicion.

Two things worth knowing before relying on it:

- **Correlated blind spots are real and unfixable this way.** Where an error
  came from carelessness — not grepping for the class, not reading the header
  nine lines up — a fresh reader catches it. Where it came from how the model
  reasons, another instance of the same model may reason identically. Copilot
  being a *different* model was part of its value.
- **Watching what it investigates is a signal in itself.** Its task summary
  ("confirming which Docker-dependent tests are untagged") prompted a check
  that found a wrong claim before the session posted anything at all. That was
  luck rather than design, but it is worth knowing it happened.

One finding was wrong in its mechanism while right to raise: a warning that a
digest pin would expire to registry pruning. Investigating it found no pruning
(no cleanup action, no retention pattern across six org packages) but did find
that guppy's dependabot auto-merges use `secrets.GITHUB_TOKEN`, which triggers
no workflow run — so `ghcr.io/fil-forge/guppy:main` does not reflect guppy's
`main`. A wrong alarm that leads somewhere real is still worth the trip.

---

## Part 4 — Open items

### O1. Pin what we do not build; build what we do

The right reference for anything in the monorepo is "the product of this
commit", not a pinned digest of someone else's build. That splits B5 three
ways:

| | services | answer |
|---|---|---|
| in the repo | piri, sprue, hilt, ingot, delegator, signer | **build from HEAD** — done |
| coming in | indexer, swarf, guppy | build from HEAD once they land; pin meanwhile |
| never in the repo | plc, filecoin-localdev, storetheindex, dynamodb-local, minio | digest pin, permanently |

guppy sits in the middle row, not the bottom: the plan's Phase 4 is
"Consolidate the client, archive `guppy`".

### O2. Content-addressed image caching — designed, deliberately not built

Recorded so it is not re-derived from scratch when G1 stops being true.

Do **not** use path filters — that is the plan's own Trap #1 (silent green),
and its failure mode is skipping work that was needed. Use a content address,
whose failure mode is rebuilding when you did not need to:

```
key(svc) = sha256( git rev-parse HEAD:<svc>
                 + git rev-parse HEAD:<dep> for dep in go-list-deps closure
                 + base image digests )
```

Tag `:src-<key>`, check the registry, skip on hit. Both inputs are derived —
git tree hashes for content, `go list -deps` for the closure — so neither can
go stale. Base images must be digest-pinned first (L5) or the content address
lies.

**It must skip the build, never the test.** The suite runs every time; this
caches an artifact, not a verdict.

Expected hit rate, from G3's closure: a `smelt/`-only change rebuilds
**nothing**, and neither does a root-file change — provided the Dockerfiles
keep narrow `COPY` lists rather than `COPY . .`.

### O3. Sweep the per-service agent docs

B4 and L2 are the same sweep. Best done at rename time, when the clone URLs
become wrong too.

---

## Working principles that fell out of this

1. **A green check is a claim about what ran, not about what is correct.** Ask
   what the job would have had to *do* to catch the fault. B1 and B2 both
   passed every check that existed.
2. **Test what you ship, not a reconstruction of it.** A bind-mounted binary
   in a published image is not the artifact that reaches production.
3. **Derive lists; never hand-maintain them.** `go list -deps` over a
   `replace` list; git tree hashes over path filters. Derived lists are
   narrower and cannot drift.
4. **Prefer relative references to absolute ones** in anything that survives a
   move. `.` over `github.com/fil-forge/delegator` — the second broke at
   consolidation and would have broken again at the rename.
5. **Measure the thing you are about to optimise.** Caching was designed for a
   build-time problem that turned out not to exist (G1).
6. **When a fix is missing, ask whether it was ever made.** B3 was not an
   oversight; it was a regression introduced by re-importing upstream.
7. **A warning is a behaviour you asked for and did not get.** If you depend
   on it, assert it. L9 sat in plain sight in every green log for the life of
   the repository.
8. **"Not yet observed" is not "unlikely"** on a failure mode you have just
   proved is live. The Docker-side module fetch was called residual exposure
   and failed within the hour (L9).
9. **When you fix one, grep for the rest before you commit.** Characterising
   a defect is a separate act from finding it, and only the second tells you
   where else it lives (L11). Prefer a command that enumerates the class to
   reading carefully for it.
10. **A partial guard is worse than none**, because it reads as complete and
   nobody re-derives its coverage (L12). Ask what it would have to observe.
11. **When you add a local note, read the global claim it might contradict.**
   The header nine lines up is where the stale version lives (L12).
12. **A count can be wrong without being stale** — it can have counted the
   wrong event from the start. "39 pushes to guppy's main in 90 days" was a
   true count of *commits*, offered as a count of *publishes*; guppy's
   dependabot auto-merges use `secrets.GITHUB_TOKEN`, which triggers no
   workflow run, so the tag had not moved in 24 days. Check what the number
   counts, not only when it was taken.
13. **Don't write counts in prose.** "The other three take their own
   directory" is a derived value nothing recomputes; state the rule instead
   and let the list below it be the answer (L11).
14. **A correct edit can invalidate a derived property.** The module-path
   rewrite was right; import *order* was computed from the old paths and
   nobody recomputed it (L10). Ask what else was derived from what you just
   changed.
15. **Some defects only change a probability.** They are the hardest to
   attribute, because every individual failure already has a complete and
   correct explanation that is not them (L9). "This failure was a transient"
   and "our setup makes transients frequent" are both true at once.
