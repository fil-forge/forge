# Build stage - runs natively on build machine, cross-compiles to target
FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS build
ARG TARGETARCH
ARG TARGETOS=linux
# BUILD CONTEXT IS THE REPO ROOT, not this directory:
#
#     docker build -f hilt/Dockerfile --target prod .
#
# hilt has no intra-repo replace directives, so only its own subtree is copied
# — a change to any sibling service leaves this image's layer cache intact.
# The root go.work is deliberately not copied, keeping the build hermetic on
# hilt's own go.mod.
WORKDIR /go/src
COPY hilt/go.mod hilt/go.sum* ./hilt/
WORKDIR /go/src/hilt
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download || true
WORKDIR /go/src
COPY hilt/ ./hilt/
WORKDIR /go/src/hilt

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
