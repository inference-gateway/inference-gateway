package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	mcp_golang "github.com/metoro-io/mcp-golang"
	httpTransport "github.com/metoro-io/mcp-golang/transport/http"
)

// FileWriteArgs defines the arguments for writing to a file
type FileWriteArgs struct {
	Path    string `json:"path" jsonschema:"description=The file path to write to"`
	Content string `json:"content" jsonschema:"description=The content to write to the file"`
	Mode    string `json:"mode,omitempty" jsonschema:"description=Write mode: 'overwrite' (default) or 'append'"`
}

// FileReadArgs defines the arguments for reading from a file
type FileReadArgs struct {
	Path string `json:"path" jsonschema:"description=The file path to read from"`
}

// FileDeleteArgs defines the arguments for deleting a file
type FileDeleteArgs struct {
	Path string `json:"path" jsonschema:"description=The file path to delete"`
}

// DirectoryListArgs defines the arguments for listing directory contents
type DirectoryListArgs struct {
	Path      string `json:"path" jsonschema:"description=The directory path to list"`
	Recursive bool   `json:"recursive,omitempty" jsonschema:"description=Whether to list recursively (default: false)"`
}

// DirectoryCreateArgs defines the arguments for creating a directory
type DirectoryCreateArgs struct {
	Path string `json:"path" jsonschema:"description=The directory path to create"`
}

// FileExistsArgs defines the arguments for checking if a file exists
type FileExistsArgs struct {
	Path string `json:"path" jsonschema:"description=The file path to check"`
}

// FileInfoArgs defines the arguments for getting file information
type FileInfoArgs struct {
	Path string `json:"path" jsonschema:"description=The file path to get info for"`
}

const (
	// Base directory for file operations - in a real deployment this should be configurable
	BASE_DIR = "/tmp/mcp-files"
)

// SSE Handler for filesystem operations streaming
func setupFilesystemSSEHandler(server *mcp_golang.Server, router *gin.Engine) {
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
			"message": "MCP Filesystem Server SSE stream established",
		}

		data, _ := json.Marshal(initialMsg)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		c.Writer.Flush()

		// Listen for streaming requests
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-clientGone:
				log.Println("Filesystem SSE client disconnected")
				return
			case <-ticker.C:
				// Send periodic filesystem status
				statusUpdate := map[string]interface{}{
					"type":      "status",
					"timestamp": time.Now().Format(time.RFC3339),
					"server":    "mcp-filesystem-server",
					"base_dir":  BASE_DIR,
					"ready":     true,
				}

				data, _ := json.Marshal(statusUpdate)
				fmt.Fprintf(c.Writer, "data: %s\n\n", data)
				c.Writer.Flush()
			}
		}
	})

	// SSE endpoint for streaming file operations
	router.POST("/mcp/stream/file", func(c *gin.Context) {
		// Set SSE headers
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Cache-Control")

		var request struct {
			Operation string `json:"operation"`
			Path      string `json:"path"`
			Content   string `json:"content,omitempty"`
			Mode      string `json:"mode,omitempty"`
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

		// Stream file operation processing
		fmt.Fprintf(c.Writer, "data: %s\n\n",
			fmt.Sprintf(`{"type":"processing","message":"Starting %s operation...","path":"%s"}`,
				request.Operation, request.Path))
		c.Writer.Flush()

		time.Sleep(100 * time.Millisecond)

		// Simulate processing based on operation type
		switch request.Operation {
		case "read":
			fmt.Fprintf(c.Writer, "data: %s\n\n", `{"type":"progress","message":"Reading file content..."}`)
			c.Writer.Flush()
			time.Sleep(100 * time.Millisecond)

		case "write":
			fmt.Fprintf(c.Writer, "data: %s\n\n", `{"type":"progress","message":"Writing file content..."}`)
			c.Writer.Flush()
			time.Sleep(150 * time.Millisecond)

		case "list":
			fmt.Fprintf(c.Writer, "data: %s\n\n", `{"type":"progress","message":"Scanning directory..."}`)
			c.Writer.Flush()
			time.Sleep(200 * time.Millisecond)
		}

		// Send completion message
		completionMsg := map[string]interface{}{
			"type":      "complete",
			"operation": request.Operation,
			"path":      request.Path,
			"message":   fmt.Sprintf("Operation %s completed successfully", request.Operation),
		}

		data, _ := json.Marshal(completionMsg)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		c.Writer.Flush()
	})
}

func main() {
	// Ensure base directory exists
	if err := os.MkdirAll(BASE_DIR, 0755); err != nil {
		log.Fatalf("Failed to create base directory: %v", err)
	}

	// Create a Gin transport
	transport := httpTransport.NewGinTransport()

	// Create a new server with the transport
	server := mcp_golang.NewServer(transport, mcp_golang.WithName("mcp-filesystem-server"), mcp_golang.WithVersion("0.0.1"))

	// Register write_file tool
	err := server.RegisterTool("write_file", "Write content to a file", func(ctx context.Context, args FileWriteArgs) (*mcp_golang.ToolResponse, error) {
		ginCtx, ok := ctx.Value("ginContext").(*gin.Context)
		if !ok {
			return nil, fmt.Errorf("ginContext not found in context")
		}
		userAgent := ginCtx.GetHeader("User-Agent")
		log.Printf("write_file request from User-Agent: %s", userAgent)

		log.Printf("Writing to file: %s", args.Path)

		// Validate and sanitize path
		fullPath, err := validatePath(args.Path)
		if err != nil {
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Error: %v", err))), nil
		}

		// Create directory if it doesn't exist
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Failed to create directory: %v", err))), nil
		}

		// Determine write mode
		flags := os.O_CREATE | os.O_WRONLY
		switch args.Mode {
		case "append":
			flags |= os.O_APPEND
		default: // "overwrite" or empty
			flags |= os.O_TRUNC
		}

		// Write file
		file, err := os.OpenFile(fullPath, flags, 0644)
		if err != nil {
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Failed to open file: %v", err))), nil
		}
		defer file.Close()

		if _, err := file.WriteString(args.Content); err != nil {
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Failed to write content: %v", err))), nil
		}

		responseText := fmt.Sprintf("Successfully wrote %d bytes to %s", len(args.Content), args.Path)
		return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(responseText)), nil
	})
	if err != nil {
		panic(err)
	}

	// Register read_file tool
	err = server.RegisterTool("read_file", "Read content from a file", func(ctx context.Context, args FileReadArgs) (*mcp_golang.ToolResponse, error) {
		ginCtx, ok := ctx.Value("ginContext").(*gin.Context)
		if !ok {
			return nil, fmt.Errorf("ginContext not found in context")
		}
		userAgent := ginCtx.GetHeader("User-Agent")
		log.Printf("read_file request from User-Agent: %s", userAgent)

		log.Printf("Reading from file: %s", args.Path)

		// Validate and sanitize path
		fullPath, err := validatePath(args.Path)
		if err != nil {
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Error: %v", err))), nil
		}

		// Read file
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Failed to read file: %v", err))), nil
		}

		responseText := fmt.Sprintf("File content from %s:\n\n%s", args.Path, string(content))
		return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(responseText)), nil
	})
	if err != nil {
		panic(err)
	}

	// Register delete_file tool
	err = server.RegisterTool("delete_file", "Delete a file", func(ctx context.Context, args FileDeleteArgs) (*mcp_golang.ToolResponse, error) {
		ginCtx, ok := ctx.Value("ginContext").(*gin.Context)
		if !ok {
			return nil, fmt.Errorf("ginContext not found in context")
		}
		userAgent := ginCtx.GetHeader("User-Agent")
		log.Printf("delete_file request from User-Agent: %s", userAgent)

		log.Printf("Deleting file: %s", args.Path)

		// Validate and sanitize path
		fullPath, err := validatePath(args.Path)
		if err != nil {
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Error: %v", err))), nil
		}

		// Delete file
		if err := os.Remove(fullPath); err != nil {
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Failed to delete file: %v", err))), nil
		}

		responseText := fmt.Sprintf("Successfully deleted file: %s", args.Path)
		return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(responseText)), nil
	})
	if err != nil {
		panic(err)
	}

	// Register list_directory tool
	err = server.RegisterTool("list_directory", "List the contents of a directory", func(ctx context.Context, args DirectoryListArgs) (*mcp_golang.ToolResponse, error) {
		ginCtx, ok := ctx.Value("ginContext").(*gin.Context)
		if !ok {
			return nil, fmt.Errorf("ginContext not found in context")
		}
		userAgent := ginCtx.GetHeader("User-Agent")
		log.Printf("list_directory request from User-Agent: %s", userAgent)

		log.Printf("Listing directory: %s (recursive: %t)", args.Path, args.Recursive)

		// Validate and sanitize path
		fullPath, err := validatePath(args.Path)
		if err != nil {
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Error: %v", err))), nil
		}

		var result strings.Builder
		result.WriteString(fmt.Sprintf("Contents of directory %s:\n\n", args.Path))

		if args.Recursive {
			err = filepath.WalkDir(fullPath, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				// Get relative path from base
				relPath, err := filepath.Rel(fullPath, path)
				if err != nil {
					return err
				}
				if relPath == "." {
					return nil // Skip the root directory itself
				}

				info, err := d.Info()
				if err != nil {
					result.WriteString(fmt.Sprintf("  %s (error getting info: %v)\n", relPath, err))
					return nil
				}

				if d.IsDir() {
					result.WriteString(fmt.Sprintf("  📁 %s/\n", relPath))
				} else {
					result.WriteString(fmt.Sprintf("  📄 %s (%d bytes)\n", relPath, info.Size()))
				}
				return nil
			})
		} else {
			entries, err := os.ReadDir(fullPath)
			if err != nil {
				return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Failed to read directory: %v", err))), nil
			}

			for _, entry := range entries {
				info, err := entry.Info()
				if err != nil {
					result.WriteString(fmt.Sprintf("  %s (error getting info: %v)\n", entry.Name(), err))
					continue
				}

				if entry.IsDir() {
					result.WriteString(fmt.Sprintf("  📁 %s/\n", entry.Name()))
				} else {
					result.WriteString(fmt.Sprintf("  📄 %s (%d bytes)\n", entry.Name(), info.Size()))
				}
			}
		}

		if err != nil {
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Error listing directory: %v", err))), nil
		}

		return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(result.String())), nil
	})
	if err != nil {
		panic(err)
	}

	// Register create_directory tool
	err = server.RegisterTool("create_directory", "Create a directory", func(ctx context.Context, args DirectoryCreateArgs) (*mcp_golang.ToolResponse, error) {
		ginCtx, ok := ctx.Value("ginContext").(*gin.Context)
		if !ok {
			return nil, fmt.Errorf("ginContext not found in context")
		}
		userAgent := ginCtx.GetHeader("User-Agent")
		log.Printf("create_directory request from User-Agent: %s", userAgent)

		log.Printf("Creating directory: %s", args.Path)

		// Validate and sanitize path
		fullPath, err := validatePath(args.Path)
		if err != nil {
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Error: %v", err))), nil
		}

		// Create directory
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Failed to create directory: %v", err))), nil
		}

		responseText := fmt.Sprintf("Successfully created directory: %s", args.Path)
		return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(responseText)), nil
	})
	if err != nil {
		panic(err)
	}

	// Register file_exists tool
	err = server.RegisterTool("file_exists", "Check if a file or directory exists", func(ctx context.Context, args FileExistsArgs) (*mcp_golang.ToolResponse, error) {
		ginCtx, ok := ctx.Value("ginContext").(*gin.Context)
		if !ok {
			return nil, fmt.Errorf("ginContext not found in context")
		}
		userAgent := ginCtx.GetHeader("User-Agent")
		log.Printf("file_exists request from User-Agent: %s", userAgent)

		log.Printf("Checking if file exists: %s", args.Path)

		// Validate and sanitize path
		fullPath, err := validatePath(args.Path)
		if err != nil {
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Error: %v", err))), nil
		}

		// Check if file exists
		info, err := os.Stat(fullPath)
		if os.IsNotExist(err) {
			responseText := fmt.Sprintf("File or directory does not exist: %s", args.Path)
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(responseText)), nil
		} else if err != nil {
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Error checking file: %v", err))), nil
		}

		var responseText string
		if info.IsDir() {
			responseText = fmt.Sprintf("Directory exists: %s", args.Path)
		} else {
			responseText = fmt.Sprintf("File exists: %s", args.Path)
		}

		return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(responseText)), nil
	})
	if err != nil {
		panic(err)
	}

	// Register file_info tool
	err = server.RegisterTool("file_info", "Get detailed information about a file or directory", func(ctx context.Context, args FileInfoArgs) (*mcp_golang.ToolResponse, error) {
		ginCtx, ok := ctx.Value("ginContext").(*gin.Context)
		if !ok {
			return nil, fmt.Errorf("ginContext not found in context")
		}
		userAgent := ginCtx.GetHeader("User-Agent")
		log.Printf("file_info request from User-Agent: %s", userAgent)

		log.Printf("Getting info for: %s", args.Path)

		// Validate and sanitize path
		fullPath, err := validatePath(args.Path)
		if err != nil {
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Error: %v", err))), nil
		}

		// Get file info
		info, err := os.Stat(fullPath)
		if err != nil {
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Failed to get file info: %v", err))), nil
		}

		var result strings.Builder
		result.WriteString(fmt.Sprintf("File information for %s:\n\n", args.Path))
		result.WriteString(fmt.Sprintf("  Name: %s\n", info.Name()))
		result.WriteString(fmt.Sprintf("  Size: %d bytes\n", info.Size()))
		result.WriteString(fmt.Sprintf("  Mode: %s\n", info.Mode()))
		result.WriteString(fmt.Sprintf("  Modified: %s\n", info.ModTime().Format("2006-01-02 15:04:05")))
		if info.IsDir() {
			result.WriteString("  Type: Directory\n")
		} else {
			result.WriteString("  Type: Regular file\n")
		}

		return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(result.String())), nil
	})
	if err != nil {
		panic(err)
	}

	go server.Serve()

	// Create a Gin router
	r := gin.Default()

	// Setup SSE endpoints for real-time filesystem streaming
	setupFilesystemSSEHandler(server, r)

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
			"server_name": "mcp-filesystem-server",
			"features": []string{
				"filesystem",
				"server-sent-events",
				"real-time-streaming",
				"file-operations",
			},
			"endpoints": gin.H{
				"mcp":          "/mcp",
				"sse_stream":   "/mcp/stream",
				"sse_file":     "/mcp/stream/file",
				"health":       "/health",
				"capabilities": "/capabilities",
			},
		})
	})

	// Start the server
	log.Println("Starting MCP Filesystem Server with SSE support on :8083...")
	log.Println("Endpoints:")
	log.Println("  - POST /mcp (traditional MCP)")
	log.Println("  - GET  /mcp/stream (SSE stream)")
	log.Println("  - POST /mcp/stream/file (SSE file ops)")
	log.Println("  - GET  /health (health check)")
	log.Println("  - GET  /capabilities (server info)")
	if err := r.Run(":8083"); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// validatePath validates and sanitizes the file path to prevent directory traversal attacks
func validatePath(path string) (string, error) {
	// Clean the path to remove any relative components
	cleanPath := filepath.Clean(path)

	// Convert to absolute path within BASE_DIR
	fullPath := filepath.Join(BASE_DIR, cleanPath)

	// Ensure the path is within BASE_DIR
	if !strings.HasPrefix(fullPath, BASE_DIR) {
		return "", fmt.Errorf("path is outside allowed directory")
	}

	return fullPath, nil
}
