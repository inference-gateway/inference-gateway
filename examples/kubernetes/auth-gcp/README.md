# Authentication with Google Cloud service accounts on Kubernetes

The [Keycloak example](../auth-keycloak/README.md) shows how the operator wires
OIDC into the gateway, and the [docker-compose Google example](../../docker-compose/auth-gcp/README.md)
explains the service account, the audience and why a guardrails policy is
required with Google's shared issuer. This example reuses both: complete the
compose example's **Setup** section first. It writes the `.env` this Taskfile
reads, provides the token script it calls, and holds the policy that is shipped
here as a ConfigMap with your service account filled in.

## Prerequisites

- [Task](https://taskfile.dev/installation/), kubectl, helm, ctlptl
- `envsubst` (part of gettext, `brew install gettext` on macOS)
- `../../docker-compose/auth-gcp/.env` filled in by that example's setup

## Quick Start

1. Provision the cluster, Gateway API, Envoy Gateway and the operator:

   ```bash
   task deploy-infrastructure
   ```

2. Deploy the gateway. The task creates the policy ConfigMap from the compose
   example's `identity.rego` and renders `gateway.yaml` with `envsubst`:

   ```bash
   task deploy-inference-gateway
   ```

3. In another terminal, forward the Envoy data plane:

   ```bash
   task port-forward-gateway
   ```

4. Mint a token and call the gateway:

   ```bash
   TOKEN=$(task fetch-access-token)
   curl -i http://localhost:8080/v1/models \
     -H 'Host: api.inference-gateway.local' \
     -H "Authorization: Bearer ${TOKEN}"
   ```

   Without the token the gateway answers `401` with a `WWW-Authenticate: Bearer`
   challenge. A token minted by any other Google service account for the same
   audience passes authentication and is then blocked by the policy.

## Cleanup

```bash
task clean
```
