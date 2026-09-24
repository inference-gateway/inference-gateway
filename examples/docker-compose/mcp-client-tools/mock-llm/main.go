// Command mock-llm is a scripted OpenAI-compatible chat completions server.
//
// When offered any tools it asks for the MCP `time` tool if the user asked for
// the time, and for the client tool `bash` otherwise, even if the gateway
// removed it, so the gateway's handling of either kind of call is visible.
// Once a tool result comes back, or when no tools were offered, it answers
// with the tools it received and the last message, so the client can see what
// the gateway did without reading any logs.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"
)

const (
	listenAddr     = ":8080"
	modelName      = "mock"
	clientToolName = "bash"
	clientToolArgs = `{"command":"ls"}`
	timeKeyword    = "time"
	// The gateway offers MCP tools either through the selector meta-tool
	// (MCP_TOOL_MODE=selector, the default) or one by one (direct).
	selectorExecute  = "mcp_tools_execute"
	selectorTimeArgs = `{"name":"time","arguments":{}}`
	directTimeTool   = "mcp_time"
	directTimeArgs   = `{}`
	toolCallID       = "call_mock_1"
	toolRole         = "tool"
	finishStop       = "stop"
	finishTools      = "tool_calls"
	sseDone          = "data: [DONE]\n\n"
)

type chatRequest struct {
	Stream   bool `json:"stream"`
	Messages []struct {
		Role    string `json:"role"`
		Content any    `json:"content"`
	} `json:"messages"`
	Tools []struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	} `json:"tools"`
}

func main() {
	http.HandleFunc("POST /v1/chat/completions", chatCompletions)
	log.Printf("mock-llm listening on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

func chatCompletions(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Messages) == 0 {
		http.Error(w, "invalid chat completion request", http.StatusBadRequest)
		return
	}

	tools := make([]string, 0, len(req.Tools))
	for _, t := range req.Tools {
		tools = append(tools, t.Function.Name)
	}
	last := req.Messages[len(req.Messages)-1]
	log.Printf("stream=%t tools=%v last_role=%s", req.Stream, tools, last.Role)

	if last.Role == toolRole || len(tools) == 0 {
		reply(w, req.Stream, map[string]any{
			"role":    "assistant",
			"content": fmt.Sprintf("tools I received: %v; last %s message: %v", tools, last.Role, last.Content),
		}, finishStop)
		return
	}

	name, args := clientToolName, clientToolArgs
	if strings.Contains(strings.ToLower(fmt.Sprint(last.Content)), timeKeyword) {
		name, args = directTimeTool, directTimeArgs
		if slices.Contains(tools, selectorExecute) {
			name, args = selectorExecute, selectorTimeArgs
		}
	}

	reply(w, req.Stream, map[string]any{
		"role":    "assistant",
		"content": "",
		"tool_calls": []map[string]any{{
			"index":    0,
			"id":       toolCallID,
			"type":     "function",
			"function": map[string]any{"name": name, "arguments": args},
		}},
	}, finishTools)
}

// reply writes message as a single completion, or as two SSE chunks (the
// message as a delta, then the finish reason) when stream is set.
func reply(w http.ResponseWriter, stream bool, message map[string]any, finish string) {
	base := map[string]any{"id": "chatcmpl-mock", "created": time.Now().Unix(), "model": modelName}

	if !stream {
		base["object"] = "chat.completion"
		base["choices"] = []map[string]any{{"index": 0, "message": message, "finish_reason": finish}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(base)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	base["object"] = "chat.completion.chunk"
	for _, choice := range []map[string]any{
		{"index": 0, "delta": message, "finish_reason": nil},
		{"index": 0, "delta": map[string]any{}, "finish_reason": finish},
	} {
		base["choices"] = []map[string]any{choice}
		chunk, _ := json.Marshal(base)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk)
	}
	_, _ = fmt.Fprint(w, sseDone)
}
