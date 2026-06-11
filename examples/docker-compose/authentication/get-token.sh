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

curl -sf -X POST \
  "${KEYCLOAK_URL}/realms/${REALM}/protocol/openid-connect/token" \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  -d 'grant_type=password' \
  -d "client_id=${CLIENT_ID}" \
  -d "client_secret=${CLIENT_SECRET}" \
  -d "username=${USERNAME}" \
  -d "password=${PASSWORD}" \
  -d 'scope=openid' |
  sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p'
