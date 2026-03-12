# protoc-gen-go-mcp

Protoc plugin + runtime library for generating MCP (Model Context Protocol) server handlers from gRPC service definitions.

## Build & Test

```bash
./taskw test              # Run unit tests with race detector
./taskw test-cover        # Run tests with coverage report
./taskw build             # Build binary
./taskw lint              # Run golangci-lint
./taskw conformancetest   # Run conformance tests against LLM providers (needs GOOGLE_API_KEY)
./taskw integrationtest   # Run all integration tests (needs API keys)
```

## Architecture

```
cmd/protoc-gen-go-mcp/     Entry point for protoc plugin
pkg/gen/                    Core library (THE important package):
  schema.go                 JSON schema generation from protoreflect descriptors
  register.go               Dynamic MCP tool registration at runtime
pkg/generator/              Protoc plugin: Go template output, delegates to pkg/gen
pkg/runtime/                Runtime helpers: FixOpenAI, error handling, extra properties
pkg/testdata/               Proto files + generated code for testing
conformancetest/            E2E tests against real LLM providers (Gemini, OpenAI, Anthropic)
```

### Two modes of operation

1. **Static (codegen)**: `protoc-gen-go-mcp` generates `*.pb.mcp.go` with pre-computed schemas
2. **Dynamic (runtime)**: `gen.RegisterService()` creates MCP tools from any `protoreflect.ServiceDescriptor` at runtime - no codegen needed. This is the proxy/gateway mode.

## Key APIs

### Static registration (from generated code)
```go
testdatamcp.RegisterTestServiceHandler(mcpServer, myServiceImpl)
testdatamcp.RegisterTestServiceHandlerOpenAI(mcpServer, myServiceImpl)
```

### Dynamic registration (runtime, no codegen)
```go
gen.RegisterService(mcpServer, serviceDescriptor, handler, gen.RegisterServiceOptions{
    Provider:   runtime.LLMProviderOpenAI,
    NewMessage: func(md protoreflect.MessageDescriptor) proto.Message { ... },
})
```

### Schema generation (library use)
```go
schema := gen.MessageSchema(msgDescriptor, gen.SchemaOptions{OpenAICompat: true})
standard, openAI := gen.ToolForMethod(methodDescriptor, "description")
```

## Key Design Decisions

- Two schema modes: standard MCP and OpenAI-compatible (`gen.SchemaOptions`)
- OpenAI mode: maps -> arrays of KV pairs, all fields required, additionalProperties: false
- Well-known types (Struct, Value, ListValue) become JSON strings in OpenAI mode
- Tool names > 64 chars get hash-mangled (Claude desktop limit)
- `pkg/gen` is fully independent of protoc - works with any protoreflect descriptor
- Golden file tests catch generated code drift

## Testing

- Unit tests: `go test ./pkg/...` (with -race, always)
- Conformance tests: `go test -tags=integration ./conformancetest/` (needs GOOGLE_API_KEY)
- Golden file tests: in-process generator re-run vs checked-in `gen/go/*.pb.mcp.go`
- Edge case protos: `pkg/testdata/proto/testdata/edge_cases.proto`
- Fuzz tests: `pkg/runtime/fix_fuzz_test.go`, `pkg/gen/schema_fuzz_test.go`

## Proto Generation

Uses `buf`. Test protos in `pkg/testdata/proto/`.
After changing protos: `./taskw generate`.

## Development workflow

1. Edit proto or generator code
2. `./taskw generate` (regenerates test proto Go code + descriptor set)
3. `./taskw test` (runs unit tests, including golden comparison)
