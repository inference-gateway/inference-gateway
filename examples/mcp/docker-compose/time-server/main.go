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

// TimeArgs defines the arguments for the time tool
type TimeArgs struct {
	Format string `json:"format,omitempty" jsonschema:"description=The time format to use"`
}

// SSEHandler handles Server-Sent Events for real-time MCP responses
func setupSSEHandler(server *mcp_golang.Server, router *gin.Engine) {
	router.GET("/mcp/stream", func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Cache-Control")

		responseChan := make(chan map[string]any, 10)
		defer close(responseChan)

		clientGone := c.Request.Context().Done()

		initialMsg := map[string]any{
			"type":    "connection",
			"status":  "connected",
			"message": "MCP Time Server SSE stream established",
		}

		data, _ := json.Marshal(initialMsg)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		c.Writer.Flush()

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-clientGone:
				log.Println("SSE client disconnected")
				return
			case <-ticker.C:
				timeUpdate := map[string]any{
					"type":      "time_update",
					"timestamp": time.Now().Format(time.RFC3339),
					"server":    "mcp-time-server",
				}

				data, _ := json.Marshal(timeUpdate)
				fmt.Fprintf(c.Writer, "data: %s\n\n", data)
				c.Writer.Flush()
			}
		}
	})

	router.POST("/mcp/stream/tool", func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Cache-Control")

		var request struct {
			Method string `json:"method"`
			Params any    `json:"params"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			errorMsg := map[string]any{
				"type":  "error",
				"error": err.Error(),
			}
			data, _ := json.Marshal(errorMsg)
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()
			return
		}

		if request.Method == "tools/call" {
			fmt.Fprintf(c.Writer, "data: %s\n\n", `{"type":"processing","message":"Processing time tool request..."}`)
			c.Writer.Flush()

			time.Sleep(100 * time.Millisecond)

			result := map[string]any{
				"type":   "result",
				"result": fmt.Sprintf("Current time (streamed): %s", time.Now().Format(time.RFC3339)),
			}

			data, _ := json.Marshal(result)
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()
		}
	})
}

func main() {
	transport := httpTransport.NewGinTransport()

	server := mcp_golang.NewServer(transport, mcp_golang.WithName("mcp-time-server"), mcp_golang.WithVersion("0.0.1"))

	err := server.RegisterTool("time", "Returns the current time in the specified format", func(ctx context.Context, args TimeArgs) (*mcp_golang.ToolResponse, error) {
		ginCtx, ok := ctx.Value("ginContext").(*gin.Context)
		if !ok {
			return nil, fmt.Errorf("ginContext not found in context")
		}
		userAgent := ginCtx.GetHeader("User-Agent")
		log.Printf("Request from User-Agent: %s", userAgent)
		log.Printf("Request traceparent header: %s", ginCtx.GetHeader("traceparent"))

		log.Printf("Received request with args: %+v", args)

		now := time.Now()
		var currentTime string

		switch args.Format {
		case "":
			currentTime = now.Format(time.RFC3339)
		case "HH:mm:ss":
			currentTime = now.Format("15:04:05")
		case "hh:mm:ss":
			currentTime = now.Format("03:04:05 PM")
		case "YYYY-MM-DD":
			currentTime = now.Format("2006-01-02")
		case "DD/MM/YYYY":
			currentTime = now.Format("02/01/2006")
		case "MM/DD/YYYY":
			currentTime = now.Format("01/02/2006")
		case "full":
			currentTime = now.Format("Monday, January 2, 2006 at 3:04:05 PM MST")
		case "date-time":
			currentTime = now.Format("2006-01-02 15:04:05")
		case "%Y-%m-%d %H:%M:%S":
			currentTime = now.Format("2006-01-02 15:04:05")
		default:
			if strings.Contains(args.Format, "2006") || strings.Contains(args.Format, "15:04") {
				currentTime = now.Format(args.Format)
			} else if strings.Contains(args.Format, "%Y") || strings.Contains(args.Format, "%H") {
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
				currentTime = now.Format(time.RFC3339)
				log.Printf("Unrecognized format '%s', using RFC3339 instead", args.Format)
			}
		}

		log.Printf("Returning time: %s", currentTime)

		responseText := fmt.Sprintf("The current time is %s on %s",
			now.Format("15:04:05"),
			now.Format("Monday, January 2, 2006"))
		return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(responseText)), nil
	})
	if err != nil {
		panic(err)
	}

	go server.Serve()

	r := gin.Default()

	setupSSEHandler(server, r)

	r.POST("/mcp", transport.Handler())

	r.GET("/capabilities", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"mcp_version": "0.0.1",
			"server_name": "mcp-time-server",
			"features": []string{
				"tools",
				"server-sent-events",
				"real-time-streaming",
			},
			"endpoints": gin.H{
				"mcp":          "/mcp",
				"sse_stream":   "/mcp/stream",
				"sse_tool":     "/mcp/stream/tool",
				"health":       "/health",
				"capabilities": "/capabilities",
			},
		})
	})

	log.Println("Starting MCP Time Server with SSE support on :8081...")
	log.Println("Endpoints:")
	log.Println("  - POST /mcp (traditional MCP)")
	log.Println("  - GET  /mcp/stream (SSE stream)")
	log.Println("  - POST /mcp/stream/tool (SSE tool calls)")
	log.Println("  - GET  /capabilities (server info)")
	if err := r.Run(":8081"); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
