package rag

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vedranvuk/locolm/internal/client"
	"github.com/vedranvuk/locolm/internal/mcp"
	_ "modernc.org/sqlite"
	_ "modernc.org/sqlite/vec"
)

var db *sql.DB

// ... existing vars
var llamaClient *client.Client

func init() {
		// Initialize client targeting the secondary llama-server running the embedding model
	llamaClient = client.New("http://127.0.0.1:11500")

	// Init database
	exePath, err := os.Executable()
	if err != nil {                
		exePath = "."
	}
	dbPath := filepath.Join(filepath.Dir(exePath), "locolm_vec.db")

	db, err = sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {                                                                              
		panic(fmt.Sprintf("failed to open database: %v", err))
	}

	// Initialize tables for payload and vectors
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS memory_payload (id INTEGER PRIMARY KEY, content TEXT NOT NULL)`)
	// Note: Change float[768] to match your specific GGUF embedding model dimensions (e.g., 384 for MiniLM, 768 for nomic)
	_, _ = db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS vec_items USING vec0(id INTEGER PRIMARY KEY, embedding float[768])`)

	mcp.RegisterTool(
		"remember_semantic",
		"Write context to local embedded vector database for semantic search.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"text": { "type": "string" }
			},
			"required": ["text"]
		}`),
		rememberSemantic,
	)

	mcp.RegisterTool(
		"recall_semantic",
		"Semantic search memory via L2 distance.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": { "type": "string" }
			},
			"required": ["query"]
		}`),
		recallSemantic,
	)
}

func fetchEmbedding(text string) ([]float32, error) {
	// Context can be passed down from the MCP handler if mcp-go supports it, 
	// otherwise Background is sufficient for local synchronous tool execution.
	// Note: Requires a model with embedding support (e.g., nomic-embed-text, all-minilm)
	// The model must be started with --pooling mean (or cls, last, rank)
	return llamaClient.CreateEmbedding(context.Background(), text, "nomic-embed-text-v1.5")
}

func rememberSemantic(args map[string]string) (string, error) {
	text := args["text"]
	emb, err := fetchEmbedding(text)
	if err != nil {
		// Check if the error is about pooling type (model doesn't support embeddings)
		errMsg := err.Error()
		if strings.Contains(errMsg, "Pooling type") || strings.Contains(errMsg, "embedding") {
			return fmt.Sprintf("RAG not available: The current llama-server model does not support embeddings.\n\nTo enable semantic memory:\n1. Download an embedding model (e.g., nomic-embed-text, all-minilm-384-v2)\n2. Start a second llama-server instance with the embedding model:\n   llama-server -m <path-to-embedding-model> --pooling mean --port 11501\n3. Update the llamaClient URL in this file to point to the embedding server (http://127.0.0.1:11501)\n\nExample command:\n   D:\\Develop\\llama-cpp\\llama-server.exe -m e:\\Models\\GGUF\\nomic-embed-text-gguf.gguf --pooling mean --host 0.0.0.0 --port 11501", errMsg), nil
		}
		return "", fmt.Errorf("embedding failed: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %v", err)
	}

	res, err := tx.Exec("INSERT INTO memory_payload (content) VALUES (?)", text)
	if err != nil {
		tx.Rollback()
		return "", fmt.Errorf("failed to insert payload: %v", err)
	}
	id, _ := res.LastInsertId()

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

func recallSemantic(args map[string]string) (string, error) {
	query := args["query"]
	emb, err := fetchEmbedding(query)
	if err != nil {
		// Check if the error is about pooling type (model doesn't support embeddings)
		errMsg := err.Error()
		if strings.Contains(errMsg, "Pooling type") || strings.Contains(errMsg, "embedding") {
			return fmt.Sprintf("RAG not available: The current llama-server model does not support embeddings.\n\nTo enable semantic memory:\n1. Download an embedding model (e.g., nomic-embed-text, all-minilm-384-v2)\n2. Start a second llama-server instance with the embedding model:\n   llama-server -m <path-to-embedding-model> --pooling mean --port 11501\n3. Update the llamaClient URL in this file to point to the embedding server (http://127.0.0.1:11501)\n\nExample command:\n   D:\\Develop\\llama-cpp\\llama-server.exe -m e:\\Models\\GGUF\\nomic-embed-text-gguf.gguf --pooling mean --host 0.0.0.0 --port 11501", errMsg), nil
		}
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

	rows, err := db.Query(queryStr)
	if err != nil {
		return "", fmt.Errorf("failed to query vectors: %v", err)
	}
	defer rows.Close()

	var out []string 
	for rows.Next() {
		var c string     
		_ = rows.Scan(&c)
		out = append(out, "- "+c)
	}
	
	if len(out) == 0 {
		return "No relevant memories found.", nil
	}
	return "Semantic Matches:\n" + strings.Join(out, "\n"), nil
}

