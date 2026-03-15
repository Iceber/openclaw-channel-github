# OpenClaw GitHub Channel - Build & Install

BINARY_NAME := openclaw-channel-github
ENTRY_POINT := ./cmd/openclaw-github-channel/
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME  := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS     := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.buildTime=$(BUILD_TIME)

GO          ?= go
GOFLAGS     ?= -trimpath
CGO_ENABLED ?= 0
GOARCH      ?= $(shell $(GO) env GOARCH)
GOOS        ?= $(shell $(GO) env GOOS)

PREFIX      ?= /usr/local
BINDIR      ?= $(PREFIX)/bin

# -- Targets ------------------------------------------------------------------

.PHONY: all build install uninstall clean test test-race lint vet fmt help

all: build  ## Build the binary (default)

build:  ## Build the channel binary
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o bin/$(BINARY_NAME) $(ENTRY_POINT)
	@echo "Built bin/$(BINARY_NAME)"

install: build  ## Install the binary to BINDIR (default: /usr/local/bin)
	install -d $(DESTDIR)$(BINDIR)
	install -m 0755 bin/$(BINARY_NAME) $(DESTDIR)$(BINDIR)/$(BINARY_NAME)
	@echo "Installed $(DESTDIR)$(BINDIR)/$(BINARY_NAME)"

uninstall:  ## Remove the installed binary
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY_NAME)
	@echo "Uninstalled $(DESTDIR)$(BINDIR)/$(BINARY_NAME)"

clean:  ## Remove build artifacts
	rm -rf bin/ dist/
	$(GO) clean -cache

test:  ## Run all tests
	$(GO) test ./...

test-race:  ## Run all tests with the race detector
	$(GO) test -race ./...

test-e2e:  ## Run end-to-end tests only
	$(GO) test -v ./e2e/...

lint: vet  ## Run all linters
	@echo "Lint passed (go vet)"

vet:  ## Run go vet
	$(GO) vet ./...

fmt:  ## Check code formatting
	@gofmt -l . | grep -v vendor | tee /dev/stderr | xargs -I {} test -z "{}"

help:  ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
