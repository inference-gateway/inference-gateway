#!/usr/bin/env bash
# Print a client-credentials access token for the gateway's API registration.
set -euo pipefail
set -a
. "$(dirname "$0")/.env"
set +a

curl -sS -f -X POST "https://login.microsoftonline.com/${AZURE_TENANT_ID}/oauth2/v2.0/token" \
  -d grant_type=client_credentials \
  -d "client_id=${AZURE_CLIENT_APP_ID}" \
  -d "client_secret=${AZURE_CLIENT_SECRET}" \
  -d "scope=api://${AZURE_API_APP_ID}/.default" |
  sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p'
