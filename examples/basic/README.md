# Basic Example

This example demonstrates runtime LLM provider selection using the TestService from pkg/testdata. You can choose between standard MCP and OpenAI-compatible handlers using the `LLM_PROVIDER` environment variable.

**Usage:**
```bash
# Use standard MCP handlers (default)
go run .

# Use OpenAI-compatible handlers  
LLM_PROVIDER=openai go run .
```

This example uses the generated MCP handlers from `pkg/testdata/gen/go/testdata/testdatamcp` which are created by running `task generate` in the project root.

## Test / list tools

[f/mcptools](https://github.com/f/mcptools) can be useful.

Run `mcptools tools go run .`
