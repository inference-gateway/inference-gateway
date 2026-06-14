# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

The Inference Gateway is a Go service that proxies a single OpenAI-compatible API to many upstream LLM providers (OpenAI, Anthropic, Groq, Ollama, Cohere, DeepSeek, Google, Mistral, Cloudflare, Moonshot, Ollama Cloud). Most of the per-provider code is generated from `openapi.yaml`; the runtime is a thin Gin server with a configurable middleware chain.

## Commands

Everyday tasks go through `Taskfile.yml`:

- `task run` — run the gateway from `cmd/gateway/main.go`
- `task build` — produce `bin/inference-gateway`
- `task test` — `go test -v ./...`
- `task benchmark` — benchmarks under `./tests/...` (run when touching routing / transformers / MCP)
- `task generate` — regenerate everything from `openapi.yaml` + `internal/mcp/mcp-schema.yaml` (see "Code generation")
- `task format` — `prettier --write .` then `go fmt ./...`
- `task lint` — `golangci-lint run` + `markdownlint` (CLAUDE.md, AGENTS.md, CHANGELOG.md, and Configurations.md are excluded)
- `task openapi:lint` — Spectral lint of `openapi.yaml`
- `task pre-commit:install` — symlinks `scripts/pre-commit-check.sh` into `.git/hooks/pre-commit`; the hook runs generate → format → lint → openapi:lint → build → test, and CI mirrors it

Running a single test: `go test -v -run TestName ./path/to/pkg`. The pinned toolchain (Go 1.26.2, golangci-lint, mockgen, Spectral, kubectl, helm, etc.) is declared in `.flox/env/manifest.toml`; `flox activate` brings it all in.

## Architecture

### Request pipeline

`cmd/gateway/main.go` is the only entry point. It loads `config.Config` from env vars via `sethvargo/go-envconfig`, initializes the logger, optionally starts an OpenTelemetry Prometheus metrics server on `:9464` (`TELEMETRY_ENABLE=true`), builds the provider registry and shared HTTP client, optionally wires up the MCP client / agent / middleware, and registers Gin handlers.

Routes (`api/routes.go`):

- `GET  /health`
- `GET  /v1/models`
- `GET  /v1/mcp/tools`
- `POST /v1/chat/completions` — the main inference endpoint
- `ANY  /proxy/:provider/*path` — passthrough that injects the provider's API key and forwards to the upstream; still subject to the global middleware (notably OIDC auth when enabled)

Middleware chain (registered in `main.go`, defined in `api/middlewares/`): `logger` → `telemetry` (if enabled) → `OIDC auth` (if enabled) → `MCP` (if enabled). The MCP middleware inspects responses for tool calls and re-invokes the upstream provider with tool results; to prevent loops, its internal follow-up requests set `X-MCP-Bypass: true`. Clients can set the same header to opt out. Only `/health` is exempt from the OIDC auth middleware; `/proxy/...` is **not** — so the gateway's own self-proxy calls (chat completions, model listing) must forward the caller's token onto the internal hop (`ctx.Value("authToken")` in `providers/core/provider.go`).

### Provider abstraction

A "provider" is one upstream LLM API. The runtime pieces live under `providers/`:

- `core/` — `IProvider` interface and base `ProviderImpl` (hand-written).
- `client/` — shared HTTP client config (`client.go` is generated).
- `registry/` — `ProviderRegistry.BuildProvider(id, client)` constructs a provider on demand from `cfg.Providers` (`registry.go` is generated).
- `transformers/` — per-provider request/response transformers, one file per provider. All are generated from `openapi.yaml` and start with `// Code generated from OpenAPI schema. DO NOT EDIT.`; protect any that need hand-edits via `.openapi-ignore`.
- `routing/model_mapping.go` — maps a model string like `openai/gpt-4o` to a provider. The only routing rule is the explicit prefix (`openai/`, `groq/`, `anthropic/`, ...); without a prefix, the request must include `?provider=...`.
- `constants/`, `types/` — generated identifiers and OpenAPI-derived Go types.

### Code generation

`openapi.yaml` and `internal/mcp/mcp-schema.yaml` are the source of truth. `task generate`:

1. Builds the `bin/generator` helper (pinned via `task install:generator`).
2. Runs `cmd/generate/main.go` (a thin CLI over `internal/codegen`, `internal/dockergen`, `internal/kubegen`, `internal/mdgen`) repeatedly with different `-type` flags to emit: `providers/client/client.go`, `providers/constants/constants.go`, `providers/transformers/*.go`, `providers/registry/registry.go`, `config/config.go`, `Configurations.md`, `charts/inference-gateway/templates/{secrets,configmap}-defaults.yaml`, `charts/inference-gateway/values.yaml`, and every `examples/docker-compose/*/.env.example`.
3. Runs `oapi-codegen` to emit `providers/types/common_types.go` (then `sed`s `interface{}` → `any`).
4. Runs `bin/generator` to emit `internal/mcp/generated_types.go` from `mcp-schema.yaml`.
5. Runs `go generate ./...` to refresh mocks under `tests/mocks/` (driven by `//go:generate mockgen ...` directives at the top of each interface file — `api/routes.go`, `providers/core/interfaces.go`, `providers/registry/registry.go`, `providers/client/client.go`, `internal/mcp/agent.go`, `internal/mcp/client.go`, `logger/logger.go`, plus OTel).

Anything with the "DO NOT EDIT" header will be clobbered on the next run. Adding a new provider: edit `openapi.yaml`'s `Provider` enum + `x-provider-configs` and run `task generate` — full flow in `CONTRIBUTING.md`. Provider IDs must be lowercase Go-identifier-safe (`openai`, `deepseek`, `newai`).

CI runs `task generate` and fails the build if the working tree is dirty afterwards, so always commit the regenerated files.

### Configuration

`config/config.go` is generated — every supported env var lives in struct tags there and is mirrored into `Configurations.md`. Link users to `Configurations.md` rather than enumerating vars in prose; it'll go stale.

### MCP

`internal/mcp/client.go`, `internal/mcp/init.go`, `internal/mcp/tools.go` (port interface + client implementation) connects to the comma-separated list in `MCP_SERVERS`. `internal/mcp/agent.go` orchestrates the tool-call loop (capped at 10 iterations via `MaxAgentIterations` / `MaxMCPAgentIterations`). `internal/mcp/generated_types.go` is regenerated from `mcp-schema.yaml`. `internal/mcp/transport.go` handles Streamable HTTP and SSE transport modes. `internal/mcp/health.go` handles status polling and health checks. Background reconnection kicks in when `MCP_ENABLE_RECONNECT=true`; the gateway will start even if no MCP server is reachable at boot, as long as reconnect is enabled.

The gateway request handlers (`api/routes.go`, `api/middlewares/mcp.go`) depend on the `mcp.MCPClientInterface` and `mcp.Agent` port interfaces defined in `internal/mcp/`, not on concrete types. Mocks live in `tests/mocks/mcp/` and are regenerated by `go generate ./internal/mcp/...`.

## Conventions

- **Conventional Commits** are enforced by semantic-release (`.releaserc.yaml`); CI uses them to compute the next version, so non-conforming subjects break releases.
- **Tests** live either next to the package (`*_test.go`) or in the top-level `tests/` directory for cross-package and end-to-end flows. Mocks are committed under `tests/mocks/` and regenerated by `task generate`.
- **Pre-commit** is the source of truth for "is this PR-ready": if `scripts/pre-commit-check.sh` passes locally, CI will pass too.
