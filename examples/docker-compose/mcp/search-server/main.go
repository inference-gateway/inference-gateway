package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

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

	// Add the MCP endpoint
	r.POST("/mcp", transport.Handler())

	// Add a health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	// Start the server
	log.Println("Starting MCP Search Server on :8082...")
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
