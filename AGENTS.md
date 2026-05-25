# Repository Guidelines

## Project Structure & Module Organization

The runtime entrypoint is `cmd/gateway/main.go`; code generation lives in
`cmd/generate` and `internal/*gen`.
HTTP routes and middleware are in `api/`, configuration in `config/`, provider
logic in `providers/`, MCP support in `mcp/`, logging in `logger/`, telemetry in
`otel/`, and shared proxy internals in `internal/proxy`. Integration and package
tests are under `tests/`, with some package-local tests such as
`config/config_test.go`. Deployment assets live in `charts/`, `Dockerfile*`, and
`examples/docker-compose` or `examples/kubernetes`.

## Build, Test, and Development Commands

Use `task --list` to see available workflows. Common commands:

- `task build`: builds `bin/inference-gateway` from `cmd/gateway/main.go`.
- `task run`: runs the gateway locally with `go run`.
- `task test`: runs `go test -v ./...`.
- `task benchmark`: runs benchmarks under `tests/`.
- `task format`: runs `prettier --write .` and `go fmt ./...`.
- `task lint`: runs `golangci-lint` and Markdown linting.
- `task openapi:lint`: validates `openapi.yaml` with Spectral.
- `task generate`: regenerates provider, config, Helm, environment, MCP, and
  OpenAPI-derived files. Run it after changing `openapi.yaml` or generator code.
- `task pre-commit:install`: installs the repository pre-commit hook.

Use Flox (`flox activate`) for pinned Go 1.26.2 and tooling.

## Coding Style & Naming Conventions

Follow standard Go formatting with tabs via `go fmt`. Use package names that are
short, lowercase, and domain-specific.
Provider identifiers in `openapi.yaml` must be valid Go identifiers, preferably
one lowercase word such as `openai` or `deepseek`. Keep generated files generated:
change source specs or generators, then run `task generate`.

## Testing Guidelines

Name Go tests `*_test.go` and test functions `TestXxx`. Put package-specific
tests beside the package when they only cover local behavior; use `tests/` for
cross-package gateway, provider, middleware, MCP, and route coverage. Run
`task test` before submitting changes, and run `task benchmark` when changing
performance-sensitive behavior.

## Commit & Pull Request Guidelines

History uses Conventional Commits, for example `fix(ci): Ignore Markdown files`
and `chore(deps): Add codex and bump infer CLI`. Use concise subjects with an
optional scope. Pull requests should describe the change, link issues, include
test results, and mention generated-file updates. For user-visible API,
configuration, Helm, or example changes, update the matching docs or examples.

## Security & Configuration Tips

Do not commit provider API keys, tokens, or local `.env` files. Use the checked-in
`.env.example` files as templates. Review authentication, telemetry, and routing
changes carefully because they affect gateway-wide behavior.
