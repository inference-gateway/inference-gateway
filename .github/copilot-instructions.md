# Custom Instructions for Copilot

- Always run `task pre-commit:install` before starting any development work to ensure code quality checks are in place.
- Always use context7 to check for the latest updates, features, or best practices of a library relevant to the task at hand.
- Always prefer Table-Driven Testing: When writing tests.
- Always use Early Returns: Favor early returns to simplify logic and avoid deep nesting with if-else structures.
- Always prefer switch statements over if-else chains: Use switch statements for cleaner and more readable code when checking multiple conditions.
- Always run `task lint` before committing code to ensure it adheres to the project's linting rules.
- When working on MCP (Model Context Protocol) related tasks, always refer to the official MCP documentation and examples for guidance and ensure you run `task mcp:schema:download` and `task generate` to keep the MCP Golang types up to date.
- Always run `task build` to verify compilation after making changes.
- Always run `task test` before committing code to ensure all tests pass.
- Always search for the simplest solution first before considering more complex alternatives.
- Always prefer type safety over dynamic typing: Use strong typing and interfaces to ensure type safety and reduce runtime errors.
- Always use lowercase log messages for consistency and readability.
- When possible code to an interface so it's easier to mock in tests.
- When writing tests, each test case should have it's own isolated mock server mock dependecies so it's easier to understand and maintain.

## Development Workflow

1. Run `task pre-commit:install` to install pre-commit hooks for automatic code quality checks.
2. Run `task mcp:schema:download` to download the latest MCP schema - when working on MCP.
3. Update `openapi.yaml` with the new configuration fields in the appropriate section.
4. Run `task generate` If added new Schemas to openapi.yaml, update internal/openapi/schemas.go to include the new schemas.
5. Run `task lint` to ensure code quality.
6. Run `task build` to verify successful compilation.
7. Run `task test` to ensure all tests pass.

## Available Tools and MCPs

- context7 - Helps by finding the latest updates, features, or best practices of a library relevant to the task at hand.

## Related Repositories

### Core Inference Gateway

- **[Main Repository](https://github.com/inference-gateway)** - The main inference gateway org.
- **[Documentation](https://github.com/inference-gateway/docs)** - Official documentation and guides
- **[UI](https://github.com/inference-gateway/ui)** - Web interface for the inference gateway

### SDKs & Client Libraries

- **[Go SDK](https://github.com/inference-gateway/go-sdk)** - Go client library
- **[Rust SDK](https://github.com/inference-gateway/rust-sdk)** - Rust client library
- **[TypeScript SDK](https://github.com/inference-gateway/typescript-sdk)** - TypeScript/JavaScript client library
- **[Python SDK](https://github.com/inference-gateway/python-sdk)** - Python client library

### A2A (Agent-to-Agent) Ecosystem

- **[Awesome A2A](https://github.com/inference-gateway/awesome-a2a)** - Curated list of A2A-compatible agents
- **[Google Calendar Agent](https://github.com/inference-gateway/google-calendar-agent)** - Agent for Google Calendar integration
- **[Browser Agent](https://github.com/inference-gateway/browser-agent)** - Agent for Browser automation
- **[Documentation Agent](https://github.com/inference-gateway/documentation-agent)** - Agent for Context7 documentation
- **[N8N Agent](https://github.com/inference-gateway/n8n-agent)** - Agent for n8n workflows automation

### Internal Tools

- **[Internal Tools](https://github.com/inference-gateway/tools)** - Collection of internal tools and utilities
