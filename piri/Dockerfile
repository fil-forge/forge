# piri — storage provider node.
#
# BUILD CONTEXT IS THE REPOSITORY ROOT, not this directory:
#
#   docker build -f piri/Dockerfile --target prod .
#
# piri's go.mod carries `replace => ../delegator` and
# `replace => ../piri-signing-service`. Go resolves every replace target
# before it will download anything, so a context scoped to piri/ dies in
# `go mod download` reading ../delegator/go.mod.
#
# The COPY list below is the module closure of the binary, derived -- do not
# hand-edit it from go.mod's replace list, which is wider:
#
#   cd piri && GOWORK=off go list -deps -tags skiff ./cmd \
#     | grep fil-forge/forge/
#   -> delegator, piri, piri-signing-service
#
# Both siblings are linked into the binary (pkg/pdp/service, pkg/fx/pdp,
# pkg/service/signer), so both need their source, not just their go.mod.
#
# go.work is deliberately NOT copied: the build must resolve through this
# module's own go.mod, as it did when this was a standalone repo. Copying it
# (or a broad `COPY . .`) would silently switch the build to workspace mode.

FROM --platform=$BUILDPLATFORM golang:1.27-bookworm@sha256:648f440f42a0958804efb24df176f806f9d353b41f1c0627f666428e40310f6b AS build

ARG TARGETARCH
ARG TARGETOS=linux

WORKDIR /src

# Dependency manifests first, for layer caching.
COPY piri/go.mod piri/go.sum ./piri/
COPY delegator/go.mod delegator/go.sum ./delegator/
COPY piri-signing-service/go.mod piri-signing-service/go.sum ./piri-signing-service/
# proxy.golang.org drops HTTP/2 streams mid-transfer (INTERNAL_ERROR) and the
# go command does not retry, so one bad fetch out of several hundred fails the
# image build.
RUN cd piri && (go mod download || (sleep 5 && go mod download) || (sleep 15 && go mod download))

# Source code: piri and the two siblings linked into its binary.
COPY piri/ ./piri/
COPY delegator/ ./delegator/
COPY piri-signing-service/ ./piri-signing-service/

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
FROM alpine:latest@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b AS prod

USER nobody

# Copy binary from build stage
COPY --from=build /app /usr/bin/piri

ENTRYPOINT ["/usr/bin/piri"]
