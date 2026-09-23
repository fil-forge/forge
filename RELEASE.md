# Releasing

Per service, not per repository. Every service carries its own version and cuts
its own tag; there is no repo-wide version number and no release train.

## The path a change takes

1. **A pull request to `main`.** CI runs unfiltered — every module's
   build/vet/staticcheck/tidy/test, the `guards` job, images, e2e and itest.
   Agent review rounds run until one comes back with nothing (`AGENTS.md`
   rule 10). Merge when it is green.
2. **A release pull request.** Branch it `release/<service>` and bump
   `<service>/version.json`. **The branch name is load-bearing**: of the pull
   requests that open, `compat.yml` runs its compatibility suite for those
   whose head branch starts with `release/` and no others, and
   `compat-refresh.yml` finds the ones to refresh the same way. (A manual
   dispatch runs it on any ref — the branch name gates the automatic run.)
3. **Merge it**, then **create the tag by hand** (below).
4. **Dispatch `release.yml`** for that service.

## Versions

`<service>/version.json`, one key: `{"version": "v0.2.4"}`. It is the single
source of the version — the tag is checked against it, and goreleaser's `-X`
ldflags stamp it into the binary. Bump it by hand on the release branch.

Semver per service, independently. What counts as a minor versus a patch is
not written down anywhere yet, and the fleet is barely tagged upstream, so
treat the existing numbers as a starting point rather than a precedent.

## Tagging

Tags are `<service>/vX.Y.Z` — `piri/v0.2.5`, not `v0.2.5`. Ten services share
one tag namespace, so an unprefixed tag is ambiguous and a second service
releasing at a version the first already used would collide.

**`release.yml` never creates a tag.** It asserts one exists *and* points at
the commit being built, and fails otherwise. Tagging is deliberately a human
step: it is the point of no return, and the assertion is what stops a release
built from a commit the tag does not name.

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
