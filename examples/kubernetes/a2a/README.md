# A2A Kubernetes Example

This directory contains an example setup for running A2A (Agent-to-Agent) agents with the Inference Gateway on Kubernetes.

## Overview

The A2A example demonstrates how to deploy and connect multiple agents using Kubernetes manifests. It is designed to help you understand how to orchestrate agent services, manage their configuration, and enable secure, scalable communication between agents in a Kubernetes environment.

## Contents

- **Deployment Manifests:** Example YAML files for deploying agents and related services.
- **Configuration Files:** Sample configuration for agents and gateway integration.
- **Instructions:** Steps to deploy, test, and extend the A2A setup.

## Prerequisites

- Kubernetes cluster (local or cloud)
- `kubectl` configured for your cluster
- [Inference Gateway](https://github.com/inference-gateway) deployed or accessible
- [Helm](https://helm.sh/) for easier deployment

## Architecture

- **Gateway**: Inference Gateway deployed via inference gateway operator with A2A service discovery enabled
- **Agents**: A2A agents deployed in the `agents` namespace and automatically discovered by the gateway
- **Service Discovery**: Kubernetes-native agent discovery using label selectors instead of manual URL configuration
- **Ingress**: Basic ingress configuration

## Quick Start

1. Deploy infrastructure:

```bash
task deploy-infrastructure
```

2. Deploy Inference Gateway:

```bash
task deploy-inference-gateway
```

3. Test the gateway:

```bash
curl -L -k https://api.inference-gateway.local/v1/models
```

4. Deploy the A2A agents (they will be automatically discovered by the gateway):

```bash
kubectl apply -f agents/
```

5. Let's view the agents cards:

```bash
 curl -L -k https://api.inference-gateway.local/v1/a2a/agents
```

Or if you would like to get a specific agent:

```bash
curl -L -k https://api.inference-gateway.local/v1/a2a/agents/<agent_id>
```

6. To test the agents, we can send a general question that one of those agents can answer:

```bash
curl -X POST http://api.inference-gateway.local/chat/completions \
-H "Content-Type: application/json" \
-d '{
    "model": "deepseek/deepseek-chat",
    "messages": [
        {
            "role": "user",
            "content": "Can you list my meetings for today?"
        }
    ]
}'
```

7. If you view the logs of the gateway, you should see that the Gateway (as an A2A-client agent) queried the relevant agent for their card using `a2a_query_agent_card` tool call.
8. Then it delegated the task to the correct agent using `a2a_submit_task_to_agent` tool call.
9. The agent processed the request (with possible few iterations and internal tool calls) and returned the response to the Gateway, which then returned it to the user.

** If you send it as a streaming request with the headers `Accept: text/event-stream` and `Content-Type: application/json`, you will see the response in a streaming fashion. **

## A2A Service Discovery

This example demonstrates the new **Kubernetes Service Discovery** feature for A2A agents. Instead of manually configuring agent URLs, the gateway automatically discovers agents using Kubernetes label selectors.

### How it works

1. **Agent Labeling**: A2AServer resources are labeled with `inference-gateway.com/a2a-agent=true`
2. **Service Discovery**: The gateway periodically scans the `agents` namespace for services with this label
3. **Automatic Registration**: Discovered agents are automatically registered and made available for delegation
4. **Dynamic Updates**: New agents are discovered and unavailable agents are removed automatically

### Configuration

The gateway uses these service discovery settings (configured in `gateway.yaml`):

```yaml
a2a:
  serviceDiscovery:
    enabled: true
    namespace: 'agents' # Namespace to scan for agents
    labelSelector: 'inference-gateway.com/a2a-agent=true' # Label selector for agent services
    pollingInterval: '30s' # How often to check for new agents
```

### Agent Requirements

For agents to be discoverable, they must:

- Be deployed in the configured namespace (`agents` in this example)
- Have services labeled with the discovery label (`inference-gateway.com/a2a-agent=true`)
- Expose their A2A API on a standard port (8080, or annotated with `inference-gateway.com/a2a-port`)

### Benefits

- **Zero Configuration**: No need to manually specify agent URLs
- **Dynamic Discovery**: New agents are automatically available
- **Kubernetes Native**: Uses standard Kubernetes service discovery mechanisms
- **Scalable**: Supports dynamic scaling of agent instances

## Cleanup

```bash
task clean
```

## References

- [Inference Gateway Documentation](https://docs.inference-gateway.com/a2a)
- [Awesome A2A Agents](https://github.com/inference-gateway/awesome-a2a)
- [Google Calendar Agent](https://github.com/inference-gateway/google-calendar-agent)

## Notes

Bare in mind that a2a is still relatively new and experimental feature. Some features may not work as expected, and we're actively working on improving the experience.
