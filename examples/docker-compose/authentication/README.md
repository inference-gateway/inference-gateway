# Authentication Example

This example protects the Inference Gateway with OIDC authentication using
[Keycloak](https://www.keycloak.org/) as the identity provider, entirely via
Docker Compose.

When `AUTH_ENABLE=true`, every request (except `/health`) must carry a valid
`Authorization: Bearer <token>` header. The gateway verifies the token against
the OIDC issuer it discovers at startup.

## Overview

The stack runs three services:

- **keycloak** - identity provider, pre-seeded with a realm, a confidential
  client, and a test user via an imported realm file.
- **keycloak-ready** - a short-lived helper that blocks the gateway from starting
  until Keycloak's discovery endpoint is live. The gateway performs OIDC
  discovery at boot and exits if the issuer is unreachable, so this ordering
  matters.
- **inference-gateway** - the gateway, started with authentication enabled and
  pointed at the Keycloak realm.

| Setting                  | Value                                                 |
| ------------------------ | ----------------------------------------------------- |
| Realm                    | `inference-gateway-realm`                             |
| Client ID                | `inference-gateway-client`                            |
| Client secret            | `very-secret`                                         |
| Test user / password     | `user` / `password`                                   |
| OIDC issuer (in-network) | `http://keycloak:8080/realms/inference-gateway-realm` |
| Gateway                  | `http://localhost:8080`                               |
| Keycloak                 | `http://localhost:8081`                               |

> These are insecure demo values. Never reuse them outside local testing.

## Prerequisites

- Docker
- Docker Compose

## Setup

1. Create a `.env` file from the template (provider keys are optional for the
   auth demo) and make the token helper executable:

   ```bash
   cp .env.example .env
   chmod +x get-token.sh
   ```

2. Start the stack:

   ```bash
   docker compose up -d
   ```

   The gateway waits for Keycloak to finish importing the realm before it
   starts. Follow its logs with:

   ```bash
   docker compose logs -f inference-gateway
   ```

## Testing authentication

1. An unauthenticated request is rejected with `401 Unauthorized`:

   ```bash
   curl -i http://localhost:8080/v1/models
   ```

2. Fetch an access token from Keycloak and retry with it:

   ```bash
   TOKEN="$(./get-token.sh)"

   curl -i http://localhost:8080/v1/models \
     -H "Authorization: Bearer ${TOKEN}"
   ```

   This request returns `200 OK`.

`./get-token.sh` uses the OAuth2 password grant against the demo realm. To do it
by hand:

```bash
curl -s -X POST \
  http://localhost:8081/realms/inference-gateway-realm/protocol/openid-connect/token \
  -d grant_type=password \
  -d client_id=inference-gateway-client \
  -d client_secret=very-secret \
  -d username=user \
  -d password=password \
  -d scope=openid
```

## Running a real chat completion

Every endpoint except `/health` requires a token once auth is enabled. To
exercise a real completion, add a provider key to `.env` (for example
`OPENAI_API_KEY=...`), recreate the gateway with `docker compose up -d`, then:

```bash
curl -s http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"model": "deepseek/deepseek-v4-flash", "messages": [{"role": "user", "content": "Hello!"}]}'
```

## Troubleshooting

The shape of an error tells you which layer rejected the request:

- **`401` with `{"error":"unauthorized"}`** - the gateway rejected _your_ bearer
  token (missing, expired, or invalid). Re-fetch it with
  `TOKEN="$(./get-token.sh)"` and resend. `get-token.sh` now prints a clear error
  and exits non-zero if Keycloak itself rejects the request, so an empty `$TOKEN`
  no longer slips through silently.
- **`400` with `Provider requires an API key`** - no key is configured in `.env`
  for the provider you addressed (for example `DEEPSEEK_API_KEY` for
  `deepseek/...`).
- **A _nested_ error such as `{"error":"{\"error\":\"...\"}"}`** - the gateway
  authenticated you and forwarded the request upstream, but the _provider_
  rejected it (usually an invalid API key, or a model the provider does not
  serve). The inner JSON is the provider's own error; fix the key or model.

## How it works

- Authentication is enabled through the gateway service's `environment` block in
  `docker-compose.yml` (`AUTH_ENABLE`, `AUTH_OIDC_ISSUER`, `AUTH_OIDC_CLIENT_ID`,
  `AUTH_OIDC_CLIENT_SECRET`), so the generated `.env.example` stays untouched.
- Keycloak starts with `--import-realm` and the realm definition in
  `keycloak/realm-export.json`. That realm adds an **audience mapper** so issued
  tokens include `inference-gateway-client` in their `aud` claim - the gateway
  verifies the audience against `AUTH_OIDC_CLIENT_ID`.
- `KC_HOSTNAME` pins Keycloak's public URL to `http://keycloak:8080`, so tokens
  always carry the same issuer the gateway discovered on the compose network,
  even when you request them from the host on port `8081`.

## Keycloak admin console

The realm, confidential client, and test user are created automatically from
`keycloak/realm-export.json`, so the admin console is not required for this
example. Keycloak's admin endpoints are reachable on
[http://localhost:8081](http://localhost:8081) with `admin` / `admin`, though
browser login flows assume the internal `keycloak` hostname and are intended for
the in-network gateway rather than host browsers.

## Cleanup

```bash
docker compose down -v
```

## Additional Resources

- [Configuration Guide](../../../Configurations.md) - all `AUTH_*` settings
- [Kubernetes Authentication Example](../../kubernetes/authentication/README.md) -
  the same idea on Kubernetes with the operator
- [Main Documentation](../../../README.md)
