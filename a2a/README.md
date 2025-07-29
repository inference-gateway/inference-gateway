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
task a2a:schema:download

# Generate Go types from the schema
task generate
```

## Usage

The A2A client enables communication with multiple remote A2A-compliant agents, allowing clients to utilize services provided by these remote agents based on their capabilities and "Agent Cards".

### Manual Agent Configuration

For traditional setups, configure agents manually using URLs:

```bash
export A2A_ENABLE=true
export A2A_AGENTS="http://agent1:8080,http://agent2:8080,http://agent3:8080"
```

### Kubernetes Service Discovery

For Kubernetes deployments, enable automatic service discovery:

```bash
export A2A_ENABLE=true
export A2A_SERVICE_DISCOVERY_ENABLE=true
export A2A_SERVICE_DISCOVERY_NAMESPACE=agents  # Optional: defaults to current namespace
export A2A_SERVICE_DISCOVERY_POLLING_INTERVAL=30s  # Optional: defaults to 30s
```

With service discovery enabled, the gateway will automatically:

- Discover agents deployed as `A2AServer` CRDs in the specified namespace
- Register new agents as they become available
- Remove agents that become unavailable
- Handle agent scaling automatically

### Example Configuration

```yaml
# Gateway deployment with service discovery
apiVersion: inference-gateway.com/v1alpha1
kind: Gateway
metadata:
  name: inference-gateway
spec:
  a2a:
    serviceDiscovery:
      enabled: true
      namespace: 'agents'
      pollingInterval: '30s'
```

## Configuration

A2A configuration is managed through environment variables:

### Basic Configuration

- `A2A_ENABLE`: Enable A2A protocol support (default: false)
- `A2A_EXPOSE`: Expose A2A agents list cards endpoint (default: false)
- `A2A_AGENTS`: Comma-separated list of A2A agent URLs
- `A2A_CLIENT_TIMEOUT`: A2A client timeout (default: 30s)

### Kubernetes Service Discovery

The Inference Gateway supports automatic discovery of A2A agents in Kubernetes environments using the inference-gateway operator CRDs. This eliminates the need to manually configure agent URLs.

- `A2A_SERVICE_DISCOVERY_ENABLE`: Enable Kubernetes service discovery for A2A agents (default: false)
- `A2A_SERVICE_DISCOVERY_NAMESPACE`: Kubernetes namespace to search for A2A services (empty means current namespace)
- `A2A_SERVICE_DISCOVERY_POLLING_INTERVAL`: Interval between service discovery polling requests (default: 30s)

#### How Service Discovery Works

1. **Environment Detection**: The gateway automatically detects if it's running in a Kubernetes environment
2. **CRD Discovery**: Scans the configured namespace for `A2AServer` custom resources
3. **Service Resolution**: Finds corresponding Kubernetes services for each A2A resource
4. **URL Construction**: Builds internal cluster URLs (e.g., `http://agent.namespace.svc.cluster.local:8080`)
5. **Dynamic Updates**: Continuously polls for new agents and removes unavailable ones

#### Service Discovery Requirements

For agents to be discoverable:

- Must be deployed using the inference-gateway operator with `A2AServer` CRDs
- Services must be accessible within the Kubernetes cluster
- Must expose A2A API on standard ports (8080, or ports named "a2a", "agent", or "http")

#### Benefits Over Manual Configuration

- **Zero Configuration**: No need to manually specify agent URLs
- **Dynamic Discovery**: New agents are automatically available without gateway restart
- **Kubernetes Native**: Uses standard Kubernetes service discovery mechanisms
- **Fault Tolerant**: Automatically removes unavailable agents from routing
- **Scalable**: Supports dynamic scaling of agent instances

## Related Links

- [A2A Protocol Specification](https://github.com/google/a2a/blob/main/docs/specification.md)
- [Google Agent Development Kit](https://github.com/google/adk-docs)
- [A2A Examples](https://github.com/google/a2a/tree/main/examples)
- [Curated A2A Agents](https://github.com/inference-gateway/awesome-a2a) - The A2A community is still new, here is a list of ready-to-use A2A agents
