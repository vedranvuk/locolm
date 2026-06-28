# locolm

A local MCP (Model Context Protocol) server that gives your LLM access to tools for web search, web fetching, filesystem operations, persistent memory, and more. locolm runs as a standalone MCP server — connect any MCP client to it.

## What it does

locolm provides an MCP server that exposes tools for:

- **Web & Research** — web search (Google, Exa), web fetching, Wikidata queries
- **Filesystem** — sandboxed file read/write/delete/find so the LLM can work with your local files
- **Command execution** — run CLI commands with allowlist security
- **Memory** — SQLite-backed persistent memory so the LLM recalls context across conversations
- **System info** — date, OS, arch, hostname, uptime

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
| `memory_save` | Create or update a memory in a bucket |
| `memory_edit` | Update an existing memory (fails if not found) |
| `memory_delete` | Delete a specific memory |
| `memory_load` | Load a single memory's value |
| `memory_list` | List memories (all or by bucket) |
| `memory_delete_bucket` | Delete a bucket and all its memories |
| `memory_list_buckets` | List all buckets with counts |

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

## System prompt

See `prompt.md` for the system prompt to use with your LLM. It teaches the model when and how to use each tool.

## Project status

This is a work in progress. Some planned features are not yet implemented:

- Automatic download of llama-cpp binaries
- Model downloading
- First-run setup wizard

## License

MIT
