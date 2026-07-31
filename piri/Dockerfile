# Build stage - use native platform for faster cross-compilation
FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS build

ARG TARGETARCH
ARG TARGETOS=linux

# BUILD CONTEXT IS THE REPO ROOT, not this directory:
#
#     docker build -f piri/Dockerfile --target prod .
#
# piri has no intra-repo replace directives, so only its own subtree is copied
# — a change to any sibling service leaves this image's layer cache intact.
# The root go.work is deliberately not copied, keeping the build hermetic on
# piri's own go.mod.
WORKDIR /src

# Copy dependency files first for better layer caching
COPY piri/go.mod piri/go.sum ./piri/
WORKDIR /src/piri
RUN go mod download

# Copy source code
WORKDIR /src
COPY piri/ ./piri/
WORKDIR /src/piri

# Build with cross-compilation and stripped binary
# skiff selects Curio's FFI-free variants (CGO-free build); see Makefile for
# why no other tags are needed.
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -tags "skiff" \
    -ldflags="-s -w" \
    -o /app \
    ./cmd

# Runtime stage - alpine for wget healthchecks per RFC
FROM alpine:latest AS prod

USER nobody

# Copy binary from build stage
COPY --from=build /app /usr/bin/piri

ENTRYPOINT ["/usr/bin/piri"]
