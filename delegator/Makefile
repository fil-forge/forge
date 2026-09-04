.PHONY: build test clean run help

# Default target - just build
build:
	go build -o bin/delegator ./main.go

test:
	go test -v ./...

clean:
	rm -rf bin/
	go clean

run: build
	./bin/delegator

help:
	@echo "Available targets:"
	@echo "  make        - Build the application (default)"
	@echo "  make build  - Build the application"
	@echo "  make test   - Run all tests"
	@echo "  make clean  - Clean build artifacts"
	@echo "  make run    - Build and run the application"
	@echo "  make help   - Show this help message"
# --- monorepo verb contract (see the root Makefile): vet, image, image-dev ---
# Resolve the docker binary to an absolute path: directories named 'docker'
# on PATH (including this repo's own docker/ when '.' is in PATH) break make's
# direct exec of a bare 'docker'.
DOCKER := $(shell command -v docker)

# Container image via the monorepo-shared docker/Dockerfile (context = the repo root).
IMAGE_BUILD_ARGS := --build-arg SERVICE=delegator --build-arg MAIN_PKG=. --build-arg BIN=registrar

.PHONY: vet image image-dev
vet:
	go vet ./...

image:
	$(DOCKER) build -f ../docker/Dockerfile $(IMAGE_BUILD_ARGS) --target prod -t forge/delegator:local ..

image-dev:
	$(DOCKER) build -f ../docker/Dockerfile $(IMAGE_BUILD_ARGS) --target dev -t forge/delegator:local-dev ..

