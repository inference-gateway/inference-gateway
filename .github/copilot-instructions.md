# Custom Instructions for Copilot

Today is May 23, 2025.

- Always use context7 to check for the latest updates, features, or best practices of a library relevant to the task at hand.
- Always prefer Table-Driven Testing: When writing tests.
- Always use Early Returns: Favor early returns to simplify logic and avoid deep nesting with if-else structures.
- Always prefer switch statements over if-else chains: Use switch statements for cleaner and more readable code when checking multiple conditions.
- Always run `task lint` before committing code to ensure it adheres to the project's linting rules.
- Always run `task test` before committing code to ensure all tests pass.
- Always run `task build` to verify compilation after making changes.
- Always search for the simplest solution first before considering more complex alternatives.
- Always prefer type safety over dynamic typing: Use strong typing and interfaces to ensure type safety and reduce runtime errors.
- When working on MCP (Model Context Protocol) related tasks, always refer to the official MCP documentation and examples for guidance and ensure you run `task jrpc-mcp-schema-download` and `task generate` to keep the MCP Golang types up to date.
- When possible code to an interface so it's easier to mock in tests.
- When writing tests, each test case should have it's own isolated mock server mock dependecies so it's easier to understand and maintain.

## Development Workflow

### Configuration Changes

When adding new configuration fields:

1. Update `openapi.yaml` with the new configuration fields in the appropriate section
2. If added new Schemas to openapi.yaml, update internal/openapi/schemas.go to include the new schemas
3. Run `task generate` to regenerate all configuration-related files
4. Run `task lint` to ensure code quality
5. Run `task build` to verify successful compilation
6. Run `task test` to ensure all tests pass

### Common Commands

- `task jrpc-mcp-schema-download` - Downloads latest MCP schema
- `task generate` - Regenerates configuration code, documentation, and environment files from openapi.yaml
- `task lint` - Runs linting checks
- `task build` - Builds the project binary
- `task test` - Runs all unit tests

## Available Tools and MCPs

- context7 - Helps by finding the latest updates, features, or best practices of a library relevant to the task at hand.

## Related Repositories

- [Inference Gateway](https://github.com/inference-gateway)
  - [Inference Gateway UI](https://github.com/inference-gateway/ui)
  - [Go SDK](https://github.com/inference-gateway/go-sdk)
  - [Rust SDK](https://github.com/inference-gateway/rust-sdk)
  - [TypeScript SDK](https://github.com/inference-gateway/typescript-sdk)
  - [Documentation](https://github.com/inference-gateway/docs)

## MCP Useful links

- [Introduction](https://modelcontextprotocol.io/introduction)
- [Specification](https://modelcontextprotocol.io/specification)
- [Examples](https://modelcontextprotocol.io/examples)
- [Schema](https://raw.githubusercontent.com/modelcontextprotocol/modelcontextprotocol/refs/heads/main/schema/draft/schema.json)

The MCP Golang types are being generated from the MCP official MCP schema using `task jrpc-mcp-schema-download` and `task generate`.

## Recent Completed Tasks

### MCP Client Timeout Configuration (May 25, 2025)

Successfully made MCP client timeouts configurable:

**What was accomplished:**

- Added 6 new MCP timeout configuration fields to `openapi.yaml`:
  - `MCP_CLIENT_TIMEOUT` (5s) - HTTP client timeout
  - `MCP_DIAL_TIMEOUT` (3s) - Dial timeout
  - `MCP_TLS_HANDSHAKE_TIMEOUT` (3s) - TLS handshake timeout
  - `MCP_RESPONSE_HEADER_TIMEOUT` (3s) - Response header timeout
  - `MCP_EXPECT_CONTINUE_TIMEOUT` (1s) - Expect continue timeout
  - `MCP_REQUEST_TIMEOUT` (5s) - Request timeout for initialize and tool calls

**Files modified:**

- `openapi.yaml` - Added timeout configuration fields
- `mcp/client.go` - Updated to use configurable timeouts instead of hardcoded values
- `cmd/gateway/main.go` - Updated NewMCPClient call to pass config
- `config/config_test.go` - Updated test cases to include new timeout fields

**Generated files updated via `task generate`:**

- `config/config.go` - Added new timeout fields
- All `.env.example` files - Added new timeout environment variables
- `Configurations.md` - Added documentation for new timeout fields
- Helm templates and values - Added new timeout configurations

**Commands run:**

```bash
task generate  # Regenerated configuration code
task test      # Verified all tests pass
task lint      # Ensured code quality
task build     # Verified successful compilation
```

The MCP client now uses fully configurable timeouts instead of hardcoded values.
