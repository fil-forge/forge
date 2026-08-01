# Developing Against The Working Tree

Code enters the smelt stack as **container images, never as mounted binaries**.
Working on a service (or a cross-service change — the monorepo makes that one
commit) and validating it in the stack means building the in-repo images from
your working tree and pointing the stack at them.

## What this gives you

- The artifact you test is the artifact that ships: your code AND its
  packaging (Dockerfile, base image, entrypoint) — not a host-compiled binary
  smuggled into yesterday's image.
- One code path everywhere: `make up-local`, `make itest`, and CI's stack job
  all differ only in image tags.

## Run the stack on your working tree

```bash
make up-local    # builds forge/<svc>:local for piri/hilt/sprue/ingot, then `up`
```

`up-local` runs the root `make images` (each service's `image` target builds
via the shared `docker/Dockerfile` with the service directory as context) and
starts the stack with the `*_IMAGE` compose vars pointing at the local tags.
A plain `make up` runs published `:main` images, exactly as before.

## Fast per-edit loop

Once the stack is up you don't need to re-boot it for a one-line change —
rebuild the affected service's image and recreate its container. Deps-first
layer caching makes the rebuild seconds after the first build; the rest of the
stack (and the chain state) stays up:

```bash
make -C ../piri image
PIRI_IMAGE=forge/piri:local docker compose up -d --force-recreate piri-0
```

## Stack test suites

`make test-s3` and `make test-e2e` (root: `make itest` / `make e2e`) build the
four local images and run the suites against them. The s3 suite hard-fails if
the `*_IMAGE` env is missing rather than fall back to published images — a
local run can never silently test `:main`.

```bash
make test-s3                                    # the whole S3 system suite
cd tests/s3 && PIRI_IMAGE=forge/piri:local HILT_IMAGE=forge/hilt:local \
  INGOT_IMAGE=forge/ingot:local UPLOAD_IMAGE=forge/sprue:local \
  GOWORK=off go test -tags itest -run TestForgeVersity/PutObject -v ./...
```

Config files are the one thing tests may mount (`stack.WithServiceConfig`):
they are data, not code.

## Troubleshooting

- **A service won't come up healthy:** that's your local image running — check
  `docker compose logs -f <service>`. The injection is working; the failure is
  in the code (or its packaging, which is the point).
- **Old code seems to be running:** confirm the container's image
  (`docker compose images <svc>`) is the `:local` tag and rebuild it —
  `make -C ../<svc> image` is cheap.

## Service repos own their e2e tests (smelt as SDK)

The local-images flow above is for validating changes *inside smelt runs*. The
complementary direction is an out-of-repo service importing **smelt as a Go
test dependency** and orchestrating the stack from its own e2e tests:

```go
s := stack.MustNewStack(t,
    stack.WithPiriNodes(stack.PiriNodeConfig{}),
    // Run the working tree's container image. BuildGuppyImage builds it from
    // the repo's own Dockerfile; the returned tag is cleaned up with the test.
    stack.WithGuppyImage(stack.BuildGuppyImage(t, "..")),
    // Optional: exercise a config change without a smelt release. Config
    // files are data, not code — the one thing tests may mount.
    stack.WithServiceConfig("ingot", "testdata/config.yaml"),
)
```

Everything travels with the Go import — compose files, per-service configs, and embedded
snapshots ride along via `go:embed`, so no smelt checkout is required. Add a `TestMain` that
calls `stack.CleanupLeaked` to sweep `smeltery-*` leftovers from crashed runs, and
`guppy.LoginViaEmail` for flows that need a logged-in agent.

> **One e2e run per host at a time.** `CleanupLeaked` sweeps *every* `smeltery-*` container,
> including live ones — concurrently starting another repo's e2e suite on the same Docker
> host will tear the first run's stacks out from under it mid-boot.

**Division of responsibility:** smelt owns each service's *system definition* (compose
topology, ports, default config, key wiring) and asserts it boots healthy. Where the
*behavior* tests live depends on where the service lives. An out-of-repo service (guppy,
delegator) keeps them in its own repo, importing `smelt/pkg/stack` at a pseudo-version of
the monorepo. An in-repo service's system suite lives in smelt's `tests/` directory —
`tests/s3/` (ingot's S3 conformance partition + scenarios, gated by the `itest` build tag,
run via `make test-s3`) is the reference implementation — because those suites exercise the
entire stack at one commit and belong to the harness, not to any one service's module.
