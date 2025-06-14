# Basic Example

This example demonstrates runtime LLM provider selection. You can choose between standard MCP and OpenAI-compatible handlers using the `LLM_PROVIDER` environment variable.

**Usage:**
```bash
# Use standard MCP handlers (default)
go run .

# Use OpenAI-compatible handlers  
LLM_PROVIDER=openai go run .
```

## Re-generate protos

```shell
buf generate
```

## Test / list tools

[f/mcptools](https://github.com/f/mcptools) can be usedful.

Run `mcptools tools go run .`
