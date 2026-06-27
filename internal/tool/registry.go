package tool

import (
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
