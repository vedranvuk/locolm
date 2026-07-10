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
	"github.com/vedranvuk/locolm/internal/tool/gopls"
	"github.com/vedranvuk/locolm/internal/tool/web"

	// Blank imports: trigger init() in each tool package, which registers
	// its tools via mcp.RegisterTool (replayed into the server in main).
	_ "github.com/vedranvuk/locolm/internal/tool/memory"
	_ "github.com/vedranvuk/locolm/internal/tool/newsapi"
	_ "github.com/vedranvuk/locolm/internal/tool/rag"
	_ "github.com/vedranvuk/locolm/internal/tool/search"
	_ "github.com/vedranvuk/locolm/internal/tool/sysinfo"
	_ "github.com/vedranvuk/locolm/internal/tool/wikidata"
	_ "github.com/vedranvuk/locolm/internal/tool/wolfram"
)

func main() {
	// Load config from locolm.json + GOOGLE_* env overrides
	cfg := config.Load()

	// Load tool-specific configs
	web.LoadWebFetchConfig(cfg.WebFetch)
	fs.LoadFSConfig(cfg.FS)
	exec.LoadExecConfig(cfg.Exec)
	gopls.LoadGoplsConfig(cfg.Gopls)

	port := cfg.MCPPort
	if port == "" {
		log.Fatal("[LOCOLM] mcp_port is required in locolm.json")
	}

	// Create MCP server — init() functions in tool packages register tools
	mcpServer := mcp.New()

	server := &http.Server{
		Addr:         "0.0.0.0:" + port,
		Handler:      mcpServer,
		ReadTimeout:  15 * time.Second,  // Added timeouts to prevent hanging connections
		WriteTimeout: 15 * time.Second,
	}

	// Channel to block main until the graceful shutdown routine completes
	shutdownDone := make(chan struct{})

	// Background routine listening for OS lifecycle traps
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		sig := <-sigChan
		log.Printf("[LOCOLM] Caught signal %v, initiating graceful shutdown...", sig)

		// Graceful shutdown timeout ceiling
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Shutdown gracefully sheds active connections and stops listening
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("[LOCOLM] MCP server forced to shutdown: %v", err)
		}
		
		// Unblock the main goroutine
		close(shutdownDone)
	}()

	log.Printf("MCP server starting on :%s", port)
	
	// ListenAndServe blocks right here on the main routine under normal operation.
	// When server.Shutdown() triggers above, it unblocks immediately returning ErrServerClosed.
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("MCP server runtime failure: %v", err)
	}

	// Ensure we don't drop out of main until the shutdown procedures fully conclude
	<-shutdownDone
	log.Printf("[LOCOLM] Shutdown complete.")
}