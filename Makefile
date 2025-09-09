# Makefile for cemcp project
.PHONY: all build clean test deps lint format help \
        build-filesystem build-stochastic-clarity \
        test-filesystem test-stochastic-clarity \
        install-filesystem install-stochastic-clarity

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt

# Build output directories
BUILD_DIR=./bin
FILESYSTEM_BINARY=$(BUILD_DIR)/filesystem
STOCHASTIC_BINARY=$(BUILD_DIR)/stochastic-clarity

# CGO environment for filesystem service (Tesseract/Leptonica dependencies)
export PKG_CONFIG_PATH=/opt/homebrew/lib/pkgconfig:/opt/homebrew/share/pkgconfig
export CGO_CPPFLAGS=-I/opt/homebrew/include -I/opt/homebrew/include/leptonica
export CGO_LDFLAGS=-L/opt/homebrew/lib

# Default target
all: deps build

# Help target
help:
	@echo "Available targets:"
	@echo "  all                     - Download dependencies and build all services"
	@echo "  build                   - Build all services"
	@echo "  build-filesystem        - Build filesystem service only"
	@echo "  build-stochastic-clarity - Build stochastic-clarity service only"
	@echo "  test                    - Run tests for all services"
	@echo "  test-filesystem         - Run tests for filesystem service only"
	@echo "  test-stochastic-clarity - Run tests for stochastic-clarity service only"
	@echo "  deps                    - Download and update dependencies"
	@echo "  clean                   - Clean build artifacts"
	@echo "  lint                    - Run linter on all Go code"
	@echo "  format                  - Format all Go code"
	@echo "  install-filesystem      - Install filesystem service to GOPATH/bin"
	@echo "  install-stochastic-clarity - Install stochastic-clarity service to GOPATH/bin"

# Create build directory
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Download and update dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

# Build all services
build: build-filesystem build-stochastic-clarity

# Build filesystem service with CGO environment
build-filesystem: $(BUILD_DIR)
	@echo "Building filesystem service..."
	cd services/filesystem && \
	PKG_CONFIG_PATH=$(PKG_CONFIG_PATH) \
	CGO_CPPFLAGS="$(CGO_CPPFLAGS)" \
	CGO_LDFLAGS="$(CGO_LDFLAGS)" \
	$(GOBUILD) -o ../../$(FILESYSTEM_BINARY) .

# Build stochastic-clarity service
build-stochastic-clarity: $(BUILD_DIR)
	@echo "Building stochastic-clarity service..."
	cd services/stochastic-clarity && $(GOBUILD) -o ../../$(STOCHASTIC_BINARY) .

# Install filesystem service
install-filesystem:
	@echo "Installing filesystem service..."
	cd services/filesystem && \
	PKG_CONFIG_PATH=$(PKG_CONFIG_PATH) \
	CGO_CPPFLAGS="$(CGO_CPPFLAGS)" \
	CGO_LDFLAGS="$(CGO_LDFLAGS)" \
	$(GOCMD) install .

# Install stochastic-clarity service
install-stochastic-clarity:
	@echo "Installing stochastic-clarity service..."
	cd services/stochastic-clarity && $(GOCMD) install .

# Run tests for all services
test: test-filesystem test-stochastic-clarity

# Run tests for filesystem service
test-filesystem:
	@echo "Running filesystem service tests..."
	cd services/filesystem && \
	PKG_CONFIG_PATH=$(PKG_CONFIG_PATH) \
	CGO_CPPFLAGS="$(CGO_CPPFLAGS)" \
	CGO_LDFLAGS="$(CGO_LDFLAGS)" \
	$(GOTEST) -v ./...

# Run tests for stochastic-clarity service
test-stochastic-clarity:
	@echo "Running stochastic-clarity service tests..."
	cd services/stochastic-clarity && $(GOTEST) -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f filesystem stochastic-clarity

# Format all Go code
format:
	@echo "Formatting Go code..."
	$(GOFMT) -s -w .

# Lint all Go code (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Development helpers
dev-filesystem:
	@echo "Building and running filesystem service for development..."
	cd services/filesystem && \
	PKG_CONFIG_PATH=$(PKG_CONFIG_PATH) \
	CGO_CPPFLAGS="$(CGO_CPPFLAGS)" \
	CGO_LDFLAGS="$(CGO_LDFLAGS)" \
	$(GOBUILD) -o filesystem . && ./filesystem

dev-stochastic-clarity:
	@echo "Building and running stochastic-clarity service for development..."
	cd services/stochastic-clarity && $(GOBUILD) -o stochastic-clarity . && ./stochastic-clarity

# Check system dependencies
check-deps:
	@echo "Checking system dependencies..."
	@echo "Go version:"
	@$(GOCMD) version
	@echo ""
	@echo "Tesseract version:"
	@if command -v tesseract >/dev/null 2>&1; then \
		tesseract --version | head -1; \
	else \
		echo "Tesseract not found. Install with: brew install tesseract"; \
	fi
	@echo ""
	@echo "Leptonica (pkg-config):"
	@if pkg-config --exists lept; then \
		echo "Leptonica found: $$(pkg-config --modversion lept)"; \
	else \
		echo "Leptonica not found. Install with: brew install leptonica"; \
	fi

# Update all dependencies
update-deps:
	@echo "Updating all dependencies..."
	$(GOMOD) tidy
	cd services/filesystem && $(GOMOD) tidy
	cd services/stochastic-clarity && $(GOMOD) tidy
	$(GOMOD) download