package server

import (
	"context"
	"net/http"

	"github.com/vedranvuk/locolm/internal/mcp"
)

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
	run        chan error
	config     *Config
	httpServer http.Server
	mcpHandler *mcp.Handler
}

func New(config *Config) (*Server, error) {
	var handler = mcp.New()
	return &Server{
		run:    make(chan error, 1),
		config: config,
		httpServer: http.Server{
			Addr:    config.Addr,
			Handler: handler,
		},
		mcpHandler: handler,
	}, nil
}

func (self *Server) Run() (err error) {
	if self.config.TLS {
		err = self.httpServer.ListenAndServeTLS(self.config.TLSCertFile, self.config.TLSKeyFile)
	} else {
		err = self.httpServer.ListenAndServe()

	}
	if err == http.ErrServerClosed {
		err = nil
	}
	return
}

func (self *Server) Stop() error { return self.httpServer.Shutdown(context.Background()) }

func (self *Server) RegisterMCPTools(tools ...mcp.Tool) {
	for _, tool := range tools {
		tool.Register(self.mcpHandler)
	}
}
