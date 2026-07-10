package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/vedranvuk/locolm/internal/database"
	"github.com/vedranvuk/locolm/internal/server"
	"github.com/vedranvuk/locolm/internal/tool/exasearch"
	"github.com/vedranvuk/locolm/internal/tool/gsearch"
	"github.com/vedranvuk/locolm/internal/tool/memory"
	"github.com/vedranvuk/locolm/internal/tool/newsapi"
	"github.com/vedranvuk/locolm/internal/tool/rag"
	"github.com/vedranvuk/locolm/internal/tool/sysinfo"
	"github.com/vedranvuk/locolm/internal/tool/wikidata"
	"github.com/vedranvuk/locolm/internal/tool/wolfram"
)

// Config holds all locolm configuration. Loaded from locolm.json with
// GOOGLE_* / EXA_* env overrides for third-party credentials.
type Config struct {
	MCPServer *server.Config `json:"mcp_server"`

	Database *database.Config `json:"database"`

	ExaSearch *exasearch.Config `json:"exa_search"`

	// Tool configurations
	GoogleSearch *gsearch.Config  `json:"google_search"`
	Memory       *memory.Config   `json:"memory"`
	NewsAPI      *newsapi.Config  `json:"newsapi"`
	RAG          *rag.Config      `json:"rag"`
	SysInfo      *sysinfo.Config  `json:"sysinfo"`
	Wikidata     *wikidata.Config `json:"wikidata"`
	Wolfram      *wolfram.Config  `json:"wolfram"`

	// Raw tool configs — each tool package unmarshals its own.
	WebFetch json.RawMessage `json:"web_fetch,omitempty"`
	FS       json.RawMessage `json:"fs,omitempty"`
	Exec     json.RawMessage `json:"exec,omitempty"`
}

// Load reads locolm.json, applies GOOGLE_* env overrides, dispatches
// tool-specific configs to registered loaders, and returns the resolved config.
// locolm.json is read from the working directory (go run .) or the exe directory.
func Load() *Config {

	var cfg = &Config{
		MCPServer:    server.DefaultConfig(),
		Database:     database.DefaultConfig(),
		ExaSearch:    exasearch.DefaultConfig(),
		GoogleSearch: gsearch.DefaultConfig(),
		Memory:       memory.DefaultConfig(),
		NewsAPI:      newsapi.DefaultConfig(),
		RAG:          rag.DefaultConfig(),
		SysInfo:      sysinfo.DefaultConfig(),
		Wikidata:     wikidata.DefaultConfig(),
		Wolfram:      wolfram.DefaultConfig(),
	}

	wd, _ := os.Getwd()
	jsonPath := filepath.Join(wd, "locolm.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		exePath, e := os.Executable()
		if e == nil {
			data, err = os.ReadFile(filepath.Join(filepath.Dir(exePath), "locolm.json"))
		}
	}
	if err == nil {
		json.Unmarshal(data, &cfg)
	}

	return cfg
}
