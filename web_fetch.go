package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
	pdf "github.com/ledongthuc/pdf"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

type webFetchConfig struct {
	MaxBytes     int64
	MaxTextBytes int64
	Timeout      time.Duration
}

var webFetchCfg = webFetchConfig{
	MaxBytes:     5 * 1024 * 1024,
	MaxTextBytes: 200 * 1024,
	Timeout:      30 * time.Second,
}

func SetWebFetchConfig(maxBytes, maxTextBytes int64, timeoutSec int) {
	webFetchCfg = webFetchConfig{
		MaxBytes:     maxBytes,
		MaxTextBytes: maxTextBytes,
		Timeout:      time.Duration(timeoutSec) * time.Second,
	}
}

// ---------------------------------------------------------------------------
// Content-Type registry — add new extractors here
// ---------------------------------------------------------------------------

type contentType struct {
	mediaType string
	extract   func(body []byte, pageURL *url.URL) (string, error)
}

var contentTypes = []contentType{
	{
		mediaType: "text/plain",
		extract:   extractTextPlain,
	},
	{
		mediaType: "application/pdf",
		extract:   extractTextPDF,
	},
	{
		mediaType: "text/html",
		extract:   extractTextHTML,
	},
	{
		mediaType: "application/xhtml+xml",
		extract:   extractTextHTML,
	},
}

// blockedTypes are MIME types we explicitly reject.
var blockedTypes = []string{
	"application/zip",
	"application/gzip",
	"application/octet-stream",
	"application/msword",
	"application/vnd.openxmlformats-officedocument",
	"application/vnd.ms-excel",
	"application/vnd.ms-powerpoint",
	"image/",
	"audio/",
	"video/",
}

// ---------------------------------------------------------------------------
// HTTP client
// ---------------------------------------------------------------------------

func newHTTPClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			if isPrivateHost(req.URL.Hostname()) {
				return fmt.Errorf("redirect target is a private or internal network address")
			}
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// Public entry point
// ---------------------------------------------------------------------------

func webFetch(args map[string]string) (string, error) {
	pageURL, ok := args["url"]
	if !ok || pageURL == "" {
		return "", fmt.Errorf("missing required argument: url")
	}

	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	if err := validateURL(parsedURL); err != nil {
		return "", err
	}

	cfg := webFetchCfg
	client := newHTTPClient(cfg.Timeout)

	resp, err := doRequest(client, pageURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	log.Printf("[WEB_FETCH] Status: %d  Content-Type: %s", resp.StatusCode, resp.Header.Get("Content-Type"))

	if resp.StatusCode != http.StatusOK {
		return readErrorBody(resp)
	}

	mediatype := parseMediaType(resp.Header.Get("Content-Type"))
	if err := checkContentType(mediatype); err != nil {
		return "", err
	}

	body, err := readBody(resp.Body, cfg.MaxBytes)
	if err != nil {
		return "", err
	}

	result, err := extractText(mediatype, body, parsedURL)
	if err != nil {
		return "", err
	}

	return truncateText(result, cfg.MaxTextBytes), nil
}

// ---------------------------------------------------------------------------
// URL validation
// ---------------------------------------------------------------------------

func validateURL(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s (only http and https are allowed)", u.Scheme)
	}
	if isPrivateHost(u.Hostname()) {
		return fmt.Errorf("URL resolves to a private or internal network address, which is not allowed")
	}
	return nil
}

// ---------------------------------------------------------------------------
// HTTP request
// ---------------------------------------------------------------------------

func doRequest(client *http.Client, pageURL string) (*http.Response, error) {
	log.Printf("[WEB_FETCH] Fetching: %s", pageURL)

	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html, application/xhtml+xml, text/plain, application/pdf")
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	return resp, nil
}

// ---------------------------------------------------------------------------
// Content-Type parsing & validation
// ---------------------------------------------------------------------------

func parseMediaType(header string) string {
	if header == "" {
		return ""
	}
	mediatype, _, err := mime.ParseMediaType(header)
	if err != nil {
		log.Printf("[WEB_FETCH] Warning: could not parse Content-Type %q: %v", header, err)
		return ""
	}
	return mediatype
}

func checkContentType(mediatype string) error {
	if mediatype == "" {
		log.Printf("[WEB_FETCH] Warning: no Content-Type header, attempting HTML parse")
		return nil
	}
	for _, blocked := range blockedTypes {
		if strings.HasSuffix(blocked, "/") {
			if strings.HasPrefix(mediatype, blocked) {
				return fmt.Errorf("unsupported content type: %s — web_fetch does not extract text from binary or document formats", mediatype)
			}
		} else if mediatype == blocked {
			return fmt.Errorf("unsupported content type: %s — web_fetch does not extract text from binary or document formats", mediatype)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Body reading
// ---------------------------------------------------------------------------

func readBody(body io.Reader, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(body, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("response body exceeds maximum allowed size of %d bytes", maxBytes)
	}
	return data, nil
}

func readErrorBody(resp *http.Response) (string, error) {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return "", fmt.Errorf("URL returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

// ---------------------------------------------------------------------------
// Text extraction dispatcher
// ---------------------------------------------------------------------------

func extractText(mediatype string, body []byte, pageURL *url.URL) (string, error) {
	for _, ct := range contentTypes {
		if ct.mediaType == mediatype {
			return ct.extract(body, pageURL)
		}
	}
	// Unknown type — try HTML as fallback
	return extractTextHTML(body, pageURL)
}

// ---------------------------------------------------------------------------
// Extractors
// ---------------------------------------------------------------------------

func extractTextPlain(body []byte, _ *url.URL) (string, error) {
	return string(body), nil
}

func extractTextHTML(body []byte, pageURL *url.URL) (string, error) {
	article, err := readability.FromReader(bytes.NewReader(body), pageURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse page content: %w", err)
	}
	var sb strings.Builder
	if article.Title != "" {
		sb.WriteString(fmt.Sprintf("# %s\n\n", article.Title))
	}
	if article.Byline != "" {
		sb.WriteString(fmt.Sprintf("By: %s\n\n", article.Byline))
	}
	sb.WriteString(article.TextContent)
	return sb.String(), nil
}

func extractTextPDF(body []byte, _ *url.URL) (string, error) {
	reader := bytes.NewReader(body)
	pdfReader, err := pdf.NewReader(reader, int64(len(body)))
	if err != nil {
		return "", fmt.Errorf("failed to open PDF: %w", err)
	}

	var buf strings.Builder
	numPages := pdfReader.NumPage()

	for i := 1; i <= numPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}

		fonts := make(map[string]*pdf.Font)
		for _, name := range page.Fonts() {
			f := page.Font(name)
			fonts[name] = &f
		}

		text, err := page.GetPlainText(fonts)
		if err != nil {
			log.Printf("[WEB_FETCH] Warning: failed to extract text from PDF page %d: %v", i, err)
			continue
		}

		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString("\n\n")
			}
			buf.WriteString(text)
		}
	}

	if buf.Len() == 0 {
		return "", fmt.Errorf("PDF contained no extractable text (may be scanned/image-based)")
	}
	return buf.String(), nil
}

// ---------------------------------------------------------------------------
// Truncation
// ---------------------------------------------------------------------------

func truncateText(text string, maxBytes int64) string {
	if int64(len(text)) <= maxBytes {
		return text
	}
	return text[:maxBytes] + "\n\n[truncated — content exceeded maximum text length]"
}

// ---------------------------------------------------------------------------
// SSRF guard
// ---------------------------------------------------------------------------

var privateIPNetworks = []string{
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"127.0.0.0/8",
	"169.254.0.0/16",
	"::1/128",
	"fc00::/7",
	"fe80::/10",
}

func isPrivateHost(hostname string) bool {
	if hostname == "" {
		return false
	}
	ip := net.ParseIP(hostname)
	if ip != nil {
		return isPrivateIP(ip)
	}
	ips, err := net.LookupIP(hostname)
	if err != nil {
		log.Printf("[WEB_FETCH] Warning: could not resolve %s: %v", hostname, err)
		return false
	}
	for _, resolved := range ips {
		if isPrivateIP(resolved) {
			return true
		}
	}
	return false
}

func isPrivateIP(ip net.IP) bool {
	for _, cidr := range privateIPNetworks {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipnet.Contains(ip) {
			return true
		}
	}
	return false
}


