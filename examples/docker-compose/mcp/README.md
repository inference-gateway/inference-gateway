# Model Context Protocol Integration Example

This example demonstrates integrating the Model Context Protocol (MCP) with
Inference Gateway, enabling LLMs to access external tools and data through
multiple MCP servers.

## Features

- **✨ Server-Sent Events (SSE)**: Real-time streaming with dual JSON-RPC and SSE protocol support
- **🔍 MCP Inspector**: Web-based debugging tool for exploring and testing MCP servers
- **🛠️ Multiple Tools**: Time, search, filesystem, and pizza-related tools
- **🔧 Easy Setup**: Docker Compose configuration with CORS support

## Table of Contents

- [Quick Start](#quick-start)
- [Components](#components)
- [MCP Inspector](#mcp-inspector)
- [Usage](#usage)
- [How It Works](#how-it-works)
- [Available Tools](#available-tools)
- [Adding Your Own MCP Servers](#adding-your-own-mcp-servers)
- [Learn More](#learn-more)

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Groq API key

### Setup

```bash
export GROQ_API_KEY=your_groq_api_key
docker-compose up
```

### Test and Troubleshoot

Use the MCP Inspector at `http://localhost:6274` to explore servers, test tools, and troubleshoot any issues.

## Components

- **Inference Gateway**: Main service that proxies requests to LLM providers
- **MCP Time Server**: Provides time data tools
- **MCP Search Server**: Provides web search functionality
- **MCP Filesystem Server**: Provides file operations (read, write, delete, list directories)
- **MCP Pizza Server**: TypeScript MCP server providing pizza-related tools using `@modelcontextprotocol/sdk`
- **MCP Inspector**: Web-based debugging tool for exploring MCP servers

## MCP Inspector

Debug and explore your MCP servers with the web interface at `http://localhost:6274`.

**Capabilities:**

- View all connected servers and their tools
- Browse tool schemas and parameters
- Execute tool calls and see responses
- Monitor protocol messages and debug issues

**Connected Servers:**

- Time Server: `http://mcp-time-server:8081/mcp`
- Search Server: `http://mcp-search-server:8082/mcp`
- Filesystem Server: `http://mcp-filesystem-server:8083/mcp`
- Pizza Server: `http://mcp-pizza-server:8084/mcp`

## Usage

Once the services are running, you can make requests to the Inference Gateway using the MCP middleware:

### Example 1: Time Tool

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
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
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
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
  ],
  "stream": true
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

### Example 9: Pizza Server Tools

This example demonstrates using tools from the official TypeScript MCP server
built with `@modelcontextprotocol/sdk`. The pizza server provides pizza-related
tools:

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
  "model": "groq/meta-llama/llama-4-scout-17b-16e-instruct",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Top 5 pizzas in the world. Go!"
    }
  ]
}'
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

### Pizza Server Tools

- **get-top-pizzas**: Returns mock data of the top 5 pizzas in the world with
  detailed information

The pizza server showcases best practices using the `@modelcontextprotocol/sdk`
and includes comprehensive session management, type validation with Zod schemas,
and dual transport support.

## Adding Your Own MCP Servers

**It's incredibly easy to add more MCP servers!** Simply follow these steps:

### Quick Setup

1. **Add your server URL** to the `MCP_SERVERS` environment variable:

   ```bash
   MCP_SERVERS=http://mcp-time-server:8081/mcp,http://mcp-search-server:8082/mcp,http://your-new-server:8085/mcp
   ```

2. **Include your server** in the docker-compose.yml file (if running in Docker)

3. **Restart the services** - that's it! Your tools will automatically be available.

### Requirements for Your MCP Server

- Implements the [MCP specification](https://modelcontextprotocol.io/specification)
- Responds to HTTP requests on the `/mcp` endpoint
- Supports CORS for web clients (if using the MCP Inspector)

### Pre-configured Example Servers

This example includes four pre-configured servers:

- **Time Server**: `http://mcp-time-server:8081/mcp` - Get current time
- **Search Server**: `http://mcp-search-server:8082/mcp` - Web search
  functionality
- **Filesystem Server**: `http://mcp-filesystem-server:8083/mcp` - File
  operations
- **TypeScript Server**: `http://official-ts-server:8084/mcp` - Math and utility
  functions

### Configuration Options

Environment variables you can configure:

- `MCP_ENABLE`: Set to "true" to enable MCP middleware
- `MCP_EXPOSE`: Set to "true" to expose MCP endpoints
- `MCP_SERVERS`: Comma-separated list of MCP server URLs

## Learn More

- [Model Context Protocol Documentation](https://modelcontextprotocol.github.io/)
- [Inference Gateway Documentation](https://github.com/inference-gateway/inference-gateway)
- [MCP Server Implementation](https://github.com/modelcontextprotocol/server)
