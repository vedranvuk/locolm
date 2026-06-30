package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vedranvuk/locolm/internal/config"
	"github.com/vedranvuk/locolm/internal/mcp"
	"github.com/vedranvuk/locolm/internal/tool/exec"
	"github.com/vedranvuk/locolm/internal/tool/fs"
	"github.com/vedranvuk/locolm/internal/tool/web"

	// Blank imports: trigger init() in each tool package, which registers
	// its tools via mcp.RegisterTool (replayed into the server in main).
	_ "github.com/vedranvuk/locolm/internal/tool/memory"
	_ "github.com/vedranvuk/locolm/internal/tool/newsapi"
	_ "github.com/vedranvuk/locolm/internal/tool/search"
	_ "github.com/vedranvuk/locolm/internal/tool/sysinfo"
	_ "github.com/vedranvuk/locolm/internal/tool/wikidata"
	_ "github.com/vedranvuk/locolm/internal/tool/wolfram"
)

func main() {
	// Load config from locolm.json + GOOGLE_* env overrides
	cfg := config.LoadConfig()

	// Load tool-specific configs
	web.LoadWebFetchConfig(cfg.WebFetch)
	fs.LoadFSConfig(cfg.FS)
	exec.LoadExecConfig(cfg.Exec)

	port := cfg.MCPPort
	if port == "" {
		log.Fatal("[LOCOLM] mcp_port is required in locolm.json")
	}

	// Create MCP server — init() functions in tool packages register tools
	// via mcp.RegisterTool, which are replayed into this server.
	mcpServer := mcp.New()

	// Start MCP server first so it's ready when llama-server connects
	server := &http.Server{Addr: ":" + port, Handler: mcpServer}

	go func() {
		log.Printf("MCP server starting on :%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("MCP server failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Printf("[LOCOLM] Shutting down...")

	// Graceful shutdown: close MCP server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(ctx)

	log.Printf("[LOCOLM] Shutdown complete.")
}
