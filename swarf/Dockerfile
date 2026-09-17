# Build stage - runs natively on build machine, cross-compiles to target.
FROM --platform=$BUILDPLATFORM golang:1.27-bookworm@sha256:648f440f42a0958804efb24df176f806f9d353b41f1c0627f666428e40310f6b AS build
ARG TARGETARCH
ARG TARGETOS=linux
WORKDIR /go/src/swarf
COPY go.mod go.sum* ./
# Retried, and allowed to fail: a swallowed `go mod download` does not avoid
# the network, it just defers the same fetch to `go build` below, where there
# is no retry and the failure is harder to read. Matches the other Dockerfiles
# and is asserted by .github/scripts/check-dockerfile-retry.sh.
RUN --mount=type=cache,target=/go/pkg/mod \
    (go mod download || (sleep 5 && go mod download) || (sleep 15 && go mod download))
COPY . .

# Production build - stripped binary.
FROM build AS build-prod
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /swarf ./cmd/swarf

# Development build - debug-friendly binary.
FROM build AS build-dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -gcflags="all=-N -l" -o /swarf ./cmd/swarf
RUN GOARCH=${TARGETARCH} go install github.com/go-delve/delve/cmd/dlv@latest && \
    cp /go/bin/linux_${TARGETARCH}/dlv /go/bin/dlv 2>/dev/null || cp /go/bin/dlv /go/bin/dlv 2>/dev/null || true

# Production target - minimal runtime.
FROM debian:bookworm-slim@sha256:88200866dfff7ea7f5cbcb6ec7c8a701889efe6fe859fe64d6990e4b07ea4171 AS prod
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build-prod /swarf /usr/bin/swarf
EXPOSE 8080
ENTRYPOINT ["/usr/bin/swarf", "serve"]

# Development target - includes debug tools.
FROM debian:bookworm-slim@sha256:88200866dfff7ea7f5cbcb6ec7c8a701889efe6fe859fe64d6990e4b07ea4171 AS dev
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
COPY --from=build-dev /swarf /usr/bin/swarf
COPY --from=build-dev /go/bin/dlv /usr/bin/dlv
EXPOSE 8080
ENTRYPOINT ["/usr/bin/swarf", "serve"]
