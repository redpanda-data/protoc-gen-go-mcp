# `protoc-gen-go-mcp`

**`protoc-gen-go-mcp`** is a [Protocol Buffers](https://protobuf.dev) compiler plugin that generates [Model Context Protocol (MCP)](https://modelcontextprotocol.io) servers for your `gRPC` or `ConnectRPC` APIs.

It generates `*.pb.mcp.go` files for each protobuf service, enabling you to delegate handlers directly to gRPC servers or clients. Under the hood, MCP uses JSON Schema for tool inputs—`protoc-gen-go-mcp` auto-generates these schemas from your method input descriptors.

> ⚠️ Currently supports [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) as the MCP server runtime. Future support is planned for official Go SDKs and additional runtimes.

## ✨ Features

- 🚀 Auto-generates MCP handlers from your `.proto` services  
- 📦 Outputs JSON Schema for method inputs  
- 🔄 Wire up to gRPC or ConnectRPC servers/clients  
- 🧩 Easy integration with [`buf`](https://buf.build)  
- 🏷️ **Custom tool naming** via type-safe protobuf annotations  

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
See [here](./example/buf.gen.yaml) for a complete example.

After running `buf generate`, you will see a new folder for each package with protobuf Service definitions:

```
tree example/gen/
gen
└── go
    └── proto
        └── example
            └── v1
                ├── example.pb.go
                └── examplev1mcp
                    └── example.pb.mcp.go
```

### Wiring Up MCP with gRPC server (in-process)

Example for in-process registration:

```go
srv := exampleServer{} // your gRPC implementation

// Register all RPC methods as tools on the MCP server
examplev1mcp.RegisterExampleServiceHandler(mcpServer, &srv)
```

Each RPC method in your protobuf service becomes an MCP tool.

➡️ See the [full example](./example) for details.

### Wiring up with grpc and connectrpc client

It is also possible to directly forward MCP tool calls to gRPC clients. 

```go
examplev1mcp.ForwardToExampleServiceClient(mcpServer, myGrpcClient)
```

Same for connectrpc:

```go
examplev1mcp.ForwardToConnectExampleServiceClient(mcpServer, myConnectClient)
```

This directly connects the MCP handler to the connectrpc client, requiring zero boilerplate.

## 🏷️ Custom Tool Naming

By default, MCP tool names are auto-generated from the full protobuf method name (e.g., `example_v1_ExampleService_CreateExample`). You can override this with custom, user-friendly names using **protobuf annotations** (recommended) or comment annotations (backwards compatibility).

### Usage

#### Protobuf Annotations (Recommended)

Use the `mcp.mcp_tool_name` option inside your RPC method definitions. First, import the annotations:

```protobuf
import "mcp/v1/annotations.proto";

service UserService {
  // Get user profile information for authentication and personalization
  rpc GetUserProfile(GetUserProfileRequest) returns (GetUserProfileResponse) {
    option (mcp.v1.mcp_tool_name) = "get_user_profile";
  }
  
  // Create a new user account with validation and verification  
  rpc CreateUser(CreateUserRequest) returns (CreateUserResponse) {
    option (mcp.v1.mcp_tool_name) = "create_user";
  }
}
```

#### Comment Annotations (Backwards Compatibility)

For backwards compatibility, you can still use comment-based annotations:

```protobuf
service UserService {
  // Get user profile information for authentication and personalization
  // mcp_tool_name:get_user_profile
  rpc GetUserProfile(GetUserProfileRequest) returns (GetUserProfileResponse);
  
  // Create a new user account with validation and verification
  // mcp_tool_name:create_user
  rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);
}
```

### Tool Name Validation

Tool names are validated at compile-time with the following rules:

- **snake_case only**: Use lowercase letters, numbers, and underscores
- **Start with letter**: Cannot begin with numbers or underscores
- **No consecutive underscores**: `create__user` is invalid
- **No trailing underscores**: `create_user_` is invalid
- **Length limit**: Maximum 64 characters
- **Unique per service**: No duplicate names within the same service

**Valid examples**: `create_user`, `get_profile`, `delete_item2`, `fetch_user_data`

**Invalid examples**: `CreateUser`, `create-user`, `create__user`, `create_user_`, `_create`

### Generated Output

**Without custom naming:**
```go
UserService_GetUserProfileTool = mcp.Tool{Name: "example_v1_UserService_GetUserProfile", ...}
UserService_CreateUserTool = mcp.Tool{Name: "example_v1_UserService_CreateUser", ...}
```

**With custom naming:**
```go
UserService_GetUserProfileTool = mcp.Tool{Name: "get_user_profile", ...}
UserService_CreateUserTool = mcp.Tool{Name: "create_user", ...}
```

### Benefits

- **Type-safe**: Protobuf annotations provide compile-time validation
- **IDE support**: Better autocomplete and tooling integration
- **User-friendly**: AI assistants see intuitive tool names like `get_user_profile`
- **Backwards compatible**: Methods without annotations use auto-generated names
- **Error reporting**: Clear validation errors for invalid names
- **Discoverable**: Annotations are visible in proto files and documentation

## Compatibility

OpenAI imposes some limitations, because it does not support JSON Schema features like additionalProperties, anyOf, oneOf.
Use the protoc opt `openai_compat=true` (false by default) to make the generator emit OpenAI compatible schemas.

## ⚠️ Limitations

- No interceptor support (yet). Registering with a gRPC server bypasses interceptors.
- Tool name mangling for long RPC names: If the full RPC name exceeds 64 characters (Claude desktop limit), the head of the tool name is mangled to fit.

## 🗺️ Roadmap

- Reflection/proxy mode
- Interceptor middleware support in gRPC server mode
- Support for the official Go MCP SDK (once published)

## 🛠️ Developer Guide

### Project Structure

```
├── cmd/protoc-gen-go-mcp/     # Main plugin entry point
├── pkg/
│   ├── generator/             # Core code generation logic
│   │   ├── generator.go       # Main generator implementation
│   │   ├── *_test.go         # Comprehensive test suite
│   └── runtime/               # OpenAI compatibility runtime
├── proto/mcp/v1/             # MCP annotation definitions
├── example/                   # Working example with generated code
└── example-openai-compat/    # OpenAI-compatible example
```

### Building from Source

```bash
# Clone the repository
git clone https://github.com/redpanda-data/protoc-gen-go-mcp.git
cd protoc-gen-go-mcp

# Build the plugin
go build -o bin/protoc-gen-go-mcp ./cmd/protoc-gen-go-mcp

# Run tests
go test ./...

# Generate example code
cd example && buf generate
```

### Development Workflow

1. **Make changes** to the generator code in `pkg/generator/`
2. **Run tests** to ensure functionality: `go test ./pkg/generator/... -v`
3. **Test with examples**: `cd example && buf generate && go build`
4. **Verify OpenAI compatibility**: `cd example-openai-compat && buf generate && go build`

### Testing

The project includes comprehensive tests:

- **Unit tests** for validation and helper functions (`annotation_test.go`)
- **Integration tests** for tool naming behavior (`integration_test.go`) 
- **End-to-end tests** for the complete pipeline (`e2e_test.go`)
- **Compatibility tests** for JSON Schema generation (`compatibility_test.go`)
- **Runtime tests** for OpenAI compatibility features (`fix_test.go`)

Run specific test suites:
```bash
# All generator tests
go test ./pkg/generator/... -v

# Runtime compatibility tests  
go test ./pkg/runtime/... -v

# Test coverage
go test ./... -cover
```

### Adding New Features

When adding new features:

1. **Add tests first** - follow TDD principles
2. **Update validation** if adding new annotation rules
3. **Test both examples** - ensure standard and OpenAI modes work
4. **Update documentation** - add examples and usage instructions
5. **Consider backwards compatibility** - don't break existing code

### Extending Annotations

To add new protobuf annotations:

1. **Define in proto**: Add to `proto/mcp/v1/annotations.proto`
2. **Generate Go code**: Run `buf generate` in `proto/` directory  
3. **Update generator**: Import and use in `pkg/generator/generator.go`
4. **Add validation**: Include appropriate validation logic
5. **Write tests**: Cover all edge cases and error conditions

### Code Generation Process

The generator follows this pipeline:

1. **Parse proto files** using protogen
2. **Extract annotations** from method options and comments
3. **Validate tool names** using strict snake_case rules
4. **Generate JSON schemas** from protobuf message descriptors
5. **Apply OpenAI fixes** if compatibility mode is enabled
6. **Render Go code** using text templates
7. **Write output files** with `.pb.mcp.go` extension

### Common Issues

**Proto registration conflicts**: When testing, ensure only one version of generated annotations is imported to avoid `"proto: file already registered"` errors.

**Import path issues**: Generated code imports are relative to the module root. Ensure your `go_package` options are correct.

**Schema validation**: OpenAI mode has stricter requirements. Test both modes when making schema changes.

## 💬 Feedback

We'd love feedback, bug reports, or PRs! Join the discussion and help shape the future of Go and Protobuf MCP tooling.

### Contributing

1. **Fork the repository** and create a feature branch
2. **Write tests** for your changes  
3. **Ensure all tests pass**: `go test ./...`
4. **Update documentation** as needed
5. **Submit a pull request** with a clear description
