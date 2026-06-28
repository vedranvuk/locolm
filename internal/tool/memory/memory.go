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

	// FTS5 full-text search index over key, keywords, and bucket.
	// This is a standalone FTS5 table that we manually keep in sync
	// with the memories table on writes.
	_, err = db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
		key,
		keywords,
		bucket
	)`)
	if err != nil {
		panic(fmt.Sprintf("failed to create FTS5 table: %v", err))
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
		"Update an existing memory's value and/or keywords in a bucket. Fails if the memory doesn't exist.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"bucket":  {"type": "string", "description": "The bucket containing the memory"},
				"key":     {"type": "string", "description": "The key of the memory to update"},
				"value":   {"type": "string", "description": "The new value for the memory"},
				"keywords":{"type": "string", "description": "Optional new comma-separated keywords"}
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
		"List memory keys. Provide a bucket to list only that bucket's keys; omit to list all keys across all buckets. Provide a key to check a specific memory.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"bucket": {"type": "string", "description": "Optional bucket name. If omitted, lists keys from all buckets."},
				"key":    {"type": "string", "description": "Optional key name. If provided with bucket, checks that specific memory."}
			},
			"required": []
		}`),
		memoryList,
	)

	mcp.RegisterTool(
		"memory_find",
		"Search memories by keyword using full-text search. Returns matching bucket and key pairs.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"query":  {"type": "string", "description": "Search query — words or phrases to match against keywords, keys, and bucket names"},
				"bucket": {"type": "string", "description": "Optional bucket to restrict search to"}
			},
			"required": ["query"]
		}`),
		memoryFind,
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

// --- FTS5 sync helpers ---

func ftsInsert(key, keywords, bucket string) {
	db.Exec("INSERT INTO memories_fts(key, keywords, bucket) VALUES (?, ?, ?)", key, keywords, bucket)
}

func ftsUpdate(oldKey, oldBucket, newKey, newKeywords, newBucket string) {
	db.Exec("DELETE FROM memories_fts WHERE key = ? AND bucket = ?", oldKey, oldBucket)
	ftsInsert(newKey, newKeywords, newBucket)
}

func ftsDelete(key, bucket string) {
	db.Exec("DELETE FROM memories_fts WHERE key = ? AND bucket = ?", key, bucket)
}

func ftsDeleteBucket(bucket string) {
	db.Exec("DELETE FROM memories_fts WHERE bucket = ?", bucket)
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

	// Sync FTS5 index.
	ftsDelete(key, bucket) // remove old entry if existed
	ftsInsert(key, keywords, bucket)

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
	keywords := args["keywords"]

	result, err := db.Exec(
		`UPDATE memories SET value = ?, keywords = ?, updated_at = CURRENT_TIMESTAMP WHERE key = ? AND bucket = ?`,
		value, keywords, key, bucket,
	)
	if err != nil {
		return "", fmt.Errorf("failed to edit memory: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return "", fmt.Errorf("memory '%s' not found in bucket '%s'", key, bucket)
	}

	// Sync FTS5 index.
	ftsDelete(key, bucket)
	ftsInsert(key, keywords, bucket)

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

	// Sync FTS5 index.
	ftsDelete(key, bucket)

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
	key := args["key"]

	// Specific key lookup: return just the key name if it exists.
	if bucket != "" && key != "" {
		var exists string
		err := db.QueryRow(
			"SELECT key FROM memories WHERE key = ? AND bucket = ?", key, bucket,
		).Scan(&exists)
		if err == sql.ErrNoRows {
			return fmt.Sprintf("Memory '%s' not found in bucket '%s'.", key, bucket), nil
		}
		if err != nil {
			return "", fmt.Errorf("failed to check memory: %w", err)
		}
		return exists, nil
	}

	// List keys in a specific bucket.
	if bucket != "" {
		rows, err := db.Query("SELECT key FROM memories WHERE bucket = ? ORDER BY updated_at DESC", bucket)
		if err != nil {
			return "", fmt.Errorf("failed to list memories: %w", err)
		}
		defer rows.Close()

		var keys []string
		for rows.Next() {
			var k string
			if err := rows.Scan(&k); err != nil {
				return "", fmt.Errorf("failed to scan row: %w", err)
			}
			keys = append(keys, k)
		}

		if len(keys) == 0 {
			return fmt.Sprintf("No memories in bucket '%s'.", bucket), nil
		}
		return fmt.Sprintf("Keys in bucket '%s':\n%s", bucket, strings.Join(keys, "\n")), nil
	}

	// List all keys across all buckets.
	rows, err := db.Query("SELECT bucket, key FROM memories ORDER BY bucket, updated_at DESC")
	if err != nil {
		return "", fmt.Errorf("failed to list memories: %w", err)
	}
	defer rows.Close()

	var results []string
	var currentBucket string
	for rows.Next() {
		var b, k string
		if err := rows.Scan(&b, &k); err != nil {
			return "", fmt.Errorf("failed to scan row: %w", err)
		}
		if b != currentBucket {
			currentBucket = b
			results = append(results, fmt.Sprintf("[%s]", b))
		}
		results = append(results, fmt.Sprintf("  %s", k))
	}

	if len(results) == 0 {
		return "No memories stored.", nil
	}
	return strings.Join(results, "\n"), nil
}

func memoryFind(args map[string]string) (string, error) {
	query, ok := args["query"]
	if !ok || query == "" {
		return "", fmt.Errorf("missing required argument: query")
	}
	bucket := args["bucket"]

	var rows *sql.Rows
	var err error
	if bucket != "" {
		rows, err = db.Query(
			"SELECT bucket, key FROM memories_fts WHERE memories_fts MATCH ? AND bucket = ? ORDER BY rank",
			query, bucket,
		)
	} else {
		rows, err = db.Query(
			"SELECT bucket, key FROM memories_fts WHERE memories_fts MATCH ? ORDER BY rank",
			query,
		)
	}
	if err != nil {
		return "", fmt.Errorf("failed to search memories: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var b, k string
		if err := rows.Scan(&b, &k); err != nil {
			return "", fmt.Errorf("failed to scan row: %w", err)
		}
		results = append(results, fmt.Sprintf("[%s] %s", b, k))
	}

	if len(results) == 0 {
		return fmt.Sprintf("No memories matching '%s'.", query), nil
	}
	return fmt.Sprintf("Search results for '%s':\n%s", query, strings.Join(results, "\n")), nil
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

	// Sync FTS5 index.
	ftsDeleteBucket(bucket)

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
