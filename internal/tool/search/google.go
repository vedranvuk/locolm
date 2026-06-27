package search

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/vedranvuk/locolm/internal/tool"
)

// --- Google Custom Search types ---

type SearchResponse struct {
	Items []struct {
		Title       string `json:"title"`
		Link        string `json:"link"`
		DisplayLink string `json:"displayLink"`
		Snippet     string `json:"snippet"`
	} `json:"items"`
}

func init() {
	tool.Register("google_search", tool.Tool{
		Name:        "google_search",
		Description: "Search the web using Google Custom Search",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "The search query"
				}
			},
			"required": ["query"]
		}`),
		Func: searchGoogle,
	})
}

// --- Google Search implementation ---

func searchGoogle(args map[string]string) (string, error) {
	query, ok := args["query"]
	if !ok || query == "" {
		return "", fmt.Errorf("missing required argument: query")
	}

	result, err := searchGoogleRaw(query)
	if err != nil {
		return "", err
	}
	return result, nil
}

func searchGoogleRaw(query string) (string, error) {
	apiURL := fmt.Sprintf(
		"https://www.googleapis.com/customsearch/v1?q=%s&cx=%s&key=%s",
		url.QueryEscape(query), os.Getenv("GOOGLE_CSE_ID"), os.Getenv("GOOGLE_API_KEY"),
	)

	log.Printf("[SEARCH] Query: %q", query)

	resp, err := http.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("Google API request failed: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[API] Google responded with status %d", resp.StatusCode)

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

	log.Printf("[RESULT] Found %d results", len(searchResp.Items))

	var results []string
	for i, item := range searchResp.Items {
		entry := fmt.Sprintf("%d. %s\n   URL: %s", i+1, item.Title, item.Link)
		if item.Snippet != "" {
			entry += fmt.Sprintf("\n   Snippet: %s", item.Snippet)
		}
		results = append(results, entry)
	}

	return fmt.Sprintf("Found %d results:\n\n%s", len(searchResp.Items), strings.Join(results, "\n\n")), nil
}
