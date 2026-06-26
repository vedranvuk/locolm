package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	MCPPort            string `json:"mcp_port"`
	LlamaServerCommand string `json:"llama_server_command"`
	BrowserCommand     string `json:"browser_command"`
	GoogleAPIKey       string `json:"google_api_key"`
	GoogleCSEID        string `json:"google_cse_id"`
}

// LoadConfig reads locolm.json, applies GOOGLE_* env overrides, and returns the resolved config.
// It also sets LOCOLM_* env vars so the rest of the codebase can read them via os.Getenv.
func LoadConfig() Config {
	cfg := Config{}

	// 1. Read locolm.json from working directory (go run .) or exe directory (binary)
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

	// 2. Third-party env vars override JSON
	if v := os.Getenv("GOOGLE_API_KEY"); v != "" {
		cfg.GoogleAPIKey = v
	}
	if v := os.Getenv("GOOGLE_CSE_ID"); v != "" {
		cfg.GoogleCSEID = v
	}

	// 3. Always set LOCOLM_* env vars (even empty) so downstream code can read them
	os.Setenv("LOCOLM_MCP_PORT", cfg.MCPPort)
	os.Setenv("LOCOLM_BOOTSTRAP_LLAMA_SERVER_COMMAND", cfg.LlamaServerCommand)
	os.Setenv("LOCOLM_BOOTSTRAP_BROWSER_COMMAND", cfg.BrowserCommand)
	if cfg.GoogleAPIKey != "" {
		os.Setenv("GOOGLE_API_KEY", cfg.GoogleAPIKey)
	}
	if cfg.GoogleCSEID != "" {
		os.Setenv("GOOGLE_CSE_ID", cfg.GoogleCSEID)
	}

	return cfg
}


