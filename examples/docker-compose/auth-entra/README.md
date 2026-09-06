# Authentication with Microsoft Entra ID

The same OIDC setup as the [Keycloak example](../authentication/README.md), with
Microsoft Entra ID as the issuer. Read that example first: it explains how the
gateway verifies tokens, the `AUTH_*` settings and the troubleshooting steps,
none of which change here.

Two app registrations are involved: one represents the gateway (the API the
tokens are issued _for_) and one the caller that requests them.

## Setup

Requires the [Azure CLI](https://learn.microsoft.com/cli/azure/) logged in to
the tenant.

```bash
TENANT_ID=$(az account show --query tenantId -o tsv)

# The API. Request v2 tokens, or Entra issues v1 tokens with another issuer.
API_APP_ID=$(az ad app create --display-name inference-gateway-api --query appId -o tsv)
az ad app update --id "$API_APP_ID" --identifier-uris "api://$API_APP_ID" \
  --set api.requestedAccessTokenVersion=2
az ad sp create --id "$API_APP_ID"

# The caller: a confidential client with a secret.
CLIENT_APP_ID=$(az ad app create --display-name inference-gateway-client --query appId -o tsv)
az ad sp create --id "$CLIENT_APP_ID"
CLIENT_SECRET=$(az ad app credential reset --id "$CLIENT_APP_ID" --query password -o tsv)

cat > .env <<ENV
AZURE_TENANT_ID=$TENANT_ID
AZURE_API_APP_ID=$API_APP_ID
AZURE_CLIENT_APP_ID=$CLIENT_APP_ID
AZURE_CLIENT_SECRET=$CLIENT_SECRET
ENV
```

## Run

```bash
docker compose up -d
TOKEN="$(./get-token.sh)"
curl -i http://localhost:8080/v1/models -H "Authorization: Bearer ${TOKEN}"
```

`get-token.sh` requests `api://<API_APP_ID>/.default` with the client
credentials grant. The token's `aud` is the API's client ID, which the compose
file sets as `AUTH_OIDC_AUDIENCE`, and its `iss` is the tenant's `/v2.0` issuer.

## Gotchas

- **v1 tokens.** If `requestedAccessTokenVersion` stays unset, Entra issues v1
  tokens with `iss: https://sts.windows.net/<tenant>/`, which the gateway rejects
  because it discovered the `/v2.0` issuer. Decode the token at
  [jwt.ms](https://jwt.ms) and check the `ver` claim.
- **`AADSTS500011`** means the API has no service principal in the tenant; the
  `az ad sp create` step above creates it.
- **Every app in the tenant can request a token for your API** unless the API's
  service principal has "assignment required" turned on. Authentication proves
  the token came from your tenant for your API, not _which_ app asked; check
  `input.identity.azp` or `roles` in a guardrails policy, see the
  [guardrails example](../guardrails/README.md).
