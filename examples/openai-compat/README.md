# OpenAI Compatible Example

This example demonstrates how to use OpenAI-compatible MCP handlers. The key difference from the basic example is that this uses the OpenAI-specific handler registration to ensure compatibility with OpenAI's JSON schema restrictions.

## Re-generate protos

```shell
buf generate
```

## Test / list tools

[f/mcptools](https://github.com/f/mcptools) can be usedful.

Run `mcptools tools go run .`
