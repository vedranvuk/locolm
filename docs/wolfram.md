# Wolfram Alpha API — Implementation Plan

## Overview

This plan adds comprehensive Wolfram Alpha API support to the locolm MCP server. It covers all Wolfram APIs that can be meaningfully exposed as MCP tools, respecting the MCP specification (JSON-RPC 2.0, Streamable HTTP), RFC standards, and the project's existing architecture and conventions.

---

## 1. Research Summary

### Wolfram APIs Available

| API | Auth | Output | Best For |
|-----|------|--------|----------|
| **Full Results API** | AppID | XML/JSON (pods, images, MathML, plaintext) | Structured computational knowledge, disambiguation, pod selection |
| **LLM API** | AppID | Plain text tables + image URLs | LLM-consumable text results (ideal for MCP return values) |
| **Short Answers API** | AppID | Short text string | Quick factual answers |
| **Simple API** | AppID | PNG/GIF image | Full-page result screenshots |
| **Spoken Results API** | AppID | Audio (MIDI/WAV) | Voice interfaces |
| **Fast Query Recognizer API** | AppID | JSON classification | Query triage/classification (<10ms) |
| **Summary Boxes API** | AppID | JSON + image | Pre-configured entity summaries |
| **Instant Calculators API** | AppID | JSON + image | Form-based calculators |

### Authentication Model

- Single **AppID** per application, obtained from [developer.wolframalpha.com](https://developer.wolframalpha.com/)
- Passed as `?appid=YOUR_APPID` query parameter on every request
- Alternatively as `Authorization: Bearer <AppID>` header (LLM API)
- AppID is NOT secret — it's a public identifier (like a Google CSE ID, not like an API key)

### Pricing

- **Free tier:** 2,000 calls/month (non-commercial)
- **Paid tiers:** Contact Wolfram sales — "flexible commercial licensing with low monthly plans"
- Rate limits: not explicitly documented; assume standard HTTP 429 for exceeded quotas

### Key Constraints

- All URLs must be URL-encoded
- Default response is XML; JSON available via `output=json`
- Async pod URLs expire after ~30 minutes
- Image URLs (except async) are permanent
- API versioned as `v2` (Full Results, Short Answers, Simple, etc.) and `v1` (LLM API)

---

## 2. Architecture Design

### 2.1 File Structure

```
internal/tool/wolfram/
    wolfram.go          # Core: HTTP client, config, shared types, pod parsing
    full_results.go     # wolfram_query tool (Full Results API)
    llm.go              # wolfram_llm tool (LLM API)
    short_answers.go    # wolfram_short tool (Short Answers API)
    simple.go           # wolfram_image tool (Simple API)
    recognize.go        # wolfram_recognize tool (Fast Query Recognizer API)
```

### 2.2 Config Integration

Following project conventions (API keys read via `os.Getenv()` in tool files, not stored in Config struct):

```go
// In wolfram.go
var wolframAppID string

func LoadWolframConfig(json.RawMessage) {
    // Optional: load from locolm.json
}

func init() {
    wolframAppID = os.Getenv("WOLFRAM_APPID")
    // Register all wolfram tools...
}
```

**Env var:** `WOLFRAM_APPID` — follows the existing `GOOGLE_API_KEY` / `EXA_API_ID` pattern. This is the ONLY new env var introduced.

**locolm.json support:** Optional `"wolfram"` object:
```json
{
    "wolfram": {
        "app_id_env": "WOLFRAM_APPID",
        "default_timeout_sec": 30,
        "max_width": 500
    }
}
```

### 2.3 Registration Pattern

Follows existing pattern — blank import in `main.go`:

```go
// In cmd/locolm/main.go
_ "github.com/vedranvuk/locolm/internal/tool/wolfram"
```

Each tool file's `init()` calls `mcp.RegisterTool()`.

### 2.4 HTTP Client

Dedicated function following the pattern in `google.go` and `wikidata.go`:

```go
func wolframRequest(baseURL string, params url.Values) ([]byte, error) {
    params.Set("appid", wolframAppID)
    fullURL := baseURL + "?" + params.Encode()
    
    log.Printf("[WOLFRAM] Request: %s", /* params without appid */)
    
    resp, err := http.Get(fullURL)
    if err != nil {
        return nil, fmt.Errorf("Wolfram API request failed: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("Wolfram API returned %d: %s", resp.StatusCode, string(body))
    }
    
    return io.ReadAll(resp.Body)
}
```

---

## 3. Tool Specifications

### 3.1 `wolfram_query` — Full Results API

**Endpoint:** `http://api.wolframalpha.com/v2/query`

**Purpose:** Full programmable access to Wolfram Alpha's computational knowledge. Returns structured results with pod-level granularity.

**Input Schema:**
```json
{
    "type": "object",
    "properties": {
        "input": {
            "type": "string",
            "description": "The query to compute. Natural language (e.g. 'population of France'), math (e.g. 'integrate x^2 dx'), or any Wolfram Alpha input."
        },
        "format": {
            "type": "string",
            "description": "Output format per pod. Comma-separated: 'plaintext', 'image', 'mathml', 'minput', 'moutput', 'cell'. Default: 'plaintext'.",
            "enum": ["plaintext", "image", "mathml", "minput", "moutput", "cell"]
        },
        "include_pod": {
            "type": "string",
            "description": "Include only pods by ID (e.g. 'Result', 'DecimalApproximation'). Comma-separated for multiple."
        },
        "exclude_pod": {
            "type": "string",
            "description": "Exclude pods by ID. Comma-separated for multiple."
        },
        "pod_title": {
            "type": "string",
            "description": "Include only pods matching this title. Supports * wildcard."
        },
        "pod_index": {
            "type": "string",
            "description": "Include only pods by position index (1-based). Comma-separated."
        },
        "scanner": {
            "type": "string",
            "description": "Include only pods from specific scanners (e.g. 'Data', 'Numeric'). Comma-separated."
        },
        "location": {
            "type": "string",
            "description": "Override query location (e.g. 'Boston, MA', 'Tokyo')."
        },
        "units": {
            "type": "string",
            "description": "Unit system: 'metric' or 'nonmetric' (US customary).",
            "enum": ["metric", "nonmetric"]
        },
        "width": {
            "type": "string",
            "description": "Approximate width for text/table pods in pixels (default 500)."
        },
        "timeout": {
            "type": "string",
            "description": "Max seconds for the query (default 20). Increase for heavy computations."
        },
        "assumption": {
            "type": "string",
            "description": "Apply an assumption token from a previous query's assumptions."
        }
    },
    "required": ["input"]
}
```

**Implementation Notes:**
- Parse XML response into structured text output
- Extract each pod's title, scanner, plaintext content
- For image format: return image URLs as Markdown `![alt](url)` 
- For mathml: return raw MathML XML
- Handle `success="false"` gracefully (return "Wolfram Alpha could not interpret the query")
- Handle `error="true"` (return error message)
- Parse `<didyoumeans>` for suggestions when query fails
- Parse `<assumptions>` and include in response metadata for follow-up queries
- Parse `<warnings>` (spellcheck, translation, reinterpret) and surface to client

**Output Format:**
```
Query: population of France

Success: true
Pods: 5
Timing: 6.27s

--- Pod 1: Input interpretation ---
France | country

--- Pod 2: Result (primary) ---
64.1 million people (world rank: 21st) (2014 estimate)

--- Pod 3: Recent population history ---
[table or image]

--- Pod 4: Long-term population history ---
[table or image]

--- Pod 5: Demographics ---
[table or image]

Assumptions used:
- "France" referring to a country

Did you mean: "population of Frankfurt"?
```

### 3.2 `wolfram_llm` — LLM API

**Endpoint:** `https://www.wolframalpha.com/api/v1/llm-api`

**Purpose:** Get Wolfram Alpha results optimized for LLM consumption. Returns structured text tables, image URLs, and a link to the full results page. This is the simplest tool for most use cases.

**Input Schema:**
```json
{
    "type": "object",
    "properties": {
        "input": {
            "type": "string",
            "description": "The query to compute. Natural language or mathematical expression."
        },
        "maxchars": {
            "type": "string",
            "description": "Maximum characters in response (default 6800)."
        },
        "location": {
            "type": "string",
            "description": "Override query location (e.g. 'Boston, MA')."
        },
        "units": {
            "type": "string",
            "description": "Unit system: 'metric' or 'nonmetric'.",
            "enum": ["metric", "nonmetric"]
        },
        "assumption": {
            "type": "string",
            "description": "Apply an assumption token from a previous query."
        }
    },
    "required": ["input"]
}
```

**Implementation Notes:**
- Returns plain text directly — no XML/JSON parsing needed
- Output is already structured for LLM consumption
- Image URLs in the output should be preserved as-is
- HTTP 501 means uninterpretable input — return helpful error with any suggested inputs
- This is the **recommended default** for most queries

**Output Format:** (direct passthrough of LLM API response)
```
Query: "10 densest elemental metals"

Input interpretation:
10 densest metallic elements | by mass density

Result:
1 | hassium | 41 g/cm^3 | 
2 | meitnerium | 37.4 g/cm^3 | 
...

Wolfram|Alpha website result: https://www.wolframalpha.com/input?i=...
```

### 3.3 `wolfram_short` — Short Answers API

**Endpoint:** `https://api.wolframalpha.com/v2/shortanswers`

**Purpose:** Get a short textual answer for a query. Optimized for rapid responses, small screens, and bot integration.

**Input Schema:**
```json
{
    "type": "object",
    "properties": {
        "input": {
            "type": "string",
            "description": "The query to answer. Best for simple factual questions like 'What is the capital of France?' or 'distance Earth Moon'."
        }
    },
    "required": ["input"]
}
```

**Implementation Notes:**
- Returns a single short text string
- Returns empty string if no short answer available
- Very fast (<1s typical)

### 3.4 `wolfram_image` — Simple API

**Endpoint:** `https://api.wolframalpha.com/v2/simple`

**Purpose:** Get a rendered image of the full Wolfram Alpha result page. Useful for displaying visual results.

**Input Schema:**
```json
{
    "type": "object",
    "properties": {
        "input": {
            "type": "string",
            "description": "The query to compute."
        },
        "width": {
            "type": "string",
            "description": "Image width in pixels (default 500)."
        },
        "font_size": {
            "type": "string",
            "description": "Font size for the rendering."
        },
        "layout": {
            "type": "string",
            "description": "Layout: 'label' or 'flow'."
        },
        "units": {
            "type": "string",
            "description": "Unit system: 'metric' or 'nonmetric'.",
            "enum": ["metric", "nonmetric"]
        },
        "location": {
            "type": "string",
            "description": "Override query location."
        },
        "exclude_pod": {
            "type": "string",
            "description": "Exclude pods by ID from the rendering."
        },
        "include_pod": {
            "type": "string",
            "description": "Include only specific pods by ID."
        }
    },
    "required": ["input"]
}
```

**Implementation Notes:**
- Returns a PNG or GIF image URL
- Return as Markdown image: `![Wolfram Alpha result](url)`
- Large images possible — respect `width` parameter

### 3.5 `wolfram_recognize` — Fast Query Recognizer API

**Endpoint:** `https://api.wolframalpha.com/v2/queryrecognizer`

**Purpose:** Quickly classify a query and determine if Wolfram Alpha can handle it. Runs in <10ms. Useful for triage before making a full query.

**Input Schema:**
```json
{
    "type": "object",
    "properties": {
        "input": {
            "type": "string",
            "description": "The query to classify."
        }
    },
    "required": ["input"]
}
```

**Implementation Notes:**
- Returns JSON with classification: whether WA can handle the query, category, priority
- Extremely fast — useful as a pre-check
- Return the classification result as formatted text

---

## 4. Tools NOT Implemented (and why)

### 4.1 Spoken Results API — **NOT SUPPORTED**
- **Reason:** MCP protocol has no audio transport mechanism. The Streamable HTTP transport does not support audio streaming. The tool would return audio file URLs, but MCP clients (llama-server, Claude Desktop, etc.) cannot render audio. This would be a hack.

### 4.2 Summary Boxes API — **NOT SUPPORTED**
- **Reason:** This API requires pre-configuration of summary boxes in the Wolfram Developer Portal. It's not a general-purpose query API — it serves pre-defined content that the developer has explicitly set up. It cannot be used dynamically for arbitrary queries. Adding it would require the user to manually configure boxes in the Wolfram portal first, which is out of scope for an MCP tool.

### 4.3 Instant Calculators API — **NOT SUPPORTED**
- **Reason:** This API generates HTML form widgets for web deployment. It returns HTML/JS embed code, not data. MCP clients render text, not web forms. This is a web widget API, not a data API. It would require a browser context to be useful.

### 4.4 `validatequery` Function — **NOT EXPOSED AS TOOL**
- **Reason:** This is a developer utility for testing, not a user-facing tool. It can be used internally by `wolfram_query` to provide better error messages ("Did you mean...?"), but doesn't need its own MCP tool.

---

## 5. Implementation Details

### 5.1 XML Parsing (Full Results API)

Go's `encoding/xml` with struct tags:

```go
type QueryResult struct {
    XMLName    xml.Name `xml:"queryresult"`
    Success    string   `xml:"success,attr"`
    Error      string   `xml:"error,attr"`
    NumPods    int      `xml:"numpods,attr"`
    DataTypes  string   `xml:"datatypes,attr"`
    Timing     string   `xml:"timing,attr"`
    TimedOut   string   `xml:"timedout,attr"`
    ParseTime  string   `xml:"parsetiming,attr"`
    Pods       []Pod    `xml:"pod"`
    Assumptions []Assumption `xml:"assumptions>assumption"`
    Warnings   []Warning `xml:"warnings>warning"`
    Sources    []Source `xml:"sources>source"`
    DidYouMeans []DidYouMean `xml:"didyoumeans>didyoumean"`
}

type Pod struct {
    XMLName   xml.Name `xml:"pod"`
    Title     string   `xml:"title,attr"`
    Scanner   string   `xml:"scanner,attr"`
    ID        string   `xml:"id,attr"`
    Position  int      `xml:"position,attr"`
    Primary   bool     `xml:"primary,attr"`
    SubPods   []SubPod `xml:"subpod"`
    States    []State  `xml:"states>state"`
}

type SubPod struct {
    XMLName  xml.Name `xml:"subpod"`
    Title    string   `xml:"title,attr"`
    PlainText string  `xml:"plaintext"`
    Image    string   `xml:"img>src,attr"`
    MathML   string   `xml:"mathml"`
}
```

### 5.2 Error Handling

| HTTP Status | Meaning | MCP Response |
|-------------|---------|--------------|
| 200 | OK | Return parsed result |
| 400 | Missing/invalid input | Error: "Wolfram API: missing or invalid input parameter" |
| 403 | Invalid/missing AppID | Error: "Wolfram API: invalid or missing AppID. Set WOLFRAM_APPID env var." |
| 429 | Rate limit exceeded | Error: "Wolfram API: rate limit exceeded. Try again later." |
| 501 | Cannot interpret (LLM API) | Error: "Wolfram Alpha could not interpret this query" + suggestions |
| 500 | Internal server error | Error: "Wolfram API: internal server error" |

### 5.3 Timeout Handling

- Default timeout: 20 seconds (Wolfram's `totaltimeout` default)
- For heavy computations: user can increase via `timeout` parameter
- Go `http.Client` with `Timeout` set accordingly
- Wolfram's internal timeouts: parse=5s, scan=3s, format=8s per pod, total=20s

### 5.4 Logging

Following existing pattern (`[SEARCH]`, `[WIKIDATA]`):
```
[WOLFRAM] Query: "population of France" (full)
[WOLFRAM] Query: "2+2" (llm)
[WOLFRAM] API responded with status 200
[WOLFRAM] Parsed 5 pods in 6.27s
```

### 5.5 Output Sanitization

Following the `sanitizeRawJSON` lesson — Wolfram returns XML, so:
- Strip control characters from plaintext values (XML-safe but may contain odd chars)
- Do NOT transform double-escaped sequences (same Windows path issue)
- Preserve MathML XML as-is
- Preserve image URLs as-is

---

## 6. MCP Spec Compliance

### 6.1 Tool Definition Requirements

- ✅ `name`: unique, descriptive, snake_case
- ✅ `description`: detailed, includes parameter guidance
- ✅ `inputSchema`: valid JSON Schema (Draft 4+), as `json.RawMessage`
- ❌ `outputSchema`: NOT included (project convention — confuses clients)

### 6.2 Result Format

All tools return `(string, error)` → mapped to `CallToolResult` with `TextContent`:
```go
Content: []mcp.Content{
    &mcp.TextContent{Text: result},
}
```

Errors return `IsError: true`:
```go
Content: []mcp.Content{
    &mcp.TextContent{Text: err.Error()},
},
IsError: true,
```

### 6.3 No Breaking Changes

- New tool package is additive — no modifications to existing tools
- New env var (`WOLFRAM_APPID`) is additive — existing env vars unchanged
- New optional `locolm.json` section is additive — existing configs work unchanged
- Blank import in `main.go` is additive — no other file changes needed

---

## 7. Implementation Order

### Phase 1: Core Infrastructure
1. Create `internal/tool/wolfram/wolfram.go`
   - Config loading (`WOLFRAM_APPID` env var)
   - HTTP client with timeout
   - XML types for Full Results API response
   - Shared utility functions (pod formatting, error handling)

### Phase 2: Primary Tools
2. `wolfram_query` — Full Results API (most capable tool)
3. `wolfram_llm` — LLM API (simplest, most useful for MCP)

### Phase 4: Secondary Tools
4. `wolfram_short` — Short Answers API
5. `wolfram_image` — Simple API
6. `wolfram_recognize` — Fast Query Recognizer API

### Phase 5: Integration
7. Add blank import to `cmd/locolm/main.go`
8. Update `memory.md` with Wolfram documentation
9. Update `prompt.md` with Wolfram tool usage guidance

---

## 8. Testing Plan

### 8.1 Unit Tests

- XML parsing: verify correct pod extraction from sample responses
- Error handling: 403, 429, 501, malformed XML
- Parameter encoding: special characters, Unicode, math expressions

### 8.2 Integration Tests (curl.exe)

```powershell
# Start server
cd E:\Dev\Go\locolm\bin; .\locolm.exe

# Initialize session
curl.exe -X POST http://127.0.0.1:11501/mcp `
  -H "Content-Type: application/json" `
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'

# List tools (verify wolfram tools appear)
curl.exe -X POST http://127.0.0.1:11501/mcp `
  -H "Content-Type: application/json" `
  -H "Mcp-Session-Id: <session-id>" `
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'

# Call wolfram_query
curl.exe -X POST http://127.0.0.1:11501/mcp `
  -H "Content-Type: application/json" `
  -H "Mcp-Session-Id: <session-id>" `
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"wolfram_query","arguments":{"input":"2+2"}}}'

# Call wolfram_llm
curl.exe -X POST http://127.0.0.1:11501/mcp `
  -H "Content-Type: application/json" `
  -H "Mcp-Session-Id: <session-id>" `
  -d '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"wolfram_llm","arguments":{"input":"population of France"}}}'

# Test error handling (missing AppID)
# Test error handling (uninterpretable query)
# Test pod selection (include_pod, exclude_pod)
# Test location override
# Test units parameter
```

### 8.3 Test Cases

| Query | Tool | Expected |
|-------|------|----------|
| `2+2` | wolfram_query | Result: 4 |
| `population of France` | wolfram_llm | ~64 million |
| `integrate x^2 dx` | wolfram_query, format=mathml | MathML output |
| `weather Boston` | wolfram_query, location=Boston, MA | Weather data |
| `10 densest elements` | wolfram_llm | Table of elements |
| `distance Earth Moon` | wolfram_short | ~384,400 km |
| `asdkjhasdfk` | wolfram_query | Error: cannot interpret |
| `pi to 100 digits` | wolfram_query, include_pod=DecimalApproximation | 100 digits of pi |

---

## 9. Security Considerations

- AppID is not secret, but should still be read from env var (not hardcoded)
- URL encoding is critical — use `url.QueryEscape` for all user input
- No user input is executed or evaluated — all sent as data to Wolfram API
- Timeout enforcement prevents slowloris-style resource exhaustion
- CORS already handled at server level (all responses get `*`)

---

## 10. Dependencies

**Zero new dependencies.** All Wolfram tools use only Go stdlib:
- `net/http` — HTTP requests
- `net/url` — URL encoding
- `encoding/xml` — XML parsing (Full Results API)
- `encoding/json` — JSON parsing (LLM API, Short Answers, Recognizer)
- `os` — environment variable access
- `fmt`, `log`, `io`, `strings`, `time` — standard utilities

---

## 11. User Decisions & Rationale

### 11.1 AppID
**User provided:** `7VKAVL337Q` — this is a Full Results API AppID. The LLM API uses the same AppID, so this works for all tools. The user should set `WOLFRAM_APPID=7VKAVL337Q` in their environment.

### 11.2 Default Tool
**Decision:** `wolfram_llm` is the default/recommended tool for most queries. `wolfram_query` is for when the client needs structured pod data, specific formats (MathML, image), or fine-grained pod selection.

**Rationale:** MCP clients (llama-server, Claude Desktop) consume text. The LLM API returns structured text that's immediately useful without XML parsing on the client side. `wolfram_query` is the "advanced" tool for when you need the full power.

### 11.3 Image Handling
**Decision:** Return image URLs as Markdown `![alt](url)`.

**Rationale:** llama-server UI renders Markdown. Plain URLs would be unrendered text. Markdown images are displayed inline. The `alt` text provides accessibility and context.

### 11.4 Assumption Tokens
**Decision:** Yes — `wolfram_query` will include assumption tokens in the response when the query produces ambiguous results. Format:
```
Assumptions:
- "pi" is assumed to be a named mathematical constant
  To change: &assumption=*C.pi-_*Movie-
```

**Rationale:** This enables multi-turn disambiguation. The client can show assumptions to the user and re-query with a different assumption. This is a core Wolfram feature that makes the tool actually useful for ambiguous queries.

### 11.5 Pod Selection Defaults
**Decision:** `wolfram_query` returns **all pods by default**. The client can use `include_pod` / `exclude_pod` to filter.

**Rationale:** The LLM API already provides a "Result" pod equivalent in its output. `wolfram_query` is the full-power tool — returning all pods and letting the client filter is more flexible than defaulting to just the Result pod. The client (LLM) can decide which pods are relevant.
