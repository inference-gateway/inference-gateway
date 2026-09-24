# AGENTS.md

Guidance for coding agents working in this repository. `CLAUDE.md` is a symlink to this file.

The Inference Gateway is a Go service that proxies a single OpenAI-compatible API to many upstream LLM providers. Most per-provider code is generated from `openapi.yaml`; the runtime is a thin Gin server with a configurable middleware chain.

## Commands

Everyday tasks go through `Taskfile.yml`:

- `task run` — run the gateway (`cmd/gateway/main.go`)
- `task build` — produce `bin/inference-gateway`
- `task test` — `go test -race -v ./...`
- `task generate` — regenerate everything from `openapi.yaml` + `internal/mcp/mcp-schema.yaml`
- `task format` — `prettier --write .` then `go fmt ./...`
- `task lint` — `golangci-lint run` + `markdownlint`
- `task openapi:lint` — Spectral lint of `openapi.yaml`
- `task benchmark` — benchmarks under `./tests/...`
- `task benchmark:check` — benchmarks + `benchstat` gate against `tests/benchmarks/baseline.txt` (tolerances are vars on the task)
- `task benchmark:baseline` — refresh the baseline; run it on a GitHub-hosted runner (numbers are machine-specific) and commit the file in the same PR as an intentional perf change or a new benchmark

Single test: `go test -v -run TestName ./path/to/pkg`. The pinned toolchain (Go 1.26.7, golangci-lint, mockgen, Spectral) is declared in `.flox/env/manifest.toml`; `flox activate` brings it in.

## Code generation

`openapi.yaml` is the source of truth. `task generate` emits `providers/transformers/*.go`, `providers/registry/registry_data.go`, `providers/types/common_types.go`, `providers/client/config.go`, `providers/constants/constants.go`, `config/config.go`, `internal/mcp/generated_types.go`, `Configurations.md`, the `examples/docker-compose/*/.env.example` files, and the mocks. Anything with a `// Code generated ... DO NOT EDIT.` header will be clobbered — edit the spec or the hand-written siblings (`providers/client/client.go`, `providers/registry/registry.go`, `config/load.go`, `providers/constants/static.go`) instead. CI only re-runs `go generate` (mocks) before its dirty-tree check; the full `task generate` drift check lives in the pre-commit hook (triggered when `openapi.yaml` or `internal/codegen/` is staged), so run it yourself and commit the output. The spec files themselves — `openapi.yaml` and `internal/mcp/mcp-schema.{json,yaml}` — are vendored from the [inference-gateway/schemas](https://github.com/inference-gateway/schemas) repo (`task oas-sync` fetches both and regenerates; `task oas-download` / `task mcp:schema:download` fetch one each; all pinned with `SCHEMAS_REF`); land spec changes there too, or the next download reverts them.

`providers/core/community_*.json` (pricing, context windows, modalities) are synced from models.dev via `task pricing:sync` / `contextwindow:sync` / `modalities:sync`; hand-maintained entries go in the `*.overrides.json` companions, not the synced tables.

Adding a provider: edit `openapi.yaml` (`Provider` enum + `x-provider-configs`, and the `Config` schema's `x-config` section) then `task generate`; `tests/provider_drift_test.go` fails if wiring is incomplete. Provider IDs must be lowercase Go-identifier-safe. Transformers listed in `.openapi-ignore` are exempt from regeneration — hand-edit and list a transformer there when a provider's models endpoint isn't OpenAI-compatible.

## Architecture

`cmd/gateway/main.go` is the only entry point. Routes are registered in `cmd/gateway/main.go` (handlers live in `api/routes.go` and `api/metrics.go`): `/health`, `/v1/models`, `/v1/mcp/tools`, `/v1/chat/completions`, `/v1/responses`, `/v1/messages`, `/v1/images/generations`, `/v1/images/edits`, `/v1/audio/speech`, `/v1/audio/sfx`, `/v1/audio/music`, `/v1/videos`, `/v1/videos/:video_id`, `/v1/videos/:video_id/content`, `/v1/metrics`, and `ANY /proxy/:provider/*path`. Middleware chain: `tracing` (opt-in) → `logger` → `telemetry` (opt-in) → `OIDC auth` (opt-in) → `guardrails` (always registered, active when `GUARDRAILS_ENABLED`; placed before MCP so it wraps MCP's writer) → `MCP` (opt-in). For tool calls the MCP middleware hands off to the in-process agent loop in `internal/mcp/agent.go`, which calls `provider.ChatCompletions` / `provider.StreamChatCompletions` directly for up to `MaxAgentIterations` rounds - no HTTP request re-enters the gateway. `X-MCP-Bypass` is only honoured as an inbound client header in `api/middlewares/mcp.go` (any non-empty value skips the middleware); nothing sets it.

A "provider" is one upstream LLM API. `providers/core/` holds the `IProvider` interface; `providers/registry/` builds providers on demand; `providers/routing/model_mapping.go` maps a `provider/model` prefix (or `?provider=` query param) to a provider. Optional model pools (`ROUTING_ENABLED` + `ROUTING_CONFIG_PATH`, a YAML of logical aliases → round-robin deployments in `providers/routing/pool.go`) are resolved first, but only for `/v1/chat/completions` and only when `?provider=` is absent.

Config is env-only: `config/load.go` fills the generated `config.Config` via `go-envconfig`, then adds `<ID>_API_URL` / `<ID>_API_KEY` for every provider in the registry. `Configurations.md` is the generated reference of every variable.

## Conventions

- **Conventional Commits** are enforced by semantic-release; non-conforming subjects break releases.
- **Import order** is enforced by the `gci` formatter (see `.golangci.yml`): standard library, `github.com/stretchr/testify` + `go.uber.org/mock`, `tests/mocks`, third-party (`default`), `github.com/inference-gateway/*`, then this module. Fix locally with `golangci-lint fmt`. Every non-standard-library import must be named after its last path element (`gin "github.com/gin-gonic/gin"`), enforced by `importas`; pin an alias in `.golangci.yml` only when two packages would collide. Fix with `golangci-lint run --fix`.
- **Releases are automated** — never create tags/releases, publish packages, edit `CHANGELOG.md`, or bump versions manually.
- **No magic numbers/strings** — name meaningful literals as consts (see `internal/guardrails`) and reference them everywhere, including tests.
- **Tests** live next to the package or in `tests/`; mocks are committed under `tests/mocks/` (`mockgen` via `//go:generate`); after changing a mocked interface, `go generate ./providers/... ./api/... ./otel/... ./logger/... ./internal/mcp/...` refreshes just the mocks.
- **Pre-commit** (`task pre-commit:install`) is the source of truth for "PR-ready".

## Security

Never commit provider API keys, tokens, or local `.env` files. Use `.env.example` as templates. Review auth, telemetry, and routing changes carefully — they affect gateway-wide behavior.
