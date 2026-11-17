package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	mcp_golang "github.com/metoro-io/mcp-golang"
	httpTransport "github.com/metoro-io/mcp-golang/transport/http"
)

// TimeArgs is the empty argument set of the time tool.
type TimeArgs struct{}

// logTraceparent logs the W3C trace context the gateway propagates, which the monitoring example inspects.
func logTraceparent(ctx context.Context) {
	if ginCtx, ok := ctx.Value("ginContext").(*gin.Context); ok {
		log.Printf("Request traceparent header: %s", ginCtx.GetHeader("traceparent"))
	}
}

func main() {
	transport := httpTransport.NewGinTransport()
	server := mcp_golang.NewServer(transport, mcp_golang.WithName("mcp-time-server"), mcp_golang.WithVersion("0.0.1"))

	err := server.RegisterTool("time", "Returns the current time", func(ctx context.Context, _ TimeArgs) (*mcp_golang.ToolResponse, error) {
		logTraceparent(ctx)
		now := time.Now()
		text := fmt.Sprintf("The current time is %s on %s", now.Format("15:04:05"), now.Format("Monday, January 2, 2006"))
		return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(text)), nil
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
	log.Fatal(r.Run(":8081"))
}
