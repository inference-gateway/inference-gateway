# Inference Gateway Configuration

## General Settings

| Environment Variable | Default Value | Description |
|---------------------|---------------|-------------|
| APPLICATION_NAME | `inference-gateway` | The name of the application |
| ENABLE_AUTH | `false` | Enable authentication |
| ENABLE_TELEMETRY | `false` | Enable telemetry |
| ENVIRONMENT | `production` | The environment |

## OIDC Settings

| Environment Variable | Default Value | Description |
|---------------------|---------------|-------------|
| OIDC_CLIENT_ID | `inference-gateway-client` | OIDC client ID |
| OIDC_CLIENT_SECRET | `""` | OIDC client secret |
| OIDC_ISSUER_URL | `http://keycloak:8080/realms/inference-gateway-realm` | OIDC issuer URL |

## Server Settings

| Environment Variable | Default Value | Description |
|---------------------|---------------|-------------|
| SERVER_HOST | `0.0.0.0` | Server host |
| SERVER_IDLE_TIMEOUT | `120s` | Idle timeout |
| SERVER_PORT | `8080` | Server port |
| SERVER_READ_TIMEOUT | `30s` | Read timeout |
| SERVER_TLS_CERT_PATH | `""` | TLS certificate path |
| SERVER_TLS_KEY_PATH | `""` | TLS key path |
| SERVER_WRITE_TIMEOUT | `30s` | Write timeout |

## API URLs and keys

| Environment Variable | Default Value | Description |
|---------------------|---------------|-------------|
| ANTHROPIC_API_URL | `https://api.anthropic.com` | The URL for Anthropic API |
| ANTHROPIC_API_KEY | `""` | The Access token for Anthropic API |
| CLOUDFLARE_API_URL | `https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}` | The URL for Cloudflare API |
| CLOUDFLARE_API_KEY | `""` | The Access token for Cloudflare API |
| COHERE_API_URL | `https://api.cohere.com` | The URL for Cohere API |
| COHERE_API_KEY | `""` | The Access token for Cohere API |
| GOOGLE_API_URL | `https://generativelanguage.googleapis.com` | The URL for Google API |
| GOOGLE_API_KEY | `""` | The Access token for Google API |
| GROQ_API_URL | `https://api.groq.com` | The URL for Groq API |
| GROQ_API_KEY | `""` | The Access token for Groq API |
| OLLAMA_API_URL | `http://ollama:8080` | The URL for Ollama API |
| OPENAI_API_URL | `https://api.openai.com` | The URL for Openai API |
| OPENAI_API_KEY | `""` | The Access token for Openai API |

