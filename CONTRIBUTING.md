# Contributing to protoc-gen-go-mcp

Thank you for your interest in contributing to protoc-gen-go-mcp! We welcome contributions from the community.

## Development Setup

1. **Prerequisites**
   - Go 1.21 or later
   - [Buf](https://buf.build/docs/installation)
   - [golangci-lint](https://golangci-lint.run/welcome/install/) (for linting)

2. **Clone and setup**
   ```bash
   git clone https://github.com/redpanda-data/protoc-gen-go-mcp.git
   cd protoc-gen-go-mcp
   go mod download
   ```

3. **Run tests**
   ```bash
   make test
   ```

## Development Workflow

1. **Make changes**
   - Create a feature branch: `git checkout -b feature/my-feature`
   - Make your changes
   - Add tests for new functionality

2. **Run checks locally**
   ```bash
   # Format code
   make fmt
   
   # Run all CI checks (linting, tests, examples)
   make ci
   ```

3. **Testing**
   - Unit tests: `make test`
   - Golden file tests: Add `.proto` files to `pkg/generator/testdata/` to test new scenarios
   - Update golden files: `make update-golden` (if generator output changes)

4. **Submit PR**
   - Push your branch: `git push origin feature/my-feature`
   - Open a pull request with a clear description

## Code Style

- We use `gofumpt` for formatting (stricter than `gofmt`)
- Follow Go best practices and idioms
- Keep functions focused and small
- Add comments for exported functions and complex logic
- Use table-driven tests where appropriate

## Testing Guidelines

### Unit Tests
- Write tests for all new functionality
- Use table-driven tests for multiple scenarios
- Use meaningful test names that describe the scenario

### Golden File Tests
- Located in `pkg/generator/testdata/`
- Add new `.proto` files to test additional scenarios
- Files are automatically discovered and tested
- Update with `make update-golden` when generator changes

### Example Structure
```
pkg/generator/testdata/
├── *.proto              # Input files (add new ones here)
├── actual/              # Current generated output  
├── golden/              # Expected output (baseline)
├── buf.gen.yaml         # Generates into actual/
└── buf.gen.golden.yaml  # Generates into golden/
```

## Architecture

### Generator (`pkg/generator/`)
- `generator.go`: Main generator logic
- Converts protobuf services to MCP tools
- Generates both standard and OpenAI-compatible schemas
- Uses Go templates for code generation

### Runtime (`pkg/runtime/`)
- `fix.go`: OpenAI compatibility fixes for runtime data
- Converts maps from arrays to objects
- Handles well-known types

### Examples (`examples/`)
- `basic/`: Shows runtime provider selection
- `openai-compat/`: Shows explicit OpenAI handler usage

## Pull Request Guidelines

1. **Title**: Use clear, descriptive titles
2. **Description**: Explain what and why, not just how
3. **Tests**: Include tests for new functionality
4. **Breaking Changes**: Clearly mark and justify any breaking changes
5. **Documentation**: Update README if needed

## Issue Guidelines

- Use the provided issue templates
- Include reproduction steps for bugs
- Provide context and motivation for feature requests
- Check existing issues first to avoid duplicates

## Release Process

Releases are automated via GitHub Actions when tags are pushed:

1. Tags should follow semantic versioning: `v1.2.3`
2. GitHub Actions builds binaries for multiple platforms
3. Release notes are auto-generated from commits

## Getting Help

- Open an issue for bugs or feature requests
- Check existing documentation in README.md
- Look at examples in the `examples/` directory

## Code of Conduct

Please be respectful and constructive in all interactions. This project follows the Go community standards for inclusive and welcoming behavior.