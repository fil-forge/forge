# Current state

**Snapshot as of 2026-09-14.** Replace this page as things change; do not
append to it. For history and reasoning, see [[Consolidation Findings]].

Consolidating the Fil Forge polyrepo into a monorepo at
[`fil-forge/forge-2`](https://github.com/fil-forge/forge-2), which becomes
`forge` at the end. The plan is `forge-consolidation-plan.md` (Phases 0–6).

## Approach

Seven rules that have actually decided things:

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
     being dismantled and archived now, not at some later phase.

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
   - Corollary, learned three times in one afternoon: when you fix one
     instance, run a command that enumerates the class before committing.
     Careful reading missed the rest every time; `gofmt -l` and
     `check-dockerfile-retry.sh` found them instantly. See
     [[Consolidation Findings]] L11.
   - And the same question applies to the guards themselves: a guard over
     *part* of a chain reads exactly like a guard over the chain, so it
     converts "unchecked" into "checked" for free. `check-image-lists.sh`
     shipped that way (L12).
6. **Stacked branches rebase onto their base; they do not merge it.** A merge
   buries the branch's own commits under someone else's and makes the PR diff
   grow every time the base moves. One exception, and it is load-bearing:
   `git subtree add` produces a merge commit that *carries* the imported
   history, and a plain `git rebase` silently flattens it. Rebuild those
   branches instead — base tip, fresh `git subtree add` at the same upstream
   commit, then cherry-pick — and check afterwards that the tree is unchanged
   and the upstream root is still an ancestor.

7. **Subtree history is never squashed.** No `git subtree add --squash`, no
   `git subtree pull --squash`, on any prefix, ever. Same principle as rule
   6's exception, different mechanism: a rebase flattens imported history
   after the fact, `--squash` declines to import it in the first place, and
   both end with a monorepo that cannot say where its code came from. The
   cost is visible and is meant to be paid — `main` carries 1010 commits and
   59 merges because seven services' full histories are in it, and each
   `git subtree pull` adds that service's new commits behind a merge. That is
   the feature.

   The corollary that actually bites: because the modes are not
   interchangeable, a branch that carries subtree merges is *merged* when its
   base moves, not rebased — the one place rule 6 inverts.

## Where it stands

Four guards now run in `ci.yml`: `check-replaces.sh`,
`check-image-lists.sh`, `check-setup-go-cache.sh` and
`check-dockerfile-retry.sh` — each written after a defect that CI could not
see.

**`forge-2` `main` is at `046807f`** — Phase 0 complete: 7 services
subtree-merged with history, module paths rewritten, `go.work`, per-module
CI, library pins unified.

Two open PRs, **both green on `ci` / `images` / `e2e`, neither merged**.
#3 stacks on #1:

| PR | branch | what |
|---|---|---|
| [#1](https://github.com/fil-forge/forge-2/pull/1) | `claude/images-from-head` | stack runs on images built from HEAD, not the polyrepos' daily builds; fixes 2 Dockerfiles unbuildable since consolidation; minio repoint; restores the Go module cache, which had never worked |
| [#3](https://github.com/fil-forge/forge-2/pull/3) | `claude/bring-in-swarf` | swarf subtree-merged — the 8th module |

#3 is rebased onto #1 (rule 6) rather than carrying merges from it.

**#2 (pin guppy by digest) was closed unmerged.** guppy is being dismantled
and archived and the remaining phases are expected to complete within the
week, so the tag cannot move under us in that window. Two claims on it were
also wrong on inspection: the reviewer's warning that registry pruning would
break the pin (no cleanup action, no retention pattern across the org), and
the PR's own "republished on every push to main — 39 in 90 days". guppy's
dependabot auto-merges use `secrets.GITHUB_TOKEN`, which does not trigger
workflow runs, so those commits publish nothing: `:main-dev` last moved
2026-08-21. A side effect, left alone deliberately: `ghcr.io/fil-forge/guppy:main`
does not reflect guppy's `main`.

**Every CI red so far was one external fault.** Seven `proxy.golang.org`
`INTERNAL_ERROR` stream drops, across build, `go mod tidy` and test-compile —
no code failure among them. #1 now sets `cache-dependency-path` (the cache had
silently never been on) and retries dependency resolution, which is the part
that actually stops the red. All three branches are green, and the cache saved
440 MB for the first time in the repository's life. The Docker-side fetch was
left unretried as "residual"; it failed twenty minutes later and is now
retried too. See [[Consolidation Findings]] L9.

`bring-in-swarf` merges once `images-from-head` lands.

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
