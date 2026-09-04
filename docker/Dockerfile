# The one service image. Every in-repo Go service (piri, hilt, sprue, ingot,
# delegator, piri-signing-service, indexing-service) builds from this file —
# there are deliberately no per-service Dockerfiles. The services are
# structurally identical (a single static Go binary listening on ports the
# compose files map), so their images are one parameterized build:
#
#     docker build -f docker/Dockerfile --build-arg SERVICE=<svc> \
#         --target {prod|dev} .
#
# BUILD CONTEXT IS THE REPOSITORY ROOT. Every service reaches the shared
# modules (protocol/, internal/, attestation/) — and some reach their siblings
# (hilt uses smelt's stack harness in its integration tests, ingot uses the
# indexer's query client) — through replace directives in its go.mod,
# and a replace target outside the build context cannot be seen, so the
# context is the whole tree. The root .dockerignore keeps it small. Dependency
# downloads are cached on the manifests layer (every module's go.mod/go.sum,
# copied before any source) and compilation on the go-build cache mount, so a
# rebuild after a one-file change is still incremental even though the source
# COPY covers the tree. go.work is ignored, so each build is hermetic on its
# module's go.mod.
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
#   SERVICE     (required) the service directory; also names the image
#   MAIN_PKG    main package path within the service (default ./cmd; ingot
#               passes ./cmd/ingot, delegator and piri-signing-service pass .)
#   BIN         binary name inside the image (default: SERVICE). The
#               satellites keep the names their compose files and smelt's
#               workspace table expect: delegator -> registrar,
#               piri-signing-service -> signer, indexing-service -> indexer
#   BUILD_TAGS  go build tags (piri passes "skiff" — selects Curio's FFI-free
#               variants, which is what makes its CGO_ENABLED=0 build possible)
#
# ENTRYPOINT is the bare binary for every service: the compose `command:`
# supplies the subcommand (e.g. serve --config …), and the dev image can be
# launched under dlv by overriding `command`. Exec-form ENTRYPOINT cannot
# expand build args, so the binary lives at /usr/bin/${BIN} (where smelt's
# WithServiceBinary mounts over it) with a fixed-name symlink for the
# entrypoint to exec through.

FROM --platform=$BUILDPLATFORM golang:1.27-bookworm AS build
ARG SERVICE
ARG TARGETOS=linux
ARG TARGETARCH
WORKDIR /src
# Manifests first, for layer caching. Every module's go.mod/go.sum is copied,
# not just the service's, because go mod download follows the service's
# replace directives and needs the replaced modules' go.mod files in place.
COPY protocol/go.mod protocol/go.sum ./protocol/
COPY internal/go.mod internal/go.sum ./internal/
COPY attestation/go.mod attestation/go.sum ./attestation/
COPY smelt/go.mod smelt/go.sum ./smelt/
COPY indexing-service/go.mod indexing-service/go.sum ./indexing-service/
COPY ${SERVICE}/go.mod ${SERVICE}/go.sum* ./${SERVICE}/
WORKDIR /src/${SERVICE}
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
WORKDIR /src
COPY . .
# Every later stage builds from inside the service module.
WORKDIR /src/${SERVICE}

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
ARG BIN=${SERVICE}
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    wget \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build-prod /out/app /usr/bin/${BIN}
RUN ln -s "/usr/bin/${BIN}" /usr/local/bin/service-entrypoint
ENTRYPOINT ["/usr/local/bin/service-entrypoint"]

# ---- development runtime: prod + debug tooling + delve ----
FROM debian:bookworm-slim AS dev
ARG SERVICE
ARG BIN=${SERVICE}
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
COPY --from=build-dev /out/app /usr/bin/${BIN}
COPY --from=build-dev /go/bin/dlv /usr/bin/dlv
RUN ln -s "/usr/bin/${BIN}" /usr/local/bin/service-entrypoint
ENTRYPOINT ["/usr/local/bin/service-entrypoint"]
