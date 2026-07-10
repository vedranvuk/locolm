package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/vedranvuk/locolm/internal/mcp"
)

// MCPTOol defines a MCP tool.
type MCPTool interface {
	// Register allows MCP tools to register themselves with the MCP server.
	Register(mcp.Registry)
}

type Config struct {
	Addr        string
	TLS         bool
	TLSCertFile string
	TLSKeyFile  string
}

func DefaultConfig() *Config {
	return &Config{
		Addr: "0.0.0.0:11501",
	}
}

type Server struct {
	run     chan error
	config  *Config
	server  http.Server
	handler *mcp.Handler
}

func New(config *Config) (*Server, error) {
	return &Server{
		run:    make(chan error, 1),
		config: config,
		server: http.Server{
			Addr: config.Addr,
		},
		handler: mcp.New(),
	}, nil
}

func (self *Server) Run() (err error) {
	if self.config.TLS {
		err = self.server.ListenAndServeTLS(self.config.TLSCertFile, self.config.TLSKeyFile)
	} else {
		err = self.server.ListenAndServe()

	}
	if err == http.ErrServerClosed {
		err = nil
	}
	return
}

func (self *Server) Stop() error { return self.server.Shutdown(context.Background()) }

func (self *Server) RegisterTool(name, description string, inputSchema json.RawMessage, handler mcp.HandlerFunc) {
	self.handler.RegisterTool(name, description, inputSchema, handler)
}

func (self *Server) RegisterMCPTools(tools ...MCPTool) {
	for _, tool := range tools {
		tool.Register(self)
	}
}
