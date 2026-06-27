package mcp

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/vedranvuk/locolm/internal/tool"
)

// Handler returns an http.Handler that serves the MCP JSON-RPC 2.0 protocol.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", mcpHandler)
	return mux
}

func mcpHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, mcp-protocol-version")

	// Read raw body for debug logging
	rawBody, _ := io.ReadAll(r.Body)
	log.Printf("[REQUEST] %s %s from %s body=%s", r.Method, r.URL.Path, r.RemoteAddr, string(rawBody))

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Log non-JSON-RPC requests (e.g. GET for SSE)
	if r.Method != http.MethodPost {
		log.Printf("[MCP] Non-POST request, returning 405")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"error":"only POST supported for JSON-RPC"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req JSONRPCRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		log.Printf("[ERROR] Failed to decode request: %v", err)
		writeError(w, nil, -32700, "Parse error")
		return
	}

	log.Printf("[MCP] Method: %s ID: %v", req.Method, req.ID)

	switch req.Method {
	case "initialize":
		handleInitialize(w, req)
	case "notifications/initialized":
		log.Printf("[MCP] Received initialized notification")
		w.WriteHeader(http.StatusNoContent)
	case "notifications/cancelled":
		log.Printf("[MCP] Received cancelled notification")
		w.WriteHeader(http.StatusNoContent)
	case "tools/list":
		handleToolsList(w, req)
	case "tools/call":
		handleToolsCall(w, req)
	default:
		log.Printf("[WARN] Unknown method: %s", req.Method)
		writeError(w, req.ID, -32601, "Method not found")
	}
}

func handleInitialize(w http.ResponseWriter, req JSONRPCRequest) {
	log.Printf("[MCP] initialize request")
	// Negotiate protocol version: use the client's version if it's supported,
	// otherwise fall back to our preferred version.
	protocolVersion := "2024-11-05"
	if req.Params != nil {
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		json.Unmarshal(req.Params, &params)
		if params.ProtocolVersion != "" {
			protocolVersion = params.ProtocolVersion
		}
	}
	result := map[string]interface{}{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "locolm",
			"version": "1.0.0",
		},
	}
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
	data, _ := json.Marshal(resp)
	log.Printf("[MCP] initialize response: %s", string(data))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

func handleToolsList(w http.ResponseWriter, req JSONRPCRequest) {
	log.Printf("[MCP] tools/list request")
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"tools": tool.Definitions()},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal tools/list response: %v", err)
		writeError(w, req.ID, -32603, "Internal error")
		return
	}
	log.Printf("[MCP] tools/list response: %d bytes", len(data))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

func handleToolsCall(w http.ResponseWriter, req JSONRPCRequest) {
	var params struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("[ERROR] Failed to parse tool call params: %v", err)
		writeError(w, req.ID, -32602, "Invalid params")
		return
	}

	log.Printf("[MCP] tools/call: %s with args: %v", params.Name, params.Arguments)

	t, ok := tool.Get(params.Name)
	if !ok {
		writeError(w, req.ID, -32601, "Unknown tool: "+params.Name)
		return
	}

	result, err := t.Func(params.Arguments)
	if err != nil {
		log.Printf("[ERROR] Tool %s failed: %v", params.Name, err)
		// Per MCP spec: return tool error as a result with isError: true,
		// not as a JSON-RPC error. This lets the LLM see the error context.
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type":    "text",
						"text":    err.Error(),
						"isError": true,
					},
				},
			},
		}
		data, _ := json.Marshal(resp)
		log.Printf("[MCP] tools/call error response: %s", string(data))
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Write(data)
		return
	}

	// If the tool result is already valid JSON, pass it through as raw JSON
	// to avoid double-escaping. Otherwise, wrap it as a text string.
	var contentText interface{} = result
	if json.Valid([]byte(result)) {
		contentText = json.RawMessage(result)
	}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": contentText,
				},
			},
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal tools/call response: %v", err)
		writeError(w, req.ID, -32603, "Internal error")
		return
	}
	log.Printf("[MCP] tools/call response: %d bytes, isJSON=%v", len(data), contentText != result)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

func writeError(w http.ResponseWriter, id interface{}, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
	data, _ := json.Marshal(resp)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}
