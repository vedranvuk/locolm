package exa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/vedranvuk/locolm/internal/mcp"
)

// --- Exa Search types ---

type exaSearchRequest struct {
	Query        string          `json:"query"`
	Type         string          `json:"type"`
	NumResults   int             `json:"numResults"`
	Contents     exaContents     `json:"contents"`
	SystemPrompt string          `json:"systemPrompt,omitempty"`
	OutputSchema json.RawMessage `json:"outputSchema,omitempty"`

	IncludeDomains []string `json:"includeDomains,omitempty"`
	ExcludeDomains []string `json:"excludeDomains,omitempty"`
	StartPublished string   `json:"startPublishedDate,omitempty"`
	EndPublished   string   `json:"endPublishedDate,omitempty"`
}

type exaContents struct {
	Highlights bool        `json:"highlights,omitempty"`
	Text       *exaText    `json:"text,omitempty"`
	Summary    interface{} `json:"summary,omitempty"` // bool or {query, schema}
}

type exaText struct {
	MaxCharacters   int    `json:"maxCharacters,omitempty"`
	Verbosity       string `json:"verbosity,omitempty"`
	IncludeHTMLTags bool   `json:"includeHTMLTags,omitempty"`
}

type exaSearchResponse struct {
	RequestID  string      `json:"requestId"`
	SearchType string      `json:"searchType"`
	Results    []exaResult `json:"results"`
	Output     *exaOutput  `json:"output,omitempty"`
	Cost       exaCost     `json:"costDollars"`
}

type exaResult struct {
	Title           string    `json:"title"`
	URL             string    `json:"url"`
	ID              string    `json:"id"`
	PublishedDate   string    `json:"publishedDate"`
	Author          string    `json:"author"`
	Text            string    `json:"text"`
	Highlights      []string  `json:"highlights"`
	HighlightScores []float64 `json:"highlightScores"`
	Summary         string    `json:"summary"`
}

type exaOutput struct {
	Content   interface{}    `json:"content"`
	Grounding []exaGrounding `json:"grounding"`
}

type exaGrounding struct {
	Field      string        `json:"field"`
	Citations  []exaCitation `json:"citations"`
	Confidence string        `json:"confidence"`
}

type exaCitation struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type exaCost struct {
	Total float64 `json:"total"`
}

type Config struct {
}

func DefaultConfig() *Config { return &Config{} }

type Exa struct {
	config *Config
}

func New(config *Config) (*Exa, error) {
	return &Exa{
		config: config,
	}, nil
}

func (self *Exa) Register(r mcp.Registry) {
	r.RegisterTool(
		"exa_search",
		"Neural web search via Exa AI with highlights and synthesized answers. Best for deep research and structured data.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"query":             {"type": "string", "description": "Search query"},
				"type":              {"type": "string", "description": "Search type: auto (default), fast, instant, deep, deep-lite, deep-reasoning"},
				"num":               {"type": "string", "description": "Number of results (default 10)"},
				"include_domains":   {"type": "string", "description": "Comma-separated list of domains to restrict search to (e.g. 'github.com,stackoverflow.com')"},
				"exclude_domains":   {"type": "string", "description": "Comma-separated list of domains to exclude from results"},
				"start_date":        {"type": "string", "description": "Start date filter (e.g. '2025-01-01' or '2025-01-01T00:00:00Z')"},
				"end_date":          {"type": "string", "description": "End date filter (e.g. '2025-12-31' or '2025-12-31T23:59:59Z')"},
				"system_prompt":     {"type": "string", "description": "System prompt to guide synthesis behavior (used with output_schema)"},
				"output_schema":     {"type": "string", "description": "JSON Schema string for structured output (triggers synthesis)"}
			},
			"required": ["query"]
		}`),
		self.exaSearch,
	)
}

func (self *Exa) exaSearch(args map[string]string) (string, error) {
	query, ok := args["query"]
	if !ok || query == "" {
		return "", fmt.Errorf("missing required argument: query")
	}

	apiKey := os.Getenv("EXA_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("EXA_API_KEY environment variable is not set")
	}

	// Build request
	reqBody := exaSearchRequest{
		Query:      query,
		Type:       "auto",
		NumResults: 10,
		Contents:   exaContents{Highlights: true},
	}

	// Optional parameters
	if v, ok := args["type"]; ok && v != "" {
		reqBody.Type = v
	}
	if v, ok := args["num"]; ok && v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			reqBody.NumResults = n
		}
	}
	if v, ok := args["include_domains"]; ok && v != "" {
		reqBody.IncludeDomains = splitAndTrim(v)
	}
	if v, ok := args["exclude_domains"]; ok && v != "" {
		reqBody.ExcludeDomains = splitAndTrim(v)
	}
	if v, ok := args["start_date"]; ok && v != "" {
		reqBody.StartPublished = v
	}
	if v, ok := args["end_date"]; ok && v != "" {
		reqBody.EndPublished = v
	}
	if v, ok := args["system_prompt"]; ok && v != "" {
		reqBody.SystemPrompt = v
	}
	if v, ok := args["output_schema"]; ok && v != "" {
		var schema json.RawMessage
		if err := json.Unmarshal([]byte(v), &schema); err != nil {
			return "", fmt.Errorf("invalid output_schema JSON: %w", err)
		}
		reqBody.OutputSchema = schema
	}

	// Marshal request
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Exa request: %w", err)
	}

	log.Printf("[EXA] Query: %q (type=%s, num=%d)", query, reqBody.Type, reqBody.NumResults)

	// Make HTTP request
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", "https://api.exa.ai/search", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create Exa request: %w", err)
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Exa API request failed: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[EXA] Exa responded with status %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Exa API returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Exa response: %w", err)
	}

	var searchResp exaSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal Exa response: %w", err)
	}

	log.Printf("[EXA] Found %d results (cost: $%.4f)", len(searchResp.Results), searchResp.Cost.Total)

	// Format results
	var results []string
	for i, r := range searchResp.Results {
		entry := fmt.Sprintf("%d. %s\n   URL: %s", i+1, r.Title, r.URL)
		if r.Author != "" {
			entry += fmt.Sprintf("\n   Author: %s", r.Author)
		}
		if r.PublishedDate != "" {
			entry += fmt.Sprintf("\n   Date: %s", r.PublishedDate)
		}
		if r.Summary != "" {
			entry += fmt.Sprintf("\n   Summary: %s", r.Summary)
		}
		if len(r.Highlights) > 0 {
			for _, h := range r.Highlights {
				entry += fmt.Sprintf("\n   > %s", h)
			}
		}
		results = append(results, entry)
	}

	output := fmt.Sprintf("Found %d results (cost: $%.4f):\n\n%s", len(searchResp.Results), searchResp.Cost.Total, strings.Join(results, "\n\n"))

	// Include synthesized output if present
	if searchResp.Output != nil && searchResp.Output.Content != nil {
		contentBytes, err := json.MarshalIndent(searchResp.Output.Content, "", "  ")
		if err == nil {
			output += fmt.Sprintf("\n\n[Synthesized Output]\n%s", string(contentBytes))
		}
	}

	return output, nil
}

func splitAndTrim(s string) []string {
	var result []string
	for _, part := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
