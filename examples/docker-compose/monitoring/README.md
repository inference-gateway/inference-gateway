# Monitoring Example with Enhanced Function/Tool Call Metrics

This example demonstrates comprehensive monitoring setup for the Inference Gateway using:

- **Prometheus** for metrics collection
- **Grafana** for visualization with enhanced dashboards
- **Function/Tool Call Metrics** tracking MCP and A2A tool executions

## 📊 Dashboard Features

The enhanced Grafana dashboard provides:

### Function/Tool Call Metrics

- **Total Tool Calls** - Real-time count of all function/tool executions
- **Tool Call Success Rate** - Percentage of successful tool calls with thresholds
- **Failed Tool Calls** - Count of failures for quick issue identification
- **Average Tool Call Duration** - Performance monitoring for tool execution
- **Tool Call Rate by Type** - Breakdown by MCP, A2A, and other tool types
- **Tool Call Duration by Provider** - Latency analysis across providers
- **Top Tool Names by Usage** - Most frequently called tools
- **Tool Failures by Error Type** - Detailed failure analysis

### Traditional LLM Metrics

- **Request Latency by Provider** - End-to-end request performance
- **Tokens per Second by Provider** - Throughput monitoring
- **API Error Rate by Provider** - Success/failure rates
- **Prompt Token Usage** - Token consumption patterns
- **Memory Usage** - System resource monitoring
- **System Metrics** - CPU, memory, and goroutines

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

   - **Inference Gateway**: http://localhost:8080
   - **Prometheus**: http://localhost:9090
   - **Grafana**: http://localhost:3000 (admin/admin)

4. **View enhanced metrics:**
   - Navigate to the "Inference Gateway - Enhanced Metrics" dashboard
   - Send requests with tool calls to see metrics populate

## 🔧 Configuration

### Gateway Configuration

The gateway is configured with telemetry enabled:

```yaml
environment:
  - TELEMETRY_ENABLE=true
  - TELEMETRY_METRICS_PORT=9464
```

### Prometheus Configuration

Scrapes gateway metrics every 5 seconds:

```yaml
- job_name: "inference-gateway"
  static_configs:
    - targets: ["inference-gateway:9464"]
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
    "model": "deepseek/deepseek-chat",
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

### Example A2A Agent Request

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Calculate 15 + 25 using the calculator agent"}
    ]
  }'
```

## 📈 Metrics Details

### Function/Tool Call Metrics Labels

All tool call metrics include rich labeling:

- `provider` - LLM provider (openai, anthropic, etc.)
- `model` - Model name (gpt-4, claude-3-sonnet, etc.)
- `tool_type` - Tool type (mcp, a2a, native)
- `tool_name` - Specific tool name (list_files, calculator, etc.)
- `error_type` - Error classification (for failures only)

### Metric Types

- **Counters**: `llm_tool_calls_total`, `llm_tool_calls_success_total`, `llm_tool_calls_failure_total`
- **Histograms**: `llm_tool_call_duration` (includes \_bucket, \_sum, \_count)

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
2. Check Prometheus targets: http://localhost:9090/targets
3. Ensure gateway has telemetry enabled in environment

### Tool Call Metrics Empty

1. Send requests that actually trigger tool calls
2. Verify MCP or A2A middleware is enabled
3. Check gateway logs for tool execution

### Dashboard Not Loading

1. Verify Grafana provisioning: `docker compose logs grafana`
2. Check dashboard JSON syntax
3. Restart Grafana: `docker compose restart grafana`

## 🔗 Related Documentation

- [Main Repository](https://github.com/inference-gateway/inference-gateway)
- [MCP Integration Guide](../../mcp/README.md)
- [A2A Integration Guide](../../a2a/README.md)
- [Kubernetes Monitoring Example](../../kubernetes/monitoring/README.md)

## 🧹 Cleanup

```bash
docker compose down -v
```

This removes all containers and volumes, including stored metrics data.
