# Authentication Example

This example demonstrates Keycloak (OIDC) authentication with the Inference Gateway using the
[Inference Gateway Operator](https://github.com/inference-gateway/operator).

> **Note:** The Helm chart is deprecated. Authentication is configured declaratively under `spec.auth.oidc`
> in `gateway.yaml`, including `caCertRef` to trust Keycloak's self-signed CA.
>
> **Operator version:** This example requires an operator release that wires the OIDC issuer/clientId env
> vars and supports `spec.auth.oidc.caCertRef` (pinned as `OPERATOR_VERSION` in `Taskfile.yaml`). Earlier
> releases only set `AUTH_ENABLE`, so OIDC will not function.

## Table of Contents

- [Authentication Example](#authentication-example)
  - [Table of Contents](#table-of-contents)
  - [Architecture](#architecture)
  - [Prerequisites](#prerequisites)
  - [Quick Start](#quick-start)
  - [Configuration](#configuration)
  - [Cleanup](#cleanup)

## Architecture

- **Identity Provider**: Keycloak (with a PostgreSQL backend) handles user authentication. It keeps its own
  TLS (self-signed via cert-manager) and is reached at `https://keycloak.inference-gateway.local:8543`.
- **Gateway**: An `inference-gateway` `Gateway` custom resource with `spec.auth.oidc` enabled. The operator
  emits `AUTH_ENABLE`, `AUTH_OIDC_ISSUER`, `AUTH_OIDC_CLIENT_ID`, `AUTH_OIDC_CLIENT_SECRET`, and — via
  `caCertRef` — mounts Keycloak's CA and sets `SSL_CERT_FILE` so OIDC discovery over HTTPS is trusted.
- **In-cluster issuer resolution**: A CoreDNS rewrite points `keycloak.inference-gateway.local` at the
  in-cluster Keycloak Service, so the gateway reaches the same issuer hostname Keycloak stamps into tokens.
- **Routing**: The gateway is exposed over HTTPS via the Kubernetes Gateway API (Envoy Gateway), with the
  certificate issued by cert-manager.

## Prerequisites

- [Task](https://taskfile.dev/installation/)
- kubectl, helm, ctlptl
- curl and jq

## Quick Start

1. Deploy the infrastructure (cluster, Gateway API, cert-manager, Envoy Gateway, PostgreSQL, Keycloak and the
   operator):

   ```bash
   task deploy-infrastructure
   ```

2. Deploy the gateway with authentication (this also publishes Keycloak's CA to the gateway):

   ```bash
   task deploy-inference-gateway
   ```

3. (Optional) Inspect Keycloak. Get the admin password and port-forward the console:

   ```bash
   task keycloak-admin-password
   task port-forward-keycloak   # then open https://localhost:8543 (temp-admin)
   ```

   The `inference-gateway-realm` realm and `inference-gateway-client` are imported automatically (see
   `keycloak/job-import-realm.yaml`).

4. Fetch an access token (keep `task port-forward-keycloak` running in another terminal):

   ```bash
   TOKEN=$(task fetch-access-token)
   ```

5. Call the gateway with the token (run `task port-forward-gateway` in another terminal):

   ```bash
   curl -k --resolve api.inference-gateway.local:8443:127.0.0.1 \
     -H "Authorization: Bearer ${TOKEN}" \
     https://api.inference-gateway.local:8443/v1/models
   ```

   Without a valid token the gateway responds `401 Unauthorized`.

## Configuration

- **Keycloak**: edit the YAMLs in `keycloak/` (instance, realm import, DB secret) and the issuer in cert
  settings.
- **Gateway auth**: configured in `gateway.yaml` under `spec.auth.oidc` (issuer URL, client ID, client
  secret reference, and the CA certificate reference).

## Cleanup

```bash
task clean
```

**Note**: This example uses a self-signed certificate for Keycloak. In production, use a trusted CA
certificate (and `caCertRef` becomes unnecessary).
