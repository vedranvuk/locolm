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
