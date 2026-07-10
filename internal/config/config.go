package config

import (
	"encoding/json"
	"errors"
	"log"
	"os"

	"github.com/vedranvuk/locolm/internal/database"
	"github.com/vedranvuk/locolm/internal/server"
	"github.com/vedranvuk/locolm/internal/tool/exa"
	"github.com/vedranvuk/locolm/internal/tool/exec"
	"github.com/vedranvuk/locolm/internal/tool/fetch"
	"github.com/vedranvuk/locolm/internal/tool/fs"
	"github.com/vedranvuk/locolm/internal/tool/google"
	"github.com/vedranvuk/locolm/internal/tool/gopls"
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
	MCP      *server.Config   `json:"mcp"`
	Database *database.Config `json:"database"`

	Exa      *exa.Config      `json:"exa,omitzero"`
	Exec     *exec.Config     `json:"exec,omitzero"`
	FS       *fs.Config       `json:"fs,omitzero"`
	Gopls    *gopls.Config    `json:"gopls,omitzero"`
	Google   *google.Config   `json:"google,omitzero"`
	Memory   *memory.Config   `json:"memory,omitzero"`
	NewsAPI  *newsapi.Config  `json:"newsapi,omitzero"`
	RAG      *rag.Config      `json:"rag,omitzero"`
	SysInfo  *sysinfo.Config  `json:"sysinfo,omitzero"`
	Fetch    *fetch.Config    `json:"fetch,omitzero"`
	Wikidata *wikidata.Config `json:"wikidata,omitzero"`
	Wolfram  *wolfram.Config  `json:"wolfram,omitzero"`
}

// Load reads locolm.json, applies GOOGLE_* env overrides, dispatches
// tool-specific configs to registered loaders, and returns the resolved config.
// locolm.json is read from the working directory (go run .) or the exe directory.
func Load() (*Config, error) {

	var cfg = &Config{
		MCP:      server.DefaultConfig(),
		Database: database.DefaultConfig(),

		Exa:      exa.DefaultConfig(),
		Exec:     exec.DefaultConfig(),
		Fetch:    fetch.DefaultConfig(),
		FS:       fs.DefaultConfig(),
		Gopls:    gopls.DefaultConfig(),
		Google:   google.DefaultConfig(),
		Memory:   memory.DefaultConfig(),
		NewsAPI:  newsapi.DefaultConfig(),
		RAG:      rag.DefaultConfig(),
		SysInfo:  sysinfo.DefaultConfig(),
		Wikidata: wikidata.DefaultConfig(),
		Wolfram:  wolfram.DefaultConfig(),
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
