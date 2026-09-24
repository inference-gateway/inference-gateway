# MCP with Client-Declared Tools

Shows what the gateway does with tools a client declares itself (opencode, IDE
agents, `infer`) while MCP is enabled, and what happens when every MCP server
is down. Tracked in
[#698](https://github.com/inference-gateway/inference-gateway/issues/698).

No API key is needed: the LLM is a scripted mock. Both gateways are built from
this checkout, so rerunning the example after a change shows whether it fixed
the behaviour.

## Run

Needs Docker and [Task](https://taskfile.dev).

```bash
task up     # build both gateways from this checkout and wait until healthy
task test   # run every case; `task <case>` runs one
task logs   # the tools each upstream call carried
task down
```

## Services

- **inference-gateway** (`:8080`): MCP enabled with one server, the time server
  from the [MCP example](../mcp/README.md).
- **inference-gateway-mcp-down** (`:8081`): MCP enabled, but its only server is
  unreachable.
- **mock-llm**: when offered any tools it asks for the MCP `time` tool if the
  user asked for the time, and for `bash` otherwise, even if the gateway removed
  it, so you can see what the gateway does with either kind of call. After a
  tool result, or when offered no tools, it replies with the tools it received
  and the last message.
- **mcp-time-server**: reused from `../mcp/time-server`.

## Cases

The client tools are `bash` and `read`. The `client-tools` and `mcp-down` cases
ask to list files, the `mcp-tool` cases ask for the time.

| Task                    | Request                               | Expected                                                       |
| ----------------------- | ------------------------------------- | -------------------------------------------------------------- |
| `client-tools`          | Client tools, non-streaming           | `bash` tool call returned to the client                        |
| `client-tools:stream`   | Client tools, streaming               | `bash` tool-call deltas streamed to the client                 |
| `client-tools:bypass`   | `client-tools` with `X-MCP-Bypass`    | Control: the tool call reaches the client untouched            |
| `mcp-down`              | All MCP servers down, no client tools | `200`, the request passes through without MCP tools            |
| `mcp-down:bypass`       | `mcp-down` with `X-MCP-Bypass`        | Control: `200`                                                 |
| `mcp-tool`              | No client tools, non-streaming        | The gateway runs the MCP `time` tool and returns the answer    |
| `mcp-tool:stream`       | No client tools, streaming            | Only the answer is streamed, no tool-call deltas               |
| `mcp-tool:client-tools` | Client tools, non-streaming           | The gateway runs the MCP `time` tool; client tools stay unused |
