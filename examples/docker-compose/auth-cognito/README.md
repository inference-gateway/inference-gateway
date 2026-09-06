# Authentication with Amazon Cognito

The same OIDC setup as the [Keycloak example](../authentication/README.md), with
a Cognito user pool as the issuer. Read that example first: it explains how the
gateway verifies tokens, the `AUTH_*` settings and the troubleshooting steps,
none of which change here.

Cognito issues machine-to-machine tokens to an _app client_ that holds a secret.
Those tokens carry no `aud` claim (Cognito binds audiences only for user
logins), so the gateway checks the `client_id` claim against
`AUTH_OIDC_AUDIENCE` instead, which is the check AWS documents for resource
servers. It proves the caller holds that app client's secret.

## Setup

Requires the [AWS CLI](https://aws.amazon.com/cli/) with credentials that can
manage Cognito.

```bash
AWS_REGION=eu-west-1
POOL_ID=$(aws cognito-idp create-user-pool --region "$AWS_REGION" \
  --pool-name inference-gateway --query UserPool.Id --output text)

# Machine clients must be allowed at least one custom scope from a resource server.
aws cognito-idp create-resource-server --region "$AWS_REGION" --user-pool-id "$POOL_ID" \
  --identifier inference-gateway --name "Inference Gateway" \
  --scopes ScopeName=invoke,ScopeDescription="Call the gateway"

CLIENT_ID=$(aws cognito-idp create-user-pool-client --region "$AWS_REGION" --user-pool-id "$POOL_ID" \
  --client-name inference-gateway-client --generate-secret \
  --explicit-auth-flows ALLOW_CLIENT_TOKEN_AUTH \
  --allowed-o-auth-scopes inference-gateway/invoke \
  --query UserPoolClient.ClientId --output text)
CLIENT_SECRET=$(aws cognito-idp describe-user-pool-client --region "$AWS_REGION" --user-pool-id "$POOL_ID" \
  --client-id "$CLIENT_ID" --query UserPoolClient.ClientSecret --output text)

cat > .env <<ENV
AWS_REGION=$AWS_REGION
COGNITO_USER_POOL_ID=$POOL_ID
COGNITO_CLIENT_ID=$CLIENT_ID
COGNITO_CLIENT_SECRET=$CLIENT_SECRET
ENV
```

## Run

```bash
docker compose up -d
TOKEN="$(./get-token.sh)"
curl -i http://localhost:8080/v1/models -H "Authorization: Bearer ${TOKEN}"
```

`get-token.sh` calls `aws cognito-idp get-client-token`, which needs no user
pool domain. The token endpoint's client credentials grant works the same way
once a domain is configured.

## Gotchas

- **No `aud` claim.** Decode the token and you find `client_id`,
  `token_use: access` and `scope`, but no `aud`. That is expected, and why
  `AUTH_OIDC_AUDIENCE` is the app client ID.
- **Several callers.** Each app client gets its own ID; list them all in
  `AUTH_OIDC_AUDIENCE`. To authorize by scope, check `input.identity.scope` in a
  guardrails policy, see the [guardrails example](../guardrails/README.md).
- **`get-client-token` needs `ALLOW_CLIENT_TOKEN_AUTH`.** An app client created
  for user sign-in flows cannot use it; create a separate machine client.
