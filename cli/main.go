package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dceluis/mcp-go/mcp"
	"github.com/dceluis/mcp-go/server"
)

// Global variables for Tasker server host and port.
var toolsPath string
var taskerHost string
var taskerPort string
var taskerApiKey string

const (
	toolsEnvKey      = "TASKER_MCP_TOOLS"
	hostEnvKey       = "TASKER_MCP_HOST"
	portEnvKey       = "TASKER_MCP_PORT"
	modeEnvKey       = "TASKER_MCP_MODE"
	taskerHostEnvKey = "TASKER_MCP_TASKER_HOST"
	taskerPortEnvKey = "TASKER_MCP_TASKER_PORT"
	taskerKeyEnvKey  = "TASKER_MCP_TASKER_API_KEY"
)

// GenericMap is a new type for tool arguments.
type GenericMap map[string]interface{}

// TaskerTool defines the structure for a tool loaded from JSON.
type TaskerTool struct {
	TaskerName  string                 `json:"tasker_name"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// genericToolHandler returns a tool handler function for a given Tasker tool.
func genericToolHandler(tool TaskerTool) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.Params.Arguments
		if args == nil {
			return mcp.NewToolResultError("Arguments must be provided"), nil
		}
		// Log the tool call.
		log.Printf("Tool called: %s with args: %+v", tool.Name, args)
		// Execute the Tasker task.
		result, err := runTaskerTask(tool.TaskerName, args)
		if err != nil {
			return nil, err
		}
		// Return the result using the new result constructor.
		return mcp.NewToolResultText(result), nil
	}
}

// runTaskerTask sends an HTTP POST to the Tasker endpoint to execute the task.
func runTaskerTask(taskerName string, args map[string]interface{}) (string, error) {
	payload := map[string]interface{}{
		"name":      taskerName,
		"arguments": args,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	// Build the URL using the specified taskerHost and taskerPort.
	taskerURL := fmt.Sprintf("http://%s:%s/run_task", taskerHost, taskerPort)
	req, err := http.NewRequest("POST", taskerURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if taskerApiKey != "" {
		req.Header.Set("Authorization", "Bearer "+taskerApiKey)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP error: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bodyBytes), nil
}

// loadToolsFromFile reads and unmarshals the JSON file containing tool definitions.
func loadToolsFromBytes(data []byte) ([]TaskerTool, error) {
	var tools []TaskerTool
	if err := json.Unmarshal(data, &tools); err != nil {
		return nil, err
	}
	return tools, nil
}

func loadToolsFromFile(filePath string) ([]TaskerTool, error) {
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return loadToolsFromBytes(fileBytes)
}

func NewMCPServer(selectedTools []TaskerTool) *server.MCPServer {
	mcpServer := server.NewMCPServer(
		"tasker-mcp-server",
		"1.0.0",
		server.WithLogging(),
	)

	// Map to hold tool handlers for STDIO transport.
	toolHandlers := make(map[string]server.ToolHandlerFunc)

	for _, tool := range selectedTools {
		// Since tool.InputSchema is already a map[string]interface{}, assign it directly.
		inputSchema := tool.InputSchema

		var opts []mcp.ToolOption
		// Check if inputSchema is not nil.
		if inputSchema != nil {
			// Extract required fields if available.
			var required []string
			if req, ok := inputSchema["required"].([]interface{}); ok {
				for _, r := range req {
					if str, ok := r.(string); ok {
						required = append(required, str)
					}
				}
			}
			// Process properties.
			if props, ok := inputSchema["properties"].(map[string]interface{}); ok {
				for key, propRaw := range props {
					if prop, ok := propRaw.(map[string]interface{}); ok {
						desc := ""
						if d, ok := prop["description"].(string); ok {
							desc = d
						}
						var propOpts []mcp.PropertyOption
						for _, reqKey := range required {
							if reqKey == key {
								propOpts = append(propOpts, mcp.Required())
								break
							}
						}
						if desc != "" {
							propOpts = append(propOpts, mcp.Description(desc))
						}
						// Based on type, add the proper argument option.
						switch t := prop["type"].(string); t {
						case "string":
							opts = append(opts, mcp.WithString(key, propOpts...))
						case "number":
							opts = append(opts, mcp.WithNumber(key, propOpts...))
						default:
							opts = append(opts, mcp.WithString(key, propOpts...))
						}
					}
				}
			}
		}
		// Use ... to expand the opts slice into variadic arguments.
		allOpts := append([]mcp.ToolOption{mcp.WithDescription(tool.Description)}, opts...)
		toolObj := mcp.NewTool(tool.Name, allOpts...)
		handler := genericToolHandler(tool)
		mcpServer.AddTool(toolObj, handler)
		toolHandlers[tool.Name] = handler
	}

	return mcpServer
}

func main() {
	toolsPathFlag := flag.String("tools", getEnv(toolsEnvKey, ""), "Path to JSON file with Tasker tool definitions")
	host := flag.String("host", getEnv(hostEnvKey, "0.0.0.0"), "Host address to listen on for SSE server (default: 0.0.0.0)")
	port := flag.String("port", getEnv(portEnvKey, "8000"), "Port to listen on for SSE server (default: 8000)")
	mode := flag.String("mode", getEnv(modeEnvKey, "stdio"), "Transport mode: sse, or stdio (default: stdio)")
	taskerHostFlag := flag.String("tasker-host", getEnv(taskerHostEnvKey, "0.0.0.0"), "Tasker server host (default: 0.0.0.0)")
	taskerPortFlag := flag.String("tasker-port", getEnv(taskerPortEnvKey, "1821"), "Tasker server port (default: 1821)")
	taskerApiKeyFlag := flag.String("tasker-api-key", getEnv(taskerKeyEnvKey, ""), "Tasker API Key")
	flag.Parse()

	// Set the global Tasker server variables.
	taskerHost = *taskerHostFlag
	taskerPort = *taskerPortFlag
	taskerApiKey = *taskerApiKeyFlag
	toolsPath = resolveToolsPath(*toolsPathFlag)
	if toolsPath == "" {
		log.Fatal("Please provide the -tools flag or set the TASKER_MCP_TOOLS environment variable with the path to the JSON file containing tool definitions")
	}

	if !fileExists(toolsPath) {
		log.Fatalf("tool descriptions file not found: %s", toolsPath)
	}

	log.Printf("Using tool descriptions file: %s", toolsPath)

	selectedTools, err := loadToolsFromFile(toolsPath)
	if err != nil {
		log.Fatalf("failed to load tools: %v", err)
	}

	// Instantiate the MCP server using the new mcp-go-sdk API.
	mcpServer := NewMCPServer(selectedTools)

	switch strings.ToLower(*mode) {
	case "sse":
		addr := fmt.Sprintf("%s:%s", *host, *port)
		if err := startHTTPServer(mcpServer, addr); err != nil {
			log.Fatalf("SSE server error: %v", err)
		}
	case "stdio":
		if err := server.ServeStdio(mcpServer); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	default:
		log.Fatalf("Unknown transport mode: %s", *mode)
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func resolveToolsPath(initial string) string {
	if initial != "" {
		return initial
	}

	candidates := []string{
		filepath.Join(".", "dist", "toolDescriptions.json"),
		filepath.Join(".", "toolDescriptions.json"),
	}

	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "dist", "toolDescriptions.json"),
			filepath.Join(dir, "toolDescriptions.json"),
		)
	}

	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}

	return ""
}

func startHTTPServer(mcpServer *server.MCPServer, addr string) error {
	sseServer := server.NewSSEServer(mcpServer)
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.Handle("/sse", sseServer)
	mux.Handle("/message", sseServer)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := map[string]any{
			"status":      "ok",
			"endpoints":   []string{"/sse", "/message"},
			"healthcheck": "/healthz",
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("failed to encode root response: %v", err)
		}
	})

	httpServer := &http.Server{ //nolint:exhaustruct
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("Starting SSE server on %s...", addr)
	return httpServer.ListenAndServe()
}
