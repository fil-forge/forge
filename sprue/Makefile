VERSION=$(shell awk -F'"' '/"version":/ {print $$4}' version.json)
COMMIT=$(shell git rev-parse --short HEAD)
DATE=$(shell date -u -Iseconds)
# LDFLAGS, not GOFLAGS: make re-exports a variable that was in its own
# environment, so with GOFLAGS already set in the shell `go` parses this
# string as $GOFLAGS and dies on "non-flag".
LDFLAGS=-ldflags="-X github.com/fil-forge/forge/sprue/pkg/build.version=$(VERSION) -X github.com/fil-forge/forge/sprue/pkg/build.Commit=$(COMMIT) -X github.com/fil-forge/forge/sprue/pkg/build.Date=$(DATE) -X github.com/fil-forge/forge/sprue/pkg/build.BuiltBy=make"
DOCKER := $(shell which docker)

.PHONY: all build test lint clean docker-build

all: build

build: sprue

# Rebuild if any Go source files changed
sprue: FORCE
	@if [ ! -f sprue ] || \
	   [ -n "$$(find cmd pkg internal -name '*.go' -type f -newer sprue 2>/dev/null)" ]; then \
		echo "Building sprue..."; \
		go build $(LDFLAGS) -o ./sprue ./cmd/main.go; \
	fi

FORCE:

test:
	go test ./...

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

docker-build:
	$(DOCKER) build -t sprue:latest .

gen:
	go generate ./...
