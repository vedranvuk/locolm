package memory

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/vedranvuk/locolm/internal/mcp"
	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

type Config struct {
	// No specific configuration needed for memory tool
}

func DefaultConfig() *Config {
	return &Config{}
}

// ---------------------------------------------------------------------------
// Tool
// ---------------------------------------------------------------------------

type Memory struct {
	config *Config
	db     *sql.DB
}

func New(config *Config, db *sql.DB) (*Memory, error) {
	if db == nil {
		return nil, errors.New("memory: db is nil")
	}
	if config == nil {
		config = DefaultConfig()
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS entities (id TEXT PRIMARY KEY, name TEXT UNIQUE NOT NULL, category TEXT NOT NULL)`); err != nil {
		return nil, fmt.Errorf("memory: create entities table: %w", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS observations (id TEXT PRIMARY KEY, entity_id TEXT REFERENCES entities(id) ON DELETE CASCADE, content TEXT NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return nil, fmt.Errorf("memory: create observations table: %w", err)
	}
	if _, err := db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts USING fts5(id UNINDEXED, content)`); err != nil {
		return nil, fmt.Errorf("memory: create observations_fts table: %w", err)
	}

	return &Memory{
		config: config,
		db:     db,
	}, nil
}

func (self *Memory) Register(r mcp.Registry) {

	r.RegisterTool(
		"add_observations",
		"Store factual knowledge about an entity (preferences, project details, user traits). `facts` is a JSON array of strings, e.g. '[\"Fact 1\", \"Fact 2\"]'.",
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
		self.addObservations,
	)

	r.RegisterTool(
		"remove_observations",
		"Delete specific facts for an entity. `facts` is a JSON array of exact observation strings to remove.",
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
		self.removeObservations,
	)

	r.RegisterTool(
		"search_memory",
		"Full-text search across all stored memories. Use when the entity name is unknown or you need historical context.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": { "type": "string" }
			},
			"required": [
				"query"
			]
		}`),
		self.searchMemory,
	)

	r.RegisterTool(
		"get_entity_context",
		"Load the full memory profile for an entity. Call before acting on a known entity (user, project) to get up-to-date context.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"entity_name": { "type": "string" }
			},
			"required": [
				"entity_name"
			]
		}`),
		self.getEntityContext,
	)
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (self *Memory) addObservations(args map[string]string) (string, error) {
	var facts []string
	if err := json.Unmarshal([]byte(args["facts"]), &facts); err != nil {
		return "", fmt.Errorf("invalid JSON for facts")
	}

	tx, err := self.db.Begin()
	if err != nil {
		return "", fmt.Errorf("memory: begin tx: %w", err)
	}
	defer tx.Rollback()

	var entityID string
	err = tx.QueryRow("SELECT id FROM entities WHERE name = ?", args["entity_name"]).Scan(&entityID)
	if err == sql.ErrNoRows {
		entityID = "ent_" + generateID()
		if _, err := tx.Exec("INSERT INTO entities (id, name, category) VALUES (?, ?, ?)", entityID, args["entity_name"], args["category"]); err != nil {
			return "", fmt.Errorf("memory: insert entity: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("memory: query entity: %w", err)
	}

	for _, c := range facts {
		id := "obs_" + generateID()
		if _, err := tx.Exec("INSERT INTO observations (id, entity_id, content) VALUES (?, ?, ?)", id, entityID, c); err != nil {
			return "", fmt.Errorf("memory: insert observation: %w", err)
		}
		if _, err := tx.Exec("INSERT INTO observations_fts (id, content) VALUES (?, ?)", id, c); err != nil {
			return "", fmt.Errorf("memory: insert observation fts: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("memory: commit tx: %w", err)
	}
	return "Added facts successfully.", nil
}

func (self *Memory) removeObservations(args map[string]string) (string, error) {
	var facts []string
	if err := json.Unmarshal([]byte(args["facts"]), &facts); err != nil {
		return "", fmt.Errorf("memory: invalid JSON for facts: %w", err)
	}

	tx, err := self.db.Begin()
	if err != nil {
		return "", fmt.Errorf("memory: begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, c := range facts {
		res, err := tx.Exec("DELETE FROM observations WHERE entity_id = (SELECT id FROM entities WHERE name = ?) AND content = ?", args["entity_name"], c)
		if err != nil {
			return "", fmt.Errorf("memory: delete observation: %w", err)
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return "", fmt.Errorf("memory: rows affected: %w", err)
		}
		if rows > 0 {
			if _, err := tx.Exec("DELETE FROM observations_fts WHERE id IN (SELECT id FROM observations WHERE content = ?)", c); err != nil {
				return "", fmt.Errorf("memory: delete observation fts: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("memory: commit tx: %w", err)
	}
	return "Pruning complete.", nil
}

func (self *Memory) getEntityContext(args map[string]string) (string, error) {
	var category string
	var entityID string
	err := self.db.QueryRow("SELECT id, category FROM entities WHERE name = ?", args["entity_name"]).Scan(&entityID, &category)
	if err != nil {
		return "Entity not found.", nil
	}

	rows, err := self.db.Query("SELECT content FROM observations WHERE entity_id = ? ORDER BY created_at DESC", entityID)
	if err != nil {
		return "", fmt.Errorf("memory: query observations: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return "", fmt.Errorf("memory: scan observation: %w", err)
		}
		out = append(out, "- "+c)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("memory: iterate observations: %w", err)
	}
	return fmt.Sprintf("Entity: %s (%s)\n%s", args["entity_name"], category, strings.Join(out, "\n")), nil
}

func (self *Memory) searchMemory(args map[string]string) (string, error) {
	rows, err := self.db.Query(`SELECT e.name, o.content FROM observations_fts f JOIN observations o ON f.id = o.id JOIN entities e ON o.entity_id = e.id WHERE observations_fts MATCH ? ORDER BY rank`, args["query"])
	if err != nil {
		return "", fmt.Errorf("memory: search memory: %w", err)
	}
	defer rows.Close()
	var res []string
	for rows.Next() {
		var n, c string
		if err := rows.Scan(&n, &c); err != nil {
			return "", fmt.Errorf("memory: scan match: %w", err)
		}
		res = append(res, "["+n+"]: "+c)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("memory: iterate matches: %w", err)
	}
	return "Matches:\n" + strings.Join(res, "\n"), nil
}
