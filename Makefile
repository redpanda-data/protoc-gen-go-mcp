.PHONY: test build update-golden clean install examples lint fmt check-fmt buf-lint ci

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
	cd examples/basic && buf generate
	cd examples/openai-compat && buf generate

# Generate integration test code
integrationtest-generate:
	cd integrationtest && buf generate

# Run integration tests (requires OPENAI_API_KEY)
integrationtest: integrationtest-generate
	@if [ -f .env ]; then \
		export $$(cat .env | xargs) && go test -tags=integration ./integrationtest -v; \
	else \
		go test -tags=integration ./integrationtest -v; \
	fi

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

# Lint code
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...
	gofumpt -l -w .

# Check if code is formatted (only check main source files)
check-fmt:
	@files=$$(gofumpt -l cmd/ pkg/generator/generator.go pkg/runtime/ examples/*/main.go 2>/dev/null || true); \
	if [ -n "$$files" ]; then \
		echo "Code is not formatted. Please run 'make fmt'"; \
		echo "$$files"; \
		exit 1; \
	fi

# Lint protobuf files (disabled due to package structure issues)
buf-lint:
	@echo "Buf linting disabled (package structure issues)"

# Run all CI checks locally
ci: check-fmt lint test examples
	@echo "All CI checks passed!"

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
	@echo "  lint           - Run golangci-lint"
	@echo "  fmt            - Format Go code"
	@echo "  check-fmt      - Check if code is formatted"
	@echo "  buf-lint       - Lint protobuf files"
	@echo "  ci             - Run all CI checks locally"
	@echo "  integrationtest - Run OpenAI integration tests (requires OPENAI_API_KEY)"
	@echo "  integrationtest-generate - Generate integration test code"
	@echo "  help           - Show this help"