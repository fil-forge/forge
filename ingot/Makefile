# ingot — embeddable S3 gateway over the Forge network.
#
# ingot is a standalone module that must be built with the workspace disabled
# (the parent go.work declares a go version above the installed toolchain), so
# every go invocation here forces GOWORK=off.

# Force the workspace off for every recipe (see CLAUDE.md "Build, test, run").
export GOWORK := off

GO ?= go

.DEFAULT_GOAL := build

# Resolve the docker binary to an absolute path: directories named 'docker'
# on PATH (including this repo's own docker/ when '.' is in PATH) break make's
# direct exec of a bare 'docker'.
DOCKER := $(shell command -v docker)

# Container image via the monorepo-shared docker/Dockerfile (context = this dir).
IMAGE_BUILD_ARGS := --build-arg SERVICE=ingot --build-arg MAIN_PKG=./cmd/ingot

.PHONY: build test vet itest image image-dev gen clean help

## build: compile the daemon binary (-> ./ingot)
build:
	$(GO) build -o ingot ./cmd/ingot

## test: run the unit test suite (fast, no Docker)
test:
	$(GO) test ./...

## vet: go vet the module
vet:
	$(GO) vet ./...

## image: build the prod container via the shared docker/Dockerfile
image:
	$(DOCKER) build -f ../docker/Dockerfile $(IMAGE_BUILD_ARGS) --target prod -t forge/ingot:local .

## image-dev: build the dev container (debug toolset + dlv)
image-dev:
	$(DOCKER) build -f ../docker/Dockerfile $(IMAGE_BUILD_ARGS) --target dev -t forge/ingot:local-dev .

## itest: run ingot's system suite (boots the Forge stack in Docker; ~6 min)
# The suite lives in smelt/tests/s3 — it validates the whole stack through
# ingot's S3 API, so it belongs to the harness, not this library module.
itest:
	$(MAKE) -C ../smelt test-s3

## gen: regenerate CBOR marshalers (idempotent)
gen:
	$(GO) generate ./...

## clean: remove the go build/test cache
clean:
	$(GO) clean -cache -testcache

## help: list available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
