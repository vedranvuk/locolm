# locolm

Local MCP server exposing tools for web fetch/search, filesystem ops, command execution, memory, and integrations (Wikidata, Wolfram, NewsAPI, etc.) for agents.

## Build & run

```powershell
go build -o bin/locolm.exe ./cmd/locolm/
cd bin; .\locolm.exe
```

MCP server listens on `http://127.0.0.1:11501` (Streamable HTTP, `/mcp`). On Windows, stop the running exe before rebuilding.

## Configuration

`locolm.json` sits next to the executable.

API keys via environment variables (tools needing them fail gracefully if unset):

- `GOOGLE_API_KEY`, `GOOGLE_CSE_ID` — Google Custom Search
- `EXA_API_KEY` — Exa search
- `NEWSAPI_API_KEY` — NewsAPI.org
- `WOLFRAM_APPID` — Wolfram Alpha (tools unregistered if unset)

## Data files

Stored next to the executable (`bin/` when built): `locolm.json` (config), `locolm.db` (memory DB).

## Tools

Listed via the MCP `tools/list` endpoint. Tools needing external services fail gracefully if the service/key is absent.

**Filesystem** (sandboxed to `fs.allowed_paths`)
- `fs_list` — list a directory's entries (name, size, type, mtime).
- `fs_read` — read a file, optionally a line range.
- `fs_write` — create/overwrite a file (max 1 MB).
- `fs_append` / `fs_prepend` — add text to start/end of a file.
- `fs_replace` — replace `old_content` with `new_content` (literal or regex).
- `fs_delete` — delete a single file.
- `fs_move` — move/rename a file (`path` → `new_path`).
- `fs_find` — find files by glob pattern.
- `fs_tree` — render a directory as an indented tree.
- `fs_run` — execute a shell command (`sh -c` / `cmd /C`); allowlisted by config.

**Memory**
- `add_observations` — store facts about an entity.
- `remove_observations` — delete specific facts for an entity.
- `search_memory` — full-text search across stored memories.
- `get_entity_context` — load an entity's full memory profile.
- `remember_semantic` / `recall_semantic` / `forget_semantic` — store/semantic-search/delete text in the local vector DB (needs embedding server).

**Web & search**
- `web_fetch` — fetch a page/PDF and extract readable text (optional Tor proxy).
- `google_search` — Google Custom Search (needs `GOOGLE_API_KEY`, `GOOGLE_CSE_ID`).
- `exa_search` — neural web search via Exa AI (needs `EXA_API_KEY`).
- `news_search` / `news_sources` — NewsAPI.org article search and source listing (needs `NEWSAPI_API_KEY`).
- `wikidata_query` — Wikidata entity lookup, text search, or SPARQL query.

**Computation**
- `wolfram_query` — full pod-level Wolfram Alpha results.
- `wolfram_llm` — LLM-optimized Wolfram output (recommended default).
- `wolfram_short` — single short factual answer.
- `wolfram_image` — rendered image of the result page.
- `wolfram_recognize` — triage whether a query is computable (may 401 on standard keys).
- All wolfram tools need `WOLFRAM_APPID`.

**Go language** (via gopls)
- `gopls_workspace_activate` — set the active workspace (`path` = dir with go.mod).
- `gopls_definition` / `gopls_references` / `gopls_implementation` — jump to declaration / list usages / find implementers.
- `gopls_symbols` — workspace-wide fuzzy symbol search.
- `gopls_diagnostics` — live compiler errors/warnings.
- `gopls_completion` — code completion at a position.
- `gopls_rename` — rename an identifier across the workspace.
- `gopls_format` — gofmt + organize imports.

**System**
- `sys_info` — report OS, CPU, memory, disk, and Go runtime details.

## License

MIT
