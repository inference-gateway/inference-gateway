# Authentication Docker Compose Example

This example demonstrates how to set up and use the Inference Gateway UI with Authentication enabled using Docker Compose.

## Prerequisites

- Docker Engine 24.0+
- Docker Compose 2.20+
- Valid OAuth credentials from your provider(s)

## Setup Steps

1. Copy environment templates:

```bash
cp .env.backend.example .env.backend
cp .env.frontend.example .env.frontend
```

2. Configure backend environment (.env.backend):

```ini
AUTH_ENABLED=true
OIDC_ISSUER_URL=http://localhost:8080/realms/app-realm
OIDC_CLIENT_ID=app-client
OIDC_CLIENT_SECRET=very-secret
```

3. Configure frontend environment (.env.frontend):

```ini
AUTH_ENABLED="true"
SECURE_COOKIES=false # Set to true if you are using HTTPS for production
NEXTAUTH_URL=http://localhost:3000
NEXTAUTH_SECRET=very-secret
NEXTAUTH_TRUST_HOST=true

# Keycloak Configuration
KEYCLOAK_ISSUER=http://localhost:8080/realms/app-realm
KEYCLOAK_ID=app-client
KEYCLOAK_SECRET=very-secret
```

1. Start the services:

```bash
docker compose -f docker-compose.yaml up
```

## Accessing the Application

- UI: http://localhost:3000

## Configuration Notes

- Add additional OAuth providers by following the NextAuth.js documentation
- For production deployments:
  - Set `NEXTAUTH_URL` to your public domain
  - Enable HTTPS
  - Use proper secret management
  - Configure session storage

## Troubleshooting

View container logs:

```bash
docker compose -f examples/docker-compose/authentication/docker-compose.yaml logs -f
```
