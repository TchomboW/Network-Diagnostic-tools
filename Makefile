# NetworkTools Makefile

# Variables
GO=go
BUILD_DIR=bin
BINARY_NAME=network_tool
TARGET_FILE=network_tool.go

.PHONY: all build test clean run help

all: build

## build: Compile the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) $(TARGET_FILE)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

## run: Run the tool (requires sudo for ICMP/Raw Sockets)
run: build
	@echo "Running $(BINARY_NAME)..."
	@sudo ./$(BUILD_DIR)/$(BINARY_NAME)

## test: Run all Go tests
test:
	@echo "Running tests..."
	$(GO) test -v ./...

## clean: Remove build artifacts
clean:
	@echo "Cleaning up..."
	rm -rf $(BUILD_DIR)
	@echo "Clean complete."

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z\-]+:' Makefile | sed 's/:/: /' | awk 'BEGIN {FS = ": "; print ok=1} {print $$0}' | sed 's/^/  /'

