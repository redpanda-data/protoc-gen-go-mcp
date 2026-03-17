# Bazel-native build commands. Proto codegen stays with buf.

default:
    @just --list

# Build everything
build:
    bazelisk build //...

# Run all tests (unit, golden, conformance, integration). Needs API keys in env.
test:
    bazelisk test //... \
        --test_env=GOOGLE_API_KEY --test_env=OPENAI_API_KEY --test_env=ANTHROPIC_API_KEY

# Run unit + golden tests only (no API keys needed)
test-unit:
    bazelisk test //...

# Run tests with coverage report
test-cover:
    go test -race -coverprofile=coverage.out -covermode=atomic ./pkg/...
    go tool cover -func=coverage.out
    go tool cover -html=coverage.out -o coverage.html

# Run conformance tests against all cloud providers (requires API keys)
conformancetest:
    go test ./conformancetest -v -timeout=300s

# Run integration tests (requires API keys)
integrationtest:
    go test ./integrationtest ./conformancetest -v -timeout=120s

# Run gazelle to sync BUILD files from go.mod
gazelle:
    bazelisk run //:gazelle

# Update BUILD files after adding/removing Go files or deps
update-build: gazelle

# Generate proto code (outside Bazel)
generate:
    cd pkg/testdata && buf generate buf.build/googleapis/googleapis
    cd pkg/testdata && buf generate --include-imports --exclude-path buf/validate
    rm -rf pkg/testdata/gen/go/buf/
    cd pkg/testdata && buf build -o gen/descriptors.binpb --exclude-path buf/validate
    go run mvdan.cc/gofumpt@latest -l -w pkg/testdata/

# Install binary to GOPATH/bin
install:
    go install ./cmd/protoc-gen-go-mcp

# Lint code
lint:
    golangci-lint run

# Format code (excludes generated protobuf files)
fmt:
    find . -name '*.go' -not -path './pkg/testdata/gen/*' | xargs go run mvdan.cc/gofumpt@latest -l -w
