package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	mcp_golang "github.com/metoro-io/mcp-golang"
	httpTransport "github.com/metoro-io/mcp-golang/transport/http"
)

const baseDir = "/tmp/mcp-files"

// FileWriteArgs defines the arguments for writing to a file
type FileWriteArgs struct {
	Path    string `json:"path" jsonschema:"description=The file path to write to"`
	Content string `json:"content" jsonschema:"description=The content to write to the file"`
}

// FileReadArgs defines the arguments for reading from a file
type FileReadArgs struct {
	Path string `json:"path" jsonschema:"description=The file path to read from"`
}

// FileDeleteArgs defines the arguments for deleting a file
type FileDeleteArgs struct {
	Path string `json:"path" jsonschema:"description=The file path to delete"`
}

// sandboxed resolves path inside baseDir - cleaning it as an absolute path first drops any "..".
func sandboxed(path string) string {
	return filepath.Join(baseDir, filepath.Clean("/"+path))
}

func text(format string, args ...any) *mcp_golang.ToolResponse {
	return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf(format, args...)))
}

func writeFile(_ context.Context, args FileWriteArgs) (*mcp_golang.ToolResponse, error) {
	fullPath := sandboxed(args.Path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return text("Failed to create directory: %v", err), nil
	}
	if err := os.WriteFile(fullPath, []byte(args.Content), 0o644); err != nil {
		return text("Failed to write file: %v", err), nil
	}
	return text("Successfully wrote %d bytes to %s", len(args.Content), args.Path), nil
}

func readFile(_ context.Context, args FileReadArgs) (*mcp_golang.ToolResponse, error) {
	content, err := os.ReadFile(sandboxed(args.Path))
	if err != nil {
		return text("Failed to read file: %v", err), nil
	}
	return text("File content from %s:\n\n%s", args.Path, content), nil
}

func deleteFile(_ context.Context, args FileDeleteArgs) (*mcp_golang.ToolResponse, error) {
	if err := os.Remove(sandboxed(args.Path)); err != nil {
		return text("Failed to delete file: %v", err), nil
	}
	return text("Successfully deleted file: %s", args.Path), nil
}

func main() {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		log.Fatalf("create base directory: %v", err)
	}

	transport := httpTransport.NewGinTransport()
	server := mcp_golang.NewServer(transport, mcp_golang.WithName("mcp-filesystem-server"), mcp_golang.WithVersion("0.0.1"))

	for name, tool := range map[string]struct {
		description string
		handler     any
	}{
		"write_file":  {"Write content to a file, replacing it", writeFile},
		"read_file":   {"Read content from a file", readFile},
		"delete_file": {"Delete a file", deleteFile},
	} {
		if err := server.RegisterTool(name, tool.description, tool.handler); err != nil {
			log.Fatalf("register tool %s: %v", name, err)
		}
	}

	go func() {
		if err := server.Serve(); err != nil {
			log.Fatalf("serve: %v", err)
		}
	}()

	r := gin.Default()
	r.POST("/mcp", transport.Handler())
	log.Fatal(r.Run(":8083"))
}
