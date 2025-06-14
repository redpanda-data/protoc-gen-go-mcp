# Integration Tests

This directory contains integration tests that validate the end-to-end functionality of `protoc-gen-go-mcp` with OpenAI's API.

## What It Tests

1. **Schema Generation**: Validates that generated JSON schemas comply with OpenAI's requirements
2. **API Compatibility**: Makes actual API calls to OpenAI to ensure the schemas work
3. **Data Transformation**: Verifies that OpenAI-specific transformations work correctly:
   - Maps → Arrays of key-value pairs
   - `google.protobuf.Struct/Value` → JSON strings
   - `google.protobuf.Any` → Object (not `["object", "null"]`)
   - No `format` fields in schemas
   - All fields marked as required

## Running the Tests

### Prerequisites

1. Set your OpenAI API key:
   ```bash
   export OPENAI_API_KEY="your-api-key"
   ```

2. Generate the test code:
   ```bash
   make integrationtest-generate
   ```

### Run Tests

```bash
# Run integration tests
make integrationtest

# Or run directly with go test
go test -tags=integration ./integrationtest -v
```

### Run Only Schema Validation (No API Key Required)

```bash
go test -tags=integration ./integrationtest -run TestSchemaValidation -v
```

## Test Structure

- `proto/integrationtest/v1/test.proto`: Test service definition with various field types
- `openai_integration_test.go`: Integration tests that:
  - Create actual OpenAI function tools from generated schemas
  - Make API calls to trigger tool usage
  - Validate the responses match expected structure
  - Verify schema compliance with OpenAI requirements

## Adding New Test Cases

1. Add new message types or fields to `test.proto`
2. Run `make integrationtest-generate` to regenerate code
3. Add test cases in `openai_integration_test.go`

## Debugging

If tests fail, check:

1. **Schema Generation**: Look at the generated schemas in `gen/go/integrationtest/v1/integrationtestv1mcp/test.pb.mcp.go`
2. **API Response**: The test logs the actual tool call arguments received from OpenAI
3. **Schema Validation**: Run `TestSchemaValidation` to check schema compliance without making API calls