// Package client implements a llama-server client.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client handles communication with llama-server API.
type Client struct {
	baseURL string
	client  *http.Client
}

// New creates a new client for llama-server
func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Chat sends a chat request to the llama-server
func (c *Client) Chat(ctx context.Context, messages []Message, opts ...func(*ChatRequest)) (*ChatResponse, error) {
	req := &ChatRequest{
		Messages: messages,
		Stream:   false,
	}

	// Apply optional parameters
	for _, opt := range opts {
		opt(req)
	}

	// Set default values if not specified
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 256
	}
	if req.TopP == 0 {
		req.TopP = 1.0
	}

	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	url := c.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Send request
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("llama-server error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Decode response
	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chatResp, nil
}

// WithTemperature sets the temperature for chat requests
func WithTemperature(temp float64) func(*ChatRequest) {
	return func(r *ChatRequest) {
		r.Temperature = temp
	}
}

// WithMaxTokens sets the max tokens for chat requests
func WithMaxTokens(max int) func(*ChatRequest) {
	return func(r *ChatRequest) {
		r.MaxTokens = max
	}
}

// WithTopP sets the top_p for chat requests
func WithTopP(topP float64) func(*ChatRequest) {
	return func(r *ChatRequest) {
		r.TopP = topP
	}
}

// ChatStream sends a streaming chat request to the llama-server
func (c *Client) ChatStream(ctx context.Context, messages []Message, opts ...func(*ChatRequest)) (io.ReadCloser, error) {
	req := &ChatRequest{
		Messages: messages,
		Stream:   true,
	}

	// Apply optional parameters
	for _, opt := range opts {
		opt(req)
	}

	// Set default values if not specified
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 256
	}
	if req.TopP == 0 {
		req.TopP = 1.0
	}

	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	url := c.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")

	// Send request
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("llama-server error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return resp.Body, nil
}

// GetModelList retrieves the list of available models
func (c *Client) GetModelList(ctx context.Context) ([]string, error) {
	url := c.baseURL + "/v1/models"

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("llama-server error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var modelsResp struct {
		Object string  `json:"object"`
		Data   []Model `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var modelNames []string
	for _, model := range modelsResp.Data {
		modelNames = append(modelNames, model.ID)
	}

	return modelNames, nil
}

// Model represents a model in the model list
type Model struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Owned   string       `json:"owned_by"`
	Details ModelDetails `json:"details,omitempty"`
}

// ModelDetails represents model details
type ModelDetails struct {
	TotalSize int64 `json:"total_size"`
}

// HealthCheck performs a health check on the llama-server
func (c *Client) HealthCheck(ctx context.Context) error {
	url := c.baseURL + "/health"

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	return nil
}

// CreateEmbedding sends a text input to the embeddings endpoint and returns the float32 vector
func (c *Client) CreateEmbedding(ctx context.Context, input string, model string) ([]float32, error) {
	req := &EmbeddingRequest{
		Input: input,
		Model: model,
		Options: &EmbeddingOptions{
			PoolingType: "mean",
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	url := c.baseURL + "/v1/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send embedding request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("llama-server embedding error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var embedResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode embedding response: %w", err)
	}

	if len(embedResp.Data) == 0 {
		return nil, fmt.Errorf("llama-server returned empty embedding data")
	}

	return embedResp.Data[0].Embedding, nil
}

// ChatRequest represents a request to the llama-server chat endpoint
type ChatRequest struct {
	Messages    []Message `json:"messages"`
	Model       string    `json:"model,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// ChatResponse represents the response from llama-server
type ChatResponse struct {
	ID                string   `json:"id"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	Choices           []Choice `json:"choices"`
	Usage             Usage    `json:"usage"`
	SystemFingerprint string   `json:"system_fingerprint"`
}

// Choice represents a choice in the response
type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatMessage represents a message in the response
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Usage represents token usage statistics
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// EmbeddingOptions represents embedding request options
type EmbeddingOptions struct {
	PoolingType string `json:"pooling_type,omitempty"`
}

// EmbeddingRequest represents a request to the llama-server embeddings endpoint
type EmbeddingRequest struct {
	Input    string            `json:"input"`
	Model    string            `json:"model,omitempty"`
	Options  *EmbeddingOptions `json:"options,omitempty"`
}

// EmbeddingResponse represents the response from llama-server
type EmbeddingResponse struct {
	Object string      `json:"object"`
	Data   []Embedding `json:"data"`
	Model  string      `json:"model"`
	Usage  Usage       `json:"usage"`
}

// Embedding represents a single embedding vector
type Embedding struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}
