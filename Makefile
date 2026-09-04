.PHONY: build test test-integration lint

build:
	go build -o bin/kubecrypt ./cmd/kubecrypt

test:
	go test ./...

test-integration:
	go test -tags=integration ./tests/integration

lint:
	gofmt -w $$(find cmd internal tests -name '*.go' -type f 2>/dev/null)
	go vet ./...
