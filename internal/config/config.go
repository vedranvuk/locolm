package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds all locolm configuration. Loaded from locolm.json with
// GOOGLE_* / EXA_* env overrides for third-party credentials.
type Config struct {
	MCPPort            string `json:"mcp_port"`
	LlamaServerCommand string `json:"llama_server_command"`
	BrowserCommand     string `json:"browser_command"`
	GoogleAPIKey       string `json:"google_api_key"`
	GoogleCSEID        string `json:"google_cse_id"`

	// Raw tool configs — each tool package unmarshals its own.
	// Keys match the JSON object keys in locolm.json.
	WebFetch json.RawMessage `json:"web_fetch,omitempty"`
	FS       json.RawMessage `json:"fs,omitempty"`
	Exec     json.RawMessage `json:"exec,omitempty"`
	Wikidata json.RawMessage `json:"wikidata,omitempty"`
}

// LoadConfig reads locolm.json, applies GOOGLE_* env overrides, dispatches
// tool-specific configs to registered loaders, and returns the resolved config.
// locolm.json is read from the working directory (go run .) or the exe directory.
func LoadConfig() Config {
	cfg := Config{}

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

	// Third-party env vars override JSON
	if v := os.Getenv("GOOGLE_API_KEY"); v != "" {
		cfg.GoogleAPIKey = v
	}
	if v := os.Getenv("GOOGLE_CSE_ID"); v != "" {
		cfg.GoogleCSEID = v
	}

	return cfg
}
