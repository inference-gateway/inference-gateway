# AGENTS.md - Inference Gateway

This document provides essential information for AI agents working with the Inference Gateway project.
It covers project structure, development workflow, conventions, and key commands.

## Project Overview

**Inference Gateway** is a unified API proxy server for multiple LLM providers
(OpenAI, Anthropic, Groq, Ollama, Cohere, etc.) with Model Context Protocol (MCP) integration,
OpenTelemetry metrics, and production-ready features.

**Key Technologies:**

- **Language**: Go 1.25.4
- **Framework**: Gin for HTTP routing
- **Code Generation**: OpenAPI-driven generation system
- **Containerization**: Docker, Docker Compose, Kubernetes
- **Monitoring**: OpenTelemetry, Prometheus, Grafana
- **Protocols**: Model Context Protocol (MCP), OpenAI-compatible API

## Architecture and Structure

### Directory Structure

```text
├── api/                    # HTTP API layer and middleware
│   ├── middlewares/        # Auth, logging, telemetry, MCP middleware
│   └── routes.go           # Main request handlers
├── cmd/                    # Entry points
│   ├── gateway/           # Main gateway server
│   └── generate/          # Code generation tool
├── config/                # Configuration structures
├── providers/             # LLM provider implementations
├── mcp/                   # Model Context Protocol integration
├── internal/              # Internal packages (codegen, dockergen)
├── tests/                 # Unit and integration tests
├── examples/              # Deployment examples
│   ├── docker-compose/    # Docker Compose setups
│   └── kubernetes/        # Kubernetes deployments
├── charts/                # Helm charts
├── logger/                # Structured logging
├── otel/                  # OpenTelemetry integration
└── scripts/               # Development scripts
```

### Core Components

1. **Gateway Server** (`cmd/gateway/main.go`): Entry point with graceful shutdown
2. **API Layer** (`api/`): Handles routing, middleware, and request processing
3. **Providers** (`providers/`): Unified interface for LLM providers
4. **MCP Integration** (`mcp/`): Model Context Protocol client and agent
5. **Configuration** (`config/`): Environment-based configuration
6. **Code Generation**: OpenAPI-driven generation system

### Middleware Flow

```text
Request → Auth (OIDC) → Logging → Telemetry → MCP → Provider Routing → Response
```

## Development Environment Setup

### Prerequisites

- **Go 1.25.4**
- **Docker & Docker Compose** (for containerized development)
- **Task** (task runner)

### Quick Start with Flox (Recommended)

```bash
# Install Flox (https://flox.dev/docs/install-flox/)
git clone https://github.com/inference-gateway/inference-gateway.git
cd inference-gateway
flox activate  # Activates development environment with all tools
task pre-commit:install  # Install pre-commit hooks
```

### Alternative: Dev Container (VS Code)

1. Install VS Code with Dev Containers extension
2. Clone repository
3. Open in VS Code → "Reopen in Container"

### Manual Setup

```bash
# Install Go dependencies
go mod download

# Install development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install go.uber.org/mock/mockgen@latest
npm install -g prettier @stoplight/spectral markdownlint-cli
```

## Key Commands

All development tasks are managed through `task` (Taskfile.yml). Run `task --list` to see all available tasks.

### Building and Running

```bash
task build              # Build gateway binary
task run                # Run locally
task build:container    # Build Docker container
```

### Testing and Quality

```bash
task test               # Run all tests
task lint               # Run Go linting (golangci-lint)
task openapi:lint       # Lint OpenAPI specification
task format             # Format code (prettier + go fmt)
task benchmark          # Run performance benchmarks
```

### Code Generation

```bash
task generate           # Generate code from OpenAPI spec
task mcp:schema:download # Download latest MCP schema
task openapi:download   # Download latest OpenAPI spec
```

### Git Hooks

```bash
task pre-commit:install   # Install pre-commit hooks (RECOMMENDED)
task pre-commit:uninstall # Remove pre-commit hooks
```

### Release Management

```bash
task release-dry-run     # Dry run of semantic-release and goreleaser
```

## Testing Instructions

### Test Structure

- **Unit Tests**: Table-driven tests in individual packages
- **Integration Tests**: Full request flow tests in `tests/` directory
- **Mock Generation**: Uses `mockgen` for interface mocking

### Running Tests

```bash
# Run all tests
task test

# Run specific package tests
go test -v ./providers/...
go test -v ./api/...
go test -v ./mcp/...

# Run tests with coverage
go test -v -cover ./...

# Run specific test
go test -v -run TestSpecificName ./path/to/package
```

### Test Conventions

- Use table-driven testing pattern
- Each test should have isolated mock dependencies
- Mock interfaces using `mockgen` generated mocks
- Test both streaming and non-streaming responses

## Project Conventions and Coding Standards

### Code Style

- **Formatting**: Use `gofmt` (enforced by `task format`)
- **Linting**: Follow `golangci-lint` rules (`.golangci.yml`)
- **Imports**: Group standard library, external, internal imports
- **Naming**: Use descriptive names, follow Go conventions

### Development Best Practices

1. **Early Returns**: Favor early returns to avoid deep nesting
2. **Switch Statements**: Prefer switch over if-else chains for multiple conditions
3. **Type Safety**: Use strong typing and interfaces
4. **Logging**: Use structured logging with lowercase messages
5. **Error Handling**: Handle errors explicitly, don't ignore them
6. **Interface Design**: Code to interfaces for easier testing

### Commit Conventions

- Follow [Conventional Commits](https://www.conventionalcommits.org/)
- Use semantic-release for automated versioning
- Pre-commit hooks enforce code quality

### Adding New Providers

1. Add provider configuration to `openapi.yaml` under `x-provider-configs`
2. Run `task generate` to auto-generate provider files
3. Add environment variables for API keys/URLs
4. Test with both streaming and non-streaming requests

### Code Generation Workflow

1. Update `openapi.yaml` with new schemas/configurations
2. Run `task generate` to regenerate:
   - Provider implementations
   - Configuration structures
   - Helm charts values
   - Docker Compose `.env` examples
   - Documentation (`Configurations.md`)

## Important Files and Configurations

### Configuration Files

- **`.golangci.yml`**: Go linting configuration
- **`.goreleaser.yaml`**: Release automation
- **`.releaserc.yaml`**: semantic-release configuration
- **`.spectral.yaml`**: OpenAPI linting rules
- **`.prettierrc`**: Code formatting rules

### Documentation

- **`README.md`**: Project overview and usage
- **`CONTRIBUTING.md`**: Contribution guidelines
- **`CLAUDE.md`**: Development guidance for AI assistants
- **`Configurations.md`**: Auto-generated environment variables
- **`.github/copilot-instructions.md`**: Copilot custom instructions

### Key Source Files

- **`openapi.yaml`**: API specification and code generation source
- **`Taskfile.yml`**: Task runner configuration
- **`cmd/generate/main.go`**: Code generation entry point
- **`providers/registry.go`**: Provider registry and configuration
- **`api/routes.go`**: Main API request handlers

## Development Workflow

### Standard Development Process

1. **Setup**: `task pre-commit:install` (install git hooks)
2. **Update**: Modify `openapi.yaml` for new features/providers
3. **Generate**: `task generate` (regenerate code)
4. **Implement**: Write business logic if needed
5. **Test**: `task test` (run tests)
6. **Lint**: `task lint` (check code quality)
7. **Format**: `task format` (auto-format code)
8. **Build**: `task build` (verify compilation)
9. **Commit**: Follow conventional commit format

### Pre-commit Hook

The pre-commit hook (`scripts/pre-commit-check.sh`) automatically runs:

- Code generation (`task generate`)
- Formatting (`task format`)
- Linting (`task lint` and `task openapi:lint`)
- Building (`task build`)
- Testing (`task test`)

### MCP Development

When working on MCP features:

1. Run `task mcp:schema:download` to get latest schema
2. Update MCP-related code in `mcp/` directory
3. Run `task generate` to update generated types
4. Test with MCP tool servers

## Environment Variables

All configuration is environment-based. Key categories:

### General Settings

- `ENVIRONMENT`: Runtime environment (production/development)
- `ENABLE_VISION`: Enable multimodal/vision support
- `ALLOWED_MODELS`: Comma-separated allowed models
- `DISALLOWED_MODELS`: Comma-separated blocked models

### Provider Configuration

- `{PROVIDER}_API_KEY`: API key for each provider
- `{PROVIDER}_API_URL`: Optional URL override
- Example: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`

### Telemetry

- `TELEMETRY_ENABLE`: Enable OpenTelemetry metrics
- `TELEMETRY_METRICS_PORT`: Metrics server port (default: 9464)

### MCP (Model Context Protocol)

- `MCP_ENABLE`: Enable MCP integration
- `MCP_SERVERS`: Comma-separated MCP server URLs
- `MCP_EXPOSE`: Expose tools endpoint

### Authentication

- `AUTH_ENABLE`: Enable OIDC authentication
- `AUTH_OIDC_ISSUER`: OIDC issuer URL
- `AUTH_OIDC_CLIENT_ID`: OIDC client ID

## Deployment

### Docker Compose

```bash
cd examples/docker-compose/basic/
cp .env.example .env
# Edit .env with your API keys
docker compose up -d
```

### Kubernetes

```bash
cd examples/kubernetes/basic/
task deploy  # Uses Taskfile for deployment
```

### Helm

```bash
helm install inference-gateway ./charts/inference-gateway
```

## Monitoring and Observability

### Metrics Endpoint

When `TELEMETRY_ENABLE=true`:

- Metrics available at `http://localhost:9464/metrics`
- Prometheus format
- Token usage, request latency, tool call metrics

### Grafana Dashboards

Pre-built dashboards available in `examples/docker-compose/monitoring/` and `examples/kubernetes/monitoring/`

## Troubleshooting

### Common Issues

1. **Code generation conflicts**: Run `task generate` and commit changes
2. **Linting errors**: Check `.golangci.yml` and run `task lint`
3. **Test failures**: Ensure mocks are generated with `go generate ./...`
4. **Build failures**: Verify Go version compatibility (1.25.4)

### Debug Mode

Set `ENVIRONMENT=development` for detailed logging:

- Request/response logging
- Content truncation for large messages
- Debug-level telemetry

## Related Repositories

### Core Ecosystem

- **Documentation**: <https://github.com/inference-gateway/docs>
- **UI**: <https://github.com/inference-gateway/ui>
- **Schemas**: <https://github.com/inference-gateway/schemas>

### SDKs

- **Go SDK**: <https://github.com/inference-gateway/go-sdk>
- **TypeScript SDK**: <https://github.com/inference-gateway/typescript-sdk>
- **Python SDK**: <https://github.com/inference-gateway/python-sdk>
- **Rust SDK**: <https://github.com/inference-gateway/rust-sdk>

### A2A Agents

- **Awesome A2A**: <https://github.com/inference-gateway/awesome-a2a>
- **Google Calendar Agent**: <https://github.com/inference-gateway/google-calendar-agent>
- **Browser Agent**: <https://github.com/inference-gateway/browser-agent>

## Security Notes

- Never commit API keys or secrets
- Use environment variables for configuration
- Enable authentication for production deployments
- Vision support disabled by default for security
- Regular dependency updates via Dependabot

---

_Last Updated: December 11, 2025_  
_For questions, refer to CONTRIBUTING.md or open an issue on GitHub._
