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

## What the sources say

Primary: [minio/minio#21647](https://github.com/minio/minio/issues/21647) —
a user reports a security release (`RELEASE.2025-10-15T17-29-55Z`) missing
from **Quay.io *and* DockerHub**. Maintainers labelled it `community` and
**`working as intended`**, and closed it.

That the report names Quay too is the important part: **Quay is not a
fallback.** It was the obvious one-line fix and it would have burned a CI
cycle to discover.

Secondary, several independent outlets, consistent with each other: MinIO
moved the community edition to source-only distribution, announced around
October 2025, and has since archived the repository. These are summaries of
the change, not statements by MinIO; treat the details as second-hand.

- [GIGAZINE](https://gigazine.net/gsc_news/en/20251023-minio-stops-distributing-free-docker-images/)
- [faun.dev roundup](https://faun.dev/c/news/devopslinks/minio-pulls-docker-images-and-documentation-community-calls-move-malicious-and-lock-in-strategy/)
- [DevPro](https://devpro.fr/minio-container-images-gone-best-alternatives-2025/)

One discrepancy worth keeping in view: the announcement is dated to late
2025, but the image was pullable here until today. The consistent reading is
that publishing **new** images stopped then, and the repository itself was
**deleted** much later. We have first-hand evidence only for the deletion
being recent.

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
