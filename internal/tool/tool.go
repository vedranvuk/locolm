// Package tool defines the tool contract and registration mechanism.
// Tool packages self-register via init(), analogous to database/sql drivers.
package tool

import "encoding/json"

// ToolFunc is the signature for all tool implementations.
type ToolFunc func(args map[string]string) (string, error)

// Tool describes a single MCP tool: metadata + implementation.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	Func        ToolFunc        `json:"-"`
}
