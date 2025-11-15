## Configurations

### General settings

| Environment Variable         | Default Value | Description                                                                                                                                    |
| ---------------------------- | ------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| ENVIRONMENT                  | `production`  | The environment                                                                                                                                |
| ALLOWED_MODELS               | `""`          | Comma-separated list of models to allow. If empty, all models will be available                                                                |
| ENABLE_VISION                | `false`       | Enable vision/multimodal support for all providers. When disabled, image inputs will be rejected even if the provider and model support vision |
| DEBUG_CONTENT_TRUNCATE_WORDS | `10`          | Number of words to truncate per content section in debug logs (development mode only)                                                          |
| DEBUG_MAX_MESSAGES           | `100`         | Maximum number of messages to show in debug logs (development mode only)                                                                       |

### Telemetry

| Environment Variable   | Default Value | Description                       |
| ---------------------- | ------------- | --------------------------------- |
| TELEMETRY_ENABLE       | `false`       | Enable telemetry                  |
| TELEMETRY_METRICS_PORT | `9464`        | Port for telemetry metrics server |

### Model Context Protocol (MCP)

| Environment Variable         | Default Value | Description                                              |
| ---------------------------- | ------------- | -------------------------------------------------------- |
| MCP_ENABLE                   | `false`       | Enable MCP                                               |
| MCP_EXPOSE                   | `false`       | Expose MCP tools endpoint                                |
| MCP_SERVERS                  | `""`          | List of MCP servers                                      |
| MCP_CLIENT_TIMEOUT           | `5s`          | MCP client HTTP timeout                                  |
| MCP_DIAL_TIMEOUT             | `3s`          | MCP client dial timeout                                  |
| MCP_TLS_HANDSHAKE_TIMEOUT    | `3s`          | MCP client TLS handshake timeout                         |
| MCP_RESPONSE_HEADER_TIMEOUT  | `3s`          | MCP client response header timeout                       |
| MCP_EXPECT_CONTINUE_TIMEOUT  | `1s`          | MCP client expect continue timeout                       |
| MCP_REQUEST_TIMEOUT          | `5s`          | MCP client request timeout for initialize and tool calls |
| MCP_MAX_RETRIES              | `3`           | Maximum number of connection retry attempts              |
| MCP_RETRY_INTERVAL           | `5s`          | Interval between connection retry attempts               |
| MCP_INITIAL_BACKOFF          | `1s`          | Initial backoff duration for exponential backoff retry   |
| MCP_ENABLE_RECONNECT         | `true`        | Enable automatic reconnection for failed servers         |
| MCP_RECONNECT_INTERVAL       | `30s`         | Interval between reconnection attempts                   |
| MCP_POLLING_ENABLE           | `true`        | Enable health check polling                              |
| MCP_POLLING_INTERVAL         | `30s`         | Interval between health check polling requests           |
| MCP_POLLING_TIMEOUT          | `5s`          | Timeout for individual health check requests             |
| MCP_DISABLE_HEALTHCHECK_LOGS | `true`        | Disable health check log messages to reduce noise        |

### Authentication

| Environment Variable    | Default Value                                         | Description           |
| ----------------------- | ----------------------------------------------------- | --------------------- |
| AUTH_ENABLE             | `false`                                               | Enable authentication |
| AUTH_OIDC_ISSUER        | `http://keycloak:8080/realms/inference-gateway-realm` | OIDC issuer URL       |
| AUTH_OIDC_CLIENT_ID     | `inference-gateway-client`                            | OIDC client ID        |
| AUTH_OIDC_CLIENT_SECRET | `""`                                                  | OIDC client secret    |

### Server settings

| Environment Variable | Default Value | Description          |
| -------------------- | ------------- | -------------------- |
| SERVER_HOST          | `0.0.0.0`     | Server host          |
| SERVER_PORT          | `8080`        | Server port          |
| SERVER_READ_TIMEOUT  | `30s`         | Read timeout         |
| SERVER_WRITE_TIMEOUT | `30s`         | Write timeout        |
| SERVER_IDLE_TIMEOUT  | `120s`        | Idle timeout         |
| SERVER_TLS_CERT_PATH | `""`          | TLS certificate path |
| SERVER_TLS_KEY_PATH  | `""`          | TLS key path         |

### Client settings

| Environment Variable           | Default Value | Description                              |
| ------------------------------ | ------------- | ---------------------------------------- |
| CLIENT_TIMEOUT                 | `30s`         | Client timeout                           |
| CLIENT_MAX_IDLE_CONNS          | `20`          | Maximum idle connections                 |
| CLIENT_MAX_IDLE_CONNS_PER_HOST | `20`          | Maximum idle connections per host        |
| CLIENT_IDLE_CONN_TIMEOUT       | `30s`         | Idle connection timeout                  |
| CLIENT_TLS_MIN_VERSION         | `TLS12`       | Minimum TLS version                      |
| CLIENT_DISABLE_COMPRESSION     | `true`        | Disable compression for faster streaming |
| CLIENT_RESPONSE_HEADER_TIMEOUT | `10s`         | Response header timeout                  |
| CLIENT_EXPECT_CONTINUE_TIMEOUT | `1s`          | Expect continue timeout                  |

### Providers

| Environment Variable | Default Value                                                   | Description        |
| -------------------- | --------------------------------------------------------------- | ------------------ |
| ANTHROPIC_API_URL    | `https://api.anthropic.com/v1`                                  | Anthropic API URL  |
| ANTHROPIC_API_KEY    | `""`                                                            | Anthropic API Key  |
| CLOUDFLARE_API_URL   | `https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}/ai` | Cloudflare API URL |
| CLOUDFLARE_API_KEY   | `""`                                                            | Cloudflare API Key |
| COHERE_API_URL       | `https://api.cohere.ai`                                         | Cohere API URL     |
| COHERE_API_KEY       | `""`                                                            | Cohere API Key     |
| GROQ_API_URL         | `https://api.groq.com/openai/v1`                                | Groq API URL       |
| GROQ_API_KEY         | `""`                                                            | Groq API Key       |
| OLLAMA_API_URL       | `http://ollama:8080/v1`                                         | Ollama API URL     |
| OLLAMA_API_KEY       | `""`                                                            | Ollama API Key     |
| OPENAI_API_URL       | `https://api.openai.com/v1`                                     | OpenAI API URL     |
| OPENAI_API_KEY       | `""`                                                            | OpenAI API Key     |
| DEEPSEEK_API_URL     | `https://api.deepseek.com`                                      | DeepSeek API URL   |
| DEEPSEEK_API_KEY     | `""`                                                            | DeepSeek API Key   |
| GOOGLE_API_URL       | `https://generativelanguage.googleapis.com/v1beta/openai`       | Google API URL     |
| GOOGLE_API_KEY       | `""`                                                            | Google API Key     |
| MISTRAL_API_URL      | `https://api.mistral.ai/v1`                                     | Mistral API URL    |
| MISTRAL_API_KEY      | `""`                                                            | Mistral API Key    |
