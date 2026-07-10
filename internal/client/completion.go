package client

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// ---------------------------------------------------------------------------
// Completion (native) — POST /completion
// ---------------------------------------------------------------------------

// CompletionRequest is the request body for POST /completion.
type CompletionRequest struct {
	common
	Prompt           any      `json:"prompt"`
	Temperature      float64  `json:"temperature,omitempty"`
	DynamicTempRange float64  `json:"dynatemp_range,omitempty"`
	DynamicTempExp   float64  `json:"dynatemp_exponent,omitempty"`
	TopK             int      `json:"top_k,omitempty"`
	TopP             float64  `json:"top_p,omitempty"`
	MinP             float64  `json:"min_p,omitempty"`
	TopNSigma        float64  `json:"top_n_sigma,omitempty"`
	XTCProbability   float64  `json:"xtc_probability,omitempty"`
	XTCThreshold     float64  `json:"xtc_threshold,omitempty"`
	TypicalP         float64  `json:"typical_p,omitempty"`
	RepeatPenalty    float64  `json:"repeat_penalty,omitempty"`
	RepeatLastN      int      `json:"repeat_last_n,omitempty"`
	PresencePenalty  float64  `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64  `json:"frequency_penalty,omitempty"`
	DRYMultiplier    float64  `json:"dry_multiplier,omitempty"`
	DRYBase          float64  `json:"dry_base,omitempty"`
	DRYAllowedLength int      `json:"dry_allowed_length,omitempty"`
	DRYPenaltyLastN  int      `json:"dry_penalty_last_n,omitempty"`
	DRYSequenceBreak []string `json:"dry_sequence_breakers,omitempty"`
	Mirostat         int      `json:"mirostat,omitempty"`
	MirostatTau      float64  `json:"mirostat_tau,omitempty"`
	MirostatEta      float64  `json:"mirostat_eta,omitempty"`
	Grammar          string   `json:"grammar,omitempty"`
	JSONSchema       any      `json:"json_schema,omitempty"`
	Seed             int      `json:"seed,omitempty"`
	IgnoreEOS        bool     `json:"ignore_eos,omitempty"`
	LogitBias        any      `json:"logit_bias,omitempty"`
	NProbs           int      `json:"n_probs,omitempty"`
	MinKeep          int      `json:"min_keep,omitempty"`
	NPredict         int      `json:"n_predict,omitempty"`
	NIndent          int      `json:"n_indent,omitempty"`
	NKeep            int      `json:"n_keep,omitempty"`
	NCMPL            int      `json:"n_cmpl,omitempty"`
	NCacheReuse      int      `json:"n_cache_reuse,omitempty"`
	Stream           bool     `json:"stream,omitempty"`
	Stop             []string `json:"stop,omitempty"`
	CachePrompt      *bool    `json:"cache_prompt,omitempty"`
	ReturnTokens     bool     `json:"return_tokens,omitempty"`
	Samplers         []string `json:"samplers,omitempty"`
	IDSlot           int      `json:"id_slot,omitempty"`
	TimingsPerToken  bool     `json:"timings_per_token,omitempty"`
	ReturnProgress   bool     `json:"return_progress,omitempty"`
	PostSamplingProbs bool    `json:"post_sampling_probs,omitempty"`
}

// WithCompletionModel sets the model alias.
func WithCompletionModel(model string) func(*CompletionRequest) {
	return func(r *CompletionRequest) { r.Model = model }
}

// WithCompletionPrompt sets the prompt (string or token array).
func WithCompletionPrompt(prompt any) func(*CompletionRequest) {
	return func(r *CompletionRequest) { r.Prompt = prompt }
}

// WithCompletionTemperature sets the sampling temperature.
func WithCompletionTemperature(t float64) func(*CompletionRequest) {
	return func(r *CompletionRequest) { r.Temperature = t }
}

// WithCompletionTopP sets nucleus sampling probability.
func WithCompletionTopP(p float64) func(*CompletionRequest) {
	return func(r *CompletionRequest) { r.TopP = p }
}

// WithCompletionTopK sets top-k sampling.
func WithCompletionTopK(k int) func(*CompletionRequest) {
	return func(r *CompletionRequest) { r.TopK = k }
}

// WithCompletionNPredict sets the number of tokens to predict (-1 = infinity).
func WithCompletionNPredict(n int) func(*CompletionRequest) {
	return func(r *CompletionRequest) { r.NPredict = n }
}

// WithCompletionStop sets the stop sequences.
func WithCompletionStop(stop ...string) func(*CompletionRequest) {
	return func(r *CompletionRequest) { r.Stop = stop }
}

// WithCompletionSeed sets the RNG seed.
func WithCompletionSeed(seed int) func(*CompletionRequest) {
	return func(r *CompletionRequest) { r.Seed = seed }
}

// WithCompletionGrammar constrains generation with a BNF grammar.
func WithCompletionGrammar(g string) func(*CompletionRequest) {
	return func(r *CompletionRequest) { r.Grammar = g }
}

// WithCompletionJSONSchema constrains generation with a JSON schema.
func WithCompletionJSONSchema(s any) func(*CompletionRequest) {
	return func(r *CompletionRequest) { r.JSONSchema = s }
}

// WithCompletionCachePrompt toggles prompt caching.
func WithCompletionCachePrompt(enabled bool) func(*CompletionRequest) {
	return func(r *CompletionRequest) { r.CachePrompt = &enabled }
}

// WithCompletionSamplers sets the ordered sampler chain.
func WithCompletionSamplers(samplers ...string) func(*CompletionRequest) {
	return func(r *CompletionRequest) { r.Samplers = samplers }
}

// Completion sends a non-streaming native completion request.
func (c *Client) Completion(ctx context.Context, prompt any, opts ...func(*CompletionRequest)) (*CompletionResponse, error) {
	req := &CompletionRequest{Prompt: prompt}
	for _, opt := range opts {
		opt(req)
	}
	req.Stream = false

	resp, err := c.do(ctx, http.MethodPost, "/completion", nil, req, nil)
	if err != nil {
		return nil, err
	}
	var out CompletionResponse
	if err := decodeOK(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CompletionStream sends a streaming native completion request.
func (c *Client) CompletionStream(ctx context.Context, prompt any, opts ...func(*CompletionRequest)) (*CompletionStream, error) {
	req := &CompletionRequest{Prompt: prompt}
	for _, opt := range opts {
		opt(req)
	}
	req.Stream = true

	resp, err := c.do(ctx, http.MethodPost, "/completion", nil, req, map[string]string{
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
	return &CompletionStream{scanner: bufio.NewScanner(resp.Body), closer: resp.Body}, nil
}

// CompletionStream decodes SSE events from a streaming completion response.
type CompletionStream struct {
	scanner *bufio.Scanner
	closer  io.Closer
}

// Recv returns the next completion event, or io.EOF when done.
func (s *CompletionStream) Recv() (*CompletionResponse, error) {
	for s.scanner.Scan() {
		line := strings.TrimSpace(s.scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return nil, io.EOF
		}
		var resp CompletionResponse
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			return nil, err
		}
		return &resp, nil
	}
	if err := s.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

// Close releases the underlying connection.
func (s *CompletionStream) Close() error { return s.closer.Close() }

// CompletionResponse is the native /completion response.
type CompletionResponse struct {
	Content              string             `json:"content"`
	Tokens               []int              `json:"tokens,omitempty"`
	Stop                 bool               `json:"stop"`
	GenerationSettings   any                `json:"generation_settings,omitempty"`
	Model                string             `json:"model"`
	Prompt               string             `json:"prompt"`
	StopType             string             `json:"stop_type"`
	StoppingWord         string             `json:"stopping_word"`
	Timings              any                `json:"timings,omitempty"`
	TokensCached         int                `json:"tokens_cached"`
	TokensEvaluated      int                `json:"tokens_evaluated"`
	Truncated            bool               `json:"truncated"`
	CompletionProbabilities []CompletionProb `json:"completion_probabilities,omitempty"`
}

// CompletionProb holds per-token top logprobs.
type CompletionProb struct {
	TopLogProbs []TopLogProb `json:"top_logprobs"`
}

// TopLogProb is a single token probability entry.
type TopLogProb struct {
	Token   string  `json:"token"`
	LogProb float64 `json:"logprob"`
}

// ---------------------------------------------------------------------------
// Completions (OpenAI-compatible) — POST /v1/completions
// ---------------------------------------------------------------------------

// OAICRequest is the request body for POST /v1/completions.
type OAICRequest struct {
	common
	Prompt            any      `json:"prompt"`
	MaxTokens         int      `json:"max_tokens,omitempty"`
	Temperature       float64  `json:"temperature,omitempty"`
	TopP              float64  `json:"top_p,omitempty"`
	TopK              int      `json:"top_k,omitempty"`
	N                 int      `json:"n,omitempty"`
	Stop              []string `json:"stop,omitempty"`
	Stream            bool     `json:"stream,omitempty"`
	Seed              int      `json:"seed,omitempty"`
	LogProbs          bool     `json:"logprobs,omitempty"`
	TopLogProbs       int      `json:"top_logprobs,omitempty"`
	Echo              bool     `json:"echo,omitempty"`
	LogitBias         any      `json:"logit_bias,omitempty"`
	FrequencyPenalty  float64  `json:"frequency_penalty,omitempty"`
	PresencePenalty   float64  `json:"presence_penalty,omitempty"`
	RepeatPenalty     float64  `json:"repeat_penalty,omitempty"`
	Grammar           string   `json:"grammar,omitempty"`
	JSONSchema        any      `json:"json_schema,omitempty"`
}

// WithOAICModel sets the model alias.
func WithOAICModel(model string) func(*OAICRequest) {
	return func(r *OAICRequest) { r.Model = model }
}

// WithOAICPrompt sets the prompt.
func WithOAICPrompt(prompt any) func(*OAICRequest) {
	return func(r *OAICRequest) { r.Prompt = prompt }
}

// WithOAICMaxTokens sets the max tokens.
func WithOAICMaxTokens(n int) func(*OAICRequest) {
	return func(r *OAICRequest) { r.MaxTokens = n }
}

// WithOAICTemperature sets the temperature.
func WithOAICTemperature(t float64) func(*OAICRequest) {
	return func(r *OAICRequest) { r.Temperature = t }
}

// WithOAICTopP sets nucleus sampling probability.
func WithOAICTopP(p float64) func(*OAICRequest) {
	return func(r *OAICRequest) { r.TopP = p }
}

// WithOAICStop sets the stop sequences.
func WithOAICStop(stop ...string) func(*OAICRequest) {
	return func(r *OAICRequest) { r.Stop = stop }
}

// WithOAICSeed sets the RNG seed.
func WithOAICSeed(seed int) func(*OAICRequest) {
	return func(r *OAICRequest) { r.Seed = seed }
}

// Completions sends an OpenAI-compatible completion request.
func (c *Client) Completions(ctx context.Context, prompt any, opts ...func(*OAICRequest)) (*OAICResponse, error) {
	req := &OAICRequest{Prompt: prompt}
	for _, opt := range opts {
		opt(req)
	}
	req.Stream = false

	resp, err := c.do(ctx, http.MethodPost, "/v1/completions", nil, req, nil)
	if err != nil {
		return nil, err
	}
	var out OAICResponse
	if err := decodeOK(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// OAICResponse is the OpenAI-compatible completion response.
type OAICResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OAICChoice   `json:"choices"`
	Usage   Usage          `json:"usage"`
}

// OAICChoice is a single OpenAI-compatible completion choice.
type OAICChoice struct {
	Text         string `json:"text"`
	Index        int    `json:"index"`
	FinishReason string `json:"finish_reason"`
	LogProbs     any    `json:"logprobs,omitempty"`
}

// ---------------------------------------------------------------------------
// Infill — POST /infill
// ---------------------------------------------------------------------------

// InfillRequest is the request body for POST /infill.
type InfillRequest struct {
	common
	InputPrefix string          `json:"input_prefix"`
	InputSuffix string          `json:"input_suffix"`
	InputExtra  []InfillExtra   `json:"input_extra,omitempty"`
	Prompt      string          `json:"prompt,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	// All /completion options are also accepted; reuse CompletionRequest fields.
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
	NPredict    int     `json:"n_predict,omitempty"`
	Stop        []string `json:"stop,omitempty"`
	Seed        int     `json:"seed,omitempty"`
	Grammar     string  `json:"grammar,omitempty"`
}

// InfillExtra is additional context for repo-level infilling.
type InfillExtra struct {
	Filename string `json:"filename"`
	Text     string `json:"text"`
}

// WithInfillModel sets the model alias.
func WithInfillModel(model string) func(*InfillRequest) {
	return func(r *InfillRequest) { r.Model = model }
}

// WithInfillContext sets prefix/suffix/extra context.
func WithInfillContext(prefix, suffix string, extra ...InfillExtra) func(*InfillRequest) {
	return func(r *InfillRequest) {
		r.InputPrefix = prefix
		r.InputSuffix = suffix
		r.InputExtra = extra
	}
}

// WithInfillPrompt sets the prompt inserted after FIM_MID.
func WithInfillPrompt(p string) func(*InfillRequest) {
	return func(r *InfillRequest) { r.Prompt = p }
}

// WithInfillNPredict sets the number of tokens to predict.
func WithInfillNPredict(n int) func(*InfillRequest) {
	return func(r *InfillRequest) { r.NPredict = n }
}

// WithInfillStop sets the stop sequences.
func WithInfillStop(stop ...string) func(*InfillRequest) {
	return func(r *InfillRequest) { r.Stop = stop }
}

// Infill performs code infilling and returns a native CompletionResponse.
func (c *Client) Infill(ctx context.Context, opts ...func(*InfillRequest)) (*CompletionResponse, error) {
	req := &InfillRequest{}
	for _, opt := range opts {
		opt(req)
	}
	req.Stream = false

	resp, err := c.do(ctx, http.MethodPost, "/infill", nil, req, nil)
	if err != nil {
		return nil, err
	}
	var out CompletionResponse
	if err := decodeOK(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
