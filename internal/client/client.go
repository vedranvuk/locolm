// Package client implements a complete client for the llama-server HTTP API.
//
// All methods follow a uniform calling convention:
//   - Required inputs are positional arguments.
//   - Every optional input is supplied through functional options of the form
//     func(*XxxRequest), created with the With<Param> helpers.
//   - Cross-cutting request options (model selection) and client configuration
//     (HTTP client, timeout, API key) are also provided as functional options.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client handles communication with the llama-server API.
type Client struct {
	baseURL      string
	httpClient   *http.Client
	apiKey       string
	extraHeaders map[string]string
}

// ClientOption configures a Client at construction time.
type ClientOption func(*Client)

// WithHTTPClient sets a custom *http.Client.
func WithHTTPClient(c *http.Client) ClientOption {
	return func(cl *Client) { cl.httpClient = c }
}

// WithTimeout sets the default request timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(cl *Client) { cl.httpClient.Timeout = d }
}

// WithAPIKey sets the Bearer API key sent with every request.
func WithAPIKey(key string) ClientOption {
	return func(cl *Client) { cl.apiKey = key }
}

// WithHeader sets a default header sent with every request.
func WithHeader(key, value string) ClientOption {
	return func(cl *Client) {
		if cl.extraHeaders == nil {
			cl.extraHeaders = make(map[string]string)
		}
		cl.extraHeaders[key] = value
	}
}

// New creates a new llama-server client. baseURL should include the scheme and
// host (e.g. "http://127.0.0.1:8080"); a trailing slash is optional.
func New(baseURL string, opts ...ClientOption) *Client {
	cl := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(cl)
	}
	return cl
}

// do builds, sends and returns an HTTP request. The caller is responsible for
// closing the response body.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, headers map[string]string) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	for k, v := range c.extraHeaders {
		req.Header.Set(k, v)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	return resp, nil
}

// decodeOK checks the response status and decodes a successful JSON body into out.
func decodeOK(resp *http.Response, out any) error {
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("llama-server error (status %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// HealthCheck performs a health check against GET /health.
func (c *Client) HealthCheck(ctx context.Context) error {
	resp, err := c.do(ctx, http.MethodGet, "/health", nil, nil, nil)
	if err != nil {
		return err
	}
	return decodeOK(resp, nil)
}

// Models retrieves the list of loaded models (GET /v1/models).
func (c *Client) Models(ctx context.Context, opts ...func(*ModelsRequest)) ([]Model, error) {
	req := &ModelsRequest{}
	for _, opt := range opts {
		opt(req)
	}
	resp, err := c.do(ctx, http.MethodGet, "/v1/models", nil, nil, nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Data []Model `json:"data"`
	}
	if err := decodeOK(resp, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// ModelsRequest is the request for Models. The endpoint currently takes no
// body parameters; the type exists for option uniformity.
type ModelsRequest struct{}

// Model represents a model entry returned by /v1/models.
type Model struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	OwnedBy string       `json:"owned_by"`
	Details ModelDetails `json:"details,omitempty"`
}

// ModelDetails holds optional model metadata.
type ModelDetails struct {
	TotalSize int64 `json:"total_size,omitempty"`
}

// Usage represents token usage statistics.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Message represents a chat message.
type Message struct {
	Role       string     `json:"role"` // "system", "user", "assistant", "tool"
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolCall represents a tool invocation produced by the model.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction is the function portion of a ToolCall.
type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// itoa is a small helper for building numeric URL path segments.
func itoa(i int) string { return strconv.Itoa(i) }
