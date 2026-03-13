# Bazel-native build commands. Proto codegen stays with buf (see Taskfile.yml).

default:
    @just --list

# Build everything
build:
    bazelisk build //...

# Run all tests (unit, golden, conformance, integration). Needs API keys in env.
test:
    bazelisk test //... //conformancetest:conformancetest_test //integrationtest:integrationtest_test \
        --test_env=GOOGLE_API_KEY --test_env=OPENAI_API_KEY --test_env=ANTHROPIC_API_KEY

# Run unit + golden tests only (no API keys needed)
test-unit:
    bazelisk test //...

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
