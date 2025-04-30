# Model Context Protocol Integration Example

This example demonstrates how to integrate the Model Context Protocol (MCP) with Inference Gateway, allowing LLMs to access external tools and data through the MCP server.

## Overview

The Model Context Protocol is an open standard for implementing function calling in AI applications. This example shows how to:

1. Connect the Inference Gateway to an MCP server
2. Route LLM requests through the MCP middleware
3. Discover and utilize tools provided by the MCP server
4. Execute tool calls and return results to the LLM

## Components

- **Inference Gateway**: The main service that proxies requests to LLM providers
- **MCP Weather Server**: A simple MCP server that provides weather data tools

## Setup Instructions

### Prerequisites

- Docker and Docker Compose
- OpenAI API key (or other supported LLM provider)

### Environment Variables

Set your OpenAI API key:

```bash
export OPENAI_API_KEY=your_openai_api_key
```

### Start the Services

```bash
docker-compose up
```

## Usage

Once the services are running, you can make requests to the Inference Gateway specifying the MCP provider:

```bash
curl -X POST http://localhost:8080/v1/chat/completions?provider=mcp \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openai/gpt-3.5-turbo",
    "messages": [
      {"role": "user", "content": "What's the weather like in San Francisco?"}
    ]
  }'
```

The Inference Gateway will:

1. Discover the weather tools available from the MCP server
2. Inject these tools into the LLM request
3. Process any tool calls made by the LLM
4. Return the complete response with tool results

## Configuration Options

The following environment variables can be configured:

- `MCP_SERVER_URL`: URL of the MCP server
- `MCP_AUTH_TOKEN`: Optional authentication token for the MCP server
- `MCP_ENABLE_SSE`: Set to "true" to enable Server-Sent Events for streaming
- `PROVIDERS_MCP_ENABLED`: Set to "true" to enable the MCP provider

## Adding Custom MCP Servers

You can replace the example weather server with any MCP-compliant server by:

1. Updating the `MCP_SERVER_URL` environment variable
2. Ensuring the server implements the MCP specification
3. Verifying the server has proper CORS settings for web clients

## Learn More

- [Model Context Protocol Documentation](https://modelcontextprotocol.io)
- [Inference Gateway Documentation](https://github.com/inference-gateway/inference-gateway)
