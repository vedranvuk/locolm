# locolm

Local MCP server exposing tools for web fetch/search, filesystem ops, command execution, memory, and integrations (Wikidata, Wolfram, NewsAPI, etc.) for agents.

## Build & run

```powershell
Stop-Process -Name "locolm" -Force -ErrorAction SilentlyContinue
go build -o bin/locolm.exe ./cmd/locolm/
cd bin; .\locolm.exe
```

MCP server listens on `http://127.0.0.1:11501` (Streamable HTTP, `/mcp`). On Windows, stop the running exe before rebuilding.

## Configuration

`locolm.json` sits next to the executable (or project root with `go run`):

```json
{
  "mcp_port": "11501",
  "web_fetch": { "proxy_url": "socks5://localhost:9050" },
  "fs": { "allowed_paths": [".", "~"] }
}
```

API keys via environment variables (tools needing them fail gracefully if unset):

- `GOOGLE_API_KEY`, `GOOGLE_CSE_ID` — Google Custom Search
- `EXA_API_KEY` — Exa search
- `NEWSAPI_API_KEY` — NewsAPI.org
- `WOLFRAM_APPID` — Wolfram Alpha (tools unregistered if unset)

## Data files

Stored next to the executable (`bin/` when built): `locolm.json` (config), `locolm.db` (memory DB).

## Tools

List current tools via the MCP `tools/list` endpoint.

## License

MIT
