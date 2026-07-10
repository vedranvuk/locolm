package config

import (
	"encoding/json"
	"errors"
	"log"
	"os"

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

// Config holds all locolm configuration. Loaded from locolm.json with
// GOOGLE_* / EXA_* env overrides for third-party credentials.
type Config struct {
	MCPServer *server.Config   `json:"mcp_server"`
	Database  *database.Config `json:"database"`

	ExaSearch    *exasearch.Config `json:"exa_search"`
	Exec         *exec.Config      `json:"exec,omitempty"`
	FS           *fs.Config        `json:"fs,omitempty"`
	Gopls        *gopls.Config     `json:"gopls"`
	GoogleSearch *gsearch.Config   `json:"google_search"`
	Memory       *memory.Config    `json:"memory"`
	NewsAPI      *newsapi.Config   `json:"newsapi"`
	RAG          *rag.Config       `json:"rag"`
	SysInfo      *sysinfo.Config   `json:"sysinfo"`
	WebFetch     *web.Config       `json:"web_fetch,omitempty"`
	Wikidata     *wikidata.Config  `json:"wikidata"`
	Wolfram      *wolfram.Config   `json:"wolfram"`
}

// Load reads locolm.json, applies GOOGLE_* env overrides, dispatches
// tool-specific configs to registered loaders, and returns the resolved config.
// locolm.json is read from the working directory (go run .) or the exe directory.
func Load() (*Config, error) {

	var cfg = &Config{
		MCPServer: server.DefaultConfig(),
		Database:  database.DefaultConfig(),

		ExaSearch:    exasearch.DefaultConfig(),
		Exec:         exec.DefaultConfig(),
		FS:           fs.DefaultConfig(),
		Gopls:        gopls.DefaultConfig(),
		GoogleSearch: gsearch.DefaultConfig(),
		Memory:       memory.DefaultConfig(),
		NewsAPI:      newsapi.DefaultConfig(),
		RAG:          rag.DefaultConfig(),
		SysInfo:      sysinfo.DefaultConfig(),
		WebFetch:     web.DefaultConfig(),
		Wikidata:     wikidata.DefaultConfig(),
		Wolfram:      wolfram.DefaultConfig(),
	}

	var data, err = os.ReadFile("locolm.json")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Println("no config file found, using defaults")
			return cfg, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	log.Println("config loaded.")

	return cfg, nil
}
