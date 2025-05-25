package middlewares

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	config "github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/mcp"
	"github.com/inference-gateway/inference-gateway/providers"
)

// contextKey is a custom type for context keys to avoid collisions
type mcpContextKey string

const (
	// mcpInternalKey is the context key for marking internal MCP requests
	mcpInternalKey mcpContextKey = "X-MCP-Internal"
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

const ChatCompletionsPath = "/v1/chat/completions"

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
		if c.GetHeader("X-MCP-Internal") != "" {
			c.Next()
			return
		}

		if c.Request.URL.Path != ChatCompletionsPath {
			c.Next()
			return
		}

		m.logger.Debug("MCP middleware invoked", "path", c.Request.URL.Path)

		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			m.logger.Error("Failed to read request body", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		var originalRequest providers.CreateChatCompletionRequest
		if err := json.Unmarshal(bodyBytes, &originalRequest); err != nil {
			m.logger.Error("Failed to parse request body", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		if m.mcpClient.IsInitialized() {
			c.Set("use_mcp", true)

			mcpTools := m.mcpClient.GetAllChatCompletionTools()
			if len(mcpTools) > 0 {
				if originalRequest.Tools == nil {
					originalRequest.Tools = &mcpTools
				} else {
					existingTools := *originalRequest.Tools
					existingTools = append(existingTools, mcpTools...)
					originalRequest.Tools = &existingTools
				}

				m.logger.Debug("Added MCP tools to request", "toolCount", len(mcpTools))

				modifiedBodyBytes, err := json.Marshal(originalRequest)
				if err != nil {
					m.logger.Error("Failed to marshal modified request", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process request"})
					return
				}

				c.Request.Body = io.NopCloser(bytes.NewBuffer(modifiedBodyBytes))
			}
		}

		providerPtr, providerModel := providers.DetermineProviderAndModelName(originalRequest.Model)
		if providerPtr == nil {
			m.logger.Error("Failed to determine provider", fmt.Errorf("unsupported model: %s", originalRequest.Model), "model", originalRequest.Model)
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported model: %s", originalRequest.Model)})
			return
		}
		providerID := *providerPtr

		provider, err := m.registry.BuildProvider(providerID, m.inferenceGatewayClient)
		if err != nil {
			m.logger.Error("Failed to get provider", err, "provider", providerID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Provider not available"})
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
			err = m.processStreamingResponse(ctx, customWriter, &originalRequest, provider, providerModel)
		} else {
			err = m.processNonStreamingResponse(ctx, customWriter, &originalRequest, provider, providerModel)
		}

		if err != nil {
			m.logger.Error("Failed to process response", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process response"})
			return
		}
	}
}

// processNonStreamingResponse handles non-streaming chat completion responses with agent loop
func (m *MCPMiddlewareImpl) processNonStreamingResponse(ctx context.Context, w *customResponseWriter, originalRequest *providers.CreateChatCompletionRequest, provider providers.IProvider, providerModel string) error {
	responseBody := w.body.String()
	if responseBody == "" {
		w.writeToClient = true
		w.ResponseWriter.WriteHeader(w.statusCode)
		return nil
	}

	var response providers.CreateChatCompletionResponse
	if err := json.Unmarshal(w.body.Bytes(), &response); err != nil {
		m.logger.Error("Failed to parse response", err)
		w.writeToClient = true
		w.ResponseWriter.WriteHeader(w.statusCode)
		if _, err := w.ResponseWriter.Write(w.body.Bytes()); err != nil {
			m.logger.Error("Failed to write response", err)
		}
		return nil
	}

	if len(response.Choices) == 0 || response.Choices[0].Message.ToolCalls == nil || len(*response.Choices[0].Message.ToolCalls) == 0 {
		w.writeToClient = true
		w.ResponseWriter.WriteHeader(w.statusCode)
		if _, err := w.ResponseWriter.Write(w.body.Bytes()); err != nil {
			m.logger.Error("Failed to write response", err)
		}
		return nil
	}

	currentRequest := *originalRequest
	currentResponse := response
	maxIterations := 10
	iteration := 0

	for iteration < maxIterations {
		if len(currentResponse.Choices) == 0 || currentResponse.Choices[0].Message.ToolCalls == nil || len(*currentResponse.Choices[0].Message.ToolCalls) == 0 {
			break
		}

		m.logger.Debug("Agent loop iteration", "iteration", iteration+1, "toolCalls", len(*currentResponse.Choices[0].Message.ToolCalls))

		toolResults, err := m.executeToolCalls(ctx, *currentResponse.Choices[0].Message.ToolCalls)
		if err != nil {
			m.logger.Error("Failed to execute tool calls", err)
			finalResponseBytes, _ := json.Marshal(currentResponse)
			w.ResponseWriter.Header().Set("Content-Type", "application/json")
			w.ResponseWriter.WriteHeader(http.StatusOK)
			if _, err := w.ResponseWriter.Write(finalResponseBytes); err != nil {
				m.logger.Error("Failed to write response", err)
			}
			return nil
		}

		currentRequest.Messages = append(currentRequest.Messages, currentResponse.Choices[0].Message)
		currentRequest.Messages = append(currentRequest.Messages, toolResults...)

		internalCtx := context.WithValue(ctx, mcpInternalKey, "true")
		currentRequest.Model = providerModel
		nextResponse, err := provider.ChatCompletions(internalCtx, currentRequest)
		if err != nil {
			m.logger.Error("Failed to get response in agent loop", err)
			finalResponseBytes, _ := json.Marshal(currentResponse)
			w.ResponseWriter.Header().Set("Content-Type", "application/json")
			w.ResponseWriter.WriteHeader(http.StatusOK)
			if _, err := w.ResponseWriter.Write(finalResponseBytes); err != nil {
				m.logger.Error("Failed to write response", err)
			}
			return nil
		}

		currentResponse = nextResponse
		iteration++
	}

	if iteration >= maxIterations {
		m.logger.Error("Agent loop reached maximum iterations", fmt.Errorf("max iterations reached: %d", maxIterations))
	}

	finalResponseBytes, err := json.Marshal(currentResponse)
	if err != nil {
		m.logger.Error("Failed to marshal final response", err)
		w.writeToClient = true
		w.ResponseWriter.WriteHeader(w.statusCode)
		if _, err := w.ResponseWriter.Write(w.body.Bytes()); err != nil {
			m.logger.Error("Failed to write response", err)
		}
		return nil
	}

	w.ResponseWriter.Header().Set("Content-Type", "application/json")
	w.ResponseWriter.WriteHeader(http.StatusOK)
	if _, err := w.ResponseWriter.Write(finalResponseBytes); err != nil {
		m.logger.Error("Failed to write response", err)
	}
	return nil
}

// processStreamingResponse handles streaming chat completion responses with agent loop
func (m *MCPMiddlewareImpl) processStreamingResponse(ctx context.Context, w *customResponseWriter, originalRequest *providers.CreateChatCompletionRequest, provider providers.IProvider, providerModel string) error {
	responseBody := w.body.String()
	if responseBody == "" {
		w.writeToClient = true
		w.ResponseWriter.WriteHeader(w.statusCode)
		return nil
	}

	toolCalls, err := m.parseStreamingToolCalls(responseBody)
	if err != nil {
		m.logger.Error("Failed to parse streaming tool calls", err)
		w.writeToClient = true
		w.ResponseWriter.WriteHeader(w.statusCode)
		if _, err := w.ResponseWriter.Write(w.body.Bytes()); err != nil {
			m.logger.Error("Failed to write response", err)
		}
		return nil
	}

	if len(toolCalls) == 0 {
		w.writeToClient = true
		w.ResponseWriter.WriteHeader(w.statusCode)
		if _, err := w.ResponseWriter.Write(w.body.Bytes()); err != nil {
			m.logger.Error("Failed to write response", err)
		}
		return nil
	}

	currentRequest := *originalRequest
	maxIterations := 10
	iteration := 0

	w.ResponseWriter.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.ResponseWriter.Header().Set("Cache-Control", "no-cache")
	w.ResponseWriter.Header().Set("Connection", "keep-alive")
	w.ResponseWriter.WriteHeader(http.StatusOK)

	initialResponseLines := strings.Split(responseBody, "\n")
	for _, line := range initialResponseLines {
		if strings.TrimSpace(line) != "" {
			if _, err := w.ResponseWriter.Write([]byte(line + "\n")); err != nil {
				m.logger.Error("Failed to write initial response line", err)
				break
			}
		}
	}

	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}

	for iteration < maxIterations {
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

	if iteration >= maxIterations {
		m.logger.Error("Streaming agent loop reached maximum iterations", fmt.Errorf("max iterations reached: %d", maxIterations))
	}

	if _, err := w.ResponseWriter.Write([]byte("data: [DONE]\n\n")); err != nil {
		m.logger.Error("Failed to write done marker", err)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}

	return nil
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

	// Use the parsed tool calls directly - they already contain the correct data
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
