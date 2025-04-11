# Taskfile Usage Guide

This document explains how to use the Taskfile.yml in the hack directory to manage local Kubernetes development.

- [Taskfile Usage Guide](#taskfile-usage-guide)
  - [Prerequisites](#prerequisites)
  - [Available Tasks](#available-tasks)
    - [Cluster Management](#cluster-management)
    - [Helm Testing](#helm-testing)
    - [Deployment](#deployment)
    - [Keycloak Operations](#keycloak-operations)
    - [LLM Operations](#llm-operations)
  - [Typical Workflow](#typical-workflow)
  - [Cleanup](#cleanup)

## Prerequisites

- [Task](https://taskfile.dev/installation/)
- kubectl
- helm
- ctlptl (for cluster management)
- jq (for JSON processing)

## Available Tasks

### Cluster Management

```bash
task deploy-infrastructure
```

Creates local k3d cluster with:

- ingress-nginx (v4.12.1)
- cert-manager (v1.17.1)
- kube-prometheus-stack (70.4.2)
- grafana-operator (v5.17.0)
- Keycloak with PostgreSQL

```bash
task clean
```

Deletes the entire k3d cluster

### Helm Testing

```bash
task test-helm
```

- Updates helm dependencies
- Lints the inference-gateway chart
- Dry-runs template with autoscaling

### Deployment

```bash
task deploy-inference-gateway
```

Deploys inference-gateway with:

- Autoscaling enabled
- Ingress enabled
- Keycloak integration
- Environment configs

### Keycloak Operations

```bash
task import-realm
```

Imports inference-gateway-realm with test user

```bash
task fetch-access-token
```

Gets access token for test user

### LLM Operations

```bash
task generate-completions
```

Interactive prompt for Ollama completions

```bash
task deploy-ollama-deepseek-r1
```

Deploys Ollama with deepseek-r1 model

## Typical Workflow

1. Start cluster: `task deploy-infrastructure`
2. Deploy gateway: `task deploy-inference-gateway`
3. Test auth: `task fetch-access-token`
4. Test LLM: `task generate-completions`

## Cleanup

```bash
task clean
```
