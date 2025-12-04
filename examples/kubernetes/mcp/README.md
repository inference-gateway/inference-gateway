# Model Context Protocol (MCP) Integration Example

This example demonstrates how to deploy the Inference Gateway with Model
Context Protocol (MCP) support in Kubernetes, allowing LLMs to access external
tools and data through multiple MCP servers.

> **⚠️ Important Notice**: The MCP servers included in this example (time,
> search, and filesystem servers) are simplified implementations designed for
> demonstration and testing purposes only. They should **NOT** be used in
> production environments without proper security hardening, input validation,
> authentication, authorization, and error handling. For production deployments,
> implement proper MCP servers with comprehensive security measures and robust
> error handling.

## Table of Contents

- [Model Context Protocol (MCP) Integration Example](#model-context-protocol-mcp-integration-example)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Architecture](#architecture)
  - [Components](#components)
  - [Prerequisites](#prerequisites)
  - [Quick Start](#quick-start)
  - [Taskfile Commands](#taskfile-commands)
    - [Quick Operations](#quick-operations)
    - [Step-by-Step Deployment](#step-by-step-deployment)
    - [Testing and Validation](#testing-and-validation)
    - [Monitoring and Debugging](#monitoring-and-debugging)
    - [Maintenance](#maintenance)
    - [Cleanup](#cleanup)
  - [Configuration](#configuration)
    - [Gateway Settings](#gateway-settings)
    - [MCP Server Configuration](#mcp-server-configuration)
  - [Usage Examples](#usage-examples)
    - [Example 1: Time Tool](#example-1-time-tool)
    - [Example 2: Search Tool](#example-2-search-tool)
    - [Example 3: Filesystem Operations](#example-3-filesystem-operations)
    - [Example 4: Multiple Tools](#example-4-multiple-tools)
    - [Example 5: List Available MCP Tools](#example-5-list-available-mcp-tools)
  - [MCP Inspector](#mcp-inspector)
  - [Operations and Troubleshooting](#operations-and-troubleshooting)
    - [View Logs](#view-logs)
    - [Check Status](#check-status)
    - [Run Tests](#run-tests)
  - [Adding Custom MCP Servers](#adding-custom-mcp-servers)
  - [Resource Cleanup](#resource-cleanup)
    - [Remove the deployment only](#remove-the-deployment-only)
    - [Remove everything including the cluster](#remove-everything-including-the-cluster)
  - [Learn More](#learn-more)

## Overview

The Model Context Protocol is an open standard for implementing function
calling in AI applications. This example shows how to:

1. Deploy the Inference Gateway using its Helm chart with MCP configuration
2. Deploy multiple MCP servers as standalone services
3. Route LLM requests through the MCP middleware
4. Discover and utilize tools provided by different MCP servers
5. Execute tool calls and return results to the LLM

## Architecture

```text
┌─────────────────┐   HTTP    ┌─────────────────────────┐
│                 │  Request  │                         │
│   Client/Agent  │──────────▶│   NGINX Ingress         │
│                 │           │                         │
└─────────────────┘           └─────────────────────────┘
                                           │
                                           ▼
                              ┌─────────────────────────┐
                              │   Inference Gateway     │
                              │   (with MCP Middleware) │
                              └─────────────────────────┘
                                     │           │
                               ┌─────┴─────┐     │
                               ▼           ▼     ▼
                   ┌──────────────────┐  ┌────────────────┐
                   │  LLM Providers   │  │  MCP Protocol  │
                   │  (OpenAI, Groq,  │  │  Communication │
                   │   Anthropic)     │  │                │
                   └──────────────────┘  └────────────────┘
                                                 │
                               ┌─────────────────┼─────────────────┐
                               ▼                 ▼                 ▼
                    ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐
                    │  MCP Time Server │ │ MCP Search Server│ │MCP Filesystem Srv│
                    │  (Port 8081)     │ │  (Port 8082)     │ │  (Port 8083)     │
                    │                  │ │                  │ │                  │
                    │ Tools:           │ │ Tools:           │ │ Tools:           │
                    │ • get_time       │ │ • web_search     │ │ • read_file      │
                    │ • get_timezone   │ │ • find_info      │ │ • write_file     │
                    │                  │ │                  │ │ • list_directory │
                    └──────────────────┘ └──────────────────┘ └──────────────────┘
                                                 │
                                                 ▼
                                    ┌──────────────────────┐
                                    │   MCP Inspector      │
                                    │   (Port 6274)        │
                                    │   Debug & Monitor    │
                                    └──────────────────────┘
```

**Data Flow:**

1. Client sends chat completion request to Inference Gateway
2. Gateway processes request through MCP middleware
3. If tools are needed, Gateway discovers available MCP tools
4. Gateway executes tool calls via MCP protocol to appropriate servers
5. MCP servers return tool results
6. Gateway integrates results and sends response to LLM provider
7. The provider's LLM iterates with the MCP servers as needed (max 10 iterations)
8. Final response returned to client

## Components

- **Inference Gateway**: Main service deployed via official Helm chart with
  MCP configuration
- **MCP Time Server**: Provides time-related tools and utilities _(example
  implementation only)_
- **MCP Search Server**: Provides web search functionality _(mock implementation
  for demo purposes)_
- **MCP Filesystem Server**: Provides file operations (read, write, delete, list
  directories) _(example implementation only)_
- **MCP Inspector**: Web-based debugging tool for exploring and testing MCP servers
- **NGINX Ingress**: Routes external traffic to the gateway

> **Note**: The MCP servers in this example are basic implementations for
> demonstration purposes. They lack production-ready features such as proper
> authentication, authorization, input validation, rate limiting, and
> comprehensive error handling.

## Prerequisites

- [Task](https://taskfile.dev/installation/)
- kubectl configured to access a Kubernetes cluster
- helm 3.x
- ctlptl (for local cluster management)
- A running Kubernetes cluster or the ability to create one

## Quick Start

1. **Deploy the complete setup:**

   ```bash
   task deploy
   ```

   This will:
   - Create a k3d cluster with ingress
   - Set up API keys for inference providers (press enter to skip)
   - Deploy all MCP servers
   - Deploy the Inference Gateway with MCP configuration using Helm

2. **Configure DNS (for local testing):**
   Add to your `/etc/hosts` file:

   ```text
   127.0.0.1 api.inference-gateway.local
   ```

   If using vscode dev container, you can skip this step.

3. **Access the services:**
   - Inference Gateway: <http://api.inference-gateway.local>
   - MCP Inspector: <http://localhost:6274> (after running `task port-forward`)

4. **Test the deployment:**

   ```bash
   task test
   ```

## Taskfile Commands

This example uses a comprehensive Taskfile for automation. Here are the key commands:

### Quick Operations

- `task` - Show available options and quick start guide
- `task quick-start` - Deploy everything and run comprehensive tests (one command deployment)
- `task validate-requirements` - Check if all required tools are installed

### Step-by-Step Deployment

- `task deploy` - Deploy complete MCP setup (infrastructure + gateway + tests)
- `task deploy-infrastructure` - Deploy k3d cluster and NGINX ingress only
- `task deploy-mcp-servers` - Deploy MCP servers only
- `task deploy-inference-gateway` - Deploy Inference Gateway with MCP configuration

### Testing and Validation

- `task test` - Run comprehensive MCP integration tests
- `task test:health` - Test gateway and MCP server health
- `task test:mcp-tools` - Test MCP tools discovery
- `task test:interactive` - Run interactive tests with external access

### Monitoring and Debugging

- `task logs` - Show logs from all pods
- `task logs:gateway` - Show logs from inference gateway only
- `task logs:mcp-servers` - Show logs from MCP servers only
- `task status` - Show status of all resources
- `task status:detailed` - Show detailed status with events and pod descriptions
- `task port-forward` - Forward ports for local access to MCP Inspector

### Maintenance

- `task wait` - Wait for all pods to be ready
- `task restart` - Restart all deployments
- `task setup-hosts` - Help with /etc/hosts configuration for local access
- `task setup-secrets` - Configure API keys for inference providers

### Cleanup

- `task undeploy` - Remove MCP components but keep cluster
- `task clean` - Clean up everything including cluster

Run `task --list` to see all available tasks.

## Configuration

### Gateway Settings

The Inference Gateway is deployed using the official Helm chart with
MCP-specific configuration in `values-mcp.yaml`:

- **MCP_ENABLE**: Enables MCP middleware (`true`)
- **MCP_EXPOSE**: Exposes MCP endpoints (`true`)
- **MCP_SERVERS**: Comma-separated list of MCP server URLs
- **MCP_CLIENT_TIMEOUT**: HTTP client timeout (10s)
- **MCP_DIAL_TIMEOUT**: Connection dial timeout (5s)
- **MCP_TLS_HANDSHAKE_TIMEOUT**: TLS handshake timeout (5s)
- **MCP_RESPONSE_HEADER_TIMEOUT**: Response header timeout (5s)
- **MCP_EXPECT_CONTINUE_TIMEOUT**: Expect continue timeout (2s)
- **MCP_REQUEST_TIMEOUT**: Request timeout for MCP operations (10s)

### MCP Server Configuration

The example deploys three MCP servers:

1. **Time Server** (port 8081): Provides current time in various formats
2. **Search Server** (port 8082): Provides mock web search functionality
3. **Filesystem Server** (port 8083): Provides file operations with persistent
   storage

## Usage Examples

Once deployed, you can interact with the MCP-enabled Inference Gateway:

### Example 1: Time Tool

```bash
curl -X POST http://api.inference-gateway.local/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "groq/meta-llama/llama-4-scout-17b-16e-instruct",
    "messages": [
      {
        "role": "system",
        "content": "You are a helpful assistant with access to various tools."
      },
      {
        "role": "user",
        "content": "What is the current time?"
      }
    ]
  }'
```

### Example 2: Search Tool

```bash
curl -X POST http://api.inference-gateway.local/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "groq/meta-llama/llama-4-scout-17b-16e-instruct",
    "messages": [
      {
        "role": "system",
        "content": "You are a helpful assistant with access to search capabilities."
      },
      {
        "role": "user",
        "content": "Find me information about the Model Context Protocol."
      }
    ]
  }'
```

### Example 3: Filesystem Operations

```bash
curl -X POST http://api.inference-gateway.local/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
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

### Example 4: Multiple Tools

```bash
curl -X POST http://api.inference-gateway.local/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "groq/meta-llama/llama-4-scout-17b-16e-instruct",
    "messages": [
      {
        "role": "system",
        "content": "You are a helpful assistant with access to multiple tools including time, search, and filesystem operations."
      },
      {
        "role": "user",
        "content": "What is the current time, find information about Kubernetes, and create a file with a summary of both."
      }
    ]
  }'
```

### Example 5: List Available MCP Tools

Query the Inference Gateway to see all available tools:

```bash
curl -X GET http://api.inference-gateway.local/v1/mcp/tools
```

## MCP Inspector

The MCP Inspector provides a web interface for debugging and exploring your MCP servers:

1. **Start port forwarding:**

   ```bash
   task port-forward
   ```

2. **Access the inspector:**
   Open <http://localhost:6274> in your browser

3. **Features:**
   - Browse all connected MCP servers and their capabilities
   - Explore available tools with their schemas
   - Execute tool calls directly and see responses
   - Monitor MCP protocol messages

## Operations and Troubleshooting

### View Logs

```bash
# All components
task logs

# Specific component
kubectl logs -f deployment/mcp-time-server -n inference-gateway
kubectl logs -f deployment/inference-gateway -n inference-gateway
```

### Check Status

```bash
task status
```

### Run Tests

```bash
task test
```

## Adding Custom MCP Servers

To add your own MCP server:

1. **Create a new manifest** in `mcp-servers/`:

   ```yaml
   # mcp-servers/my-custom-server.yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: my-custom-mcp-server
     labels:
       app.kubernetes.io/name: my-custom-mcp-server
       app.kubernetes.io/part-of: inference-gateway-mcp
       app.kubernetes.io/component: mcp-server
   spec:
     # ... deployment spec
   ```

2. **Update the MCP servers list** in `values-mcp.yaml`:

   ```yaml
   MCP_SERVERS: 'http://mcp-time-server:8081/mcp,http://mcp-search-server:8082/mcp,http://mcp-filesystem-server:8083/mcp,http://my-custom-mcp-server:8084/mcp'
   ```

3. **Redeploy:**

   ```bash
   task deploy
   ```

## Resource Cleanup

### Remove the deployment only

```bash
task undeploy
```

### Remove everything including the cluster

```bash
task clean
```

## Learn More

- [Model Context Protocol Documentation](https://modelcontextprotocol.io/)
- [Inference Gateway Documentation](https://github.com/inference-gateway/inference-gateway)
- [Inference Gateway Helm Chart](https://github.com/inference-gateway/inference-gateway/tree/main/charts)
- [MCP Specification](https://modelcontextprotocol.io/specification)
- [MCP Examples](https://modelcontextprotocol.io/examples)
