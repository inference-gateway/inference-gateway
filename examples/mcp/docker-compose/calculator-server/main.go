// Command calculator-server is an MCP server built on the official Go SDK
// (github.com/modelcontextprotocol/go-sdk). Its stateless Streamable HTTP
// handler speaks MCP 2026-07-28, the revision the gateway uses upstream, and
// still answers 2025-era clients.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const addr = ":8085"

type calculateInput struct {
	A         float64 `json:"a" jsonschema:"the first operand"`
	B         float64 `json:"b" jsonschema:"the second operand"`
	Operation string  `json:"operation" jsonschema:"one of add, subtract, multiply, divide"`
}

type calculateOutput struct {
	Result float64 `json:"result" jsonschema:"the result of the operation"`
}

var errDivisionByZero = errors.New("division by zero")

// calculate returns its result as structuredContent, matching the output
// schema the SDK derives from calculateOutput. A returned error becomes a
// tool result with isError set, which the gateway relays as is.
func calculate(_ context.Context, _ *mcp.CallToolRequest, in calculateInput) (*mcp.CallToolResult, calculateOutput, error) {
	switch in.Operation {
	case "add":
		return nil, calculateOutput{Result: in.A + in.B}, nil
	case "subtract":
		return nil, calculateOutput{Result: in.A - in.B}, nil
	case "multiply":
		return nil, calculateOutput{Result: in.A * in.B}, nil
	case "divide":
		if in.B == 0 {
			return nil, calculateOutput{}, errDivisionByZero
		}
		return nil, calculateOutput{Result: in.A / in.B}, nil
	default:
		return nil, calculateOutput{}, fmt.Errorf("unknown operation %q", in.Operation)
	}
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "calculator", Version: "1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "calculate",
		Description: "Add, subtract, multiply or divide two numbers",
	}, calculate)

	// 2026-07-28 requests are only served in stateless mode.
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true})

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("calculator MCP server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
