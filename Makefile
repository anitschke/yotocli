.PHONY: build test test-v generate-noto install help

# Default target
build: generate-noto
	go build -o yoto main.go

# Run all tests
test:
	go test ./...

# Generate Noto Emoji assets and metadata index
generate-noto:
	go run ./internal/tools/gennoto

# Install the binary
install: build
	sudo mv yoto /usr/local/bin/

help:
	@echo "Available targets:"
	@echo "  build         - Build the yoto binary"
	@echo "  test          - Run all tests"
	@echo "  test-v        - Run all tests (verbose)"
	@echo "  generate-noto - Download and generate embedded Noto Emoji assets and tag index"
	@echo "  install       - Install the binary to /usr/local/bin"
