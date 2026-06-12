#!/usr/bin/env bash
#
# Fetch an access token from the example Keycloak realm using the OAuth2
# Resource Owner Password Credentials (direct access) grant, and print the raw
# access token to stdout.
#
# Usage:
#   ./get-token.sh
#   TOKEN="$(./get-token.sh)"
#
# Every value can be overridden via environment variables, e.g.
#   KEYCLOAK_URL=http://localhost:8081 ./get-token.sh
set -euo pipefail

KEYCLOAK_URL="${KEYCLOAK_URL:-http://localhost:8081}"
REALM="${REALM:-inference-gateway-realm}"
CLIENT_ID="${CLIENT_ID:-inference-gateway-client}"
CLIENT_SECRET="${CLIENT_SECRET:-very-secret}"
USERNAME="${USERNAME:-user}"
PASSWORD="${PASSWORD:-password}"

response="$(curl -sS -w $'\n%{http_code}' -X POST \
  "${KEYCLOAK_URL}/realms/${REALM}/protocol/openid-connect/token" \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  -d 'grant_type=password' \
  -d "client_id=${CLIENT_ID}" \
  -d "client_secret=${CLIENT_SECRET}" \
  -d "username=${USERNAME}" \
  -d "password=${PASSWORD}" \
  -d 'scope=openid')" || {
  echo "Error: could not reach Keycloak at ${KEYCLOAK_URL}. Is the stack up? (docker compose up -d)" >&2
  exit 1
}

http_code="${response##*$'\n'}"
body="${response%$'\n'*}"

if [ "${http_code}" != "200" ]; then
  echo "Error: token request failed (HTTP ${http_code}):" >&2
  echo "${body}" >&2
  exit 1
fi

access_token="$(printf '%s' "${body}" | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')"

if [ -z "${access_token}" ]; then
  echo "Error: no access_token in Keycloak response:" >&2
  echo "${body}" >&2
  exit 1
fi

printf '%s\n' "${access_token}"
