#!/usr/bin/env bash
# Print a machine-to-machine access token for the app client in .env.
set -euo pipefail
set -a
. "$(dirname "$0")/.env"
set +a

aws cognito-idp get-client-token \
  --region "${AWS_REGION}" \
  --client-id "${COGNITO_CLIENT_ID}" \
  --secret "${COGNITO_CLIENT_SECRET}" \
  --query ClientAuthenticationResult.AccessToken \
  --output text
