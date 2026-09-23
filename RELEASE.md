# Releasing

Per service, not per repository. Every service carries its own version and cuts
its own tag; there is no repo-wide version number and no release train.

## The path a change takes

**Most changes only take the first step.**

1. **A pull request to `main`.** CI runs unfiltered — every module's
   build/vet/staticcheck/tidy/test, the `guards` job, images, e2e and itest.
   Agent review rounds run until one comes back with nothing (`AGENTS.md`
   rule 10). Merge when it is green. That is the whole path for an ordinary
   change: nothing about it is per-release, and it waits for no release.

2. **A release pull request, when a module is ready to ship.** It contains
   **only the version bump** — one edit to `<service>/version.json` and
   nothing else. Merging it means exactly one thing: *the current `main` of
   that module is now version `vX.Y.Z`*. It has no content of its own, so it
   sweeps up every change to that module merged since its last release. There
   is no cherry-picking, no release branch to maintain, and no second pull
   request for an ordinary change.

   Branch it `release/<service>`. **The name is load-bearing**: of the pull
   requests that open, `compat.yml` runs its compatibility suite for those
   whose head branch starts with `release/` and no others, and
   `compat-refresh.yml` finds the ones to refresh the same way. (A manual
   dispatch runs it on any ref — the name gates the automatic run.)

3. **Merging it is the release decision.** The tag and the release build
   follow from that merge — today by hand; see *Tagging*.

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

- **Release pull requests issued automatically.** A module needs one once it
  has been touched since its last release, so what has to be derived is which
  commits touch which module since `<service>/vX.Y.Z`. Leaving it to a person
  means leaving it to someone noticing, which is the failure this repository
  keeps repeating.
- **Tagging on merge of the release pull request**, per *Tagging* above.
- **Arming `release.yml` to publish**, per *What is not armed* above — the
  largest of the three, because it is blocked on the tag-namespace decision
  and not only on wiring.
