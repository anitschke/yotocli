.PHONY: build test test-v help

# Default target
build:
	go build -o yoto main.go

# Run all tests
test:
	go test ./...

# Install the binary
install: build
	sudo mv yoto /usr/local/bin/

help:
	@echo "Available targets:"
	@echo "  build     - Build the yoto binary"
	@echo "  test      - Run all tests"
	@echo "  test-v    - Run all tests (verbose)"
	@echo "  install   - Install the binary to /usr/local/bin"
