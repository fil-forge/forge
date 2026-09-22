BINARY := hilt
BIN_DIR := .
CMD := ./cmd

VERSION=$(shell awk -F'"' '/"version":/ {print $$4}' version.json)
COMMIT=$(shell git rev-parse --short HEAD)
DATE=$(shell date -u -Iseconds)
# LDFLAGS, not GOFLAGS: make re-exports a variable that was in its own
# environment, so with GOFLAGS already set in the shell `go` parses this
# string as $GOFLAGS and dies on "non-flag".
LDFLAGS=-ldflags="-X github.com/fil-forge/forge/hilt/pkg/build.version=$(VERSION) -X github.com/fil-forge/forge/hilt/pkg/build.Commit=$(COMMIT) -X github.com/fil-forge/forge/hilt/pkg/build.Date=$(DATE) -X github.com/fil-forge/forge/hilt/pkg/build.BuiltBy=make"

.PHONY: build test itest vet clean gen

build:
	go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY) $(CMD)

gen:
	go generate ./...

# itest/ is its own module, so `go test ./...` here does not reach it. Vet it
# so a broken import cannot sit undetected until someone runs the suite.
test:
	go test ./...
	cd itest && GOWORK=off go vet ./...

# Integration suite: boots the full Forge stack in Docker via smelt. It is its
# own module (itest/go.mod) rather than a build-tagged package, so it needs no
# tag to stay out of the build above. GOWORK=off keeps it hermetic — the
# harness builds the hilt binary the same way; drop it to resolve unmerged
# sibling repos through a parent go.work.
itest:
	cd itest && GOWORK=off go test -count=1 -v -timeout 30m ./...

vet:
	go vet ./...

clean:
	rm $(BIN_DIR)/$(BINARY)
