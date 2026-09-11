# MinIO image removal

**Status: resolved.** Opened and closed 2026-09-11.

`minio/minio` has been removed from Docker Hub. It is a hard dependency of
four places in this repository, so `unit piri`, `unit sprue` and the whole of
`e2e` fail on **every branch, `main` included**. Nothing we did caused it.

## What is verified, first-hand

Docker Hub, probed directly (`hub.docker.com/v2/repositories/...`):

| repository | |
|---|---|
| `minio/minio` | **404** `{"message":"object not found"}` |
| `minio/mc` | **404** |
| `minio/console` | **404** |
| `minio/operator` | 200 |
| `minio/sidekick` | 200 |
| org `minio` | 200 |
| control: `library/alpine`, `bitnami/minio` | 200 |

So this is neither a Hub outage nor an org deletion: the server, the client
and the console were withdrawn, while the Kubernetes operator and the
load-balancer remain.

Our own timeline, from CI on one commit:

- **17:30Z** — e2e run pulls `minio/minio:latest` and passes, including
  `TestUploadAndRetrieve/s3`.
- **20:15Z** — `pull access denied for minio/minio, repository does not
  exist or may require 'docker login'`. No container starts.
- **20:27Z** — re-run, identical.

Under three hours between working and gone.

## Two events, ten months apart

This is the thing to get straight, because the public record conflates them:

1. **October 2025 — publishing stopped.** MinIO moved the community edition
   to source-only: no *new* images to Docker Hub or Quay. Widely reported at
   the time ([GIGAZINE](https://gigazine.net/gsc_news/en/20251023-minio-stops-distributing-free-docker-images/),
   [faun.dev](https://faun.dev/c/news/devopslinks/minio-pulls-docker-images-and-documentation-community-calls-move-malicious-and-lock-in-strategy/),
   [r/laravel](https://www.reddit.com/r/laravel/comments/1od75f3/minio_moving_to_sourceonly_no_docker_images/)).
   **This broke nothing for us.** `:latest` still resolved to their last
   push, and the stack kept pulling it for ten months.
2. **2026-09-11 — the existing tags were deleted.** That is what broke CI.

Primary source for (1): [minio/minio#21647](https://github.com/minio/minio/issues/21647),
where a user reports a security release missing from **Quay.io *and*
DockerHub** and maintainers label it `community` / `working as intended`.
Note the Quay half — **Quay is not a fallback**, which was the obvious
one-line fix and would have cost a CI cycle to disprove.

Evidence for (2) being recent is our own CI, and it is an inference rather
than an observed pull, so stated plainly: at 20:15Z compose failed pulling
minio for **all three** tests, `TestUploadAndRetrieve/filesystem` included,
so minio is in the base stack regardless of node config. At 17:30Z the same
compose files ran the same three tests and passed. Nothing between the runs
changed which services compose builds.

### Independent corroboration that the deletion is today's

Two issues filed by unrelated projects on 2026-09-11, minutes either side of
our own failure:

| issue | filed | |
|---|---|---|
| [smartsolutionslab/smart-sentinel-eye#2265](https://github.com/smartsolutionslab/smart-sentinel-eye/issues/2265) | 19:59Z | "MinIO withdrew their Docker Hub organisation today"; 354 tests failed across 1416 pull attempts |
| [Renaissance-Analytics/genie#636](https://github.com/Renaissance-Analytics/genie/issues/636) | 20:14Z | same denial on `minio/minio:latest`, red on `main` |

Different organisations, different stacks (.NET Aspire, Node). The first
published a probe table **identical to the one above** — `library/postgres`
200, `bitnami/minio` 200, `minio/minio` 404, `minio/mc` 404 — reached
independently. Our own last success was 17:30Z.

**They were pulling a pinned tag** (`RELEASE.2025-09-07T16-13-09Z`) and it
failed the same way. That is first-hand confirmation from outside this
project that **pinning would not have helped**: the repository is gone, so
every tag in it is gone.

One correction to their wording: #2265 says the *organisation* was
withdrawn. It was not — `minio/operator` and `minio/sidekick` still answer
200. The server, the client and the console were.

### Registry, not just the web API

Worth separating, since "delisted" and "deleted" are different failures.
Same auth flow, same proxy, seconds apart:

| | |
|---|---|
| `library/alpine:latest` | **200** |
| `minio/minio:latest` | **401** |
| `minio/minio:RELEASE.2025-10-15T17-29-55Z` | **401** |

The control rules out a sandbox or rate-limit artifact — a per-IP rate limit
would have taken alpine down too, and `minio/operator` answered 200 from the
same address in the same second.

### Treat the secondary coverage with care

It is inconsistent on specifics and some of it is written by vendors selling
replacements. One widely-syndicated summary states `bitnami/minio` is "now
deleted"; it answered **HTTP 200** when measured here. Verify before
repeating.

## Where it bites

| site | module | breaks |
|---|---|---|
| `smelt/systems/common/compose.yml:47` | smelt | the whole stack |
| `smelt/pkg/generate/compose.go:192` | smelt | generated piri stack |
| `piri/pkg/internal/testutil/minio.go:24` | piri | `unit piri` |
| `sprue/internal/testutil/s3.go:17` | sprue | `unit sprue` |

Two aggravating details:

- **No indirection.** Every first-party image is overridable
  (`${PIRI_IMAGE:-…}`); minio is hardcoded at all four sites. During an
  outage that is the difference between an environment variable and a code
  change across three modules.
- **Two of the four are in testcontainers calls**, not compose, so they were
  never covered by the image-override thinking at all.

## What we did

**Forked MinIO and build the image ourselves.** `fil-forge/minio` is a fork of
`minio/minio` carrying all 523 upstream release tags; its `Dockerfile` builds
from source and `publish-image.yml` pushes
`ghcr.io/fil-forge/minio:<upstream tag>`. `forge-2` consumes that image and
builds nothing third-party itself.

Pinned to `RELEASE.2025-10-15T17-29-55Z` — upstream's last release, and the
one whose absence as an image opened minio/minio#21647. So the build carries
a security fix that was never published as a container: **ahead of the
`:latest` we had been running, not a restoration of it.**

Why this and not the alternatives:

- **Quay was out** — the primary source reports the release missing there too.
- **Chainguard's free tier and community forks** re-acquire the same class of
  dependency: a free artifact a third party can withdraw. That is what broke,
  twice (publishing stopped Oct 2025; tags deleted today). One of the
  recommended alternatives, Minimus, is itself reportedly shutting down next
  month.
- **Upstream is archived** (2026-04-25), so there is no upstream release to
  track. A fork is not optional scaffolding; it is the only place future
  MinIO maintenance — a CVE patch, say — can happen at all.
- **Source and recipe end up co-located**, so the AGPLv3 Corresponding Source
  is complete in one repository on a server we operate, and the image's
  `org.opencontainers.image.source` label points at it. The package is
  public, which is redistribution; that is permitted, and the labels plus the
  in-image `LICENSE`/`CREDITS`/`NOTICE` are what make it clean rather than
  merely unnoticed.

### Deliberate differences from what we used to pull

- **No `mc`.** Upstream shipped the client in the same image; we used it for
  the compose healthcheck `mc ready local` and nothing else. The healthcheck
  now polls `/minio/health/cluster`, which piri's own testutil had already
  identified as the correct readiness probe because `live` and `ready` answer
  200 before the object layer is up.
- **Runtime is `debian:bookworm-slim`, not upstream's `ubi-micro`.** That
  exact combination — bookworm build, bookworm-slim runtime, upstream's
  entrypoint unmodified — passed the full e2e suite: upload and retrieve over
  both filesystem and S3, plus a three-node snapshot boot. ubi-micro was not
  tested, and their hotfix image targets RedHat certification, which is not
  our requirement.
- **`MINIO_IMAGE` now exists.** MinIO was the one image with no override
  hook: hardcoded at both smelt sites and passed as a literal at both
  testcontainers sites. That is why its disappearance cost a code change
  across three modules instead of an environment variable.

### Things that went wrong on the way, worth not repeating

- **The build was put in the wrong repository first.** `forge-2` grew a
  `third_party/minio/` Dockerfile and published from there before the fork
  existed. That claimed the GHCR package name under `forge-2`, and when the
  build moved to the fork the fork was locked out —
  `denied: permission_denied: write_package` — because GHCR ties a package to
  its creating repository. Fixed by deleting the package and letting the fork
  create it. **Decide where an artifact is produced before publishing it
  anywhere.**
- **A workflow comment claimed the package was private.** Visibility is not
  expressible in a workflow: GHCR inherits it from the repository on first
  publish. Asserting it in a committed file made a guess look verified.
- **Tag pushes were blocked** from the agent environment (403 on
  `refs/tags/*`, `refs/heads/*` fine), so a human pushed the 523 tags. The
  publish workflow is `workflow_dispatch`-only partly because of that bulk
  push: a tag trigger would have fanned out or silently fired for none.
- **Upstream's CI was removed from the fork.** Fourteen of its workflows
  trigger on `pull_request` and one on `push`; enabling Actions to run our
  publish workflow would have armed all of them, against services and secrets
  the fork does not have. A dozen permanently-red checks nobody can fix is
  worse than none — people learn to scroll past red, and then a real failure
  scrolls past too.

## The lesson, stated plainly

This is [[Consolidation Findings]] L6 and B5 arriving at full size, within
hours of being written down as hypotheticals. The entry said an unpinned
third-party tag could take the stack offline. The correction is not "pin it"
— pinning would not have helped, the repository is gone. It is:

> A dependency you do not control and cannot rebuild is a dependency you can
> lose outright. For anything load-bearing, hold a copy you can serve
> yourself.

And a second lesson, which is the quieter one. L6 said floating tags are
dangerous because they **move** under you. The inverse is just as real and
far harder to see: a floating tag that **stops moving** hands you a frozen
dependency while still reading as current. `minio/minio:latest` had not been
republished since October 2025 — by the reporting, with a known high-severity
CVE in that final image (a vendor claim, from sellers of alternatives, though
the identifier is checkable). We ran it for ten months and nothing said a
word.

Today's outage is the better outcome: it turned ten months of silent
staleness into one loud failure. A check on **tag age** would have caught it
in week one; nothing we had was looking.
