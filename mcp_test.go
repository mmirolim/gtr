package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestMCPServerInitialize(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	server := NewMCPServer(logger)
	server.writer = &out

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
		Params: json.RawMessage(`{
			"protocolVersion": "2024-11-05",
			"capabilities": {},
			"clientInfo": {"name": "test", "version": "0.1.0"}
		}`),
	}

	server.handleRequest(req)

	var resp JSONRPCResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["protocolVersion"] != mcpProtocolVersion {
		t.Errorf("expected protocol version %s, got %v", mcpProtocolVersion, result["protocolVersion"])
	}

	serverInfo, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatal("serverInfo is not a map")
	}
	if serverInfo["name"] != "gtr" {
		t.Errorf("expected server name 'gtr', got %v", serverInfo["name"])
	}
}

func TestMCPServerToolsList(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	server := NewMCPServer(logger)
	server.writer = &out

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/list",
	}

	server.handleRequest(req)

	var resp JSONRPCResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}

	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatal("tools is not an array")
	}

	if len(tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(tools))
	}

	// Verify tool names
	expectedNames := map[string]bool{
		"affected_tests":      false,
		"run_affected_tests":  false,
		"file_impact_analysis": false,
	}

	for _, tool := range tools {
		toolMap, ok := tool.(map[string]any)
		if !ok {
			t.Error("tool is not a map")
			continue
		}
		name, ok := toolMap["name"].(string)
		if !ok {
			t.Error("tool name is not a string")
			continue
		}
		if _, exists := expectedNames[name]; !exists {
			t.Errorf("unexpected tool name: %s", name)
		}
		expectedNames[name] = true
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestMCPServerUnknownMethod(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	server := NewMCPServer(logger)
	server.writer = &out

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Method:  "unknown/method",
	}

	server.handleRequest(req)

	var resp JSONRPCResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}

	if resp.Error.Code != -32601 {
		t.Errorf("expected error code -32601, got %d", resp.Error.Code)
	}
}

func TestMCPServerPing(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	server := NewMCPServer(logger)
	server.writer = &out

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`4`),
		Method:  "ping",
	}

	server.handleRequest(req)

	var resp JSONRPCResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error.Message)
	}
}

func TestJSONRPCMessageRoundtrip(t *testing.T) {
	// Test request serialization/deserialization
	reqJSON := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	var req JSONRPCRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("failed to parse request: %v", err)
	}
	if req.Method != "tools/list" {
		t.Errorf("expected method 'tools/list', got %s", req.Method)
	}

	// Test response serialization
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result:  map[string]string{"status": "ok"},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed JSONRPCResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse marshaled response: %v", err)
	}
}
