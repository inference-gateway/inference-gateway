# Authentication Example

This example demonstrates Keycloak authentication integration with the Inference Gateway using:

- Keycloak for identity management
- Helm chart for gateway deployment with auth enabled

## Architecture

- **Identity Provider**: Keycloak handles user authentication
- **Gateway**: Inference Gateway deployed via helm chart with auth enabled
- **Integration**: OIDC configuration between gateway and Keycloak

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

2. Deploy Inference Gateway with authentication:

```bash
task deploy-inference-gateway
```

3. Test authentication:

```bash
curl -v -H "Authorization: Bearer $(task fetch-access-token)" https://api.inference-gateway.local
```

## Configuration

### Keycloak Setup

- Edit YAMLs in `keycloak/` directory
- Configure realm and client settings

### Gateway Auth

- Auth settings configured via helm values in Taskfile.yaml
- OIDC issuer URL and client credentials in Secrets

## Cleanup

```bash
task clean
```
