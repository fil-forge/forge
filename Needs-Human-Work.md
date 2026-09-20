# Needs human work

**Updated 2026-09-19 18:10Z.** Everything here is waiting on a person — either
because it is a judgement call, or because the agent cannot perform the action.
Work that is merely unfinished does not belong here; see
[[Current State]] for the broad picture and [[Consolidation Findings]] for why
each item exists.

Kept current as things move. Items leave via **Recently cleared**, which is
pruned once it stops being useful.

## Blocking

**Nothing is blocked. Nine pull requests and one issue are waiting for you.**
Per your call on 2026-09-19, **`forge` PRs are fully Open** when they look
ready — you are the only one looking at them right now — while
**upstream PRs stay draft** so other engineers do not spend time on them before
you have given them a pass.

~~**Blocked on one permission**, the force-push to trim
indexing-service #106.~~ **Resolved without it, and no permission is needed.**
The split is done: [#107](https://github.com/fil-forge/indexing-service/pull/107)
carries the version work, cherry-picked onto `main` and **verified
byte-identical to the original commit**; #106 then took a plain revert of that
commit (`38923a3`), so **its net diff against `main` is now the one test
file**, which is what review reads. Reverting rather than rewriting cost only a
noisier commit list on a draft PR, and a squash merge erases even that.

Worth recording for next time, since you asked how to grant the permission:
**in this session none of the usual routes would have worked.** Per the
[settings docs](https://code.claude.com/docs/en/settings), a cloud session does
not read `~/.claude/settings.json` or `.claude/settings.local.json` at all, and
a session with **several repositories starts above the clones**, so from each
repository's `.claude/settings.json` it loads only plugins and marketplaces —
**not permission rules**. This session has eighteen repositories attached. The
routes that would work are a server-managed setting, an environment variable on
the cloud environment, or simply doing the push yourself.

`sprue` #106 was red for about eighteen hours before I caught it — see *How
sprue went red for eighteen hours*, which is as much about a gap in my own
checking as about the bug.

### Review-before-you-look is now the practice, and its first result

Petra's instruction, 2026-09-20: **run a review on every `forge` PR we open and
work its findings before she reads it.** Done for
[#12](https://github.com/fil-forge/forge/pull/12) (four rounds) and
[#15](https://github.com/fil-forge/forge/pull/15) (one round, below);
[#13](https://github.com/fil-forge/forge/pull/13) has had one round and needs a
second; #14 and the new #16 are in flight.

**It found more than expected, and the headline is not any single bug.** Four
rounds, each finding real defects, several verified by *executing* rather than
reading. The structural cause is one sentence:

> `assert-released-version.sh` is called only from `release.yml`, which is
> dispatch-only and has never been dispatched. **Nothing in CI had ever
> executed it.**

Three separate portability bugs shipped in that one file as a result — `mapfile`
(bash 4, absent from the macOS runner the workflow routes piri to), a perl
`alarm` cap that Go binaries ignore outright, and `mktemp` with no template
(GNU-only). Each was found by reading, because nothing ran. That is now fixed at
the root: `check-assert-released-version.sh` runs it in the `guards` job against
a fixture built on the fly, asserting all four directions, and it was verified
to go red when the script is broken two different ways.

Worth knowing beyond this PR:

- **The release built in Go workspace mode.** `go list -m all` for
  indexing-service differs on **1402 lines** between workspace mode and
  `GOWORK=off`. `ci.yml` pins `GOWORK=off`; `release.yml` did not, so the
  artifact released would not have been the artifact CI proved green. Rule 4.
- **goreleaser creates the tag**, via the GitHub release API, which is why
  publishing in that workflow is now closed rather than merely unarmed. Fed the
  plain semver it would create an unprefixed `v0.2.4` in the namespace ten
  services share, and all four configs set `release: mode: keep-existing`, so a
  second service at the same version would upload into the first's release.
  That is the one-tag-namespace problem arriving from a direction the plan did
  not anticipate.
- **Four dead `-X main.*` ldflags** in `indexing-service/.goreleaser.yaml` —
  package `main` declares none of them, and `check-goreleaser-ldflags.sh` passes
  them because it special-cases `main`. The live instance of the exact defect
  the PR is about, inside the one config the PR edits.
- **Three of the defects were mine, introduced while fixing the round before.**
  `@latest` replacing a pinned action in a job with write tokens; routing piri
  to a macOS runner the script could not run on; and a header claiming no
  goreleaser was available while the same diff contained a `goreleaser check`
  result. All recorded in the commit messages rather than quietly corrected.

**The judgement this leaves for Petra:** #12 took four rounds on five files and
was still yielding real findings at the fourth. It probably should not merge
until a dispatch has actually run — which is cheap, dry-run by default, and now
the only thing that can exercise goreleaser's behaviour at all.

### The second result: the review's most useful find was in a document

#15 adds **one Markdown entry and no code**, which is the least promising thing
to review. Its review found the entry wrong in four ways, and one of those
corrections turned out to name a live defect in the tree:

- **`.dockerignore` was not a cause**, and the entry blamed it for two of
  three. The version fallback is a *runtime* read of `version.json` by relative
  path from `init()`, and no `prod` stage sets a `WORKDIR` or copies the file —
  so cwd is `/` and the open fails whatever the build context held. Only piri's
  and sprue's `.dockerignore` name `version.json` at all, and Docker reads only
  the one at the **context root**, which for piri is the repository root. The
  missing `vcs.revision` is the same shape: no Dockerfile `COPY`s `.git`, and
  `swarf` has no `.dockerignore` whatsoever and reports `unknown` like the rest.
- **"This is images only" was false.** The survey behind that sentence read
  Dockerfiles and never looked at a Makefile. `hilt`, `piri` and `sprue` each
  injected four `-X` flags at `github.com/fil-forge/<svc>`, the
  pre-consolidation path, so `make build` produced an unstamped binary too; and
  `piri-signing-service` injected three into `main` symbols nobody declared.
  **That is the same defect #9 fixed for goreleaser, applied to half the
  corpus** — and `check-goreleaser-ldflags.sh` globbed `.goreleaser.y*ml`, so
  nothing was watching the other half. Now
  [#16](https://github.com/fil-forge/forge/pull/16).
- `piri/Makefile`'s build target named `github.com/fil-forge/piri/cmd`, which
  does not resolve. **`make build` in `piri/` has failed outright since
  consolidation** and nobody noticed, because no CI job runs `make`.
- Smaller: "nine Dockerfiles" omitted the four `Dockerfile.release` files (it is
  thirteen); `ingot`'s and `sprue`'s *released* images are stamped, because
  their `dockers:` stanza packages the goreleaser binary; and the
  `MAJOR_DECISIONS.md` line the entry quoted for guppy is not in that file.

**What this says about the practice**, which is the part worth keeping: the
review was aimed at a document and found a code defect, because checking a
claim meant running the command the claim was made from. The original survey
was a `grep` whose *pattern* had been validated and whose *corpus* never was —
the same failure mode as the eighteen-hour sprue miss, two weeks apart.

### The pull requests, in the order worth reading them

| | what | state |
|---|---|---|
| [forge #13](https://github.com/fil-forge/forge/pull/13) | **two** subtree tools now: `finish-subtree-pull.sh` (merge + the deletion audit) and `resolve-rewrite-conflicts.sh` (the module-path collisions, with the check that makes them safe) | **Open**, green all 24, on `563c27b5` — round two |
| [forge #14](https://github.com/fil-forge/forge/pull/14) | every subtree resynced to its upstream `main` — 37 commits across eight prefixes; **caught a live wire break**, see below. **This is the one that gates Phase 1.** | **Open**, green all 24, on `a032681d` — round one fixes; `itest hilt` was red on a Docker Hub connection reset and passed on its one re-run |
| [indexing-service #106](https://github.com/fil-forge/indexing-service/pull/106) | the poller flake, and **now only that** — net diff is one test file | draft, on `38923a3` |
| [indexing-service #107](https://github.com/fil-forge/indexing-service/pull/107) | the `version` subcommand + **four dead `-X` ldflags**, split out of #106 as you asked | draft, on `894f1c0` |
| [hilt #77](https://github.com/fil-forge/hilt/pull/77) | swarf bumped 9 commits for the firehose fixes, **plus** the same `./cmd/main.go` build bug sprue had | draft, on `4857e93` |
| [ingot #175](https://github.com/fil-forge/ingot/pull/175) | swarf bumped 9 commits — ingot is the repo that actually consumes the firehose | draft, on `6e205ef` |
| [forge #15](https://github.com/fil-forge/forge/pull/15) | one entry in `MONOREPO_TODO.md`: service images report no build metadata, filed as a question not a fix. **Second push corrects four wrong claims its own review found** | **Open**, green all 24, on `8972b1f6` |
| [forge #12](https://github.com/fil-forge/forge/pull/12) | round five found `--skip=publish` does not skip goreleaser's docker build — the premise round four deleted two setup steps on. Fixed, measured both ways | **Open**, on `06e49742` — round five complete |
| [forge #16](https://github.com/fil-forge/forge/pull/16) | the code half of #15's review: four Makefiles that stamped nothing, and the guard that globbed `.goreleaser.y*ml` and so never looked. **#12 must merge first** — tried rebasing it onto `main` and it conflicts in five files, two of which only exist on #12's branch | **Open**, on `e5ea4209` |
| [sprue #106](https://github.com/fil-forge/sprue/pull/106) | a `version` subcommand — **plus the fix for the container build it broke**, see below | draft, green on `2e8f17c`, after being red on `a50db97` |

### What #14's `e2e` caught, which is the most useful thing today

`e2e` went red on #14's first head, and the cause is worth knowing even if the
PR never merges.

**`libforge` renamed `/ucan/conclude`'s argument and its CBOR key on
2026-09-17:**

| libforge | field |
|---|---|
| `63f5b20` (09-14) — what `main` pins | `Receipt cid.Cid` &nbsp;`cborgen:"receipt"` |
| `96b4969` (09-17) — what #14 pins | `Receipts []cid.Cid` &nbsp;`cborgen:"receipts"` |

sprue took it in `de71035` (#98), ingot in #167, guppy in `02e997f` (#60) — all
three on the same day. **A renamed CBOR key does not fail to decode. It decodes
to nothing**: the old key is simply absent and `Receipts` comes back empty. So a
server one commit ahead of its client concludes nothing, never calls
`/blob/accept`, and the client polls for a receipt that will never exist:

```
polling accept receipt: receipt for bafyrei… was not found after 6 attempts
```

Nothing errors. Nothing logs a mismatch. **This is what a schema break looks
like when the participants disagree by one deploy**, and it is precisely the
failure the monorepo's e2e exists to catch — `main` is green because its
libforge is still the singular one.

**Confirmed by the fix working**: `e2e` passed on `9b3a0b00`
([run 35394930163](https://github.com/fil-forge/forge/actions/runs/35394930163/job/105761469672)).

**What it actually was here:** our pinned `ghcr.io/fil-forge/guppy:main-dev`
digest was ten commits behind guppy's own fix. One-line re-pin, by digest:
`sha256:4d8950d8…` (`sha-d74fd06-dev`) → `sha256:a44c1800…`
(`sha-02e997f-dev`). The snapshot manifest names the old digest too and was
left alone — it records which images *wrote* that snapshot's state in August.

**A correction worth keeping**, since the first comment on the PR is wrong and
stays in the thread: I first reported that guppy had *not* adopted the new
schema and that #14 was blocked on a guppy change and a republish. Wrong — I
had read a stale local branch as guppy's `main`. Guppy was already fixed; only
our pin was old.

**For Petra, the real question this raises:** four repositories changed a wire
format on one day, and the only thing that noticed a mismatch was an
end-to-end test in a fourth repository. Nothing in any of the four would have
told you. Worth deciding whether that is acceptable before Phase 1's compat
suite, not after.

**A fifth PR is open and is not a draft**:
[forge #12](https://github.com/fil-forge/forge/pull/12), the release workflow —
written, verified, deliberately not armed. It predates the draft-everything
instruction and is green. Left ready rather than converted, but say the word and
it goes back to draft.

### How sprue went red for eighteen hours, and what it says about my checking

`sprue` #106 — the `version` subcommand — broke sprue's **container build**
the moment it was pushed, and I did not notice until 14:34Z today. It had been
red since 2026-09-18 20:46Z. That is about eighteen hours.

**The break.** `cmd/version.go` is the first *second* file in sprue's
`package main`. Both Dockerfile stages and `make build` spelled the build as
`go build ... ./cmd/main.go` — a **file**, not a package. `go help build`:
*"If the arguments to build are a list of .go files from a single directory,
build treats them as a list of source files specifying a single package."* One
file listed, one file compiled, so `version.go` never reached the compiler:

```
cmd/main.go:37:21: undefined: versionCmd
```

It had never mattered, because `cmd/` had only ever held one file of its own.
The imports of `cmd/client` and `cmd/identity` were fine throughout — separate
packages resolve normally. Only a **sibling file in the same package**
disappears. Fixed in `2e8f17c` by naming the package in all three places;
`.goreleaser.yaml` already said `main: ./cmd` and was always right, so no
released binary was ever affected.

**And the survey I ran off the back of it was wrong — the correction is worth
more than the original claim.** I wrote that no other fil-forge repo spells a
build this way, across eleven repositories, and noted that the grep pattern was
validated against the pre-fix line so the empty result was a true negative.
The pattern was fine. **The corpus was not**: the survey ran over the
repositories checked out in the session, and `hilt` and `ingot` were not among
them, so it could not have found either. Validating the pattern made a negative
feel proven that had never been tested against the population I was claiming
about.

Re-run once both were cloned: **`hilt` had the identical spelling** in both
Dockerfile stages, latent for exactly the reason sprue's was — `cmd/` holds one
`.go` file today, and `cmd/client/` is a separate package that resolves
normally. Fixed in [hilt#77](https://github.com/fil-forge/hilt/pull/77), with
the break reproduced first by dropping a throwaway second file into
`package main`. `ingot` correctly builds `./cmd/ingot`. Of the twelve
repositories now checked, sprue and hilt were the two.

**Worth sitting with**: `go build ./...`, `go vet ./...`, `gofmt`, `go test
./...`, and `go-check`/`go-test` on ubuntu, macos and windows were **all green
on the broken tree**. Package-level checks cannot see a file-level build. The
container build was the only thing in the world that could, and it did — which
is an argument for the monorepo's CI covering container builds and not only
`go build ./...`.

**The gap was mine.** My hourly check-in polled the three `forge` PRs and
nothing else, even though the trigger is named "forge #13/#14 + upstream
drafts". The evidence was sitting on this page the whole time: the state column
for both upstream rows in the table above was **blank**, and I never filled it
in. A derived list would have had no blank to leave. The check now covers all
eight and reports CI, mergeability and review threads per PR.

### What the review found on #13, which is worse than #12's

The subtree tools had **the same class of silent hole they were built to
close**, and a fixture proved both.

**The audit read the wrong merge.** It bound to `HEAD^2` without checking that
HEAD's merge belonged to the prefix it was asked about. A resync pulls eight
prefixes, so **seven of eight audits looked at some other prefix's pull**, found
nothing, and exited 0 — while a file upstream had deleted sat in the tree. A
mistyped prefix passed for exactly the same reason. That is the failure #13
exists to prevent, inside #13.

Fixed by anchoring on the prefix's own subtree-**add**: `git subtree pull`
records no trailer, only `add` does, and the add's `git-subtree-split` names the
upstream root — so every later pull of that prefix has a second parent
descending from it, and no other prefix's does.

**My first fix for it was wrong in the same shape**, and only the fixture
caught it. I took "the most recent merge whose diff is confined to the prefix"
— but when upstream deletes a file we had already moved out, **the merge changes
nothing on our side, so its diff is empty**. An empty diff is precisely the case
the audit exists for, and the heuristic skipped it. Reading would not have found
that.

**The rewrite tool wrote 404s into the tree.** It turned
`https://github.com/fil-forge/piri/releases/download/v1/piri.tar.gz` into
`.../fil-forge/forge/releases/download/...` and reported success. That URL is
live in `piri/deploy/.../install-from-release.sh`. The rule now fires only for a
bare repository URL. Worth keeping: **17 tracked files still carry un-rewritten
service URLs**, so the monorepo never applied that rule — leaving them for a
human is correct, not a gap.

And it was locale-dependent. The service alternation was built in collation
order, so under `en_US.UTF-8` `piri` preceded `piri-signing-service` and matched
inside it, rewriting those URLs to `forge-signing-service` — a repository that
does not exist. Same script, different answer per machine.

Plus: `git show | grep -Iq .` returns 141 under `pipefail` for any text file
past the pipe buffer, so every large **text** file was declared binary and
refused (`go.sum`, `cbor_gen.go`); the rename-limit warning went to
`/dev/null`, so when git stops doing rename detection every upstream rename
reads as a delete; and `finish-subtree-pull.sh <prefix> --dry-run` ignored the
flag and wrote anyway.

**One cross-PR dependency you should know about:** #13's `AGENTS.md` told agents
to run `check-module-paths.sh`, which exists only on **#14's** branch. Whichever
merges first, that instruction dangled. Replaced with the grep it stands for
plus a note saying where the script comes from.

### A decision for you: containers report no build metadata, in six of seven

> **Corrected 2026-09-20.** Two claims below are wrong and are kept, struck
> through in prose rather than deleted, because the correction is the useful
> part. (1) "Checked every service's actual `go build` invocation" — it checked
> **Dockerfiles only**; four Makefiles also inject `-X`, three of them at a dead
> path, now [#16](https://github.com/fil-forge/forge/pull/16). (2) The three
> numbered causes are one cause: nothing passes `-X`, and `.dockerignore` is
> irrelevant to both fallbacks. See the review section above for the detail.

Found while fixing the above, **not** fixed, because it is a design call and a
wider diff than that PR.

**Your instruction on this was "if sprue is different from the others, issue an
upstream PR to bring it in line — sprue will have just missed it." It is not
different, so that instruction does not apply as written.** Checked every
service's actual `go build` invocation, continuation lines included:

| | injects `-X` build metadata |
|---|---|
| `guppy` | **yes** — `ARG VERSION/COMMIT/DATE/BUILT_BY` → four `-X` flags |
| `piri`, `sprue`, `ingot`, `hilt`, `swarf`, `indexing-service` | **no** — `-ldflags="-s -w"` only |

sprue did not miss something the others got right; **guppy is the only one that
did it at all**. A PR to sprue alone would make it two of seven rather than fix
the class. (My first pass at this survey reported guppy as a "no" too — a
single-line grep against a `-ldflags` that continues across lines. Corrected.)

My suggestion, not yet acted on: one PR per repo from a shared pattern copied
off guppy's Dockerfile. That is six PRs, so it wants your word first.

A sprue running in a container answers:

```
version: v0.0.0-unknown
commit: unknown
built at: unknown
built by: unknown
```

Every field dead, from three independent pre-existing causes:

1. Neither Dockerfile stage passes any `-X` flag, so `Commit`, `Date` and
   `BuiltBy` keep their `"unknown"` defaults.
2. `.dockerignore` excludes `version.json`, so `pkg/build`'s development
   fallback `readVersionFromFile()` fails and `version` falls back to
   `defaultVersion` (`v0.0.0`).
3. `.dockerignore` also excludes `.git`, so the compiler stamps no
   `vcs.revision` and `pkg/internal/revision` reports `unknown`.

Observed rather than inferred — built under the same conditions
(`-buildvcs=false`, no `-X`, run from a cwd without `version.json`) to produce
exactly that output. ~~**Released goreleaser binaries are unaffected**; this is
containers only.~~ Wrong in both directions: `ingot`'s and `sprue`'s released
*images* are stamped, and `make build` was unstamped for three services. See
the correction note at the top of this section.

It matters because it cuts against why the subcommand was added: a release
check that runs the binary and asks it what it is. That check works on release
artifacts and would learn nothing from an image. Fixing it means plumbing
`VERSION`/`COMMIT`/`DATE` through as buildx `ARG`s and deciding where they come
from in `Build Check` versus the publish workflow — which is your call, not
mine to make inside a PR about a subcommand.

### Nothing will ever tell you an in-house dependency is stale

The most useful thing cloning `hilt` and `ingot` turned up, and it is not the
bump itself.

You unblocked me to bump a swarf pin that was a month old. The interesting
question is why it got that way, and the answer is that **no automation in the
fleet can see an in-house Go dependency fall behind.**

Measured, on `ingot`, which has a weekly `gomod` dependabot:

> **35 of its last 100 pull requests are dependabot's. Zero of the 35 bump a
> `fil-forge/*` module.** All 35 are third-party — aws-sdk, moby, openbao,
> pgx, fasthttp.

`hilt` has no dependabot config at all.

The cause is tagging, and it is not about privacy — `swarf` is a public repo:

| repo | tags | latest |
|---|---|---|
| `swarf`, `hilt`, `ingot` | 1 | `v0.0.0` |
| `sprue`, `libforge`, `ucantone`, `smelt`, `guppy` | 0 | — |
| `piri` | 1 | `v0.2.4` |
| `indexing-service` | 1 | `v1.13.4` |

The 35-vs-0 count is measured; the mechanism below is inference from that
table, though it is not a subtle one. A pseudo-version like
`v0.0.1-0.20260821142121-d5d1a0a56f00` sorts **above** `v0.0.0` — it is a
prerelease of `v0.0.1`, and `v0.0.1 > v0.0.0` — so for `swarf`, `hilt` and
`ingot` there is no higher tagged version for dependabot to offer. For the five
repos with no tags at all there is nothing to target whatsoever. Dependabot
does not chase untagged commits on a default branch.

**So eight of ten repos have in-house dependencies that no tooling will ever
flag as behind.** Every `fil-forge/*` bump across the fleet is a manual
pseudo-version edit by whoever happens to look, and the month-old swarf pin was
the normal outcome of that, not an oversight by anyone.

**Why this matters beyond the bump:** it is an independent argument for Phase 1
step 1, "cut initial release tags". Until now that step was justified only from
the consolidation plan's own sequencing. This says the polyrepo has a live
supply-chain blind spot that tagging closes, monorepo or not — and it is the
second time in two days that a cross-repo staleness went unnoticed until
something unrelated tripped over it (the first being the `/ucan/conclude` key
rename, below).

### The issue

[swarf #20](https://github.com/fil-forge/swarf/issues/20) — **the revocation
lookup contract. You asked whether `ucan-wg/revocation` answers it. It does.**

[UCAN Revocation v1.0.0-rc.1](https://github.com/ucan-wg/revocation), line 111,
verbatim:

> _Revocations MUST be immutable and irreversible._ Recipients of revocations
> SHOULD treat them as a monotonically-growing set. If a Revocation was issued
> in error, it MUST NOT be retracted — a new, unique UCAN delegation MAY be
> issued (e.g. by updating the nonce or changing the time bounds). This prevents
> confusion as the revocation moves through the network and makes revocation
> stores append-only and highly amenable to caching and gossip.

So the question the issue posed — is a second revocation of the same delegation
a *correction* of the first or an *addition* to it? — is settled: **an
addition.** Append-only, monotonically growing. That makes **both** current
behaviours wrong for the same reason: PostgreSQL returning the newest row, and
the memory store keeping one by overwriting.

It sharpens the cache header rather than settling it. Once a delegation has
*any* revocation the answer is permanently yes, so caching a **positive** answer
`immutable` for a year is correct and caching a **negative** one is not. The
spec gives the semantics; it does not dictate the API shape — whether the lookup
returns the newest record, all records, or a boolean is still ours to pick.

The original write-up, before the spec was consulted: Three places
answer "what does looking up a revocation by delegation CID return?" and no two
agree: the interface does not say, PostgreSQL returns the newest row, the memory
store keeps only one by overwriting, and the HTTP route caches the answer
`immutable` for a year. The issue lays out both readings — *newest-matching*
(the lookup is a mutable view, so the cache header is wrong) and *immutable*
(the record is a fact, so the route should be keyed by cause CID) — and says
what would settle it: **whether a second revocation of the same delegation is a
correction of the first or an addition to it.**

### One thing to know about indexing-service #106

It now carries **two independent commits**, because that repository's designated
branch is the only one I may push to and GitHub allows one PR per branch:

- `2c48785` the poller flake fix (the original subject)
- `72193d7` a `version` subcommand, **and four dead `-X` ldflags**

The second half found something. `.goreleaser.yaml` was passing `-X main.version
-X main.commit -X main.date -X main.builtBy`, and package `main` under `./cmd`
declares none of them. **The Go linker accepts a `-X` for a symbol that does not
exist and says nothing** — exit 0, no diagnostic, and `go version -m` still
lists the flag. Measured on that tree:

```
-X .../pkg/build.Commit=deadbee   ->  commit: deadbee
-X main.commit=deadbee            ->  commit: unknown
-X main.doesnotexist=hello        ->  builds, exit 0, no output
```

So every indexing-service release so far has reported its version (that one flag
was right) and `unknown` for the other three. Split the PR if you would rather
review them apart; the commits are clean.

### The indexing-service race: ported after all, and why that changed

Still not in upstream `main`, so the resync commit did not carry it and said so.
**Then it went red on #14**, with the identical signature, and that made the
earlier reasoning wrong for this PR: the fix exists, I wrote it, and waiting on
my own open PR to merge is still waiting while the branch sits red for a reason
unrelated to the resync. Ported in `d4505701`, byte for byte from upstream's
`2c48785` — the change is entirely inside the test body and touches no import
line, so it applied to the monorepo copy unmodified, and it no-ops when a later
pull brings the identical change down. 30 runs under `-race -shuffle=on` on this
tree, 0 failures.

The upstream PR stays open and is still the place the fix belongs.

**The first diagnosis of that race was wrong and the record should keep saying
so.** The claim was "`poller.Stop()` races the final `Delete`". `Stop()` is not
a barrier at all: it cancels its context and then passes that cancelled context
to `jobqueue.Shutdown`, which returns `ctx.Err()` immediately without waiting
for workers. Probed directly — 0 of 11 deletes had happened when `Stop()`
returned after 49µs, 11 of 11 by 300ms later. The WaitGroup was the test's only
synchronisation.

**Standing rule from Petra's decision, unchanged:** when a problem turns up in
imported code, ask *first* whether it belongs upstream, and fix it there
whenever it makes any sense.

## Open pull requests

All four are listed above under Blocking. Merged earlier today, for the record:

- **[#11](https://github.com/fil-forge/forge/pull/11)** 17:04Z as `9870d48a` —
  itest sharding.
- **[#10](https://github.com/fil-forge/forge/pull/10)** 19:08Z as `0d8fb04c` —
  the subtree merge tool. **#13 supersedes its interface**: same script, renamed
  `finish-subtree-pull.sh`, with the audit #10 could not do.

**The gap #10 left is now closed.** When upstream deletes a file we moved out of
a prefix, both sides deleted the path, so git raises no conflict and the pull
succeeds in silence. #13 reads upstream's own diff against the previous split
point instead of waiting for a conflict, runs in both modes — including after a
pull where nothing conflicted, which is exactly when it is the only thing
looking — and exits non-zero if it finds one.

## Waiting on Petra

- ~~Archive `fil-forge/forge-2`, or delete it?~~ **Done — archived 20:45Z.**
  Which is what the record needed: this wiki links 11 distinct `forge-2` PRs
  in 16 places and `main`'s merge commits name those numbers, and the links
  could not have been repointed (`forge`'s numbering is separate and **#2
  already collides**). Archiving keeps them all live. Side effect worth
  knowing: `forge-2` is read-only, so #28 stays frozen as *open* and cannot
  be closed — the archive banner is the signal, not its state.
- ~~Where does #28 land?~~ **Decided: it does not — and the ordering worked
  out.** [`swarf` #17](https://github.com/fil-forge/swarf/pull/17) (and #18,
  #19) merged upstream before the pull, so
  [forge #14](https://github.com/fil-forge/forge/pull/14) brought the fix in
  rather than the hang: `swarf/pkg/client/client.go` now reads through
  `sse.NewScanner` and returns `ErrEventTooLong` to the caller instead of
  discarding it. Checked in the pulled tree, not assumed from the commit
  subjects. #28 is superseded and can be closed whenever convenient.

**Three new decisions taken while you were away, all reversible, all flagged on
the PR that makes them:**

- **Two itest shardings in ingot** rather than picking one — upstream's named
  Makefile lists and our derived round-robin. Neither subsumes the other, and
  dropping upstream's means re-conflicting on every future pull. Details in
  Subtree drift below.
- **`indexing-service`'s dead ldflags fixed in the same PR as the poller flake**,
  because the branch policy gives that repository one branch and GitHub one PR
  per branch. Commits are clean; split if you prefer.
- ~~**forge #14 does not carry the indexing-service poller fix**~~ —
  **reversed, and the reversal is the current state.** It does carry it, as
  `d4505701`. The original reasoning (upstream is the source of truth; a resync
  that runs ahead of upstream is not a resync) stopped holding the moment the
  branch actually went red for a reason unrelated to the resync, with a fix I
  had already written sitting in an open PR. Full account under *The
  indexing-service race* above.

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

## Subtree drift: done, and three things it taught

**All ten prefixes are at their upstream `main`** on
[forge #14](https://github.com/fil-forge/forge/pull/14) — 37 commits across
eight of them; `hilt` and `piri-signing-service` were already current.

### 1. Never measure drift from commit messages

**`git subtree pull` records no `git-subtree-split` trailer.** Only `git subtree
add` does. Checked on a purpose-built fixture, because it had already fooled me:
without `-m` the message is `Merge commit '<sha>'`, with `-m` it is exactly what
you gave, and neither carries `git-subtree-dir:` or `git-subtree-split:`.

My first measurement grepped for those trailers and reported **91 commits across
nine prefixes**. The truth was 37 across eight — `main` already carried
`e6550212 subtree: pull hilt to 9815d93` and three siblings, invisible to the
grep. Ancestry is the only sound derivation and needs no metadata:

```sh
git merge-base --is-ancestor "up-$p/main" HEAD   # up to date
git rev-list --count "up-$p/main" --not HEAD     # how far behind
```

### 2. The quiet half of a subtree pull, and what it can cost

A conflict is loud. **A cleanly-merged hunk carrying a polyrepo import path is
silent**, and it happened in four of the eight pulls — piri 5 files, ingot 2,
swarf 2, sprue 3.

Two of those did not fail. They *resolved*:

- `go mod tidy` added `github.com/fil-forge/ingot v0.0.0` to ingot's go.mod
- `go mod tidy` added `github.com/fil-forge/sprue@506f5f6` to **sprue's own**
  go.mod — the module depending on itself at its old published path, pinned to
  the very commit being merged

That builds, and it builds green, against a copy downloaded from the polyrepo
instead of the tree in front of you.

`swarf` also kills the assumption that this is about *added* files:
`swarf/cmd/swarf/stream.go` already existed and had no conflict — upstream's new
import merged into our block without touching a rewritten line.

**`.github/scripts/check-module-paths.sh` (on #14) is the guard**, verified by
failure: restoring the real unrewritten `piri/pkg/fx/app/init.go` from
`up-piri/main` makes it report all three import lines.

### 3. The image pins did not move — measured, not assumed

The compose guard does not cover Go testcontainers references
(`piri/.../testutil/minio.go`, `sprue/internal/testutil/s3.go`), so the whole
`<image>@sha256:<64 hex>` population was snapshotted before and after:
**97 occurrences and 40 distinct refs, both times, nothing lost or gained.**
No new image-shaped literals in the branch's added Go lines either.

The earlier note here predicting a loud conflict on those files was right in
spirit and never got tested, because nothing upstream touched them.

### One decision #14 leaves open

**ingot now has two itest shardings.** Upstream added its own (#165, #166) —
named lists in the Makefile, balanced by measured runtime, `rest` as the
complement. Ours derives three shards by round-robin over `go test -list`, so
there is no list to maintain at all. Both are kept, CI uses ours, and the
Makefile and `itest/README.md` say so where someone would look. **Whether to
keep two is yours.**

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
