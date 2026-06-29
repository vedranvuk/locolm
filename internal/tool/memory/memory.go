package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vedranvuk/locolm/internal/mcp"
	_ "modernc.org/sqlite"
)

var db *sql.DB

func init() {
	// Open database and create table.
	exePath, err := os.Executable()
	if err != nil {
		exePath = "."
	}
	dbPath := filepath.Join(filepath.Dir(exePath), "locolm.db")

	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		panic(fmt.Sprintf("failed to open database: %v", err))
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS memories (
		path        TEXT PRIMARY KEY,
		value       TEXT NOT NULL,
		keywords    TEXT DEFAULT '',
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		panic(fmt.Sprintf("failed to create table: %v", err))
	}

	// FTS5 full-text search index over path and keywords.
	// This is a standalone FTS5 table that we manually keep in sync
	// with the memories table on writes.
	_, err = db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
		path,
		keywords
	)`)
	if err != nil {
		panic(fmt.Sprintf("failed to create FTS5 table: %v", err))
	}

	// Register all memory tools.
	mcp.RegisterTool(
		"memory_save",
		"Create or update a memory. Use this to remember something for future conversations. Use path-style keys for hierarchy (e.g. 'project/theme_preference').",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path":    {"type": "string", "description": "Unique path for the memory, use '/' for hierarchy (e.g. 'user/theme', 'project/ideas/cli_tool')"},
				"value":   {"type": "string", "description": "The memory content to store"},
				"keywords":{"type": "string", "description": "Optional comma-separated keywords for better search recall (e.g. 'user,theme,dark')"}
			},
			"required": ["path", "value"]
		}`),
		memorySave,
	)

	mcp.RegisterTool(
		"memory_edit",
		"Update an existing memory's value and/or keywords. Fails if the memory doesn't exist.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path":    {"type": "string", "description": "The path of the memory to update"},
				"value":   {"type": "string", "description": "The new value for the memory"},
				"keywords":{"type": "string", "description": "Optional new comma-separated keywords"}
			},
			"required": ["path", "value"]
		}`),
		memoryEdit,
	)

	mcp.RegisterTool(
		"memory_delete",
		"Delete a specific memory. Use a trailing '/' to delete all memories under a path prefix (e.g. 'project/' deletes everything under project/).",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "The path of the memory to delete, or a prefix with trailing '/' to delete all under it"}
			},
			"required": ["path"]
		}`),
		memoryDelete,
	)

	mcp.RegisterTool(
		"memory_load",
		"Load a single memory's value and timestamps by path.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "The path of the memory to load"}
			},
			"required": ["path"]
		}`),
		memoryLoad,
	)

	mcp.RegisterTool(
		"memory_list",
		"List memory paths. Provide a path prefix to list only memories under that hierarchy (e.g. 'project/' lists all project memories). Omit to list all memories.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Optional path prefix. Lists all memories under this hierarchy, or all memories if omitted."}
			},
			"required": []
		}`),
		memoryList,
	)

	mcp.RegisterTool(
		"memory_find",
		"Search memories by keyword using full-text search. Returns matching paths.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path":   {"type": "string", "description": "Optional path prefix to restrict search to (e.g. 'project/')"},
				"query":  {"type": "string", "description": "Search query — words or phrases to match against keywords and paths"}
			},
			"required": ["query"]
		}`),
		memoryFind,
	)
}

// --- FTS5 sync helpers ---

func ftsInsert(path, keywords string) {
	db.Exec("INSERT INTO memories_fts(path, keywords) VALUES (?, ?)", path, keywords)
}

func ftsUpdate(oldPath, newPath, newKeywords string) {
	db.Exec("DELETE FROM memories_fts WHERE path = ?", oldPath)
	ftsInsert(newPath, newKeywords)
}

func ftsDelete(path string) {
	db.Exec("DELETE FROM memories_fts WHERE path = ?", path)
}

func ftsDeletePrefix(prefix string) {
	db.Exec("DELETE FROM memories_fts WHERE path LIKE ?", prefix+"%")
}

// --- Tool implementations ---

func memorySave(args map[string]string) (string, error) {
	path, ok := args["path"]
	if !ok || path == "" {
		return "", fmt.Errorf("missing required argument: path")
	}
	value, ok := args["value"]
	if !ok {
		return "", fmt.Errorf("missing required argument: value")
	}
	keywords := args["keywords"]

	_, err := db.Exec(
		`INSERT INTO memories (path, value, keywords, updated_at)
		 VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(path) DO UPDATE SET value = excluded.value, keywords = excluded.keywords, updated_at = CURRENT_TIMESTAMP`,
		path, value, keywords,
	)
	if err != nil {
		return "", fmt.Errorf("failed to save memory: %w", err)
	}

	// Sync FTS5 index.
	ftsDelete(path) // remove old entry if existed
	ftsInsert(path, keywords)

	return fmt.Sprintf("Memory '%s' saved.", path), nil
}

func memoryEdit(args map[string]string) (string, error) {
	path, ok := args["path"]
	if !ok || path == "" {
		return "", fmt.Errorf("missing required argument: path")
	}
	value, ok := args["value"]
	if !ok {
		return "", fmt.Errorf("missing required argument: value")
	}
	keywords := args["keywords"]

	result, err := db.Exec(
		`UPDATE memories SET value = ?, keywords = ?, updated_at = CURRENT_TIMESTAMP WHERE path = ?`,
		value, keywords, path,
	)
	if err != nil {
		return "", fmt.Errorf("failed to edit memory: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return "", fmt.Errorf("memory '%s' not found", path)
	}

	// Sync FTS5 index.
	ftsDelete(path)
	ftsInsert(path, keywords)

	return fmt.Sprintf("Memory '%s' updated.", path), nil
}

func memoryDelete(args map[string]string) (string, error) {
	path, ok := args["path"]
	if !ok || path == "" {
		return "", fmt.Errorf("missing required argument: path")
	}

	// If path ends with '/', treat as prefix delete.
	if strings.HasSuffix(path, "/") {
		likePrefix := path + "%"

		// Get count before delete for message.
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM memories WHERE path LIKE ?", likePrefix,
		).Scan(&count)
		if err != nil {
			return "", fmt.Errorf("failed to count memories: %w", err)
		}

		_, err = db.Exec("DELETE FROM memories WHERE path LIKE ?", likePrefix)
		if err != nil {
			return "", fmt.Errorf("failed to delete memories: %w", err)
		}

		ftsDeletePrefix(path)

		return fmt.Sprintf("Deleted %d memories under '%s'.", count, path), nil
	}

	result, err := db.Exec("DELETE FROM memories WHERE path = ?", path)
	if err != nil {
		return "", fmt.Errorf("failed to delete memory: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return "", fmt.Errorf("memory '%s' not found", path)
	}

	ftsDelete(path)

	return fmt.Sprintf("Memory '%s' deleted.", path), nil
}

func memoryLoad(args map[string]string) (string, error) {
	path, ok := args["path"]
	if !ok || path == "" {
		return "", fmt.Errorf("missing required argument: path")
	}

	var value, createdAt, updatedAt string
	err := db.QueryRow(
		"SELECT value, created_at, updated_at FROM memories WHERE path = ?", path,
	).Scan(&value, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("memory '%s' not found", path)
	}
	if err != nil {
		return "", fmt.Errorf("failed to load memory: %w", err)
	}

	return fmt.Sprintf("Path: %s\nCreated: %s\nModified: %s\nValue: %s", path, createdAt, updatedAt, value), nil
}

func memoryList(args map[string]string) (string, error) {
	prefix := args["path"]

	var rows *sql.Rows
	var err error

	if prefix != "" {
		// Ensure prefix ends with '/' for hierarchy matching, unless it's a full path.
		if !strings.HasSuffix(prefix, "/") {
			rows, err = db.Query(
				"SELECT path, updated_at FROM memories WHERE path = ? OR path LIKE ? ORDER BY path",
				prefix, prefix+"/%",
			)
		} else {
			rows, err = db.Query(
				"SELECT path, updated_at FROM memories WHERE path LIKE ? ORDER BY path",
				prefix+"%",
			)
		}
	} else {
		rows, err = db.Query("SELECT path, updated_at FROM memories ORDER BY path")
	}
	if err != nil {
		return "", fmt.Errorf("failed to list memories: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var p, updatedAt string
		if err := rows.Scan(&p, &updatedAt); err != nil {
			return "", fmt.Errorf("failed to scan row: %w", err)
		}
		results = append(results, fmt.Sprintf("%s  (modified: %s)", p, updatedAt))
	}

	if len(results) == 0 {
		if prefix != "" {
			return fmt.Sprintf("No memories found under '%s'.", prefix), nil
		}
		return "No memories stored.", nil
	}

	return strings.Join(results, "\n"), nil
}

func memoryFind(args map[string]string) (string, error) {
	query, ok := args["query"]
	if !ok || query == "" {
		return "", fmt.Errorf("missing required argument: query")
	}
	prefix := args["path"]

	var rows *sql.Rows
	var err error
	if prefix != "" {
		rows, err = db.Query(
			"SELECT path FROM memories_fts WHERE memories_fts MATCH ? AND path LIKE ? ORDER BY rank",
			query, prefix+"%",
		)
	} else {
		rows, err = db.Query(
			"SELECT path FROM memories_fts WHERE memories_fts MATCH ? ORDER BY rank",
			query,
		)
	}
	if err != nil {
		return "", fmt.Errorf("failed to search memories: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return "", fmt.Errorf("failed to scan row: %w", err)
		}
		results = append(results, p)
	}

	if len(results) == 0 {
		return fmt.Sprintf("No memories matching '%s'.", query), nil
	}
	return fmt.Sprintf("Search results for '%s':\n%s", query, strings.Join(results, "\n")), nil
}
