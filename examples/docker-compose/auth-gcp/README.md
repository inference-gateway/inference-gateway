# Authentication with Google Cloud service accounts

The same OIDC setup as the [Keycloak example](../auth-keycloak/README.md), with
Google as the issuer. Read that example first: it explains how the gateway
verifies tokens, the `AUTH_*` settings and the troubleshooting steps, none of
which change here.

No identity-provider setup is needed. A Google service account can mint an OIDC
ID token for any audience you name, which is how Google's own services (Cloud
Run, IAP) authenticate service-to-service calls. The catch is that
`https://accounts.google.com` is one issuer for every Google account, so a valid
token only proves "some Google identity". A guardrails policy narrows that to
your service account.

## Setup

Requires the [gcloud CLI](https://cloud.google.com/sdk/docs/install) logged in.

```bash
PROJECT_ID=$(gcloud config get-value project)
SA="inference-gateway-client@${PROJECT_ID}.iam.gserviceaccount.com"

gcloud iam service-accounts create inference-gateway-client
# Let your own account mint tokens as the service account.
gcloud iam service-accounts add-iam-policy-binding "$SA" \
  --member="user:$(gcloud config get-value account)" \
  --role=roles/iam.serviceAccountTokenCreator

cat > .env <<ENV
GCP_SERVICE_ACCOUNT=$SA
GATEWAY_AUDIENCE=https://inference-gateway.example.com
ENV
sed -i.bak "s/YOUR_PROJECT_ID/$PROJECT_ID/" policies/identity.rego && rm policies/identity.rego.bak
```

## Run

```bash
docker compose up -d
TOKEN="$(./get-token.sh)"
curl -i http://localhost:8080/v1/models -H "Authorization: Bearer ${TOKEN}"
```

`get-token.sh` runs `gcloud auth print-identity-token` impersonating the service
account, with `--audiences` set to `GATEWAY_AUDIENCE` and `--include-email` so
the token carries the `email` claim the policy checks.

## Gotchas

- **The audience is whatever you say it is.** `GATEWAY_AUDIENCE` is an arbitrary
  URL; it must be identical in `.env` (read by the gateway) and in the token
  request (read by `get-token.sh`).
- **User credentials cannot set an audience.** `gcloud auth print-identity-token`
  without impersonation returns a token for gcloud's own client ID, which the
  gateway rejects. Impersonate a service account as above, or on GCP attach the
  service account to the workload and use the metadata server.
- **Any Google account can pass authentication.** Remove
  `policies/identity.rego` and every service account in the world that mints a
  token for your audience is accepted. The policy blocks callers whose `email`
  is not in `allowed_emails`; the same check works on `sub` if you prefer.
