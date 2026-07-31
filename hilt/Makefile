BINARY := hilt
BIN_DIR := .
CMD := ./cmd

VERSION=$(shell awk -F'"' '/"version":/ {print $$4}' version.json)
COMMIT=$(shell git rev-parse --short HEAD)
DATE=$(shell date -u -Iseconds)
GOFLAGS=-ldflags="-X github.com/fil-forge/forge/hilt/pkg/build.version=$(VERSION) -X github.com/fil-forge/forge/hilt/pkg/build.Commit=$(COMMIT) -X github.com/fil-forge/forge/hilt/pkg/build.Date=$(DATE) -X github.com/fil-forge/forge/hilt/pkg/build.BuiltBy=make"

# go.mod is the source of truth; the workspace is a dev convenience (matches CI).
export GOWORK := off

# Resolve the docker binary to an absolute path: directories named 'docker'
# on PATH (including this repo's own docker/ when '.' is in PATH) break make's
# direct exec of a bare 'docker'.
DOCKER := $(shell command -v docker)

# Container image via the monorepo-shared docker/Dockerfile (context = this dir).
IMAGE_BUILD_ARGS := --build-arg SERVICE=hilt

.PHONY: build test vet image image-dev clean gen

build:
	go build $(GOFLAGS) -o $(BIN_DIR)/$(BINARY) $(CMD)

gen:
	go generate ./...

test:
	go test ./...

vet:
	go vet ./...

image:
	$(DOCKER) build -f ../docker/Dockerfile $(IMAGE_BUILD_ARGS) --target prod -t forge/hilt:local .

image-dev:
	$(DOCKER) build -f ../docker/Dockerfile $(IMAGE_BUILD_ARGS) --target dev -t forge/hilt:local-dev .

clean:
	rm $(BIN_DIR)/$(BINARY)
