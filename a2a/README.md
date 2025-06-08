# Agent-to-Agent (A2A) Protocol

This package implements Google's Agent-to-Agent (A2A) protocol integration for the Inference Gateway. The A2A protocol is an open standard designed to facilitate interoperability between AI agents, allowing them to discover each other's capabilities, exchange information securely, and coordinate actions.

## Features

- **Agent Discovery**: Agents can publish an Agent Card (JSON metadata) describing their capabilities, endpoints, and authentication requirements
- **Task Management**: Standardized methods for initiating, updating, and completing tasks between agents
- **Streaming & Notifications**: Support for Server-Sent Events (SSE) and push notifications for real-time updates
- **Modality Agnostic**: Supports various content types, including text, files, forms, and streams

## Schema Management

The A2A types are automatically generated from Google's official A2A schema repository:

```bash
# Download the latest A2A schema
task a2a-schema-download

# Generate Go types from the schema
task generate
```

## Usage

The A2A client will be integrated with the Inference Gateway to enable communication with multiple remote A2A-compliant agents, allowing clients to utilize services provided by these remote agents based on their capabilities and "Agent Cards".

## Configuration

A2A configuration is managed through environment variables:

- `A2A_ENABLE`: Enable A2A protocol support (default: false)
- `A2A_EXPOSE`: Expose A2A agents list cards endpoint (default: false)
- `A2A_AGENTS`: Comma-separated list of A2A agent URLs
- `A2A_CLIENT_TIMEOUT`: A2A client timeout (default: 30s)

## Related Links

- [A2A Protocol Specification](https://github.com/google/a2a/blob/main/docs/specification.md)
- [Google Agent Development Kit](https://github.com/google/adk-docs)
- [A2A Examples](https://github.com/google/a2a/tree/main/examples)
- [Curated A2A Agents](https://github.com/inference-gateway/awesome-a2a) - The A2A community is still new, here is a list of ready-to-use A2A agents
