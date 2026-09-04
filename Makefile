# forge — root Makefile.
#
# Pure delegation: every recipe here runs `make -C` into a module; no build
# logic lives at the root. `cd <module> && make <target>` always works
# standalone, and adding a module means adding it to a list below, nothing
# more.
#
# The verb contract every service Makefile implements:
#
#   build   compile the service binary        vet    go vet the module
#   test    unit tests (fast, no Docker)      image  prod container via
#   clean   remove build outputs                     docker/Dockerfile
#
# The shared library modules (protocol, internal, attestation, forgeclient) implement
# build/vet/test only — they ship no binary and no image. protocol and
# attestation also carry the codegen gate (`make gen-check`).
#
# smelt is the stack harness, not a service: it joins test/vet, and its own
# `build`/`clean`/`up`/`down` are STACK lifecycle commands — drive those from
# smelt/ directly.

# Services: independent Go modules that ship a container.
SERVICES := piri hilt sprue ingot delegator piri-signing-service indexing-service
# Library modules shared by the services (no binary, no image).
LIBRARIES := protocol internal attestation forgeclient
# Everything with Go unit tests (smelt's units join test/vet).
GO_MODULES := $(LIBRARIES) $(SERVICES) smelt

.DEFAULT_GOAL := help

.PHONY: build vet test gen-check check-replaces images itest e2e clean help

## build: compile every service binary
build:
	@set -e; for s in $(SERVICES); do $(MAKE) -C $$s build; done

## vet: go vet every module
vet:
	@set -e; for s in $(GO_MODULES); do $(MAKE) -C $$s vet; done

## test: unit tests for every module (fast, no Docker)
test:
	@set -e; for s in $(GO_MODULES); do $(MAKE) -C $$s test; done

## gen-check: regenerate the wire codecs and fail on any diff (protocol, attestation)
gen-check:
	@set -e; for s in protocol attestation; do $(MAKE) -C $$s codegen-build gen-check; done

## check-replaces: every in-repo require in every go.mod has a matching replace
check-replaces:
	.github/scripts/check-replaces.sh

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

## <module>/<target>: run any target in any module, e.g. `make piri/test`
%/build %/vet %/test %/image %/image-dev %/clean %/gen %/gen-check %/itest:
	$(MAKE) -C $(@D) $(@F)

## help: list root targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
