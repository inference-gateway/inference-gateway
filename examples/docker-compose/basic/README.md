# Basic Docker Compose Example

This example demonstrates how to set up the Inference Gateway with Docker Compose for a simple configuration with a single cloud-based model provider.

## Overview

The Basic example sets up:

- Inference Gateway service
- Single cloud provider configuration

## Prerequisites

- Docker
- Docker Compose

## Setup Instructions

1. Create a `.env` file based on the provided example:

```bash
cp .env.example .env
```

2. Edit the `.env` file to configure your model provider:

```
OPENAI_API_KEY=your_api_key_here
```

Or any other provider you want to use.

3. Start the Inference Gateway:

```bash
docker compose up -d
```

4. Verify the gateway is running:

```bash
docker compose ps
```

## Testing the Gateway

You can test the gateway using curl commands:

### List available models

```bash
curl -X GET http://localhost:8080/v1/models
```

### Send a chat completion request

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {
        "role": "user",
        "content": "Hello, how are you today?"
      }
    ]
  }'
```

## Configuration Options

You can configure additional options in the `.env` file:

- `SERVER_PORT` - The port the gateway listens on
- `LOG_LEVEL` - Logging level (debug, info, warn, error)
- `ENABLE_TELEMETRY` - Enable/disable telemetry (true/false)
- `ENABLE_AUTH` - Enable/disable authentication (true/false)
- `*_API_URL` - Custom API URL for the provider (if needed)
- `*_API_KEY` - API key for the provider

## Additional Resources

- [Main Documentation](../../README.md)
- [Hybrid Example](../hybrid/README.md) - For using both local and cloud providers
- [Configuration Guide](../../../docs/configuration.md)
