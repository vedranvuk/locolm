package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	MCPPort              string `json:"mcp_port"`
	LlamaServerCommand   string `json:"llama_server_command"`
	BrowserCommand       string `json:"browser_command"`
	GoogleAPIKey         string `json:"google_api_key"`
	GoogleCSEID          string `json:"google_cse_id"`
	WebFetchMaxBytes     int64  `json:"web_fetch_max_bytes"`
	WebFetchMaxTextBytes int64  `json:"web_fetch_max_text_bytes"`
	WebFetchTimeoutSec   int    `json:"web_fetch_timeout_seconds"`
}

// LoadConfig reads locolm.json, applies GOOGLE_* env overrides, and returns the resolved config.
func LoadConfig() Config {
	cfg := Config{
		WebFetchMaxBytes:     5 * 1024 * 1024,
		WebFetchMaxTextBytes: 200 * 1024,
		WebFetchTimeoutSec:   30,
	}

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

	SetWebFetchConfig(cfg.WebFetchMaxBytes, cfg.WebFetchMaxTextBytes, cfg.WebFetchTimeoutSec)

	return cfg
}


