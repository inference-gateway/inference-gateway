# Examples using Docker Compose

This directory contains examples that demonstrate how to use the Inference Gateway with Docker Compose.

## Prerequisites

- Docker
- Docker Compose

## Available Examples

- [Basic](basic/README.md) - Simple setup with a single model provider
- [Hybrid](hybrid/README.md) - Configuration with multiple model providers (cloud and local)
- [Authentication](authentication/README.md) - Adding authentication to your gateway
- [Monitoring](monitoring/README.md) - Setting up monitoring for your gateway
- [UI](ui/README.md) - Setting up a user interface for the gateway

## Quick Start

Each example directory contains:

- A README with specific instructions
- A `docker-compose.yml` file
- An `.env.example` file

To run any example:

1. Navigate to the example directory:

```bash
cd examples/docker-compose/[example-name]
```

2. Copy the environment file and customize as needed:

```bash
cp .env.example .env
```

3. Start the services:

```bash
docker compose up -d
```

4. Follow the specific instructions in the example's README for testing and usage

## Environment Variables

Common environment variables used across examples:

| Variable             | Description                    | Default |
| -------------------- | ------------------------------ | ------- |
| `SERVER_PORT`        | Port the gateway listens on    | `8080`  |
| `LOG_LEVEL`          | Logging level                  | `info`  |
| `PROVIDER_*_API_KEY` | API key for specific providers | -       |

## Additional Resources

- [Main Documentation](../../README.md)
- [Kubernetes Examples](../kubernetes/README.md)
- [Configuration Guide](../../Configurations.md)

## Support

If you encounter any issues with these examples, please [open an issue](https://github.com/inference-gateway/inference-gateway/issues/new) on GitHub.
