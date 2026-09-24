package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	gin "github.com/gin-gonic/gin"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	guardrails "github.com/inference-gateway/inference-gateway/internal/guardrails"
	mcp "github.com/inference-gateway/inference-gateway/internal/mcp"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// JSON-RPC 2.0 error codes, as listed in the /mcp spec.
const (
	jsonRPCParseError     = -32700
	jsonRPCInvalidRequest = -32600
	jsonRPCMethodNotFound = -32601
	jsonRPCInvalidParams  = -32602
	jsonRPCInternalError  = -32603

	// jsonRPCHeaderMismatch and jsonRPCUnsupportedVersion are the codes MCP
	// 2026-07-28 reserves for request metadata failures.
	jsonRPCHeaderMismatch     = -32020
	jsonRPCUnsupportedVersion = -32022
)

const (
	// headerOrigin is refused on /mcp, and the meta keys are the params._meta
	// key carrying the protocol version and the result _meta key naming the
	// gateway. The mirrored MCP headers are shared with the outbound client.
	headerOrigin        = "Origin"
	metaProtocolVersion = "io.modelcontextprotocol/protocolVersion"
	metaServerInfo      = "io.modelcontextprotocol/serverInfo"

	// base64HeaderPrefix and base64HeaderSuffix wrap header values that are
	// not plain ASCII: =?base64?<value>?=.
	base64HeaderPrefix = "=?base64?"
	base64HeaderSuffix = "?="

	errMsgMCPNotExposed = "MCP endpoint is not exposed. Set MCP_EXPOSE=true to enable."
	errMsgMCPOrigin     = "MCP endpoint does not accept browser requests"
	errMsgMCPUnusable   = "no mcp servers are available"
	errMsgParse         = "parse error"
	errMsgInvalidReq    = "invalid request: jsonrpc must be \"2.0\" and method is required"
	errMsgUnsupported   = "unsupported protocol version"
)

// MCPJSONRPCHandler serves POST /mcp: the JSON-RPC 2.0 surface that exposes
// every configured MCP server's tools through the gateway, so a client
// configures one entry and gets the whole fleet. Gated by MCP_ENABLED and
// MCP_EXPOSE; gateway auth applies like it does to every route but /health.
func (router *RouterImpl) MCPJSONRPCHandler(c *gin.Context) {
	if !router.cfg.MCP.Enabled || !router.cfg.MCP.Expose {
		router.logger.Error("mcp endpoint access attempted but not exposed", nil)
		c.JSON(http.StatusForbidden, ErrorResponse{Error: errMsgMCPNotExposed})
		return
	}
	if origin := c.GetHeader(headerOrigin); origin != "" {
		router.logger.Error("mcp request with an origin header rejected", nil, "origin", origin)
		c.JSON(http.StatusForbidden, ErrorResponse{Error: errMsgMCPOrigin})
		return
	}

	body, tooLarge, err := router.readBoundedBody(c)
	if err != nil {
		router.logger.Error("failed to read mcp jsonrpc request body", err)
		router.respondMCPError(c, nil, jsonRPCParseError, errMsgParse)
		return
	}
	if tooLarge {
		c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
		return
	}
	if !json.Valid(body) {
		router.logger.Error("mcp jsonrpc request is not valid json", nil)
		router.respondMCPError(c, nil, jsonRPCParseError, errMsgParse)
		return
	}

	var req types.MCPJSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("mcp jsonrpc request has an unexpected shape", err)
		router.respondMCPError(c, nil, jsonRPCInvalidRequest, errMsgInvalidReq)
		return
	}
	if req.Jsonrpc != types.MCPJSONRPCRequestJsonrpcN20 || req.Method == "" {
		router.logger.Error("mcp jsonrpc request is missing jsonrpc version or method", nil, "method", string(req.Method))
		router.respondMCPError(c, &req, jsonRPCInvalidRequest, errMsgInvalidReq)
		return
	}

	if req.ID == nil {
		router.logger.Debug("mcp notification accepted", "method", string(req.Method))
		c.Status(http.StatusAccepted)
		return
	}

	if rpcErr := validateMCPRequest(c.Request.Header, &req); rpcErr != nil {
		router.logger.Error("mcp request metadata rejected", nil, "method", string(req.Method), "reason", rpcErr.Message)
		router.writeMCPError(c, &req, rpcErr)
		return
	}

	switch req.Method {
	case types.ServerDiscover:
		router.respondMCPResult(c, &req, discoverResult())
	case types.ToolsList:
		router.respondMCPResult(c, &req, router.mcpToolsList())
	case types.ToolsCall:
		router.mcpToolsCall(c, &req)
	default:
		router.logger.Error("unsupported mcp method", nil, "method", string(req.Method))
		router.respondMCPError(c, &req, jsonRPCMethodNotFound, "method not found: "+string(req.Method))
	}
}

// validateMCPRequest enforces the 2026-07-28 request metadata: the protocol
// version in params._meta, mirrored into MCP-Protocol-Version, plus the method
// and, for tools/call, the tool name mirrored into Mcp-Method and Mcp-Name. A
// legacy client opening with initialize sends none of these and is told which
// version the gateway speaks.
// ponytail: Mcp-Param-* headers are not validated; add it once an upstream
// tool declares x-mcp-header in its inputSchema.
func validateMCPRequest(header http.Header, req *types.MCPJSONRPCRequest) *types.MCPJSONRPCError {
	for _, name := range []string{mcp.HeaderProtocolVersion, mcp.HeaderMethod, mcp.HeaderName} {
		if len(header.Values(name)) > 1 {
			return headerMismatch(name + " header sent more than once")
		}
	}

	version := header.Get(mcp.HeaderProtocolVersion)
	if version == "" {
		return headerMismatch("missing " + mcp.HeaderProtocolVersion + " header; this server speaks MCP " + mcp.ProtocolVersion)
	}
	if version != mcp.ProtocolVersion {
		return &types.MCPJSONRPCError{
			Code:    jsonRPCUnsupportedVersion,
			Message: errMsgUnsupported,
			Data:    map[string]any{"requested": version, "supported": []string{mcp.ProtocolVersion}},
		}
	}

	params := map[string]any{}
	if req.Params != nil {
		params = *req.Params
	}
	meta, _ := params["_meta"].(map[string]any)
	if bodyVersion, _ := meta[metaProtocolVersion].(string); bodyVersion != version {
		return headerMismatch(mcp.HeaderProtocolVersion + " header does not match params._meta " + metaProtocolVersion)
	}
	if header.Get(mcp.HeaderMethod) != string(req.Method) {
		return headerMismatch(mcp.HeaderMethod + " header does not match method " + string(req.Method))
	}
	if req.Method == types.ToolsCall {
		name, _ := params["name"].(string)
		if headerName, ok := decodeHeaderValue(header.Get(mcp.HeaderName)); !ok || headerName == "" || headerName != name {
			return headerMismatch(mcp.HeaderName + " header does not match params.name")
		}
	}
	return nil
}

func headerMismatch(message string) *types.MCPJSONRPCError {
	return &types.MCPJSONRPCError{Code: jsonRPCHeaderMismatch, Message: "header mismatch: " + message}
}

// decodeHeaderValue undoes the =?base64?<value>?= encoding clients use for
// header values that are not plain ASCII; ok is false when it is malformed.
func decodeHeaderValue(value string) (string, bool) {
	encoded, found := strings.CutPrefix(value, base64HeaderPrefix)
	if !found {
		return value, true
	}
	encoded, found = strings.CutSuffix(encoded, base64HeaderSuffix)
	if !found {
		return "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", false
	}
	return string(decoded), true
}

// discoverResult advertises the one protocol version /mcp speaks and tools as
// the only capability. The scope is private because the tools a caller sees
// depend on its auth context, and ttlMs is 0 because servers come and go with
// their health.
func discoverResult() mcp.DiscoverResult {
	listChanged := false
	capabilities := mcp.ServerCapabilities{}
	capabilities.Tools = &struct {
		ListChanged *bool `json:"listChanged,omitempty"`
	}{ListChanged: &listChanged}

	return mcp.DiscoverResult{
		ResultType:        mcp.ResultTypeComplete,
		SupportedVersions: []string{mcp.ProtocolVersion},
		Capabilities:      capabilities,
		CacheScope:        mcp.DiscoverResultCacheScopePrivate,
	}
}

// mcpToolsList aggregates the tools of every healthy MCP server under their
// namespaced mcp_<alias>_<tool> names. An unavailable server is skipped rather
// than failing the whole call, and MCP_INCLUDE_TOOLS/MCP_EXCLUDE_TOOLS apply
// exactly as they do to the tools injected into chat completions.
func (router *RouterImpl) mcpToolsList() mcp.ListToolsResult {
	tools := make([]mcp.Tool, 0)

	if router.mcpClient != nil && router.mcpClient.IsInitialized() {
		statuses := router.mcpClient.GetAllServerStatuses()
		for _, alias := range router.mcpClient.GetServers() {
			if statuses[alias] == mcp.ServerStatusUnavailable {
				router.logger.Debug("skipping unavailable mcp server", "server", alias)
				continue
			}

			serverTools, err := router.mcpClient.GetServerTools(alias)
			if err != nil {
				router.logger.Error("failed to get tools from mcp server", err, "server", alias)
				continue
			}

			for _, tool := range serverTools {
				if !mcp.IsToolAllowed(alias, tool.Name, router.cfg.MCP.IncludeTools, router.cfg.MCP.ExcludeTools) {
					continue
				}
				if tool.InputSchema == nil {
					tool.InputSchema = make(map[string]any)
				}
				tool.Name = mcp.NamespacedToolName(alias, tool.Name)
				tools = append(tools, tool)
			}
		}
	}

	return mcp.ListToolsResult{
		Tools:      tools,
		ResultType: mcp.ResultTypeComplete,
		CacheScope: mcp.ListToolsResultCacheScopePrivate,
	}
}

// mcpToolsCall hands an advertised, allowed tool to the agent, which runs the
// same guardrails, span and counter as the chat-completions loop. Unknown names
// never reach it, and upstream failures come back as JSON-RPC errors.
func (router *RouterImpl) mcpToolsCall(c *gin.Context, req *types.MCPJSONRPCRequest) {
	if router.mcpClient == nil || !router.mcpClient.IsInitialized() || router.mcpAgent == nil {
		router.logger.Error("mcp tools/call with no usable mcp client", nil)
		router.respondMCPError(c, req, jsonRPCInternalError, errMsgMCPUnusable)
		return
	}

	params := map[string]any{}
	if req.Params != nil {
		params = *req.Params
	}

	name, _ := params["name"].(string)
	alias, toolName, err := router.mcpClient.ResolveTool(name)
	if err != nil {
		router.logger.Error("failed to resolve mcp tool", err, "tool", name)
		router.respondMCPError(c, req, jsonRPCInvalidParams, "unknown tool: "+name)
		return
	}
	// ResolveTool falls back to the longest matching alias for a name it never
	// discovered; /mcp only serves the tools tools/list advertised.
	serverTools, _ := router.mcpClient.GetServerTools(alias)
	if !slices.ContainsFunc(serverTools, func(tool mcp.Tool) bool { return tool.Name == toolName }) {
		router.logger.Error("mcp tool call for a tool the server never listed", nil, "tool", name, "server", alias)
		router.respondMCPError(c, req, jsonRPCInvalidParams, "unknown tool: "+name)
		return
	}
	if !mcp.IsToolAllowed(alias, toolName, router.cfg.MCP.IncludeTools, router.cfg.MCP.ExcludeTools) {
		router.logger.Error("mcp tool call rejected by include/exclude config", nil, "tool", name, "server", alias)
		router.respondMCPError(c, req, jsonRPCInvalidParams, "unknown tool: "+name)
		return
	}
	if router.mcpClient.GetAllServerStatuses()[alias] == mcp.ServerStatusUnavailable {
		router.logger.Error("mcp tool call routed to an unavailable server", nil, "tool", name, "server", alias)
		router.respondMCPError(c, req, jsonRPCInternalError, "mcp server "+alias+" is unavailable")
		return
	}

	arguments, _ := params["arguments"].(map[string]any)
	if arguments == nil {
		arguments = make(map[string]any)
	}
	argsJSON, err := json.Marshal(arguments)
	if err != nil {
		router.logger.Error("failed to encode mcp tool arguments", err, "tool", name)
		router.respondMCPError(c, req, jsonRPCInvalidParams, "invalid arguments for tool: "+name)
		return
	}

	router.logger.Debug("executing mcp tool call", "tool", toolName, "server", alias)
	result, err := router.mcpAgent.ExecuteToolCall(c.Request.Context(), name, string(argsJSON), arguments)
	if err != nil {
		var blocked *guardrails.BlockedError
		if errors.As(err, &blocked) {
			router.respondMCPError(c, req, middlewares.JSONRPCGuardrailBlocked, blocked.Message)
			return
		}
		// The upstream error can name internal hosts, so it stays in the log.
		router.logger.Error("mcp tool call failed", err, "tool", toolName, "server", alias)
		router.respondMCPError(c, req, jsonRPCInternalError, "mcp server "+alias+" failed")
		return
	}
	if result == nil {
		router.respondMCPError(c, req, jsonRPCInternalError, "mcp server "+alias+" returned no result")
		return
	}

	router.respondMCPResult(c, req, result)
}

// respondMCPResult writes a JSON-RPC success envelope around an MCP result,
// naming the gateway as the server that produced it.
func (router *RouterImpl) respondMCPResult(c *gin.Context, req *types.MCPJSONRPCRequest, result any) {
	payload, err := mcpResultObject(result)
	if err != nil {
		router.logger.Error("failed to encode mcp result", err, "method", string(req.Method))
		router.respondMCPError(c, req, jsonRPCInternalError, "failed to encode result")
		return
	}

	meta, _ := (*payload)["_meta"].(map[string]any)
	if meta == nil {
		meta = make(map[string]any)
	}
	meta[metaServerInfo] = mcp.GatewayInfo
	(*payload)["_meta"] = meta

	c.JSON(http.StatusOK, types.MCPJSONRPCResponse{
		Jsonrpc: types.MCPJSONRPCResponseJsonrpcN20,
		ID:      mcpResponseID(req),
		Result:  payload,
	})
}

// respondMCPError writes a JSON-RPC error envelope with no error data.
func (router *RouterImpl) respondMCPError(c *gin.Context, req *types.MCPJSONRPCRequest, code int, message string) {
	router.writeMCPError(c, req, &types.MCPJSONRPCError{Code: code, Message: message})
}

// writeMCPError writes a JSON-RPC error envelope with HTTP 200, except 400 for
// header and version failures and 404 for an unknown method (both pinned by
// 2026-07-28), and 403 for a guardrails block, like every other blocked route.
func (router *RouterImpl) writeMCPError(c *gin.Context, req *types.MCPJSONRPCRequest, rpcErr *types.MCPJSONRPCError) {
	status := http.StatusOK
	switch rpcErr.Code {
	case jsonRPCHeaderMismatch, jsonRPCUnsupportedVersion:
		status = http.StatusBadRequest
	case jsonRPCMethodNotFound:
		status = http.StatusNotFound
	case middlewares.JSONRPCGuardrailBlocked:
		status = http.StatusForbidden
	}

	c.JSON(status, types.MCPJSONRPCResponse{
		Jsonrpc: types.MCPJSONRPCResponseJsonrpcN20,
		ID:      mcpResponseID(req),
		Error:   rpcErr,
	})
}

// mcpResponseID echoes the request id back. The zero value marshals to null,
// which is what JSON-RPC wants for an error that cannot be attributed to a
// request.
func mcpResponseID(req *types.MCPJSONRPCRequest) types.MCPJSONRPCResponse_ID {
	var id types.MCPJSONRPCResponse_ID
	if req == nil || req.ID == nil {
		return id
	}
	raw, err := req.ID.MarshalJSON()
	if err != nil {
		return id
	}
	_ = id.UnmarshalJSON(raw)
	return id
}

// mcpResultObject renders a typed MCP result into the generic result object of
// the generated response envelope.
func mcpResultObject(result any) (*map[string]any, error) {
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	return &object, nil
}
