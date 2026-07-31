# forge — root Makefile.
#
# Pure delegation: every recipe here runs `make -C` into a service; no build
# logic lives at the root. `cd <svc> && make <target>` always works standalone,
# and adding a service means adding it to a list below, nothing more.
#
# The verb contract every service Makefile implements:
#
#   build   compile the service binary        vet    go vet the module
#   test    unit tests (fast, no Docker)      image  prod container via
#   clean   remove build outputs                     docker/Dockerfile
#
# smelt is the stack harness, not a service: it joins test/vet, and its own
# `build`/`clean`/`up`/`down` are STACK lifecycle commands — drive those from
# smelt/ directly.

# Services: independent Go modules that ship a container.
SERVICES := piri hilt sprue ingot
# Everything with Go unit tests (smelt's units join test/vet).
GO_MODULES := $(SERVICES) smelt

.DEFAULT_GOAL := help

.PHONY: build vet test images itest e2e clean help

## build: compile every service binary
build:
	@set -e; for s in $(SERVICES); do $(MAKE) -C $$s build; done

## vet: go vet every module
vet:
	@set -e; for s in $(GO_MODULES); do $(MAKE) -C $$s vet; done

## test: unit tests for every module (fast, no Docker)
test:
	@set -e; for s in $(GO_MODULES); do $(MAKE) -C $$s test; done

## images: build every service's prod container from docker/Dockerfile
images:
	@set -e; for s in $(SERVICES); do $(MAKE) -C $$s image; done

## itest: the S3 gateway system suite — boots the full Forge stack in Docker
itest:
	$(MAKE) -C smelt test-s3

## e2e: smelt's stack smoke suite (boots the full stack in Docker)
e2e:
	$(MAKE) -C smelt test-e2e

## clean: remove every service's build outputs (smelt's stack is untouched)
clean:
	@set -e; for s in $(SERVICES); do $(MAKE) -C $$s clean; done

## <svc>/<target>: run any target in any service, e.g. `make piri/test`
%/build %/vet %/test %/image %/image-dev %/clean %/gen %/itest:
	$(MAKE) -C $(@D) $(@F)

## help: list root targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
