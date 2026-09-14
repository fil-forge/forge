# Build stage - runs natively on build machine, cross-compiles to target
FROM --platform=$BUILDPLATFORM golang:1.27-bookworm AS build
ARG TARGETARCH
ARG TARGETOS=linux
# BUILD CONTEXT IS THE REPOSITORY ROOT, not this directory:
#
#   docker build -f hilt/Dockerfile --target prod .
#
# hilt's go.mod carries `replace => ../smelt`, and Go resolves every replace
# target before it will download anything -- a context scoped to hilt/ dies in
# `go mod download` reading ../smelt/go.mod.
#
# smelt is a TEST-only dependency (hilt/itest/stack_test.go); no smelt code is
# linked into the binary, so its go.mod/go.sum alone satisfy the module graph
# and no smelt source is copied. Consequently a smelt change does not
# invalidate this image. The closure is derived, not hand-maintained:
#
#   cd hilt && GOWORK=off go list -deps ./cmd | grep fil-forge/forge/
#   -> hilt
#
# go.work is deliberately NOT copied: the build must resolve through this
# module's own go.mod, as it did when this was a standalone repo. Copying it
# (or a broad `COPY . .`) would silently switch the build to workspace mode.
WORKDIR /src
COPY hilt/go.mod hilt/go.sum* ./hilt/
COPY smelt/go.mod smelt/go.sum ./smelt/
# Retried, and allowed to fail: a swallowed `go mod download` does not avoid
# the network, it just defers the same fetch to `go build` below, where there
# is no retry and the failure is harder to read. See the other Dockerfiles and
# .github/scripts/retry.sh.
RUN --mount=type=cache,target=/go/pkg/mod \
    cd hilt && (go mod download || (sleep 5 && go mod download) || (sleep 15 && go mod download))
COPY hilt/ ./hilt/
WORKDIR /src/hilt

# Production build - stripped binary
FROM build AS build-prod
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /hilt ./cmd/main.go

# Development build - debug-friendly binary
FROM build AS build-dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -gcflags="all=-N -l" -o /hilt ./cmd/main.go
RUN GOARCH=${TARGETARCH} go install github.com/go-delve/delve/cmd/dlv@latest && \
    cp /go/bin/linux_${TARGETARCH}/dlv /go/bin/dlv 2>/dev/null || cp /go/bin/dlv /go/bin/dlv 2>/dev/null || true

# Production target - minimal runtime
FROM debian:bookworm-slim AS prod
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build-prod /hilt /usr/bin/hilt
EXPOSE 8080
ENTRYPOINT ["/usr/bin/hilt", "serve"]

# Development target - includes debug tools
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
COPY --from=build-dev /hilt /usr/bin/hilt
COPY --from=build-dev /go/bin/dlv /usr/bin/dlv
EXPOSE 8080
ENTRYPOINT ["/usr/bin/hilt", "serve"]
