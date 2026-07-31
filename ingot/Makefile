# ingot — embeddable S3 gateway over the Forge network.
#
# ingot is a standalone module that must be built with the workspace disabled
# (the parent go.work declares a go version above the installed toolchain), so
# every go invocation here forces GOWORK=off.

# Force the workspace off for every recipe (see CLAUDE.md "Build, test, run").
export GOWORK := off

GO ?= go

.DEFAULT_GOAL := build

.PHONY: build test itest gen clean help

## build: compile the daemon binary (-> ./ingot)
build:
	$(GO) build -o ingot ./cmd/ingot

## test: run the unit test suite (fast, no Docker)
test:
	$(GO) test ./...

## itest: run the integration test suite (boots the Forge stack in Docker; ~6 min)
# itest/ is its own (test-only) module so ingot's library go.mod does not
# require the unpublished smelt harness — hence the cd.
itest:
	cd itest && $(GO) test -tags itest -v -timeout 30m ./...

## gen: regenerate CBOR marshalers (idempotent)
gen:
	$(GO) generate ./...

## clean: remove the go build/test cache
clean:
	$(GO) clean -cache -testcache

## help: list available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
