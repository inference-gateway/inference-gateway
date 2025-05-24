# Model Context Protocol Integration Example

This example demonstrates how to integrate the Model Context Protocol (MCP) with Inference Gateway, allowing LLMs to access external tools and data through multiple MCP servers.

## Overview

The Model Context Protocol is an open standard for implementing function calling in AI applications. This example shows how to:

1. Connect the Inference Gateway to multiple MCP servers simultaneously
2. Route LLM requests through the MCP middleware
3. Discover and utilize tools provided by different MCP servers
4. Execute tool calls and return results to the LLM

## Components

- **Inference Gateway**: The main service that proxies requests to LLM providers
- **MCP Time Server**: A simple MCP server that provides time data tools
- **MCP Search Server**: A simple MCP server that provides web search functionality

## Setup Instructions

### Prerequisites

- Docker and Docker Compose
- Groq API key

### Environment Variables

Set your Groq API key:

```bash
export GROQ_API_KEY=your_groq_api_key
```

### Start the Services

```bash
docker-compose up
```

## Usage

Once the services are running, you can make requests to the Inference Gateway using the MCP middleware:

### Example 1: Time Tool

```bash
curl -X POST http://localhost:8080/v1/chat/completions -d '{
  "model": "groq/meta-llama/llama-4-scout-17b-16e-instruct",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Hi, whats the current time?"
    }
  ]
}'
```

### Example 2: Search Tool

```bash
curl -X POST http://localhost:8080/v1/chat/completions -d '{
  "model": "groq/meta-llama/llama-4-scout-17b-16e-instruct",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Find me information about the Model Context Protocol."
    }
  ]
}'
```

### Example 3: Multiple Tools

```bash
curl -X POST http://localhost:8080/v1/chat/completions -d '{
  "model": "groq/meta-llama/llama-4-scout-17b-16e-instruct",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "What is the current time and also find me information about the Model Context Protocol."
    }
  ]
}'
```

### Example 4: MCP Streaming

```bash
curl -X POST http://localhost:8080/v1/chat/completions -d '{
  "model": "groq/meta-llama/llama-4-scout-17b-16e-instruct",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "What is the current time? and also find me information about the Model Context Protocol."
    }
  ],
  "stream": true
}'
```

## How It Works

When you send a request to the Inference Gateway, it will:

1. Discover the tools available from both MCP servers (time and search)
2. Inject these tools into the LLM request
3. Process any tool calls made by the LLM
4. Return the complete response with tool results

## Configuration Options

The following environment variables can be configured:

- `ENABLE_MCP`: Set to "true" to enable MCP middleware
- `MCP_SERVERS`: Comma-separated list of MCP server URLs

## Adding Custom MCP Servers

You can add more MCP-compliant servers to the setup by:

1. Adding the server URL to the `MCP_SERVERS` environment variable
2. Ensuring the server implements the MCP specification
3. Verifying the server has proper CORS settings for web clients
4. Updating the docker-compose.yml to include your new MCP server

## Learn More

- [Model Context Protocol Documentation](https://modelcontextprotocol.github.io/)
- [Inference Gateway Documentation](https://github.com/inference-gateway/inference-gateway)
- [MCP Server Implementation](https://github.com/modelcontextprotocol/server)
