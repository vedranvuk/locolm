package rag

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/vedranvuk/locolm/internal/client"
	"github.com/vedranvuk/locolm/internal/mcp"
	_ "modernc.org/sqlite"
	_ "modernc.org/sqlite/vec"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

type Config struct {
	// No specific configuration needed for rag tool
	// Embedding model dimensions can be configured here if needed
	EmbeddingDimensions int `json:"embedding_dimensions"`
	// EmbeddingEndpoint is the openai compatible endpoint running an embeddings model.
	EmbeddingEndpoint string `json:"embedding_endpoint"`
}

func DefaultConfig() *Config {
	return &Config{
		EmbeddingDimensions: 768, // Default for nomic-embed-text-v1.5
		EmbeddingEndpoint:   "http://192.168.1.100:11502",
	}
}

// ---------------------------------------------------------------------------
// Tool
// ---------------------------------------------------------------------------

type RAG struct {
	config *Config
	db     *sql.DB
	client *client.Client
}

func New(config *Config, db *sql.DB) (*RAG, error) {
	if db == nil {
		return nil, errors.New("rag: db is nil")
	}
	if config == nil {
		config = DefaultConfig()
	}

	var err error
	// Initialize tables for payload and vectors
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS memory_payload (id INTEGER PRIMARY KEY, content TEXT NOT NULL)`); err != nil {
		return nil, fmt.Errorf("rag: create memory table: %v", err)
	}
	if _, err = db.Exec(fmt.Sprintf(`CREATE VIRTUAL TABLE IF NOT EXISTS vec_items USING vec0(id INTEGER PRIMARY KEY, embedding float[%d])`, config.EmbeddingDimensions)); err != nil {
		return nil, fmt.Errorf("rag: create vector table: %v", err)
	}

	return &RAG{
		config: config,
		db:     db,
		client: client.New(config.EmbeddingEndpoint),
	}, nil
}

func (self *RAG) Register(r mcp.Registry) {

	r.RegisterTool(
		"remember_semantic",
		"Store text in the local vector database for later semantic (meaning-based) retrieval.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"text": { "type": "string" }
			},
			"required": ["text"]
		}`),
		self.rememberSemantic,
	)

	r.RegisterTool(
		"recall_semantic",
		"Semantic search over stored memories. Returns texts closest in meaning to `query`.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": { "type": "string" }
			},
			"required": ["query"]
		}`),
		self.recallSemantic,
	)

	r.RegisterTool(
		"forget_semantic",
		"Delete an exact stored text from the semantic database to remove outdated context.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"text": { "type": "string" }
			},
			"required": ["text"]
		}`),
		self.forgetSemantic,
	)
}

func (self *RAG) fetchEmbedding(text string) ([]float32, error) {
	// Context can be passed down from the MCP handler if mcp-go supports it,
	// otherwise Background is sufficient for local synchronous tool execution.
	// Note: Requires a model with embedding support (e.g., nomic-embed-text, all-minilm)
	// The model must be started with --pooling mean (or cls, last, rank)
	resp, err := self.client.Embedding(context.Background(), text, client.WithEmbeddingModel("nomic-embed-text-v1.5"), client.WithEmbeddingPooling("mean"))
	if err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("embedding returned no data")
	}
	return resp.Data[0].Embedding, nil
}

func (self *RAG) rememberSemantic(args map[string]string) (string, error) {
	text := args["text"]
	emb, err := self.fetchEmbedding(text)
	if err != nil {
		return "", fmt.Errorf("embedding failed: %v", err)
	}

	tx, err := self.db.Begin()
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %v", err)
	}

	res, err := tx.Exec("INSERT INTO memory_payload (content) VALUES (?)", text)
	if err != nil {
		tx.Rollback()
		return "", fmt.Errorf("failed to insert payload: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		tx.Rollback()
		return "", fmt.Errorf("failed to get last insert id: %v", err)
	}

	embJSON, err := json.Marshal(emb)
	if err != nil {
		tx.Rollback()
		return "", fmt.Errorf("failed to marshal embedding: %v", err)
	}
	_, err = tx.Exec(fmt.Sprintf("INSERT INTO vec_items (id, embedding) VALUES (?, vec_f32('%s'))", string(embJSON)), id)
	if err != nil {
		tx.Rollback()
		return "", fmt.Errorf("failed to insert vector: %v", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %v", err)
	}

	return "Stored in sqlite-vec.", nil
}

func (self *RAG) recallSemantic(args map[string]string) (string, error) {
	query := args["query"]
	emb, err := self.fetchEmbedding(query)
	if err != nil {
		return "", fmt.Errorf("embedding failed: %v", err)
	}

	embJSON, err := json.Marshal(emb)
	if err != nil {
		return "", fmt.Errorf("failed to marshal embedding: %v", err)
	}

	queryStr := fmt.Sprintf(`
		SELECT p.content 
		FROM vec_items v
		JOIN memory_payload p ON v.id = p.id
		ORDER BY vec_distance_L2(v.embedding, vec_f32('%s')) 
		LIMIT 3;
	`, string(embJSON))

	rows, err := self.db.Query(queryStr)
	if err != nil {
		return "", fmt.Errorf("failed to query vectors: %v", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return "", fmt.Errorf("failed to scan memory: %v", err)
		}
		out = append(out, "- "+c)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate memories: %v", err)
	}

	if len(out) == 0 {
		return "No relevant memories found.", nil
	}
	return "Semantic Matches:\n" + strings.Join(out, "\n"), nil
}

func (self *RAG) forgetSemantic(args map[string]string) (string, error) {
	text := args["text"]

	tx, err := self.db.Begin()
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	var id int64
	err = tx.QueryRow("SELECT id FROM memory_payload WHERE content = ?", text).Scan(&id)
	if err == sql.ErrNoRows {
		return "Memory not found. Nothing to delete.", nil
	}
	if err != nil {
		return "", fmt.Errorf("database error: %v", err)
	}

	// Delete from both the standard table and the sqlite-vec virtual table
	res, err := tx.Exec("DELETE FROM memory_payload WHERE id = ?", id)
	if err != nil {
		return "", fmt.Errorf("failed to delete payload: %v", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected > 0 {
		if _, err := tx.Exec("DELETE FROM vec_items WHERE id = ?", id); err != nil {
			return "", fmt.Errorf("failed to delete vector: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %v", err)
	}
	return "Memory successfully forgotten.", nil
}
