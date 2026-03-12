# TODO

## Bugs Found

- [x] **BUG: Duplicate required fields for oneof in OpenAI mode** (`generator.go:486-491`)
  When `isFieldRequired()` is true for a oneof field AND openAICompat is true, the field gets added to `required` twice.

- [x] **BUG: OpenAI `google.protobuf.Any` missing `additionalProperties: false`** (`generator.go:632-638`)
  OpenAI requires `additionalProperties: false` on ALL objects. The Any type schema omits it.

- [x] **BUG: OpenAI wrapper types use non-standard `nullable: true`** (`generator.go:650-663`)
  Wrapper types use `"nullable": true` which is not a standard JSON Schema keyword. OpenAI mode should use `type: ["number", "null"]` instead.

- [x] **BUG: FixOpenAI doesn't handle repeated message fields** (`fix.go`)
  Nested maps or well-known types inside repeated messages won't be transformed.

- [x] **BUG: FixOpenAI doesn't recurse into map message values** (`fix.go`)
  `map<string, SomeMessage>` where SomeMessage has maps/WKTs won't be fixed.

- [x] **BUG: `Base32String` uses base 36, not base 32** (`generator.go:723-726`)
  Misleading function name.

- [x] **BUG: Missing validate constraints for uint32, uint64, float, double** (`generator.go:419-446`)
  Only int32 and int64 are handled. uint32/uint64/float/double constraints are silently dropped.

## Test Coverage

- [x] Round-trip tests: proto -> JSON -> schema validate -> (OpenAI fix) -> unmarshal -> proto equality
- [x] All well-known types: Duration, FieldMask, Any, wrapper types
- [x] Enum field schema generation
- [x] Nested message schema generation
- [x] Oneof schema generation (standard + OpenAI)
- [x] Repeated fields in schema generation
- [x] Bytes field handling (standard + OpenAI)
- [x] MangleHeadIfTooLong edge cases
- [x] cleanComment edge cases
- [x] extractValidateConstraints coverage for all numeric types
- [x] FixOpenAI for repeated messages with nested maps/WKTs
- [x] Extra properties with OpenAI schemas

## Conformance Suite

- [x] Generic conformance test framework (provider-agnostic)
- [x] Google Gemini conformance tests (GOOGLE_API_KEY) - 8 tests
- [x] OpenAI conformance tests (OPENAI_API_KEY) - 7 tests (CreateItem standard skipped: OpenAI rejects anyOf)
- [x] Anthropic conformance tests (ANTHROPIC_API_KEY) - 8 tests

- [x] **BUG: FixOpenAI only checked proto names, not JSON names** (`fix.go:41`)
  If an LLM returns camelCase field names (JSON convention), FixOpenAI wouldn't find/transform them.

- [x] **BUG: Non-deterministic anyOf ordering** (`schema.go:85-90`)
  Iterating over map produced random anyOf order. Fixed to use proto declaration order.

## Architecture

- [x] Extract core schema logic into `pkg/gen` (reusable library independent of protoc plugin)
- [x] Add dynamic/reflection-based MCP server registration (`gen.RegisterService`)
- [x] Add `dynamicpb`-based NewMessage helper for zero-config proxy mode (`gen.DynamicNewMessage`)
- [x] Fix non-deterministic anyOf ordering (map iteration -> declaration order)

## CI/CD and Tooling

- [~] Bazel 9.0.1 migration (in progress)
  - [x] MODULE.bazel, .bazelrc, .bazelversion
  - [x] BUILD.bazel for all Go packages (cmd, pkg/gen, pkg/runtime, pkg/generator, examples, testdata)
  - [x] Unit tests pass in Bazel (`bazelisk test //...`)
  - [x] Conformance + integration tests build in Bazel (tagged `manual`, need API keys via `--test_env`)
  - [x] In-process golden test (`golden_test.go`) replacing sh_test + gen/go-golden
  - [ ] justfile with Bazel wrappers
  - [ ] CI integration (GitHub Actions with bazelisk)
- [x] Coverage reporting in CI
- [x] Race detector in CI
- [x] Fuzz testing (found MangleHeadIfTooLong panics on small maxLen/short hashes)
