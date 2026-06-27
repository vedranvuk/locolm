package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// --- Tool Registry ---

type ToolFunc func(args map[string]string) (string, error)

var toolRegistry = map[string]ToolFunc{
	"google_search":      searchGoogle,
	"web_fetch":          webFetch,
	"memory_save":        memorySave,
	"memory_edit":        memoryEdit,
	"memory_delete":      memoryDelete,
	"memory_load":        memoryLoad,
	"memory_list":        memoryList,
	"memory_delete_bucket": memoryDeleteBucket,
	"memory_list_buckets": memoryListBuckets,
	"fs_run":             runCommand,
	"exa_search":         searchExa,
	"sys_info":          SysInfo,
}

// --- Tool definitions ---

var toolDefinitions = []Tool{
	{
		Name:        "google_search",
		Description: "Search the web using Google Custom Search",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "The search query"
				}
			},
			"required": ["query"]
		}`),
	},
	{
		Name:        "web_fetch",
		Description: "Fetch and read the content of a web page",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"url": {
					"type": "string",
					"description": "The URL of the web page to fetch"
				}
			},
			"required": ["url"]
		}`),
	},
	{
		Name:        "memory_save",
		Description: "Create or update a memory in a bucket. Use this to remember something for future conversations.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"bucket": {
					"type": "string",
					"description": "The bucket (category) to store the memory in (e.g. 'user', 'work', 'general')"
				},
				"key": {
					"type": "string",
					"description": "Unique key for the memory within the bucket (e.g. 'theme_preference')"
				},
				"value": {
					"type": "string",
					"description": "The memory content to store"
				},
				"keywords": {
					"type": "string",
					"description": "Optional comma-separated keywords for better search recall (e.g. 'user, theme, dark')"
				}
			},
			"required": ["bucket", "key", "value"]
		}`),
	},
	{
		Name:        "memory_edit",
		Description: "Update an existing memory's value in a bucket. Fails if the memory doesn't exist.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"bucket": {
					"type": "string",
					"description": "The bucket containing the memory"
				},
				"key": {
					"type": "string",
					"description": "The key of the memory to update"
				},
				"value": {
					"type": "string",
					"description": "The new value for the memory"
				}
			},
			"required": ["bucket", "key", "value"]
		}`),
	},
	{
		Name:        "memory_delete",
		Description: "Delete a specific memory from a bucket.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"bucket": {
					"type": "string",
					"description": "The bucket containing the memory"
				},
				"key": {
					"type": "string",
					"description": "The key of the memory to delete"
				}
			},
			"required": ["bucket", "key"]
		}`),
	},
	{
		Name:        "memory_load",
		Description: "Load a single memory's value from a bucket.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"bucket": {
					"type": "string",
					"description": "The bucket containing the memory"
				},
				"key": {
					"type": "string",
					"description": "The key of the memory to load"
				}
			},
			"required": ["bucket", "key"]
		}`),
	},
	{
		Name:        "memory_list",
		Description: "List memories. Provide a bucket to list only that bucket; omit to list all memories across all buckets.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"bucket": {
					"type": "string",
					"description": "Optional bucket name. If omitted, lists all memories across all buckets."
				}
			},
			"required": []
		}`),
	},
	{
		Name:        "memory_delete_bucket",
		Description: "Delete a bucket and all memories in it.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"bucket": {
					"type": "string",
					"description": "The name of the bucket to delete"
				}
			},
			"required": ["bucket"]
		}`),
	},
	{
		Name:        "memory_list_buckets",
		Description: "List all memory buckets with their memory counts.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {},
			"required": []
		}`),
	},
	{
		Name:        "fs_run",
		Description: "Execute a command and capture its output. Runs via cmd /C on Windows.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {
					"type": "string",
					"description": "The command to execute (e.g. 'dir', 'git status', 'python script.py')"
				},
				"timeout": {
					"type": "string",
					"description": "Optional timeout in seconds (default 30)"
				}
			},
			"required": ["command"]
		}`),
	},
	{
		Name:        "sys_info",
		Description: "Get current system information: date, time, timezone, OS, architecture, hostname, working directory, user, Go version, and uptime. Call this at the start of every conversation to orient yourself.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {},
			"required": []
		}`),
	},
	{
		Name:        "exa_search",
		Description: "Search the web using Exa AI (neural search with highlights and synthesized answers). Requires EXA_API_KEY env var.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "The search query"
				},
				"type": {
					"type": "string",
					"description": "Search type: auto (default), fast, instant, deep, deep-lite, deep-reasoning"
				},
				"num": {
					"type": "string",
					"description": "Number of results (default 10)"
				},
				"include_domains": {
					"type": "string",
					"description": "Comma-separated list of domains to restrict search to (e.g. 'github.com,stackoverflow.com')"
				},
				"exclude_domains": {
					"type": "string",
					"description": "Comma-separated list of domains to exclude from results"
				},
				"start_date": {
					"type": "string",
					"description": "Start date filter (e.g. '2025-01-01' or '2025-01-01T00:00:00Z')"
				},
				"end_date": {
					"type": "string",
					"description": "End date filter (e.g. '2025-12-31' or '2025-12-31T23:59:59Z')"
				},
				"system_prompt": {
					"type": "string",
					"description": "System prompt to guide synthesis behavior (used with output_schema)"
				},
				"output_schema": {
					"type": "string",
					"description": "JSON Schema string for structured output (triggers synthesis)"
				}
			},
			"required": ["query"]
		}`),
	},
}

// --- MCP Handler ---

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
		Result:  map[string]interface{}{"tools": toolDefinitions},
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

	toolFunc, ok := toolRegistry[params.Name]
	if !ok {
		writeError(w, req.ID, -32601, "Unknown tool: "+params.Name)
		return
	}

	result, err := toolFunc(params.Arguments)
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
