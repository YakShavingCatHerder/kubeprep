.PHONY: build snapshot test test-integration lint install validate-pack

LDFLAGS := -X github.com/YakShavingCatHerder/kubecrypt/internal/cli.Version=dev
GOBIN := $(shell go env GOBIN)
ifeq ($(strip $(GOBIN)),)
GOBIN := $(shell go env GOPATH)/bin
endif

build:
	go build -ldflags="$(LDFLAGS)" -o bin/kubecrypt ./cmd/kubecrypt
	@mkdir -p "$(GOBIN)"
	@cp bin/kubecrypt "$(GOBIN)/kubecrypt"
	@echo "installed $(GOBIN)/kubecrypt"
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

PACK ?= curriculum

validate-pack: build
	./bin/kubecrypt pack validate "$(PACK)"
