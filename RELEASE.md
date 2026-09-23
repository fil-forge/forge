# Releasing

Per service, not per repository. Every service carries its own version and cuts
its own tag; there is no repo-wide version number and no release train.

## The path a change takes

1. **A pull request to `main`.** CI runs unfiltered — every module's
   build/vet/staticcheck/tidy/test, the `guards` job, images, e2e and itest.
   Agent review rounds run until one comes back with nothing (`AGENTS.md`
   rule 10). Merge when it is green.
2. **A release pull request.** Branch it `release/<service>` and bump
   `<service>/version.json`. **The branch name is load-bearing**: `compat.yml`
   runs its compatibility suite only for a head branch starting with
   `release/`, and `compat-refresh.yml` finds the pull requests to refresh the
   same way.
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
dispatch-only (never triggered by a push or a tag), `dry_run` defaults to
true, and it is fail-closed twice over: goreleaser's `publish` and `docker`
steps are skipped, *and* the job's token is `contents: read`, so even if a
future edit drops the skips the release cannot be created.

Arming it means: `contents: write` and `packages: write`, `setup-qemu` and
`setup-buildx`, and dropping `docker` from the skip list. That is a decision
about who owns releases, not a workflow change — see the wiki's *Needs Human
Work*.
