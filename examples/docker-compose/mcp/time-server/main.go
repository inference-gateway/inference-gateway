package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/http"
)

// TimeArgs defines the arguments for the time tool
type TimeArgs struct {
	Format string `json:"format,omitempty" jsonschema:"description=The time format to use"`
}

func main() {
	// Create a Gin transport
	transport := http.NewGinTransport()

	// Create a new server with the transport
	server := mcp_golang.NewServer(transport, mcp_golang.WithName("mcp-time-server"), mcp_golang.WithVersion("0.0.1"))

	// Register a simple tool
	err := server.RegisterTool("time", "Returns the current time in the specified format", func(ctx context.Context, args TimeArgs) (*mcp_golang.ToolResponse, error) {
		ginCtx, ok := ctx.Value("ginContext").(*gin.Context)
		if !ok {
			return nil, fmt.Errorf("ginContext not found in context")
		}
		userAgent := ginCtx.GetHeader("User-Agent")
		log.Printf("Request from User-Agent: %s", userAgent)

		log.Printf("Received request with args: %+v", args)

		// Get the current time
		now := time.Now()
		var currentTime string

		// Handle special format requests, defaulting to RFC3339
		switch args.Format {
		case "":
			// Default format if none specified
			currentTime = now.Format(time.RFC3339)
		case "HH:mm:ss":
			// Common 24-hour format
			currentTime = now.Format("15:04:05")
		case "hh:mm:ss":
			// 12-hour format
			currentTime = now.Format("03:04:05 PM")
		case "YYYY-MM-DD":
			// Common date format
			currentTime = now.Format("2006-01-02")
		case "DD/MM/YYYY":
			// European date format
			currentTime = now.Format("02/01/2006")
		case "MM/DD/YYYY":
			// US date format
			currentTime = now.Format("01/02/2006")
		case "full":
			// Full date and time
			currentTime = now.Format("Monday, January 2, 2006 at 3:04:05 PM MST")
		case "date-time":
			// Date and time combined
			currentTime = now.Format("2006-01-02 15:04:05")
		case "%Y-%m-%d %H:%M:%S":
			// Common format used by Python/C - convert to Go format
			currentTime = now.Format("2006-01-02 15:04:05")
		default:
			// Try to use the provided format directly (for Go-style format strings)
			// or convert common format strings from other languages
			if strings.Contains(args.Format, "2006") || strings.Contains(args.Format, "15:04") {
				// Go format - use directly
				currentTime = now.Format(args.Format)
			} else if strings.Contains(args.Format, "%Y") || strings.Contains(args.Format, "%H") {
				// Handle Python/C style format strings
				format := args.Format
				format = strings.ReplaceAll(format, "%Y", "2006")
				format = strings.ReplaceAll(format, "%y", "06")
				format = strings.ReplaceAll(format, "%m", "01")
				format = strings.ReplaceAll(format, "%d", "02")
				format = strings.ReplaceAll(format, "%H", "15")
				format = strings.ReplaceAll(format, "%I", "03")
				format = strings.ReplaceAll(format, "%M", "04")
				format = strings.ReplaceAll(format, "%S", "05")
				format = strings.ReplaceAll(format, "%p", "PM")

				currentTime = now.Format(format)
			} else {
				currentTime = now.Format(time.RFC3339) // Default to standard format
				log.Printf("Unrecognized format '%s', using RFC3339 instead", args.Format)
			}
		}

		log.Printf("Returning time: %s", currentTime)

		// Create a more detailed response that will be easy for the LLM to understand
		responseText := fmt.Sprintf("The current time is %s on %s",
			now.Format("15:04:05"),
			now.Format("Monday, January 2, 2006"))
		return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(responseText)), nil
	})
	if err != nil {
		panic(err)
	}

	go server.Serve()

	// Create a Gin router
	r := gin.Default()

	// Add the MCP endpoint
	r.POST("/mcp", transport.Handler())

	// Add a health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	// Start the server
	log.Println("Starting Gin server on :8081...")
	if err := r.Run(":8081"); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
