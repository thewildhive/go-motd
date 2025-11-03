# Makefile for MOTD Go implementation

BINARY_NAME=motd
BIN_DIR=bin
GO=go
LDFLAGS=-ldflags="-s -w"
INSTALL_PATH=/usr/local/bin

.PHONY: all build build-optimized clean test install uninstall cross-compile checksums package release help

all: build-optimized

# Build regular binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$(BINARY_NAME) main.go
	@echo "Build complete: $(BIN_DIR)/$(BINARY_NAME)"

# Build optimized binary (smaller, faster)
build-optimized:
	@echo "Building optimized $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GO) build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) main.go
	@echo "Optimized build complete: $(BIN_DIR)/$(BINARY_NAME)"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BIN_DIR)
	@echo "Clean complete"

# Run basic test
test: build-optimized
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
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 main.go
	GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-linux-arm64 main.go
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64 main.go
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 main.go
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-windows-amd64.exe main.go
	@echo "Cross-compilation complete"
	@ls -lh $(BIN_DIR)/$(BINARY_NAME)-*

# Generate checksums for release assets
checksums:
	@echo "Generating checksums..."
	@cd $(BIN_DIR) && sha256sum * > checksums.txt
	@echo "Checksums generated: $(BIN_DIR)/checksums.txt"

# Package release assets
package: clean
	@echo "Creating release packages..."
	@mkdir -p $(BIN_DIR)/release
	@VERSION=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.1.0"); \
	VERSION=$$(echo $$VERSION | sed 's/^v//'); \
	echo "Building version $$VERSION"; \
	for os in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64; do \
		os_name=$$(echo $$os | cut -d- -f1); \
		arch=$$(echo $$os | cut -d- -f2); \
		if [ "$$os_name" = "windows" ]; then \
			GOOS=$$os_name GOARCH=$$arch $(GO) build -ldflags="-s -w -X main.VERSION=$$VERSION" -o $(BIN_DIR)/release/motd-$$os.exe main.go; \
			cd $(BIN_DIR)/release && zip motd-$$VERSION-$$os.zip motd-$$os.exe && rm motd-$$os.exe; \
		else \
			GOOS=$$os_name GOARCH=$$arch $(GO) build -ldflags="-s -w -X main.VERSION=$$VERSION" -o $(BIN_DIR)/release/motd-$$os main.go; \
			cd $(BIN_DIR)/release && tar -czf motd-$$VERSION-$$os.tar.gz motd-$$os && rm motd-$$os; \
		fi; \
		cd -; \
	done
	@$(MAKE) checksums BIN_DIR=$(BIN_DIR)/release
	@echo "Release packages created in $(BIN_DIR)/release/"
	@ls -la $(BIN_DIR)/release/

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
	@echo "  make test            - Run basic test"
	@echo "  make install         - Install to $(INSTALL_PATH)"
	@echo "  make uninstall       - Remove from $(INSTALL_PATH)"
	@echo "  make cross-compile   - Build for multiple platforms"
	@echo "  make checksums       - Generate SHA256 checksums"
	@echo "  make package         - Create release packages"
	@echo "  make release         - Full release process"
	@echo "  make help            - Show this help message"
