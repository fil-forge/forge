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
# BUILD CONTEXT IS THE REPO ROOT, not this directory:
#
#     docker build -f ingot/Dockerfile --target prod .
#
# ingot's go.mod carries `replace` directives pointing at ../hilt and ../smelt,
# so those sibling modules must exist in the context before `go mod download`
# can resolve them. Only the three modules ingot actually needs are copied, so
# a change to piri or sprue does not invalidate this image's layer cache.
#
# The build stays hermetic on go.mod: the root go.work is deliberately NOT
# copied in, so the container build resolves exactly what a GOWORK=off build
# resolves. Don't add it.
#
# ENTRYPOINT is the bare binary: the compose `command:` supplies the subcommand
# (e.g. serve --config ...), matching the smelt house style, and lets the dev
# image be launched under dlv by overriding `command`.
#
# To debug the dev image, override the command to exec under delve headless:
#   dlv exec --headless --listen=:2345 --api-version=2 --accept-multiclient \
#     /usr/bin/ingot -- serve --config /etc/ingot/config.yaml
# then attach your client on the mapped 2345 (smelt's compose.debug.yml pattern).

FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS build
ARG TARGETOS=linux
ARG TARGETARCH
WORKDIR /src
# Manifests first, for layer caching — including the replace targets, whose
# go.mod files must be present for module resolution to succeed.
COPY ingot/go.mod ingot/go.sum ./ingot/
COPY hilt/go.mod hilt/go.sum ./hilt/
COPY smelt/go.mod smelt/go.sum ./smelt/
WORKDIR /src/ingot
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
WORKDIR /src
COPY hilt/ ./hilt/
COPY smelt/ ./smelt/
COPY ingot/ ./ingot/
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
# S3 listener (override addr in config; container should bind 0.0.0.0).
EXPOSE 9000
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
# S3 listener + delve headless port (used when launched under dlv).
EXPOSE 9000 2345
ENTRYPOINT ["/usr/bin/ingot"]
