indexer:
	go build -o ./indexer ./cmd

.PHONY: clean-indexer
clean-indexer:
	rm -f ./indexer

.PHONY: test
test:
	go test -race -v ./...

.PHONY: test-nocache
test-nocache:
	go clean -testcache && make test

.PHONY: ucankey
ucankey: ucangen
	./ucangen

.PHONY: mockery
mockery:
	mockery --config=.mockery.yaml

gen:
	go generate ./...

# --- monorepo verb contract (see the root Makefile): vet, image, image-dev ---
# Resolve the docker binary to an absolute path: directories named 'docker'
# on PATH (including this repo's own docker/ when '.' is in PATH) break make's
# direct exec of a bare 'docker'.
DOCKER := $(shell command -v docker)

# Container image via the monorepo-shared docker/Dockerfile (context = the repo root).
IMAGE_BUILD_ARGS := --build-arg SERVICE=indexing-service --build-arg BIN=indexer

.PHONY: vet image image-dev
vet:
	go vet ./...

image:
	$(DOCKER) build -f ../docker/Dockerfile $(IMAGE_BUILD_ARGS) --target prod -t forge/indexing-service:local ..

image-dev:
	$(DOCKER) build -f ../docker/Dockerfile $(IMAGE_BUILD_ARGS) --target dev -t forge/indexing-service:local-dev ..

.PHONY: build clean
build: indexer

clean: clean-indexer
