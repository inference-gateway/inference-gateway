package main

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	mcp_golang "github.com/metoro-io/mcp-golang"
	httpTransport "github.com/metoro-io/mcp-golang/transport/http"
)

const defaultLimit = 5

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

var mockResults = []SearchResult{
	{Title: "Understanding the Model Context Protocol", Snippet: "Learn about MCP and how it enables AI models to interact with external tools and data sources.", URL: "https://modelcontextprotocol.io/"},
	{Title: "Inference Gateway Documentation", Snippet: "Comprehensive guide to using Inference Gateway for AI model inference and integration.", URL: "https://github.com/inference-gateway/inference-gateway"},
	{Title: "Function Calling in LLMs", Snippet: "How large language models use function calling to interact with external systems and APIs.", URL: "https://ai-docs.example.com/function-calling"},
	{Title: "Building MCP Servers in Go", Snippet: "Tutorial on creating Model Context Protocol servers in Go.", URL: "https://golang-tutorials.example.com/mcp"},
}

// mockSearch returns the mock results matching query, or all of them when none match.
func mockSearch(query string, limit int) SearchResponse {
	var matches []SearchResult
	for _, result := range mockResults {
		if strings.Contains(strings.ToLower(result.Title+" "+result.Snippet), strings.ToLower(query)) {
			matches = append(matches, result)
		}
	}
	if len(matches) == 0 {
		matches = mockResults
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	matches = matches[:min(limit, len(matches))]
	return SearchResponse{Results: matches, Total: len(matches), Query: query}
}

func main() {
	transport := httpTransport.NewGinTransport()
	server := mcp_golang.NewServer(transport, mcp_golang.WithName("mcp-search-server"), mcp_golang.WithVersion("0.0.1"))

	err := server.RegisterTool("search", "Performs a web search with the given query", func(_ context.Context, args SearchArgs) (*mcp_golang.ToolResponse, error) {
		results, err := json.Marshal(mockSearch(args.Query, args.Limit))
		if err != nil {
			return nil, err
		}
		return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(string(results))), nil
	})
	if err != nil {
		log.Fatalf("register tool: %v", err)
	}

	go func() {
		if err := server.Serve(); err != nil {
			log.Fatalf("serve: %v", err)
		}
	}()

	r := gin.Default()
	r.POST("/mcp", transport.Handler())
	log.Fatal(r.Run(":8082"))
}
