# Build stage - use native platform for faster cross-compilation
FROM --platform=$BUILDPLATFORM golang:1.25-bookworm AS build

ARG TARGETARCH
ARG TARGETOS=linux

WORKDIR /src

# Copy dependency files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with cross-compilation and stripped binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /app github.com/fil-forge/piri-signing-service

FROM alpine:latest AS prod

USER nobody

COPY --from=build /app /usr/bin/signer

EXPOSE 7446

ENTRYPOINT ["/usr/bin/signer"]
CMD ["--host", "0.0.0.0", "--port", "7446"]
