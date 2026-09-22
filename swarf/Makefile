BINARY := swarf
BIN_DIR := .
CMD := ./cmd/swarf

VERSION := $(shell awk -F'"' '/"version":/ {print $$4}' version.json)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE := $(shell date -u -Iseconds)
# LDFLAGS, not GOFLAGS: make re-exports a variable that was in its own
# environment, so with GOFLAGS already set in the shell `go` parses this
# string as $GOFLAGS and dies on "non-flag".
LDFLAGS := -ldflags="-X github.com/fil-forge/forge/swarf/pkg/build.version=$(VERSION) -X github.com/fil-forge/forge/swarf/pkg/build.Commit=$(COMMIT) -X github.com/fil-forge/forge/swarf/pkg/build.Date=$(DATE) -X github.com/fil-forge/forge/swarf/pkg/build.BuiltBy=make"

.PHONY: build test vet clean gen

build:
	GOWORK=off go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY) $(CMD)

gen:
	GOWORK=off go generate ./...

test:
	GOWORK=off go test ./...

vet:
	GOWORK=off go vet ./...

clean:
	rm -f $(BIN_DIR)/$(BINARY)
