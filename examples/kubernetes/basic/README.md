# Basic Deployment Example

This example demonstrates the simplest deployment of the Inference Gateway using Helm.

## Table of Contents

- [Basic Deployment Example](#basic-deployment-example)
  - [Table of Contents](#table-of-contents)
  - [Architecture](#architecture)
  - [Prerequisites](#prerequisites)
  - [Quick Start](#quick-start)
  - [Configuration](#configuration)
    - [Gateway Settings](#gateway-settings)
  - [Cleanup](#cleanup)

## Architecture

- **Gateway**: Inference Gateway deployed via inference gateway operator
- **Ingress**: Basic ingress configuration

## Prerequisites

- [Task](https://taskfile.dev/installation/)
- kubectl
- helm
- ctlptl (for cluster management)

## Quick Start

1. Deploy infrastructure:

```bash
task deploy-infrastructure
```

1. Deploy Inference Gateway:

```bash
task deploy-inference-gateway
```

1. Test the gateway:

```bash
curl http://api.inference-gateway.local/v1/models
```

## Configuration

### Gateway Settings

- Configured via helm values in Taskfile.yaml
- No additional components required

## Cleanup

```bash
task clean
```
