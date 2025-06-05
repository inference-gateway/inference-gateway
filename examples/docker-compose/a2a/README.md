# Agent-to-Agent (A2A) Protocol Integration Example

This example demonstrates integrating the Agent-to-Agent (A2A) protocol with Inference Gateway, enabling LLMs to discover and utilize capabilities from remote A2A-compliant agents through Google's A2A specification.

## Features

- **✨ Agent Discovery**: Automatic discovery of agent capabilities through `.well-known/agent.json` endpoints
- **🔧 Tool Integration**: A2A agent skills are seamlessly converted to chat completion tools
- **🤝 Multi-Agent Coordination**: LLMs can coordinate with multiple specialized agents
- **📡 JSON-RPC 2.0**: Standards-based communication using JSON-RPC 2.0 protocol
- **🛡️ Health Monitoring**: Built-in health checks for all agents
- **🐳 Docker Compose**: Complete containerized setup with networking

## Table of Contents

- [Quick Start](#quick-start)
- [Components](#components)
- [Available Agents](#available-agents)
- [Testing the Setup](#testing-the-setup)
- [How It Works](#how-it-works)
- [Adding Your Own A2A Agents](#adding-your-own-a2a-agents)
- [Learn More](#learn-more)

## Quick Start

### Prerequisites

- Docker and Docker Compose
- DeepSeek API key (or any other supported provider)

### Setup

```bash
export DEEPSEEK_API_KEY=your_deepseek_api_key
docker-compose up
```

### Test and Troubleshoot

The A2A agents will be available at the following endpoints:

- **Hello World Agent**: `http://localhost:8081`
- **Calculator Agent**: `http://localhost:8082`
- **Weather Agent**: `http://localhost:8083`

## Components

- **Inference Gateway**: Main service that proxies requests to LLM providers and coordinates with A2A agents
- **Hello World Agent**: Simple A2A agent demonstrating basic message handling
- **Calculator Agent**: Mathematical computation agent with arithmetic operations
- **Weather Agent**: Weather information agent providing current weather data

## Usage

Once the services are running, you can make requests to the Inference Gateway. The gateway will automatically discover available A2A agent skills and present them as tools to the LLM.

### Example 1: Hello World Agent

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
  "model": "deepseek/deepseek-chat",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Say hello using the hello world agent."
    }
  ]
}'
```

### Example 2: Calculator Agent

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
  "model": "deepseek/deepseek-chat",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant with access to mathematical tools."
    },
    {
      "role": "user",
      "content": "Calculate the result of 15 + 27 * 3."
    }
  ]
}'
```

### Example 3: Weather Agent

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
  "model": "deepseek/deepseek-chat",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant with access to weather information."
    },
    {
      "role": "user",
      "content": "What is the current weather in New York?"
    }
  ]
}'
```

### Example 4: Multi-Agent Coordination

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
  "model": "deepseek/deepseek-chat",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant with access to multiple specialized agents."
    },
    {
      "role": "user",
      "content": "Say hello, then calculate 25 * 4, and finally tell me the weather in London."
    }
  ]
}'
```

### Example 5: List Available A2A Agent Skills

```bash
curl -X GET http://localhost:8080/a2a/agents
```

Response will show available agents and their capabilities:

```json
{
  "agents": [
    {
      "url": "http://helloworld-agent:8081",
      "capabilities": {
        "skills": [
          {
            "id": "hello_world",
            "name": "Hello World",
            "description": "Returns a hello world greeting"
          }
        ]
      }
    },
    {
      "url": "http://calculator-agent:8082",
      "capabilities": {
        "skills": [
          {
            "id": "add",
            "name": "Add Numbers",
            "description": "Add two numbers together"
          },
          {
            "id": "multiply",
            "name": "Multiply Numbers",
            "description": "Multiply two numbers together"
          }
        ]
      }
    }
  ]
}
```

## How It Works

When you send a request to the Inference Gateway with A2A enabled, it will:

1. **Agent Discovery**: Connect to all configured A2A agents and fetch their Agent Cards
2. **Skill Registration**: Convert agent skills into OpenAI-compatible function tools
3. **Tool Injection**: Add available tools to the LLM request
4. **Execution Coordination**: When the LLM calls a tool, route the request to the appropriate agent
5. **Response Integration**: Collect agent responses and integrate them into the conversation

The A2A middleware handles all the protocol-specific communication, including:

- Agent Card parsing
- JSON-RPC message/send method calls
- Error handling and retries
- Response formatting

## Available Agents

### Hello World Agent

- **Endpoint**: `http://helloworld-agent:8081`
- **Skills**:
  - `hello_world`: Returns a simple greeting message

### Calculator Agent

- **Endpoint**: `http://calculator-agent:8082`
- **Skills**:
  - `add`: Add two numbers together
  - `subtract`: Subtract one number from another
  - `multiply`: Multiply two numbers together
  - `divide`: Divide one number by another

### Weather Agent

- **Endpoint**: `http://weather-agent:8083`
- **Skills**:
  - `get_weather`: Get current weather information for a location
  - `get_forecast`: Get weather forecast for a location

## Adding Your Own A2A Agents

**It's incredibly easy to add more A2A agents!** Simply follow these steps:

### Quick Setup

1. **Add your agent URL** to the `A2A_AGENTS` environment variable:

   ```bash
   A2A_AGENTS=http://helloworld-agent:8081,http://calculator-agent:8082,http://your-new-agent:3004
   ```

2. **Include your agent service** in the docker-compose.yml file (if running in Docker)

3. **Restart the services** - that's it! Your agent skills will automatically be available.

### Requirements for Your A2A Agent

- Implements the [A2A specification](https://github.com/google/A2A/blob/main/docs/specification.md)
- Provides an Agent Card at the root endpoint describing capabilities
- Supports the `message/send` JSON-RPC method
- Responds with proper A2A response formats

### Configuration Options

Environment variables you can configure:

- `A2A_ENABLE`: Set to "true" to enable A2A middleware
- `A2A_EXPOSE`: Set to "true" to expose A2A endpoints for debugging
- `A2A_AGENTS`: Comma-separated list of A2A agent URLs
- `A2A_CLIENT_TIMEOUT`: Timeout for A2A client requests (default: 30s)

### Current Example Agents

This example includes three pre-configured agents:

- **Hello World Agent**: `http://helloworld-agent:8081` - Basic greeting functionality
- **Calculator Agent**: `http://calculator-agent:8082` - Mathematical operations
- **Weather Agent**: `http://weather-agent:8083` - Weather information services

## Learn More

- [A2A Protocol Specification](https://github.com/google/A2A/blob/main/docs/specification.md)
- [A2A Python SDK](https://github.com/google/a2a-python)
- [Google Agent Development Kit](https://github.com/google/adk-docs)
- [Inference Gateway Documentation](https://github.com/inference-gateway/inference-gateway)
