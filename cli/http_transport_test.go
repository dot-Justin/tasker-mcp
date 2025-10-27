package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"testing"

	"github.com/dceluis/mcp-go/mcp"
)

func TestStreamableHTTPInitialize(t *testing.T) {
	ts, _ := newTestHTTPServer(t)
	defer ts.Close()

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": mcp.LATEST_PROTOCOL_VERSION,
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "tasker-mcp-test",
				"version": "0.0.1",
			},
		},
	}

	resp := sendRequest(t, ts, payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var rpcResponse struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			ServerInfo struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rpcResponse); err != nil {
		t.Fatalf("failed to decode initialize response: %v", err)
	}

	if rpcResponse.Error != nil {
		t.Fatalf("initialize returned error: %+v", rpcResponse.Error)
	}

	if rpcResponse.Result.ServerInfo.Name != "tasker-mcp-server" {
		t.Fatalf("expected server name tasker-mcp-server, got %q", rpcResponse.Result.ServerInfo.Name)
	}

	if rpcResponse.Result.ProtocolVersion == "" {
		t.Fatal("expected protocol version in initialize response")
	}
}

func TestStreamableHTTPListTools(t *testing.T) {
	ts, expectedNames := newTestHTTPServer(t)
	defer ts.Close()

	initPayload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": mcp.LATEST_PROTOCOL_VERSION,
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "tasker-mcp-test",
				"version": "0.0.1",
			},
		},
	}

	initResp := sendRequest(t, ts, initPayload)
	initResp.Body.Close()

	listPayload := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "list_tools",
		"params":  map[string]any{},
	}

	resp := sendRequest(t, ts, listPayload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var rpcResponse struct {
		Result struct {
			Tools []mcp.Tool `json:"tools"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rpcResponse); err != nil {
		t.Fatalf("failed to decode list tools response: %v", err)
	}

	if rpcResponse.Error != nil {
		t.Fatalf("list tools returned error: %+v", rpcResponse.Error)
	}

	if len(rpcResponse.Result.Tools) == 0 {
		t.Fatal("expected at least one tool in response")
	}

	actualNames := make([]string, len(rpcResponse.Result.Tools))
	for i, tool := range rpcResponse.Result.Tools {
		actualNames[i] = tool.Name
	}

	sort.Strings(actualNames)
	sort.Strings(expectedNames)

	if len(actualNames) != len(expectedNames) {
		t.Fatalf("expected %d tools, got %d", len(expectedNames), len(actualNames))
	}

	for i := range expectedNames {
		if expectedNames[i] != actualNames[i] {
			t.Fatalf("expected tool %q at index %d, got %q", expectedNames[i], i, actualNames[i])
		}
	}
}

func newTestHTTPServer(t *testing.T) (*httptest.Server, []string) {
	t.Helper()

	toolPath := filepath.Join("assets", "toolDescriptions.json")
	tools, err := loadToolsFromFile(toolPath)
	if err != nil {
		t.Fatalf("failed to load tools: %v", err)
	}

	expectedNames := make([]string, len(tools))
	for i, tool := range tools {
		expectedNames[i] = tool.Name
	}

	mcpServer := NewMCPServer(tools)

	mux := http.NewServeMux()
	mux.Handle("/mcp", newStreamableHTTPHandler(mcpServer))

	ts := httptest.NewServer(mux)
	return ts, expectedNames
}

func sendRequest(t *testing.T, ts *httptest.Server, payload map[string]any) *http.Response {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	resp, err := http.Post(ts.URL+"/mcp", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}

	return resp
}
