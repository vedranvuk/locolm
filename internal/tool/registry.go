package tool

import (
	"encoding/json"
	"fmt"
	"log"
)

var (
	registry    = map[string]Tool{}
	definitions []Tool
)

// Register adds a tool to the global registry. Called from init() in tool packages.
// Panics if a tool with the same name is registered twice — this catches
// programming errors at startup rather than silently dropping a tool.
func Register(name string, tool Tool) {
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("tool %q already registered", name))
	}
	registry[name] = tool
	definitions = append(definitions, tool)
	log.Printf("[TOOL] Registered: %s", name)
}

// Get returns a tool by name.
func Get(name string) (Tool, bool) {
	t, ok := registry[name]
	return t, ok
}

// Definitions returns all registered tool definitions (for MCP tools/list).
func Definitions() []Tool {
	return definitions
}

// ---------------------------------------------------------------------------
// Config registry — tools self-register config loaders via init()
// ---------------------------------------------------------------------------

// ConfigLoader unmarshals a json.RawMessage into the tool's config struct.
// Each tool package registers one via RegisterConfig in its init().
type ConfigLoader func(raw json.RawMessage) error

var configLoaders = map[string]ConfigLoader{}

// RegisterConfig registers a config loader for a tool package.
// Key is the JSON key in locolm.json (e.g. "fs", "exec", "web_fetch").
func RegisterConfig(key string, loader ConfigLoader) {
	if _, exists := configLoaders[key]; exists {
		panic(fmt.Sprintf("config loader %q already registered", key))
	}
	configLoaders[key] = loader
	log.Printf("[TOOL] Config registered: %s", key)
}

// LoadConfigs dispatches each raw config field to its registered loader.
// Called by config.LoadConfig() after the top-level Config is unmarshaled.
// Missing keys are silently skipped (tool keeps its defaults).
func LoadConfigs(m map[string]json.RawMessage) {
	for key, loader := range configLoaders {
		raw, ok := m[key]
		if !ok || len(raw) == 0 {
			continue
		}
		if err := loader(raw); err != nil {
			log.Printf("[TOOL] Config error for %q: %v (using defaults)", key, err)
		}
	}
}
