package main

import (
	"database/sql"
	"log"
	"os"
	"os/signal"

	"github.com/vedranvuk/locolm/internal/config"
	"github.com/vedranvuk/locolm/internal/database"
	"github.com/vedranvuk/locolm/internal/server"
	"github.com/vedranvuk/locolm/internal/tool/exasearch"
	"github.com/vedranvuk/locolm/internal/tool/exec"
	"github.com/vedranvuk/locolm/internal/tool/fs"
	"github.com/vedranvuk/locolm/internal/tool/gopls"
	"github.com/vedranvuk/locolm/internal/tool/gsearch"
	"github.com/vedranvuk/locolm/internal/tool/memory"
	"github.com/vedranvuk/locolm/internal/tool/newsapi"
	"github.com/vedranvuk/locolm/internal/tool/rag"
	"github.com/vedranvuk/locolm/internal/tool/sysinfo"
	"github.com/vedranvuk/locolm/internal/tool/web"
	"github.com/vedranvuk/locolm/internal/tool/wikidata"
	"github.com/vedranvuk/locolm/internal/tool/wolfram"
)

func main() {

	var (
		err           error
		cfg           *config.Config
		httpServer    *server.Server
		db            *sql.DB
		serverChan    = make(chan error)
		interruptChan = make(chan os.Signal, 1)
	)

	// Init infra.

	if cfg, err = config.Load(); err != nil {
		log.Fatalf("load config: %v", err)
	}

	if httpServer, err = server.New(cfg.MCPServer); err != nil {
		log.Fatalf("create server: %v", err)
	}
	
	if db, err = database.Open(cfg.Database); err != nil {
		log.Fatalf("initialize database: %v", err)
	}
	defer db.Close()
	
	// Initialize and register tools.
	{
		var (
			exaSearchTool    *exasearch.ExaSearch
			execTool         *exec.ExecTool
			fsTool           *fs.FSTool
			goplsTool        *gopls.Gopls
			googleSearchTool *gsearch.GoogleSearch
			memTool          *memory.MemoryTool
			newsAPITool      *newsapi.NewsAPITool
			ragTool          *rag.RAGTool
			sysInfoTool      *sysinfo.SysInfoTool
			webFetchTool     *web.WebFetchTool
			wikidataTool     *wikidata.WikidataTool
			wolframTool      *wolfram.WolframTool
		)
		if exaSearchTool, err = exasearch.New(cfg.ExaSearch); err != nil {
			log.Fatalf("initialize exasearch: %v", err)
		}
		if execTool, err = exec.New(cfg.Exec); err != nil {
			log.Fatalf("initialize exec: %v", err)
		}
		if fsTool, err = fs.New(cfg.FS); err != nil {
			log.Fatalf("intialize fs: %v", err)
		}
		if goplsTool, err = gopls.New(cfg.Gopls); err != nil {
			log.Fatalf("intialize fs: %v", err)
		}
		if googleSearchTool, err = gsearch.New(cfg.GoogleSearch); err != nil {
			log.Fatalf("initialize gsearch: %v", err)
		}
		if memTool, err = memory.New(cfg.Memory, db); err != nil {
			log.Fatalf("initialize memory: %v", err)
		}
		if newsAPITool, err = newsapi.New(cfg.NewsAPI); err != nil {
			log.Fatalf("initialize newsapi: %v", err)
		}
		if ragTool, err = rag.New(cfg.RAG, db); err != nil {
			log.Fatalf("initialize rag: %v", err)
		}
		if sysInfoTool, err = sysinfo.New(cfg.SysInfo); err != nil {
			log.Fatalf("initialize sysinfo: %v", err)
		}
		if wikidataTool, err = wikidata.New(cfg.Wikidata); err != nil {
			log.Fatalf("initialize wikidata: %v", err)
		}
		if wolframTool, err = wolfram.New(cfg.Wolfram); err != nil {
			log.Fatalf("initialize wolfram: %v", err)
		}
		if webFetchTool, err = web.New(cfg.WebFetch); err != nil {
			log.Fatalf("initialize web fetch: %v", err)
		}
		httpServer.RegisterMCPTools(exaSearchTool)
		httpServer.RegisterMCPTools(execTool)
		httpServer.RegisterMCPTools(fsTool)
		httpServer.RegisterMCPTools(goplsTool)
		httpServer.RegisterMCPTools(googleSearchTool)
		httpServer.RegisterMCPTools(memTool)
		httpServer.RegisterMCPTools(newsAPITool)
		httpServer.RegisterMCPTools(ragTool)
		httpServer.RegisterMCPTools(sysInfoTool)
		httpServer.RegisterMCPTools(wikidataTool)
		httpServer.RegisterMCPTools(wolframTool)
		httpServer.RegisterMCPTools(webFetchTool)
	}

	// Run server.
	go func() {
		serverChan <- httpServer.Run()
	}()

	log.Println("server running...")

	// Loop.
	signal.Notify(interruptChan, os.Interrupt)
	select {
	case err := <-serverChan:
		log.Fatalf("server error: %v", err)
	case <-interruptChan:
		if err := httpServer.Stop(); err != nil {
			log.Fatalf("server stop error: %v", err)
		}
	}

	// Exit.
	log.Println("bye.")
}
