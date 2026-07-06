# locolm

AI slopped local MCP with some basic tools for agenticks.

## Quick start

```bash
go build -o locolm.exe ./cmd/locolm/
./locolm.exe
```

The MCP server listens on `http://127.0.0.1:11501`. Point your MCP client at it.

## Tools

| Tool | Description |
|------|-------------|
| `wikidata_query` | Query Wikidata for structured knowledge (entities, facts, SPARQL) |
| `google_search` | Search the web via Google Custom Search |
| `exa_search` | Search the web via Exa AI (neural search with highlights and synthesis) |
| `web_fetch` | Fetch and read a web page (HTML, PDF, plain text) |
| `sys_info` | Get system information (date, OS, arch, hostname, uptime, etc.) |
| `fs_run` | Execute a command and capture its output (with allowlist security) |
| `fs_list` | List directory contents |
| `fs_read` | Read a text file |
| `fs_write` | Create or overwrite a file |
| `fs_delete` | Delete a single file |
| `fs_find` | Find files by glob pattern |
| `fs_tree` | Display directory tree structure |
| `patch_memory` | Create or update memory facts for an entity |
| `search_memory` | Search historical memory observations |
| `get_entity_context` | Load all facts for a named entity |
| `news_search` | Search news articles using newsapi.org |
| `news_sources` | List available newsapi.org sources |
| `wolfram_query` | Query Wolfram Alpha Full Results API |
| `wolfram_llm` | Query Wolfram Alpha LLM API |
| `wolfram_short` | Query Wolfram Alpha Short Answers API |
| `wolfram_image` | Query Wolfram Alpha Simple API for images |
| `wolfram_recognize` | Query Wolfram Alpha Recognize API |

## Configuration

Configuration is via `locolm.json` in the working directory or exe directory:

```json
{
  "mcp_port": "11501",
  "web_fetch": {
    "max_bytes": 5242880,
    "max_text_bytes": 204800,
    "timeout_sec": 30,
    "proxy_url": "socks5://localhost:9050"
  },
  "fs": {
    "allowed_paths": [".", "~"],
    "read_max_bytes": 1048576,
    "write_max_bytes": 1048576,
    "find_max_results": 200,
    "tree_max_depth": 3
  },
  "exec": {
    "allowed_commands": ["^git\\s", "^go\\s", "^python\\s", "^node\\s", "^npm\\s"],
    "timeout_sec": 30,
    "max_output_bytes": 102400
  },
  "wikidata": {
    "endpoint": "https://www.wikidata.org/w/api.php",
    "sparql_endpoint": "https://query.wikidata.org/sparql",
    "user_agent": "locolm/1.0 (https://github.com/vedranvuk/locolm)",
    "timeout_sec": 30,
    "max_entities_per_request": 50
  }
}
```

Third-party API keys are set via environment variables:

| Variable | Description |
|----------|-------------|
| `GOOGLE_API_KEY` | Google Search API key |
| `GOOGLE_CSE_ID` | Google Custom Search Engine ID |
| `EXA_API_KEY` | Exa AI search API key |
| `NEWSAPI_API_KEY` | NewsAPI.org API key |
| `WOLFRAM_APPID` | Wolfram Alpha AppID |

## Proxy configuration

The `web_fetch` tool supports a SOCKS5 proxy via the `proxy_url` setting. This is useful for routing traffic through Tor or other proxy services.

- If `proxy_url` is set, all web_fetch requests go through the proxy
- If `proxy_url` is empty or omitted, web_fetch connects directly
- Default: `socks5://localhost:9050` (Tor default)

## Data files

locolm stores its data next to the executable:

| File | Purpose |
|------|---------|
| `locolm.json` | Configuration |
| `locolm.db` | SQLite memory database |

Both must be in the same directory as `locolm.exe`. When running via `go run ./cmd/locolm/`, they must be in the project root (the working directory).



## License

MIT
