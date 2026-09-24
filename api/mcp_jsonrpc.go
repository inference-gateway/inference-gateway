package api

import (
	"encoding/json"
	"net/http"
	"slices"

	gin "github.com/gin-gonic/gin"

	mcp "github.com/inference-gateway/inference-gateway/internal/mcp"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// Version is the gateway version reported as the MCP serverInfo version.
// main overwrites it with the build version at startup.
var Version = "dev"

// JSON-RPC 2.0 error codes, as listed in the /mcp spec.
const (
	jsonRPCParseError     = -32700
	jsonRPCInvalidRequest = -32600
	jsonRPCMethodNotFound = -32601
	jsonRPCInvalidParams  = -32602
	jsonRPCInternalError  = -32603
)

const (
	// mcpServerName is the serverInfo name MCP clients see for the gateway.
	mcpServerName = "inference-gateway"

	// defaultProtocolVersion is the MCP protocol version reported when the
	// client asks for one the gateway does not know.
	defaultProtocolVersion = "2025-06-18"

	// resultTypeComplete marks a result as final rather than partial.
	resultTypeComplete = "complete"

	errMsgMCPNotExposed = "MCP endpoint is not exposed. Set MCP_EXPOSE=true to enable."
	errMsgMCPUnusable   = "no mcp servers are available"
	errMsgParse         = "parse error"
	errMsgInvalidReq    = "invalid request: jsonrpc must be \"2.0\" and method is required"
)

// supportedProtocolVersions are the MCP protocol revisions the gateway answers
// with verbatim; anything else is answered with defaultProtocolVersion so the
// client can decide whether to continue.
var supportedProtocolVersions = []string{"2024-11-05", "2025-03-26", defaultProtocolVersion}

// mcpInitializeResult is the `initialize` result. The vendored MCP schema
// models the handshake-free draft, so the three fields clients expect are
// assembled from the spec types here.
type mcpInitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    mcp.ServerCapabilities `json:"capabilities"`
	ServerInfo      mcp.Implementation     `json:"serverInfo"`
}

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

	// A request without an id is a notification: accept it, answer nothing.
	// notifications/initialized is the one clients send after the handshake.
	if req.ID == nil {
		router.logger.Debug("mcp notification accepted", "method", string(req.Method))
		c.Status(http.StatusAccepted)
		return
	}

	switch req.Method {
	case types.Initialize:
		router.respondMCPResult(c, &req, initializeResult(req.Params))
	case types.NotificationsInitialized:
		// Belt and braces: the method is a notification, but a client that
		// sends it with an id is owed a response rather than "method not found".
		router.respondMCPResult(c, &req, struct{}{})
	case types.ToolsList:
		router.respondMCPResult(c, &req, router.mcpToolsList())
	case types.ToolsCall:
		router.mcpToolsCall(c, &req)
	default:
		router.logger.Error("unsupported mcp method", nil, "method", string(req.Method))
		router.respondMCPError(c, &req, jsonRPCMethodNotFound, "method not found: "+string(req.Method))
	}
}

// initializeResult echoes back the client's protocol version when the gateway
// knows it, and advertises tools as the only supported capability.
func initializeResult(params *map[string]any) mcpInitializeResult {
	protocolVersion := defaultProtocolVersion
	if params != nil {
		if requested, ok := (*params)["protocolVersion"].(string); ok && slices.Contains(supportedProtocolVersions, requested) {
			protocolVersion = requested
		}
	}

	listChanged := false
	capabilities := mcp.ServerCapabilities{}
	capabilities.Tools = &struct {
		ListChanged *bool `json:"listChanged,omitempty"`
	}{ListChanged: &listChanged}

	return mcpInitializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities:    capabilities,
		ServerInfo:      mcp.Implementation{Name: mcpServerName, Version: Version},
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
		ResultType: resultTypeComplete,
		CacheScope: mcp.ListToolsResultCacheScopePrivate,
	}
}

// mcpToolsCall resolves the namespaced tool name to its server and executes it
// there. Upstream failures come back as JSON-RPC errors, never as a 500.
func (router *RouterImpl) mcpToolsCall(c *gin.Context, req *types.MCPJSONRPCRequest) {
	if router.mcpClient == nil || !router.mcpClient.IsInitialized() {
		router.logger.Error("mcp tools/call with no usable mcp client", nil)
		router.respondMCPError(c, req, jsonRPCInternalError, errMsgMCPUnusable)
		return
	}

	params := map[string]any{}
	if req.Params != nil {
		params = *req.Params
	}

	name, _ := params["name"].(string)
	if name == "" {
		router.respondMCPError(c, req, jsonRPCInvalidParams, "tools/call requires a 'name' parameter")
		return
	}

	alias, toolName, err := router.mcpClient.ResolveTool(name)
	if err != nil {
		router.logger.Error("failed to resolve mcp tool", err, "tool", name)
		router.respondMCPError(c, req, jsonRPCInvalidParams, "unknown tool: "+name)
		return
	}
	// A tool the include/exclude lists hide is not callable either, and stays
	// indistinguishable from one that does not exist.
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

	router.logger.Debug("executing mcp tool call", "tool", toolName, "server", alias)
	result, err := router.mcpClient.ExecuteTool(c.Request.Context(), mcp.Request{
		Method: string(types.ToolsCall),
		Params: map[string]any{"name": toolName, "arguments": arguments},
	}, alias)
	if err != nil {
		router.logger.Error("mcp tool call failed", err, "tool", toolName, "server", alias)
		router.respondMCPError(c, req, jsonRPCInternalError, err.Error())
		return
	}
	if result == nil {
		router.respondMCPError(c, req, jsonRPCInternalError, "mcp server "+alias+" returned no result")
		return
	}

	if result.ResultType == "" {
		result.ResultType = resultTypeComplete
	}
	router.respondMCPResult(c, req, result)
}

// respondMCPResult writes a JSON-RPC success envelope around an MCP result.
func (router *RouterImpl) respondMCPResult(c *gin.Context, req *types.MCPJSONRPCRequest, result any) {
	payload, err := mcpResultObject(result)
	if err != nil {
		router.logger.Error("failed to encode mcp result", err, "method", string(req.Method))
		router.respondMCPError(c, req, jsonRPCInternalError, "failed to encode result")
		return
	}

	c.JSON(http.StatusOK, types.MCPJSONRPCResponse{
		Jsonrpc: types.MCPJSONRPCResponseJsonrpcN20,
		ID:      mcpResponseID(req),
		Result:  payload,
	})
}

// respondMCPError writes a JSON-RPC error envelope. Protocol errors travel with
// HTTP 200; only transport-level failures (auth, feature flag) use a status code.
func (router *RouterImpl) respondMCPError(c *gin.Context, req *types.MCPJSONRPCRequest, code int, message string) {
	c.JSON(http.StatusOK, types.MCPJSONRPCResponse{
		Jsonrpc: types.MCPJSONRPCResponseJsonrpcN20,
		ID:      mcpResponseID(req),
		Error:   &types.MCPJSONRPCError{Code: code, Message: message},
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
