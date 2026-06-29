# locolm — Copilot Instructions

## Project Overview

locolm is an MCP (Model Context Protocol) server written in Go. It provides tools for web search, web fetching, filesystem operations, persistent memory, command execution, Wikidata queries, and system info. Any MCP client can connect to it.

- **Language:** Go 1.26.1
- **MCP SDK:** `github.com/modelcontextprotocol/go-sdk` v1.6.1
- **Server name:** `locolm`, version `1.0.0`
- **Listen address:** `http://127.0.0.1:11501` (configurable via `locolm.json`)

## Session Startup

At the start of every conversation, read `memory.md` at the project root. This file is the single source of truth for all project knowledge — architecture, conventions, bug fixes, lessons learned, and operational procedures. Read it before answering any questions or making any changes.

## Memory File Maintenance

`memory.md` at the project root is the **only** project memory/documentation file. It is a living document that must be kept up to date.

**When to update `memory.md`:**
- New architectural decisions or patterns are established
- New dependencies are added or existing ones change
- Bug fixes or workarounds are discovered (especially non-obvious ones)
- New conventions or rules are established
- File structure changes significantly
- New tools or features are added
- Operational procedures change (build, test, deploy)

**What belongs in `memory.md`:**
- Architecture and design decisions
- Conventions and rules
- Bug fixes and lessons learned
- Operational procedures (building, testing, running)
- Dependencies and their purposes
- File structure explanations

**What does NOT belong in `memory.md`:**
- Temporary task tracking (use the todo tool for that)
- Code-level documentation (use code comments)
- System prompt content (that belongs in `prompt.md`)

**Format:** Keep entries concise. Use bullet points. Prefer facts over prose.

## Build & Run

```powershell
# ALWAYS kill running instances FIRST — Windows cannot overwrite a running binary
Stop-Process -Name "locolm" -Force -ErrorAction SilentlyContinue
# Then build
go build -o E:\Dev\Go\locolm\bin\locolm.exe ./cmd/locolm/
# Then run (config files must be in the working directory)
cd E:\Dev\Go\locolm\bin; .\locolm.exe
```

- Windows CANNOT overwrite a running executable — always `Stop-Process` before rebuild
- `locolm.json` and `locolm.db` must be in the executable's directory (or project root for `go run`)
- All Go source lives under `cmd/` and `internal/` — no `.go` files at the module root

## Testing

- **Always use `curl.exe`** for testing on Windows (PowerShell's `curl` alias mangles JSON)
- The MCP server speaks JSON-RPC 2.0 over Streamable HTTP
- Test `tools/list` first to confirm server is running, then `tools/call` for specific tools
- Include `Mcp-Session-Id` header in all requests after initialize
- **ALWAYS stop the test server when done** — `Stop-Process -Name "locolm" -Force`

## Key Conventions

- Keep dependencies minimal; prefer stdlib (except go-sdk for MCP protocol)
- ONLY three env vars: `GOOGLE_API_KEY`, `GOOGLE_CSE_ID`, `EXA_API_KEY` — never introduce `LOCOLM_*` env vars
- Third-party API keys are read directly via `os.Getenv()` in their respective tool files
- Tool registration: each package has an `init()` calling `mcp.RegisterTool()` — blank imports in `main.go` trigger them
- ToolFunc signature: `func(args map[string]string) (string, error)`
- CORS: `Access-Control-Allow-Origin: *` on all responses
- MCP output: all results as TextContent (JSON-returning tools return JSON as string)
- `outputSchema` is NOT included on MCP tool definitions (confuses some clients)

## MCP Server Details

- Protocol version: negotiates with client (prefers `2025-03-26`)
- Content-Length header explicitly set on all responses (fixes chunked encoding issues)
- `sanitizeRawJSON` handles literal control chars in JSON strings — does NOT convert double-escaped sequences (`\\n` → `\n`) to avoid corrupting Windows paths
- SPARQL-specific normalization stays in `normalizeSPARQL` where semantic context exists

## System Prompt

`prompt.md` is the canonical source for the LLM system prompt. When tools or instructions change, update `prompt.md`. The assistant name is **Dave**.

## Architecture

```
cmd/locolm/main.go          # Entry point: blank imports → init() → config → mcp.New() → serve
internal/config/config.go    # Config struct + LoadConfig (reads locolm.json + GOOGLE_* env)
internal/mcp/server.go      # MCP server wrapper (RegisterTool, ServeHTTP with CORS, sanitizeRawJSON)
internal/tool/memory/       # 8 memory_* tools (SQLite + FTS5)
internal/tool/fs/           # fs_list, fs_read, fs_write, fs_delete, fs_find, fs_tree
internal/tool/exec/         # fs_run (command execution with regex allowlist)
internal/tool/web/          # web_fetch (HTML content extraction)
internal/tool/search/       # google_search, exa_search
internal/tool/sysinfo/      # sys_info
internal/tool/wikidata/     # wikidata_query (entity, search, sparql modes)
```

## Dependencies

- `github.com/modelcontextprotocol/go-sdk` v1.6.1 — MCP protocol
- `codeberg.org/readeck/go-readability/v2` — HTML content extraction
- `modernc.org/sqlite` — Pure-Go SQLite (no CGO) with FTS5 for memory storage
- `github.com/ledongthuc/pdf` — PDF text extraction
