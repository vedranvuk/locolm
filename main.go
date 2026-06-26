package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Load config from locolm.json + GOOGLE_* env overrides
	cfg := LoadConfig()

	port := cfg.MCPPort
	if port == "" {
		log.Fatal("[LOCOLM] mcp_port is required in locolm.json")
	}

	// Start MCP server first so it's ready when llama-server connects
	mux := http.NewServeMux()
	mux.HandleFunc("/", mcpHandler)
	server := &http.Server{Addr: ":" + port, Handler: mux}

	go func() {
		log.Printf("MCP server starting on :%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("MCP server failed: %v", err)
		}
	}()

	// Bootstrap: llama-server → browser
	Bootstrap(cfg.LlamaServerCommand, cfg.BrowserCommand)

	// Wait for shutdown signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Printf("[LOCOLM] Shutting down...")

	// Graceful shutdown: stop llama-server, then close MCP server
	StopLlama()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(ctx)

	log.Printf("[LOCOLM] Shutdown complete.")
}
