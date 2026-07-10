// Package mcp provides the MCP server implementation using the official
// go-sdk (github.com/modelcontextprotocol/go-sdk). It exposes a single HTTP handler
// that serves the Streamable HTTP transport as defined by the MCP spec.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const mcpInstructions = `You are Dave, an AI assistant with access to powerful tools for web search, filesystem operations, code analysis, and knowledge retrieval.

ALWAYS START EVERY CONVERSATION with sys_info to establish context. This is mandatory and must be your first tool call.

For filesystem operations, ALWAYS validate paths are within allowed directories. Use fs_list to explore directories before reading or writing. For editing files, use fs_replace with exact matching. Use fs_read with offset/limit for large files. Never write more than 1MB at once.

For code tasks, always activate the workspace first with gopls_workspace_activate. Then use gopls_definition, gopls_references, or gopls_implementation to navigate code. Use gopls_diagnostics to check for errors before refactoring. Use gopls_rename for safe renaming across files. Use gopls_symbols for fuzzy searches.

For web research, ALWAYS start with google_search for general queries. For deep research or structured data, use exa_search with appropriate search types. When you need to read specific web pages, use web_fetch. For PDFs, web_fetch extracts text automatically.

For knowledge and context, use add_observations to store important facts about entities. Use get_entity_context before making decisions about known entities. Use search_memory when you need historical context. For semantic similarity, use remember_semantic to store and recall_semantic to retrieve.

For Wolfram Alpha queries, ALWAYS use wolfram_llm for most questions as it provides structured, LLM-friendly output. Use wolfram_query when you need specific pod-level data. Use wolfram_short for simple factual questions. Use wolfram_recognize to check if a query is computable before running it.

For Wikidata, use wikidata_query with mode 'entity' for specific Q-IDs, 'search' for text searches, or 'sparql' for complex queries. The tool automatically adds language labels and normalizes queries.

For news, use news_search with mode 'everything' for article searches or 'headlines' for top stories. Use news_sources to discover available sources.

For command execution, use fs_run only when absolutely necessary. Commands are sandboxed and can be restricted by regex patterns. Always check output carefully.

For image results from Wolfram, use wolfram_image to get rendered visual outputs.

When multiple tools could apply, choose the most specific one. When in doubt about a tool's capability, use wolfram_recognize or search tools first to understand options.

Never hallucinate tool capabilities. If a tool is not available or fails, explain the limitation clearly.

For complex multi-step tasks, break them down: gather context first, then execute step by step, then synthesize results.

Always prefer tools over general knowledge when tool-specific data is available.`

// Server wraps the go-sdk mcp.Server and provides an HTTP handler.
type Server struct {
	mcpServer *mcp.Server
	handler   http.Handler
}

// New creates a new MCP server. Tools registered via RegisterTool before
// this call are replayed into the new server.
func New() *Server {
	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "locolm",
			Title:   "locolm",
			Version: "1.0.0",
		}, 
		&mcp.ServerOptions{
			Instructions: mcpInstructions,	
		},
	)

	s := &Server{
		mcpServer: mcpServer,
	}

	// Replay any registrations that arrived before the server was created
	for _, reg := range pendingRegistrations {
		reg(s)
	}
	pendingRegistrations = nil

	return s
}

// AddTool adds a tool to the server. Each tool package calls this from its
// init() function, analogous to the previous self-registering pattern.
//
// The handler receives raw arguments as map[string]string (matching the
// existing ToolFunc signature) and returns a string result or error.
// The inputSchema is a JSON Schema object (as json.RawMessage).
func (s *Server) AddTool(name, description string, inputSchema json.RawMessage, handler func(args map[string]string) (string, error)) {
	tool := &mcp.Tool{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
	}

	s.mcpServer.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse arguments from the raw JSON-RPC params
		args := make(map[string]string)
		if req.Params.Arguments != nil {
			// Sanitize raw arguments: LLMs sometimes emit literal newlines or
			// other control characters inside JSON string values, which produces
			// invalid JSON. Replace them with escaped equivalents before parsing.
			sanitized := sanitizeRawJSON(req.Params.Arguments)

			var rawArgs map[string]json.RawMessage
			if err := json.Unmarshal(sanitized, &rawArgs); err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						&mcp.TextContent{Text: "failed to parse arguments: " + err.Error()},
					},
					IsError: true,
				}, nil
			}
			for k, v := range rawArgs {
				var str string
				if err := json.Unmarshal(v, &str); err == nil {
					args[k] = str
				} else {
					// If it's not a string, store the raw JSON
					args[k] = string(v)
				}
			}
		}

		result, err := handler(args)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: err.Error()},
				},
				IsError: true,
			}, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: result},
			},
		}, nil
	})

	log.Printf("[MCP] Registered tool: %s", name)
}

// ServeHTTP implements http.Handler. It delegates to the SDK's
// StreamableHTTPHandler, which handles the Streamable HTTP transport
// (POST for JSON-RPC messages, GET for SSE).
// CORS headers are added so the browser-based llama-ui (on a different port)
// can reach this server.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers for cross-origin browser requests
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, mcp-Session-Id, Mcp-Protocol-Version")
	w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id, Mcp-Protocol-Version")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if s.handler == nil {
		s.handler = mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return s.mcpServer
		}, &mcp.StreamableHTTPOptions{
			DisableLocalhostProtection: true,
		})
	}
	s.handler.ServeHTTP(w, r)
}

// pendingRegistrations holds tool registrations from init() calls that
// arrived before a server was created. They are replayed into the next
// server created by New().
var pendingRegistrations []func(*Server)

// RegisterTool is called from tool packages' init() functions to register
// a tool. The registration is queued and replayed when New() is called.
func RegisterTool(name, description string, inputSchema json.RawMessage, handler func(args map[string]string) (string, error)) {
	pendingRegistrations = append(pendingRegistrations, func(s *Server) {
		s.AddTool(name, description, inputSchema, handler)
	})
}

// sanitizeRawJSON replaces literal control characters (newlines, tabs, etc.)
// inside JSON string values with their escaped equivalents. LLMs sometimes
// emit raw newlines within JSON string values (e.g. in SPARQL queries),
// which produces invalid JSON that the parser rejects.
//
// This function tracks whether we are inside a JSON string (respecting
// backslash escapes) and replaces control characters with their \uXXXX
// escape sequences.
//
// Note: We do NOT convert double-escaped sequences (\\n, \\t, \\r) to their
// single-escaped equivalents. While LLMs sometimes emit \\n when they mean a
// newline, this is ambiguous: \\n in valid JSON also represents a literal
// backslash followed by n/t/r, which occurs in Windows paths (e.g. C:\\new,
// C:\\temp, D:\\root). Converting these would corrupt paths. Tool-specific
// normalization (e.g. SPARQL query handling) is done after JSON parsing in
// the respective tool handlers, where semantic context is available.
func sanitizeRawJSON(raw []byte) []byte {
	var buf strings.Builder
	buf.Grow(len(raw))

	inString := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]

		if inString {
			if c == '\\' {
				// Escape sequence: copy the backslash and the next character
				buf.WriteByte(c)
				if i+1 < len(raw) {
					i++
					buf.WriteByte(raw[i])
				}
				continue
			}
			if c == '"' {
				// End of string
				inString = false
				buf.WriteByte(c)
				continue
			}
			// Inside a string: replace control characters
			switch c {
			case '\n':
				buf.WriteString(`\n`)
			case '\r':
				buf.WriteString(`\r`)
			case '\t':
				buf.WriteString(`\t`)
			case '\b':
				buf.WriteString(`\b`)
			case '\f':
				buf.WriteString(`\f`)
			default:
				if c < 0x20 {
					buf.WriteString(fmt.Sprintf(`\u%04x`, c))
				} else {
					buf.WriteByte(c)
				}
			}
		} else {
			if c == '"' {
				inString = true
			}
			buf.WriteByte(c)
		}
	}

	return []byte(buf.String())
}
