# The one service image. Every in-repo Go service (piri, hilt, sprue, ingot)
# builds from this file — there are deliberately no per-service Dockerfiles.
# The services are structurally identical (a single static Go binary listening
# on ports the compose files map), so their images are one parameterized build:
#
#     docker build -f docker/Dockerfile --build-arg SERVICE=<svc> \
#         --target {prod|dev} <svc>/
#
# BUILD CONTEXT IS THE SERVICE DIRECTORY — each service stays a self-contained
# context (scoped by its own .dockerignore), and a change to one service can
# never invalidate another's layer cache. The repo-root go.work is outside
# every context, keeping builds hermetic on each service's go.mod.
#
# Targets:
#   build       deps + source, native toolchain cross-compiling to TARGETARCH
#   build-prod  stripped static binary (-s -w)
#   build-dev   debuggable binary (-N -l: no optimization/inlining) + delve
#   prod        minimal debian runtime + ca-certificates, curl and wget (the
#               compose healthchecks use curl; smelt's generated piri
#               healthcheck uses wget — both ship so no compose file cares)
#   dev         prod + a network/debug toolset and the dlv debugger; published
#               as :main-dev for ad-hoc engineer debugging
#
# Build args:
#   SERVICE     (required) the service name; names the binary at /usr/bin/<svc>
#   MAIN_PKG    main package path (default ./cmd; ingot passes ./cmd/ingot)
#   BUILD_TAGS  go build tags (piri passes "skiff" — selects Curio's FFI-free
#               variants, which is what makes its CGO_ENABLED=0 build possible)
#
# ENTRYPOINT is the bare binary for every service: the compose `command:`
# supplies the subcommand (e.g. serve --config …), and the dev image can be
# launched under dlv by overriding `command`. Exec-form ENTRYPOINT cannot
# expand build args, so the binary lives at /usr/bin/${SERVICE} (where smelt's
# WithServiceBinary mounts over it) with a fixed-name symlink for the
# entrypoint to exec through.

FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS build
ARG TARGETOS=linux
ARG TARGETARCH
WORKDIR /src
# Manifests first, for layer caching.
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .

# ---- production binary: stripped + static ----
FROM build AS build-prod
ARG TARGETOS=linux
ARG TARGETARCH
ARG MAIN_PKG=./cmd
ARG BUILD_TAGS=""
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -tags "${BUILD_TAGS}" -ldflags="-s -w" -o /out/app ${MAIN_PKG}

# ---- development binary: debuggable (no optimization/inlining) + delve ----
FROM build AS build-dev
ARG TARGETOS=linux
ARG TARGETARCH
ARG MAIN_PKG=./cmd
ARG BUILD_TAGS=""
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -tags "${BUILD_TAGS}" -gcflags="all=-N -l" -o /out/app ${MAIN_PKG}
# Cross-compiled `go install` lands in /go/bin/<os>_<arch>/; native in /go/bin.
# Normalize to /go/bin/dlv either way (the cp is a no-op on the native path).
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go install github.com/go-delve/delve/cmd/dlv@latest && \
    cp /go/bin/${TARGETOS}_${TARGETARCH}/dlv /go/bin/dlv 2>/dev/null || true

# ---- production runtime ----
FROM debian:bookworm-slim AS prod
ARG SERVICE
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    wget \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build-prod /out/app /usr/bin/${SERVICE}
RUN ln -s "/usr/bin/${SERVICE}" /usr/local/bin/service-entrypoint
ENTRYPOINT ["/usr/local/bin/service-entrypoint"]

# ---- development runtime: prod + debug tooling + delve ----
FROM debian:bookworm-slim AS dev
ARG SERVICE
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    wget \
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
COPY --from=build-dev /out/app /usr/bin/${SERVICE}
COPY --from=build-dev /go/bin/dlv /usr/bin/dlv
RUN ln -s "/usr/bin/${SERVICE}" /usr/local/bin/service-entrypoint
ENTRYPOINT ["/usr/local/bin/service-entrypoint"]
