# @inference-gateway/inference-gateway

npx wrapper for the [Inference Gateway](https://github.com/inference-gateway/inference-gateway) binary.

This package downloads the platform-matching gateway binary from GitHub
Releases on first use, caches it under your user cache directory, and execs it
with the arguments you pass.

> **Not recommended for production.** This is a convenience shim for users who
> already have Node.js installed and want to try the gateway without picking up
> curl/sh or Docker. For production deployments use the
> [container image](https://github.com/inference-gateway/inference-gateway#installation),
> the [install script](https://github.com/inference-gateway/inference-gateway/blob/main/install.sh),
> or download the release archive directly.

## Usage

```bash
# Latest published version
npx @inference-gateway/inference-gateway --help

# Pinned version
npx @inference-gateway/inference-gateway@0.24.6 --version

# Run the gateway (it reads its config from env vars; see Configurations.md)
OPENAI_API_KEY=... npx @inference-gateway/inference-gateway
```

The package version tracks the gateway release version 1:1.

## Supported platforms

| OS    | Architectures                         |
| ----- | ------------------------------------- |
| Linux | x86_64, arm64, armv7                  |
| macOS | x86_64 (Intel), arm64 (Apple Silicon) |

Windows is not supported - use WSL or Docker.

## Environment variables

| Variable                      | Purpose                                                                                   |
| ----------------------------- | ----------------------------------------------------------------------------------------- |
| `INFERENCE_GATEWAY_VERSION`   | Override which release tag to download (e.g. `v0.22.3`). Defaults to the package version. |
| `INFERENCE_GATEWAY_CACHE_DIR` | Override the binary cache directory.                                                      |
| `XDG_CACHE_HOME`              | Standard XDG path; the cache lives under `$XDG_CACHE_HOME/inference-gateway/`.            |

Gateway runtime configuration (API keys, ports, MCP, OIDC, etc.) is read from
the same env vars as the binary - see
[`Configurations.md`](https://github.com/inference-gateway/inference-gateway/blob/main/Configurations.md).

## License

Apache-2.0
