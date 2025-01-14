# Inference Gateway Configuration

## General settings

| Key | Default Value | Description |
| --- | ------------- | ----------- |
| APPLICATION_NAME | inference-gateway | The name of the application |
| ENABLE_TELEMETRY | false | Enable telemetry for the server |
| ENVIRONMENT | production | The environment in which the application is running |
| ENABLE_AUTH | false | Enable authentication |
| OIDC_ISSUER_URL | http://keycloak:8080/realms/inference-gateway-realm | The OIDC issuer URL |
| OIDC_CLIENT_ID | inference-gateway-client | The OIDC client ID |
| OIDC_CLIENT_SECRET |  | The OIDC client secret |

## Server settings

| Key | Default Value | Description |
| --- | ------------- | ----------- |
| SERVER_HOST | 127.0.0.1 | The host address for the server |
| SERVER_PORT | 8080 | The port on which the server will listen |
| SERVER_READ_TIMEOUT | 30s | The server read timeout |
| SERVER_WRITE_TIMEOUT | 30s | The server write timeout |
| SERVER_IDLE_TIMEOUT | 120s | The server idle timeout |
| SERVER_TLS_CERT_PATH |  | The path to the TLS certificate |
| SERVER_TLS_KEY_PATH |  | The path to the TLS key |

## API URLs and keys

| Key | Default Value | Description |
| --- | ------------- | ----------- |
| OLLAMA_API_URL | http://ollama:8080 | The URL for Ollama API |
| GROQ_API_URL | https://api.groq.com | The URL for Groq Cloud API |
| GROQ_API_KEY |  | The Access token for Groq Cloud API |
| OPENAI_API_URL | https://api.openai.com | The URL for OpenAI API |
| OPENAI_API_KEY |  | The Access token for OpenAI API |
| GOOGLE_AISTUDIO_API_URL | https://generativelanguage.googleapis.com | The URL for Google AI Studio API |
| GOOGLE_AISTUDIO_API_KEY |  | The Access token for Google AI Studio API |
| CLOUDFLARE_API_URL | https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID} | The URL for Cloudflare API |
| CLOUDFLARE_API_KEY |  | The Access token for Cloudflare API |
