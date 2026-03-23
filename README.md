# `protoc-gen-go-mcp`

[![Test](https://github.com/redpanda-data/protoc-gen-go-mcp/actions/workflows/test.yml/badge.svg)](https://github.com/redpanda-data/protoc-gen-go-mcp/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/redpanda-data/protoc-gen-go-mcp)](https://goreportcard.com/report/github.com/redpanda-data/protoc-gen-go-mcp)
[![codecov](https://codecov.io/gh/redpanda-data/protoc-gen-go-mcp/branch/main/graph/badge.svg)](https://codecov.io/gh/redpanda-data/protoc-gen-go-mcp)

**`protoc-gen-go-mcp`** is a [Protocol Buffers](https://protobuf.dev) compiler plugin that generates [Model Context Protocol (MCP)](https://modelcontextprotocol.io) servers for your `gRPC` or `ConnectRPC` APIs.

It generates `*.pb.mcp.go` files for each protobuf service, enabling you to delegate handlers directly to gRPC servers or clients. Under the hood, MCP uses JSON Schema for tool inputs -- `protoc-gen-go-mcp` auto-generates these schemas from your method input descriptors.

Generated code is **MCP-library-agnostic**: pick [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) (the official SDK) or [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) at server creation time through a thin adapter -- no regeneration needed.

## Features

- Auto-generates MCP handlers from your `.proto` services
- Outputs JSON Schema for method inputs
- Wire up to gRPC or ConnectRPC servers/clients
- Easy integration with [`buf`](https://buf.build)
- Runtime LLM provider selection -- standard MCP or OpenAI-compatible schemas
- MCP library agnostic -- official go-sdk and mark3labs/mcp-go both supported

## Usage

### Generate code

Add entry to your `buf.gen.yaml`:
```
...
plugins:
  - local:
      - go
      - run
      - github.com/redpanda-data/protoc-gen-go-mcp/cmd/protoc-gen-go-mcp@latest
    out: ./gen/go
    opt: paths=source_relative
```

You need to generate the standard `*.pb.go` files as well. `protoc-gen-go-mcp` by defaults uses a separate subfolder `{$servicename}mcp`, and imports the `*pb.go` files -- similar to connectrpc-go.

After running `buf generate`, you will see a new folder for each package with protobuf Service definitions:

```
tree pkg/testdata/gen/
gen
  go
    testdata
      test_service.pb.go
      testdataconnect/
        test_service.connect.go
      testdatamcp/
        test_service.pb.mcp.go
```

### Setting up the MCP server

Generated code programs against the `runtime.MCPServer` interface. You choose the backing MCP library by importing the corresponding adapter package.

#### With the official go-sdk (recommended)

```go
import (
    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime/gosdk"
)

// Create server -- raw is *mcp.Server for transport, s is runtime.MCPServer for tools.
raw, s := gosdk.NewServer("my-server", "1.0.0")

testdatamcp.RegisterTestServiceHandler(s, &srv)

// Serve over stdio
raw.Run(ctx, &mcp.StdioTransport{})
```

#### With mark3labs/mcp-go

```go
import (
    "github.com/mark3labs/mcp-go/server"
    "github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime/mark3labs"
)

// Create server -- raw is *server.MCPServer for transport, s is runtime.MCPServer for tools.
raw, s := mark3labs.NewServer("my-server", "1.0.0")

testdatamcp.RegisterTestServiceHandler(s, &srv)

// Serve over stdio
server.ServeStdio(raw)
```

### Wiring up handlers

Example for in-process registration:

```go
srv := testServer{} // your gRPC implementation

// Register all RPC methods as tools on the MCP server
testdatamcp.RegisterTestServiceHandler(s, &srv)
```

Each RPC method in your protobuf service becomes an MCP tool.

### Runtime LLM Provider Selection

You can choose LLM compatibility at runtime without regenerating code:

```go
// Option 1: Use convenience function with runtime provider selection
testdatamcp.RegisterTestServiceHandlerWithProvider(s, &srv, runtime.LLMProviderOpenAI)

// Option 2: Register specific handlers directly
testdatamcp.RegisterTestServiceHandler(s, &srv)        // Standard MCP
testdatamcp.RegisterTestServiceHandlerOpenAI(s, &srv)  // OpenAI-compatible
```

### Wiring up with grpc and connectrpc client

Forward MCP tool calls directly to gRPC clients:

```go
testdatamcp.ForwardToTestServiceClient(s, myGrpcClient)
```

Same for connectrpc:

```go
testdatamcp.ForwardToConnectTestServiceClient(s, myConnectClient)
```

This directly connects the MCP handler to the client, requiring zero boilerplate.

### Extra properties

It's possible to add extra properties to MCP tools, that are not in the proto. These are written into context.


```go
// Enable URL override with custom field name and description
option := runtime.WithExtraProperties(
    runtime.ExtraProperty{
        Name:        "base_url",
        Description: "Base URL for the API",
        Required:    true,
        ContextKey:  MyURLOverrideKey{},
    },
)

// Use with any generated function
testdatamcp.RegisterTestServiceHandler(s, &srv, option)
testdatamcp.ForwardToTestServiceClient(s, client, option)
```

### Tool name prefixing

When registering the same service multiple times (e.g. separate database instances), use `WithNamePrefix` to namespace tools:

```go
sqlv1mcp.RegisterSQLServiceHandler(s, postgresHandler, runtime.WithNamePrefix("postgres"))
sqlv1mcp.RegisterSQLServiceHandler(s, clickhouseHandler, runtime.WithNamePrefix("clickhouse"))
// Tools: postgres_SQLService_Query, clickhouse_SQLService_Query, ...
```

## Migrating from mark3labs-only (pre-v0.2)

Generated code no longer imports `mark3labs/mcp-go` directly. It programs against the `runtime.MCPServer` interface, and you pick the MCP library via an adapter package.

### 1. Server creation

```go
// Before
s := server.NewMCPServer("name", "1.0", server.WithToolCapabilities(true))

// After -- mark3labs
raw, s := mark3labs.NewServer("name", "1.0", server.WithToolCapabilities(true))
// raw is *server.MCPServer for transport (ServeStdio, NewStreamableHTTPServer, etc.)
// s is runtime.MCPServer for tool registration

// After -- official go-sdk
raw, s := gosdk.NewServer("name", "1.0")
```

### 2. Tool registration

Generated `Register*Handler` functions now take `runtime.MCPServer` instead of `*server.MCPServer`. Just pass `s` from step 1.

If you were calling `s.AddTool()` manually with mark3labs types to do name prefixing, delete that code and use `WithNamePrefix`:

```go
// Before: 60 lines of manual register.go per service
sql.RegisterTools(s, "postgres", handler) // custom wrapper around s.AddTool(mcp.Tool, ...)

// After: one line
sqlv1mcp.RegisterSQLServiceHandler(s, handler, runtime.WithNamePrefix("postgres"))
```

### 3. HandleError return type

`runtime.HandleError()` now returns `*runtime.CallToolResult` instead of `*mcp.CallToolResult`. If you call it from custom handlers, update the return type.

### 4. Transport

Transport setup is unchanged -- use the raw server from step 1:

```go
// mark3labs stdio
server.ServeStdio(raw)

// mark3labs streamable HTTP
mcpserver.NewStreamableHTTPServer(raw)

// go-sdk stdio
raw.Run(ctx, &mcp.StdioTransport{})
```

## LLM Provider Compatibility

The generator creates both standard MCP and OpenAI-compatible handlers automatically. Choose which to use at runtime.

### Standard MCP
- Full JSON Schema support (additionalProperties, anyOf, oneOf)
- Maps represented as JSON objects
- Well-known types use native JSON representations

### OpenAI Compatible
- Restricted JSON Schema (no additionalProperties, anyOf, oneOf)
- Maps converted to arrays of key-value pairs
- Well-known types (Struct, Value, ListValue) encoded as JSON strings
- All fields marked as required with nullable unions

## Development & Testing

### Commands

All commands use [just](https://github.com/casey/just):

```bash
just build                # Build everything (Bazel)
just test-unit            # Unit + golden tests (no API keys needed)
just test                 # All tests including conformance/integration (needs API keys)
just test-cover           # Tests with coverage report
just generate             # Regenerate proto code + descriptor set
just lint                 # Run golangci-lint
just fmt                  # Format code
just install              # Install binary to GOPATH/bin
just gazelle              # Sync BUILD files from go.mod

# View all available commands
just --list
```

### Development Workflow

```bash
# 1. Edit proto or generator code
# 2. Regenerate
just generate
# 3. Run tests
just test-unit
```

### Golden File Testing

The generator uses golden file testing to ensure output consistency. Tests re-run the generator in-process from compiled descriptors and compare against checked-in `*.pb.mcp.go` files.

**To add new tests:** Drop a `.proto` file in `pkg/testdata/proto/testdata/` and run `just generate`. The golden test automatically discovers all generated files and compares them.

## Proto-to-JSON Schema mapping

The plugin maps protobuf types to JSON Schema. Understanding these mappings matters because LLMs see the schema, not your proto definitions.

### Scalar types

| Proto type | JSON Schema type | Notes |
|---|---|---|
| `double`, `float` | `number` | |
| `int32`, `uint32`, `sint32`, `fixed32`, `sfixed32` | `integer` | |
| `int64`, `uint64`, `sint64`, `fixed64`, `sfixed64` | `string` | JSON cannot represent 64-bit integers precisely |
| `bool` | `boolean` | |
| `string` | `string` | |
| `bytes` | `string` | `contentEncoding: base64` |
| `enum` | `string` | `enum` array with all value names |

### Messages and maps

Nested messages become nested `object` schemas. Maps depend on the mode:

**Standard mode:** `map<K, V>` becomes a JSON object with `propertyNames` constraints (e.g., `pattern` for int keys, `enum` for bool keys).

**OpenAI mode:** `map<K, V>` becomes an array of `{key, value}` objects. Key constraints (patterns, enums) are preserved on the `key` field. At runtime, `FixOpenAI` converts these arrays back to objects before proto unmarshaling.

### Well-known types

| Proto type | Standard schema | OpenAI schema |
|---|---|---|
| `google.protobuf.Timestamp` | `string` with `format: date-time` | Same |
| `google.protobuf.Duration` | `string` with duration pattern | Same |
| `google.protobuf.Struct` | `object` with `additionalProperties: true` | `string` (JSON-encoded) |
| `google.protobuf.Value` | any (no type constraint) | `string` (JSON-encoded) |
| `google.protobuf.ListValue` | `array` | `string` (JSON-encoded) |
| `google.protobuf.FieldMask` | `string` | Same |
| `google.protobuf.Any` | `object` with `@type` + `value` | Same, with `additionalProperties: false` |
| Wrapper types (`StringValue`, etc.) | nullable scalar (e.g., `["string", "null"]`) | Same |

In OpenAI mode, `FixOpenAI` handles the reverse transformation at runtime: JSON-encoded strings are parsed back, wrapper objects `{"value": X}` are unwrapped to `X`, and map arrays are converted to objects.

### Oneof fields

**Standard mode:** Uses JSON Schema `anyOf` with one entry per oneof group. Each entry specifies one allowed alternative.

**OpenAI mode:** Oneof fields are flattened into nullable fields (e.g., `type: ["string", "null"]`) with a description noting the oneof constraint. At runtime, `FixOpenAI` strips null oneof alternatives and keeps only the first non-null field per group.

### Validation constraints

[buf.validate](https://buf.build/bufbuild/protovalidate) annotations are mapped to JSON Schema keywords:

| Validation | JSON Schema |
|---|---|
| `string.uuid` | `format: uuid` |
| `string.email` | `format: email` |
| `string.pattern` | `pattern` |
| `string.min_len/max_len` | `minLength/maxLength` |
| `int32.gte/lte` | `minimum/maximum` |
| `int32.gt/lt` | `minimum+1 / maximum-1` |
| `float.gt/lt` | `exclusiveMinimum/exclusiveMaximum` |
| `double.gte/lte` | `minimum/maximum` |

### Recursive messages

Self-referencing and mutually recursive messages (e.g., tree nodes, linked lists) are supported. The schema expands up to 3 levels deep with full field detail. Beyond that, recursive fields become `{"type": "string", "description": "JSON-encoded TreeNode. Provide a JSON object as a string."}` -- the same pattern used for `Struct`/`Value`/`ListValue` in OpenAI mode.

At runtime, `FixOpenAI` parses these string-encoded fields back into JSON objects before proto unmarshaling. The actual data can nest arbitrarily deep -- the depth limit only applies to the schema the LLM sees, not to what it can send.

### Tool name mangling

If the fully qualified RPC name (dots replaced with underscores) exceeds 64 characters, the name is truncated: the head is replaced with a 10-character SHA-256 hash prefix, preserving the tail (the most specific part, typically `ServiceName_MethodName`). The 64-char limit exists because Claude desktop enforces it.

## Limitations

- No interceptor support (yet). Registering with a gRPC server bypasses interceptors.
- Recursive message schemas lose field-level detail beyond 3 levels of nesting (see above). The LLM must encode deeper levels as JSON strings.
- Extra properties added via `WithExtraProperties` must not collide with proto field names. If they do, the extra property value will be extracted into context but will also leak into the proto message.

## Feedback

We'd love feedback, bug reports, or PRs! Join the discussion and help shape the future of Go and Protobuf MCP tooling.
