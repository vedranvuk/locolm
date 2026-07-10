package newsapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/vedranvuk/locolm/internal/mcp"
)

// --- Response types ---

type newsAPIResponse struct {
	Status       string `json:"status"`
	TotalResults int    `json:"totalResults"`
	Articles     []struct {
		Source struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"source"`
		Author      string `json:"author"`
		Title       string `json:"title"`
		Description string `json:"description"`
		URL         string `json:"url"`
		URLToImage  string `json:"urlToImage"`
		PublishedAt string `json:"publishedAt"`
		Content     string `json:"content"`
	} `json:"articles"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type newsSourceResponse struct {
	Status  string `json:"status"`
	Sources []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		URL         string `json:"url"`
		Category    string `json:"category"`
		Language    string `json:"language"`
		Country     string `json:"country"`
	} `json:"sources"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

type Config struct {
	// No specific configuration needed for memory tool
}

func DefaultConfig() *Config {
	return &Config{}
}

type NewsAPITool struct {
	config *Config
}

func New(config *Config) (*NewsAPITool, error) {
	return &NewsAPITool{
		config: config,
	}, nil
}

// --- Tool registration ---
func (self *NewsAPITool) Register(r mcp.Registry) {

	r.RegisterTool(
		"news_search",
		"Search for news articles or headlines using newsapi.org. Supports two modes: 'everything' (full article search with filters like query, sources, domains, date range, language, sort) and 'headlines' (top/breaking headlines by country, category, or sources). Requires NEWSAPI_API_KEY env var.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"mode": {
					"type": "string",
					"description": "Search mode: 'everything' for full article search, 'headlines' for top headlines"
				},
				"q": {
					"type": "string",
					"description": "Keywords or phrase to search for (supports advanced search: quotes for exact match, +required, -excluded, AND/OR/NOT)"
				},
				"sources": {
					"type": "string",
					"description": "Comma-separated list of source identifiers (max 20). Use news_sources tool to find IDs."
				},
				"domains": {
					"type": "string",
					"description": "Comma-separated list of domains to restrict search to (e.g. 'bbc.co.uk,techcrunch.com')"
				},
				"exclude_domains": {
					"type": "string",
					"description": "Comma-separated list of domains to exclude from results"
				},
				"from": {
					"type": "string",
					"description": "Start date in ISO 8601 format (e.g. '2026-06-29' or '2026-06-29T19:31:02')"
				},
				"to": {
					"type": "string",
					"description": "End date in ISO 8601 format"
				},
				"language": {
					"type": "string",
					"description": "2-letter ISO-639-1 language code: ar, de, en, es, fr, he, it, nl, no, pt, ru, sv, ud, zh"
				},
				"sort_by": {
					"type": "string",
					"description": "Sort order for 'everything' mode: relevancy, popularity, publishedAt (default)"
				},
				"page_size": {
					"type": "string",
					"description": "Number of results per page (default 20, max 100)"
				},
				"page": {
					"type": "string",
					"description": "Page number for pagination (default 1)"
				},
				"country": {
					"type": "string",
					"description": "2-letter country code for headlines (headlines mode only): us, gb, de, fr, etc."
				},
				"category": {
					"type": "string",
					"description": "Category for headlines (headlines mode only): business, entertainment, general, health, science, sports, technology"
				}
			},
			"required": ["mode"]
		}`),
		newsSearchTool,
	)

	r.RegisterTool(
		"news_sources",
		"List available news sources from newsapi.org. Filter by category, language, or country. Requires NEWSAPI_API_KEY env var.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"category": {
					"type": "string",
					"description": "Filter by category: business, entertainment, general, health, science, sports, technology"
				},
				"language": {
					"type": "string",
					"description": "Filter by 2-letter language code: ar, de, en, es, fr, he, it, nl, no, pt, ru, sv, ud, zh"
				},
				"country": {
					"type": "string",
					"description": "Filter by 2-letter country code: us, gb, de, fr, etc."
				}
			},
			"required": []
		}`),
		newsSourcesTool,
	)
}

// --- Tool implementations ---

func newsSearchTool(args map[string]string) (string, error) {
	mode, ok := args["mode"]
	if !ok || mode == "" {
		return "", fmt.Errorf("missing required argument: mode ('everything' or 'headlines')")
	}

	if os.Getenv("NEWSAPI_API_KEY") == "" {
		return "", fmt.Errorf("news_search requires NEWSAPI_API_KEY environment variable to be set")
	}

	switch mode {
	case "everything":
		return searchEverything(args)
	case "headlines":
		return searchHeadlines(args)
	default:
		return "", fmt.Errorf("invalid mode: %q (must be 'everything' or 'headlines')", mode)
	}
}

func searchEverything(args map[string]string) (string, error) {
	apiKey := os.Getenv("NEWSAPI_API_KEY")

	params := url.Values{}
	params.Set("apiKey", apiKey)
	params.Set("pageSize", "20")

	if v, ok := args["q"]; ok && v != "" {
		params.Set("q", v)
	}
	if v, ok := args["sources"]; ok && v != "" {
		params.Set("sources", v)
	}
	if v, ok := args["domains"]; ok && v != "" {
		params.Set("domains", v)
	}
	if v, ok := args["exclude_domains"]; ok && v != "" {
		params.Set("excludeDomains", v)
	}
	if v, ok := args["from"]; ok && v != "" {
		params.Set("from", v)
	}
	if v, ok := args["to"]; ok && v != "" {
		params.Set("to", v)
	}
	if v, ok := args["language"]; ok && v != "" {
		params.Set("language", v)
	}
	if v, ok := args["sort_by"]; ok && v != "" {
		params.Set("sortBy", v)
	}
	if v, ok := args["page_size"]; ok && v != "" {
		params.Set("pageSize", v)
	}
	if v, ok := args["page"]; ok && v != "" {
		params.Set("page", v)
	}

	// If no query and no sources, require at least one filter
	if params.Get("q") == "" && params.Get("sources") == "" && params.Get("domains") == "" {
		return "", fmt.Errorf("everything mode requires at least one of: q, sources, or domains")
	}

	apiURL := "https://newsapi.org/v2/everything?" + params.Encode()
	log.Printf("[NEWSAPI] Everything search: %s", params.Get("q"))

	return fetchAndFormatNews(apiURL)
}

func searchHeadlines(args map[string]string) (string, error) {
	apiKey := os.Getenv("NEWSAPI_API_KEY")

	params := url.Values{}
	params.Set("apiKey", apiKey)
	params.Set("pageSize", "20")

	if v, ok := args["q"]; ok && v != "" {
		params.Set("q", v)
	}
	if v, ok := args["sources"]; ok && v != "" {
		params.Set("sources", v)
	}
	if v, ok := args["country"]; ok && v != "" {
		params.Set("country", v)
	}
	if v, ok := args["category"]; ok && v != "" {
		params.Set("category", v)
	}
	if v, ok := args["page_size"]; ok && v != "" {
		params.Set("pageSize", v)
	}
	if v, ok := args["page"]; ok && v != "" {
		params.Set("page", v)
	}

	// country, category, sources, or q required
	if params.Get("country") == "" && params.Get("category") == "" && params.Get("sources") == "" && params.Get("q") == "" {
		return "", fmt.Errorf("headlines mode requires at least one of: country, category, sources, or q")
	}

	apiURL := "https://newsapi.org/v2/top-headlines?" + params.Encode()
	log.Printf("[NEWSAPI] Headlines search")

	return fetchAndFormatNews(apiURL)
}

func fetchAndFormatNews(apiURL string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("newsapi.org request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("newsapi.org returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var newsResp newsAPIResponse
	if err := json.Unmarshal(body, &newsResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if newsResp.Status == "error" {
		return "", fmt.Errorf("newsapi.org error (%s): %s", newsResp.Code, newsResp.Message)
	}

	if len(newsResp.Articles) == 0 {
		return "No articles found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("News Results (%d total, showing %d)\n", newsResp.TotalResults, len(newsResp.Articles)))
	sb.WriteString(strings.Repeat("─", 60) + "\n")

	for i, article := range newsResp.Articles {
		sb.WriteString(fmt.Sprintf("\n%d. %s\n", i+1, article.Title))
		if article.Source.Name != "" {
			sb.WriteString(fmt.Sprintf("   Source: %s\n", article.Source.Name))
		}
		if article.Author != "" {
			sb.WriteString(fmt.Sprintf("   Author: %s\n", article.Author))
		}
		if article.PublishedAt != "" {
			sb.WriteString(fmt.Sprintf("   Published: %s\n", article.PublishedAt))
		}
		if article.Description != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", article.Description))
		}
		sb.WriteString(fmt.Sprintf("   URL: %s\n", article.URL))
	}

	return sb.String(), nil
}

func newsSourcesTool(args map[string]string) (string, error) {
	if os.Getenv("NEWSAPI_API_KEY") == "" {
		return "", fmt.Errorf("news_sources requires NEWSAPI_API_KEY environment variable to be set")
	}

	apiKey := os.Getenv("NEWSAPI_API_KEY")
	params := url.Values{}
	params.Set("apiKey", apiKey)

	if v, ok := args["category"]; ok && v != "" {
		params.Set("category", v)
	}
	if v, ok := args["language"]; ok && v != "" {
		params.Set("language", v)
	}
	if v, ok := args["country"]; ok && v != "" {
		params.Set("country", v)
	}

	apiURL := "https://newsapi.org/v2/top-headlines/sources?" + params.Encode()
	log.Printf("[NEWSAPI] Sources list")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("newsapi.org request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("newsapi.org returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var srcResp newsSourceResponse
	if err := json.Unmarshal(body, &srcResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if srcResp.Status == "error" {
		return "", fmt.Errorf("newsapi.org error (%s): %s", srcResp.Code, srcResp.Message)
	}

	if len(srcResp.Sources) == 0 {
		return "No sources found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("News Sources (%d found)\n", len(srcResp.Sources)))
	sb.WriteString(strings.Repeat("─", 60) + "\n")

	for _, src := range srcResp.Sources {
		sb.WriteString(fmt.Sprintf("\n• %s (%s)\n", src.Name, src.ID))
		if src.Description != "" {
			// Truncate long descriptions
			desc := src.Description
			if len(desc) > 120 {
				desc = desc[:117] + "..."
			}
			sb.WriteString(fmt.Sprintf("  %s\n", desc))
		}
		sb.WriteString(fmt.Sprintf("  Category: %s | Language: %s | Country: %s\n", src.Category, src.Language, src.Country))
		sb.WriteString(fmt.Sprintf("  URL: %s\n", src.URL))
	}

	return sb.String(), nil
}
