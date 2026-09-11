#!/usr/bin/env bash
# Print an ID token for the service account in .env, minted for the gateway's
# audience and carrying the email claim the guardrails policy checks.
set -euo pipefail
set -a
. "$(dirname "$0")/.env"
set +a

gcloud auth print-identity-token \
  --impersonate-service-account="${GCP_SERVICE_ACCOUNT}" \
  --audiences="${GATEWAY_AUDIENCE}" \
  --include-email
