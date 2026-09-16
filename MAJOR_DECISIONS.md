# Major decisions

Decisions this repository has made and proposes to keep — the ones a reviewer
would otherwise have to reverse-engineer, and the ones whose first impression
is "that looks wrong". Each says what was decided, what it costs, and what
would make it worth revisiting.

Standing decisions only. Anything temporary — an oddity that exists during the
consolidation and resolves when it finishes — does not belong here even if it
is surprising; that is what makes this list worth trusting later. What is
blocking right now lives on the wiki's **Needs Human Work**, questions deferred
until the monorepo can be seen whole live in `MONOREPO_TODO.md`, and the
consolidation's own transient state lives on the wiki's **Current State**.

---

## Each `itest/` suite is its own Go module

The presenting problem was that `hilt/Dockerfile` needed the **repository root**
as its build context: `hilt/go.mod` carried `replace => ../smelt`, and Go
resolves every replace target before downloading anything, so a context scoped
to `hilt/` died in `go mod download` reading `../smelt/go.mod`.

The real problem was that nothing hilt's binary links has ever lived outside
`hilt/`. The smelt dependency belonged to `itest/` alone, and `//go:build itest`
was doing a module boundary's job — hiding files from the parent module's build.
A module boundary does that properly, so the tag is gone.

**What it cost.** A new module re-opens the version-skew question that unifying
the pins closed: a fresh `go mod tidy` in `ingot/itest` resolved `versitygw` to
a commit still declaring the upstream module path and picked a newer `libforge`
than the unified pin; `hilt/itest` picked a newer `ucantone`. All pinned back
by hand. Expect this every time a module is added.

**What it revealed.** Two of the five sibling `replace` directives were
test-only, not one: both `hilt → smelt` and `ingot → smelt` moved into the
nested `itest` modules. Three remain genuinely linked and keep the root build
context: `ingot → hilt`, `piri → delegator`, `piri → piri-signing-service`.

## Two stack suites, and both run on images built from HEAD

They look redundant — both boot ~20 containers and drive a real deployment —
and the first instinct is to delete one. They are asking different questions.

| | `itest` (`hilt/itest`, `ingot/itest`) | `e2e` (`smelt/tests/e2e`) |
|---|---|---|
| asks | does **this service** honour its own contract? | does the **system** assemble and work? |
| shape | deep and exhaustive on one service | one journey across everything |
| size | 13 test functions in `ingot/itest` alone | `TestUploadAndRetrieve`, `TestStackFromSnapshot` |
| owner | the service | the harness |

`ingot`'s contract is S3 semantics, so its suite is a conformance battery —
`TestForgeVersity` against versitygw, plus encryption, multipart, eviction,
copy authorization, an XFail table. It needs a live stack because ingot's auth
and blob-release behaviour does not exist outside a deployment. **The other
services are scaffolding, not the subject.**

`e2e` earns its place on the axis `itest` structurally cannot see: it builds
real images and leaves `SMELT_WORKSPACE` unset, so a broken Dockerfile, base
image or entrypoint fails it. A bind-mounted binary cannot catch that, and the
gap was not hypothetical — the module-path rewrite never reached the
Dockerfiles, leaving `delegator` and `piri-signing-service` unbuildable since
consolidation, under a green CI.

**Both take their images from this commit.** `e2e` always did; `itest` now
does too, via the same `*_IMAGE` variables and `pkg/stack.OptionsFromEnv`.
Scaffolding that moves under a conformance suite makes its reds ambiguous:
`TestForgeVersity/UploadPartCopy/invalid_part_number` once went red with
nothing in the tree changed, because `ghcr.io/fil-forge/hilt:main` had moved
while our `ingot` sat at `a80bea0`. A red in `itest ingot` should mean ingot
regressed. That one meant somebody else merged, and it cost an investigation
to find out.

`WithPublishedImages()` remains the **local** default, so `make itest` still
boots in a minute or two with no image builds. CI overrides it.

Worth being explicit about what this gives up: nothing here now asks *do we
still work against what is actually deployed?* That question is real and it is
`compat.yml`'s, in Phase 1. It should be asked by a suite built to ask it, not
inherited as a side effect of where the polyrepo happened to keep source —
which is all the published peers ever were.

## `-count=1` on every live-stack test invocation

`setup-go` restores `GOCACHE`, which holds Go's **test-result** cache. Per `go
help test`, a result is reused when the files and environment variables the
test consulted are unchanged — and a Docker stack is neither. The first green
`itest hilt` took 26 seconds and started no containers:

```
ok  github.com/fil-forge/forge/hilt/itest  (cached)
```

So `-count=1` is on the CI invocations *and* on the commands the docs tell
people to paste. The CI paths cannot produce a silent green build; the
documented ones can produce a silent green developer.

## MinIO comes from our own fork

`minio/minio` images were **deleted from Docker Hub** on 2026-09-11 and the
project archived; Quay is affected too. Pinning would not have helped — the
whole repository is gone, so every tag in it is gone.
[`fil-forge/minio`](https://github.com/fil-forge/minio) builds from source and
publishes `ghcr.io/fil-forge/minio:<upstream tag>`, currently
`RELEASE.2025-10-15T17-29-55Z`, which carries a security fix that was never
published as a container. See the wiki's **MinIO Image Removal**.

## The history is full of merges, and is meant to be

`main` carries ~1000 commits and ~60 merges because seven services' full
upstream histories are in it, imported by unsquashed `git subtree add`, and
each `git subtree pull` adds that service's new commits behind another merge.

**`--squash` is never used, on any prefix.** A rebase flattens imported history
after the fact; `--squash` declines to import it at all; both leave a monorepo
that cannot say where its code came from.

The consequence for contributors: when the base moves under a branch carrying
subtree merges, that branch is **rebuilt** — replayed onto the new base with
each `git subtree pull` re-run — not merged and not plainly rebased.
`git rebase --rebase-merges` does *not* do this; it recreates the topology but
re-runs a plain merge that knows nothing about the subtree prefix. The wiki's
**Current State**, rule 7, has the full mechanics and the sweep it requires.

## A module comes in by audience, not by whether it is a service

The previous attempt drew the line at **services**: in, only if it produces a
deployed binary. That is no longer the rule, and the replacement is worth
stating before it is first exercised rather than after.

**In: what ships as the Forge network and what that network is built and
tested with** — deployed together, versioned together, and consumed by nobody
outside. Out, in three categories:

- **Libraries with consumers beyond Forge**, which need real semantic versions
  and their own cadence: `ucantone`, `automobile`. This is the *only* reason a
  library stays out.
- **Forks of upstream software** we patch or repackage — `minio`,
  `storetheindex`, `did-method-plc`, `filecoin-localdev`, `versitygw`,
  `filecoin-services`. Folding these in would destroy what makes them useful:
  upstream history, provenance, and the ability to take upstream changes.
- **Things being retired**: `guppy` is being dismantled and archived rather
  than moved.

**Nothing here exercises the change yet.** Every module currently in the
repository produces a binary — `delegator`, `hilt`, `ingot`, `piri`,
`piri-signing-service`, `smelt`, `sprue` — so "services only" would still
admit all of them. (`smelt` is a partial exception in shape, not in
membership: it ships a CLI but is mostly consumed as a library by both `itest`
suites and the stress-tester. It was in the original attempt too, and being
the test harness it had to be here either way. It is not evidence of a new
rule.)

**`libforge` is what the change is for.** It is still an external dependency,
pinned across the tree at `v0.0.0-20260914100934-63f5b20252fe`, and it
dissolves in a later phase. When it does it splits on exactly this line: the
parts only Forge ever used come in as libraries, and the parts that are
genuinely useful elsewhere become properly versioned libraries outside. Under
services-only there would be nowhere to put the first group.

Why audience rather than artifact: a binary/library test answers a question
about packaging, and the thing that actually hurts is release coupling. A
module with outside consumers must be versioned for them, so it pays the
monorepo's costs and gets nothing back. A module only we consume pays nothing,
whatever it compiles to.

`indexing-service` and `forgectl` are in scope and pending. `swarf` — a
service, with its own Dockerfile and `cmd/swarf` — is landing via
[#3](https://github.com/fil-forge/forge-2/pull/3) and loses its image pin on
arrival, like everything else that moves in.
