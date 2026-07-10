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

	"github.com/vedranvuk/locolm/internal/mcp"
	_ "modernc.org/sqlite"
)

var db *sql.DB

func init() {
	exePath, err := os.Executable()
	if err != nil {
		exePath = "."
	}
	dbPath := filepath.Join(filepath.Dir(exePath), "locolm_mem.db")

	db, err = sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		panic(fmt.Sprintf("failed to open database: %v", err))
	}

	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS entities (id TEXT PRIMARY KEY, name TEXT UNIQUE NOT NULL, category TEXT NOT NULL)`)
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS observations (id TEXT PRIMARY KEY, entity_id TEXT REFERENCES entities(id) ON DELETE CASCADE, content TEXT NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
	_, _ = db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts USING fts5(id UNINDEXED, content)`)

	mcp.RegisterTool(
		"add_observations",
		"Adds new factual knowledge about an entity. Use this to record preferences, project details, or persistent user traits. The 'facts' argument must be a JSON array of strings (e.g., '[\"Fact 1\", \"Fact 2\"]').",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"entity_name": { "type": "string" },
				"category": { "type": "string" },
				"facts": {
					"type": "string", 
					"description": "JSON array of strings to be saved as individual atomic facts."
				}
			},
			"required": [
				"entity_name",
				"category",
				"facts"
			]
		}`),
		addObservations,
	)

	mcp.RegisterTool(
		"remove_observations",
		"Deletes specific outdated or incorrect facts for an entity. Provide the exact string content of the observation to be pruned. The 'facts' argument must be a JSON array of strings.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"entity_name": { "type": "string" },
				"facts": {
					"type": "string",
					"description": "JSON array of exact observation strings to remove."
				}
			},
			"required": [
				"entity_name",
				"facts"
			]
		}`),
		removeObservations,
	)

	mcp.RegisterTool(
		"search_memory",
		"Performs a full-text search across all stored memories. Use this to retrieve historical context, previous project notes, or related user preferences when the entity name is unknown.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": { "type": "string" }
			},
			"required": [
				"query"
			]
		}`),
		searchMemory,
	)

	mcp.RegisterTool(
		"get_entity_context",
		"Loads the full memory profile for a specific entity. Use this before making decisions involving an entity (like a user or project) to ensure you have the most up-to-date context.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"entity_name": { "type": "string" }
			},
			"required": [
				"entity_name"
			]
		}`),
		getEntityContext,
	)
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func addObservations(args map[string]string) (string, error) {
	var facts []string
	if err := json.Unmarshal([]byte(args["facts"]), &facts); err != nil {
		return "", fmt.Errorf("invalid JSON for facts")
	}

	tx, _ := db.Begin()
	defer tx.Rollback()

	var entityID string
	err := tx.QueryRow("SELECT id FROM entities WHERE name = ?", args["entity_name"]).Scan(&entityID)
	if err == sql.ErrNoRows {
		entityID = "ent_" + generateID()
		_, _ = tx.Exec("INSERT INTO entities (id, name, category) VALUES (?, ?, ?)", entityID, args["entity_name"], args["category"])
	}

	for _, c := range facts {
		id := "obs_" + generateID()
		_, _ = tx.Exec("INSERT INTO observations (id, entity_id, content) VALUES (?, ?, ?)", id, entityID, c)
		_, _ = tx.Exec("INSERT INTO observations_fts (id, content) VALUES (?, ?)", id, c)
	}
	return "Added facts successfully.", tx.Commit()
}

func removeObservations(args map[string]string) (string, error) {
	var facts []string
	_ = json.Unmarshal([]byte(args["facts"]), &facts)

	tx, _ := db.Begin()
	defer tx.Rollback()

	for _, c := range facts {
		res, _ := tx.Exec("DELETE FROM observations WHERE entity_id = (SELECT id FROM entities WHERE name = ?) AND content = ?", args["entity_name"], c)
		rows, _ := res.RowsAffected()
		if rows > 0 {
			_, _ = tx.Exec("DELETE FROM observations_fts WHERE id IN (SELECT id FROM observations WHERE content = ?)", c)
		}
	}
	return "Pruning complete.", tx.Commit()
}

func getEntityContext(args map[string]string) (string, error) {
	var category string
	var entityID string
	err := db.QueryRow("SELECT id, category FROM entities WHERE name = ?", args["entity_name"]).Scan(&entityID, &category)
	if err != nil {
		return "Entity not found.", nil
	}

	rows, _ := db.Query("SELECT content FROM observations WHERE entity_id = ? ORDER BY created_at DESC", entityID)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		_ = rows.Scan(&c)
		out = append(out, "- "+c)
	}
	return fmt.Sprintf("Entity: %s (%s)\n%s", args["entity_name"], category, strings.Join(out, "\n")), nil
}

func searchMemory(args map[string]string) (string, error) {
	rows, _ := db.Query(`SELECT e.name, o.content FROM observations_fts f JOIN observations o ON f.id = o.id JOIN entities e ON o.entity_id = e.id WHERE observations_fts MATCH ? ORDER BY rank`, args["query"])
	defer rows.Close()
	var res []string
	for rows.Next() {
		var n, c string
		_ = rows.Scan(&n, &c)
		res = append(res, "["+n+"]: "+c)
	}
	return "Matches:\n" + strings.Join(res, "\n"), nil
}
