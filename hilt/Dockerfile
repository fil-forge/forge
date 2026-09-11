# Build stage - runs natively on build machine, cross-compiles to target
FROM --platform=$BUILDPLATFORM golang:1.27-bookworm AS build
ARG TARGETARCH
ARG TARGETOS=linux
# Build context is this directory:
#
#   docker build -f hilt/Dockerfile --target prod hilt
#
# It was the repository root until hilt/itest became its own module. hilt's
# go.mod carried `replace => ../smelt` for that suite alone, and Go resolves
# every replace target before it downloads anything, so a context scoped here
# died in `go mod download` reading ../smelt/go.mod. Nothing hilt's binary
# links has ever lived outside this directory -- the module boundary is what
# let the context say so.
#
# swarf IS linked into the binary (the revocation client), so this context is
# the repository root again and swarf's source is copied. #4 had narrowed it
# to hilt/ because hilt's only sibling dependency was smelt, and that one is
# test-only -- it lives in hilt/itest now and is absent from this module's
# graph entirely, so no smelt anything is copied here.
#
# The closure is derived, not hand-maintained:
#
#   cd hilt && GOWORK=off go list -deps ./cmd/main.go | grep fil-forge/forge/
#   -> hilt, swarf
#
# go.work is deliberately NOT copied: the build must resolve through this
# module's own go.mod, as it did when this was a standalone repo. Copying it
# (or a broad `COPY . .`) would silently switch the build to workspace mode.
WORKDIR /src
COPY hilt/go.mod hilt/go.sum ./hilt/
COPY swarf/go.mod swarf/go.sum ./swarf/
# proxy.golang.org drops HTTP/2 streams mid-transfer (INTERNAL_ERROR) and the
# go command does not retry, so one bad fetch out of several hundred fails the
# image build.
RUN --mount=type=cache,target=/go/pkg/mod \
    cd hilt && (go mod download || (sleep 5 && go mod download) || (sleep 15 && go mod download))
COPY hilt/ ./hilt/
COPY swarf/ ./swarf/
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
