# Makefile for LocalCloud

# Variables
BINARY_NAME=localcloud
MAIN_PATH=cmd/localcloud/main.go
BUILD_DIR=build
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
LDFLAGS=-ldflags "-X github.com/localcloud/localcloud/internal/cli.Version=$(VERSION)"

# Go related variables
GOBASE=$(shell pwd)
GOBIN=$(GOBASE)/bin
GOFILES=$(shell find . -name "*.go" -type f -not -path "./vendor/*")

# Ensure GOPATH is set
GOPATH := $(shell go env GOPATH)

.PHONY: all build clean test coverage lint run help install

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete! Binary at $(BUILD_DIR)/$(BINARY_NAME)"

## install: Install the binary to $GOPATH/bin
install: build
	@echo "Installing $(BINARY_NAME) to $(GOPATH)/bin..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/
	@echo "Creating 'lc' symlink..."
	@ln -sf $(GOPATH)/bin/$(BINARY_NAME) $(GOPATH)/bin/lc
	@echo "Installation complete! Run 'localcloud' or 'lc' to get started."

## run: Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	@$(BUILD_DIR)/$(BINARY_NAME)

## clean: Clean build directory
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@go clean
	@echo "Clean complete!"

## test: Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

## coverage: Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"

## lint: Run linter
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: brew install golangci-lint"; \
	fi

## fmt: Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Code formatted!"

## vet: Run go vet
vet:
	@echo "Running go vet..."
	@go vet ./...

## mod: Download and tidy modules
mod:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies updated!"

## dev: Run in development mode with hot reload
dev:
	@if command -v air >/dev/null; then \
		air; \
	else \
		echo "Air not installed. Install with: go install github.com/cosmtrek/air@latest"; \
		echo "Running without hot reload..."; \
		$(MAKE) run; \
	fi

## build-all: Build for all platforms
build-all:
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)

	@echo "Building for macOS (AMD64)..."
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)

	@echo "Building for macOS (ARM64)..."
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)

	@echo "Building for Linux (AMD64)..."
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)

	@echo "Building for Linux (ARM64)..."
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)

	@echo "Building for Windows (AMD64)..."
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)

	@echo "All builds complete!"

## help: Show this help message
help:
	@echo "LocalCloud Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## fix: Fix all imports, formatting and common issues
fix:
	@echo "Fixing imports..."
	@goimports -w .
	@echo "Running go mod tidy..."
	@go mod tidy
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Running go vet..."
	@go vet ./...
	@echo "All fixes applied!"

# Default target
all: mod fmt vet lint test build