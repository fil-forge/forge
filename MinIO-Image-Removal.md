# MinIO image removal

**Status: open — blocking, needs a decision.** Opened 2026-09-11.

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

## Options

Not chosen — this needs a human call. Vendor claims are the vendors' own.

1. **Chainguard `minio` / `minio-client`.** Chainguard states these are on
   its free tier with no account approval, built from source. Closest to a
   drop-in, if the entrypoint and the `mc ready local` healthcheck match.
2. **Community fork, e.g. `pgsty/minio`.** Reported to be a CVE-patched
   continuation. Trust and longevity are the open questions.
3. **Build from source and mirror into `ghcr.io/fil-forge`.** Most control,
   permanently immune to a repeat, and the only option that does not add a
   new third party. Costs a build pipeline and an owner.
4. **Replace S3-in-tests entirely** (moto, s3proxy, seaweedfs, …). Cheapest
   to adopt, but ingot's S3 conformance suite cares about fidelity, so this
   trades a supply-chain problem for a correctness one.
5. **Split the decision**: something quick for `piri`/`sprue` unit tests,
   something faithful for the stack. Two dependencies instead of one.

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
