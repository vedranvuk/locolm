package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-shiori/go-readability"
)

func webFetch(args map[string]string) (string, error) {
	pageURL, ok := args["url"]
	if !ok || pageURL == "" {
		return "", fmt.Errorf("missing required argument: url")
	}

	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	log.Printf("[WEB_FETCH] Fetching: %s", pageURL)

	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[WEB_FETCH] Status: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("URL returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	article, err := readability.FromReader(resp.Body, parsedURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse page content: %w", err)
	}

	var result strings.Builder
	if article.Title != "" {
		result.WriteString(fmt.Sprintf("# %s\n\n", article.Title))
	}
	if article.Byline != "" {
		result.WriteString(fmt.Sprintf("By: %s\n\n", article.Byline))
	}
	result.WriteString(article.TextContent)

	return result.String(), nil
}
