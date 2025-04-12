# `protoc-gen-go-mcp`

**`protoc-gen-go-mcp`** is a [Protocol Buffers](https://protobuf.dev) compiler plugin that generates [Model Context Protocol (MCP)](https://modelcontextprotocol.io) servers for your `gRPC` or `ConnectRPC` APIs.

It generates `*.pb.mcp.go` files for each protobuf service, enabling you to delegate handlers directly to gRPC servers or clients. Under the hood, MCP uses JSON Schema for tool inputs—`protoc-gen-go-mcp` auto-generates these schemas from your method input descriptors.

> ⚠️ Currently supports [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) as the MCP server runtime. Future support is planned for official Go SDKs and additional runtimes.

## ✨ Features

- 🚀 Auto-generates MCP handlers from your `.proto` services  
- 📦 Outputs JSON Schema for method inputs  
- 🔄 Wire up to gRPC or ConnectRPC servers/clients  
- 🧩 Easy integration with [`buf`](https://buf.build)  

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

### Wiring up with connectrpc client

It is also possible to directly forward MCP tool calls to connectrpc clients. 

```go
examplev1mcp.ForwardToConnectExampleServiceClient(mcpServer, &srv)
```

This directly connects the MCP handler to the connectrpc client, requiring zero boilerplate.

## ⚠️ Limitations

- No interceptor support (yet). Registering with a gRPC server bypasses interceptors.
- Tool name mangling for long RPC names: If the full RPC name exceeds 64 characters (Claude desktop limit), the head of the tool name is mangled to fit.

## 🗺️ Roadmap

- gRPC client forwarding (currently only server-side or ConnectRPC client)
- Reflection/proxy mode
- Interceptor middleware support in gRPC server mode
- Support for the official Go MCP SDK (once published)

## 💬 Feedback

We'd love feedback, bug reports, or PRs! Join the discussion and help shape the future of Go and Protobuf MCP tooling.
