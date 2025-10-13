# Makefile for MOTD Go implementation

BINARY_NAME=motd
GO=go
LDFLAGS=-ldflags="-s -w"
INSTALL_PATH=/usr/local/bin

.PHONY: all build build-optimized clean test benchmark install uninstall cross-compile help

all: build-optimized

# Build regular binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build -o $(BINARY_NAME) main.go
	@echo "Build complete: $(BINARY_NAME)"

# Build optimized binary (smaller, faster)
build-optimized:
	@echo "Building optimized $(BINARY_NAME)..."
	$(GO) build $(LDFLAGS) -o $(BINARY_NAME) main.go
	@echo "Optimized build complete: $(BINARY_NAME)"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME) motd-* *.exe
	@echo "Clean complete"

# Run basic test
test:
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME) -h

# Run benchmark comparison
benchmark:
	@echo "Running benchmark..."
	@chmod +x benchmark.sh
	./benchmark.sh

# Install to system
install: build-optimized
	@echo "Installing $(BINARY_NAME) to $(INSTALL_PATH)..."
	sudo cp $(BINARY_NAME) $(INSTALL_PATH)/
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
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 main.go
	GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-linux-arm64 main.go
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 main.go
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 main.go
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe main.go
	@echo "Cross-compilation complete"
	@ls -lh $(BINARY_NAME)-*

# Show help
help:
	@echo "MOTD Go Implementation - Makefile commands:"
	@echo ""
	@echo "  make                 - Build optimized binary (default)"
	@echo "  make build           - Build regular binary"
	@echo "  make build-optimized - Build optimized binary"
	@echo "  make clean           - Remove build artifacts"
	@echo "  make test            - Run basic test"
	@echo "  make benchmark       - Run performance comparison"
	@echo "  make install         - Install to $(INSTALL_PATH)"
	@echo "  make uninstall       - Remove from $(INSTALL_PATH)"
	@echo "  make cross-compile   - Build for multiple platforms"
	@echo "  make help            - Show this help message"
