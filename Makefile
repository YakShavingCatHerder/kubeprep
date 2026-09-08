.PHONY: build snapshot test test-integration lint install validate-lab ci

LDFLAGS := -X github.com/YakShavingCatHerder/kubeprep/internal/cli.Version=dev
GOBIN := $(shell go env GOBIN)
ifeq ($(strip $(GOBIN)),)
GOBIN := $(shell go env GOPATH)/bin
endif

build:
	go build -ldflags="$(LDFLAGS)" -o bin/kubeprep ./cmd/kubeprep
	@mkdir -p "$(GOBIN)"
	@cp bin/kubeprep "$(GOBIN)/kubeprep"
	@echo "installed $(GOBIN)/kubeprep"
	@case ":$$PATH:" in \
	  *":$(GOBIN):"*) ;; \
	  *) \
	    echo "add $(GOBIN) to PATH:"; \
	    echo "  export PATH=\"$(GOBIN):\$$PATH\""; \
	    ;; \
	esac

install: build

snapshot:
	goreleaser build --snapshot --clean

test:
	go test ./...

test-integration:
	go test -tags=integration ./tests/integration

lint:
	gofmt -w $$(find cmd internal tests curriculum -name '*.go' -type f 2>/dev/null)
	go vet ./...

CURRICULUM ?= curriculum

validate-lab:
	go build -ldflags="$(LDFLAGS)" -o bin/kubeprep ./cmd/kubeprep
	./bin/kubeprep lab validate "$(CURRICULUM)"

# Matches the GitHub unit job (not the kind integration job).
ci:
	test -z "$$(gofmt -l .)"
	go mod tidy -diff
	go vet ./...
	$(MAKE) validate-lab
	go test -race ./...
