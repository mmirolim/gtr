package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

// JSON-RPC 2.0 message types for MCP protocol

// JSONRPCRequest represents an incoming JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents an outgoing JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// MCP protocol constants
const (
	mcpProtocolVersion = "2024-11-05"
)

// MCPServer implements the MCP JSON-RPC 2.0 server over stdio
type MCPServer struct {
	log    *slog.Logger
	writer io.Writer
	mu     sync.Mutex // protects writer
}

// NewMCPServer creates a new MCP server
func NewMCPServer(logger *slog.Logger) *MCPServer {
	return &MCPServer{
		log:    logger,
		writer: os.Stdout,
	}
}

// Run starts the MCP server, reading from stdin and writing to stdout
func (s *MCPServer) Run() error {
	s.log.Info("MCP server starting", "transport", "stdio")

	scanner := bufio.NewScanner(os.Stdin)
	// Support large messages (up to 10MB)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.log.Error("failed to parse JSON-RPC request", "err", err)
			s.sendError(nil, -32700, "Parse error", nil)
			continue
		}

		s.handleRequest(req)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stdin scanner: %w", err)
	}

	s.log.Info("MCP server shutting down")
	return nil
}

func (s *MCPServer) handleRequest(req JSONRPCRequest) {
	s.log.Info("handling request", "method", req.Method)

	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "notifications/initialized":
		// Client acknowledged initialization — nothing to do
		s.log.Info("client initialized")
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(req)
	case "ping":
		s.sendResult(req.ID, map[string]string{})
	default:
		s.sendError(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method), nil)
	}
}

func (s *MCPServer) handleInitialize(req JSONRPCRequest) {
	result := map[string]any{
		"protocolVersion": mcpProtocolVersion,
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "gtr",
			"version": "0.2.0",
		},
	}
	s.sendResult(req.ID, result)
}

func (s *MCPServer) handleToolsList(req JSONRPCRequest) {
	tools := []map[string]any{
		{
			"name":        "affected_tests",
			"description": "Determine which tests are affected by file changes using call-graph analysis (CHA/VTA/RTA) or coverage-based analysis. Returns test names and subtests.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"files": map[string]any{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "List of changed file paths relative to the project root. If empty, uses uncommitted git changes.",
					},
					"git_ref": map[string]any{
						"type":        "string",
						"description": "Git ref to diff against (e.g. 'main', 'HEAD~1'). If provided, overrides 'files'.",
					},
					"analysis": map[string]any{
						"type":        "string",
						"description": "Analysis type: 'vta' (default), 'cha', 'rta', 'static', or 'coverage'.",
						"default":     "vta",
					},
					"work_dir": map[string]any{
						"type":        "string",
						"description": "Working directory of the Go project. Defaults to current directory.",
						"default":     ".",
					},
				},
			},
		},
		{
			"name":        "run_affected_tests",
			"description": "Analyze which tests are affected by changes and run them. Returns test results with pass/fail status.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"analysis": map[string]any{
						"type":        "string",
						"description": "Analysis type: 'vta' (default), 'cha', 'rta', 'static', or 'coverage'.",
						"default":     "vta",
					},
					"work_dir": map[string]any{
						"type":        "string",
						"description": "Working directory of the Go project. Defaults to current directory.",
						"default":     ".",
					},
					"args": map[string]any{
						"type":        "string",
						"description": "Additional args to pass to the test binary.",
					},
				},
			},
		},
		{
			"name":        "file_impact_analysis",
			"description": "Analyze the impact of file changes: returns affected packages, tests, and changed functions/methods.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"work_dir": map[string]any{
						"type":        "string",
						"description": "Working directory of the Go project. Defaults to current directory.",
						"default":     ".",
					},
					"analysis": map[string]any{
						"type":        "string",
						"description": "Analysis type: 'vta' (default), 'cha', 'rta', 'static'.",
						"default":     "vta",
					},
				},
			},
		},
	}

	s.sendResult(req.ID, map[string]any{"tools": tools})
}

func (s *MCPServer) handleToolsCall(req JSONRPCRequest) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, -32602, "Invalid params", nil)
		return
	}

	switch params.Name {
	case "affected_tests":
		s.toolAffectedTests(req.ID, params.Arguments)
	case "run_affected_tests":
		s.toolRunAffectedTests(req.ID, params.Arguments)
	case "file_impact_analysis":
		s.toolFileImpactAnalysis(req.ID, params.Arguments)
	default:
		s.sendError(req.ID, -32602, fmt.Sprintf("Unknown tool: %s", params.Name), nil)
	}
}

func (s *MCPServer) sendResult(id json.RawMessage, result any) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	s.writeResponse(resp)
}

func (s *MCPServer) sendError(id json.RawMessage, code int, message string, data any) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	s.writeResponse(resp)
}

func (s *MCPServer) writeResponse(resp JSONRPCResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(resp)
	if err != nil {
		s.log.Error("failed to marshal response", "err", err)
		return
	}

	// Write JSON line (newline-delimited)
	_, err = fmt.Fprintf(s.writer, "%s\n", data)
	if err != nil {
		s.log.Error("failed to write response", "err", err)
	}
}

// TODO: Add SSE/HTTP transport support
// TODO: Design section for AI-augmented semantic test detection using embeddings
