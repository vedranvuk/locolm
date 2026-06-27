package mcp

import (
	"encoding/json"
	"log"
	"net/http"

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
	w.Header().Set("Content-Type", "application/json")

	log.Printf("[REQUEST] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	case "notifications/cancelled":
		log.Printf("[MCP] Received cancelled notification")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
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
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
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
	json.NewEncoder(w).Encode(resp)
}

func handleToolsList(w http.ResponseWriter, req JSONRPCRequest) {
	log.Printf("[MCP] tools/list request")
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"tools": tool.Definitions()},
	}
	json.NewEncoder(w).Encode(resp)
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
		writeError(w, req.ID, -32603, err.Error())
		return
	}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": result,
				},
			},
		},
	}
	json.NewEncoder(w).Encode(resp)
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
	json.NewEncoder(w).Encode(resp)
}
