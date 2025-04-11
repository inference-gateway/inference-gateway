# TLS Example

## Table of Contents

- [TLS Example](#tls-example)
  - [Table of Contents](#table-of-contents)
  - [Architecture](#architecture)
  - [Prerequisites](#prerequisites)
  - [Quick Start](#quick-start)
  - [Configuration](#configuration)
    - [Certificate Setup](#certificate-setup)
    - [Gateway TLS](#gateway-tls)
  - [Cleanup](#cleanup)

This example demonstrates secure TLS communication with the Inference Gateway using:

- Cert-manager for certificate management
- Helm chart for gateway deployment with TLS enabled
- Automatic certificate issuance

## Architecture

- **Certificate Management**: Cert-manager handles TLS certificates
- **Gateway**: Inference Gateway deployed via helm chart with TLS
- **Ingress**: Configured with automatic HTTPS

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

2. Deploy Inference Gateway with TLS:

```bash
task deploy-inference-gateway
```

3. Test secure endpoint:

```bash
curl -k https://api.inference-gateway.local/v1/models
```

## Configuration

### Certificate Setup

- Edit YAMLs in `cert-manager/` directory
- Configure issuer and certificate settings

### Gateway TLS

- TLS settings configured via helm values in Taskfile.yaml
- Automatic certificate provisioning

## Cleanup

```bash
task clean
```
