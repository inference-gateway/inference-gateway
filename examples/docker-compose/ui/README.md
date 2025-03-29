# UI Docker Compose Example with Open-WebUI

This example demonstrates how to set up the Inference Gateway with a user interface (Open-WebUI) using Docker Compose.

## Overview

The UI example sets up:

- Inference Gateway service
- Open-WebUI for a user-friendly interface
- Multiple provider configurations
- (Optional) Ollama for local model hosting

## Prerequisites

- Docker
- Docker Compose

## Setup Instructions

1. Create a `.env` file based on the provided example:

```bash
cp .env.example .env
```

2. Edit the `.env` file to configure your model providers. At minimum, you should set up one provider's API key:

```
# Choose one or more providers and configure API keys
ANTHROPIC_API_KEY=your_anthropic_api_key
CLOUDFLARE_API_KEY=your_cloudflare_api_key
COHERE_API_KEY=your_cohere_api_key
GROQ_API_KEY=your_groq_api_key
OPENAI_API_KEY=your_openai_api_key
DEEPSEEK_API_KEY=your_deepseek_api_key
```

For Cloudflare, you'll also need to update the API URL with your account ID:

```
CLOUDFLARE_API_URL=https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}/ai/v1
```

3. Start the Inference Gateway and Open-WebUI:

```bash
docker compose up -d
```

4. Verify the services are running:

```bash
docker compose ps
```

## Accessing the UI

Once the services are running, you can access the Open-WebUI at:

```
http://localhost:3000
```

## Configuration

### Inference Gateway Configuration

The Inference Gateway is configured with the following settings in the `.env` file:

- Server settings (ports, timeouts)
- Client connection parameters
- Provider API endpoints and keys
- Authentication options (disabled by default)

### Open-WebUI Configuration

The Open-WebUI is configured to connect to the Inference Gateway at startup. You can:

1. Add models through the UI
2. Create and manage chats
3. Customize the UI settings
4. Configure API providers

## Testing the Gateway Directly

You can also test the gateway API directly using curl:

```bash
curl -X GET http://localhost:8080/v1/models
```

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

## Adding Local Models with Ollama

This example can be extended to include local models using Ollama:

1. Uncomment the Ollama service in the `docker-compose.yml` file
2. Set the `OLLAMA_API_URL=http://ollama:8080/v1` in your `.env` file
3. Restart the services with `docker compose up -d`
4. Pull models in Ollama using `docker compose exec ollama ollama pull llama3`

## Additional Resources

- [Main Documentation](../../README.md)
- [Basic Example](../basic/README.md) - Simple setup without UI
- [Hybrid Example](../hybrid/README.md) - For using both local and cloud providers
- [Authentication Example](../authentication/README.md) - For adding authentication
- [Configuration Guide](../../../Configurations.md)

## Support

If you encounter any issues with this example, please [open an issue](https://github.com/inference-gateway/inference-gateway/issues/new) on GitHub.
