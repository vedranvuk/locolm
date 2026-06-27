# MCP Server Facts

## Build & Run
- Build: `go build -o bin/locolm.exe ./cmd/locolm/`
- Run: `cd bin; .\locolm.exe`
- MCP port: 11501
- llama-server port: 11500

## MCP Handler Details
- Protocol version negotiation: echoes client's version in initialize response
- Content-Length header: explicitly set on all responses (fixes chunked encoding issues)
- JSON passthrough: tool results that are valid JSON returned as json.RawMessage (not double-escaped)
- Tool errors: returned as {"isError": true, "text": "..."} per MCP spec
- Notifications: HTTP 204 No Content

## Testing MCP
- Always use curl.exe (not curl — PowerShell alias mangles JSON)
- Write JSON body to temp file, use `curl.exe -d @<filename>`
- Python at C:\Python312\python.exe

## Key Bug Fixes (2026-06-27)
1. wikidata_query entity mode: missing User-Agent header → HTTP 403. Fixed with http.NewRequest + header.
2. MCP handler: JSON tool results were double-escaped as strings. Fixed with json.RawMessage passthrough.
3. MCP handler: chunked encoding confused some clients. Fixed with explicit Content-Length header.
4. MCP handler: protocol version mismatch. Fixed by negotiating with client.
