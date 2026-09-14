# ingot — embeddable S3 gateway over the Forge network, as a daemon.
#
# Multi-stage, multi-target (mirrors sprue so smelt can treat every forge
# service the same way):
#
#   build       base: fetch deps + source, native toolchain that
#               cross-compiles to the requested TARGETARCH
#   build-prod  stripped, static release binary (-s -w)
#   build-dev   debuggable binary (-N -l: no optimization/inlining) + delve
#   prod        minimal runtime: ca-certificates + curl (smelt's healthcheck
#               hits /health with curl) and the release binary
#   dev         prod + a network/debug toolset and the `dlv` debugger, built
#               from the debuggable binary; publishes as the :main-dev tag
#
# BUILD CONTEXT IS THE REPOSITORY ROOT, not this directory:
#
#   docker build -f ingot/Dockerfile --target prod .
#
# ingot's go.mod carries `replace => ../hilt` and `replace => ../smelt`, and Go
# resolves every replace target before it will download anything, so a context
# scoped to ingot/ dies in `go mod download`. The COPY list is the module
# closure of the binary, derived -- do not hand-edit it from the replace list,
# which is wider:
#
#   cd ingot && GOWORK=off go list -deps ./cmd/ingot | grep fil-forge/forge/
#   -> hilt, ingot
#
# hilt is linked into the binary (bucketauthority, iam, inmem, s3frontend,
# module.go) so its source is copied. smelt is TEST-only (ingot/itest/*): its
# go.mod/go.sum alone satisfy the module graph, so a smelt change does not
# invalidate this image.
#
# go.work is deliberately NOT copied: the build must resolve through this
# module's own go.mod, as it did when this was a standalone repo. Copying it
# (or a broad `COPY . .`) would silently switch the build to workspace mode.
#
# ENTRYPOINT is the bare binary: the compose `command:` supplies the
# subcommand (e.g. serve --config ...), matching the smelt house style, and
# lets the dev image be launched under dlv by overriding `command`.
# To debug the dev image, override the command to exec under delve headless:
#   dlv exec --headless --listen=:2345 --api-version=2 --accept-multiclient \
#     /usr/bin/ingot -- serve --config /etc/ingot/config.yaml
# then attach your client on the mapped 2345 (smelt's compose.debug.yml pattern).

FROM --platform=$BUILDPLATFORM golang:1.27-bookworm AS build
ARG TARGETOS=linux
ARG TARGETARCH
WORKDIR /src
COPY ingot/go.mod ingot/go.sum ./ingot/
COPY hilt/go.mod hilt/go.sum ./hilt/
COPY smelt/go.mod smelt/go.sum ./smelt/
# proxy.golang.org drops HTTP/2 streams mid-transfer (INTERNAL_ERROR) and the
# go command does not retry, so one bad fetch out of several hundred fails the
# image build. Retried here for the same reason CI retries it in
# .github/scripts/retry.sh; inline because the per-service build contexts do
# not all include .github/.
RUN --mount=type=cache,target=/go/pkg/mod \
    cd ingot && (go mod download || (sleep 5 && go mod download) || (sleep 15 && go mod download))
COPY ingot/ ./ingot/
COPY hilt/ ./hilt/
WORKDIR /src/ingot

# ---- production binary: stripped + static ----
FROM build AS build-prod
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /out/ingot ./cmd/ingot

# ---- development binary: debuggable (no optimization/inlining) + delve ----
FROM build AS build-dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -gcflags="all=-N -l" -o /out/ingot ./cmd/ingot
# Cross-compiled `go install` lands in /go/bin/<os>_<arch>/; native in /go/bin.
# Normalize to /go/bin/dlv either way (the cp is a no-op on the native path).
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go install github.com/go-delve/delve/cmd/dlv@latest && \
    cp /go/bin/${TARGETOS}_${TARGETARCH}/dlv /go/bin/dlv 2>/dev/null || true

# ---- production runtime ----
FROM debian:bookworm-slim AS prod
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build-prod /out/ingot /usr/bin/ingot
# S3 listener: :8080 by default, the port every Forge service image exposes.
# smelt overrides addr to :80 in the mounted config so did:web:ingot resolves
# to http://ingot/.well-known/did.json, and maps that port in compose.
EXPOSE 8080
ENTRYPOINT ["/usr/bin/ingot"]

# ---- development runtime: prod + debug tooling + delve ----
FROM debian:bookworm-slim AS dev
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    bash-completion \
    less \
    vim-tiny \
    procps \
    htop \
    strace \
    iputils-ping \
    dnsutils \
    net-tools \
    tcpdump \
    jq \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build-dev /out/ingot /usr/bin/ingot
COPY --from=build-dev /go/bin/dlv /usr/bin/dlv
# S3 listener (see the prod target) + delve headless port.
EXPOSE 8080 2345
ENTRYPOINT ["/usr/bin/ingot"]
