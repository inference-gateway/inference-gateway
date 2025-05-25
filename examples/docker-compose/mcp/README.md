# Model Context Protocol Integration Example

This example demonstrates how to integrate the Model Context Protocol (MCP) with Inference Gateway, allowing LLMs to access external tools and data through multiple MCP servers.

## Table of Contents

- [Model Context Protocol Integration Example](#model-context-protocol-integration-example)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Components](#components)
  - [MCP Inspector](#mcp-inspector)
    - [Accessing the Inspector](#accessing-the-inspector)
    - [Using the Inspector](#using-the-inspector)
  - [Setup Instructions](#setup-instructions)
    - [Prerequisites](#prerequisites)
    - [Environment Variables](#environment-variables)
    - [Start the Services](#start-the-services)
  - [Usage](#usage)
    - [Example 1: Time Tool](#example-1-time-tool)
    - [Example 2: Search Tool](#example-2-search-tool)
    - [Example 3: Multiple Tools](#example-3-multiple-tools)
    - [Example 4: MCP Streaming](#example-4-mcp-streaming)
    - [Example 5: Filesystem Operations](#example-5-filesystem-operations)
    - [Example 6: Directory Management](#example-6-directory-management)
    - [Example 7: File Information and Management](#example-7-file-information-and-management)
    - [Example 8: List Available MCP Tools](#example-8-list-available-mcp-tools)
  - [How It Works](#how-it-works)
  - [Available Tools](#available-tools)
    - [Time Server Tools](#time-server-tools)
    - [Search Server Tools](#search-server-tools)
    - [Filesystem Server Tools](#filesystem-server-tools)
  - [Configuration Options](#configuration-options)
  - [Adding Custom MCP Servers](#adding-custom-mcp-servers)
  - [Learn More](#learn-more)

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
- **MCP Filesystem Server**: A practical MCP server that provides file operations (read, write, delete, list directories)
- **MCP Inspector**: A web-based debugging tool for exploring and testing MCP servers

## MCP Inspector

The MCP Inspector is included in this example to help you debug and explore your MCP servers. It provides a web interface for:

- **Server Discovery**: View all connected MCP servers and their capabilities
- **Tool Exploration**: Browse available tools from each server with their schemas
- **Interactive Testing**: Execute tool calls directly and see the responses
- **Protocol Debugging**: Monitor MCP protocol messages and debug connection issues

### Accessing the Inspector

Once the services are running, you can access the MCP Inspector at:

```
http://localhost:6274
```

The inspector will automatically connect to all the MCP servers configured in the docker-compose setup:

- Time Server: `http://mcp-time-server:8081/mcp`
- Search Server: `http://mcp-search-server:8082/mcp`
- Filesystem Server: `http://mcp-filesystem-server:8083/mcp`

### Using the Inspector

1. **Browse Servers**: The left panel shows all connected MCP servers
2. **Explore Tools**: Click on a server to see its available tools and capabilities
3. **Test Tools**: Select a tool to see its input schema and execute test calls
4. **View Responses**: See real-time responses and debug any issues

The inspector is particularly useful for:

- Verifying that your MCP servers are working correctly
- Understanding the available tools and their parameters
- Testing tool calls before integrating them into your applications
- Debugging connection issues or protocol errors

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

### Example 5: Filesystem Operations

```bash
curl -X POST http://localhost:8080/v1/chat/completions -d '{
  "model": "groq/meta-llama/llama-4-scout-17b-16e-instruct",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant with access to filesystem operations."
    },
    {
      "role": "user",
      "content": "Create a file called hello.txt with the content \"Hello, MCP World!\" and then read it back to me."
    }
  ]
}'
```

Notice the file was created in filesystem-data directory.

### Example 6: Directory Management

```bash
curl -X POST http://localhost:8080/v1/chat/completions -d '{
  "model": "groq/meta-llama/llama-4-scout-17b-16e-instruct",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant with access to filesystem operations."
    },
    {
      "role": "user",
      "content": "Create a directory called projects, then create a subdirectory called mcp-demo, and list the contents of the projects directory."
    }
  ]
}'
```

### Example 7: File Information and Management

```bash
curl -X POST http://localhost:8080/v1/chat/completions -d '{
  "model": "groq/meta-llama/llama-4-scout-17b-16e-instruct",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant with access to filesystem operations."
    },
    {
      "role": "user",
      "content": "Check if a file called config.json exists, and if not, create it with some sample JSON configuration data. Then show me the file information."
    }
  ]
}'
```

### Example 8: List Available MCP Tools

You can also query the Inference Gateway to see all available tools from connected MCP servers:

```bash
curl -X GET http://localhost:8080/v1/mcp/tools
```

This endpoint returns a JSON response containing all available tools from all connected MCP servers, including:

- Tool names and descriptions
- Required and optional parameters
- Input/output schemas
- Which MCP server provides each tool

Example response:

```json
{
  "tools": [
    {
      "name": "time",
      "description": "Get the current time",
      "server": "http://mcp-time-server:8081/mcp",
      "inputSchema": {
        "type": "object",
        "properties": {
          "format": {
            "type": "string",
            "description": "Time format (ISO, human-readable, etc.)"
          }
        }
      }
    },
    {
      "name": "search",
      "description": "Perform web search",
      "server": "http://mcp-search-server:8082/mcp",
      "inputSchema": {
        "type": "object",
        "properties": {
          "query": {
            "type": "string",
            "description": "Search query"
          }
        },
        "required": ["query"]
      }
    },
    {
      "name": "write_file",
      "description": "Write content to a file",
      "server": "http://mcp-filesystem-server:8083/mcp",
      "inputSchema": {
        "type": "object",
        "properties": {
          "path": {
            "type": "string",
            "description": "File path"
          },
          "content": {
            "type": "string",
            "description": "File content"
          }
        },
        "required": ["path", "content"]
      }
    }
  ]
}
```

## How It Works

When you send a request to the Inference Gateway, it will:

1. Discover the tools available from all MCP servers (time, search, and filesystem)
2. Inject these tools into the LLM request
3. Process any tool calls made by the LLM
4. Return the complete response with tool results

## Available Tools

### Time Server Tools

- **time**: Get the current time in various formats

### Search Server Tools

- **search**: Perform web searches for information

### Filesystem Server Tools

- **write_file**: Write content to a file (supports overwrite and append modes)
- **read_file**: Read content from a file
- **delete_file**: Delete a file
- **list_directory**: List directory contents (supports recursive listing)
- **create_directory**: Create a directory
- **file_exists**: Check if a file or directory exists
- **file_info**: Get detailed information about a file or directory

All filesystem operations are sandboxed to `/tmp/mcp-files` for security.

## Configuration Options

The following environment variables can be configured:

- `MCP_ENABLE`: Set to "true" to enable MCP middleware
- `MCP_EXPOSE`: Set to "true" to expose MCP endpoints
- `MCP_SERVERS`: Comma-separated list of MCP server URLs

## Adding Custom MCP Servers

You can add more MCP-compliant servers to the setup by:

1. Adding the server URL to the `MCP_SERVERS` environment variable
2. Ensuring the server implements the MCP specification
3. Verifying the server has proper CORS settings for web clients
4. Updating the docker-compose.yml to include your new MCP server

The current example includes three servers running on different ports:

- Time Server: http://mcp-time-server:8081/mcp
- Search Server: http://mcp-search-server:8082/mcp
- Filesystem Server: http://mcp-filesystem-server:8083/mcp

## Learn More

- [Model Context Protocol Documentation](https://modelcontextprotocol.github.io/)
- [Inference Gateway Documentation](https://github.com/inference-gateway/inference-gateway)
- [MCP Server Implementation](https://github.com/modelcontextprotocol/server)
