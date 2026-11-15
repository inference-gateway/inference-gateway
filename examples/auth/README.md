# Authentication

The gateway behind OIDC: an identity provider issues the token, the gateway
verifies its issuer and audience before proxying the request. One folder per
identity provider; every provider has a `docker-compose/` and a `kubernetes/`
flavour. The cloud-provider kubernetes flavours reuse the `.env` and
`get-token.sh` of their docker-compose sibling, so run that one first.

The [guardrails example](../guardrails/README.md) layers authorization on top of
the same setup.

## Identity providers

- [Keycloak](keycloak/docker-compose/README.md) - a self-hosted Keycloak with an
  `inference-gateway-realm` ([docker-compose](keycloak/docker-compose/README.md),
  [kubernetes](keycloak/kubernetes/README.md))
- [Microsoft Entra ID](entra/docker-compose/README.md)
  ([docker-compose](entra/docker-compose/README.md),
  [kubernetes](entra/kubernetes/README.md))
- [Google Cloud service accounts](gcp/docker-compose/README.md)
  ([docker-compose](gcp/docker-compose/README.md),
  [kubernetes](gcp/kubernetes/README.md))
- [Amazon Cognito](cognito/docker-compose/README.md)
  ([docker-compose](cognito/docker-compose/README.md),
  [kubernetes](cognito/kubernetes/README.md))
