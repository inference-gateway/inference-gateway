# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Building and Running

```bash
# Build the gateway binary
task build

# Run the gateway locally
task run

# Build Docker container
task build:container
```

### Testing and Quality

```bash
# Run all tests
task test

# Run linting (golangci-lint)
task lint

# Lint OpenAPI spec
task openapi:lint

# Format code (prettier + go fmt)
task format

# Install pre-commit hooks (recommended)
task pre-commit:install

# Run benchmarks
task benchmark
```

### Code Generation

```bash
# Generate all code from OpenAPI spec (providers, config, types, etc.)
task generate

# Download latest MCP schema before working on MCP features
task mcp:schema:download

# Download latest OpenAPI spec
task openapi:download
```

### Testing Individual Components

```bash
# Run tests for a specific package
go test -v ./providers/...
go test -v ./api/...
go test -v ./mcp/...

# Run tests with coverage
go test -v -cover ./...

# Run a specific test
go test -v -run TestSpecificName ./path/to/package
```

## High-Level Architecture

### Core Components

**Gateway Server** (`cmd/gateway/main.go`)

- Entry point that initializes configuration, logger, telemetry, and HTTP server
- Uses Gin framework for HTTP routing
- Supports graceful shutdown with context cancellation

**API Layer** (`api/`)

- `routes.go`: Defines main request handlers (ChatCompletionsHandler, ListModelsHandler, ProxyHandler)
- `middlewares/`: Contains auth (OIDC), logging, telemetry, and MCP middleware
- Handles streaming and non-streaming responses
- Routes requests to appropriate providers based on model prefix or URL parameter

**Providers** (`providers/`)

- Each provider (OpenAI, Anthropic, Groq, Ollama, etc.) implements a common interface
- `registry.go`: Central registry for provider management
- `client.go`: HTTP client configuration for making provider requests
- `model_mapping.go`: Maps model names to provider endpoints
- Provider detection via model prefix (e.g., "openai/gpt-4") or explicit `?provider=` parameter

**MCP (Model Context Protocol)** (`mcp/`)

- `client.go`: MCP client for connecting to tool servers
- `agent.go`: Handles tool execution and response processing
- `generated_types.go`: Auto-generated types from MCP schema
- Middleware can be bypassed with `X-MCP-Bypass` header
- Supports up to 10 follow-up tool calls per request

**Configuration** (`config/`)

- Environment-based configuration using `sethvargo/go-envconfig`
- All settings documented in `Configurations.md` (auto-generated)
- Supports provider API keys, URLs, timeouts, and feature flags

### Middleware Flow

1. Request → Authentication (optional OIDC) → Logging → Telemetry
2. MCP middleware (if enabled) injects available tools
3. Provider routing and proxying
4. Tool execution handling (if tools in response)
5. Response streaming or standard JSON response

### Code Generation Workflow

The project uses extensive code generation from `openapi.yaml`:

1. Update `openapi.yaml` with new schemas/configurations
2. Run `task generate` to regenerate:
   - Provider implementations
   - Configuration structures
   - Helm charts values
   - Docker Compose `.env` examples
   - Documentation

### Testing Strategy

- **Unit Tests**: Table-driven tests for individual components
- **Mock Generation**: Uses `mockgen` for interface mocking
- **Integration Tests**: Test full request flow with mock providers
- **Benchmarks**: Performance testing in `tests/` directory

### Development Best Practices

From `.github/copilot-instructions.md`:

- Use early returns to avoid deep nesting
- Prefer switch statements over if-else chains
- Use table-driven testing
- Code to interfaces for easier mocking
- Use lowercase log messages
- Ensure type safety with strong typing
- Each test should have isolated mock dependencies

### Provider Addition

When adding a new provider:

1. Add configuration to `openapi.yaml`
2. Run `task generate`
3. Implement provider-specific logic if needed
4. Add environment variables for API keys/URLs
5. Test with both streaming and non-streaming requests

### Streaming Implementation

The gateway supports Server-Sent Events (SSE) streaming:

- Detects `stream: true` in request body
- Forwards streaming responses chunk by chunk
- Handles both data-only and data: prefixed chunks
- Properly manages connection lifecycle and error handling
