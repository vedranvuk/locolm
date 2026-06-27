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
		key         TEXT NOT NULL,
		value       TEXT NOT NULL,
		bucket      TEXT NOT NULL,
		keywords    TEXT DEFAULT '',
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (key, bucket)
	)`)
	if err != nil {
		panic(fmt.Sprintf("failed to create table: %v", err))
	}

	// Register all memory tools.
	mcp.RegisterTool(
		"memory_save",
		"Create or update a memory in a bucket. Use this to remember something for future conversations.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"bucket":  {"type": "string", "description": "The bucket (category) to store the memory in (e.g. 'user', 'work', 'general')"},
				"key":     {"type": "string", "description": "Unique key for the memory within the bucket (e.g. 'theme_preference')"},
				"value":   {"type": "string", "description": "The memory content to store"},
				"keywords":{"type": "string", "description": "Optional comma-separated keywords for better search recall (e.g. 'user, theme, dark')"}
			},
			"required": ["bucket", "key", "value"]
		}`),
		memorySave,
	)

	mcp.RegisterTool(
		"memory_edit",
		"Update an existing memory's value in a bucket. Fails if the memory doesn't exist.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"bucket": {"type": "string", "description": "The bucket containing the memory"},
				"key":    {"type": "string", "description": "The key of the memory to update"},
				"value":  {"type": "string", "description": "The new value for the memory"}
			},
			"required": ["bucket", "key", "value"]
		}`),
		memoryEdit,
	)

	mcp.RegisterTool(
		"memory_delete",
		"Delete a specific memory from a bucket.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"bucket": {"type": "string", "description": "The bucket containing the memory"},
				"key":    {"type": "string", "description": "The key of the memory to delete"}
			},
			"required": ["bucket", "key"]
		}`),
		memoryDelete,
	)

	mcp.RegisterTool(
		"memory_load",
		"Load a single memory's value from a bucket.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"bucket": {"type": "string", "description": "The bucket containing the memory"},
				"key":    {"type": "string", "description": "The key of the memory to load"}
			},
			"required": ["bucket", "key"]
		}`),
		memoryLoad,
	)

	mcp.RegisterTool(
		"memory_list",
		"List memories. Provide a bucket to list only that bucket; omit to list all memories across all buckets.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"bucket": {"type": "string", "description": "Optional bucket name. If omitted, lists all memories across all buckets."}
			},
			"required": []
		}`),
		memoryList,
	)

	mcp.RegisterTool(
		"memory_delete_bucket",
		"Delete a bucket and all memories in it.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"bucket": {"type": "string", "description": "The name of the bucket to delete"}
			},
			"required": ["bucket"]
		}`),
		memoryDeleteBucket,
	)

	mcp.RegisterTool(
		"memory_list_buckets",
		"List all memory buckets with their memory counts.",
		json.RawMessage(`{
			"type": "object",
			"properties": {},
			"required": []
		}`),
		memoryListBuckets,
	)
}

// --- Tool implementations ---

func memorySave(args map[string]string) (string, error) {
	bucket, ok := args["bucket"]
	if !ok || bucket == "" {
		return "", fmt.Errorf("missing required argument: bucket")
	}
	key, ok := args["key"]
	if !ok || key == "" {
		return "", fmt.Errorf("missing required argument: key")
	}
	value, ok := args["value"]
	if !ok {
		return "", fmt.Errorf("missing required argument: value")
	}
	keywords := args["keywords"]

	_, err := db.Exec(
		`INSERT INTO memories (key, value, bucket, keywords, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key, bucket) DO UPDATE SET value = excluded.value, keywords = excluded.keywords, updated_at = CURRENT_TIMESTAMP`,
		key, value, bucket, keywords,
	)
	if err != nil {
		return "", fmt.Errorf("failed to save memory: %w", err)
	}

	return fmt.Sprintf("Memory '%s' saved to bucket '%s'.", key, bucket), nil
}

func memoryEdit(args map[string]string) (string, error) {
	bucket, ok := args["bucket"]
	if !ok || bucket == "" {
		return "", fmt.Errorf("missing required argument: bucket")
	}
	key, ok := args["key"]
	if !ok || key == "" {
		return "", fmt.Errorf("missing required argument: key")
	}
	value, ok := args["value"]
	if !ok {
		return "", fmt.Errorf("missing required argument: value")
	}

	result, err := db.Exec(
		`UPDATE memories SET value = ?, updated_at = CURRENT_TIMESTAMP WHERE key = ? AND bucket = ?`,
		value, key, bucket,
	)
	if err != nil {
		return "", fmt.Errorf("failed to edit memory: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return "", fmt.Errorf("memory '%s' not found in bucket '%s'", key, bucket)
	}

	return fmt.Sprintf("Memory '%s' updated in bucket '%s'.", key, bucket), nil
}

func memoryDelete(args map[string]string) (string, error) {
	bucket, ok := args["bucket"]
	if !ok || bucket == "" {
		return "", fmt.Errorf("missing required argument: bucket")
	}
	key, ok := args["key"]
	if !ok || key == "" {
		return "", fmt.Errorf("missing required argument: key")
	}

	result, err := db.Exec("DELETE FROM memories WHERE key = ? AND bucket = ?", key, bucket)
	if err != nil {
		return "", fmt.Errorf("failed to delete memory: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return "", fmt.Errorf("memory '%s' not found in bucket '%s'", key, bucket)
	}

	return fmt.Sprintf("Memory '%s' deleted from bucket '%s'.", key, bucket), nil
}

func memoryLoad(args map[string]string) (string, error) {
	bucket, ok := args["bucket"]
	if !ok || bucket == "" {
		return "", fmt.Errorf("missing required argument: bucket")
	}
	key, ok := args["key"]
	if !ok || key == "" {
		return "", fmt.Errorf("missing required argument: key")
	}

	var value string
	err := db.QueryRow(
		"SELECT value FROM memories WHERE key = ? AND bucket = ?", key, bucket,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("memory '%s' not found in bucket '%s'", key, bucket)
	}
	if err != nil {
		return "", fmt.Errorf("failed to load memory: %w", err)
	}

	return value, nil
}

func memoryList(args map[string]string) (string, error) {
	bucket := args["bucket"]

	var rows *sql.Rows
	var err error
	if bucket != "" {
		rows, err = db.Query("SELECT key, value FROM memories WHERE bucket = ? ORDER BY updated_at DESC", bucket)
	} else {
		rows, err = db.Query("SELECT bucket, key, value FROM memories ORDER BY bucket, updated_at DESC")
	}
	if err != nil {
		return "", fmt.Errorf("failed to list memories: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var key, value string
		if bucket != "" {
			if err := rows.Scan(&key, &value); err != nil {
				return "", fmt.Errorf("failed to scan row: %w", err)
			}
			results = append(results, fmt.Sprintf("  %s: %s", key, value))
		} else {
			var b string
			if err := rows.Scan(&b, &key, &value); err != nil {
				return "", fmt.Errorf("failed to scan row: %w", err)
			}
			results = append(results, fmt.Sprintf("[%s] %s: %s", b, key, value))
		}
	}

	if len(results) == 0 {
		if bucket != "" {
			return fmt.Sprintf("No memories in bucket '%s'.", bucket), nil
		}
		return "No memories stored.", nil
	}

	if bucket != "" {
		return fmt.Sprintf("Memories in bucket '%s':\n%s", bucket, strings.Join(results, "\n")), nil
	}
	return fmt.Sprintf("All memories:\n%s", strings.Join(results, "\n")), nil
}

func memoryDeleteBucket(args map[string]string) (string, error) {
	bucket, ok := args["bucket"]
	if !ok || bucket == "" {
		return "", fmt.Errorf("missing required argument: bucket")
	}

	result, err := db.Exec("DELETE FROM memories WHERE bucket = ?", bucket)
	if err != nil {
		return "", fmt.Errorf("failed to delete bucket: %w", err)
	}

	n, _ := result.RowsAffected()
	return fmt.Sprintf("Bucket '%s' deleted (%d memories removed).", bucket, n), nil
}

func memoryListBuckets(args map[string]string) (string, error) {
	rows, err := db.Query("SELECT bucket, COUNT(*) FROM memories GROUP BY bucket ORDER BY bucket")
	if err != nil {
		return "", fmt.Errorf("failed to list buckets: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var b string
		var count int
		if err := rows.Scan(&b, &count); err != nil {
			return "", fmt.Errorf("failed to scan row: %w", err)
		}
		results = append(results, fmt.Sprintf("%s (%d)", b, count))
	}

	if len(results) == 0 {
		return "No buckets (no memories stored).", nil
	}
	return strings.Join(results, "\n"), nil
}
