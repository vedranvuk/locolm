// Package wolfram provides Wolfram Alpha API tools for the locolm MCP server.
// It exposes five tools: wolfram_query (Full Results API), wolfram_llm (LLM API),
// wolfram_short (Short Answers API), wolfram_image (Simple API), and
// wolfram_recognize (Fast Query Recognizer API).
//
// Authentication: set WOLFRAM_APPID env var with your Wolfram AppID.
// Get one at https://developer.wolframalpha.com/
package wolfram

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vedranvuk/locolm/internal/mcp"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

type Config struct {
	AppID   string `json:"appid"`
	Timeout int    `json:"timeout_sec"`
}

func DefaultConfig() *Config {
	return &Config{
		AppID:   "",
		Timeout: 30,
	}
}

// ---------------------------------------------------------------------------
// Tool
// ---------------------------------------------------------------------------

type Wolfram struct {
	config *Config
}

func New(config *Config) (*Wolfram, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if v := os.Getenv("WOLFRAM_APPID"); v != "" {
		log.Printf("[WOLFRAM] AppID loaded from WOLFRAM_APPID env: %s...", v[:8])
	} else if config.AppID != "" {
		log.Printf("[WOLFRAM] AppID loaded from config: %s...", config.AppID[:8])
	} else {
		log.Printf("[WOLFRAM] WOLFRAM_APPID not set — wolfram tools will error on call")
	}

	return &Wolfram{
		config: config,
	}, nil
}

func (self *Wolfram) Register(r mcp.Registry) {
	// Tools are always registered. The AppID is read from the WOLFRAM_APPID
	// environment variable at call time (like the exa/google/newsapi tools),
	// so the tools work as long as the key is present when invoked.
	self.registerWolframQuery(r)
	self.registerWolframLLM(r)
	self.registerWolframShort(r)
	self.registerWolframImage(r)
	self.registerWolframRecognize(r)
}

// appID returns the Wolfram AppID, preferring the WOLFRAM_APPID environment
// variable and falling back to the configured value.
func (self *Wolfram) appID() string {
	if v := os.Getenv("WOLFRAM_APPID"); v != "" {
		return v
	}
	return self.config.AppID
}

// ---------------------------------------------------------------------------
// HTTP client
// ---------------------------------------------------------------------------

func (self *Wolfram) wolframGet(baseURL string, params url.Values, timeoutSec int) ([]byte, error) {
	appID := self.appID()
	if appID == "" {
		return nil, fmt.Errorf("wolfram tools require WOLFRAM_APPID environment variable to be set")
	}

	params.Set("appid", appID)
	fullURL := baseURL + "?" + params.Encode()

	// Log query without AppID for security
	logParams := url.Values{}
	for k, v := range params {
		if k != "appid" {
			logParams[k] = v
		}
	}
	log.Printf("[WOLFRAM] Request: %s?%s", baseURL, logParams.Encode())

	client := &http.Client{
		Timeout: time.Duration(timeoutSec) * time.Second,
	}

	resp, err := client.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("Wolfram API request failed: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[WOLFRAM] Response status: %d", resp.StatusCode)

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("Wolfram API: invalid AppID. Check WOLFRAM_APPID environment variable")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("Wolfram API: rate limit exceeded. Try again later")
	}
	if resp.StatusCode == http.StatusNotImplemented {
		return nil, fmt.Errorf("Wolfram API: query could not be interpreted")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Wolfram API returned %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Wolfram API response: %w", err)
	}

	return body, nil
}

// wolframGetImage makes a request to an API that returns raw image bytes.
// It returns the full URL (which the client can use directly) and the content type.
func (self *Wolfram) wolframGetImage(baseURL string, params url.Values, timeoutSec int) (string, string, error) {
	appID := self.appID()
	if appID == "" {
		return "", "", fmt.Errorf("wolfram tools require WOLFRAM_APPID environment variable to be set")
	}

	params.Set("appid", appID)
	fullURL := baseURL + "?" + params.Encode()

	// Log query without AppID for security
	logParams := url.Values{}
	for k, v := range params {
		if k != "appid" {
			logParams[k] = v
		}
	}
	log.Printf("[WOLFRAM] Image request: %s?%s", baseURL, logParams.Encode())

	client := &http.Client{
		Timeout: time.Duration(timeoutSec) * time.Second,
	}

	resp, err := client.Get(fullURL)
	if err != nil {
		return "", "", fmt.Errorf("Wolfram API request failed: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[WOLFRAM] Image response status: %d, content-type: %s", resp.StatusCode, resp.Header.Get("Content-Type"))

	if resp.StatusCode == http.StatusForbidden {
		return "", "", fmt.Errorf("Wolfram API: invalid AppID. Check WOLFRAM_APPID environment variable")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", "", fmt.Errorf("Wolfram API: rate limit exceeded. Try again later")
	}
	if resp.StatusCode == http.StatusNotImplemented {
		return "", "", fmt.Errorf("Wolfram API: query could not be interpreted")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("Wolfram API returned %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	// Read a few bytes to detect content type if not provided
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		buf := make([]byte, 512)
		n, _ := io.ReadFull(resp.Body, buf)
		contentType = http.DetectContentType(buf[:n])
		// Drain the rest so the connection can be reused
		io.Copy(io.Discard, resp.Body)
	}

	log.Printf("[WOLFRAM] Image content-type: %s", contentType)

	return fullURL, contentType, nil
}

// ---------------------------------------------------------------------------
// XML types for Full Results API
// ---------------------------------------------------------------------------

type QueryResult struct {
	XMLName       xml.Name     `xml:"queryresult"`
	Success       string       `xml:"success,attr"`
	Error         string       `xml:"error,attr"`
	NumPods       float64      `xml:"numpods,attr"`
	DataTypes     string       `xml:"datatypes,attr"`
	Timing        string       `xml:"timing,attr"`
	TimedOut      string       `xml:"timedout,attr"`
	ParseTime     string       `xml:"parsetiming,attr"`
	ParseTimedOut string       `xml:"parsetimedout,attr"`
	Recalculate   string       `xml:"recalculate,attr"`
	Version       string       `xml:"version,attr"`
	Pods          []Pod        `xml:"pod"`
	Assumptions   []Assumption `xml:"assumptions>assumption"`
	Warnings      []Warning    `xml:"warnings"`
	Sources       []Source     `xml:"sources>source"`
	DidYouMeans   []DidYouMean `xml:"didyoumeans>didyoumean"`
}

type Pod struct {
	XMLName    xml.Name `xml:"pod"`
	Title      string   `xml:"title,attr"`
	Scanner    string   `xml:"scanner,attr"`
	ID         string   `xml:"id,attr"`
	Position   int      `xml:"position,attr"`
	Primary    bool     `xml:"primary,attr"`
	Error      string   `xml:"error,attr"`
	NumSubPods int      `xml:"numsubpods,attr"`
	SubPods    []SubPod `xml:"subpod"`
	States     []State  `xml:"states>state"`
	Infos      []Info   `xml:"infos>info"`
}

type SubPodImage struct {
	Src   string `xml:"src,attr"`
	Alt   string `xml:"alt,attr"`
	Title string `xml:"title,attr"`
}

type SubPod struct {
	XMLName   xml.Name    `xml:"subpod"`
	Title     string      `xml:"title,attr"`
	PlainText string      `xml:"plaintext"`
	Image     SubPodImage `xml:"img"`
	MathML    string      `xml:"mathml"`
	MInput    string      `xml:"minput"`
	MOutput   string      `xml:"moutput"`
}

type State struct {
	XMLName xml.Name `xml:"state"`
	Name    string   `xml:"name,attr"`
	Input   string   `xml:"input,attr"`
}

type Info struct {
	XMLName xml.Name `xml:"info"`
	Text    string   `xml:"text,attr"`
}

type Assumption struct {
	XMLName xml.Name          `xml:"assumption"`
	Type    string            `xml:"type,attr"`
	Word    string            `xml:"word,attr"`
	Count   int               `xml:"count,attr"`
	Values  []AssumptionValue `xml:"value"`
}

type AssumptionValue struct {
	XMLName xml.Name `xml:"value"`
	Name    string   `xml:"name,attr"`
	Desc    string   `xml:"desc,attr"`
	Input   string   `xml:"input,attr"`
}

type Warning struct {
	XMLName     xml.Name            `xml:"warnings"`
	Reinterpret *ReinterpretWarning `xml:"reinterpret"`
	Spellcheck  *SpellcheckWarning  `xml:"spellcheck"`
	Translation *TranslationWarning `xml:"translation"`
	Delimiters  *DelimitersWarning  `xml:"delimiters"`
}

type ReinterpretWarning struct {
	XMLName xml.Name `xml:"reinterpret"`
	Text    string   `xml:"text,attr"`
	New     string   `xml:"new,attr"`
	Score   string   `xml:"score,attr"`
}

type SpellcheckWarning struct {
	XMLName    xml.Name `xml:"spellcheck"`
	Word       string   `xml:"word,attr"`
	Suggestion string   `xml:"suggestion,attr"`
	Text       string   `xml:"text,attr"`
}

type TranslationWarning struct {
	XMLName xml.Name `xml:"translation"`
	Phrase  string   `xml:"phrase,attr"`
	Trans   string   `xml:"trans,attr"`
	Lang    string   `xml:"lang,attr"`
	Text    string   `xml:"text,attr"`
}

type DelimitersWarning struct {
	XMLName xml.Name `xml:"delimiters"`
	Text    string   `xml:"text,attr"`
}

type Source struct {
	XMLName xml.Name `xml:"source"`
	URL     string   `xml:"url,attr"`
	Text    string   `xml:"text,attr"`
}

type DidYouMean struct {
	XMLName xml.Name `xml:"didyoumean"`
	Text    string   `xml:",chardata"`
	Score   string   `xml:"score,attr"`
	Level   string   `xml:"level,attr"`
}

// ---------------------------------------------------------------------------
// Output formatting helpers
// ---------------------------------------------------------------------------

func formatPod(pod Pod, format string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "--- %s", pod.Title)
	if pod.ID != "" {
		fmt.Fprintf(&sb, " [%s]", pod.ID)
	}
	if pod.Primary {
		fmt.Fprintf(&sb, " (primary)")
	}
	sb.WriteString(" ---\n")

	for _, subpod := range pod.SubPods {
		switch format {
		case "mathml":
			if subpod.MathML != "" {
				sb.WriteString(subpod.MathML)
				sb.WriteString("\n")
			}
		case "image":
			if subpod.Image.Src != "" {
				alt := subpod.Image.Alt
				if alt == "" {
					alt = subpod.Image.Title
				}
				if alt == "" {
					alt = pod.Title
				}
				fmt.Fprintf(&sb, "![%s](%s)\n", alt, subpod.Image.Src)
			}
		case "minput":
			if subpod.MInput != "" {
				sb.WriteString(subpod.MInput)
				sb.WriteString("\n")
			}
		case "moutput":
			if subpod.MOutput != "" {
				sb.WriteString(subpod.MOutput)
				sb.WriteString("\n")
			}
		default: // plaintext
			if subpod.PlainText != "" {
				sb.WriteString(subpod.PlainText)
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

func formatAssumptions(assumptions []Assumption) string {
	if len(assumptions) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\nAssumptions:\n")
	for _, a := range assumptions {
		if len(a.Values) > 0 {
			fmt.Fprintf(&sb, "- %q is assumed to be %s\n", a.Word, a.Values[0].Desc)
			for _, v := range a.Values {
				fmt.Fprintf(&sb, "  To change: &assumption=%s\n", v.Input)
			}
		}
	}
	return sb.String()
}

func formatDidYouMeans(didyous []DidYouMean) string {
	if len(didyous) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\nDid you mean:\n")
	for _, d := range didyous {
		fmt.Fprintf(&sb, "- %s", d.Text)
		if d.Score != "" {
			fmt.Fprintf(&sb, " (confidence: %s)", d.Score)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func formatWarnings(warnings []Warning) string {
	if len(warnings) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, w := range warnings {
		if w.Reinterpret != nil {
			fmt.Fprintf(&sb, "\nNote: %s\n", w.Reinterpret.Text)
			if w.Reinterpret.New != "" {
				fmt.Fprintf(&sb, "Interpreted as: %s\n", w.Reinterpret.New)
			}
		}
		if w.Spellcheck != nil {
			fmt.Fprintf(&sb, "\nNote: Interpreting %q as %q\n", w.Spellcheck.Word, w.Spellcheck.Suggestion)
		}
		if w.Translation != nil {
			fmt.Fprintf(&sb, "\nNote: Translated from %s: %q → %q\n", w.Translation.Lang, w.Translation.Phrase, w.Translation.Trans)
		}
		if w.Delimiters != nil {
			fmt.Fprintf(&sb, "\nNote: %s\n", w.Delimiters.Text)
		}
	}
	return sb.String()
}

func formatSources(sources []Source) string {
	if len(sources) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\nSources:\n")
	for _, s := range sources {
		fmt.Fprintf(&sb, "- [%s](%s)\n", s.Text, s.URL)
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func parseIntOr(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}
