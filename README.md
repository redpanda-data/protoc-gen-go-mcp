# `protoc-gen-go-mcp`

[![Test](https://github.com/redpanda-data/protoc-gen-go-mcp/actions/workflows/test.yml/badge.svg)](https://github.com/redpanda-data/protoc-gen-go-mcp/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/redpanda-data/protoc-gen-go-mcp)](https://goreportcard.com/report/github.com/redpanda-data/protoc-gen-go-mcp)
[![codecov](https://codecov.io/gh/redpanda-data/protoc-gen-go-mcp/branch/main/graph/badge.svg)](https://codecov.io/gh/redpanda-data/protoc-gen-go-mcp)

**`protoc-gen-go-mcp`** is a [Protocol Buffers](https://protobuf.dev) compiler plugin that generates [Model Context Protocol (MCP)](https://modelcontextprotocol.io) servers for your `gRPC` or `ConnectRPC` APIs.

It generates `*.pb.mcp.go` files for each protobuf service, enabling you to delegate handlers directly to gRPC servers or clients. Under the hood, MCP uses JSON Schema for tool inputs—`protoc-gen-go-mcp` auto-generates these schemas from your method input descriptors.

> ⚠️ Currently supports [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) as the MCP server runtime. Future support is planned for official Go SDKs and additional runtimes.

## ✨ Features

- 🚀 Auto-generates MCP handlers from your `.proto` services  
- 📦 Outputs JSON Schema for method inputs  
- 🔄 Wire up to gRPC or ConnectRPC servers/clients  
- 🧩 Easy integration with [`buf`](https://buf.build)  
- 🎯 **Runtime LLM provider selection** - Choose between standard MCP and OpenAI-compatible schemas at runtime  

## 🔧 Usage

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

You need to generate the standard `*.pb.go` files as well. `protoc-gen-go-mcp` by defaults uses a separate subfolder `{$servicename}mcp`, and imports the `*pb.go` files - similar to connectrpc-go.

After running `buf generate`, you will see a new folder for each package with protobuf Service definitions:

```
tree pkg/testdata/gen/
gen
└── go
    └── testdata
        ├── test_service.pb.go
        ├── testdataconnect/
        │   └── test_service.connect.go
        └── testdatamcp/
            └── test_service.pb.mcp.go
```

### Wiring Up MCP with gRPC server (in-process)

Example for in-process registration:

```go
srv := testServer{} // your gRPC implementation

// Register all RPC methods as tools on the MCP server
testdatamcp.RegisterTestServiceHandler(mcpServer, &srv)
```

Each RPC method in your protobuf service becomes an MCP tool.

### Runtime LLM Provider Selection

**New!** You can now choose LLM compatibility at runtime without regenerating code:

```go
// Option 1: Use convenience function with runtime provider selection
provider := testdatamcp.LLMProviderOpenAI // or LLMProviderStandard
testdatamcp.RegisterTestServiceHandlerWithProvider(mcpServer, &srv, provider)

// Option 2: Register specific handlers directly
testdatamcp.RegisterTestServiceHandler(mcpServer, &srv)        // Standard MCP
testdatamcp.RegisterTestServiceHandlerOpenAI(mcpServer, &srv)  // OpenAI-compatible

// Option 3: Register both for different tool names
testdatamcp.RegisterTestServiceHandler(mcpServer, &srv)
testdatamcp.RegisterTestServiceHandlerOpenAI(mcpServer, &srv)
```

**Environment variable example:**
```go
providerStr := os.Getenv("LLM_PROVIDER")
var provider testdatamcp.LLMProvider
switch providerStr {
case "openai":
    provider = testdatamcp.LLMProviderOpenAI
case "standard":
    fallthrough
default:
    provider = testdatamcp.LLMProviderStandard
}
testdatamcp.RegisterTestServiceHandlerWithProvider(mcpServer, &srv, provider)
```

➡️ See the [full example](./examples/basic) for details.

### Wiring up with grpc and connectrpc client

It is also possible to directly forward MCP tool calls to gRPC clients. 

```go
testdatamcp.ForwardToTestServiceClient(mcpServer, myGrpcClient)
```

Same for connectrpc:

```go
testdatamcp.ForwardToConnectTestServiceClient(mcpServer, myConnectClient)
```

This directly connects the MCP handler to the connectrpc client, requiring zero boilerplate.

## LLM Provider Compatibility

The generator now creates both standard MCP and OpenAI-compatible handlers automatically. You can choose which to use at runtime:

### Standard MCP
- Full JSON Schema support (additionalProperties, anyOf, oneOf)
- Maps represented as JSON objects
- Well-known types use native JSON representations

### OpenAI Compatible  
- Restricted JSON Schema (no additionalProperties, anyOf, oneOf)
- Maps converted to arrays of key-value pairs
- Well-known types (Struct, Value, ListValue) encoded as JSON strings
- All fields marked as required with nullable unions

### Migration from openai_compat flag

The old `openai_compat=true` protoc option is **deprecated but still supported** for backward compatibility. With the new approach:

**Before (compile-time):**
```yaml
# buf.gen.yaml
plugins:
  - local: [.../protoc-gen-go-mcp]
    out: ./gen/go
    opt: paths=source_relative,openai_compat=true
```

**After (runtime):**
```go
// Choose at runtime
testdatamcp.RegisterTestServiceHandlerWithProvider(server, srv, testdatamcp.LLMProviderOpenAI)
```

## 🧪 Development & Testing

### Quick Commands

```bash
# Run all tests
task test

# Build the binary
task build

# Install to GOPATH/bin
task install

# Update golden test files
task generate-golden


# View all available commands
task --list
```

### Manual Commands

```bash
# Run tests
go test ./...

# Update golden files
./tools/update-golden.sh
# Or manually for specific packages
go test ./pkg/generator -update-golden

# Build from source
go build -o protoc-gen-go-mcp ./cmd/protoc-gen-go-mcp

# Run integration tests (requires OPENAI_API_KEY)
# Either export OPENAI_API_KEY or add to .env file
export OPENAI_API_KEY="your-api-key"
task integrationtest
```

### Development Workflow

```bash
# Format code
task fmt

# Check code formatting
task check-fmt

# Run linting
task lint

# Generate protobuf files for testdata
task generate
```

### Golden File Testing

The generator uses golden file testing to ensure output consistency. The test structure in `pkg/generator/testdata/` is organized as:

```
testdata/
├── *.proto          # Input proto files (just drop new ones here!)
├── buf.gen.yaml     # Generates into actual/
├── buf.gen.golden.yaml # Generates into golden/
├── actual/          # Current generated output (committed to track changes)
└── golden/          # Expected output (committed as test baseline)
```

**To add new tests:** Simply drop a `.proto` file in `pkg/testdata/proto/testdata/` and run the tests. The framework automatically:
1. Discovers all `.proto` files
2. Generates code using `task generate`
3. Compares with expected output
4. Creates missing golden files on first run

**To update golden files after generator changes:**
```bash
# Update all golden files
task generate-golden

# Or update specific package
go test ./pkg/generator -update-golden
```

The `actual/` directory is committed to git so you can track how generator changes affect output over time.

## ⚠️ Limitations

- No interceptor support (yet). Registering with a gRPC server bypasses interceptors.
- Tool name mangling for long RPC names: If the full RPC name exceeds 64 characters (Claude desktop limit), the head of the tool name is mangled to fit.

## 🗺️ Roadmap

- Reflection/proxy mode
- Interceptor middleware support in gRPC server mode
- Support for the official Go MCP SDK (once published)

## 💬 Feedback

We'd love feedback, bug reports, or PRs! Join the discussion and help shape the future of Go and Protobuf MCP tooling.
