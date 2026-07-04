# Monitoring Example with Enhanced Function/Tool Call Metrics

This example demonstrates comprehensive monitoring setup for the Inference Gateway using:

- **Prometheus** for metrics collection
- **Grafana** for visualization with enhanced dashboards
- **Function/Tool Call Metrics** tracking MCP tool executions

## 📊 Dashboard Features

The Grafana dashboard (identical to the Kubernetes monitoring example) is organized in rows:

- **Overview** - Requests via gateway, input/output token totals, tool calls, sources reporting, and 5m error rate
- **Traffic** - Request rate and latency (p95 + avg) by provider
- **Token Usage** - Token rate by source and cumulative totals by source & model
- **Tool Calls** - Tool call rate by type and top tools by usage
- **Pushed Client Metrics (OTLP push only)** - Tool execution duration, client operation duration, time to first
  token/chunk, tool failures by error type, and tool success rate; fed exclusively by clients pushing to `POST /v1/metrics`
- **Gateway Process** - CPU, goroutines, and resident memory

All panels are filterable by the `provider` and `source` template variables, so gateway-served traffic and pushed
client metrics (e.g. `source="claude-code-subscription"`) can be viewed together or separately.

## 🚀 Quick Start

1. **Create environment file:**

   ```bash
   cp .env.example .env
   # Edit .env with your provider API keys
   ```

2. **Start the monitoring stack:**

   ```bash
   docker compose up -d
   ```

3. **Access services:**
   - **Inference Gateway**: <http://localhost:8080>
   - **Prometheus**: <http://localhost:9090>
   - **Grafana**: <http://localhost:3000> (admin/admin)

4. **View enhanced metrics:**
   - Navigate to the "Inference Gateway" dashboard
   - Send requests with tool calls to see metrics populate

## 🔧 Configuration

### Gateway Configuration

The gateway is configured with telemetry enabled:

```yaml
environment:
  - TELEMETRY_ENABLE=true
  - TELEMETRY_METRICS_PORT=9464
  - TELEMETRY_METRICS_PUSH_ENABLE=true
```

### Prometheus Configuration

Scrapes gateway metrics every 5 seconds:

```yaml
- job_name: 'inference-gateway'
  static_configs:
    - targets: ['inference-gateway:9464']
  scrape_interval: 5s
```

### Grafana Configuration

- Automatically provisions Prometheus as datasource
- Pre-loads enhanced dashboard with function/tool call metrics
- Configured with 5-second refresh rate for real-time monitoring

## 🧪 Testing Function/Tool Call Metrics

### Example MCP Tool Call Request

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek/deepseek-v4-flash",
    "messages": [
      {"role": "user", "content": "What files are in the current directory?"}
    ],
    "tools": [
      {
        "type": "function",
        "function": {
          "name": "list_files",
          "description": "List files in a directory"
        }
      }
    ]
  }'
```

## 📈 Metrics Details

The gateway exposes metrics following the [OpenTelemetry GenAI semantic conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/).

### Metric Set

- `gen_ai_client_token_usage` (histogram, no unit suffix) - token usage; `gen_ai_token_type` is `input` or `output`
- `gen_ai_server_request_duration_seconds` (histogram) - end-to-end request duration in seconds; `error_type` (HTTP status string) is set only on errors
- `gen_ai_execute_tool_duration_seconds` (histogram) - tool execution duration in seconds (fed via the push endpoint)
- `gen_ai_client_operation_duration_seconds`, `gen_ai_client_operation_time_to_first_chunk_seconds`,
  `gen_ai_server_time_to_first_token_seconds` (histograms) - push-only client-side latency metrics
- `inference_gateway_tool_calls_total` (counter) - total function/tool calls

### Labels

- `gen_ai_provider_name` - LLM provider (openai, anthropic, etc.)
- `gen_ai_request_model` - Model name (gpt-4o, claude-sonnet-4, etc.)
- `gen_ai_operation_name` - Operation (e.g. chat)
- `gen_ai_tool_type` / `gen_ai_tool_name` - Tool metadata (tool metrics only)
- `gen_ai_token_type` - `input` or `output` (token usage only)
- `error_type` - HTTP status string, present only on errors
- `source` - `gateway` for gateway-observed traffic, or a client-supplied value (e.g. `claude-code-subscription`) for pushed metrics

### Migration from old `llm_*` metrics

| Old metric                          | New query                                                                   |
| ----------------------------------- | --------------------------------------------------------------------------- |
| `llm_usage_prompt_tokens_total`     | `gen_ai_client_token_usage_sum{gen_ai_token_type="input"}`                  |
| `llm_usage_completion_tokens_total` | `gen_ai_client_token_usage_sum{gen_ai_token_type="output"}`                 |
| `llm_usage_total_tokens_total`      | sum of `gen_ai_client_token_usage_sum` over both token types                |
| `llm_responses_total`               | `gen_ai_server_request_duration_seconds_count` (errors: `{error_type!=""}`) |
| `llm_request_duration_*` (ms)       | `gen_ai_server_request_duration_seconds_*` (seconds)                        |
| `llm_tool_calls_total`              | `inference_gateway_tool_calls_total`                                        |
| `llm_tool_calls_success_total`      | `gen_ai_execute_tool_duration_seconds_count{error_type=""}`                 |
| `llm_tool_calls_failure_total`      | `gen_ai_execute_tool_duration_seconds_count{error_type!=""}`                |
| `llm_tool_call_duration_*` (ms)     | `gen_ai_execute_tool_duration_seconds_*` (seconds)                          |

Label renames: `provider` → `gen_ai_provider_name`, `model` → `gen_ai_request_model`, `tool_name` → `gen_ai_tool_name`, `tool_type` → `gen_ai_tool_type`.

Note that duration histograms are now in **seconds** (previously milliseconds), so drop any `/1000` conversions in your queries.

## 📤 Pushing metrics (OTLP)

Subscription clients (e.g. the infer CLI driving Claude Code) can push their own metrics to the gateway.
Enable the opt-in push endpoint with `TELEMETRY_METRICS_PUSH_ENABLE=true` (alongside `TELEMETRY_ENABLE=true`),
then POST OTLP JSON to `/v1/metrics`:

```bash
curl -X POST http://localhost:8080/v1/metrics \
  -H 'Content-Type: application/json' \
  -d '{
    "resourceMetrics": [{
      "resource": {
        "attributes": [{ "key": "service.name", "value": { "stringValue": "infer-cli" } }]
      },
      "scopeMetrics": [{
        "metrics": [{
          "name": "gen_ai.client.token.usage",
          "sum": {
            "aggregationTemporality": 1,
            "dataPoints": [{
              "asInt": "1234",
              "attributes": [
                { "key": "gen_ai.provider.name", "value": { "stringValue": "anthropic" } },
                { "key": "gen_ai.token.type", "value": { "stringValue": "input" } },
                { "key": "source", "value": { "stringValue": "claude-code-subscription" } }
              ]
            }]
          }
        }]
      }]
    }]
  }'
```

Pushed series carry the client-supplied `source` label (e.g. `claude-code-subscription`), so they can be
distinguished from gateway-observed traffic (`source="gateway"`) in dashboards.

## 🎛️ Customization

### Adding Custom Dashboards

1. Create JSON dashboard file in `grafana/dashboards/`
2. Restart Grafana container: `docker compose restart grafana`

### Modifying Metrics Collection

Edit `prometheus.yml` to:

- Adjust scrape intervals
- Add additional targets
- Configure recording rules

### Dashboard Variables

The dashboard supports provider filtering via `$provider` variable:

- Select specific providers or "All"
- Dynamically populated from metrics

## 🐛 Troubleshooting

### No Metrics Showing

1. Verify gateway is exposing metrics: `curl http://localhost:9464/metrics`
2. Check Prometheus targets: <http://localhost:9090/targets>
3. Ensure gateway has telemetry enabled in environment

### Tool Call Metrics Empty

1. Send requests that actually trigger tool calls
2. Verify MCP middleware is enabled
3. Check gateway logs for tool execution

### Dashboard Not Loading

1. Verify Grafana provisioning: `docker compose logs grafana`
2. Check dashboard JSON syntax
3. Restart Grafana: `docker compose restart grafana`

## 🔗 Related Documentation

- [Main Repository](https://github.com/inference-gateway/inference-gateway)
- [MCP Integration Guide](../mcp/README.md)
- [Kubernetes Monitoring Example](../../kubernetes/monitoring/README.md)

## 🧹 Cleanup

```bash
docker compose down -v
```

This removes all containers and volumes, including stored metrics data.
