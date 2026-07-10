package client

import (
	"context"
	"net/http"
)

// ---------------------------------------------------------------------------
// Embedding — POST /embedding (native) and POST /v1/embeddings (OAI)
// ---------------------------------------------------------------------------

// EmbeddingRequest is the request body for the embeddings endpoints.
type EmbeddingRequest struct {
	common
	Input   string            `json:"input"`
	Options *EmbeddingOptions `json:"options,omitempty"`
	native  bool              `json:"-"`
}

// EmbeddingOptions controls pooling for native embeddings.
type EmbeddingOptions struct {
	PoolingType string `json:"pooling_type,omitempty"`
}

// WithEmbeddingModel sets the model alias.
func WithEmbeddingModel(model string) func(*EmbeddingRequest) {
	return func(r *EmbeddingRequest) { r.Model = model }
}

// WithEmbeddingPooling sets the pooling type (e.g. "mean", "cls", "last", "rank").
func WithEmbeddingPooling(pooling string) func(*EmbeddingRequest) {
	return func(r *EmbeddingRequest) {
		if r.Options == nil {
			r.Options = &EmbeddingOptions{}
		}
		r.Options.PoolingType = pooling
	}
}

// Embedding sends a text input to the embeddings endpoint and returns the
// full response. Use the OAI path by default; pass WithEmbeddingNative to use
// the native /embedding endpoint.
func (c *Client) Embedding(ctx context.Context, input string, opts ...func(*EmbeddingRequest)) (*EmbeddingResponse, error) {
	req := &EmbeddingRequest{Input: input}
	for _, opt := range opts {
		opt(req)
	}
	path := "/v1/embeddings"
	if req.native {
		path = "/embedding"
	}

	resp, err := c.do(ctx, http.MethodPost, path, nil, req, nil)
	if err != nil {
		return nil, err
	}
	var out EmbeddingResponse
	if err := decodeOK(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// WithEmbeddingNative selects the native /embedding endpoint instead of /v1/embeddings.
func WithEmbeddingNative() func(*EmbeddingRequest) {
	return func(r *EmbeddingRequest) { r.native = true }
}

// EmbeddingResponse is the embeddings response.
type EmbeddingResponse struct {
	Object string      `json:"object"`
	Data   []Embedding `json:"data"`
	Model  string      `json:"model"`
	Usage  Usage       `json:"usage"`
}

// Embedding is a single embedding vector entry.
type Embedding struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

// ---------------------------------------------------------------------------
// Tokenize — POST /tokenize
// ---------------------------------------------------------------------------

// TokenizeRequest is the request body for POST /tokenize.
type TokenizeRequest struct {
	common
	Content     string `json:"content"`
	AddSpecial  bool   `json:"add_special,omitempty"`
	ParseSpecial bool  `json:"parse_special,omitempty"`
	WithPieces  bool   `json:"with_pieces,omitempty"`
}

// WithTokenizeModel sets the model alias.
func WithTokenizeModel(model string) func(*TokenizeRequest) {
	return func(r *TokenizeRequest) { r.Model = model }
}

// WithTokenizeAddSpecial toggles insertion of special tokens (BOS).
func WithTokenizeAddSpecial(b bool) func(*TokenizeRequest) {
	return func(r *TokenizeRequest) { r.AddSpecial = b }
}

// WithTokenizeParseSpecial toggles tokenizing special tokens as tokens.
func WithTokenizeParseSpecial(b bool) func(*TokenizeRequest) {
	return func(r *TokenizeRequest) { r.ParseSpecial = b }
}

// WithTokenizeWithPieces includes token pieces in the response.
func WithTokenizeWithPieces(b bool) func(*TokenizeRequest) {
	return func(r *TokenizeRequest) { r.WithPieces = b }
}

// TokenizeRequestResult is a token id or id+piece pair.
type TokenizeResult struct {
	ID    int    `json:"id"`
	Piece any    `json:"piece,omitempty"`
}

// TokenizeResponse is the response from POST /tokenize.
type TokenizeResponse struct {
	Tokens []TokenizeResult `json:"tokens"`
}

// Tokenize tokenizes the given text.
func (c *Client) Tokenize(ctx context.Context, content string, opts ...func(*TokenizeRequest)) (*TokenizeResponse, error) {
	req := &TokenizeRequest{Content: content}
	for _, opt := range opts {
		opt(req)
	}
	resp, err := c.do(ctx, http.MethodPost, "/tokenize", nil, req, nil)
	if err != nil {
		return nil, err
	}
	var out TokenizeResponse
	if err := decodeOK(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---------------------------------------------------------------------------
// Detokenize — POST /detokenize
// ---------------------------------------------------------------------------

// DetokenizeRequest is the request body for POST /detokenize.
type DetokenizeRequest struct {
	common
	Tokens []int `json:"tokens"`
}

// WithDetokenizeModel sets the model alias.
func WithDetokenizeModel(model string) func(*DetokenizeRequest) {
	return func(r *DetokenizeRequest) { r.Model = model }
}

// DetokenizeResponse is the response from POST /detokenize.
type DetokenizeResponse struct {
	Content string `json:"content"`
}

// Detokenize converts tokens back to text.
func (c *Client) Detokenize(ctx context.Context, tokens []int, opts ...func(*DetokenizeRequest)) (*DetokenizeResponse, error) {
	req := &DetokenizeRequest{Tokens: tokens}
	for _, opt := range opts {
		opt(req)
	}
	resp, err := c.do(ctx, http.MethodPost, "/detokenize", nil, req, nil)
	if err != nil {
		return nil, err
	}
	var out DetokenizeResponse
	if err := decodeOK(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---------------------------------------------------------------------------
// ApplyTemplate — POST /apply-template
// ---------------------------------------------------------------------------

// ApplyTemplateRequest is the request body for POST /apply-template.
type ApplyTemplateRequest struct {
	common
	Messages []Message `json:"messages"`
}

// WithApplyTemplateModel sets the model alias.
func WithApplyTemplateModel(model string) func(*ApplyTemplateRequest) {
	return func(r *ApplyTemplateRequest) { r.Model = model }
}

// ApplyTemplateResponse is the response from POST /apply-template.
type ApplyTemplateResponse struct {
	Prompt string `json:"prompt"`
}

// ApplyTemplate formats chat messages using the model's chat template.
func (c *Client) ApplyTemplate(ctx context.Context, messages []Message, opts ...func(*ApplyTemplateRequest)) (string, error) {
	req := &ApplyTemplateRequest{Messages: messages}
	for _, opt := range opts {
		opt(req)
	}
	resp, err := c.do(ctx, http.MethodPost, "/apply-template", nil, req, nil)
	if err != nil {
		return "", err
	}
	var out ApplyTemplateResponse
	if err := decodeOK(resp, &out); err != nil {
		return "", err
	}
	return out.Prompt, nil
}

// ---------------------------------------------------------------------------
// Reranking — POST /reranking
// ---------------------------------------------------------------------------

// RerankRequest is the request body for POST /reranking.
type RerankRequest struct {
	common
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n,omitempty"`
}

// WithRerankModel sets the model alias.
func WithRerankModel(model string) func(*RerankRequest) {
	return func(r *RerankRequest) { r.Model = model }
}

// WithRerankTopN limits the number of returned results.
func WithRerankTopN(n int) func(*RerankRequest) {
	return func(r *RerankRequest) { r.TopN = n }
}

// RerankResult is a single reranked document entry.
type RerankResult struct {
	Index          int     `json:"index"`
	Document       string  `json:"document,omitempty"`
	RelevanceScore float64 `json:"relevance_score"`
}

// RerankResponse is the response from POST /reranking.
type RerankResponse struct {
	Results []RerankResult `json:"results"`
}

// Rerank reranks documents against a query.
func (c *Client) Rerank(ctx context.Context, query string, documents []string, opts ...func(*RerankRequest)) (*RerankResponse, error) {
	req := &RerankRequest{Query: query, Documents: documents}
	for _, opt := range opts {
		opt(req)
	}
	resp, err := c.do(ctx, http.MethodPost, "/reranking", nil, req, nil)
	if err != nil {
		return nil, err
	}
	var out RerankResponse
	if err := decodeOK(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
