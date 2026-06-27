// Package wikidata provides a Wikidata query tool for the locolm MCP server.
// It exposes three modes: entity lookup, text search, and SPARQL queries.
package wikidata

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/vedranvuk/locolm/internal/mcp"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

type wikidataConfig struct {
	Endpoint           string `json:"endpoint"`
	SPARQLEndpoint     string `json:"sparql_endpoint"`
	UserAgent          string `json:"user_agent"`
	TimeoutSec         int    `json:"timeout_sec"`
	MaxEntitiesPerReq  int    `json:"max_entities_per_request"`
}

var wikidataCfg = wikidataConfig{
	Endpoint:          "https://www.wikidata.org/w/api.php",
	SPARQLEndpoint:    "https://query.wikidata.org/sparql",
	UserAgent:         "locolm/1.0 (https://github.com/vedranvuk/locolm)",
	TimeoutSec:        30,
	MaxEntitiesPerReq: 50,
}

func init() {
	mcp.RegisterTool(
		"wikidata_query",
		"Query Wikidata for structured knowledge about entities, people, places, concepts, and more. Supports three modes: 'entity' (fetch by Q-ID like Q42), 'search' (text search for entities), and 'sparql' (run a SPARQL query for complex data retrieval).",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"mode":  {"type": "string", "description": "Query mode: 'entity' (fetch by Q-ID), 'search' (text search), or 'sparql' (SPARQL query)"},
				"query": {"type": "string", "description": "The query: Q-ID like 'Q42' (entity mode), search text like 'Ada Lovelace' (search mode), or a SPARQL query (sparql mode)"},
				"lang":  {"type": "string", "description": "Language code for labels, e.g. 'en', 'de', 'fr' (default: 'en')"},
				"limit": {"type": "string", "description": "Max results for search mode (default 10, max 50)"}
			},
			"required": ["mode", "query"]
		}`),
		wikidataQuery,
	)
}

// ---------------------------------------------------------------------------
// Tool implementation
// ---------------------------------------------------------------------------

func wikidataQuery(args map[string]string) (string, error) {
	mode, ok := args["mode"]
	if !ok || mode == "" {
		return "", fmt.Errorf("missing required argument: mode")
	}

	query, ok := args["query"]
	if !ok || query == "" {
		return "", fmt.Errorf("missing required argument: query")
	}

	// Normalize SPARQL queries: LLMs may send escaped newlines (\\n as literal
	// backslash-n) or double-escaped backslashes. Convert them to real newlines
	// so the SPARQL engine receives a valid query.
	query = normalizeSPARQL(query)

	lang := args["lang"]
	if lang == "" {
		lang = "en"
	}

	switch mode {
	case "entity":
		return queryEntity(query, lang)
	case "search":
		limit := 10
		if v, ok := args["limit"]; ok && v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
				if limit > 50 {
					limit = 50
				}
			}
		}
		return searchEntities(query, lang, limit)
	case "sparql":
		return querySPARQL(query, lang)
	default:
		return "", fmt.Errorf("unknown mode: %s (valid: entity, search, sparql)", mode)
	}
}

// ---------------------------------------------------------------------------
// HTTP client
// ---------------------------------------------------------------------------

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: time.Duration(wikidataCfg.TimeoutSec) * time.Second,
	}
}

// ---------------------------------------------------------------------------
// Entity mode
// ---------------------------------------------------------------------------

func queryEntity(idsStr, lang string) (string, error) {
	// Parse Q-IDs (comma or pipe separated)
	ids := parseQIDs(idsStr)
	if len(ids) == 0 {
		return "", fmt.Errorf("no valid Q-IDs found in query: %s", idsStr)
	}

	// Batch if more than max
	if len(ids) > wikidataCfg.MaxEntitiesPerReq {
		ids = ids[:wikidataCfg.MaxEntitiesPerReq]
	}

	client := newHTTPClient()
	var allResults []map[string]interface{}

	// Process in batches
	for i := 0; i < len(ids); i += wikidataCfg.MaxEntitiesPerReq {
		end := i + wikidataCfg.MaxEntitiesPerReq
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[i:end]

		params := url.Values{}
		params.Set("action", "wbgetentities")
		params.Set("ids", strings.Join(batch, "|"))
		params.Set("format", "json")
		params.Set("formatversion", "2")
		params.Set("uselang", lang)

		req, err := http.NewRequest("POST", wikidataCfg.Endpoint, strings.NewReader(params.Encode()))
		if err != nil {
			return "", fmt.Errorf("failed to create entity request: %v", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", wikidataCfg.UserAgent)

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("Wikidata API request failed: %v", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("failed to read Wikidata response: %v", err)
		}

		if resp.StatusCode == 429 {
			return "", fmt.Errorf("Wikidata rate limit exceeded, please try again later")
		}
		if resp.StatusCode != 200 {
			return "", fmt.Errorf("Wikidata API returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
		}

		var result struct {
			Entities map[string]json.RawMessage `json:"entities"`
			Error    *struct {
				Code string `json:"code"`
				Info string `json:"info"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return "", fmt.Errorf("failed to parse Wikidata response: %v", err)
		}
		if result.Error != nil {
			return "", fmt.Errorf("Wikidata API error: %s - %s", result.Error.Code, result.Error.Info)
		}

		for id, raw := range result.Entities {
			var entity map[string]interface{}
			if err := json.Unmarshal(raw, &entity); err != nil {
				log.Printf("[WIKIDATA] Failed to unmarshal entity %s: %v", id, err)
				continue
			}
			condensed := condenseEntity(entity, lang)
			condensed["_id"] = id
			allResults = append(allResults, condensed)
		}
	}

	output, _ := json.MarshalIndent(allResults, "", "  ")
	return string(output), nil
}

// parseQIDs extracts Q-IDs from a string (comma, pipe, or space separated)
func parseQIDs(s string) []string {
	// Replace commas and pipes with spaces, then split
	s = strings.ReplaceAll(s, ",", " ")
	s = strings.ReplaceAll(s, "|", " ")
	fields := strings.Fields(s)

	var ids []string
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if strings.HasPrefix(f, "Q") || strings.HasPrefix(f, "q") {
			// Validate it's Q followed by digits
			num := strings.ToUpper(f)[1:]
			if isAllDigits(num) {
				ids = append(ids, "Q"+num)
			}
		}
	}
	return ids
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// condenseEntity extracts a curated set of useful properties from a raw entity
func condenseEntity(entity map[string]interface{}, lang string) map[string]interface{} {
	result := map[string]interface{}{}

	// Basic info
	if label, ok := entity["label"].(map[string]interface{}); ok {
		if v, ok := label[lang].(string); ok {
			result["label"] = v
		} else if v, ok := label["value"].(string); ok {
			result["label"] = v
		}
	}
	if desc, ok := entity["description"].(map[string]interface{}); ok {
		if v, ok := desc[lang].(string); ok {
			result["description"] = v
		} else if v, ok := desc["value"].(string); ok {
			result["description"] = v
		}
	}
	if aliases, ok := entity["aliases"].(map[string]interface{}); ok {
		if langAliases, ok := aliases[lang].([]interface{}); ok {
			var aliasStrs []string
			for _, a := range langAliases {
				if am, ok := a.(map[string]interface{}); ok {
					if v, ok := am["value"].(string); ok {
						aliasStrs = append(aliasStrs, v)
					}
				}
			}
			if len(aliasStrs) > 0 {
				result["aliases"] = aliasStrs
			}
		}
	}

	// Claims — curated list of commonly useful properties
	claims, ok := entity["claims"].(map[string]interface{})
	if !ok {
		return result
	}

	props := []string{
		"P31",   // instance of
		"P279",  // subclass of
		"P17",   // country
		"P131",  // located in administrative entity
		"P19",   // place of birth
		"P20",   // place of death
		"P21",   // sex or gender
		"P22",   // father
		"P25",   // mother
		"P26",   // spouse
		"P40",   // child
		"P69",   // educated at
		"P106",  // occupation
		"P1082", // population
		"P159",  // headquarters location
		"P2043", // length
		"P2044", // elevation
		"P2046", // area
		"P2067", // mass
		"P569",  // date of birth
		"P570",  // date of death
		"P571",  // inception
		"P576",  // dissolved/abolished
		"P577",  // publication date
		"P580",  // start time
		"P582",  // end time
		"P585",  // point in time
		"P625",  // coordinate location
		"P856",  // official website
		"P921",  // main subject
	}

	for _, propID := range props {
		if propClaims, ok := claims[propID].([]interface{}); ok && len(propClaims) > 0 {
			var values []map[string]interface{}
			for _, c := range propClaims {
				claim, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				// Only include truthy (preferred or normal rank) statements
				rank, _ := claim["rank"].(string)
				if rank == "deprecated" {
					continue
				}
				mainsnak, ok := claim["mainsnak"].(map[string]interface{})
				if !ok {
					continue
				}
				datavalue, ok := mainsnak["datavalue"].(map[string]interface{})
				if !ok {
					continue
				}
				value := datavalue["value"]
				datatype, _ := mainsnak["datatype"].(string)

				resolved := resolveValue(value, datatype, lang)
				if resolved != nil {
					v := map[string]interface{}{
						"value": resolved,
					}
					if rank == "preferred" {
						v["rank"] = "preferred"
					}
					values = append(values, v)
				}
			}
			if len(values) > 0 {
				result["P"+strings.ToLower(propID[1:])] = values
				// Also add with full P-ID for clarity
				result[propID] = values
			}
		}
	}

	// Sitelinks count
	if sitelinks, ok := entity["sitelinks"].(map[string]interface{}); ok {
		result["sitelinks_count"] = len(sitelinks)
	}

	return result
}

// resolveValue converts a Wikidata datavalue to a human-readable form
func resolveValue(value interface{}, datatype string, lang string) interface{} {
	switch datatype {
	case "wikibase-item":
		// Entity reference — return Q-ID and try to get label
		if v, ok := value.(map[string]interface{}); ok {
			id, _ := v["id"].(string)
			return map[string]interface{}{
				"id":    id,
				"label": id, // LLM can look up the label via another query if needed
			}
		}

	case "string":
		return value

	case "quantity":
		if v, ok := value.(map[string]interface{}); ok {
			amount, _ := v["amount"].(string)
			unit, _ := v["unit"].(string)
			return map[string]interface{}{
				"amount": amount,
				"unit":   unit,
			}
		}

	case "time":
		if v, ok := value.(map[string]interface{}); ok {
			timeStr, _ := v["time"].(string)
			precision, _ := v["precision"].(float64)
			return map[string]interface{}{
				"time":      timeStr,
				"precision": int(precision),
			}
		}

	case "globecoordinate":
		if v, ok := value.(map[string]interface{}); ok {
			lat, _ := v["latitude"].(float64)
			lon, _ := v["longitude"].(float64)
			globe, _ := v["globe"].(string)
			return map[string]interface{}{
				"latitude":  lat,
				"longitude": lon,
				"globe":    globe,
			}
		}

	case "monolingualtext":
		if v, ok := value.(map[string]interface{}); ok {
			text, _ := v["text"].(string)
			lang, _ := v["language"].(string)
			return map[string]interface{}{
				"text":     text,
				"language": lang,
			}
		}
	}

	return value
}

// ---------------------------------------------------------------------------
// Search mode
// ---------------------------------------------------------------------------

func searchEntities(search, lang string, limit int) (string, error) {
	client := newHTTPClient()

	params := url.Values{}
	params.Set("action", "wbsearchentities")
	params.Set("search", search)
	params.Set("language", lang)
	params.Set("limit", strconv.Itoa(limit))
	params.Set("format", "json")
	params.Set("formatversion", "2")

	req, err := http.NewRequest("GET", wikidataCfg.Endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create search request: %v", err)
	}
	req.Header.Set("User-Agent", wikidataCfg.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Wikidata search request failed: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return "", fmt.Errorf("failed to read search response: %v", err)
	}

	if resp.StatusCode == 429 {
		return "", fmt.Errorf("Wikidata rate limit exceeded, please try again later")
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Wikidata search returned HTTP %d", resp.StatusCode)
	}

	var result struct {
		Search []struct {
			ID          string `json:"id"`
			Label       string `json:"label"`
			Description string `json:"description"`
			Aliases     []string `json:"aliases"`
		} `json:"search"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse search response: %v", err)
	}

	// Build clean output
	var results []map[string]interface{}
	for _, item := range result.Search {
		r := map[string]interface{}{
			"id":    item.ID,
			"label": item.Label,
		}
		if item.Description != "" {
			r["description"] = item.Description
		}
		if len(item.Aliases) > 0 {
			r["aliases"] = item.Aliases
		}
		results = append(results, r)
	}

	output, _ := json.MarshalIndent(results, "", "  ")
	return string(output), nil
}

// ---------------------------------------------------------------------------
// SPARQL mode
// ---------------------------------------------------------------------------

// normalizeSPARQL handles common LLM escaping artifacts in SPARQL queries:
//   - Literal backslash-n (\n as two chars) → real newline
//   - Double-escaped backslashes (\\) → single backslash
//   - Literal backslash-t (\t as two chars) → real tab
//   - Literal backslash-r (\r as two chars) → real carriage return
func normalizeSPARQL(query string) string {
	// Only process if the query contains backslash sequences
	if !strings.Contains(query, `\`) {
		return query
	}
	query = strings.ReplaceAll(query, `\\`, `█`) // temp placeholder
	query = strings.ReplaceAll(query, `\n`, "\n")
	query = strings.ReplaceAll(query, `\t`, "\t")
	query = strings.ReplaceAll(query, `\r`, "\r")
	query = strings.ReplaceAll(query, `█`, `\`)
	return query
}

func querySPARQL(query, lang string) (string, error) {
	client := newHTTPClient()

	// Inject label service if the query uses SELECT and doesn't already have it
	// (heuristic: check if query contains "SERVICE wikibase:label")
	if !strings.Contains(strings.ToLower(query), "service wikibase:label") {
		// Try to add label service — only for SELECT queries
		upperQuery := strings.ToUpper(strings.TrimSpace(query))
		if strings.HasPrefix(upperQuery, "SELECT") {
			// Add label service before the closing brace
			// This is a simple heuristic — find the last } and insert before it
			labelService := fmt.Sprintf(`
  SERVICE wikibase:label {
    bd:serviceParam wikibase:language "%s" .
  }`, lang)
			// Find last closing brace
			lastBrace := strings.LastIndex(query, "}")
			if lastBrace >= 0 {
				query = query[:lastBrace] + labelService + query[lastBrace:]
			}
		}
	}

	// POST the query
	formData := url.Values{}
	formData.Set("query", query)

	req, err := http.NewRequest("POST", wikidataCfg.SPARQLEndpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create SPARQL request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/sparql-results+json")
	req.Header.Set("User-Agent", wikidataCfg.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("SPARQL request failed: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return "", fmt.Errorf("failed to read SPARQL response: %v", err)
	}

	if resp.StatusCode == 429 {
		return "", fmt.Errorf("Wikidata rate limit exceeded, please try again later")
	}
	if resp.StatusCode == 400 {
		return "", fmt.Errorf("SPARQL query error (bad query): %s", truncate(string(body), 500))
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("SPARQL endpoint returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	// Parse and re-format for readability
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse SPARQL response: %v", err)
	}

	// Simplify the SPARQL JSON results format
	simplified := simplifySPARQLResult(result)
	output, _ := json.MarshalIndent(simplified, "", "  ")
	return string(output), nil
}

// simplifySPARQLResult converts the verbose SPARQL JSON format to a cleaner form
func simplifySPARQLResult(raw map[string]interface{}) interface{} {
	head, ok := raw["head"].(map[string]interface{})
	if !ok {
		return raw
	}
	vars, _ := head["vars"].([]interface{})

	results, ok := raw["results"].(map[string]interface{})
	if !ok {
		return raw
	}
	bindings, ok := results["bindings"].([]interface{})
	if !ok {
		return raw
	}

	var rows []map[string]interface{}
	for _, b := range bindings {
		row := map[string]interface{}{}
		binding, ok := b.(map[string]interface{})
		if !ok {
			continue
		}
		for _, v := range vars {
			varName, _ := v.(string)
			if val, ok := binding[varName]; ok {
				valMap, ok := val.(map[string]interface{})
				if ok {
					// Extract the actual value
					if value, ok := valMap["value"]; ok {
						row[varName] = value
					} else {
						row[varName] = valMap
					}
				} else {
					row[varName] = val
				}
			}
		}
		rows = append(rows, row)
	}

	return map[string]interface{}{
		"vars":    vars,
		"results": rows,
		"count":   len(rows),
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
