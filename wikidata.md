# Wikidata Query Tool — Design Document

## Overview

A new MCP tool `wikidata_query` that provides structured access to Wikidata (the knowledge base behind Wikipedia). Unlike `web_fetch` (which parses HTML prose) or `google_search` (which returns web pages), this tool returns **structured data** — entities, properties, relationships, and facts — directly from the source.

## Research Summary

### Wikidata API Landscape

Wikidata offers multiple access methods. Our tool uses two:

| Method | Endpoint | Use case |
|--------|----------|----------|
| **MediaWiki Action API** (`wbgetentities`, `wbsearchentities`) | `https://www.wikidata.org/w/api.php` | Entity lookup by Q-ID, text search for entities |
| **SPARQL Endpoint** (WDQS) | `https://query.wikidata.org/sparql` | Complex queries across the full knowledge graph |

### Key API Details

#### MediaWiki Action API
- **Max 50 entities per request** (500 for bots/admins) — we batch automatically
- **Rate limit**: ~60 requests/minute for anonymous users; 30 error queries/minute
- **User-Agent policy**: Required. Must be descriptive or risk being blocked.
- **Format**: Always `format=json&formatversion=2`
- **Language**: `uselang` parameter controls label/description language
- **No API key required**

#### SPARQL Endpoint (WDQS)
- **Hard timeout**: 60 seconds per query
- **Processing time limit**: 60 seconds per 60 seconds per client (user-agent + IP)
- **Parallel queries**: 5 per IP
- **Output formats**: JSON (recommended), XML, CSV, TSV
- **JSON via**: `format=json` query param OR `Accept: application/sparql-results+json` header
- **GET for small queries, POST for large** (POST body must include `query=` prefix, URL-encoded)
- **User-Agent policy**: Same as Action API — required
- **No API key required**

### Wikidata Entity Model (for the LLM)

Entities have:
- **Q-ID** (e.g., Q42 = Douglas Adams) — unique identifier
- **Label** — human-readable name (language-dependent)
- **Description** — short summary
- **Aliases** — alternative names
- **Claims/Statements** — property-value pairs (the core data)
- **Sitelinks** — links to Wikipedia and other Wikimedia projects

Properties have:
- **P-ID** (e.g., P31 = instance of, P279 = subclass of, P569 = date of birth)
- **Label**, **description**, **datatype** (WikibaseItem, string, quantity, time, etc.)

### Common SPARQL Patterns

**Prefixes** (most are auto-resolved by WDQS, but explicit is safer):
```
wd:    = entity (Q-number)
wdt:   = direct/truthy property
p:     = statement node
ps:    = statement value
pq:    = qualifier
wikibase: = label service
```

**Label service** (essential for getting human-readable names):
```sparql
SERVICE wikibase:label {
  bd:serviceParam wikibase:language "en" .
}
```
Auto-generates `?itemLabel`, `?itemDescription`, `?itemAltLabel` from bound variables.

**Truthy vs all statements**: `wdt:P31` returns only the "best" (truthy) value for P31. `p:P31/ps:P31` returns all values including deprecated.

**Property chain**: `wdt:P31/wdt:P279*` = instance of, or subclass of (transitive).

**VALUES for batch**: `VALUES ?q { wd:Q5 wd:Q12345 }` — efficient batch lookup.

### Key Entity Q-IDs (for the LLM to know)

Common classes:
- Q5 = human
- Q515 = city
- Q6256 = country
- Q3624078 = sovereign state
- Q13442814 = scholarly article
- Q191067 = event
- Q35666 = glacier
- Q4022 = river
- Q8502 = mountain
- Q16521 = taxon
- Q11173 = chemical compound
- Q7397 = software
- Q571 = book
- Q11424 = film
- Q188914 = programming language

Common properties:
- P31 = instance of
- P279 = subclass of
- P17 = country
- P131 = located in administrative entity
- P19 = place of birth
- P20 = place of death
- P21 = sex or gender
- P22 = father
- P25 = mother
- P26 = spouse
- P40 = child
- P69 = educated at
- P106 = occupation
- P1082 = population
- P159 = headquarters location
- P2043 = length
- P2044 = elevation
- P2046 = area
- P2067 = mass
- P569 = date of birth
- P570 = date of death
- P571 = inception
- P576 = dissolved/abolished
- P577 = publication date
- P580 = start time
- P582 = end time
- P585 = point in time
- P625 = coordinate location
- P856 = official website
- P921 = main subject

## Tool Design

### Name
`wikidata_query`

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `mode` | Yes | `entity`, `search`, or `sparql` |
| `query` | Yes | Q-ID (entity mode), search text (search mode), or SPARQL query (sparql mode) |
| `lang` | No | Language code for labels/descriptions (default `en`) |
| `limit` | No | Max results for search mode (default 10, max 50) |

### Mode: `entity`

Fetches one or more entities by Q-ID.

**Implementation**:
- Calls `action=wbgetentities` with `ids=Q42|Q60|...` (pipe-separated, max 50)
- Returns a condensed JSON per entity: label, description, aliases, key claims (P31, P279, P17, P19, P20, P21, P22, P25, P26, P40, P569, P570, P571, P576, P580, P582, P585, P625, P856, P921, P1082, P106, P69, P131, P159), and sitelinks count
- Claims are filtered to a curated list of ~25 commonly useful properties to avoid overwhelming the LLM
- For each claim, the value is resolved: WikibaseItem values get their label, time values get formatted, quantity values get amount+unit

**Example**:
```json
{"mode": "entity", "query": "Q42", "lang": "en"}
```
Returns Douglas Adams: writer, author of Hitchhiker's Guide, born 1952, died 2001, educated at St John's College Cambridge, etc.

### Mode: `search`

Searches for entities by label/alias.

**Implementation**:
- Calls `action=wbsearchentities` with `search=<query>&limit=<limit>`
- Returns Q-ID + label + description + aliases for each match
- Uses `uselang` for language

**Example**:
```json
{"mode": "search", "query": "Ada Lovelace", "limit": 5}
```

### Mode: `sparql`

Runs a SPARQL query against the full Wikidata knowledge graph.

**Implementation**:
- Submits the query to `https://query.wikidata.org/sparql` via POST (to handle large queries)
- Sets `Accept: application/sparql-results+json` header
- Sets `User-Agent` per Wikidata policy
- Returns the raw SPARQL JSON results (head + results.bindings)
- The LLM writes the SPARQL — this is the most powerful mode

**Example**:
```json
{"mode": "sparql", "query": "SELECT ?item ?itemLabel WHERE { ?item wdt:P31 wd:Q5 . ?item wdt:P27 wd:Q30 . ?item wdt:P106 wd:Q36180 . SERVICE wikibase:label { bd:serviceParam wikibase:language \"en\" . } } LIMIT 10"}
```
Returns 10 American writers.

### Config

Registered under `"wikidata"` key in `locolm.json`:

```json
{
  "wikidata": {
    "endpoint": "https://www.wikidata.org/w/api.php",
    "sparql_endpoint": "https://query.wikidata.org/sparql",
    "user_agent": "locolm/1.0 (https://github.com/vedranvuk/locolm)",
    "timeout_sec": 30,
    "max_entities_per_request": 50
  }
}
```

Defaults are always set if the key is missing. The `user_agent` has a sensible default.

### Files to Create/Modify

| File | Action |
|------|--------|
| `internal/tool/wikidata/wikidata.go` | **Create** — tool implementation |
| `cmd/locolm/main.go` | **Modify** — add blank import |
| `internal/config/config.go` | **Modify** — add `Wikidata json.RawMessage` field + dispatch |
| `memory.md` | **Modify** — document the new tool |
| `README.md` | **Modify** — add to tools table |
| `prompt.md` | **Modify** — add usage guidance for the LLM |

### Implementation Notes

1. **User-Agent is mandatory** — Wikidata policy requires it. Default: `locolm/1.0 (https://github.com/vedranvuk/locolm)`. Config can override.

2. **Entity mode response format** — curated, not raw. The raw `wbgetentities` response is enormous (all statements, qualifiers, references). We filter to ~25 common properties and resolve item values to labels. This keeps the response LLM-friendly.

3. **SPARQL mode returns raw JSON** — the LLM is capable of writing SPARQL and interpreting results. We return the standard SPARQL 1.1 JSON results format (head.vars + results.bindings). The LLM can handle this.

4. **Error handling**:
   - 429 Too Many Requests → return error with "rate limited, try again later"
   - Query timeout (SPARQL) → return "query timed out, try a more specific query"
   - Invalid Q-ID → return "entity not found"
   - Malformed SPARQL → return the error message from WDQS

5. **No external dependencies** — pure stdlib: `net/http`, `encoding/json`, `fmt`, `time`, `net/url`, `strings`, `io`. Consistent with project philosophy.

6. **No API key** — Wikidata is completely free. No env vars needed.

7. **Self-registering** — follows the established `init()` + blank-import pattern.

8. **GET vs POST for SPARQL** — always use POST to avoid URL length limits. The POST body is `query=<URL-encoded SPARQL>`.

9. **Language fallback** — the `lang` parameter is passed as `uselang` to the Action API and as `wikibase:language` in SPARQL label services. If no label exists in the requested language, Wikidata falls back to Q-ID.

### Security Considerations

- **No SSRF risk** — we only connect to hardcoded Wikidata endpoints (configurable but default to official)
- **No data exfiltration risk** — read-only, only queries the public knowledge base
- **Rate limiting** — Wikidata enforces its own limits. We don't need to implement client-side rate limiting for a tool that the LLM calls occasionally.

### Why This Fits locolm

- **Structured data** — unlike `web_fetch` (HTML prose) or `google_search` (web pages), Wikidata gives clean, structured facts
- **Complements existing tools** — `web_fetch` for prose, `google_search` for web search, `wikidata_query` for structured knowledge
- **Zero dependencies** — pure stdlib, like the rest of the codebase
- **No API key** — unlike Google/Exa, Wikidata is free. User gets a powerful knowledge tool out of the box.
- **Self-registering** — follows the established pattern exactly.
