# Authentication with Microsoft Entra ID on Kubernetes

The [Keycloak example](../auth-keycloak/README.md) shows how the operator wires
OIDC into the gateway, and the [docker-compose Entra ID example](../../docker-compose/auth-entra/README.md)
explains the two app registrations and their gotchas. This example reuses both:
complete the compose example's **Setup** section first. It writes the `.env`
this Taskfile reads and provides the token script it calls.

Entra ID is a public HTTPS issuer, so no in-cluster identity provider, CA
bundle or DNS rewrite is needed.

## Prerequisites

- [Task](https://taskfile.dev/installation/), kubectl, helm, ctlptl
- `envsubst` (part of gettext, `brew install gettext` on macOS)
- `../../docker-compose/auth-entra/.env` filled in by that example's setup

## Quick Start

1. Provision the cluster, Gateway API, Envoy Gateway and the operator:

   ```bash
   task deploy-infrastructure
   ```

2. Deploy the gateway. `gateway.yaml` is rendered with `envsubst` from the
   compose example's `.env`:

   ```bash
   task deploy-inference-gateway
   ```

3. In another terminal, forward the Envoy data plane:

   ```bash
   task port-forward-gateway
   ```

4. Fetch a token and call the gateway:

   ```bash
   TOKEN=$(task fetch-access-token)
   curl -i http://localhost:8080/v1/models \
     -H 'Host: api.inference-gateway.local' \
     -H "Authorization: Bearer ${TOKEN}"
   ```

   Without the token the gateway answers `401` with a `WWW-Authenticate: Bearer`
   challenge.

## Cleanup

```bash
task clean
```
