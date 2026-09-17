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
