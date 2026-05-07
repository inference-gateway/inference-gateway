# AGENTS.md - Inference Gateway

> **Last Updated**: May 8, 2026
> **Repository**: github.com/inference-gateway/inference-gateway
> **Current Version**: v0.24.1

This document provides comprehensive guidance for AI agents working with the Inference Gateway project.
It covers project architecture, development workflow, testing patterns, code generation, and conventions.

## Table of Contents

- [Project Overview](#project-overview)
- [Architecture and Structure](#architecture-and-structure)
- [Development Environment Setup](#development-environment-setup)
- [Key Commands](#key-commands)
- [Testing Instructions](#testing-instructions)
- [Provider System](#provider-system)
- [Code Generation Workflow](#code-generation-workflow)
- [Project Conventions](#project-conventions)
- [Important Files](#important-files)
- [Deployment](#deployment)
- [Troubleshooting](#troubleshooting)

## Project Overview

**Inference Gateway** is a unified API proxy server for multiple LLM providers with Model Context Protocol (MCP) integration,
OpenTelemetry metrics, and enterprise-ready features. It provides an OpenAI-compatible API surface that routes requests to
various LLM providers.

### Key Technologies

- **Language**: Go 1.26.2
- **HTTP Framework**: Gin (`github.com/gin-gonic/gin`)
- **Code Generation**: OpenAPI-driven custom generator + oapi-codegen
- **Configuration**: Environment-based (`sethvargo/go-envconfig`)
- **Logging**: Structured logging via zap (`go.uber.org/zap`)
- **Telemetry**: OpenTelemetry with Prometheus exporter
- **MCP**: Model Context Protocol (`metoro-io/mcp-golang`)
- **Authentication**: OIDC with `coreos/go-oidc/v3`
- **Mocking**: `go.uber.org/mock/mockgen`
- **Testing**: `stretchr/testify` + gomock
- **Task Runner**: Task (Taskfile.yml)
- **Release**: semantic-release + goreleaser
- **Containerization**: Docker, Docker Compose, Kubernetes (Helm)
- **Monitoring**: Prometheus, Grafana

## Architecture and Structure

### Directory Layout

```text
api/                        # HTTP API layer
├── middlewares/            # Auth, Logger, MCP, Telemetry middleware
└── routes.go               # Route handlers (chat, models, proxy, tools, health)
cmd/                        # Entry points
├── gateway/main.go         # Main gateway server
└── generate/main.go        # Code generation tool CLI
config/                     # Configuration (generated from OpenAPI)
├── config.go               # Config struct, Load(), provider defaults
├── config_test.go
└── meta.go                 # APPLICATION_NAME, VERSION constants
providers/                  # LLM provider implementations
├── client/client.go        # HTTP client config & interface
├── constants/              # Provider IDs, URLs, auth types (GENERATED)
├── core/                   # IProvider interface & ProviderImpl base
├── registry/               # Provider registry & config (GENERATED)
├── routing/                # Model string → provider prefix mapping
├── transformers/           # Provider-specific response transformers
└── types/                  # Shared OpenAI-compatible types (GENERATED)
mcp/                        # Model Context Protocol
├── agent.go                # MCP agent - tool execution & iteration loop
├── client.go               # MCP HTTP client with SSE transport
├── generated_types.go      # MCP schema types (GENERATED)
├── mcp-schema.json/yaml    # MCP schema definitions
internal/                   # Internal generators
├── codegen/codegen.go      # Custom code generator
├── dockergen/dockergen.go  # Docker env example generator
├── kubegen/kubegen.go      # K8s manifest generator
├── mdgen/mdgen.go          # Markdown doc generator
├── openapi/openapi.go      # OpenAPI spec parser
└── proxy/proxy.go          # Dev-mode proxy modifiers
logger/                     # Logger interface + zap implementation
otel/                       # OpenTelemetry metrics implementation
tests/                      # All tests + mocks
├── api_routes_test.go      # Route handler tests
├── providers_test.go       # Provider tests
├── mcp_agent_test.go       # MCP agent tests
├── mcp_enhanced_test.go    # Enhanced MCP tests
├── multimodal_test.go      # Vision/multimodal tests
├── logger_test.go
├── middlewares/mcp_test.go  # MCP middleware tests
└── mocks/                  # Generated mocks
examples/                   # Deployment examples
├── docker-compose/         # basic, hybrid, mcp, auth, monitoring, tools
└── kubernetes/             # basic, hybrid, mcp, agent, auth, monitoring, tls
charts/inference-gateway/   # Helm chart
scripts/                    # Pre-commit hook
hack/                       # Dev cluster management
```

### Core Components

#### 1. Gateway Server (`cmd/gateway/main.go`)

- Initializes Config (env-based), Logger (zap), Telemetry (OTel Prometheus)
- Sets up provider registry and HTTP client
- Configures middleware pipeline: Logger → Telemetry → OIDC Auth → MCP
- Registers routes on Gin engine
- Supports TLS and graceful shutdown via OS signals

#### 2. API Layer (`api/routes.go`)

- `RouterImpl` implements the `Router` interface
- Routes: `GET /health`, `GET /v1/models`, `POST /v1/chat/completions`, `GET /v1/mcp/tools`, `/proxy/:provider/*path`
- Handles both streaming (SSE) and non-streaming (JSON) responses
- Provider is determined by model prefix (e.g., `openai/gpt-4`) or `?provider=` query param
- Uses `httputil.ReverseProxy` for non-chat proxy requests
- Processes vision content when `ENABLE_VISION=true`

#### 3. Provider System (`providers/`)

- `IProvider` interface defines: `ListModels`, `ChatCompletions`, `StreamChatCompletions`, `SupportsVision`
- `ProviderImpl` base struct handles provider-agnostic logic (HTTP requests, streaming, auth)
- Provider-specific list-models transformers convert provider formats to OpenAI-compatible
- Provider detection: explicit prefix (`openai/gpt-4`) or query parameter (`?provider=openai`)
- Auth types: `bearer`, `xheader` (x-api-key), `query` (key param), `none`

#### 4. MCP Integration (`mcp/`)

- `MCPClientInterface`: Connects to MCP servers via SSE, manages tool discovery
- `Agent`: Executes tool calls in an iterative loop (up to 10 iterations)
- Middleware injects MCP tools into the request before sending to the LLM
- When tool calls are detected in the response, the agent executes them and sends results back
- Supports streaming and non-streaming modes
- Can be bypassed with `X-MCP-Bypass: true` header

### Middleware Pipeline

```text
Request → Logger → [Telemetry] → [OIDC Auth] → [MCP Agent] → Route Handler → Response
```

- Logger: Logs every request with method, host, path
- Telemetry (optional): Records token usage, request duration, tool call metrics
- OIDC Auth (optional): Validates JWT bearer tokens against OIDC provider
- MCP (optional): Injects MCP tools into requests, handles tool execution loop
- All middlewares use the bypass pattern (no-op implementations when disabled)

### API Routes

| Method | Path | Handler | Description |
| ------ | ---- | ------- | ----------- |
| GET | `/health` | `HealthcheckHandler` | Health check endpoint |
| GET | `/v1/models` | `ListModelsHandler` | List models (optional `?provider=` filter) |
| GET | `/v1/mcp/tools` | `ListToolsHandler` | List MCP tools (when `MCP_EXPOSE=true`) |
| POST | `/v1/chat/completions` | `ChatCompletionsHandler` | Chat completions (streaming + non-streaming) |
| Any | `/proxy/:provider/*path` | `ProxyHandler` | Direct provider proxy passthrough |

### Supported Providers

| Provider | ID | Auth Type | Vision |
| -------- | -- | --------- | ------ |
| OpenAI | `openai` | bearer | Yes |
| Anthropic | `anthropic` | xheader | Yes |
| Groq | `groq` | bearer | Yes |
| Ollama | `ollama` | none | Yes |
| Ollama Cloud | `ollama_cloud` | bearer | Yes |
| Cloudflare | `cloudflare` | bearer | No |
| Cohere | `cohere` | bearer | Yes |
| DeepSeek | `deepseek` | bearer | No |
| Google | `google` | bearer | Yes |
| Mistral | `mistral` | bearer | Yes |
| Moonshot | `moonshot` | bearer | No |

## Development Environment Setup

### Prerequisites

- **Go 1.26.2** (pinned in `go.mod`)
- **Task** (task runner - `go install github.com/go-task/task/v3/cmd/task@latest`)
- **Docker & Docker Compose** (for containerized development)
- **Node.js** (for prettier, spectral, markdownlint-cli)

### Quick Start with Flox (Recommended)

```bash
# Install Flox (https://flox.dev/docs/install-flox/)
git clone https://github.com/inference-gateway/inference-gateway.git
cd inference-gateway
flox activate           # Activates development environment with all tools
task pre-commit:install # Install pre-commit hooks (RECOMMENDED)
```

The Flox environment provides: Go 1.26.2, Task, Docker, Docker Compose, golangci-lint, mockgen, Spectral, kubectl, Helm, and more.

### Alternative: Dev Container (VS Code)

1. Install VS Code with the Dev Containers extension
2. Open the repository in VS Code
3. Run "Reopen in Container" from the command palette

### Manual Development Tools Setup

```bash
# Install Go dependencies
go mod download

# Install Task runner
go install github.com/go-task/task/v3/cmd/task@latest

# Install Go tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install go.uber.org/mock/mockgen@latest

# Install Node tools
npm install -g prettier @stoplight/spectral-cli markdownlint-cli

# Install pre-commit hooks
task pre-commit:install
```

### Running the Gateway

```bash
# Minimal setup (uses default production config)
export OPENAI_API_KEY="your-api-key-here"
task run

# Development mode with full logging
export ENVIRONMENT=development
export OPENAI_API_KEY="your-api-key-here"
task run
```

### Configuration

All configuration is through environment variables. Key ones:

```bash
# General
ENVIRONMENT=development                     # Enables debug logging
ENABLE_VISION=false                         # Enable multimodal support
ALLOWED_MODELS=""                           # Comma-separated allow list

# Providers - Set API keys
OPENAI_API_KEY="sk-..."
ANTHROPIC_API_KEY="sk-ant-..."
GROQ_API_KEY="gsk_..."
COHERE_API_KEY="..."
# - local Ollama needs no auth

# Telemetry
TELEMETRY_ENABLE=true                       # Enable OpenTelemetry metrics
TELEMETRY_METRICS_PORT=9464                 # Prometheus metrics port

# MCP
MCP_ENABLE=true                             # Enable MCP middleware
MCP_SERVERS="http://server1:3001/mcp,..."   # Comma-separated MCP server URLs
MCP_EXPOSE=true                             # Expose /v1/mcp/tools endpoint
MCP_REQUEST_TIMEOUT=5s                      # Tool call timeout

# Server
SERVER_PORT=8080                            # Gateway HTTP port
SERVER_READ_TIMEOUT=30s                     # Request read timeout
SERVER_WRITE_TIMEOUT=30s                    # Response write timeout

# Auth
AUTH_ENABLE=false                           # Enable OIDC authentication
AUTH_OIDC_ISSUER="http://keycloak:8080/..." # OIDC issuer URL
```

## Key Commands

All development tasks are managed through `task` (Taskfile.yml). Run `task --list` to see all available tasks.

### Building and Running

```bash
task build              # Build gateway binary → bin/inference-gateway
task run                # Run locally with go run
task build:container    # Build Docker image
task tidy               # Run go mod tidy on all modules
```

### Testing and Quality

```bash
task test               # Run all tests (go test -v ./...)
task lint               # Run Go static analysis (golangci-lint + markdownlint)
task openapi:lint       # Lint OpenAPI spec with Spectral
task format             # Format code (prettier --write . + go fmt ./...)
task benchmark          # Run benchmarks in tests/ package
```

### Code Generation

```bash
task generate                     # Full code generation from OpenAPI spec
task mcp:schema:download          # Download latest MCP schema
task openapi:download             # Download latest OpenAPI spec
task install:generator            # Install the generator binary
```

### Git Hooks

```bash
task pre-commit:install           # Install pre-commit hooks
task pre-commit:uninstall         # Remove pre-commit hooks
```

### Release

```bash
task release-dry-run              # Dry-run of semantic-release + goreleaser
task package                      # Build Docker container for packaging
```

## Testing Instructions

### Test Structure

- **Unit Tests**: Table-driven tests in individual packages and `tests/` directory
- **Integration Tests**: Full request flow tests with mock providers
- **Benchmarks**: Performance testing in `tests/` package
- **Mock Generation**: Uses `go.uber.org/mock/mockgen` for interface mocking
- **Mock Locations**: All mocks live in `tests/mocks/` directory

### Running Tests

```bash
# Run all tests
task test                         # go test -v ./...

# Run specific package tests
go test -v ./providers/...
go test -v ./api/...
go test -v ./mcp/...
go test -v ./config/...
go test -v ./providers/routing/...

# Run tests with coverage
go test -v -cover ./...

# Run specific test function
go test -v -run TestProviderRegistry ./tests/...

# Run benchmarks
task benchmark                    # go test -bench=. -benchmem -benchtime=100x -count=20 ./tests/...

# Regenerate mocks
go generate ./...
```

### Test Patterns

#### Table-Driven Tests

This is the standard testing pattern used throughout the project:

```go
func TestFeature(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {name: "case 1", input: "a", expected: "b"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := FunctionUnderTest(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

#### Mock Setup Pattern

```go
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockClient := providersmocks.NewMockClient(ctrl)
mockRegistry := providersmocks.NewMockProviderRegistry(ctrl)
mockLogger := mocks.NewMockLogger(ctrl)
```

**Important**: All test files using gomock must set `gin.SetMode(gin.TestMode)` in an `init()` function.

### Mock Generation

Mocks are defined via `//go:generate` directives in source files:

```go
//go:generate mockgen -source=interfaces.go -destination=../../../tests/mocks/providers/management.go -package=providersmocks
type IProvider interface { ... }
```

When adding a new interface that needs mocking:

1. Add a `//go:generate` directive above the interface
2. Run `go generate ./...` to regenerate mocks
3. The mock destination should be in `tests/mocks/` following the existing structure

## Provider System

### Adding a New Provider

The project uses code generation from `openapi.yaml` to auto-create provider files.

#### Quick Steps

1. Add provider config to `openapi.yaml` under `Provider.x-provider-configs`
2. Run `task generate` to auto-generate:
   - `providers/constants/constants.go` (IDs, URLs, auth types)
   - `providers/transformers/{provider}.go` (list-models transformer)
   - `providers/registry/registry.go` (registry entries)
   - `config/config.go` (env var support)
   - `Configurations.md`, Helm values, Docker env examples
3. Add environment variables (`{PROVIDER}_API_KEY`, `{PROVIDER}_API_URL`)
4. Test with both streaming and non-streaming requests

#### Provider Config in openapi.yaml

```yaml
Provider:
  type: string
  enum:
    - existing_providers...
    - newai       # Add your provider here
  x-provider-configs:
    newai:
      id: 'newai'
      url: 'https://api.newai.com/v1'
      auth_type: 'bearer'    # or xheader, query, none
      endpoints:
        models:
          name: 'list_models'
          method: 'GET'
          endpoint: '/models'
        chat:
          name: 'chat_completions'
          method: 'POST'
          endpoint: '/chat/completions'
```

**Important**: Provider names must be valid Go identifiers (lowercase, one word recommended).

#### Protected Files (`.openapi-ignore`)

Some providers have custom transformers that are NOT overwritten during code generation:

```text
# Listed in .openapi-ignore - these are manually maintained
providers/anthropic.go
providers/cohere.go
providers/cloudflare.go
providers/ollama.go
providers/openai.go
providers/deepseek.go
providers/groq.go
```

If your provider needs custom response handling, add it to `.openapi-ignore` after generation.

### Provider Runtime Flow

1. Request comes with model `openai/gpt-4` or `?provider=openai`
2. `routing.DetermineProviderAndModelName()` extracts provider prefix
3. `registry.BuildProvider(providerID, httpClient)` creates a `ProviderImpl`
4. `ProviderImpl` routes to `/proxy/{provider}/chat/completions` internally
5. The proxy handler in `api/routes.go` sets auth headers and forwards
6. For streaming: `ProviderImpl.StreamChatCompletions()` returns a channel
7. For non-streaming: `ProviderImpl.ChatCompletions()` returns the response

### Authentication Types

| Type | Method | Example Provider |
| ---- | ------ | ---------------- |
| `bearer` | `Authorization: Bearer {token}` | OpenAI, Groq, Cohere |
| `xheader` | `x-api-key: {token}` | Anthropic |
| `query` | `?key={token}` | Legacy APIs |
| `none` | No auth | Local Ollama |

## Code Generation Workflow

Code generation is a core part of this project. The `openapi.yaml` file is the **source of truth** for provider configurations, types, and settings.

### Generation Targets

Running `task generate` produces:

1. **Provider constants** → `providers/constants/constants.go`
2. **Provider client config** → `providers/client/client.go`
3. **Common types** → `providers/types/common_types.go` (via oapi-codegen)
4. **Provider transformers** → `providers/transformers/{provider}.go`
5. **Provider registry** → `providers/registry/registry.go`
6. **Configuration** → `config/config.go`
7. **Documentation** → `Configurations.md`
8. **Helm defaults** → `charts/inference-gateway/templates/*defaults.yaml`
9. **Helm values** → `charts/inference-gateway/values.yaml`
10. **Docker env examples** → `examples/docker-compose/*/.env.example`
11. **MCP types** → `mcp/generated_types.go` (from MCP schema)

### Code Gen Architecture

```text
cmd/generate/main.go                  # CLI entry
└── internal/
    ├── codegen/codegen.go            # Main generator (Go files)
    ├── dockergen/dockergen.go        # .env.example files
    ├── kubegen/kubegen.go            # K8s/Helm templates
    ├── mdgen/mdgen.go                # Configurations.md
    └── openapi/openapi.go            # OpenAPI parser
```

### When to Regenerate

Always run `task generate` after:

- Adding/modifying a provider in `openapi.yaml`
- Changing the OpenAPI schema/types
- Updating MCP schema (then also `task mcp:schema:download`)
- Before committing (pre-commit hook does this automatically)

## Project Conventions

### Code Style

- **Formatting**: `gofmt` + `prettier` (enforced by `task format`)
- **Linting**: `golangci-lint` v2 with `gocritic`, `gofmt`, `goimports` (`.golangci.yml`)
- **Imports**: Group standard library, external, internal imports
- **Naming**: Follow Go conventions with descriptive names
- **Log messages**: Use lowercase (e.g., `logger.Info("request received")`)

### Development Best Practices

1. **Early Returns**: Favor early returns to reduce nesting depth
2. **Switch Statements**: Prefer switch over if-else chains
3. **Code to Interfaces**: Define interfaces for testability (see `IProvider`, `Agent`, `Router`)
4. **Table-Driven Testing**: Use struct-based test cases with `t.Run()`
5. **Error Logging**: Pass error as second arg to `logger.Error()`
6. **Structured Logging**: Use key-value pairs (`logger.Info("msg", "key", value)`)
7. **Type Safety**: Use strong typing; avoid `interface{}` (except generated code with `any`)
8. **Isolated Tests**: Each test should have independent mock dependencies
9. **Conventional Commits**: Follow for automated releases (e.g., `feat:`, `fix:`, `chore:`)
10. **Go Generate**: Use `//go:generate` directives for mock and code generation

### File Conventions

- **Generated files**: Marked with `// Code generated from OpenAPI schema. DO NOT EDIT.`
- **Interface files**: Named `interfaces.go` or by feature name
- **Implementation files**: Named by component (e.g., `provider.go`, `client.go`, `agent.go`)
- **Test files**: Same package or `_test` suffix; integration tests in `tests/` package
- **Mock files**: Auto-generated in `tests/mocks/` with descriptive package names

### Commit Convention

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```text
feat: add support for new-provider
fix: resolve streaming timeout issue
chore(deps): bump go.opentelemetry.io/otel
docs: update configuration examples
test: add MCP middleware test cases
```

## Important Files

### Configuration & Build

| File | Purpose |
| ---- | ------- |
| `openapi.yaml` | API spec - **source of truth** for code generation |
| `Taskfile.yml` | Task runner - all dev commands |
| `go.mod` | Go module with pinned version and dependencies |
| `.golangci.yml` | Go linter configuration (v2 format) |
| `.goreleaser.yaml` | Release automation |
| `.releaserc.yaml` | semantic-release configuration |
| `.editorconfig` | Editor formatting rules |
| `.spectral.yaml` | OpenAPI linting rules |
| `Dockerfile` | Multi-stage production Docker build |

### Source Code

| File | Purpose |
| ---- | ------- |
| `cmd/gateway/main.go` | Server entry point, initialization |
| `cmd/generate/main.go` | Code generation CLI |
| `api/routes.go` | All API route handlers |
| `api/middlewares/mcp.go` | MCP middleware (injects tools, runs agent) |
| `config/config.go` | Config struct and loader |
| `providers/core/interfaces.go` | `IProvider` interface definition |
| `providers/core/provider.go` | `ProviderImpl` base implementation |
| `providers/registry/registry.go` | Provider registry (GENERATED) |
| `providers/routing/model_mapping.go` | Model-to-provider prefix mapping |
| `mcp/agent.go` | MCP agent loop (tool execution) |
| `mcp/client.go` | MCP HTTP client |
| `logger/logger.go` | Logger interface + zap impl |
| `otel/otel.go` | OpenTelemetry metrics |

### Documentation

| File | Purpose |
| ---- | ------- |
| `README.md` | Project overview and usage |
| `CONTRIBUTING.md` | Contribution guidelines |
| `CLAUDE.md` | Development guidance for Claude Code |
| `Configurations.md` | Auto-generated env var documentation |
| `CHANGELOG.md` | Auto-generated release changelog |

### CI/CD (`.github/workflows/`)

| Workflow | Purpose |
| -------- | ------- |
| `ci.yml` | PR/main CI: lint, build, test, benchmark |
| `release.yml` | Release automation with goreleaser |
| `artifacts.yml` | Build artifacts |
| `claude.yml` | Claude Code integration |
| `infer.yml` | Inference CLI integration |
| `stale.yml` | Stale issue/PR management |
| `dependabot.yml` | Automated dependency updates |

## Deployment

### Docker Compose

```bash
cd examples/docker-compose/basic/
cp .env.example .env
# Edit .env with your API keys
docker compose up -d

# Test the gateway
curl http://localhost:8080/health
```

### Kubernetes

```bash
cd examples/kubernetes/basic/
# Uses the Taskfile for deploying
task deploy

# Port-forward to access
kubectl port-forward svc/inference-gateway 8080:8080
```

### Helm

```bash
helm install inference-gateway ./charts/inference-gateway \
  --set openai.apiKey="sk-..." \
  --set environment=production
```

### Building from Source

```bash
# Build binary
task build
./bin/inference-gateway

# Build Docker image
docker build -t inference-gateway .
```

## Monitoring and Observability

### Metrics Endpoint

When `TELEMETRY_ENABLE=true`, metrics are available at `http://localhost:9464/metrics` in Prometheus format.

### Available Metrics

- **Token Usage**: `llm_usage_prompt_tokens_total`, `llm_usage_completion_tokens_total`, `llm_usage_total_tokens_total` (labels: provider, model)
- **Request/Response**: `llm_requests_total`, `llm_responses_total`, `llm_request_duration` (labels: provider, request_method, request_path, status_code)
- **Tool Calls**: `llm_tool_calls_total`, `llm_tool_calls_success_total`, `llm_tool_calls_failure_total`, `llm_tool_call_duration`
  (labels: provider, model, tool_type, tool_name)

### Pre-built Monitoring

- Docker Compose: `examples/docker-compose/monitoring/` (Grafana + Prometheus)
- Kubernetes: `examples/kubernetes/monitoring/` (Prometheus Operator + Grafana)

## Troubleshooting

### Common Issues

| Issue | Solution |
| ----- | -------- |
| Code generation conflicts | Run `task generate` and commit the changes |
| Linting errors | Run `task lint` and fix, check `.golangci.yml` |
| Test failures | Ensure mocks are current: `go generate ./...` |
| Build failures | Verify Go version (1.26.2 in `go.mod`) |
| Provider not found | Check `{PROVIDER}_API_KEY` env var is set |
| MCP not working | Verify `MCP_ENABLE=true` and server URLs are correct |
| Streaming issues | Check provider supports SSE streaming |
| Vision not working | Set `ENABLE_VISION=true` |

### Debug Mode

Set `ENVIRONMENT=development` for:

- Detailed request/response logging
- Content truncation for large messages (controlled by `DEBUG_CONTENT_TRUNCATE_WORDS`)
- Debug-level telemetry output
- Dev-mode proxy modifiers (`internal/proxy/proxy.go`)

### Pre-commit Hook

The pre-commit hook (`scripts/pre-commit-check.sh`) ensures code generation is clean before committing.
If it fails, you likely need to run `task generate` and commit the generated changes.

## Security Notes

- Never commit API keys or secrets
- All configuration is environment-based (use `.env` or secrets manager)
- Vision support is disabled by default for security (`ENABLE_VISION=false`)
- OIDC authentication available for production deployments
- TLS support with `SERVER_TLS_CERT_PATH` and `SERVER_TLS_KEY_PATH`
- Regular dependency updates via Dependabot
- OpenAPI specification is linted with Spectral for security rules

---

_For questions, refer to CONTRIBUTING.md or open an issue on GitHub._
_Generated for AI agents working with the Inference Gateway project._
