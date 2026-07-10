package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Chat (OpenAI-compatible) — POST /v1/chat/completions
// ---------------------------------------------------------------------------

// ChatRequest is the request body for POST /v1/chat/completions.
type ChatRequest struct {
	common
	Messages          []Message        `json:"messages"`
	Temperature       float64          `json:"temperature,omitempty"`
	TopP              float64          `json:"top_p,omitempty"`
	TopK              int              `json:"top_k,omitempty"`
	N                 int              `json:"n,omitempty"`
	MaxTokens         int              `json:"max_tokens,omitempty"`
	NPredict          int              `json:"n_predict,omitempty"`
	Stop              []string         `json:"stop,omitempty"`
	Stream            bool             `json:"stream,omitempty"`
	StreamOptions     *StreamOptions   `json:"stream_options,omitempty"`
	Seed              int              `json:"seed,omitempty"`
	ResponseFormat    *ResponseFormat  `json:"response_format,omitempty"`
	Tools             []Tool           `json:"tools,omitempty"`
	ToolChoice        any              `json:"tool_choice,omitempty"`
	LogitBias         map[string]any   `json:"logit_bias,omitempty"`
	LogProbs          bool             `json:"logprobs,omitempty"`
	TopLogProbs       int              `json:"top_logprobs,omitempty"`
	User              string           `json:"user,omitempty"`
	FrequencyPenalty  float64          `json:"frequency_penalty,omitempty"`
	PresencePenalty   float64          `json:"presence_penalty,omitempty"`
	RepeatPenalty     float64          `json:"repeat_penalty,omitempty"`
	RepeatLastN       int              `json:"repeat_last_n,omitempty"`
	TypicalP          float64          `json:"typical_p,omitempty"`
	MinP              float64          `json:"min_p,omitempty"`
	TemperatureLast   bool             `json:"temperature_last,omitempty"`
	Grammar           string           `json:"grammar,omitempty"`
	JSONSchema        any              `json:"json_schema,omitempty"`
	DisableSeed       bool             `json:"disable_seed,omitempty"`
	ReasonFormat      string           `json:"reasoning_format,omitempty"`
	ReasoningBudget   int              `json:"reasoning_budget,omitempty"`
	ChatTemplateKw    any              `json:"chat_template_kwargs,omitempty"`
	SkipChatParsing   bool             `json:"skip_chat_parsing,omitempty"`
	GenPrompt         string           `json:"generation_prompt,omitempty"`
	ParseToolCalls    bool             `json:"parse_tool_calls,omitempty"`
	ParallelToolCalls bool             `json:"parallel_tool_calls,omitempty"`
	CachePrompt       *bool            `json:"cache_prompt,omitempty"`
	IdSlot            int              `json:"id_slot,omitempty"`
	TimingsPerToken   bool             `json:"timings_per_token,omitempty"`
}

// common holds fields shared by all request types that accept a model alias.
type common struct {
	Model string `json:"model,omitempty"`
}

func (c *common) setModel(m string) { c.Model = m }

// StreamOptions controls incremental streaming output.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// ResponseFormat constrains the output format (e.g. JSON object).
type ResponseFormat struct {
	Type   string `json:"type"`
	Schema any    `json:"schema,omitempty"`
}

// Tool describes a tool the model may call.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// WithModel sets the model alias for the chat request.
func WithChatModel(model string) func(*ChatRequest) {
	return func(r *ChatRequest) { r.Model = model }
}

// WithTemperature sets the sampling temperature.
func WithTemperature(temp float64) func(*ChatRequest) {
	return func(r *ChatRequest) { r.Temperature = temp }
}

// WithTopP sets nucleus sampling probability.
func WithTopP(topP float64) func(*ChatRequest) {
	return func(r *ChatRequest) { r.TopP = topP }
}

// WithTopK sets top-k sampling.
func WithTopK(k int) func(*ChatRequest) {
	return func(r *ChatRequest) { r.TopK = k }
}

// WithMaxTokens sets the maximum number of tokens to generate.
func WithMaxTokens(n int) func(*ChatRequest) {
	return func(r *ChatRequest) { r.MaxTokens = n }
}

// WithNPredict sets the number of tokens to predict (native alias).
func WithNPredict(n int) func(*ChatRequest) {
	return func(r *ChatRequest) { r.NPredict = n }
}

// WithStop sets the stop sequences.
func WithStop(stop ...string) func(*ChatRequest) {
	return func(r *ChatRequest) { r.Stop = stop }
}

// WithSeed sets the RNG seed (-1 = random).
func WithSeed(seed int) func(*ChatRequest) {
	return func(r *ChatRequest) { r.Seed = seed }
}

// WithN sets the number of completions to generate.
func WithN(n int) func(*ChatRequest) {
	return func(r *ChatRequest) { r.N = n }
}

// WithResponseFormatJSON requests a JSON object response.
func WithResponseFormatJSON() func(*ChatRequest) {
	return func(r *ChatRequest) { r.ResponseFormat = &ResponseFormat{Type: "json_object"} }
}

// WithResponseFormatSchema requests schema-constrained JSON output.
func WithResponseFormatSchema(schema any) func(*ChatRequest) {
	return func(r *ChatRequest) { r.ResponseFormat = &ResponseFormat{Type: "json_schema", Schema: schema} }
}

// WithTools enables the given tools for the request.
func WithTools(tools ...Tool) func(*ChatRequest) {
	return func(r *ChatRequest) { r.Tools = tools }
}

// WithToolChoice sets the tool choice (e.g. "auto", "none", or a specific tool).
func WithToolChoice(choice any) func(*ChatRequest) {
	return func(r *ChatRequest) { r.ToolChoice = choice }
}

// WithLogitBias sets logit biases keyed by token id or string.
func WithLogitBias(bias map[string]any) func(*ChatRequest) {
	return func(r *ChatRequest) { r.LogitBias = bias }
}

// WithLogProbs enables token log probabilities with the given top count.
func WithLogProbs(topLogProbs int) func(*ChatRequest) {
	return func(r *ChatRequest) {
		r.LogProbs = true
		r.TopLogProbs = topLogProbs
	}
}

// WithGrammar constrains generation with a BNF grammar.
func WithGrammar(grammar string) func(*ChatRequest) {
	return func(r *ChatRequest) { r.Grammar = grammar }
}

// WithJSONSchema constrains generation with a JSON schema.
func WithJSONSchema(schema any) func(*ChatRequest) {
	return func(r *ChatRequest) { r.JSONSchema = schema }
}

// WithFrequencyPenalty sets the frequency penalty.
func WithFrequencyPenalty(p float64) func(*ChatRequest) {
	return func(r *ChatRequest) { r.FrequencyPenalty = p }
}

// WithPresencePenalty sets the presence penalty.
func WithPresencePenalty(p float64) func(*ChatRequest) {
	return func(r *ChatRequest) { r.PresencePenalty = p }
}

// WithRepeatPenalty sets the repeat penalty.
func WithRepeatPenalty(p float64) func(*ChatRequest) {
	return func(r *ChatRequest) { r.RepeatPenalty = p }
}

// WithRepeatLastN sets the last-n tokens considered for repetition penalty.
func WithRepeatLastN(n int) func(*ChatRequest) {
	return func(r *ChatRequest) { r.RepeatLastN = n }
}

// WithTypicalP sets locally typical sampling parameter p.
func WithTypicalP(p float64) func(*ChatRequest) {
	return func(r *ChatRequest) { r.TypicalP = p }
}

// WithMinP sets the min-p sampling threshold.
func WithMinP(p float64) func(*ChatRequest) {
	return func(r *ChatRequest) { r.MinP = p }
}

// WithReasoningFormat sets the reasoning parse format.
func WithReasoningFormat(format string) func(*ChatRequest) {
	return func(r *ChatRequest) { r.ReasonFormat = format }
}

// WithReasoningBudget sets the token budget for thinking.
func WithReasoningBudget(n int) func(*ChatRequest) {
	return func(r *ChatRequest) { r.ReasoningBudget = n }
}

// WithCachePrompt toggles prompt caching for the request.
func WithCachePrompt(enabled bool) func(*ChatRequest) {
	return func(r *ChatRequest) { r.CachePrompt = &enabled }
}

// WithIDSlot assigns the request to a specific slot.
func WithIDSlot(id int) func(*ChatRequest) {
	return func(r *ChatRequest) { r.IdSlot = id }
}

// WithStreamOptions sets incremental stream options.
func WithStreamOptions(includeUsage bool) func(*ChatRequest) {
	return func(r *ChatRequest) { r.StreamOptions = &StreamOptions{IncludeUsage: includeUsage} }
}

// Chat sends a non-streaming chat completion request.
func (c *Client) Chat(ctx context.Context, messages []Message, opts ...func(*ChatRequest)) (*ChatResponse, error) {
	req := &ChatRequest{Messages: messages}
	for _, opt := range opts {
		opt(req)
	}
	req.Stream = false

	resp, err := c.do(ctx, http.MethodPost, "/v1/chat/completions", nil, req, nil)
	if err != nil {
		return nil, err
	}
	var out ChatResponse
	if err := decodeOK(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ChatStream sends a streaming chat completion request and returns a decoder
// that yields one ChatResponse per SSE event. Call Close when finished.
func (c *Client) ChatStream(ctx context.Context, messages []Message, opts ...func(*ChatRequest)) (*ChatStream, error) {
	req := &ChatRequest{Messages: messages}
	for _, opt := range opts {
		opt(req)
	}
	req.Stream = true

	resp, err := c.do(ctx, http.MethodPost, "/v1/chat/completions", nil, req, map[string]string{
		"Cache-Control": "no-cache",
		"Connection":    "keep-alive",
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, &Error{Status: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	return &ChatStream{scanner: bufio.NewScanner(resp.Body), closer: resp.Body}, nil
}

// ChatStream decodes Server-Sent Events from a streaming chat response.
type ChatStream struct {
	scanner *bufio.Scanner
	closer  io.Closer
}

// Recv returns the next chat response event. It returns io.EOF when the stream
// is exhausted.
func (s *ChatStream) Recv() (*ChatResponse, error) {
	for s.scanner.Scan() {
		line := strings.TrimSpace(s.scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return nil, io.EOF
		}
		var resp ChatResponse
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			return nil, fmt.Errorf("decode stream event: %w", err)
		}
		return &resp, nil
	}
	if err := s.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

// Close releases the underlying connection.
func (s *ChatStream) Close() error {
	return s.closer.Close()
}

// ChatResponse is the OpenAI-compatible chat completion response.
type ChatResponse struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	Choices           []Choice `json:"choices"`
	Usage             Usage    `json:"usage"`
	SystemFingerprint string   `json:"system_fingerprint,omitempty"`
}

// Choice is a single completion choice.
type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	Delta        *ChatMessage `json:"delta,omitempty"`
	FinishReason string      `json:"finish_reason"`
	LogProbs     any         `json:"logprobs,omitempty"`
}

// ChatMessage is a message in a chat response.
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// Error represents a non-2xx llama-server response.
type Error struct {
	Status int
	Body   string
}

func (e *Error) Error() string {
	return "llama-server error (status " + strconv.Itoa(e.Status) + "): " + e.Body
}
