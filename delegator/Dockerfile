# Build stage - use native platform for faster cross-compilation
FROM --platform=$BUILDPLATFORM golang:1.27-bookworm@sha256:648f440f42a0958804efb24df176f806f9d353b41f1c0627f666428e40310f6b AS build

ARG TARGETOS=linux
ARG TARGETARCH

# Build context is the repository root:
#
#   docker build -f delegator/Dockerfile --target prod .
#
# delegator's binary links forgectl (the chain, inspector, operator and types
# services), which resolves through `replace => ../forgectl`. Go reads every
# replace target before it downloads anything, so a context scoped to
# delegator/ dies in `go mod download`. piri, hilt and ingot are here for the
# same reason.
#
# The closure is derived, not hand-maintained:
#
#   cd delegator && GOWORK=off go list -deps . | grep fil-forge/forge/
#   -> delegator, forgectl
#
# go.work is deliberately NOT copied: the build must resolve through this
# module's own go.mod, as it did when this was a standalone repo. Copying it
# (or a broad `COPY . .` from the root) would silently switch it to workspace
# mode.
WORKDIR /src

# Copy dependency files first for better layer caching
COPY delegator/go.mod delegator/go.sum ./delegator/
COPY forgectl/go.mod forgectl/go.sum ./forgectl/
# proxy.golang.org drops HTTP/2 streams mid-transfer (INTERNAL_ERROR) and the
# go command does not retry, so one bad fetch out of several hundred fails the
# image build.
RUN cd delegator && (go mod download || (sleep 5 && go mod download) || (sleep 15 && go mod download))

# Copy source code
COPY delegator/ ./delegator/
COPY forgectl/ ./forgectl/
WORKDIR /src/delegator

# Build with cross-compilation and stripped binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /app .

FROM alpine:latest@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b AS prod

USER nobody

# Copy binary from build stage
COPY --from=build /app /usr/bin/registrar

EXPOSE 8080

ENTRYPOINT ["/usr/bin/registrar"]
CMD ["serve", "--host", "0.0.0.0", "--port", "8080"]
