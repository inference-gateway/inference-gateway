package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	mcp_golang "github.com/metoro-io/mcp-golang"
	httpTransport "github.com/metoro-io/mcp-golang/transport/http"
)

// SearchArgs defines the arguments for the search tool
type SearchArgs struct {
	Query string `json:"query" jsonschema:"description=The search query string"`
	Limit int    `json:"limit,omitempty" jsonschema:"description=Maximum number of results to return (default: 5)"`
}

// SearchResult represents a single search result
type SearchResult struct {
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	URL     string `json:"url"`
}

// SearchResponse contains a list of search results
type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
	Query   string         `json:"query"`
}

// SSE Handler for search streaming
func setupSearchSSEHandler(server *mcp_golang.Server, router *gin.Engine) {
	router.GET("/mcp/stream", func(c *gin.Context) {
		// Set SSE headers
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Cache-Control")

		// Handle client disconnection
		clientGone := c.Request.Context().Done()

		// Send initial connection message
		initialMsg := map[string]interface{}{
			"type":    "connection",
			"status":  "connected",
			"message": "MCP Search Server SSE stream established",
		}

		data, _ := json.Marshal(initialMsg)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		c.Writer.Flush()

		// Listen for streaming requests
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-clientGone:
				log.Println("Search SSE client disconnected")
				return
			case <-ticker.C:
				// Send periodic server status
				statusUpdate := map[string]interface{}{
					"type":      "status",
					"timestamp": time.Now().Format(time.RFC3339),
					"server":    "mcp-search-server",
					"ready":     true,
				}

				data, _ := json.Marshal(statusUpdate)
				fmt.Fprintf(c.Writer, "data: %s\n\n", data)
				c.Writer.Flush()
			}
		}
	})

	// SSE endpoint for streaming search results
	router.POST("/mcp/stream/search", func(c *gin.Context) {
		// Set SSE headers
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Cache-Control")

		var request struct {
			Query string `json:"query"`
			Limit int    `json:"limit,omitempty"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			errorMsg := map[string]interface{}{
				"type":  "error",
				"error": err.Error(),
			}
			data, _ := json.Marshal(errorMsg)
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()
			return
		}

		// Stream search processing status
		fmt.Fprintf(c.Writer, "data: %s\n\n", `{"type":"processing","message":"Starting search request...","query":"`+request.Query+`"}`)
		c.Writer.Flush()

		// Simulate search processing with streaming results
		time.Sleep(100 * time.Millisecond)

		fmt.Fprintf(c.Writer, "data: %s\n\n", `{"type":"progress","message":"Searching databases..."}`)
		c.Writer.Flush()

		time.Sleep(200 * time.Millisecond)

		// Get search results
		results := performMockSearch(request.Query, request.Limit)

		// Stream each result individually for real-time effect
		for i, result := range results.Results {
			resultMsg := map[string]interface{}{
				"type":   "search_result",
				"index":  i + 1,
				"total":  len(results.Results),
				"result": result,
			}

			data, _ := json.Marshal(resultMsg)
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()

			// Small delay between results for streaming effect
			time.Sleep(50 * time.Millisecond)
		}

		// Send completion message
		completionMsg := map[string]interface{}{
			"type":    "complete",
			"total":   results.Total,
			"query":   results.Query,
			"message": fmt.Sprintf("Search completed. Found %d results.", results.Total),
		}

		data, _ := json.Marshal(completionMsg)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		c.Writer.Flush()
	})
}

func main() {
	// Create a Gin transport
	transport := httpTransport.NewGinTransport()

	// Create a new server with the transport
	server := mcp_golang.NewServer(transport, mcp_golang.WithName("mcp-search-server"), mcp_golang.WithVersion("0.0.1"))

	// Register search tool
	err := server.RegisterTool("search", "Performs a web search with the given query", func(ctx context.Context, args SearchArgs) (*mcp_golang.ToolResponse, error) {
		ginCtx, ok := ctx.Value("ginContext").(*gin.Context)
		if !ok {
			return nil, fmt.Errorf("ginContext not found in context")
		}
		userAgent := ginCtx.GetHeader("User-Agent")
		log.Printf("Request from User-Agent: %s", userAgent)

		log.Printf("Received search request with query: %s, limit: %d", args.Query, args.Limit)

		// Set default limit if not specified
		limit := args.Limit
		if limit <= 0 {
			limit = 5
		}

		// Perform mock search (in a real implementation, this would call a search API)
		results := performMockSearch(args.Query, limit)

		// Convert to JSON for structured response
		jsonResults, err := json.Marshal(results)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal search results: %v", err)
		}

		return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(string(jsonResults))), nil
	})
	if err != nil {
		panic(err)
	}

	go server.Serve()

	// Create a Gin router
	r := gin.Default()

	// Add CORS middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204) // Using 204 No Content status code
			return
		}

		c.Next()
	})

	// Setup SSE endpoints for real-time search streaming
	setupSearchSSEHandler(server, r)

	// Add the traditional MCP endpoint
	r.POST("/mcp", transport.Handler())

	// Add a health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	// Add capabilities endpoint
	r.GET("/capabilities", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"mcp_version": "0.0.1",
			"server_name": "mcp-search-server",
			"features": []string{
				"search",
				"server-sent-events",
				"real-time-streaming",
				"progressive-results",
			},
			"endpoints": gin.H{
				"mcp":          "/mcp",
				"sse_stream":   "/mcp/stream",
				"sse_search":   "/mcp/stream/search",
				"health":       "/health",
				"capabilities": "/capabilities",
			},
		})
	})

	// Start the server
	log.Println("Starting MCP Search Server with SSE support on :8082...")
	log.Println("Endpoints:")
	log.Println("  - POST /mcp (traditional MCP)")
	log.Println("  - GET  /mcp/stream (SSE stream)")
	log.Println("  - POST /mcp/stream/search (SSE search)")
	log.Println("  - GET  /health (health check)")
	log.Println("  - GET  /capabilities (server info)")
	if err := r.Run(":8082"); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// performMockSearch returns mock search results for demonstration purposes
func performMockSearch(query string, limit int) SearchResponse {
	// In a real implementation, this would call an actual search API
	mockResults := []SearchResult{
		{
			Title:   "Understanding the Model Context Protocol",
			Snippet: "Learn about MCP and how it enables AI models to interact with external tools and data sources.",
			URL:     "https://modelcontextprotocol.github.io/",
		},
		{
			Title:   "Inference Gateway Documentation",
			Snippet: "Comprehensive guide to using Inference Gateway for AI model inference and integration.",
			URL:     "https://github.com/inference-gateway/inference-gateway",
		},
		{
			Title:   "Function Calling in LLMs",
			Snippet: "How large language models use function calling to interact with external systems and APIs.",
			URL:     "https://ai-docs.example.com/function-calling",
		},
		{
			Title:   "Building MCP Servers in Go",
			Snippet: "Tutorial on creating Model Context Protocol servers using Golang and the mcp-golang library.",
			URL:     "https://golang-tutorials.example.com/mcp",
		},
		{
			Title:   "AI Tool Integration Best Practices",
			Snippet: "Learn the best practices for integrating AI models with external tools and services.",
			URL:     "https://ai-integration.example.com/best-practices",
		},
		{
			Title:   "Search APIs for AI Applications",
			Snippet: "Guide to implementing search functionality in AI applications and tools.",
			URL:     "https://apis.example.com/search-for-ai",
		},
	}

	// Filter results based on query (simple contains check for demonstration)
	filtered := make([]SearchResult, 0)
	for _, result := range mockResults {
		if strings.Contains(strings.ToLower(result.Title), strings.ToLower(query)) ||
			strings.Contains(strings.ToLower(result.Snippet), strings.ToLower(query)) {
			filtered = append(filtered, result)
		}
	}

	// If no results found (or query is empty), return all results
	if len(filtered) == 0 || query == "" {
		filtered = mockResults
	}

	// Limit the number of results
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return SearchResponse{
		Results: filtered,
		Total:   len(filtered),
		Query:   query,
	}
}
