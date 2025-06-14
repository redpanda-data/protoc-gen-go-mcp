.PHONY: test build update-golden clean install examples

# Run all tests
test:
	go test ./...

# Build the binary
build:
	go build -o protoc-gen-go-mcp ./cmd/protoc-gen-go-mcp

# Install the binary to GOPATH/bin
install:
	go install ./cmd/protoc-gen-go-mcp

# Update golden test files
update-golden:
	./tools/update-golden.sh

# Generate examples
examples:
	cd example && buf generate
	cd example-openai-compat && buf generate

# Clean build artifacts
clean:
	rm -f protoc-gen-go-mcp

# Run tests with verbose output
test-verbose:
	go test -v ./...

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o protoc-gen-go-mcp-linux-amd64 ./cmd/protoc-gen-go-mcp
	GOOS=darwin GOARCH=amd64 go build -o protoc-gen-go-mcp-darwin-amd64 ./cmd/protoc-gen-go-mcp
	GOOS=darwin GOARCH=arm64 go build -o protoc-gen-go-mcp-darwin-arm64 ./cmd/protoc-gen-go-mcp
	GOOS=windows GOARCH=amd64 go build -o protoc-gen-go-mcp-windows-amd64.exe ./cmd/protoc-gen-go-mcp

help:
	@echo "Available targets:"
	@echo "  test           - Run all tests"
	@echo "  build          - Build the binary"
	@echo "  install        - Install the binary to GOPATH/bin"
	@echo "  update-golden  - Update golden test files"
	@echo "  examples       - Generate example code"
	@echo "  clean          - Clean build artifacts"
	@echo "  test-verbose   - Run tests with verbose output"
	@echo "  build-all      - Build for multiple platforms"
	@echo "  help           - Show this help"