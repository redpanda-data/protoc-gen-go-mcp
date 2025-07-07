# OpenAI Compatible Example

This example demonstrates how to use OpenAI-compatible MCP handlers using the TestService from pkg/testdata. The key difference from the basic example is that this uses the OpenAI-specific handler registration to ensure compatibility with OpenAI's JSON schema restrictions.

This example uses the generated MCP handlers from `pkg/testdata/gen/go/testdata/testdatamcp` which are created by running `task generate` in the project root.

## Test / list tools

[f/mcptools](https://github.com/f/mcptools) can be useful.

Run `mcptools tools go run .`
