# Build and validation entry points for motd.

BINARY_NAME := motd
BIN_DIR := bin
GO := go
GOCACHE ?= $(CURDIR)/.cache/go-build
GOTMPDIR ?= $(CURDIR)/.cache/go-tmp
export GOCACHE GOTMPDIR
VERSION ?= dev
BUILDDATE ?= $(shell date -u +%d%m%y)
GO_BUILD_FLAGS := -buildvcs=false
LDFLAGS := -ldflags="-s -w -X main.VERSION=$(VERSION) -X main.BUILDDATE=$(BUILDDATE)"
INSTALL_PATH := /usr/local/bin

.PHONY: all cache-dirs build build-optimized clean test smoke check check-all check-workflows install uninstall cross-compile package release help

all: build-optimized

cache-dirs:
	@mkdir -p $(GOCACHE) $(GOTMPDIR)

build: cache-dirs
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GO_BUILD_FLAGS) -ldflags="-X main.VERSION=$(VERSION) -X main.BUILDDATE=$(BUILDDATE)" -o $(BIN_DIR)/$(BINARY_NAME) .

build-optimized: cache-dirs
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GO_BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) .

clean:
	rm -rf $(BIN_DIR)

test: cache-dirs
	$(GO) test -count=1 ./...

# Authoritative local gate. This intentionally includes every published target.
check: cache-dirs
	@echo "Checking formatting..."
	@files="$$(gofmt -l .)"; test -z "$$files" || { echo "$$files"; echo "Run: gofmt -w ."; exit 1; }
	@echo "Checking module consistency..."
	@before="$$(sha256sum go.mod go.sum)"; $(GO) mod tidy; after="$$(sha256sum go.mod go.sum)"; \
		test "$$before" = "$$after" || { echo "go mod tidy changed go.mod or go.sum"; exit 1; }
	$(GO) mod verify
	@echo "Running static analysis and tests..."
	$(GO) vet ./...
	$(GO) test -count=1 ./...
	$(GO) test -race -count=1 ./...
	@echo "Building native and release targets..."
	$(GO) build $(GO_BUILD_FLAGS) ./...
	GOOS=linux GOARCH=amd64 $(GO) build $(GO_BUILD_FLAGS) ./...
	GOOS=linux GOARCH=arm64 $(GO) build $(GO_BUILD_FLAGS) ./...
	GOOS=darwin GOARCH=amd64 $(GO) build $(GO_BUILD_FLAGS) ./...
	GOOS=darwin GOARCH=arm64 $(GO) build $(GO_BUILD_FLAGS) ./...
	GOOS=windows GOARCH=amd64 $(GO) build $(GO_BUILD_FLAGS) ./...
	@echo "Running native vulnerability analysis..."
	$(GO) build $(GO_BUILD_FLAGS) -o /tmp/motd-vulncheck .
	$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.6.0 -mode=binary /tmp/motd-vulncheck
	@$(MAKE) check-workflows
	@echo "All checks passed."

# Backward-compatible alias retained for existing contributor habits.
check-all: check

check-workflows: cache-dirs
	$(GO) run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
	bash -n .github/scripts/*.sh install.sh

smoke: build-optimized
	./$(BIN_DIR)/$(BINARY_NAME) -h
	./$(BIN_DIR)/$(BINARY_NAME) -v

install: build-optimized
	sudo cp $(BIN_DIR)/$(BINARY_NAME) $(INSTALL_PATH)/
	sudo chmod +x $(INSTALL_PATH)/$(BINARY_NAME)

uninstall:
	sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)

cross-compile: cache-dirs
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build $(GO_BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 $(GO) build $(GO_BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 $(GO) build $(GO_BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GO) build $(GO_BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 $(GO) build $(GO_BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-windows-amd64.exe .

# SIGNING_KEY_FILE must point to the Ed25519 private key used for this build.
package: clean cache-dirs
	SIGNING_KEY_FILE="$(SIGNING_KEY_FILE)" .github/scripts/package-release.sh "$(VERSION)" $(BIN_DIR)/release

release: package

help:
	@echo "MOTD build commands:"
	@echo "  make build             Build bin/motd"
	@echo "  make build-optimized   Build an optimized bin/motd"
	@echo "  make check             Run the authoritative local CI gate"
	@echo "  make check-all         Alias for make check"
	@echo "  make smoke             Build and run help/version smoke tests"
	@echo "  make cross-compile     Build all five supported raw binaries"
	@echo "  make package VERSION=X.Y.Z SIGNING_KEY_FILE=/path/to/key"
