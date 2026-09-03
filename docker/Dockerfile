# The one service image. Every in-repo Go service (piri, hilt, sprue, ingot)
# builds from this file — there are deliberately no per-service Dockerfiles.
# The services are structurally identical (a single static Go binary listening
# on ports the compose files map), so their images are one parameterized build:
#
#     docker build -f docker/Dockerfile --build-arg SERVICE=<svc> \
#         --target {prod|dev} .
#
# BUILD CONTEXT IS THE REPOSITORY ROOT. It used to be the service directory,
# which kept each service a self-contained context. That stopped being possible
# when the wire contract (commands/) and the repo-private helpers (internal/)
# became in-repo modules that every service reaches through a `replace
# ../commands` / `replace ../internal` in its go.mod: a sibling directory is
# outside a per-service context, so the image build could not see it. The root
# .dockerignore keeps the context small; layer caching is preserved because the
# COPY steps below hash only the paths they copy — a change to piri/ does not
# invalidate hilt's layers, but a change to commands/ or internal/ invalidates
# every service's, which is the correct dependency. go.work stays out of the
# image (ignored), so the build is still hermetic on each module's go.mod.
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

FROM --platform=$BUILDPLATFORM golang:1.27-bookworm AS build
ARG SERVICE
ARG TARGETOS=linux
ARG TARGETARCH
WORKDIR /src
# Manifests first, for layer caching: the service's own, plus those of the
# in-repo modules its go.mod replaces to (go mod download resolves the replace
# targets and needs their go.mod files present).
COPY commands/go.mod commands/go.sum ./commands/
COPY internal/go.mod internal/go.sum ./internal/
COPY ${SERVICE}/go.mod ${SERVICE}/go.sum* ./${SERVICE}/
WORKDIR /src/${SERVICE}
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
WORKDIR /src
COPY commands ./commands
COPY internal ./internal
COPY ${SERVICE} ./${SERVICE}
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
