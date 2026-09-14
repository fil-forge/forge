# Current state

**Snapshot as of 2026-09-14.** Replace this page as things change; do not
append to it. For history and reasoning, see [[Consolidation Findings]].

Consolidating the Fil Forge polyrepo into a monorepo at
[`fil-forge/forge-2`](https://github.com/fil-forge/forge-2), which becomes
`forge` at the end. The plan is `forge-consolidation-plan.md` (Phases 0–6).

## Approach

Four rules that have actually decided things:

1. **Build what we own; pin what we don't.** In-repo services are built from
   HEAD in CI. External images get digest pins. A module moving in retires
   its pin by construction — so don't pin something that is about to arrive.
2. **Derive dependency lists, never hand-maintain them.** `go list -deps` on
   the build target, not `go.mod`'s replace list. It is sometimes narrower
   and sometimes wider than the obvious guess, and it cannot go stale.
3. **Test what ships.** A bind-mounted binary in someone else's image is not
   the artifact that reaches production.
4. **A green check is a claim about what ran.** Ask what the job would have
   had to *do* to catch the fault.

## Where it stands

**`forge-2` `main` is at `046807f`** — Phase 0 complete: 7 services
subtree-merged with history, module paths rewritten, `go.work`, per-module
CI, library pins unified.

Three branches, **all green on `ci` / `images` / `e2e`, none merged**. They
stack in this order:

| branch | what |
|---|---|
| `claude/images-from-head` | stack runs on images built from HEAD, not the polyrepos' daily builds; fixes 2 Dockerfiles unbuildable since consolidation; minio repoint |
| `claude/pin-guppy` | guppy pinned by digest (the e2e driver; republished on every push to its main) |
| `claude/bring-in-swarf` | swarf subtree-merged — the 8th module |

`pin-guppy` and `bring-in-swarf` are independent of each other; either can
merge first once `images-from-head` lands.

**[`fil-forge/minio`](https://github.com/fil-forge/minio)** — our fork.
Upstream withdrew every image from Docker Hub on 2026-09-11 and archived the
project. We build from source and publish
`ghcr.io/fil-forge/minio:<upstream tag>`, currently
`RELEASE.2025-10-15T17-29-55Z`, via a dispatch-only workflow. See
[[MinIO Image Removal]].

**Polyrepos**, repointed at that image: `smelt` **merged**;
[indexing-service#96](https://github.com/fil-forge/indexing-service/pull/96),
[piri#123](https://github.com/fil-forge/piri/pull/123),
[sprue#97](https://github.com/fil-forge/sprue/pull/97) open and green.

## Next

1. **Merge the three `forge-2` branches.** Everything else builds on them.
2. **Merge the three open polyrepo PRs.**
3. **Bring in `indexing-service`.** Scouted: one consumer (`ingot`), no
   in-repo dependencies, single module, single Dockerfile. Wants #96 merged
   first so the subtree does not import a dead MinIO reference. Re-derive
   `ingot`'s closure — its docs suggest the indexer client may be
   test-only, which changes whether its Docker context needs source or only
   a `go.mod` stub.
4. **`forgectl`** — in scope, a CLI rather than a stack service, so it
   touches none of the image work. Any time.
5. **Phase 1** — release tags, `compat.yml`, publishing. Closer than it was;
   the image machinery now exists.

## Known debt

- `Dockerfile.release` (hilt, ingot, sprue) has the build-context problem the
  main Dockerfiles had, and nothing builds it. Surfaces at the first release.
- Base images float in our own Dockerfiles (`alpine:latest`,
  `debian:bookworm-slim`, `golang:1.27-bookworm`).
- `plc`, `storetheindex`, `filecoin-localdev` still float, but barely move —
  pin when convenient.
- Old `fil-forge/forge` still references the dead MinIO image. Superseded;
  left alone deliberately.
- No **image-age check** anywhere. Every image failure so far would have been
  visible months earlier from "when was this tag last pushed".
- Per-service `CLAUDE.md`/`AGENTS.md` still describe polyrepo reality; 13
  stale module paths in docs. Best swept at the `forge-2` → `forge` rename.
