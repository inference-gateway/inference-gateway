## Configurations

### General settings
| Environment Variable | Default Value | Description |
|---------------------|---------------|-------------|
| APPLICATION_NAME | `inference-gateway` | The name of the application |
| ENVIRONMENT | `production` | The environment |
| ENABLE_TELEMETRY | `false` | Enable telemetry |
| ENABLE_AUTH | `false` | Enable authentication |


### OpenID Connect
| Environment Variable | Default Value | Description |
|---------------------|---------------|-------------|
| OIDC_ISSUER_URL | `http://keycloak:8080/realms/inference-gateway-realm` | OIDC issuer URL |
| OIDC_CLIENT_ID | `inference-gateway-client` | OIDC client ID |
| OIDC_CLIENT_SECRET | `""` | OIDC client secret |


### Server settings
| Environment Variable | Default Value | Description |
|---------------------|---------------|-------------|
| SERVER_HOST | `0.0.0.0` | Server host |
| SERVER_PORT | `8080` | Server port |
| SERVER_READ_TIMEOUT | `30s` | Read timeout |
| SERVER_WRITE_TIMEOUT | `30s` | Write timeout |
| SERVER_IDLE_TIMEOUT | `120s` | Idle timeout |
| SERVER_TLS_CERT_PATH | `""` | TLS certificate path |
| SERVER_TLS_KEY_PATH | `""` | TLS key path |


### Client settings
| Environment Variable | Default Value | Description |
|---------------------|---------------|-------------|
| CLIENT_TIMEOUT | `30s` | Client timeout |
| CLIENT_MAX_IDLE_CONNS | `20` | Maximum idle connections |
| CLIENT_MAX_IDLE_CONNS_PER_HOST | `20` | Maximum idle connections per host |
| CLIENT_IDLE_CONN_TIMEOUT | `30s` | Idle connection timeout |
| CLIENT_TLS_MIN_VERSION | `TLS12` | Minimum TLS version |


### Providers
| Environment Variable | Default Value | Description |
|---------------------|---------------|-------------|
| ANTHROPIC_API_URL | `https://api.anthropic.com/v1` | Anthropic API URL |
| ANTHROPIC_API_KEY | `""` | Anthropic API Key |
| CLOUDFLARE_API_URL | `https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}/ai` | Cloudflare API URL |
| CLOUDFLARE_API_KEY | `""` | Cloudflare API Key |
| COHERE_API_URL | `https://api.cohere.ai` | Cohere API URL |
| COHERE_API_KEY | `""` | Cohere API Key |
| GROQ_API_URL | `https://api.groq.com/openai/v1` | Groq API URL |
| GROQ_API_KEY | `""` | Groq API Key |
| OLLAMA_API_URL | `http://ollama:8080/v1` | Ollama API URL |
| OLLAMA_API_KEY | `""` | Ollama API Key |
| OPENAI_API_URL | `https://api.openai.com/v1` | OpenAI API URL |
| OPENAI_API_KEY | `""` | OpenAI API Key |

