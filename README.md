# locolm

AI slopped local MCP with some basic tools for agenticks.

## Quick start

Build and run:

```powershell
Stop-Process -Name "locolm" -Force -ErrorAction SilentlyContinue
go build -o bin/locolm.exe ./cmd/locolm/
cd bin; .\locolm.exe
```

The MCP server listens on http://127.0.0.1:11501 by default. Point your MCP client there.

Note: On Windows you must stop any running `locolm.exe` before rebuilding (cannot overwrite a running exe).

## Configuration

Settings live in `locolm.json` next to the executable (or in the project root when using `go run`). A typical snippet:

```json
{
  "mcp_port": "11501",
  "web_fetch": { "proxy_url": "socks5://localhost:9050" },
  "fs": { "allowed_paths": [".", "~"] }
}
```

Third-party API keys are provided via environment variables:

- `GOOGLE_API_KEY`, `GOOGLE_CSE_ID` (Google Custom Search)
- `EXA_API_KEY` (Exa AI search)
- `NEWSAPI_API_KEY` (NewsAPI.org)
- `WOLFRAM_APPID` (Wolfram Alpha)

## Data files

Files are stored next to the executable (`bin/` when built):

- `locolm.json` — configuration
- `locolm.db` — memory DB

## Tools (high level)

The server exposes a set of tools for web fetch, searches, filesystem operations, command execution (allowlisted), memory, and integrations (Wikidata, Wolfram, NewsAPI, etc.). Use the `/tools/list` endpoint to see the current set.

## License

MIT
