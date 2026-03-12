# TODO

## Bazel

- [x] MODULE.bazel, .bazelrc, .bazelversion
- [x] BUILD.bazel for all Go packages
- [x] Unit tests pass in Bazel (`bazelisk test //...`)
- [x] Conformance + integration tests build in Bazel (tagged `manual`, need API keys via `--test_env`)
- [x] In-process golden test (`golden_test.go`) replacing sh_test + gen/go-golden
- [x] CI integration (GitHub Actions with bazelisk)
