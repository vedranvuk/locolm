package gsearch

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/vedranvuk/locolm/internal/mcp"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

type Config struct {
	NumResults   int    `json:"num_results"`
	StartIndex   int    `json:"start_index"`
	DateRestrict string `json:"date_restrict"`
	GL           string `json:"gl"`
	LR           string `json:"lr"`
}

func DefaultConfig() *Config {
	return &Config{
		NumResults:   10,
		StartIndex:   0,
		DateRestrict: "",
		GL:           "hr",
		LR:           "lang_en",
	}
}

// ---------------------------------------------------------------------------
// Tool
// ---------------------------------------------------------------------------

type GoogleSearch struct {
	config *Config
}

func New(config *Config) (*GoogleSearch, error) {
	if config == nil {
		config = DefaultConfig()
	}
	return &GoogleSearch{
		config: config,
	}, nil
}

func (self *GoogleSearch) Register(r mcp.Registry) {
	r.RegisterTool(
		"google_search",
		"Search the web using Google.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": { "type": "string", "description": "The search query" },
				"num": { "type": "string", "description": "Number of results (1-10)" },
				"start": { "type": "string", "description": "Start index for pagination" },
				"dateRestrict": { "type": "string", "description": "Date range (e.g., 'd1', 'w1', 'm1', 'y1')" },
				"gl": { "type": "string", "description": "Geopolitical country code (e.g., 'hr')" },
				"lr": { "type": "string", "description": "Language code (e.g., 'lang_en')" }
			},
			"required": ["query"]
		}`),
		self.googleSearch,
	)
}

func (self *GoogleSearch) googleSearch(args map[string]string) (string, error) {
	if os.Getenv("GOOGLE_API_KEY") == "" || os.Getenv("GOOGLE_CSE_ID") == "" {
		return "", fmt.Errorf("google_search requires GOOGLE_API_KEY and GOOGLE_CSE_ID environment variables")
	}

	params := url.Values{}
	params.Add("key", os.Getenv("GOOGLE_API_KEY"))
	params.Add("cx", os.Getenv("GOOGLE_CSE_ID"))
	params.Add("q", args["query"])

	optional := []string{"num", "start", "dateRestrict", "gl", "lr"}
	for _, key := range optional {
		if val, ok := args[key]; ok && val != "" {
			params.Add(key, val)
		}
	}

	apiURL := "https://www.googleapis.com/customsearch/v1?" + params.Encode()

	log.Printf("[SEARCH] Query: %q", args["query"])

	resp, err := http.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("Google API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Google API returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read API response: %w", err)
	}

	var searchResp SearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var results []string
	for i, item := range searchResp.Items {
		entry := fmt.Sprintf("%d. %s\n   URL: %s\n   Snippet: %s", i+1, item.Title, item.Link, item.Snippet)
		results = append(results, entry)
	}

	return fmt.Sprintf("Found %d results:\n\n%s", len(searchResp.Items), strings.Join(results, "\n\n")), nil
}

type SearchResponse struct {
	Items []struct {
		Title       string `json:"title"`
		Link        string `json:"link"`
		DisplayLink string `json:"displayLink"`
		Snippet     string `json:"snippet"`
	} `json:"items"`
}
