# Authentication Example

## Table of Contents

- [Authentication Example](#authentication-example)
  - [Table of Contents](#table-of-contents)
  - [Architecture](#architecture)
  - [Prerequisites](#prerequisites)
  - [Quick Start](#quick-start)
  - [Configuration](#configuration)
    - [Keycloak Setup](#keycloak-setup)
    - [Gateway Auth](#gateway-auth)
  - [Cleanup](#cleanup)

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

1. Deploy Inference Gateway with authentication:

```bash
task deploy-inference-gateway
```

1. Review the Keycloak UI:

```bash
task keycloak-admin-password
```

- Access Keycloak at `https://keycloak.inference-gateway.local`

- Login with `temp-admin` and the fetched password as credentials

1. Create a Realm and Client in Keycloak, no need to do it via ClickOps,
   instead review the YAML file `keycloak/job-import-realm.yaml` it was already
   deployed when you ran `deploy-infrastructure`.

2. Test authentication:

   ```bash
   curl -k -v -H "Authorization: Bearer $(task fetch-access-token)" \
     https://api.inference-gateway.local/v1/models
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

**Note**: The example uses a self-signed certificate for Keycloak. In
production, use a trusted CA certificate.
