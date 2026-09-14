# Build stage - use native platform for faster cross-compilation
FROM --platform=$BUILDPLATFORM golang:1.27-bookworm AS build

ARG TARGETOS=linux
ARG TARGETARCH

WORKDIR /src

# Copy dependency files first for better layer caching
COPY go.mod go.sum ./
# proxy.golang.org drops HTTP/2 streams mid-transfer (INTERNAL_ERROR) and the
# go command does not retry, so one bad fetch out of several hundred fails the
# image build. Retried here for the same reason CI retries it in
# .github/scripts/retry.sh; inline because the per-service build contexts do
# not all include .github/.
RUN (go mod download || (sleep 5 && go mod download) || (sleep 15 && go mod download))

# Copy source code
COPY . .

# Build with cross-compilation and stripped binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /app .

FROM alpine:latest AS prod

USER nobody

# Copy binary from build stage
COPY --from=build /app /usr/bin/registrar

EXPOSE 8080

ENTRYPOINT ["/usr/bin/registrar"]
CMD ["serve", "--host", "0.0.0.0", "--port", "8080"]
