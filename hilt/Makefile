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

# Container image via the monorepo-shared docker/Dockerfile (context = the repo root).
IMAGE_BUILD_ARGS := --build-arg SERVICE=hilt

.PHONY: build test itest vet image image-dev clean gen

build:
	go build $(GOFLAGS) -o $(BIN_DIR)/$(BINARY) $(CMD)

gen:
	go generate ./...

test:
	go test ./...

# Integration suite: boots the full Forge stack in Docker via smelt (itest/).
# GOWORK=off keeps it hermetic — the harness builds the hilt binary the same
# way; drop it to resolve unmerged sibling repos through a parent go.work.
itest:
	GOWORK=off go test -tags itest -v -timeout 30m ./itest

vet:
	go vet ./...

image:
	$(DOCKER) build -f ../docker/Dockerfile $(IMAGE_BUILD_ARGS) --target prod -t forge/hilt:local ..

image-dev:
	$(DOCKER) build -f ../docker/Dockerfile $(IMAGE_BUILD_ARGS) --target dev -t forge/hilt:local-dev ..

clean:
	rm $(BIN_DIR)/$(BINARY)
