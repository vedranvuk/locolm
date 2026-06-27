package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func main() {
	baseURL := "http://localhost:11501/mcp"
	doRequest := func(method, body, sessionID string) (string, string) {
		req, _ := http.NewRequest("POST", baseURL, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "request failed: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		for _, c := range resp.Header.Values("Mcp-Session-Id") {
			if c != "" {
				sessionID = c
			}
		}
		data, _ := io.ReadAll(resp.Body)
		return string(data), sessionID
	}

	// Step 1: Initialize
	fmt.Println("=== Initialize ===")
	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`
	resp, sid := doRequest("POST", initBody, "")
	fmt.Printf("Session-ID: %s\n", sid)
	fmt.Printf("Response: %s\n\n", resp)

	// Step 2: Initialized notification
	fmt.Println("=== Initialized Notification ===")
	initNotif := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	resp, sid = doRequest("POST", initNotif, sid)
	fmt.Printf("Session-ID: %s\n", sid)
	fmt.Printf("Response: %s\n\n", resp)

	// Helper to extract JSON from SSE data
	extractJSON := func(s string) string {
		for _, line := range strings.Split(s, "\n") {
			if strings.HasPrefix(line, "data: ") {
				return strings.TrimPrefix(line, "data: ")
			}
		}
		return ""
	}

	// Helper to send tools/call
	callTool := func(id int, argsJSON, sessionID string) {
		body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"wikidata_query","arguments":%s}}`, id, argsJSON)
		resp, _ := doRequest("POST", body, sessionID)
		jsonStr := extractJSON(resp)
		if jsonStr == "" {
			fmt.Printf("  Raw response: %s\n", resp)
			return
		}
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			fmt.Printf("  Parse error: %v\n  Raw: %s\n", err, jsonStr)
			return
		}
		pretty, _ := json.MarshalIndent(result, "  ", "  ")
		fmt.Printf("  %s\n", string(pretty))
	}

	// Test 1: SPARQL with literal newlines (the exact bug)
	fmt.Println("=== Test 1: SPARQL with literal newlines ===")
	args1 := `{"mode":"sparql","query":"PREFIX wd: <http://www.wikidata.org/entity/>\nPREFIX wdt: <http://www.wikidata.org/prop/direct/>\n\nSELECT ?item ?itemLabel WHERE {\n  ?item wdt:P31 wd:Q5 .\n  SERVICE wikibase:label { bd:serviceParam wikibase:language \"en\" . }\n}","lang":"en"}`
	callTool(10, args1, sid)
	fmt.Println()

	// Test 2: SPARQL with escaped newlines
	fmt.Println("=== Test 2: SPARQL with escaped newlines ===")
	args2 := `{"mode":"sparql","query":"PREFIX wd: <http://www.wikidata.org/entity/>\\nPREFIX wdt: <http://www.wikidata.org/prop/direct/>\\n\\nSELECT ?item ?itemLabel WHERE {\\n  ?item wdt:P31 wd:Q5 .\\n  SERVICE wikibase:label { bd:serviceParam wikibase:language \"en\" . }\\n}","lang":"en"}`
	callTool(11, args2, sid)
	fmt.Println()

	// Test 3: SPARQL with CRLF
	fmt.Println("=== Test 3: SPARQL with CRLF ===")
	args3 := "{\"mode\":\"sparql\",\"query\":\"PREFIX wd: <http://www.wikidata.org/entity/>\\r\\nPREFIX wdt: <http://www.wikidata.org/prop/direct/>\\r\\n\\r\\nSELECT ?item ?itemLabel WHERE {\\r\\n\\t?item wdt:P31 wd:Q5 .\\r\\n\\tSERVICE wikibase:label { bd:serviceParam wikibase:language \\\"en\\\" . }\\r\\n}\",\"lang\":\"en\"}"
	callTool(12, args3, sid)
	fmt.Println()

	// Test 4: Simple single-line SPARQL (baseline)
	fmt.Println("=== Test 4: Simple single-line SPARQL ===")
	args4 := `{"mode":"sparql","query":"SELECT ?item ?itemLabel WHERE { ?item wdt:P31 wd:Q5 . SERVICE wikibase:label { bd:serviceParam wikibase:language \"en\" . } }","lang":"en"}`
	callTool(13, args4, sid)
	fmt.Println()

	// Test 5: Entity mode Q42
	fmt.Println("=== Test 5: Entity mode Q42 ===")
	args5 := `{"mode":"entity","query":"Q42","lang":"en"}`
	callTool(14, args5, sid)
	fmt.Println()

	// Test 6: Search mode
	fmt.Println("=== Test 6: Search mode ===")
	args6 := `{"mode":"search","query":"Albert Einstein","lang":"en","limit":"3"}`
	callTool(15, args6, sid)
	fmt.Println()

	// Test 7: Complex SPARQL with embedded quotes and newlines
	fmt.Println("=== Test 7: Complex SPARQL with quotes and newlines ===")
	args7 := `{"mode":"sparql","query":"PREFIX wd: <http://www.wikidata.org/entity/>\nPREFIX wdt: <http://www.wikidata.org/prop/direct/>\nSELECT ?city ?cityLabel ?pop WHERE {\n  ?city wdt:P31 wd:Q515 .\n  ?city wdt:P17 wd:Q30 .\n  ?city wdt:P1082 ?pop .\n  FILTER(?pop > 1000000)\n  SERVICE wikibase:label { bd:serviceParam wikibase:language \"en\" . }\n}\nORDER BY DESC(?pop)\nLIMIT 5","lang":"en"}`
	callTool(16, args7, sid)
}
