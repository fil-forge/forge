# Current state

**Snapshot as of 2026-09-14.** Replace this page as things change; do not
append to it. For history and reasoning, see [[Consolidation Findings]].

Consolidating the Fil Forge polyrepo into a monorepo at
[`fil-forge/forge-2`](https://github.com/fil-forge/forge-2), which becomes
`forge` at the end. The plan is `forge-consolidation-plan.md` (Phases 0–6).

## Approach

Five rules that have actually decided things:

1. **What goes in: things that ship as the Forge network.** The services and
   the tools that operate them — deployed together, versioned together, and
   not built by anyone outside. Three categories stay out:

   - **Outward-facing libraries**, which have consumers beyond Forge and so
     want real semantic versions and their own cadence: `ucantone`,
     `automobile`. When `libforge` dissolves, it splits on this line — the
     parts that were only ever private to Forge come in; the parts that are
     externally useful become properly versioned libraries outside.
   - **Forks of upstream software** we patch or repackage: `minio`,
     `storetheindex`, `did-method-plc`, `filecoin-localdev`, `versitygw`,
     `filecoin-services`. Folding these in would destroy what makes them
     useful — upstream history, provenance, and the ability to take upstream
     changes.
   - **Things being retired**, which are not worth moving: `guppy` is
     archived in Phase 4 once its client is consolidated.

   In: `piri`, `hilt`, `ingot`, `sprue`, `smelt`, `delegator`,
   `piri-signing-service` (all landed), `swarf` (landed), `indexing-service`
   and `forgectl` (pending).

   This rule has a useful side effect: anything moving in stops being an
   external dependency, so it needs no image pin — see rule 2.

2. **Build what we own; pin what we don't — this is about container images.**
   In-repo services are built from HEAD in CI; external images get digest
   pins. A module moving in retires its pin by construction, so don't pin
   something that is about to arrive (rule 1).

   Go modules follow the same principle, but Go already enforces it:
   `replace => ../<svc>` for in-repo (always the matching commit), and
   `go.mod` + `go.sum` for external (a digest pin by another name). The rule
   needs stating for images precisely because the ergonomics are inverted —
   Go makes the correct thing the default and will not let you depend on a
   moving external version, while Docker makes `:latest` and `:main` the
   default and nothing complains. The instinct "dependencies are pinned,
   that's handled" is true in Go and silently false in Docker.

   Go's separate problem is *agreement*, not reproducibility: modules can be
   pinned to reproducibly-different versions of the same library, which is
   how a six-week ucantone wire skew survived a green CI. That is what
   unifying the library pins fixed.
3. **Derive dependency lists, never hand-maintain them.** `go list -deps` on
   the build target, not `go.mod`'s replace list. It is sometimes narrower
   and sometimes wider than the obvious guess, and it cannot go stale.
4. **Test what ships.** A bind-mounted binary in someone else's image is not
   the artifact that reaches production.
5. **A green check is a claim about what ran.** Ask what the job would have
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
