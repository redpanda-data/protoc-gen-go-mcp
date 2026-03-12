# Bazel-native build commands. Proto codegen stays with buf (see Taskfile.yml).

default:
    @just --list

# Build everything
build:
    bazelisk build //...

# Run all Bazel tests
test:
    bazelisk test //...

# Run gazelle to sync BUILD files from go.mod
gazelle:
    bazelisk run //:gazelle

# Update BUILD files after adding/removing Go files or deps
update-build: gazelle

# Run golden diff test only
test-golden:
    bazelisk test //pkg/generator:golden_diff_test

# Generate proto code (outside Bazel)
generate:
    cd pkg/testdata && buf generate buf.build/googleapis/googleapis
    cd pkg/testdata && buf generate --include-imports --exclude-path buf/validate
    rm -rf pkg/testdata/gen/go/buf/
    go run mvdan.cc/gofumpt@latest -l -w pkg/testdata/

# Update golden files (outside Bazel)
generate-golden:
    go test ./pkg/generator -update-golden -v
    find . -name '*.go' -not -path './pkg/testdata/gen/*' | xargs go run mvdan.cc/gofumpt@latest -l -w
    go run mvdan.cc/gofumpt@latest -l -w ./pkg/testdata/gen/go-golden/
