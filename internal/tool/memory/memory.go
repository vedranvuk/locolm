package memory

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vedranvuk/locolm/internal/mcp"
	_ "modernc.org/sqlite"
)

var db *sql.DB

func init() {
	exePath, err := os.Executable()
	if err != nil {
		exePath = "."
	}
	dbPath := filepath.Join(filepath.Dir(exePath), "locolm.db")

	// Open connection with WAL mode enabled to support concurrent read/writes during LLM streams
	db, err = sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		panic(fmt.Sprintf("failed to open database: %v", err))
	}

	// 1. Entities table: High level buckets (e.g., "user_preferences", "car_specs")
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS entities (
		id TEXT PRIMARY KEY,
		name TEXT UNIQUE NOT NULL,
		category TEXT NOT NULL
	)`)
	if err != nil {
		panic(fmt.Sprintf("failed to create entities table: %v", err))
	}

	// 2. Observations table: Atomic, discrete granular facts pinned to an entity
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS observations (
		id TEXT PRIMARY KEY,
		entity_id TEXT REFERENCES entities(id) ON DELETE CASCADE,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		panic(fmt.Sprintf("failed to create observations table: %v", err))
	}

	// 3. FTS5 Virtual Table for performant keyword searches over atomic memories
	_, err = db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts USING fts5(
		id UNINDEXED,
		content
	)`)
	if err != nil {
		panic(fmt.Sprintf("failed to create FTS5 table: %v", err))
	}

	// Register tools tailored to LLM context workflows
	mcp.RegisterTool(
		"patch_memory",
		"Declaratively update memories for an entity. Upserts the entity and handles adding new atomic facts or removing outdated observations.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"entity_name": {"type": "string", "description": "The name of the entity (e.g., 'user_profile', 'current_project')"},
				"category": {"type": "string", "description": "Broad classification context (e.g., 'preferences', 'hardware', 'codebase')"},
				"add_observations": {
					"type": "array",
					"items": {"type": "string"},
					"description": "List of new standalone, atomic facts to append to this entity."
				},
				"remove_observations": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Exact text statements or observation IDs that are no longer true and should be pruned."
				}
			},
			"required": ["entity_name", "category"]
		}`),
		patchMemory,
	)

	mcp.RegisterTool(
		"search_memory",
		"Search all historical observations using keyword full-text matching.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {"type": "string", "description": "Search terms to query facts against"}
			},
			"required": ["query"]
		}`),
		searchMemory,
	)

	mcp.RegisterTool(
		"get_entity_context",
		"Retrieve all known atomic facts and historical context tied to a specific entity name.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"entity_name": {"type": "string", "description": "The entity profile to load fully"}
			},
			"required": ["entity_name"]
		}`),
		getEntityContext,
	)
}

// generateID generates a secure, random 16-character hex string (8 bytes of entropy).
// This is perfect for light relational keys without pulling in heavy UUID dependencies.
func generateID() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		// Fallback to high-resolution timestamp if crypto/rand fails (extremely rare)
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// --- Tool Implementations ---

func patchMemory(args map[string]string) (string, error) {
	entityName := args["entity_name"]
	category := args["category"]
	if entityName == "" || category == "" {
		return "", fmt.Errorf("entity_name and category are required")
	}

	// Parse out arrays manually if packed as stringified JSON by MCP or handle directly
	var addObs, remObs []string
	if val, ok := args["add_observations"]; ok {
		_ = json.Unmarshal([]byte(val), &addObs)
	}
	if val, ok := args["remove_observations"]; ok {
		_ = json.Unmarshal([]byte(val), &remObs)
	}

	tx, err := db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	// Ensure Entity exists
	var entityID string
	err = tx.QueryRow("SELECT id FROM entities WHERE name = ?", entityName).Scan(&entityID)
	if err == sql.ErrNoRows {
		entityID = "ent_" + generateID() // Clean, random, unique ID
		_, err = tx.Exec("INSERT INTO entities (id, name, category) VALUES (?, ?, ?)", entityID, entityName, category)
		if err != nil {
			return "", fmt.Errorf("failed to create entity: %w", err)
		}
	} else if err != nil {
		return "", err
	}

	// Process removals
	for _, obs := range remObs {
		// Try targeting content matches exactly
		var obsID string
		_ = tx.QueryRow("SELECT id FROM observations WHERE entity_id = ? AND content = ?", entityID, obs).Scan(&obsID)
		if obsID != "" {
			_, _ = tx.Exec("DELETE FROM observations WHERE id = ?", obsID)
			_, _ = tx.Exec("DELETE FROM observations_fts WHERE id = ?", obsID)
		}
	}

	// Process additions
	for _, content := range addObs {
		if strings.TrimSpace(content) == "" {
			continue
		}
		obsID := "obs_" + generateID() // Clean, random, unique ID
		_, err = tx.Exec("INSERT INTO observations (id, entity_id, content) VALUES (?, ?, ?)", obsID, entityID, content)
		if err != nil {
			return "", fmt.Errorf("failed to add observation: %w", err)
		}
		_, err = tx.Exec("INSERT INTO observations_fts (id, content) VALUES (?, ?)", obsID, content)
		if err != nil {
			return "", fmt.Errorf("failed to index observation: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return fmt.Sprintf("Successfully updated memory profile for entity '%s'. Added %d, removed %d observations.", entityName, len(addObs), len(remObs)), nil
}

func getEntityContext(args map[string]string) (string, error) {
	entityName := args["entity_name"]
	if entityName == "" {
		return "", fmt.Errorf("missing required argument: entity_name")
	}

	var entityID, category string
	err := db.QueryRow("SELECT id, category FROM entities WHERE name = ?", entityName).Scan(&entityID, &category)
	if err == sql.ErrNoRows {
		return fmt.Sprintf("No memory profile found for entity '%s'.", entityName), nil
	} else if err != nil {
		return "", err
	}

	rows, err := db.Query("SELECT content FROM observations WHERE entity_id = ? ORDER BY created_at DESC", entityID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var observations []string
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err == nil {
			observations = append(observations, "- "+content)
		}
	}

	output := fmt.Sprintf("Entity: %s [Category: %s]\nKnown Facts:\n", entityName, category)
	if len(observations) == 0 {
		output += "(No facts saved yet)"
	} else {
		output += strings.Join(observations, "\n")
	}

	return output, nil
}

func searchMemory(args map[string]string) (string, error) {
	query := args["query"]
	if query == "" {
		return "", fmt.Errorf("missing required argument: query")
	}

	rows, err := db.Query(`
		SELECT e.name, o.content 
		FROM observations_fts f
		JOIN observations o ON f.id = o.id
		JOIN entities e ON o.entity_id = e.id
		WHERE observations_fts MATCH ? 
		ORDER BY rank`, query)
	if err != nil {
		return "", fmt.Errorf("fts search failed: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var name, content string
		if err := rows.Scan(&name, &content); err == nil {
			results = append(results, fmt.Sprintf("[%s]: %s", name, content))
		}
	}

	if len(results) == 0 {
		return fmt.Sprintf("No historic context found matching query: '%s'", query), nil
	}

	return fmt.Sprintf("Found %d contextual matches:\n%s", len(results), strings.Join(results, "\n")), nil
}
