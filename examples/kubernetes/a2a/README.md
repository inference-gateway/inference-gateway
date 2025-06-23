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

4. We told kubernetes where are agents discoverable, now let's deploy them:

```bash
kubectl apply -f agents/
```

### Gateway Settings

- Configured via helm values in Taskfile.yaml
- No additional components required

## Cleanup

```bash
task clean
```

## References

- [Inference Gateway Documentation](https://github.com/inference-gateway/docs)
- [Awesome A2A Agents](https://github.com/inference-gateway/awesome-a2a)
- [Kubernetes Documentation](https://kubernetes.io/docs/)
