# MCP with Client-Declared Tools

Shows what the gateway does with tools a client declares itself (opencode, IDE
agents, `infer`) while MCP is enabled, and what happens when every MCP server
is down. Tracked in
[#698](https://github.com/inference-gateway/inference-gateway/issues/698).

The LLM is a scripted mock by default, so no API key is needed and every case
takes the same path; [Real provider](#real-provider) runs the same cases
against a real model. Both gateways are built from this checkout, so rerunning
the example after a change shows whether it fixed the behaviour.

## Run

Needs Docker and [Task](https://taskfile.dev).

```bash
task up     # build both gateways from this checkout and wait until healthy
task test   # run every case; `task <case>` runs one
task logs   # the tools each upstream call carried
task down
```

## Real provider

Only the LLM is mocked, so the same cases run against a real model with one key
and a model name - the gateways, the MCP server and the requests are unchanged:

```bash
cp .env.example .env   # then set GROQ_API_KEY=<your key>
task up
task test MODEL=groq/llama-3.3-70b-versatile   # task <case> MODEL=... for one case
```

Pick a model that supports tool calling, or every case answers in plain text.

Compose reads `.env` from this directory on its own and forwards only
`GROQ_API_KEY` to both gateways, so `task test` without `MODEL` still runs
against the mock and still needs no key. For another provider, add its
`<PROVIDER>_API_KEY` next to `GROQ_API_KEY` in `docker-compose.yml`; `OPENAI_*`
is taken, both gateways pin it to the mock.

`task logs` reads the mock, which a real run never calls - use
`docker compose logs inference-gateway` instead.

## Services

- **inference-gateway** (`:8080`): MCP enabled with one server, the time server
  from the [MCP example](../mcp/README.md), aliased `time`, so its tool is
  `mcp_time_time`.
- **inference-gateway-mcp-down** (`:8081`): MCP enabled, but its only server is
  unreachable.
- **mock-llm**: when offered any tools it asks for the MCP tool `mcp_time_time`
  if the user asked for the time, and for `bash` otherwise, even if the gateway
  removed it, so you can see what the gateway does with either kind of call.
  After a tool result, or when offered no tools, it replies with the tools it
  received and the last message. Idle during a real-provider run.
- **mcp-time-server**: reused from `../mcp/time-server`.

## Cases

The client tools are `bash` and `read`. The `client-tools` and `mcp-down` cases
ask to list files, the `mcp-tool` cases ask for the time.

| Task                    | Request                               | Expected with the mock                                         |
| ----------------------- | ------------------------------------- | -------------------------------------------------------------- |
| `client-tools`          | Client tools, non-streaming           | `bash` tool call returned to the client                        |
| `client-tools:stream`   | Client tools, streaming               | `bash` tool-call deltas streamed to the client                 |
| `client-tools:bypass`   | `client-tools` with `X-MCP-Bypass`    | Control: the tool call reaches the client untouched            |
| `mcp-down`              | All MCP servers down, no client tools | `200`, the request passes through without MCP tools            |
| `mcp-down:bypass`       | `mcp-down` with `X-MCP-Bypass`        | Control: `200`                                                 |
| `mcp-tool`              | No client tools, non-streaming        | The gateway runs the MCP `time` tool and returns the answer    |
| `mcp-tool:stream`       | No client tools, streaming            | Only the answer is streamed, no tool-call deltas               |
| `mcp-tool:client-tools` | Client tools, non-streaming           | The gateway runs the MCP `time` tool; client tools stay unused |

With a real model the table is a guide, not a script: it decides whether to
call a tool at all, so compare behaviour rather than strings.

- `mcp-tool*`: the model calls `mcp_tools_get`, then `mcp_tools_execute`, and
  the time arrives inside its answer instead of a fixed sentence. Declared
  client tools still come back unused.
- `client-tools*`: a model that decides to run a shell command gets the `bash`
  call handed back for the client to execute; the gateway never runs it. A
  model that just answers in prose is not a failure.
- The two `:bypass` controls only mean something with the mock, because it asks
  for `bash` even when the gateway removed it; a real model only calls a tool
  it was actually offered.
