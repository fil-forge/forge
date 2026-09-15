# ingot — embeddable S3 gateway over the Forge network.
#
# ingot is a standalone module that must be built with the workspace disabled
# (the parent go.work declares a go version above the installed toolchain), so
# every go invocation here forces GOWORK=off.

# Force the workspace off for every recipe (see CLAUDE.md "Build, test, run").
export GOWORK := off

GO ?= go

.DEFAULT_GOAL := build

.PHONY: build test itest gen gen-check clean help

## build: compile the daemon binary (-> ./ingot)
build:
	$(GO) build -o ingot ./cmd/ingot

## test: run the unit test suite (fast, no Docker) and vet the itest module
test:
	$(GO) test ./...
	cd itest && GOWORK=off $(GO) vet ./...

## itest: run the integration test suite (boots the Forge stack in Docker; ~6 min)
## itest/ is its own module, so it needs no build tag to stay out of `make test`.
itest:
	cd itest && GOWORK=off $(GO) test -count=1 -v -timeout 30m ./...

## gen: regenerate CBOR marshalers (idempotent)
gen:
	$(GO) generate ./...

## gen-check: regenerate, then fail if the committed generated files are stale
gen-check: gen
	@git diff --exit-code -- bucket/cbor_gen.go || { \
		echo "bucket/cbor_gen.go is stale; run 'make gen' and commit the result"; \
		exit 1; \
	}

## clean: remove the go build/test cache
clean:
	$(GO) clean -cache -testcache

## help: list available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
