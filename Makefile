.PHONY: build snapshot test test-integration lint bundle-lesson install validate-pack

LDFLAGS := -X github.com/YakShavingCatHerder/kubecrypt/internal/cli.Version=dev
GOBIN := $(shell go env GOBIN)
ifeq ($(strip $(GOBIN)),)
GOBIN := $(shell go env GOPATH)/bin
endif

# Accept: make bundle-lesson 01-foundations/01-pod-creation
ifeq ($(firstword $(MAKECMDGOALS)),bundle-lesson)
POS_LESSON := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
ifneq ($(POS_LESSON),)
ifneq ($(words $(POS_LESSON)),1)
$(error usage: make bundle-lesson 01-foundations/01-pod-creation)
endif
LESSON ?= $(POS_LESSON)
$(POS_LESSON):
	@:
endif
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
	gofmt -w $$(find cmd internal tests -name '*.go' -type f 2>/dev/null)
	go vet ./...

PACK ?= curriculum

validate-pack: build
	./bin/kubecrypt pack validate "$(PACK)"

bundle-lesson:
	@sh scripts/bundle-lesson.sh "$(LESSON)"
