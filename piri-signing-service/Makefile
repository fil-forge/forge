# Signing Service Makefile
BINARY_NAME=signer
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(shell date -u -Iseconds)
GOFLAGS=-ldflags="-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: all build test clean install help

# Default target
all: build

# Build the signing service binary
build: $(BINARY_NAME)

$(BINARY_NAME): FORCE
	@if [ ! -f $(BINARY_NAME) ] || \
	   [ -n "$$(find . -name '*.go' -type f -newer $(BINARY_NAME) 2>/dev/null)" ]; then \
		echo "Building $(BINARY_NAME)..."; \
		go build $(GOFLAGS) -o $(BINARY_NAME) .; \
	else \
		echo "$(BINARY_NAME) is up to date"; \
	fi

FORCE:

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	go clean

# Install the binary to GOPATH/bin
install:
	go install $(GOFLAGS) .

# Run the signer (for development)
run: build
	./$(BINARY_NAME)

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Tidy dependencies
tidy:
	go mod tidy

# Show help
help:
	@echo "Signing Service Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make              Build the signing service"
	@echo "  make build        Build the signing service"
	@echo "  make test         Run tests"
	@echo "  make test-coverage Run tests with coverage report"
	@echo "  make clean        Remove build artifacts"
	@echo "  make install      Install binary to GOPATH/bin"
	@echo "  make run          Build and run the signer"
	@echo "  make fmt          Format code"
	@echo "  make lint         Lint code (requires golangci-lint)"
	@echo "  make tidy         Tidy go.mod dependencies"
	@echo "  make help         Show this help message"

# --- monorepo verb contract (see the root Makefile): vet, image, image-dev ---
# Resolve the docker binary to an absolute path: directories named 'docker'
# on PATH (including this repo's own docker/ when '.' is in PATH) break make's
# direct exec of a bare 'docker'.
DOCKER := $(shell command -v docker)

# Container image via the monorepo-shared docker/Dockerfile (context = the repo root).
IMAGE_BUILD_ARGS := --build-arg SERVICE=piri-signing-service --build-arg MAIN_PKG=. --build-arg BIN=signer

.PHONY: vet image image-dev
vet:
	go vet ./...

image:
	$(DOCKER) build -f ../docker/Dockerfile $(IMAGE_BUILD_ARGS) --target prod -t forge/piri-signing-service:local ..

image-dev:
	$(DOCKER) build -f ../docker/Dockerfile $(IMAGE_BUILD_ARGS) --target dev -t forge/piri-signing-service:local-dev ..

