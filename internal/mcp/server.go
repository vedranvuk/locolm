// Package mcp provides the MCP server implementation using the official
// go-sdk (github.com/modelcontextprotocol/go-sdk). It exposes a single HTTP handler
// that serves the Streamable HTTP transport as defined by the MCP spec.
package mcp

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the go-sdk mcp.Server and provides an HTTP handler.
type Server struct {
	sdkServer *mcp.Server
	handler   http.Handler
}

// New creates a new MCP server. Tools registered via RegisterTool before
// this call are replayed into the new server.
func New() *Server {
	sdkServer := mcp.NewServer(&mcp.Implementation{
		Name:    "locolm",
		Version: "1.0.0",
	}, nil)

	s := &Server{
		sdkServer: sdkServer,
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

	s.sdkServer.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse arguments from the raw JSON-RPC params
		args := make(map[string]string)
		if req.Params.Arguments != nil {
			var rawArgs map[string]json.RawMessage
			if err := json.Unmarshal(req.Params.Arguments, &rawArgs); err != nil {
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
			return s.sdkServer
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
