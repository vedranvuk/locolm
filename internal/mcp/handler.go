// Package mcp provides the MCP server implementation using the official
// go-sdk (github.com/modelcontextprotocol/go-sdk). It exposes a single HTTP handler
// that serves the Streamable HTTP transport as defined by the MCP spec.
package mcp

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed instructions.txt
var instructions string

// MCPHandler is a MCP handler function prototype.
type HandlerFunc func(map[string]string) (string, error)

// Registry is an interface to MCP tool registry.
type Registry interface {
	RegisterTool(name, description string, inputSchema json.RawMessage, handler HandlerFunc)
}

// Handler wraps the go-sdk mcp.Handler and provides an HTTP handler.
type Handler struct {
	mcpServer *mcp.Server
	handler   http.Handler
}

// New creates a new MCP handler.
func New() *Handler {
	var mcpServer = mcp.NewServer(
		&mcp.Implementation{
			Name:    "locolm",
			Title:   "locolm",
			Version: "1.0.0",
		},
		&mcp.ServerOptions{
			Instructions: instructions,
		},
	)
	var handler = mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server {
			return mcpServer
		},
		&mcp.StreamableHTTPOptions{
			DisableLocalhostProtection: true,
		},
	)
	return &Handler{mcpServer, handler}
}

// RegisterTool registers a MCP tool with the MCP [Handler].
func (self *Handler) RegisterTool(name, description string, inputSchema json.RawMessage, handler HandlerFunc) {
	tool := &mcp.Tool{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
	}

	self.mcpServer.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
func (self *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers for cross-origin browser requests
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, mcp-Session-Id, Mcp-Protocol-Version")
	w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id, Mcp-Protocol-Version")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	self.handler.ServeHTTP(w, r)
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
