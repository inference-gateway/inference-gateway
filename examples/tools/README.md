# Tools

Function calling around the gateway. Two variants, both docker-compose only:

- [agent](agent/docker-compose/README.md) - a shell agent that sends a request
  needing tools, runs the tool calls (mock weather and web search) and returns
  the results to the model
- [mcp-client](mcp-client/docker-compose/README.md) - what the MCP middleware
  does with tools the client declares itself: which calls are handed back to the
  client, which run as MCP tools, and what happens when every MCP server is
  down. A scripted mock LLM drives every case, with an opt-in real provider.
