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

- **Gateway**: Inference Gateway deployed via inference gateway operator
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
curl http://api.inference-gateway.local/v1/models
```

4. We told kubernetes where our agents are discoverable, now let's deploy them:

```bash
kubectl apply -f agents/
```

5. Let's view the agents cards:

```bash
curl http://api.inference-gateway.local/v1/agents
```

Or if you would like to get a specific agent:

```bash
curl http://api.inference-gateway.local/v1/agents/<agent_id>
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
