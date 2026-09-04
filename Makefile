.PHONY: build snapshot test test-integration lint

build:
	go build -ldflags="-X github.com/YakShavingCatHerder/kubecrypt/internal/cli.Version=dev" -o bin/kubecrypt ./cmd/kubecrypt

snapshot:
	goreleaser build --snapshot --clean

test:
	go test ./...

test-integration:
	go test -tags=integration ./tests/integration

lint:
	gofmt -w $$(find cmd internal tests -name '*.go' -type f 2>/dev/null)
	go vet ./...
