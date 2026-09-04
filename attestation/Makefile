.PHONY: gen build vet test codegen-build gen-check ci

gen:
	go generate ./...

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./...

# Compile the generators under the tag they run with, so a generator that no
# longer compiles fails here rather than at the next `go generate`.
codegen-build:
	go build -tags codegen -o $(shell mktemp -d) $(shell go list ./... | grep '/gen$$')

# Regenerate and fail on any diff: catches stale generated files and generators
# that no longer compile.
gen-check: gen
	@git diff --exit-code -- '*_gen.go' '*_gen.*.go' || \
		{ echo "generated files are out of date; run 'make gen' and commit the result" >&2; exit 1; }

ci: build vet codegen-build gen-check test
