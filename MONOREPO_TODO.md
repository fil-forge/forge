# Monorepo TODO

Two kinds of thing the consolidation turns up and cannot deal with where it
finds them.

**Questions for the whole repository**, below, which only make sense once the
monorepo is built and can be looked at as a whole. Each needs a team decision
rather than an implementation. Add to it when a branch turns up a question it
cannot answer by itself: say what was found, why it is not decidable yet, and
what someone would have to choose — not what you would do.

**Findings in the imported code**, at the end: bugs noticed while moving a
service in, which are not the migration's to fix. Add to it when a review of
an import turns up something that was already true upstream.

Neither is a place for ordinary work, which belongs in issues. What is
blocking or waiting on a person right now lives on the wiki's
**Needs Human Work** page.

---

# Questions for the whole repository

## Restore the s3-compat report pipeline

Upstream `ingot` publishes an S3 compatibility report to GitHub Pages
(`2365944c`, [ingot#131](https://github.com/fil-forge/ingot/pull/131)). The
consolidation brought the test suite across and left the reporting behind:
there is no s3compat step here, no `s3-compat-report` artifact, and no `pages`
job anywhere in the repository.

**Why it waits.** A report pipeline has to publish *somewhere*, and where that
is depends on decisions the monorepo has not made yet — whether services keep
per-service Pages sites or share one, and what the release flow looks like
once Phase 1 adds tags and publishing. Rebuilding ingot's version now would
bake in the polyrepo's answer to a question the monorepo gets to ask fresh.

**The choice.** One Pages site per service, one shared site, or drop the
published report and keep the suite's own output.

## Turn Renovate on

`renovate.json` is in the repository as of
[#6](https://github.com/fil-forge/forge-2/pull/6), covering the 18 images the
stack pulls plus the Go and Actions dependencies. It does nothing until the
Renovate GitHub App is installed on the organisation.

**Why it waits.** Installing it is an org-level action with org-level
consequences: it opens PRs across every repository it is granted, so the
scope, the schedule, and who triages the PRs are decisions for whoever owns
that surface — not for the branch that happened to write the config.

**Until then**, the pins are frozen rather than maintained. That is a real
cost and a deliberate one: a frozen pin is still reproducible, which an
unpinned tag is not, and #6 existed because a mutable `:main` tag broke a
conformance test.

**The choice.** Install it and decide the scope, or adopt something else
(Dependabot covers Docker and Go but reads `docker-compose` files less
willingly), or keep bumping by hand and accept the drift.

## Narrow hilt's Docker build context again

`hilt/Dockerfile` builds from the repository root, because hilt links swarf
through `replace => ../swarf` and Go resolves every replace target before it
downloads anything — a context scoped to `hilt/` dies in `go mod download`.
piri and ingot are the same. Recorded in the Dockerfile itself.

**Why it waits.** Narrowing it is not a Dockerfile change. An in-repo module
reached by a `replace` always lives outside `hilt/`, so no restructuring
helps — not even extracting an `internal/client/swarf`, which would just be a
different sibling directory. The only thing that narrows the context is
consuming swarf as a published, tagged module through the proxy, which is
Phase 1 work and gives up same-commit co-development in exchange.

**Agreed 2026-09-16**: keep it as it is for now, resolve before the
consolidation is finished. So this one has a decision already; what it needs
is Phase 1 to happen.

## Decide whether to cover smelt/systems/stress-tester

It is a module in its own right — `smelt/systems/stress-tester/go.mod`, with
its own `Dockerfile`, `compose.yml` and `cmd/` — and nothing builds, vets or
tests it. smelt's own `go build ./...` does not reach it because it is a
separate module, it is absent from `go.work`, and no workflow references it.
It is the only Go in this repository in that position.

**Why it waits.** It is not a check that was dropped: upstream's `go-check`
walked every `go.mod` in a repository, so this module lost its coverage at the
consolidation, when the root workflow started enumerating modules by name.
[#9](https://github.com/fil-forge/forge-2/pull/9) briefly added it and it was
taken back out, because restoring what was removed and covering something that
was never covered are two different changes.

It is also not free. Its `go.mod` says `go 1.24.0` where every other module
says `1.27.0`, which is below staticcheck's own minimum — adding it to the
matrix is what turned #9 red, and the fix had to move the toolchain source
from the job's module to `go.work`.

**The choice.** Add it to the matrix and to `go.work` and keep it building —
noting that adding it to `go.work` shifts workspace-mode resolution for every
other module (measured: `modernc.org/cc/v3` 3.40.0 → 3.41.0, `modernc.org/ccgo/v3`
3.16.13 → 3.17.0, plus a gorm and sqlite subtree) — or decide it is a
development tool that does not need CI, and say so where someone will find it.

It is clean on build, vet, tidy, test and staticcheck as it stands, so
whichever way this goes, it is not currently broken.

## Decide what to do about the macOS test run

Each service's own CI ran its tests on macOS as well as ubuntu: the shared
go-test workflow defaults to `["ubuntu", "windows", "macos"]` and every
service's config skipped only Windows. The monorepo runs ubuntu only, and
[#9](https://github.com/fil-forge/forge-2/pull/9) restored the other four
lost checks while deliberately leaving this one alone.

**Why it waits.** It is not clear the macOS jobs were ever green. Several
modules' tests boot containers through testcontainers, and GitHub's macOS
runners have no Docker daemon — so either those jobs were failing upstream,
or something not visible from here supplied one. Reproducing a job that was
already red buys nothing, and macOS runners bill at a higher multiplier than
ubuntu, so this is not free to find out by trying.

**The choice.** Establish what those runs actually did — one look at a recent
`Go Test` run on any of the eight upstream repositories settles it — then
restore macOS, restore it only for the modules that do not need Docker, or
decide ubuntu-only is what the monorepo wants and say so.

## Turn `SA4006` back on

`staticcheck.conf` at the repository root says `checks = ["inherit",
"-SA4006"]`, added by [#10](https://github.com/fil-forge/forge-2/pull/10). It
is off everywhere, for one finding in one generated file.

**What fires.** `indexing-service/pkg/service/queryresult/json_gen.go:231`.
`dag-json-gen` emits `written++` after each field and guards the *next* field
with `if written > 0 { WriteComma() }`, so the increment after the last field
has no reader. A true positive, and the JSON is correct.

**Why it waits.** Three ways out, none of them this branch's:

- **Fix the generator.** The durable answer, and it is upstream:
  `github.com/alanshaw/dag-json-gen`, pinned at `v0.0.9`, which is also the
  latest published version — so there is no newer release to take instead.
  Someone has to open that PR.
- **Pin staticcheck back to what the polyrepo ran.** Not available.
  indexing-service was `go 1.25.7`, so the shared workflow's version table gave
  it 2025.1.1 and its `Go Checks` at `bcb63ec` was green. Unifying the libforge
  pin moved it to `go 1.27.0`, and both older versions — 2025.1.1 (`v0.6.1`)
  and 2026.1 (`v0.7.0`) — fail on *every* package with `export data version 4
  is greater than maximum supported version 2`. Only 2026.2.1 reads Go 1.27
  export data, and the libforge pin requires `go >= 1.27.0`. Verified by
  installing both.
- **Narrow the suppression instead of widening it.** staticcheck's config is
  directory-scoped and walks up, so a `staticcheck.conf` in
  `indexing-service/pkg/service/queryresult/` would cover one directory rather
  than thirteen modules. That is where it started, and it was moved out: that
  directory is inside a subtree prefix, so a file there is permanent local
  divergence every future `git subtree pull` of indexing-service has to carry,
  and it would still cover the three hand-written files beside `json_gen.go`.
  A narrower scope for a permanent conflict is a real trade, not an obvious
  one.

**The cost of leaving it.** SA4006 fires exactly once across thirteen modules
today, so nothing is lost yet. What is lost is future findings: a genuine dead
assignment written tomorrow, anywhere in the repository, goes unreported. The
longer this stays, the less true "nothing is lost" gets.

**The choice.** Fix `dag-json-gen` upstream and drop the conf; or narrow it to
the one directory and accept the subtree divergence; or decide SA4006 is not
worth carrying and say so here rather than leaving the conf reading as
temporary.

## Decide whether CI should run only what a change affects

Nothing in `.github/` filters by path: no `paths:`, no `paths-ignore:`, no
changed-files detection. Every push runs all four workflows and every job in
them. `ci.yml`'s own header says why:

> Unfiltered by path on purpose: one job per module and no filter list means a
> new shared module cannot fall out of one and go silently green.

**What it costs**, measured rather than estimated. [#8](https://github.com/fil-forge/forge-2/pull/8)
changed **one Markdown file**. Its last push still ran:

| workflow | wall clock |
|---|---|
| `images` | 7m00s |
| `ci` | 8m40s |
| `e2e` | 9m57s |
| `itest` | **26m27s** |

26 minutes and 22 jobs for a documentation edit, and that repeats on every
push to every branch.

**Why it waits.** The obvious mechanism is the wrong one, in two separate
ways:

- **A hand-written `paths:` list is the same silent-green shape this
  consolidation keeps deleting.** The itest suites compiled behind a build tag
  and discarded, `SMELT_WORKSPACE` masking broken Dockerfiles, the indexer
  pulled from a published digest — each was green because something was not
  looked at. A filter list that goes stale when a module gains a dependency
  fails the same way, and looks identical while doing it.
- **A skipped job never satisfies a required status check.** GitHub reports a
  path-filtered job as *skipped*, not *success*, so any such job that is also
  required blocks the merge permanently. The usual answer is to start the job
  always and exit early inside it — which means the useful version of this
  filters *what a job does*, not *whether it runs*.

**The shape a good answer probably has**, if it is worth doing: derive the
affected set from the module graph rather than typing it. Rule 3 already says
this — `go list -deps` on the build target, not `go.mod` — and it is how the
`ingot → indexing-service` and `delegator → forgectl` edges were found, both
of which `go.mod` alone did not show. A derived set cannot go stale when
someone adds an import; a typed list silently can.

**The cheap part is already done.** `ci.yml` had no `concurrency:` block while
the other three did, so its runs piled up on a re-push instead of cancelling;
that was fixed separately and is not what this entry is about.

**The interim, in place now.** Since no check is required yet, every pull
request opens with a block naming the checks a reviewer can merge without
waiting for, and why — a manual, per-push version of the same idea, recorded in
`AGENTS.md`. It has neither fault above: it cannot go stale, and nothing is
skipped so nothing reports *skipped*. Keep the blocks; when the derived gate is
designed, they are the worked examples of what it has to express.

**The choice.** Build the derived gate, accept the cost as the price of not
having a stale filter list, or find a third thing — perhaps splitting the
slowest suites onto a different trigger.

## Decide how much more to spend making `itest ingot` fast

Sharding took it from ~28 minutes to roughly half, without touching a test.
The rest costs something, and the something is different in each case.

**Where the time actually goes**, measured rather than assumed, from the job
log for a full run:

| | |
|---|---|
| the Go test binary | **1257s = 20m57s** (`ok …/ingot/itest 1257.018s`) |
| everything else | ~7 min — checkout, setup-go, vet, staticcheck, tidy, and 8 image builds |

Inside those minutes: **13 top-level tests (2 of them skip), 11 full stack
boots.** Separating boot from work needs a test whose work is all in subtests,
and three of them are: their totals exceed their subtests by 51.1s, 51.6s and
56.1s — the same number three times, which is the boot.
`TestForgeDeferredMultipart` is the clean case, **58.3s to run 2.2s of
subtests**.

So boot is roughly **9.5 minutes of ~25, about two fifths** — a large minority,
not the whole story. An earlier version of this entry said the subtests "run in
hundredths of a second" and that the suite "is not slow"; both are wrong.
`TestForgeEncryption` spends 298.4s inside its subtests and
`TestForgeScenarios` 66.9s.

Second measurement, separate from the above: **the same 8 images are built
three times per pull request** — once as `images.yml`'s parallel jobs, once in
`itest` per suite, once in `e2e`. 24 builds for 8 images.

**Done: shard by test.** The matrix splits ingot across three runners, with
each shard deriving its own tests from `go test -list` rather than from a
hand-written `-run` regex. Separate jobs get separate Docker hosts, which
`stack.CleanupLeaked` requires. Wall clock roughly halves. It costs
runner-minutes: each shard rebuilds all 8 images, so three shards build them
three times over.

**Not done, and each is a real choice:**

- **Build the images once and load them.** `images.yml` already builds all 8;
  `itest` and `e2e` could `docker load` from an artifact instead of
  rebuilding. That saves ~6 minutes in each of several jobs *and* removes the
  multiplier sharding just added. **The win is genuinely uncertain**: 8 images
  is likely 1–2 GB of artifact round-trip, which eats back some of it, and
  nobody has measured that. A registry would be faster, but `images.yml`
  deliberately takes no `packages: write` so that fork pull requests work —
  taking this path reopens a decision already made on purpose.
- **Share one stack across tests.** Nine of the thirteen call plain
  `forgeStack(t)` with no custom config; only four need their own
  (`config-retention.yaml`, `withSmallBlobConfig`, `withMultipartTTLConfig`).
  Booting once and sharing removes about eight boots — **the largest single
  win available, 8 to 16 minutes** — but it changes test isolation, needs
  per-test bucket and tenant namespacing, and a flaky itest is the one thing
  this job exists to catch. Real work, not a tidy-up.

**The choice.** Whether to spend runner-minutes to buy wall clock (the current
trade), spend engineering time on the shared stack instead (better return,
higher risk), or measure the artifact path first and decide with a number.
Worth noting the trend the sharding does not change: every service brought
in-repo adds an image build to this job, so the fixed ~6 minutes grows while
the variable part shrinks.

---

## Decide how much further to go on CI wall clock

**Measured, not estimated.** `itest` is the long pole at ~30 minutes.
[#21](https://github.com/fil-forge/forge-2/pull/21) built test sharding, ran it
green, and was closed unmerged for simplicity; branch `claude/shard-itest`
survives at `e0205346`, so reviving it is a reopen. Its A/B is the evidence
base for everything here — #19's unsharded run finished four minutes after
#21's sharded one, same runner pool, same hour:

| | unsharded | sharded (closed) |
|---|---|---|
| `itest` workflow | **30m43s** | 23m07s |
| slowest job | 29m29s | 17m07s |
| test binary | **1267.853s** | 628.841 + 462.731 + 204.654s |
| runner-minutes | **29m29s** | 42m04s |

Total test work was unchanged (+2.2%): nothing got cheaper, it got spread. The
trade on offer was ~43% more runner-minutes and four `itest` jobs of flake
surface instead of two, for ~25% less wall clock. Declined.

**Where the ~30 minutes actually goes**, from the job log: ~7 min of checkout,
setup-go, vet, staticcheck and tidy plus the 8 image builds, and ~21 min of
test binary. Inside the test binary, 13 top-level tests boot 13 full stacks;
`stack_test.go` logs the cost itself.

### Live options, in order

- **Layer-cache the image builds — done and measured, on
  [#25](https://github.com/fil-forge/forge-2/pull/25), 22/22 green.** `itest.yml`
  and `e2e.yml` used plain `docker build`, which cannot use
  `--cache-from type=gha` at all, so the absence of a cache was never a decision;
  they now use `docker/build-push-action@v6` with `type=gha` scoped per service,
  and `images.yml` stays cold as the canary on the same events.

  | all 8 images | |
  |---|---|
  | baseline, plain `docker build` | **7m34s** (`itest ingot`), 6m08s (`itest hilt`) |
  | cold + cache write | **11m21s** — ~3 min *worse* |
  | **warm** | **23s** |

  The whole `e2e` job went **13m10s → 5m29s**. Per service warm: piri 4s, hilt 5s,
  ingot 3s, sprue 2s, delegator 2s, piri-signing-service 2s, swarf 3s,
  indexing-service 2s.

  **Three caveats, none of them small.** The warm figure came from a **re-run of
  the same commit**, so every layer hit — that is the ceiling, not the average; a
  real change invalidates the `go build` layer for the services it touches, while
  the base, apt and `go mod download` layers (the bulk) still hit, so expect
  minutes rather than seconds. **Re-measure on a real source change.** The cold
  path is ~3 minutes worse because `mode=max` exports every layer, so the first
  run on `main` after merging is slower by construction. And **nobody has checked
  the cache against GitHub's 10 GB per-repository limit** — eight images at
  `mode=max` is not small, eviction is LRU, and an overflowing cache degrades
  quietly back to cold builds.
- **`lockWaitTime` is 3s in our own versitygw fork, and it is self-imposed.**
  `tests/integration/utils.go:2654` in
  `github.com/fil-forge/versitygw` (pinned `v0.0.0-20260914113944-a628e2cc628c`)
  sets `lockWaitTime = time.Second * 3`; `cleanupLockedObjects` sets
  `RetainUntilDate: time.Now().Add(lockWaitTime)` and then sleeps the same
  duration waiting for the lock it just created to lapse. Nothing external
  requires 3 seconds — the teardown picks the date.

  **38 call sites** across `Access_Control.go`, `CopyObject.go`,
  `CreateMultipartUpload.go`, `GetObject*.go`, `PutObject*.go`,
  `WORM_protection.go` and `versioning.go`, so at one firing each that is
  **~114s of pure `time.Sleep`** inside `TestForgeVersity`, the test that bounds
  the suite. 3s → 1s saves ~76s. **1s is the safe floor until someone checks**
  whether sub-second `RetainUntilDate` round-trips through ingot's object-lock
  path; second granularity is the conservative assumption, not a verified one.
  versitygw is a fork we control, but it is a different repository, so this
  lands upstream and arrives here as a pin bump.
- **Build the images once and load them** — `images.yml` already builds all 8
  and could hand them over as an artifact, removing the build from `itest` and
  `e2e` entirely rather than just caching it. Unmeasured, and the uncertainty is
  real: 8 images is likely 1–2 GB of round-trip. A registry would be faster but
  `images.yml` deliberately takes no `packages: write` so fork pull requests
  work. **Try the cache first** — done, and on the warm path it takes the build to
  23s, which is below anything an artifact round-trip could achieve. This option
  is effectively dead unless the cache turns out to evict often.
- **Sharding, if revived, should bin-pack against measured durations.** The
  closed implementation split by test *name order*, which assumes uniform cost:
  629s / 463s / 205s, where perfect balance would be 432s — 3m17s lost to
  imbalance alone. It must be derived from a previous run's timings, never a
  hand-written grouping, which is a list a new test falls out of silently.
- **Sharing one stack across tests is parked**, and the arithmetic that made it
  look attractive was wrong. Shard 3 ran four tests — four boots — in 204.654s,
  so a boot is at most ~51s, not the ~80s taken from 1257/13. It is worth less
  than the 3–4 minutes last estimated, and it changes test isolation in the one
  job that exists to catch flakiness.

**Not a lever: queue wait.** Sharding took it from 42s to 2m25s–4m47s, because
four concurrent jobs do not all get runners at once. It is the reason more
shards buy less than they look like they should.

---

## Reproduce forgectl's mainnet metrics jobs, before the polyrepo is archived

`forgectl` is the only imported service that had genuinely scheduled workflows,
and they are operational rather than CI:

| workflow | schedule | runs |
|---|---|---|
| `metrics-payments.yaml` | every 30 minutes | `forgectl metrics payments --payer … --otlp-endpoint …` |
| `metrics-faults.yaml` | every 12 hours | `forgectl metrics faults …` |

Both target `environment: mainnet` and push to an OTLP endpoint, taking the
payer address and endpoint from repository secrets.

**Nothing is broken today.** They run in `fil-forge/forgectl`, which still
exists; the copies that came in with the subtree never ran, because GitHub reads
workflows only at the repository root, and they have now been deleted.

**The hazard is the archival step.** Whenever `fil-forge/forgectl` is archived
or its workflows are disabled, mainnet fault and payment metrics stop, silently
— a dashboard goes flat and nothing fails. Reproducing them here needs a
`mainnet` environment and the two secrets configured on this repository, which
is a person's job, not an agent's. Sequence it with the archival rather than
after it.

The seven services pruned earlier carry no equivalent risk: the only
`schedule:` keys among every file that prune deleted were dependabot intervals.

---

## Phase 1 needs machinery this repository does not have

The consolidation plan says to "cut initial release tags … via the *existing*
`release.yml` flow". **There is no `release.yml`.** `main` has four workflows —
`ci`, `e2e`, `images`, `itest` — and none of them tags, releases or publishes.
The plan treated `forge` at `f60dd59` as a starting point that already had that
apparatus; this repository was built from subtree imports instead, and the
apparatus was never part of them.

What survives is raw material, and it is uneven:

| | have it |
|---|---|
| `version.json` | 8 of 10 — not `forgectl`, not `smelt` |
| `.goreleaser.yaml` | 4 — `indexing-service`, `ingot`, `piri`, `sprue` |
| `Dockerfile.release` | 4 — `hilt`, `ingot`, `sprue`, `swarf` |

**The `Dockerfile.release` files are goreleaser-shaped, not repo-shaped.** Each
is `FROM debian:bookworm-slim@…` plus `COPY <svc> /usr/bin/<svc>` — it copies a
*pre-built binary* from goreleaser's `dist/` context. So "nothing builds them"
cannot be fixed by adding a CI job; it needs the release flow to exist first.
Note also that `hilt` and `swarf` have a `Dockerfile.release` and **no**
`.goreleaser.yaml`, so those two are doubly orphaned.

**And one trap worth seeing before it bites:** at the monorepo root, `hilt` is a
*directory*. `COPY hilt /usr/bin/hilt` is unambiguous in goreleaser's `dist/`,
and copies an entire source tree if anyone ever builds that file with the
repository root as context — which is the context every other Dockerfile here
uses.

### Sequencing: tags are gated on the rename

Module paths are already `github.com/fil-forge/forge/*`, and a submodule tag has
to be `<svc>/vX.Y.Z` in the repository the module path names. A tag pushed to
`fil-forge/forge-2` sits where no module path resolves: `go get
github.com/fil-forge/forge/piri@piri/v1.2.3` looks in `fil-forge/forge`, the old
repository, and finds nothing. **Any tag cut before the rename has to be cut
again after it**, so the rename gates the tagging half of Phase 1.

The parts that do not: deciding the tag scheme, writing the release workflow,
`compat.yml`, and giving `forgectl` and `smelt` a `version.json` if the flow
wants one.

**`compat.yml` matters more than it did.** Moving `itest` to images built from
HEAD removed the only thing that was accidentally testing compatibility against
the deployed network.

### The release flow should assert the version, not just the build

`check-goreleaser-ldflags.sh` catches today's defect — four `.goreleaser.yaml`
files naming pre-consolidation module paths — by checking that every `-X` names
a package in the module that builds it. That is a check on the *path*. The
release flow should eventually check the *effect*, and then this lint can go.

The reason is that nothing else is loud. `cmd/link` looks the `-X` symbol up
and gives up silently when it is missing — `addstrdata` in
`src/cmd/link/internal/ld/data.go` returns early on `Lookup() == 0`, on absent
type info, and on an unreachable symbol, and only *`Errorf`s* when the symbol
exists but is not a string. The linker's own `doc.go` documents what `-X` does
when the variable is there and says nothing about a miss. `go version -m`
records the `-ldflags` argument either way, so the released artifact's build
info looks correct while the binary reports its fallback. Measured on `sprue`:
a binary built with the stale path reports `v0.0.0` from a container and
`v0.0.6` — `version.json`'s value, read by *relative* path at runtime — from a
checkout.

**The check itself is cheap**: after goreleaser, run the host-arch binary out
of `dist/` and require its output to contain the tag. Sub-second against a
cross-compile. **The cost is that only half the services can be asked:**

| | how to ask the binary its version |
|---|---|
| `piri` | `piri version`, and cobra's `--version` |
| `ingot` | `ingot version`, and `--version` |
| `sprue` | nothing on the CLI — `build.Version` only reaches `serverInfoHandler` |
| `indexing-service` | nothing on the CLI — consumed in `pkg/server` and `pkg/aws` |

So `sprue` and `indexing-service` want a `version` subcommand first, ~15 lines
each modelled on `piri/cmd/cli/version.go`. Worth having anyway: operators ask
binaries their version, and today `sprue --version` cannot answer.

**Do not shortcut it by grepping the binary for the version string.** That is
uniform without touching any service, and it tests the wrong thing — whether
the bytes are present, not whether the program reports them. `indexing-service`
shows why: its config injects a working `-X main.version={{.Version}}` *and* a
broken `-X …/pkg/build.version=v{{.Version}}`, so the string is in the binary
either way and only the `v` prefix separates them.

What the assertion buys over the lint: it also catches the linker's other
silent branches, notably a correct `-X` against a symbol that got dead-code
eliminated.

**Reference material** for all of this is `indexing-service`'s old per-repo
workflows — `releaser.yml`, `tagpush.yml`, `release-binaries.yml`,
`release-check.yml`, `publish-ghcr.yml` — which were inert here and have been
deleted; they are one `git show` away in this repository's history, and still
live in the polyrepo. Note `images.yml` deliberately takes no `packages: write`
so that fork pull requests work, so publishing needs its own workflow or a job
split rather than a flag on that one.


# Findings in the imported code

Problems noticed while bringing a service in, and deliberately not fixed by
the branch that found them: changing behaviour inside a commit whose job is to
move code makes a regression and a migration fault indistinguishable. Recorded
here so they are not lost with the review thread. They belong in issues
against the owning code once someone picks them up — nobody has filed them
yet.

Each was checked against the tree rather than taken on the reviewer's word.

## swarf

Raised by the Copilot reviewer on
[#3](https://github.com/fil-forge/forge-2/pull/3), and present at `c43af97`,
the commit the subtree imported — so none of these are migration damage.

### The revocation lookup is served as immutable for a year, and is not

`swarf/pkg/fx/app.go:348` sets `public, max-age=31536000, immutable` on the
by-delegation lookup. But the route is mutable: the schema permits several
rows per `revoked_delegation`, and `Get` returns the newest of them —
`swarf/pkg/store/postgres/store.go:83` is `ORDER BY recorded_at DESC, id DESC
LIMIT 1`. A cache may therefore serve a superseded record for a year, and
`immutable` tells it not even to revalidate.

On a revocation endpoint that fails in the wrong direction: a client holding
the cached older record sees a *narrower* revocation than the one actually
recorded.

It interacts with the next finding, and the two want deciding together. If
"newest matching" is the intended contract then the caching is wrong; if the
endpoint is meant to be immutable then it should be content-addressed by cause
CID, and `Get`-by-delegation is the wrong route shape.

### The memory and PostgreSQL stores disagree about what `Get` returns

`swarf/pkg/store/memory/store.go:64` keys records by the revoked delegation, so
a second revocation of the same delegation overwrites the first. PostgreSQL
keeps every row and returns the newest. The two backends therefore answer the
same question differently after a repeated revocation, and a memory-backed
stream cannot satisfy the interface's "all revocation records" contract.

The overwrite is the small part. The divergence is the real one: a test that
passes against the memory store can be wrong about production.

### The firehose client drops oversized events and then hangs

`swarf/pkg/client/client.go:259` builds a default `bufio.NewScanner`, which
caps a token at 64 KiB, and line 286 is `_ = scanner.Err()` under a comment
saying the caller reconnects. A valid event can exceed the cap, because
`api.FirehoseRevocation.Path` may carry many CIDs. `ErrTooLong` is then
discarded and `Stream` reconnects at the same cursor indefinitely, never
yielding the record and never returning an error — it presents as a hang
rather than a failure, which is the worse of the two.

**The fix already exists in this repository, in the wrong place.** The CLI
raises the limit and checks the error — `cmd/swarf/stream.go:79` is
`scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)` and line 101 is
`if err := scanner.Err(); err != nil` — while the library every other service
consumes does neither.

### The stream's settle window assumes a bound on transaction duration

`swarf/pkg/store/postgres/store.go:22` defines `streamSettleWindow = 10 *
time.Second`, documented as bounding "how long an insert may take between its
`recorded_at` (NOW() at transaction start) and its row becoming visible".

That is an assumption, not a bound. PostgreSQL assigns `NOW()` at transaction
start, so an INSERT that blocks for longer commits with a `recorded_at`
already behind the cursor, and the stream misses that revocation permanently.
The code is aware of the shape of the problem — the comment at line 111
explains that rows can become visible out of `recorded_at` order — but ten
seconds is a guess at how far out of order. A monotonic database sequence
would make publication and cursor advancement independent of how long a
transaction took.
