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
	"github.com/vedranvuk/locolm/internal/tool/gsearch"
	"github.com/vedranvuk/locolm/internal/tool/memory"
	"github.com/vedranvuk/locolm/internal/tool/newsapi"
	"github.com/vedranvuk/locolm/internal/tool/rag"
	"github.com/vedranvuk/locolm/internal/tool/sysinfo"
	"github.com/vedranvuk/locolm/internal/tool/web"
	"github.com/vedranvuk/locolm/internal/tool/wikidata"
	"github.com/vedranvuk/locolm/internal/tool/wolfram"

	// Blank imports: trigger init() in each tool package, which registers
	// its tools via mcp.RegisterTool (replayed into the server in main).
	_ "github.com/vedranvuk/locolm/internal/tool/exasearch"
	_ "github.com/vedranvuk/locolm/internal/tool/gsearch"
	_ "github.com/vedranvuk/locolm/internal/tool/memory"
	_ "github.com/vedranvuk/locolm/internal/tool/newsapi"
	_ "github.com/vedranvuk/locolm/internal/tool/rag"
	_ "github.com/vedranvuk/locolm/internal/tool/sysinfo"
	_ "github.com/vedranvuk/locolm/internal/tool/wikidata"
	_ "github.com/vedranvuk/locolm/internal/tool/wolfram"
)

func main() {
	var (
		err           error
		cfg           = config.Load()
		httpServer    *server.Server
		serverChan    = make(chan error)
		interruptChan = make(chan os.Signal, 1)
	)

	web.LoadWebFetchConfig(cfg.WebFetch)

	signal.Notify(interruptChan, os.Interrupt)

	if httpServer, err = server.New(cfg.MCPServer); err != nil {
		log.Fatalf("create server: %v", err)
	}
	{
		var db *sql.DB
		if db, err = database.Open(cfg.Database); err != nil {
			log.Fatalf("initialize database: %v", err)
		}

		var exaSearch *exasearch.ExaSearch
		if exaSearch, err = exasearch.New(cfg.ExaSearch); err != nil {
			log.Fatalf("initialize exasearch: %v", err)
		}

		httpServer.RegisterMCPTools(exaSearch)
	}
	{
		var googleSearch *gsearch.GoogleSearch
		if googleSearch, err = gsearch.New(cfg.GoogleSearch); err != nil {
			log.Fatalf("initialize gsearch: %v", err)
		}
		httpServer.RegisterMCPTools(googleSearch)
	}
	{
		var memTool *memory.MemoryTool
		if memTool, err = memory.New(cfg.Memory, db); err != nil {
			log.Fatalf("initialize memory: %v", err)
		}
		httpServer.RegisterMCPTools(memTool)
	}
	{
		var newsAPI *newsapi.NewsAPITool
		if newsAPI, err = newsapi.New(cfg.NewsAPI); err != nil {
			log.Fatalf("initialize newsapi: %v", err)
		}
		httpServer.RegisterMCPTools(newsAPI)
	}
	{
		var rag *rag.RAGTool
		if rag, err = rag.New(cfg.RAG); err != nil {
			log.Fatalf("initialize rag: %v", err)
		}
		httpServer.RegisterMCPTools(rag)
	}
	{
		var sysInfo *sysinfo.SysInfoTool
		if sysInfo, err = sysinfo.New(cfg.SysInfo); err != nil {
			log.Fatalf("initialize sysinfo: %v", err)
		}
		httpServer.RegisterMCPTools(sysInfo)
	}
	{
		var wikidata *wikidata.WikidataTool
		if wikidata, err = wikidata.New(cfg.Wikidata); err != nil {
			log.Fatalf("initialize wikidata: %v", err)
		}
		httpServer.RegisterMCPTools(wikidata)
	}
	{
		var wolfram *wolfram.WolframTool
		if wolfram, err = wolfram.New(cfg.Wolfram); err != nil {
			log.Fatalf("initialize wolfram: %v", err)
		}
		httpServer.RegisterMCPTools(wolfram)
	}
	{
		var webFetch *web.WebFetchTool
		if webFetch, err = web.New(cfg.WebFetch); err != nil {
			log.Fatalf("initialize web fetch: %v", err)
		}
		httpServer.RegisterMCPTools(webFetch)
	}

	go func() {
		serverChan <- httpServer.Run()
	}()

	log.Println("server running...")

	select {
	case err := <-serverChan:
		log.Fatalf("server error: %v", err)
	case <-interruptChan:
		if err := httpServer.Stop(); err != nil {
			log.Fatalf("server stop error: %v", err)
		}
	}
	log.Println("bye.")
}
