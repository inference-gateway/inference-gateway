# Model Context Protocol Integration Example

This example demonstrates integrating the Model Context Protocol (MCP) with
Inference Gateway, enabling LLMs to access external tools and data through
multiple MCP servers.

## Features

- **🌐 Gateway as an MCP server**: one `POST /mcp` endpoint fronting every backend server, for agent clients
- **🔍 MCP Inspector**: Web-based debugging tool for exploring and testing MCP servers
- **🛠️ Multiple Tools**: Time, search, filesystem, pizza and calculator tools
- **📦 Official SDKs**: the pizza (TypeScript) and calculator (Go) servers show how to serve MCP `2026-07-28` with the official SDKs
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
cp .env.example .env
# Edit .env and set GROQ_API_KEY=your_groq_api_key
docker compose up
```

The gateway reads `GROQ_API_KEY` from the `.env` file (loaded via `env_file`), so
it must be set there rather than only exported in your shell.

### Test and Troubleshoot

Use the MCP Inspector to explore the gateway's tools, run them, and troubleshoot any issues. Open the URL it
prints on startup (`docker compose logs mcp-inspector`) - it carries the session token.

## Components

- **Inference Gateway**: Main service that proxies requests to LLM providers
- **MCP Time Server**: Provides time data tools
- **MCP Search Server**: Provides web search functionality
- **MCP Filesystem Server**: Provides file operations (read, write, delete, list directories)
- **MCP Pizza Server**: Pizza demo tool on the official TypeScript SDK v2 ([pizza-server](pizza-server/))
- **MCP Calculator Server**: Calculator tool with structured output on the official Go SDK ([calculator-server](calculator-server/))
- **MCP Inspector**: Web-based debugging tool for exploring MCP servers

## MCP Inspector

Debug and explore your MCP servers with the web interface on `http://localhost:6274`, using the tokenized URL
from `docker compose logs mcp-inspector`. Its ports are published on `127.0.0.1` only.

**Capabilities:**

- View all connected servers and their tools
- Browse tool schemas and parameters
- Execute tool calls and see responses
- Monitor protocol messages and debug issues

**Connected Server:**

The Inspector is launched against the gateway's own MCP endpoint,
`http://inference-gateway:8080/mcp`, so it sees the tools of all five backend
servers at once. The endpoint speaks MCP `2026-07-28` only, so the Inspector runs
with `--protocol-era modern` (its default is `legacy`). Connect the server card and
the Tools view lists every `mcp_<alias>_<tool>`. The server list is read-only,
because it comes from the launch flags in `docker-compose.yml`.

## Usage

Once the services are running, you can make requests to the Inference Gateway using the MCP middleware:

### Example 1: Time Tool

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
  "model": "deepseek/deepseek-v4-flash",
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
  "model": "deepseek/deepseek-v4-flash",
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
  "model": "deepseek/deepseek-v4-flash",
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
  "model": "deepseek/deepseek-v4-flash",
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
  "model": "deepseek/deepseek-v4-flash",
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
  "model": "deepseek/deepseek-v4-flash",
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
  "model": "deepseek/deepseek-v4-flash",
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

### Example 9: Point an MCP Client at the Gateway

With `MCP_ENABLED=true` and `MCP_EXPOSE=true` the gateway is itself an MCP
server at `POST /mcp`. An agent client declares **one** entry and discovers
every backend server, so adding or removing a server never touches the client
config. Tools are namespaced `mcp_<alias>_<tool>` from the `alias=url` entries
in `MCP_SERVERS`.

The endpoint speaks MCP `2026-07-28` only - no `initialize` handshake, no
session. Every request carries its protocol version, client info and client
capabilities in `params._meta`, and mirrors the version, the method and (for
`tools/call`) the tool name into the `MCP-Protocol-Version`, `Mcp-Method` and
`Mcp-Name` headers. A request missing them is rejected with `400`.

Discover what the gateway supports (optional - any request can go first):

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -H "MCP-Protocol-Version: 2026-07-28" \
  -H "Mcp-Method: server/discover" \
  -d '{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"curl","version":"1.0"},"io.modelcontextprotocol/clientCapabilities":{}}}}'
```

List the whole fleet's tools:

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -H "MCP-Protocol-Version: 2026-07-28" \
  -H "Mcp-Method: tools/list" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"curl","version":"1.0"},"io.modelcontextprotocol/clientCapabilities":{}}}}'
```

Call one of them - the alias decides which backend server runs it:

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -H "MCP-Protocol-Version: 2026-07-28" \
  -H "Mcp-Method: tools/call" \
  -H "Mcp-Name: mcp_time_time" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"mcp_time_time","arguments":{},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"curl","version":"1.0"},"io.modelcontextprotocol/clientCapabilities":{}}}}'
```

If a backend server is down, `tools/list` still returns the healthy servers'
tools and a `tools/call` routed to it comes back as a JSON-RPC error.

Client configuration, e.g. for opencode (the client must support MCP
`2026-07-28`):

```json
{
  "mcp": {
    "inference-gateway": {
      "type": "remote",
      "url": "http://localhost:8080/mcp",
      "enabled": true
    }
  }
}
```

The client keeps its own local tools (bash, read, edit) executing client-side;
only the gateway's tools travel over `/mcp`. When `AUTH_ENABLED=true`, send the
bearer token the same way as for every other endpoint - only `/health` skips
authentication.

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

- **get_top_pizzas**: The top 5 pizzas in the world with their origin,
  description, year created and key ingredients

### Calculator Server Tools

- **calculate**: Add, subtract, multiply or divide two numbers. The result comes
  back as `structuredContent` (`{"result": 42}`), and a division by zero as a
  result with `isError: true`

## Adding Your Own MCP Servers

**It's incredibly easy to add more MCP servers!** Simply follow these steps:

### Quick Setup

1. **Add your server URL** to the `MCP_SERVERS` environment variable, optionally
   giving it an alias with `alias=url`:

   ```bash
   MCP_SERVERS=time=http://mcp-time-server:8081/mcp,search=http://mcp-search-server:8082/mcp,http://your-new-server:8086/mcp
   ```

   Each server gets an alias that namespaces its tools as
   `mcp_<alias>_<tool name>`, so two servers can expose the same tool name. When
   you omit `alias=`, the alias is derived from the URL host (the entry above
   becomes `your-new-server`). Aliases must match `^[a-z0-9_-]+$`, be unique, and
   must not be `tools` (which is reserved for the selector meta-tools).

2. **Include your server** in the docker-compose.yml file (if running in Docker)

3. **Restart the services** - that's it! Your tools will automatically be available.

### Requirements for Your MCP Server

- Speaks MCP `2026-07-28`, the stateless revision: the gateway sends no
  `initialize` and keeps no session, so every `tools/list` and `tools/call` must
  be answerable on its own. A server that requires a handshake or a session is
  marked unavailable. The official SDKs serve it alongside the 2025-era
  protocol:
  - **TypeScript** v2 (`@modelcontextprotocol/server`): mount
    `createMcpHandler`, as the [pizza server](pizza-server/) does. The v1
    `@modelcontextprotocol/sdk` package does not speak `2026-07-28`.
  - **Go** (`github.com/modelcontextprotocol/go-sdk` v1.7+): a Streamable HTTP
    handler with `Stateless: true`, as the [calculator server](calculator-server/)
    does.
  - **Python** v2 (`mcp` 2.x): serves both revisions with no configuration.
- Responds to HTTP requests on the `/mcp` endpoint
- Supports CORS for web clients (if using the MCP Inspector)

### Pre-configured Example Servers

This example includes five pre-configured servers:

- **Time Server**: `http://mcp-time-server:8081/mcp` - Get current time
- **Search Server**: `http://mcp-search-server:8082/mcp` - Web search
  functionality
- **Filesystem Server**: `http://mcp-filesystem-server:8083/mcp` - File
  operations
- **Pizza Server**: `http://mcp-pizza-server:8084/mcp` - Pizza demo on the
  official TypeScript SDK v2
- **Calculator Server**: `http://mcp-calculator-server:8085/mcp` - Calculator
  on the official Go SDK

### Configuration Options

Environment variables you can configure:

- `MCP_ENABLED`: Set to "true" to enable MCP middleware
- `MCP_EXPOSE`: Set to "true" to expose MCP endpoints
- `MCP_SERVERS`: Comma-separated list of MCP servers as `alias=url` or `url`
- `MCP_INCLUDE_TOOLS`: Comma-separated allowlist of tool names to inject. When
  set, only these tools are injected; if empty, all tools are injected
- `MCP_EXCLUDE_TOOLS`: Comma-separated denylist of tool names to skip injecting.
  Takes lower precedence than `MCP_INCLUDE_TOOLS`

### Filtering Injected Tools

By default every tool from every connected MCP server is injected into each chat
completion request. To make requests more token-efficient you can restrict which
tools are injected with an allowlist or a denylist:

- `MCP_INCLUDE_TOOLS` is an allowlist: when set, only the listed tools are
  injected.
- `MCP_EXCLUDE_TOOLS` is a denylist: the listed tools are never injected.

`MCP_INCLUDE_TOOLS` takes precedence over `MCP_EXCLUDE_TOOLS`. Tool names are
matched case-insensitively and the `mcp_` prefix is optional. An entry matches
either the bare tool name or the namespaced `<alias>_<tool name>` form, so
`read_file` applies to every server exposing that tool while
`filesystem_read_file` only applies to the `filesystem` server.

```bash
# Only inject the time and search tools
MCP_INCLUDE_TOOLS=time,search

# Inject everything except the destructive filesystem tools
MCP_EXCLUDE_TOOLS=delete_file,write_file
```

## Learn More

- [Model Context Protocol Documentation](https://modelcontextprotocol.github.io/)
- [Inference Gateway Documentation](https://github.com/inference-gateway/inference-gateway)
- [MCP Server Implementation](https://github.com/modelcontextprotocol/server)
