VERSION=$(shell awk -F'"' '/"version":/ {print $$4}' version.json)
COMMIT=$(shell git rev-parse --short HEAD)
DATE=$(shell date -u -Iseconds)
GOFLAGS=-ldflags="-X github.com/fil-forge/forge/sprue/pkg/build.version=$(VERSION) -X github.com/fil-forge/forge/sprue/pkg/build.Commit=$(COMMIT) -X github.com/fil-forge/forge/sprue/pkg/build.Date=$(DATE) -X github.com/fil-forge/forge/sprue/pkg/build.BuiltBy=make"
# go.mod is the source of truth; the workspace is a dev convenience (matches CI).
export GOWORK := off

# Resolve the docker binary to an absolute path: directories named 'docker'
# on PATH (including this repo's own docker/ when '.' is in PATH) break make's
# direct exec of a bare 'docker'.
DOCKER := $(shell command -v docker)

# Container image via the monorepo-shared docker/Dockerfile (context = the repo root).
IMAGE_BUILD_ARGS := --build-arg SERVICE=sprue

.PHONY: all build test vet lint image image-dev clean gen

all: build

build: sprue

# Rebuild if any Go source files changed
sprue: FORCE
	@if [ ! -f sprue ] || \
	   [ -n "$$(find cmd pkg internal -name '*.go' -type f -newer sprue 2>/dev/null)" ]; then \
		echo "Building sprue..."; \
		go build $(GOFLAGS) -o ./sprue ./cmd/main.go; \
	fi

FORCE:

test:
	go test ./...

vet:
	go vet ./...

lint:
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, running go fmt check only"; \
		test -z "$$(gofmt -l .)" || (gofmt -d . && exit 1); \
	fi

clean:
	rm -f ./sprue

image:
	$(DOCKER) build -f ../docker/Dockerfile $(IMAGE_BUILD_ARGS) --target prod -t forge/sprue:local ..

image-dev:
	$(DOCKER) build -f ../docker/Dockerfile $(IMAGE_BUILD_ARGS) --target dev -t forge/sprue:local-dev ..

gen:
	go generate ./...
