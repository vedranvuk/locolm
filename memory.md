# locolm — Project Memory

## Architecture
- `cmd/locolm` — CLI entry point.
- `internal/client` — HTTP client for the llama-server API (see below).
- `internal/server` — local server wiring.
- `internal/database` — SQLite + vector storage.
- `internal/mcp` — MCP tool registry/handler.
- `internal/tool/*` — individual tools (rag, wolfram, fs, web, exasearch, gsearch, wikidata, gopls, exec, sysinfo, memory, subagent).

## llama-server client (`internal/client`)
- Full client for the llama-server HTTP API. Each endpoint is its own file:
  `client.go` (core + Client + HealthCheck/Models), `chat.go`, `completion.go`,
  `embedding.go`, `server.go` (props/slots/metrics/lora/control), `options.go` (docs).
- **Uniform calling convention:** required inputs are positional; all optional
  inputs use typed functional options `With<Param>(...)` applied to a per-request
  struct. Client config uses `WithHTTPClient`, `WithTimeout`, `WithAPIKey`, `WithHeader`.
- Streaming methods (`ChatStream`, `CompletionStream`) return a typed decoder with
  `Recv() (*XxxResponse, error)` (io.EOF at end) and `Close() error` — no raw SSE parsing.
- `New(baseURL, opts...)` — baseURL may omit trailing slash.
- `rag.go` uses `client.Embedding(ctx, text, WithEmbeddingModel(...), WithEmbeddingPooling("mean"))`.

## Conventions
- Keep option helpers type-safe (operate on concrete `*XxxRequest`), not `any`.
- Errors from non-2xx responses are returned as `*client.Error{Status, Body}`.

## MCP configuration
- Workspace MCP config is stored in [.vscode/mcp.json](.vscode/mcp.json).
- The gopls MCP server is configured to use the existing TCP/SSE listener at http://127.0.0.1:8092/mcp.

## Running & testing over MCP (curl)
- Server listens on `0.0.0.0:11501`, MCP endpoint `/mcp` (Streamable HTTP transport).
- Session flow: POST `initialize` (capture `Mcp-Session-Id` response header) →
  POST `notifications/initialized` (no id) → then `tools/list` / `tools/call`.
  All requests need `Accept: application/json, text/event-stream`; responses are
  SSE (`event: message` / `data: <json>`). Parse the `data:` line.
- Default embedding endpoint (rag tool) is `http://192.168.1.100:11502`
  (OpenAI-compatible `/v1/embeddings`). Was `127.0.0.1:11502` before 2026-07-10.
- Tools needing external services (fail gracefully if absent):
  - `remember_semantic`/`recall_semantic`/`forget_semantic` → embedding server.
  - `exa_search` → `EXA_API_KEY` env var.
  - `news_search`/`news_sources` → `NEWSAPI_API_KEY` env var (newsapi.org 401 if unset).
  - `wolfram_*` → `WOLFRAM_APPID` env var (tools NOT registered if unset).
  - `google_search`/`web_fetch`/`wikidata_query` work without keys (web access).
- `fs_run` executes via `sh -c` on Linux / `cmd /C` on Windows (fixed 2026-07-10).
- Tool arg-name gotchas (differ from obvious names):
  - `fs_replace` uses `old_content`/`new_content` (not `old`/`new`).
  - `fs_move` uses `path`/`new_path` (not `source`/`destination`).
  - `gopls_workspace_activate` uses `path` (not `root`).
  - `news_search` uses `q` (not `query`); `headlines` needs country/category/sources/q.
  - `wikidata_query` entity mode uses `query` (the Q-ID), not `id`.

## Known bugs fixed (2026-07-10)
- `rag.go`: `vec0` virtual table rejected `?` param binding in column def
  (`embedding float[?]`). Fixed by `fmt.Sprintf` with `config.EmbeddingDimensions`.
- `exec.go`: hardcoded `cmd /C` (Windows-only) → made cross-platform (`sh -c` on non-Windows).
- `wolfram.go`: tools only registered if `config.AppID` set at startup (from
  locolm.json), ignoring `WOLFRAM_APPID` env var. Refactored to read
  `WOLFRAM_APPID` via `os.Getenv()` at call time (like exa/google/newsapi), and
  register the 5 wolfram tools unconditionally. Added `os` import.
- `wolfram/full_results.go`: `QueryResult.NumPods` was `int` but Wolfram returns
  `numpods="2210.001"` (float) → XML unmarshal failed with
  `strconv.ParseInt: parsing "2210.001"`. Changed field to `float64`, format `%.0f`.

## Tool arg-name gotchas (cont.)
- `wolfram_*` tools use `input` (not `query`) as the required argument.
- `wolfram_recognize` returns 401 "Not permitted" with a standard AppID — the
  Fast Query Recognizer API requires a higher permission tier (external limit,
  not a code bug). Other wolfram tools (short/llm/query/image) work fine.
- `wolfram_image` embeds the raw AppID in the returned image URL — minor info
  leak; consider stripping/redacting the `appid` query param from the URL.

## Prompt & description cleanup (2026-07-10)
- Rewrote `internal/mcp/instructions.txt` (the Dave system prompt): removed
  redundancy, grouped by tool family, fixed the `wolfram_recognize` guidance
  (it can 401 on standard keys), tightened all wording to be clear/concise.
- Tightened all 39 tool `description` strings across `sysinfo`, `fs`, `exec`,
  `memory`, `rag`, `google`, `exa`, `fetch`, `newsapi`, `wikidata`, `gopls`,
  and `wolfram` packages — now consistent, instructive, and concise. No schema
  (arg names/required) changes, only description text.
