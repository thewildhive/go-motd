# Makefile for MOTD Go implementation

BINARY_NAME=motd
BIN_DIR=bin
GO=go
VERSION?=dev
GO_BUILD_FLAGS=-buildvcs=false
LDFLAGS=-ldflags="-s -w -X main.VERSION=$(VERSION)"
INSTALL_PATH=/usr/local/bin

.PHONY: all build build-optimized clean test smoke install uninstall cross-compile checksums package release svu-version help

all: build-optimized

# Build regular binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GO_BUILD_FLAGS) -ldflags="-X main.VERSION=$(VERSION)" -o $(BIN_DIR)/$(BINARY_NAME) .
	@echo "Build complete: $(BIN_DIR)/$(BINARY_NAME)"

# Build optimized binary (smaller, faster)
build-optimized:
	@echo "Building optimized $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GO_BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) .
	@echo "Optimized build complete: $(BIN_DIR)/$(BINARY_NAME)"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BIN_DIR)
	@echo "Clean complete"

# Run unit tests
test:
	$(GO) test ./...

# Build and run a help smoke test
smoke: build-optimized
	@echo "Running $(BINARY_NAME)..."
	./$(BIN_DIR)/$(BINARY_NAME) -h

# Install to system
install: build-optimized
	@echo "Installing $(BINARY_NAME) to $(INSTALL_PATH)..."
	sudo cp $(BIN_DIR)/$(BINARY_NAME) $(INSTALL_PATH)/
	sudo chmod +x $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Installation complete. Run 'motd' from anywhere."

# Uninstall from system
uninstall:
	@echo "Uninstalling $(BINARY_NAME) from $(INSTALL_PATH)..."
	sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Uninstall complete"

# Cross-compile for different platforms
cross-compile:
	@echo "Cross-compiling for multiple platforms..."
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build $(GO_BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 $(GO) build $(GO_BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 $(GO) build $(GO_BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GO) build $(GO_BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 $(GO) build $(GO_BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-windows-amd64.exe .
	@echo "Cross-compilation complete"
	@ls -lh $(BIN_DIR)/$(BINARY_NAME)-*

# Generate checksums for release assets
checksums:
	@echo "Generating checksums..."
	@cd $(BIN_DIR) && sha256sum motd-linux-amd64 motd-linux-arm64 motd-darwin-amd64 motd-darwin-arm64 motd-windows-amd64.exe > checksums.txt
	@echo "Checksums generated: $(BIN_DIR)/checksums.txt"

# Package release assets
package: clean
	@echo "Creating release packages..."
	@mkdir -p $(BIN_DIR)/release
	@VERSION="$(VERSION)"; \
	echo "Building version $$VERSION"; \
	for os in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64; do \
		os_name=$$(echo $$os | cut -d- -f1); \
		arch=$$(echo $$os | cut -d- -f2); \
		if [ "$$os_name" = "windows" ]; then \
			GOOS=$$os_name GOARCH=$$arch $(GO) build $(GO_BUILD_FLAGS) -ldflags="-s -w -X main.VERSION=$$VERSION" -o $(BIN_DIR)/release/motd-$$os.exe .; \
			cd $(BIN_DIR)/release && zip motd-$$VERSION-$$os.zip motd-$$os.exe; \
		else \
			GOOS=$$os_name GOARCH=$$arch $(GO) build $(GO_BUILD_FLAGS) -ldflags="-s -w -X main.VERSION=$$VERSION" -o $(BIN_DIR)/release/motd-$$os .; \
			cd $(BIN_DIR)/release && tar -czf motd-$$VERSION-$$os.tar.gz motd-$$os; \
		fi; \
		cd -; \
	done
	@$(MAKE) checksums BIN_DIR=$(BIN_DIR)/release
	@cd $(BIN_DIR)/release && sha256sum *.tar.gz *.zip > archive-checksums.txt
	@echo "Release packages created in $(BIN_DIR)/release/"
	@ls -la $(BIN_DIR)/release/

# Show next version using svu (requires svu: go install github.com/caarlos0/svu/v2@latest)
svu-version:
	@echo "Current version:  $$(svu current 2>/dev/null || echo no-tags)"
	@echo "Next version:     $$(svu next --force-patch-increment 2>/dev/null || echo unknown)"

# Full release process (build, package, checksums)
release: clean package
	@echo "Release complete!"

# Show help
help:
	@echo "MOTD Go Implementation - Makefile commands:"
	@echo ""
	@echo "  make                 - Build optimized binary (default)"
	@echo "  make build           - Build regular binary"
	@echo "  make build-optimized - Build optimized binary"
	@echo "  make clean           - Remove build artifacts"
	@echo "  make test            - Run Go tests"
	@echo "  make smoke           - Build and run help smoke test"
	@echo "  make install         - Install to $(INSTALL_PATH)"
	@echo "  make uninstall       - Remove from $(INSTALL_PATH)"
	@echo "  make cross-compile   - Build for multiple platforms"
	@echo "  make checksums       - Generate SHA256 checksums"
	@echo "  make package         - Create release packages"
	@echo "  make svu-version     - Show current and next versions using svu"
	@echo "  make help            - Show this help message"
