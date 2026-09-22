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
changed-files detection. Every push to `main` runs all four workflows that
trigger on one, and every job in them; a pull request runs five, since `release`
added a `pull_request` trigger — though on an ordinary branch that one resolves
no service and costs only its short `plan` job. `ci.yml`'s own header says why:

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
`release.yml` flow". **There was no `release.yml`**; `main` had four workflows —
`ci`, `e2e`, `images`, `itest` — and none of them tagged, released or
published. The plan treated `forge` at `f60dd59` as a starting point that
already had that apparatus; this repository was built from subtree imports
instead, and the apparatus was never part of them.

**There is one now, and it is deliberately not armed** — it publishes nothing,
is dry-run by default, and never creates a tag. (It is no longer dispatch-only:
a pull request from a `release/<svc>` branch runs it too, which is what makes
the release path continuously checked rather than checked when recalled.) It does not close this entry. What it
supplies is the build-and-verify path; what remains is the part that was always
the hard bit, and it is a decision rather than a workflow: **while the polyrepo
still releases these same services, a tag cut here gives each one two sources
of truth.** Until that is settled, nothing should be tagged in this
repository.

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
release flow should check the *effect*, and then this lint can go.

**Built, as `.github/scripts/assert-released-version.sh`.** Keeping this entry
because the lint has *not* gone: the assertion can only speak for services whose
binaries answer a version probe, which is the "only half the services can be
asked" problem below, so the two overlap rather than one replacing the other.

The measurement below is why that script pins its working directory to the
repository root. It records a stale-path binary reporting `v0.0.0` from a
container and `v0.0.6` from a checkout, because the fallback reads
`version.json` by *relative* path — and goreleaser runs inside `<svc>/`, where
that file exists. Run from there, the fallback answers and the assertion says
`ok` to the exact defect it is for.

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
the bytes are present, not whether the program reports them.
The mechanism is the one two paragraphs above: **`go version -m` records the
`-ldflags` argument in the binary**, so a dead `-X` writes its own version
string there verbatim. Measured on a fixture whose `-X` names a package that
does not exist:

```
$ ./dead                                 # what the program reports
version: v0.0.0
$ strings dead | grep -c 'v7\.7\.7'      # what a grep finds
2
$ go version -m dead | grep ldflags
	build	-ldflags="-X example.com/fx/pkg/buildTYPO.Version=v7.7.7"
```

Both hits are inside that recorded line. The grep passes; the binary is broken.

`indexing-service` carried a live example of the same shape until this pull
request: two `-X` flags for one version, of which **`-X main.version` was
dead** — `indexing-service/cmd` declares no such symbol — while the
`pkg/build.version` one worked. `check-goreleaser-ldflags.sh` passed the
`main.*` flags because it special-cases `main`. This pull request drops them.

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

## Decide whether service images should report what they are

Six of the ten services here carry a build-metadata package — `hilt`,
`indexing-service`, `ingot`, `piri`, `sprue` and `swarf`, in `pkg/build` or
`internal/build`. In an image every one of them reports its defaults, because
nothing sets them when the image is built. `piri`'s `version` subcommand in a
container:

```
version: v0.0.0-unknown
commit: unknown
built at: unknown
built by: unknown
```

`ingot` prints the same four fields in a different shape; the other four have
no such CLI output at all, and surface only a version — `hilt`, `sprue` and
`swarf` through their fx server info, `indexing-service` at `GET /`.

The remaining four — `delegator`, `forgectl`, `piri-signing-service` and
`smelt` — have no build-metadata package, so there is nothing to stamp.

**What the six declare is not what they read**, and that bears on the choice
at the end:

| | declares | read in Go |
|---|---|---|
| `piri`, `ingot` | version, `Commit`, `Date`, `BuiltBy` | all four |
| `hilt`, `sprue`, `swarf` | version, `Commit`, `Date`, `BuiltBy` | **version only** |
| `indexing-service` | `version`, `Version`, `UserAgent` | `Version` only |

`Commit`, `Date` and `BuiltBy` are already dead in three of the six, and
`indexing-service` never declared them. `UserAgent` is deader still —
`indexing-service`, `piri` and `sprue` all declare it and
`git grep -n 'build\.UserAgent' -- '*.go'` matches **nothing anywhere**.

**The symptom.** Eight service Dockerfiles — `delegator`, `hilt`,
`indexing-service`, `ingot`, `piri`, `piri-signing-service`, `sprue`, `swarf` —
and not one passes a `-X`. The only Dockerfile in the repository that does is
`smelt/systems/stress-tester/Dockerfile`, a test harness rather than a service.
(There are thirteen Dockerfiles in total; the other four are
`Dockerfile.release` — see **What a release does get** below, where two of the
four turn out to be dead.)

**One cause, not three.** Nothing passes `-X`, and neither fallback can stand
in for it:

- The development fallback reads `version.json` **by relative path, at runtime,
  from `init()`** — in `hilt`, `indexing-service`, `piri`, `sprue` and `swarf`.
  No `prod` stage sets a `WORKDIR` or copies `version.json`; each one `COPY`s
  the binary and nothing else. So cwd is `/`, the open fails, and the version
  falls back to the package default, `v0.0.0` in all five. `ingot` has no such
  read at all — nothing in its Go reads `version.json`, despite the file
  existing — and its `"dev"` comes from `init()` seeing an empty `Version`,
  which is the `-X` absence and not a second cause.
- The revision helper reads `vcs.revision` from `debug.ReadBuildInfo()`, which
  the toolchain records only when it compiles inside a git checkout. **No
  Dockerfile `COPY`s `.git` into its builder**, so there is nothing to record.

An earlier draft of this entry blamed `.dockerignore` for both of those. It
causes neither, but not for the reason that draft gave, and the real reasons
are the two above rather than anything about `.dockerignore` at all:

- On `.git`: **every one of the seven `.dockerignore` files lists `.git`**, so
  a reader checking whether it is excluded finds that it is — and that is not
  what makes it absent. **No Dockerfile here would carry `.git` into its builder
  even if nothing excluded it.** The only build context containing `.git` is the
  repository root, and `images.yml` gives `context: .` to exactly four services
  — `piri`, `ingot`, `hilt`, `delegator` — **none of which uses `COPY . .`**;
  each copies named service directories. The five Dockerfiles that do
  `COPY . .` (`indexing-service`, `piri-signing-service`, `sprue`, `swarf`,
  `smelt/systems/stress-tester`) are built with service-directory contexts,
  which contain no `.git` to copy. The exclusions are not load-bearing in
  either direction. (`hilt` and `swarf` have no `.dockerignore` at all, so
  "excluded everywhere" was never true either.)
- On `version.json`: only `piri/.dockerignore` and `sprue/.dockerignore` name
  it. Docker reads only the `.dockerignore` at the **build context root**, and
  `images.yml` builds `piri` with `context: .`, so `piri/.dockerignore` is
  never read — but it builds `sprue` with `context: sprue`, so
  `sprue/.dockerignore` **is** the context-root file and **does** exclude
  `version.json`. It makes no difference, because no `prod` stage copies the
  file in any case, which is the universal reason. `swarf` has no
  `.dockerignore` at all and reports `unknown` exactly like the rest.

**Nothing here publishes an image yet.** `images.yml` is `push: false` and
takes no `packages: write`; no other workflow pushes. So in this repository it
shows up only in the images `e2e` and `itest` build, and in local builds. The
images that *do* ship today are the polyrepos' `ghcr.io/fil-forge/<svc>:main`
that `smelt/.env.published` runs, and those report nothing either: of the
**eight** `publish-ghcr.yml` files this repository's history carries —
`delegator`, `hilt`, `indexing-service`, `ingot`, `piri`,
`piri-signing-service`, `sprue` and `swarf` — **not one passes any
`build-args`, in any of their 38 historical versions**.

  **38 is the per-service sum of distinct versions**, which is what "their
  historical versions" means. Each service's upstream history is reachable from
  the commit its subtree-add names, and the file sat at the bare
  `.github/workflows/publish-ghcr.yml` until the add moved it, so the split is:

  ```sh
  git log --oneline --merges --grep='^git-subtree-dir:' --format='%s'   # roots
  for root in <the eight>; do
    git rev-list "$root" |
      while read -r c; do
        git rev-parse -q --verify "$c:.github/workflows/publish-ghcr.yml"
      done | sort -u | wc -l
  done
  ```

  delegator 6, hilt 2, indexing-service 6, ingot 4, piri 7,
  piri-signing-service 4, sprue 7, swarf 2.

  An earlier revision of this paragraph gave 3, 5, 4, 5, 8, 3, 8, 2 — a
  different split of the same 38, and every service but `swarf` wrong. The
  total was derived; the split beside it was not, and summing to the right
  number is what let it stand. Worth keeping as the sharpest example in this
  entry of the thing the entry is about: a number that agrees with a checked
  number and was never checked itself.

  The union of distinct *blobs* reachable from `HEAD` is **29**, and
  `git rev-list HEAD --objects | grep publish-ghcr | sort -u | wc -l` returns
  29 and looks exactly like a refutation. It is not one: **six blobs** occur in
  more than one service's history, filling 15 of the 38 slots, so 38 − 29 = 9
  is the count of redundant *slots*, not of shared files.

  ```
  b4d0de53  hilt, sprue, swarf
  481e0928  delegator, indexing-service, piri-signing-service
  f80feb0a  indexing-service, piri, piri-signing-service
  0c9a803b  delegator, indexing-service
  8d2f791f  delegator, piri
  d2b0fdee  indexing-service, piri
  ```

  **None of that overlap is at import.** The eight blobs the eight subtree-adds
  brought in are eight distinct blobs — no two services were imported carrying
  an identical file. Every shared blob above is an *earlier* version on at least
  one side; `0c9a803b`, for instance, is indexing-service's import and a
  delegator version predating delegator's. An earlier revision said the
  overlap was "because several of these workflows were byte-identical at
  import", which is a cause that does not exist.

  A ninth path, `.github/workflows/publish-ghcr.yml` with no service prefix,
  also appears under `git log -m`: that is these same eight files at their
  pre-import paths on the upstream side of each subtree merge, not a ninth
  service.

  An earlier revision of this paragraph said seven and that `swarf`'s never
  existed here. It did: added by swarf's subtree-add `8ac8d922` and removed by
  `52a5979b`, both ancestors of this branch. The corpus was short because the
  `git log` that derived it lacked `-m`, and **`git log` without `-m` does not
  show file changes across a merge — and `git subtree add` is a merge**. That
  is this entry's own lesson, committed while writing the entry: a command
  whose output was read without asking what it could not see. `swarf` is also
  the service the omission mattered most for, since `smelt/.env.published`
  ships `SWARF_IMAGE`.

**What a release does get.** Four services have a goreleaser config —
`indexing-service`, `ingot`, `piri`, `sprue`. Each stamps its own build
package, so a released binary does report itself. One caveat:
`indexing-service/.goreleaser.yaml` also carries four `-X main.version`,
`main.commit`, `main.date` and `main.builtBy` flags that package `main` does
not declare, and `check-goreleaser-ldflags.sh` passes them because it
special-cases `main` — the live instance of the defect #9 was written to
catch. [#12](https://github.com/fil-forge/forge/pull/12) drops them; on `main` they are still there.

`ingot` and `sprue` additionally ship release *images* (`dockers:` →
`Dockerfile.release`), which package that stamped binary, so their released
images report themselves. `hilt/Dockerfile.release` and
`swarf/Dockerfile.release` package no such thing: **neither service has a
goreleaser config**, and nothing references either file. They are dead,
inherited from the polyrepo.

**Why it waits.** What is left is a publishing question, not a build one:
whether an image is expected to describe itself, or whether the tag and digest
are its identity and the binary need not agree. If it is expected to, each
field needs a source per workflow — a pull request build has a commit but no
version, a publish on `main` has both, a release already has them from
goreleaser — and a shared `ARG`/`-X` block would be the first thing every
service Dockerfile has in common, which is a small architectural commitment
rather than a tidy-up. Phase 1 has not settled that.

**The worked example is in a repository being archived.** `guppy` has the only
Dockerfiles in reach with `ARG VERSION/COMMIT/DATE/BUILT_BY` feeding four `-X`
flags — both its `Dockerfile` and its `Dockerfile.dev` — and, the more useful
half, its `.github/workflows/publish-ghcr.yml` answers the per-workflow
question: five `build-args:` stanzas carrying three `VERSION` policies,
`pr-<n>` with the head sha for a pull request, `main` with `github.sha` for a
`main` publish, and `v<meta.version>` for a release. `guppy` is being
dismantled and archived (`MAJOR_DECISIONS.md`). The flag list itself is
reconstructible from the four in-repo Makefiles that inject the same four
fields — `hilt`, `piri`, `sprue`, `swarf`; the workflow plumbing is the part
worth copying out before the repository goes.

**The choice.** Give every service image the metadata from a shared pattern;
or decide images identify themselves by tag and digest alone, and delete the
variables rather than leaving code that reads values nothing sets — for
`Commit`, `Date` and `BuiltBy` in `hilt`, `sprue` and `swarf` that is already
the state, so this option is partly just making it explicit; or do it
per-service as each one's release flow is settled.

Related but separate, and not part of this question: four of the six Makefiles
that inject `-X` stamp nothing. `hilt`, `piri` and `sprue` name the
pre-consolidation module path `github.com/fil-forge/<svc>/pkg/build`, and
`piri-signing-service` names `main` symbols nobody declared. `piri` has a
second, larger fault: `make build` names `github.com/fil-forge/piri/cmd` as its
build *target*, which has not resolved since consolidation, so it fails
outright rather than producing an unstamped binary. **Two were already
correct**, not one: `swarf`, and `smelt/systems/stress-tester`, whose Makefile
stamps `main.Version`, `main.Commit` and `main.BuildTime` into `./cmd/stress`
— and `cmd/stress/version.go` declares exactly those three. Its Dockerfile is
also the only one in the repository with the `ARG`/`-X` half: `ARG VERSION`,
`ARG COMMIT`, `ARG BUILD_TIME` feeding three `-X` flags.

That is the closest thing here to a worked example, and it is **not** a worked
example of the whole pattern — the missing half is a *source per builder*, and
the only thing that builds that Dockerfile, `smelt/systems/stress-tester/compose.yml`,
passes `VERSION: ${STRESS_VERSION:-dev}` and nothing else. `COMMIT` and
`BUILD_TIME` keep their `unknown` defaults in every image anyone actually
builds, which is the same symptom this entry opens with, in the one component
equipped to avoid it. Open as [#16](https://github.com/fil-forge/forge/pull/16). The wiki's *Needs Human Work* page carries the
same question for the polyrepo images, where it waits on a decision rather than
on the monorepo.

### The tag scheme Go requires is one goreleaser cannot read

Two requirements that do not currently meet. Both measured, not looked up.

**Go requires a subdirectory prefix.** A module at `piri/` is served from the
tag `piri/vX.Y.Z`, not `vX.Y.Z`. That is not a convention, it is the fetcher:
`cmd/go/internal/modfetch/coderepo.go:538` reads *"Tag must have a prefix
matching codeDir"* and builds `tagPrefix = r.codeDir + "/"`. All ten services
are in subdirectories, so all ten would need it.

**goreleaser OSS cannot parse such a tag.** Measured on v2.18.2:

| | |
|---|---|
| tag `piri/v1.2.3`, plain build | `⨯ failed to parse tag 'piri/v1.2.3' as semver` |
| same, with `GORELEASER_CURRENT_TAG` set to the **prefixed** tag `piri/v1.2.3` | identical failure — setting the variable does not make goreleaser accept a non-semver value |
| same, with `--snapshot` | "runs", and calls the version `piri/v1.2.3-SNAPSHOT-bb5b7f5` |
| a `monorepo:` block with `tag_prefix` | `field monorepo not found in type config.Project` |

That last row is the crux: `monorepo.tag_prefix` exists to solve exactly this
and is **GoReleaser Pro**. The OSS binary has no such field.

**What someone has to choose.** Not urgent, because the conflict only bites if
something consumes a forge module *by version*, and today nothing does —
`guppy`, `ucantone`, `libforge` and `automobile` all still pin the polyrepo
paths, and inside the repo the sibling edges are `replace ../<svc>`. So:

- **Pay for goreleaser Pro** and use `monorepo.tag_prefix`. Smallest change,
  costs money, and buys a thing only Go module consumers need.
- **Do not tag for Go at all.** Release these as binaries and images, which is
  what they are; keep `<svc>/vX.Y.Z` in reserve for the day someone wants
  `go get github.com/fil-forge/forge/<svc>`. Costs nothing now and defers the
  decision to the moment it has a concrete requester.
- **Drive goreleaser without a tag**, templating the version from an
  environment variable rather than `{{.Version}}`. Keeps OSS and the Go scheme,
  at the cost of rewriting the ldflags in all four configs. (An earlier version
  of this bullet also listed goreleaser's changelog as a cost. It is not one,
  under any of these three options: all four configs already set
  `changelog: disable: true`.)

**What the table does not settle, and `release.yml` depends on.** Every row in
the table above was measured with a *prefixed* tag. goreleaser's git pipe consults
`GORELEASER_CURRENT_TAG` before `git describe`, so setting it to the **plain**
semver sidesteps the parse entirely, which is what the workflow does. Measured
twice, independently: a release ran to completion with
`GORELEASER_CURRENT_TAG=v9.9.9` against a repository with no tags at all —
goreleaser reported `couldn't find any tags before "v9.9.9"`, took
`previous=<unknown> current=v9.9.9`, and the built binary printed
`version: 9.9.9`. The rows above are not evidence for that case; this paragraph
is.

**`release.yml` reaches the third option's outcome by a different mechanism,
and so does not pay its cost.** It reads the version from `version.json`, never creates
a tag, and hands goreleaser the plain semver through `GORELEASER_CURRENT_TAG`
so the parse never happens — while still requiring the Go-scheme
`<svc>/vX.Y.Z` tag to exist, and to point at the commit being built, before a
real release proceeds. That is the bullet's aim (keep OSS, keep the Go scheme)
without its method: because `GORELEASER_CURRENT_TAG` makes `{{.Version}}`
resolve correctly on its own, the four configs keep templating from it and no
ldflags are rewritten. The bullet's method — templating from an environment
variable instead — is what would force that rewrite.

It does not use `--snapshot` in either mode, which would stamp
`X.Y.Z-SNAPSHOT-<sha>` — a version the assertion can never match, so the
documented-safe default could not pass its own check. And **publishing is
closed** there, because goreleaser's release pipe creates the GitHub release's
tag itself: fed the plain semver it would create an unprefixed `v0.2.4` in the
namespace ten services share, which is this section's one-tag-namespace problem
arriving from the other direction.

# Findings in the imported code

Problems noticed while bringing a service in, and deliberately not fixed by
the branch that found them: changing behaviour inside a commit whose job is to
move code makes a regression and a migration fault indistinguishable. Recorded
here so they are not lost with the review thread. They belong in issues
against the owning code once someone picks them up — nobody has filed them
yet.

Each was checked against the tree rather than taken on the reviewer's word.

## piri: `TestPeriodicRotator` is a wall-clock race

`piri/pkg/store/local/retrievaljournal/periodic_rotator_test.go` starts a
rotator with a **1 ms** ticker, sleeps **30 ms**, then asserts that all six
non-`cid.Undef` batches were rotated. Under CPU contention the rotator
goroutine does not get six ticks in that window and the test fails with four.

It went red once in CI on a branch whose whole diff was comments in a shell
script. Reproduced locally — twelve concurrent `go test -count=20 -race`
processes:

```
total FAILs: 2 / 240
```

Zero failures in fifty consecutive solo runs, with and without `-race`. The
re-run passed.

**Its failure output reads backwards**, which matters if you meet it:

```go
require.Equal(t, actualBatches, expectedBatches)
```

`require.Equal` is `(t, expected, actual)`, so the log's `expected: … len=4` is
what happened and `actual: … len=6` is what was wanted. At face value it looks
like the rotator fired too often; it fired too few times.

The fix belongs upstream in `piri`, not in whichever branch next trips over it.

**The obvious fix is wrong, and wrong in a way that is worse than the bug**, so
it is written down here rather than left to be rediscovered. Replacing the
sleep with a bare `require.Eventually` on `len(actualBatches)` races:
`RotateFunc` is called from the rotator's own goroutine
(`periodic_rotator.go:51`) and appends to a slice the test goroutine owns, which
is safe today only because `pr.Stop()` synchronises before the read.
`Eventually` evaluates its condition on a **third** goroutine, so the read races
the appends. Measured — the patch does not flake, it fails every time under the
`-race` that `ci.yml:157` already runs:

```
WARNING: DATA RACE
--- FAIL: TestPeriodicRotator (0.01s)
    testing.go:1865: race detected during execution of test
```

All six rotations were logged: the logic succeeded and the test still failed.
Moving the call after `Stop()` does not rescue it either — the rotator is dead
by then and the condition can never become true.

What works, measured at 0 failures in 240 runs across twelve concurrent
`-race` processes:

```go
var mu sync.Mutex
pr.RotateFunc = func(batchID cid.Cid) {
    mu.Lock()
    actualBatches = append(actualBatches, batchID)
    mu.Unlock()
    t.Logf("Rotated batch: %s", batchID)
}

// HOIST the expectedBatches loop above this point. In the file today it sits
// AFTER pr.Stop(), so pasting the block below without moving it does not
// compile -- expectedBatches is not in scope yet.
pr.Start()
// assert, not require: require aborts the test goroutine, so pr.Stop() would
// never run and the rotator would leak.
assert.Eventually(t, func() bool {
    mu.Lock()
    defer mu.Unlock()
    return len(actualBatches) == len(expectedBatches)
}, 2*time.Second, time.Millisecond)
err := pr.Stop(t.Context())
require.NoError(t, err)

mu.Lock()
defer mu.Unlock()
require.Equal(t, expectedBatches, actualBatches)
```

A deadline instead of a fixed sleep removes the dependence on how much CPU the
runner has, and costs the passing case nothing.

## indexing-service: `TestCachingQueuePoller_BatchProcessing` synchronises on the wrong call

`indexing-service/pkg/service/providercacher/cachingqueuepoller_test.go` counts
down its `sync.WaitGroup` inside the `Run` hook of
`CacheProviderForIndexRecords`, then calls `poller.Stop()` as soon as
`wg.Wait()` returns. But the poller calls `Delete` **after** the cache call, and
the test also expects `Delete` `Times(numJobs)`. So the barrier releases one
call too early and `Stop()` can cut off the last `Delete`:

```
--- FAIL: TestCachingQueuePoller_BatchProcessing
    mock_CachingQueue.go:23: FAIL:  Delete(string,string)
    mock_CachingQueue.go:23: FAIL: 3 out of 4 expectation(s) were met.
        The code you are testing needs to make 1 more call(s).
```

Measured: **4 failures in 80 runs** across eight concurrent
`go test -race -count=10` processes; **zero** in ten solo runs. It went red once
in CI, on a branch touching only `.github/scripts/` and `AGENTS.md`.

**A verified fix exists, and it is NOT on a branch a pull will deliver.** It is
`indexing-service` `2c48785`, on that repository's `claude/forge-monorepo-poc-p9w0yr`
branch only — `git merge-base --is-ancestor 2c48785 origin/main` exits 1. A
`git subtree pull` tracks the default branch, so until that branch merges, this
repository keeps the racy copy described above and the entry stays live.

Measured with that patch applied here: 80 runs across eight concurrent
`go test -race -count=10` processes, **zero failures**. It needed two barriers, not one, which is the part
worth reading. Moving `wg.Done()` into `Delete`'s `Run` hook — the obvious
repair, and the right barrier for that expectation — took it from 4 in 80 only
to **1 in 80**. The remaining failure was a different unmet expectation:

```
    mock_CachingQueue.go:23: FAIL:  Read(string,int)
        at: [...cachingqueuepoller_test.go:58]
```

the `Read` expectation declared `.Once()` that blocks on `<-ctx.Done()`. That
call is simply never made: the poll loop `select`s on its root context and
returns the moment `Stop()` cancels it, so it never comes round to
`processJobs` again (`go-ipni-tools`, `pkg/queue/poller.go:173-178`).

**The two barriers answer two different mechanisms, which is the easy thing to
get wrong here.** `Stop()` cancels the root context, waits for the loop to
close `stopped`, and only *then* calls the job queue's `Shutdown` with that
same, already-cancelled context — so `Shutdown` takes its `ctx.Done()` arm and
returns without draining, losing a `Delete` still in flight. `wg.Done()` in
`Delete`'s `Run` hook covers that one. The unissued final `Read` above is the
other, and the `parked` channel — closed by that `Read`'s own `Run` hook —
covers it, holding the test until the poll loop has actually parked. Both must
be satisfied before `Stop()` is called.

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
