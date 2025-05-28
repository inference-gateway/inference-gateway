package middlewares

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	config "github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/mcp"
	"github.com/inference-gateway/inference-gateway/providers"
)

const (
	// ChatCompletionsPath is the endpoint path for chat completions
	ChatCompletionsPath = "/v1/chat/completions"

	// MCPInternalHeader marks internal MCP requests to prevent middleware loops
	MCPInternalHeader = "X-MCP-Internal"

	// MaxAgentIterations limits the number of agent loop iterations
	MaxAgentIterations = 10
)

// contextKey is a custom type for context keys to avoid collisions
type mcpContextKey string

const (
	// mcpInternalKey is the context key for marking internal MCP requests
	mcpInternalKey mcpContextKey = MCPInternalHeader
)

// customResponseWriter captures the response body but doesn't write it
// to the client until we're ready, allowing us to intercept tool calls
type customResponseWriter struct {
	gin.ResponseWriter
	body          *bytes.Buffer
	statusCode    int
	writeToClient bool
}

// WriteHeader captures the status code but doesn't write it to the client
// unless writeToClient is true
func (w *customResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	if w.writeToClient {
		w.ResponseWriter.WriteHeader(code)
	}
}

// Write captures the response body but doesn't write it to the client
// unless writeToClient is true
func (w *customResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	if w.writeToClient {
		return w.ResponseWriter.Write(b)
	}
	return len(b), nil
}

// MCPMiddleware defines the interface for MCP middleware
type MCPMiddleware interface {
	Middleware() gin.HandlerFunc
}

// MCPMiddlewareImpl implements the MCP middleware
type MCPMiddlewareImpl struct {
	registry               providers.ProviderRegistry
	inferenceGatewayClient providers.Client
	mcpClient              mcp.MCPClientInterface
	logger                 logger.Logger
	cfg                    config.Config
}

// NoopMCPMiddlewareImpl is a no-op implementation of MCPMiddleware
type NoopMCPMiddlewareImpl struct{}

// NewMCPMiddleware creates a new MCP middleware instance
func NewMCPMiddleware(registry providers.ProviderRegistry, inferenceGatewayClient providers.Client, mcpClient mcp.MCPClientInterface, logger logger.Logger, cfg config.Config) (MCPMiddleware, error) {
	if mcpClient == nil {
		logger.Info("MCP client is nil, using no-op middleware")
		return &NoopMCPMiddlewareImpl{}, nil
	}

	return &MCPMiddlewareImpl{
		registry:               registry,
		inferenceGatewayClient: inferenceGatewayClient,
		mcpClient:              mcpClient,
		logger:                 logger,
		cfg:                    cfg,
	}, nil
}

// Middleware returns the no-op middleware handler
func (n *NoopMCPMiddlewareImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// Middleware returns the MCP middleware handler
func (m *MCPMiddlewareImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader(MCPInternalHeader) != "" {
			c.Next()
			return
		}

		if c.Request.URL.Path != ChatCompletionsPath {
			c.Next()
			return
		}

		m.logger.Debug("MCP middleware invoked", "path", c.Request.URL.Path)

		originalRequest, err := m.parseRequest(c)
		if err != nil {
			m.logger.Error("Failed to parse request body", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		m.addMCPToolsToRequest(c, originalRequest)

		c.Set(string(mcpInternalKey), originalRequest)

		result, err := m.getProviderAndModel(originalRequest.Model)
		if err != nil {
			if result.ProviderID == nil {
				m.logger.Error("Failed to determine provider", err, "model", originalRequest.Model)
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported model: %s", originalRequest.Model)})
			} else {
				m.logger.Error("Failed to get provider", err, "provider", *result.ProviderID)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Provider not available"})
			}
			return
		}

		customWriter := &customResponseWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
			statusCode:     http.StatusOK,
			writeToClient:  false,
		}
		c.Writer = customWriter

		c.Next()

		ctx := c.Request.Context()
		if originalRequest.Stream != nil && *originalRequest.Stream {
			err = m.processStreamingResponse(ctx, customWriter, originalRequest, result.Provider, result.ProviderModel)
		} else {
			err = m.processNonStreamingResponse(ctx, customWriter, originalRequest, result.Provider, result.ProviderModel)
		}

		if err != nil {
			m.logger.Error("Failed to process response", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process response"})
			return
		}
	}
}

// parseRequest parses the incoming request body into a chat completion request
func (m *MCPMiddlewareImpl) parseRequest(c *gin.Context) (*providers.CreateChatCompletionRequest, error) {
	var originalRequest providers.CreateChatCompletionRequest
	if err := c.ShouldBindJSON(&originalRequest); err != nil {
		return nil, err
	}
	return &originalRequest, nil
}

// addMCPToolsToRequest adds available MCP tools to the request if any are available
func (m *MCPMiddlewareImpl) addMCPToolsToRequest(c *gin.Context, request *providers.CreateChatCompletionRequest) {
	if !m.mcpClient.IsInitialized() {
		return
	}

	availableTools := m.mcpClient.GetAllChatCompletionTools()
	if len(availableTools) == 0 {
		return
	}

	m.logger.Debug("Added MCP tools to request", "toolCount", len(availableTools))

	request.Tools = &availableTools
}

// ProviderModelResult contains the result of provider and model determination
type ProviderModelResult struct {
	Provider      providers.IProvider
	ProviderModel string
	ProviderID    *providers.Provider
}

// getProviderAndModel determines the provider and model from the request model string
func (m *MCPMiddlewareImpl) getProviderAndModel(model string) (*ProviderModelResult, error) {
	providerPtr, providerModel := providers.DetermineProviderAndModelName(model)
	if providerPtr == nil {
		return &ProviderModelResult{ProviderID: nil}, fmt.Errorf("unable to determine provider for model: %s", model)
	}

	provider, err := m.registry.BuildProvider(*providerPtr, m.inferenceGatewayClient)
	if err != nil {
		return &ProviderModelResult{ProviderID: providerPtr}, fmt.Errorf("failed to build provider: %w", err)
	}

	return &ProviderModelResult{
		Provider:      provider,
		ProviderModel: providerModel,
		ProviderID:    providerPtr,
	}, nil
}

// processNonStreamingResponse handles non-streaming chat completion responses with agent loop
func (m *MCPMiddlewareImpl) processNonStreamingResponse(ctx context.Context, w *customResponseWriter, originalRequest *providers.CreateChatCompletionRequest, provider providers.IProvider, providerModel string) error {
	responseBody := w.body.String()
	if responseBody == "" {
		m.writeFallbackResponse(w)
		return nil
	}

	var response providers.CreateChatCompletionResponse
	if err := json.Unmarshal(w.body.Bytes(), &response); err != nil {
		m.logger.Error("Failed to parse response", err)
		m.writeFallbackResponse(w)
		return nil
	}

	if len(response.Choices) == 0 || response.Choices[0].Message.ToolCalls == nil || len(*response.Choices[0].Message.ToolCalls) == 0 {
		m.writeFallbackResponse(w)
		return nil
	}

	finalResponse, err := m.executeAgentLoop(ctx, originalRequest, &response, provider, providerModel)
	if err != nil {
		m.logger.Error("Failed to execute agent loop", err)
		return m.writeResponse(w, response)
	}

	return m.writeResponse(w, finalResponse)
}

// processStreamingResponse handles streaming chat completion responses with agent loop
func (m *MCPMiddlewareImpl) processStreamingResponse(ctx context.Context, w *customResponseWriter, originalRequest *providers.CreateChatCompletionRequest, provider providers.IProvider, providerModel string) error {
	responseBody := w.body.String()
	if responseBody == "" {
		m.writeFallbackResponse(w)
		return nil
	}

	toolCalls, err := m.parseStreamingToolCalls(responseBody)
	if err != nil {
		m.logger.Error("Failed to parse streaming tool calls", err)
		m.writeFallbackResponse(w)
		return nil
	}

	if len(toolCalls) == 0 {
		m.writeFallbackResponse(w)
		return nil
	}

	return m.executeStreamingAgentLoop(ctx, w, originalRequest, responseBody, provider, providerModel)
}

// parseStreamingToolCalls parses streaming response to extract tool calls
func (m *MCPMiddlewareImpl) parseStreamingToolCalls(responseBody string) ([]providers.ChatCompletionMessageToolCall, error) {
	toolCallsMap := make(map[int]*providers.ChatCompletionMessageToolCall)
	lines := strings.Split(responseBody, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk providers.CreateChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) == 0 || chunk.Choices[0].Delta.ToolCalls == nil {
			continue
		}

		for _, toolCallChunk := range *chunk.Choices[0].Delta.ToolCalls {
			index := toolCallChunk.Index

			if _, exists := toolCallsMap[index]; !exists {
				toolCallsMap[index] = &providers.ChatCompletionMessageToolCall{
					ID:   "",
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "",
						Arguments: "",
					},
				}
			}

			toolCall := toolCallsMap[index]

			if toolCallChunk.ID != nil {
				toolCall.ID = *toolCallChunk.ID
			}

			if toolCallChunk.Type != nil {
				toolCall.Type = providers.ChatCompletionToolType(*toolCallChunk.Type)
			}
			if toolCallChunk.Function != nil {
				type TempToolCallFunction struct {
					Name      string `json:"name,omitempty"`
					Arguments string `json:"arguments,omitempty"`
				}
				type TempToolCall struct {
					Index    int                  `json:"index"`
					Function TempToolCallFunction `json:"function"`
				}
				type TempChoice struct {
					Delta struct {
						ToolCalls []TempToolCall `json:"tool_calls"`
					} `json:"delta"`
				}
				type TempResponse struct {
					Choices []TempChoice `json:"choices"`
				}

				var tempResp TempResponse
				if err := json.Unmarshal([]byte(data), &tempResp); err == nil {
					if len(tempResp.Choices) > 0 {
						for _, tc := range tempResp.Choices[0].Delta.ToolCalls {
							if tc.Index == index {
								if tc.Function.Name != "" {
									toolCall.Function.Name = tc.Function.Name
									m.logger.Debug("Parsed tool name from stream", "name", tc.Function.Name)
								}
								if tc.Function.Arguments != "" {
									toolCall.Function.Arguments += tc.Function.Arguments
									m.logger.Debug("Parsed tool arguments from stream", "args", tc.Function.Arguments)
								}
							}
						}
					}
				}
			}
		}
	}

	var toolCalls []providers.ChatCompletionMessageToolCall
	for i := 0; i < len(toolCallsMap); i++ {
		if toolCall, exists := toolCallsMap[i]; exists {
			m.logger.Debug("Final parsed tool call", "toolCall", fmt.Sprintf("id=%s name=%s args=%s", toolCall.ID, toolCall.Function.Name, toolCall.Function.Arguments))
			toolCalls = append(toolCalls, *toolCall)
		}
	}

	m.logger.Debug("Total parsed tool calls", "count", len(toolCalls))
	return toolCalls, nil
}

// extractAssistantMessageFromStream extracts the assistant message from streaming response
func (m *MCPMiddlewareImpl) extractAssistantMessageFromStream(responseBody string, toolCalls []providers.ChatCompletionMessageToolCall) (providers.Message, error) {
	content := ""
	lines := strings.Split(responseBody, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk providers.CreateChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 {
			content += chunk.Choices[0].Delta.Content
		}
	}

	message := providers.Message{
		Role:    providers.MessageRoleAssistant,
		Content: content,
	}

	if len(toolCalls) > 0 {
		m.logger.Debug("Adding tool calls to assistant message", "toolCallCount", len(toolCalls))
		for i, tc := range toolCalls {
			m.logger.Debug("Tool call in assistant message", "index", i, "id", tc.ID, "name", tc.Function.Name, "argsLength", len(tc.Function.Arguments))
		}
		message.ToolCalls = &toolCalls
	}

	m.logger.Debug("Extracted assistant message", "contentLength", len(content), "hasToolCalls", message.ToolCalls != nil)
	return message, nil
}

// executeToolCalls executes the tool calls using the MCP client
func (m *MCPMiddlewareImpl) executeToolCalls(ctx context.Context, toolCalls []providers.ChatCompletionMessageToolCall) ([]providers.Message, error) {
	var results []providers.Message

	for _, toolCall := range toolCalls {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			m.logger.Error("Failed to parse tool arguments", err, "args", toolCall.Function.Arguments)
			results = append(results, providers.Message{
				Role:       providers.MessageRoleTool,
				Content:    fmt.Sprintf("Error: Failed to parse arguments: %v", err),
				ToolCallId: &toolCall.ID,
			})
			continue
		}

		var server string
		if mcpServer, ok := args["mcpServer"].(string); ok && mcpServer != "" {
			server = mcpServer
		}

		delete(args, "mcpServer")

		mcpRequest := mcp.Request{
			Method: "tools/call",
			Params: map[string]interface{}{
				"name":      toolCall.Function.Name,
				"arguments": args,
			},
		}

		result, err := m.mcpClient.ExecuteTool(ctx, mcpRequest, server)
		if err != nil {
			m.logger.Error("Failed to execute tool call", err, "tool", toolCall.Function.Name)
			results = append(results, providers.Message{
				Role:       providers.MessageRoleTool,
				Content:    fmt.Sprintf("Error: %v", err),
				ToolCallId: &toolCall.ID,
			})
			continue
		}

		var resultStr string
		if result == nil {
			resultStr = "null"
		} else {
			resultBytes, err := json.Marshal(result)
			if err != nil {
				resultStr = fmt.Sprintf("Error marshaling result: %v", err)
			} else {
				resultStr = string(resultBytes)
			}
		}

		results = append(results, providers.Message{
			Role:       providers.MessageRoleTool,
			Content:    resultStr,
			ToolCallId: &toolCall.ID,
		})
	}

	return results, nil
}

// writeResponse writes a response to the custom response writer
func (m *MCPMiddlewareImpl) writeResponse(w *customResponseWriter, response interface{}) error {
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return err
	}

	w.ResponseWriter.Header().Set("Content-Type", "application/json")
	w.ResponseWriter.WriteHeader(http.StatusOK)
	_, err = w.ResponseWriter.Write(responseBytes)
	return err
}

// writeFallbackResponse writes the original response when processing fails
func (m *MCPMiddlewareImpl) writeFallbackResponse(w *customResponseWriter) {
	w.writeToClient = true
	w.ResponseWriter.WriteHeader(w.statusCode)
	if _, err := w.ResponseWriter.Write(w.body.Bytes()); err != nil {
		m.logger.Error("Failed to write fallback response", err)
	}
}

// executeAgentLoop executes the agent loop for non-streaming responses
func (m *MCPMiddlewareImpl) executeAgentLoop(ctx context.Context, request *providers.CreateChatCompletionRequest, response *providers.CreateChatCompletionResponse, provider providers.IProvider, providerModel string) (*providers.CreateChatCompletionResponse, error) {
	currentRequest := *request
	currentResponse := *response
	iteration := 0

	for iteration < MaxAgentIterations {
		if len(currentResponse.Choices) == 0 || currentResponse.Choices[0].Message.ToolCalls == nil || len(*currentResponse.Choices[0].Message.ToolCalls) == 0 {
			break
		}

		m.logger.Debug("Agent loop iteration", "iteration", iteration+1, "toolCalls", len(*currentResponse.Choices[0].Message.ToolCalls))

		toolResults, err := m.executeToolCalls(ctx, *currentResponse.Choices[0].Message.ToolCalls)
		if err != nil {
			m.logger.Error("Failed to execute tool calls", err)
			return &currentResponse, nil
		}

		currentRequest.Messages = append(currentRequest.Messages, currentResponse.Choices[0].Message)
		currentRequest.Messages = append(currentRequest.Messages, toolResults...)

		internalCtx := context.WithValue(ctx, mcpInternalKey, "true")
		currentRequest.Model = providerModel
		nextResponse, err := provider.ChatCompletions(internalCtx, currentRequest)
		if err != nil {
			m.logger.Error("Failed to get response in agent loop", err)
			return &currentResponse, nil
		}

		currentResponse = nextResponse
		iteration++
	}

	if iteration >= MaxAgentIterations {
		m.logger.Error("Agent loop reached maximum iterations", fmt.Errorf("max iterations reached: %d", MaxAgentIterations))
	}

	return &currentResponse, nil
}

// executeStreamingAgentLoop executes the agent loop for streaming responses
func (m *MCPMiddlewareImpl) executeStreamingAgentLoop(ctx context.Context, w *customResponseWriter, request *providers.CreateChatCompletionRequest, responseBody string, provider providers.IProvider, providerModel string) error {
	toolCalls, err := m.parseStreamingToolCalls(responseBody)
	if err != nil {
		return err
	}

	if len(toolCalls) == 0 {
		return nil
	}

	currentRequest := *request
	iteration := 0

	m.writeStreamingHeaders(w)

	if err := m.writeInitialStreamingResponse(w, responseBody); err != nil {
		m.logger.Error("Failed to write initial streaming response", err)
		return err
	}

	for iteration < MaxAgentIterations {
		if len(toolCalls) == 0 {
			break
		}

		m.logger.Debug("Streaming agent loop iteration", "iteration", iteration+1, "toolCalls", len(toolCalls))

		toolResults, err := m.executeToolCalls(ctx, toolCalls)
		if err != nil {
			m.logger.Error("Failed to execute tool calls in streaming loop", err)
			break
		}

		assistantMessage, err := m.extractAssistantMessageFromStream(responseBody, toolCalls)
		if err != nil {
			m.logger.Error("Failed to extract assistant message in streaming loop", err)
			break
		}

		currentRequest.Messages = append(currentRequest.Messages, assistantMessage)
		currentRequest.Messages = append(currentRequest.Messages, toolResults...)

		m.logger.Debug("Updated request for streaming tool call continuation", "messageCount", len(currentRequest.Messages))

		currentRequest.Model = providerModel
		streamCh, err := provider.StreamChatCompletions(ctx, currentRequest)
		if err != nil {
			m.logger.Error("Failed to stream chat completion in agent loop", err)
			break
		}

		var nextResponseBody strings.Builder
		hasContent := false

		for chunk := range streamCh {
			if ctx.Err() != nil {
				m.logger.Debug("Client disconnected during streaming", "error", ctx.Err())
				break
			}

			if _, err := w.ResponseWriter.Write([]byte("data: " + string(chunk) + "\n\n")); err != nil {
				m.logger.Error("Failed to write chunk in streaming loop", err)
				break
			}

			if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
				flusher.Flush()
			}

			nextResponseBody.WriteString("data: " + string(chunk) + "\n")
			hasContent = true
		}

		if !hasContent {
			break
		}

		responseBody = nextResponseBody.String()
		toolCalls, err = m.parseStreamingToolCalls(responseBody)
		if err != nil {
			m.logger.Error("Failed to parse streaming tool calls in agent loop", err)
			break
		}

		iteration++
	}

	if iteration >= MaxAgentIterations {
		m.logger.Error("Streaming agent loop reached maximum iterations", fmt.Errorf("max iterations reached: %d", MaxAgentIterations))
	}

	m.finishStreamingResponse(w)

	return nil
}

// writeStreamingHeaders sets up the necessary headers for streaming responses
func (m *MCPMiddlewareImpl) writeStreamingHeaders(w *customResponseWriter) {
	w.ResponseWriter.Header().Set("Content-Type", "text/event-stream")
	w.ResponseWriter.Header().Set("Cache-Control", "no-cache")
	w.ResponseWriter.Header().Set("Connection", "keep-alive")
	w.ResponseWriter.Header().Set("Access-Control-Allow-Origin", "*")
	w.ResponseWriter.Header().Set("Access-Control-Allow-Headers", "Cache-Control")
	w.ResponseWriter.WriteHeader(http.StatusOK)
}

// writeInitialStreamingResponse writes the initial streaming response to the client
func (m *MCPMiddlewareImpl) writeInitialStreamingResponse(w *customResponseWriter, responseBody string) error {
	initialResponseLines := strings.Split(responseBody, "\n")
	for _, line := range initialResponseLines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "data: ") {
			if _, err := w.ResponseWriter.Write([]byte("data: " + line + "\n\n")); err != nil {
				return err
			}
		} else if strings.HasPrefix(line, "data: ") {
			if _, err := w.ResponseWriter.Write([]byte(line + "\n\n")); err != nil {
				return err
			}
		}
	}

	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}

	return nil
}

// finishStreamingResponse writes the final streaming response markers
func (m *MCPMiddlewareImpl) finishStreamingResponse(w *customResponseWriter) {
	if _, err := w.ResponseWriter.Write([]byte("data: [DONE]\n\n")); err != nil {
		m.logger.Error("Failed to write done marker", err)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
