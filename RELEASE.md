# Releasing

Per service, not per repository. Every service carries its own version and cuts
its own tag; there is no repo-wide version number and no release train.

## Two cycles

**The development cycle.** A pull request to `main`. CI runs unfiltered — every
module's build/vet/staticcheck/tidy/test, the `guards` job, images, e2e and
itest. It is reviewed. Merge when the review is satisfied and CI is green.

**The release cycle**, independent of it and per module. A release pull request
is branched `release/<service>`, bumps `<service>/version.json`, and changes
**nothing else**. Merging it declares that the module's current `main` is
version `vX.Y.Z` — which bundles every change to that module merged since its
last release, however many development cycles that was.

**The branch name is load-bearing**: of the pull requests that open,
`compat.yml` runs its compatibility suite for those whose head branch starts
with `release/` and no others, and `compat-refresh.yml` finds the ones to
refresh the same way. (A manual dispatch runs it on any ref — the name gates
the automatic run.)

Merging is the release decision. The tag and the release build follow from it —
today by hand; see *Tagging*.

## A red compat check means do not release

**That holds however it went red.** The check exists to say a release is safe
to ship; a red one has not said it, and shipping anyway is the vacuous-green
failure moved up a layer — releasing on a gate that did not clear it, rather
than on one that cleared it without looking.

**Until the fleet has releases this repository's harness can start, it will be
red.** That is the expected state and it is accurate: the monorepo is not
finished, and a release cut now would be cut without the evidence the process
asks for. Petra's call, 2026-09-24.

What the two kinds of red differ in is **what to go and fix**, not whether to
proceed.

### The gate could not ask (today's case)

`smelt` boots the baseline images with **this tree's** entrypoints and config
templates. Both services with a published release predate that surface:

```
piri  0.2.4  (2026-04-01)   Error: unknown flag: --plc-directory
                            the entrypoint has passed it since 2026-07-22
ingot 0.0.0  (2026-07-13)   ingot: invalid config:
                            - root_access and root_secret are required
                            ingot dropped those keys 2026-09-14
```

**Note the direction: this is HEAD's configuration applied to an old binary**,
which is backwards from a real upgrade, where an operator gets the new binary
first and the new configuration with it. Old ingot demands a key HEAD removed;
old piri does not know a flag HEAD added. Neither is a break an operator would
hit upgrading — so this red is **not** evidence that the release breaks the
network.

It is also not evidence that it does not. The container never becomes healthy,
so the suite never reaches a wire-compatibility question at all. **No answer,
not a good one.**

**What fixes it:** a baseline the harness can start — the first release cut
from a recent commit, Phase 1 in the plan. Nothing here changes when that
happens: the baseline comes from the registry, so a new `X.Y.Z` image is
picked up on the next run.

### The gate asked and the answer was no

The stack comes up and the failure is inside `assertUploadRetrieve` — the
upload or the retrieval fails across the version skew. That is a real
incompatibility between the release and what is deployed, and **the fix is in
the code**, not in the harness.

**Telling them apart:** a could-not-ask red fails during `compose up`, with
`container … is unhealthy` and a config or flag error in that container's own
log, before any upload runs. Anything that gets past `compose up` is the
second kind.

## Versions

`<service>/version.json`, one key: `{"version": "v0.2.4"}`. It is the single
source of the version — the tag is checked against it, and goreleaser's `-X`
ldflags stamp it into the binary. Bump it by hand on the release branch.

Semver per service, independently. What counts as a minor versus a patch is
not written down anywhere yet, and the fleet is barely tagged upstream, so
treat the existing numbers as a starting point rather than a precedent.

## Tagging

Tags are `<service>/vX.Y.Z` — `piri/v0.2.5`, not `v0.2.5`. Ten services share
one tag namespace, so an unprefixed tag is ambiguous, and a second service
releasing at a version the first already used would collide.

**The tag should follow from merging the release pull request, automatically.**
That merge already carries the entire decision — it means "release the current
`main` of this module as the listed version" and has no other meaning — so a
human retyping the tag afterwards is a second chance to get it wrong rather
than a second check.

**That is not built yet.** Today the tag is created by hand after the merge,
and `release.yml` never creates one: it asserts a tag exists *and* points at
the commit being built, failing otherwise. That assertion is the interim
guard, not the intended design.

## Artifacts

`goreleaser`, per service, from `<service>/.goreleaser.yaml`. **A service is
releasable exactly when it has one** — `release.yml` derives that set with a
`find` at depth 2 rather than keeping a list, and refuses a dispatch for
anything else, naming what is releasable. Today that is `indexing-service`,
`ingot`, `piri` and `sprue`.

Binaries for every service with a config; container images additionally for
the two whose configs carry a `dockers:` stanza (`ingot`, `sprue`).

## What is not armed

**`release.yml` cannot publish anything at this head, on purpose.** It is
dispatch-only, `dry_run` defaults to true, and three independent things stop a
publish even with `dry_run: false`: goreleaser's `publish` and `docker` steps
are skipped unconditionally, the job's token is `contents: read`, and the
`publish` step itself fails with an error saying why.

**Arming it is not a permissions change.** goreleaser's release pipe creates
the tag it releases, from the plain semver — so it would create an unprefixed
`v0.2.4`, not `piri/v0.2.4`, in the namespace ten services share. And with
`release: mode: keep-existing` on all four configs and three services still at
`v0.0.0`, the second service to release at a version another already used
uploads its artifacts into that service's release instead of failing.

Neither is fixable by a flag. The choice is per-service `release: disable:
true` with somewhere else to put artifacts, or a tag scheme goreleaser and Go
both accept — on top of the ownership decision. The comment above
`release.yml`'s `publish` step is the long form.

## Planned, not built

Three pieces of this are designed and not built: release pull requests issued
automatically, tagging on merge, and arming `release.yml` to publish. All three
are in scope for the consolidation, so the design and what gates each one live
in the plan — the wiki's [[Plan]], under *Ahead*. This file describes what
exists.
