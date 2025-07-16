# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The Inference Gateway is a multi-provider LLM proxy server written in Go that supports OpenAI-compatible APIs. It provides a unified interface for accessing various language model providers (OpenAI, Anthropic, Groq, Ollama, etc.) and includes advanced features like Model Context Protocol (MCP) and Agent-to-Agent (A2A) integrations.

## Common Commands

### Essential Development Commands
- `task build` - Build the gateway binary (`go build -o bin/inference-gateway cmd/gateway/main.go`)
- `task run` - Run the gateway locally (`go run cmd/gateway/main.go`)
- `task test` - Run all tests (`go test -v ./...`)
- `task lint` - Run Go linting (`golangci-lint run`)

### Schema and Code Generation
- `task generate` - Generate all code from OpenAPI spec and schemas
- `task mcp-schema-download` - Download latest MCP schema when working on MCP features
- `task a2a-schema-download` - Download latest A2A schema when working on A2A features

### Running Single Tests
- `go test -v ./tests/api_routes_test.go` - Run specific test file
- `go test -v -run TestChatCompletions ./tests/` - Run specific test function

### Container and Deployment
- `task build:container` - Build Docker container
- `task package` - Package the gateway (builds container)

## Architecture Overview

### Core Components

**Entry Point**: `cmd/gateway/main.go` - Main application entry point that initializes all components and starts the HTTP server

**Configuration**: `config/` - Environment-based configuration using `go-envconfig` with support for all providers and features

**API Layer**: `api/routes.go` - HTTP handlers implementing OpenAI-compatible endpoints (`/v1/chat/completions`, `/v1/models`, etc.)

**Provider System**: `providers/` - Abstracted provider implementations with a registry pattern supporting multiple LLM providers

**Middleware Stack**: `api/middlewares/` - Request processing pipeline including:
- MCP middleware for tool injection and execution
- A2A middleware for agent skill integration  
- Authentication (OIDC), logging, and telemetry

### Key Architectural Patterns

**Provider Registry Pattern**: All LLM providers implement the `IProvider` interface and are registered in a central registry for consistent access

**Middleware Pipeline**: Requests flow through configurable middleware that can inject tools, handle authentication, and process responses

**Interface-Based Design**: Most components are interface-based (`Router`, `ProviderRegistry`, `MCPClientInterface`, `A2AClientInterface`) enabling easy mocking and testing

## MCP (Model Context Protocol) Integration

MCP enables automatic tool discovery and injection. When enabled:
- Tools are automatically discovered from configured MCP servers
- The MCP middleware injects available tools into chat completion requests
- Tool calls are executed automatically and results fed back to the LLM

**Key files**: `mcp/agent.go`, `mcp/client.go`, `api/middlewares/mcp.go`

## A2A (Agent-to-Agent) Integration

A2A enables connecting to external agents and exposing their skills as tools:
- Agent skills are automatically discovered and converted to OpenAI tool format
- Skills can be executed by LLMs through the standard tool calling mechanism
- Supports agent card retrieval and skill execution

**Key files**: `a2a/agent.go`, `a2a/client.go`, `api/middlewares/a2a.go`

## Development Best Practices

### Code Style
- Use table-driven testing for comprehensive test coverage
- Prefer early returns to avoid deep nesting
- Use switch statements over if-else chains for multiple conditions
- Code to interfaces for easier mocking and testing
- Use lowercase log messages for consistency

### Testing Patterns
- Each test case should have isolated mock dependencies
- Use `go generate` to generate mocks from interfaces
- Tests are located in `tests/` directory with comprehensive coverage

### Schema Management
- OpenAPI specification in `openapi.yaml` defines all configuration
- Run `task generate` after modifying schemas to update generated code
- MCP and A2A schemas are downloaded and converted to Go types

## Configuration System

Configuration is environment-based using structured config types:
- `Config` struct in `config/config.go` defines all settings
- Provider-specific config in `providers/` package
- Environment variables follow structured naming (e.g., `MCP_ENABLE`, `A2A_AGENTS`)

## Important Notes

- Always run `task lint` before committing code
- Always run `task build` and `task test` to verify changes
- When working on MCP: run `task mcp-schema-download` and `task generate` to update types
- When working on A2A: run `task a2a-schema-download` and `task generate` to update types
- The gateway serves on port 8080 by default with metrics on port 9464

## Related Repositories

### Core Inference Gateway
- **[Main Repository](https://github.com/inference-gateway)** - The main inference gateway org
- **[Documentation](https://github.com/inference-gateway/docs)** - Official documentation and guides
- **[UI](https://github.com/inference-gateway/ui)** - Web interface for the inference gateway

### SDKs & Client Libraries
- **[Go SDK](https://github.com/inference-gateway/go-sdk)** - Go client library
- **[Rust SDK](https://github.com/inference-gateway/rust-sdk)** - Rust client library  
- **[TypeScript SDK](https://github.com/inference-gateway/typescript-sdk)** - TypeScript/JavaScript client library
- **[Python SDK](https://github.com/inference-gateway/python-sdk)** - Python client library

### A2A (Agent-to-Agent) Ecosystem
- **[Awesome A2A](https://github.com/inference-gateway/awesome-a2a)** - Curated list of A2A-compatible agents
- **[Google Calendar Agent](https://github.com/inference-gateway/google-calendar-agent)** - Agent for Google Calendar integration

### Internal Tools
- **[Internal Tools](https://github.com/inference-gateway/tools)** - Collection of internal tools and utilities
