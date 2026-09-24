package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"

	otelapi "go.opentelemetry.io/otel"
	propagation "go.opentelemetry.io/otel/propagation"
)

// The gateway speaks one MCP revision in both directions: to clients on
// POST /mcp and to every server in MCP_SERVERS. 2026-07-28 is stateless, so
// there is no initialize handshake and no session to keep; every request
// carries its version in params._meta, mirrored into headers.
const (
	ProtocolVersion = "2026-07-28"

	HeaderProtocolVersion = "MCP-Protocol-Version"
	HeaderMethod          = "Mcp-Method"
	HeaderName            = "Mcp-Name"

	// ResultTypeComplete marks a final result; a server on an earlier
	// revision omits resultType, which clients must read as complete.
	ResultTypeComplete = "complete"
	// resultTypeInputRequired asks the client for elicitation or sampling
	// input, which the gateway cannot provide on anyone's behalf.
	resultTypeInputRequired = "input_required"

	contentTypeJSON = "application/json"
	contentTypeSSE  = "text/event-stream"

	// rpcRequestID is the id of every outbound request. Each one travels
	// alone in its own HTTP exchange, so there is nothing to tell apart.
	rpcRequestID = 1
)

// GatewayInfo names the gateway as an MCP implementation: the serverInfo
// POST /mcp returns and the clientInfo sent upstream. main sets the version.
var GatewayInfo = Implementation{Name: "inference-gateway", Version: "dev"}

// ErrInputRequired is returned when a tool asks for client input mid-call.
var ErrInputRequired = errors.New("mcp tool requires client input, which the gateway does not provide")

func requestMeta() RequestMetaObject {
	return RequestMetaObject{
		IoModelcontextprotocolProtocolVersion: ProtocolVersion,
		IoModelcontextprotocolClientInfo:      &GatewayInfo,
	}
}

// listTools fetches every page of a server's tools.
func (mc *MCPClient) listTools(ctx context.Context, serverURL string) ([]Tool, error) {
	tools := make([]Tool, 0)
	var cursor *string
	for {
		var page ListToolsResult
		req := ListToolsRequest{
			ID:      rpcRequestID,
			Jsonrpc: ListToolsRequestJsonrpcN20,
			Method:  ToolsList,
			Params:  PaginatedRequestParams{UnderscoreMeta: requestMeta(), Cursor: cursor},
		}
		if err := mc.rpc(ctx, serverURL, string(ToolsList), "", req, &page); err != nil {
			return nil, err
		}
		for _, tool := range page.Tools {
			if tool.InputSchema == nil {
				tool.InputSchema = make(map[string]any)
			}
			tools = append(tools, tool)
		}

		next := page.NextCursor
		if next == nil || *next == "" || (cursor != nil && *next == *cursor) {
			return tools, nil
		}
		cursor = next
	}
}

// callTool runs a tool on a server under its bare name. arguments is always
// sent, as {} when there are none: the generated CallToolRequestParams would
// omit it, and some servers fail to decode a call without it.
func (mc *MCPClient) callTool(ctx context.Context, serverURL, name string, arguments map[string]any) (*CallToolResult, error) {
	if arguments == nil {
		arguments = make(map[string]any)
	}
	req := JSONRPCRequest{
		ID:      rpcRequestID,
		Jsonrpc: JSONRPCRequestJsonrpcN20,
		Method:  string(ToolsCall),
		Params:  map[string]any{"_meta": requestMeta(), "name": name, "arguments": arguments},
	}
	var result CallToolResult
	if err := mc.rpc(ctx, serverURL, string(ToolsCall), name, req, &result); err != nil {
		return nil, err
	}

	switch result.ResultType {
	case "":
		result.ResultType = ResultTypeComplete
	case resultTypeInputRequired:
		return nil, ErrInputRequired
	}
	return &result, nil
}

// rpc POSTs one JSON-RPC request and decodes its result. The request is built
// from scratch, so no inbound credential can reach the upstream server.
// ponytail: Mcp-Param-* headers and base64 Mcp-Name values are not sent; add
// them once an upstream tool declares x-mcp-header or a non-ASCII name.
func (mc *MCPClient) rpc(ctx context.Context, serverURL, method, name string, req, result any) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("encoding %s request: %w", method, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, serverURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building %s request: %w", method, err)
	}
	httpReq.Header.Set("Content-Type", contentTypeJSON)
	httpReq.Header.Set("Accept", contentTypeJSON+", "+contentTypeSSE)
	httpReq.Header.Set(HeaderProtocolVersion, ProtocolVersion)
	httpReq.Header.Set(HeaderMethod, method)
	if name != "" {
		httpReq.Header.Set(HeaderName, name)
	}
	otelapi.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(httpReq.Header))

	resp, err := mc.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	var raw []byte
	if mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type")); mediaType == contentTypeSSE {
		raw, err = sseResponse(resp.Body)
	} else {
		raw, err = io.ReadAll(resp.Body)
	}
	if err != nil {
		return fmt.Errorf("%s: reading http %d response: %w", method, resp.StatusCode, err)
	}

	// A JSON-RPC error can arrive with a 4xx (e.g. -32022 with 400), so the
	// body is decoded before the status is judged.
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *Error          `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("%s: http %d with no json-rpc response", method, resp.StatusCode)
	}
	if envelope.Error != nil {
		return fmt.Errorf("%s: json-rpc error %d: %s", method, envelope.Error.Code, envelope.Error.Message)
	}
	if resp.StatusCode != http.StatusOK || envelope.Result == nil {
		return fmt.Errorf("%s: http %d with no result", method, resp.StatusCode)
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("%s: decoding result: %w", method, err)
	}
	return nil
}

// sseResponse returns the data of the first event on an SSE stream that
// carries a JSON-RPC response, skipping any notification sent before it. It
// reads with bufio.Reader because a tool result can outgrow Scanner's
// line limit.
func sseResponse(r io.Reader) ([]byte, error) {
	reader := bufio.NewReader(r)
	var data []byte
	for {
		line, readErr := reader.ReadBytes('\n')
		line = bytes.TrimRight(line, "\r\n")

		if value, ok := bytes.CutPrefix(line, []byte("data:")); ok {
			if len(data) > 0 {
				data = append(data, '\n')
			}
			data = append(data, bytes.TrimPrefix(value, []byte(" "))...)
		} else if len(line) == 0 { // a blank line ends the event
			if isJSONRPCResponse(data) {
				return data, nil
			}
			data = nil
		}

		if readErr != nil {
			if isJSONRPCResponse(data) { // the stream closed without a final blank line
				return data, nil
			}
			if readErr == io.EOF {
				return nil, errors.New("sse stream ended without a json-rpc response")
			}
			return nil, readErr
		}
	}
}

// isJSONRPCResponse reports whether an SSE event's data is a response (it has
// an id and no method) rather than a notification or a server request.
func isJSONRPCResponse(data []byte) bool {
	var msg struct {
		ID     any    `json:"id"`
		Method string `json:"method"`
	}
	return len(data) > 0 && json.Unmarshal(data, &msg) == nil && msg.ID != nil && msg.Method == ""
}
