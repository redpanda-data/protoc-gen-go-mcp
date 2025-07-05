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
import "mcp/annotations.proto";

service UserService {
  // Get user profile information for authentication and personalization
  rpc GetUserProfile(GetUserProfileRequest) returns (GetUserProfileResponse) {
    option (mcp.mcp_tool_name) = "get_user_profile";
  }
  
  // Create a new user account with validation and verification  
  rpc CreateUser(CreateUserRequest) returns (CreateUserResponse) {
    option (mcp.mcp_tool_name) = "create_user";
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

## 💬 Feedback

We'd love feedback, bug reports, or PRs! Join the discussion and help shape the future of Go and Protobuf MCP tooling.
