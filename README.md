# locolm

A vibe-coded local AI platform that gets anyone up and running with a local LLM in seconds. No fiddling with config files, no manual setup — just run one executable and everything bootstraps for you.

## What it does

locolm ties together everything you need for a local AI experience:

- **llama-server** — automatically starts the LLM engine with your chosen model
- **Chrome app** — opens llama-server as a standalone windowed app in your browser
- **MCP server** — gives the LLM access to tools so it can search the web, read pages, and remember things across conversations
- **Memory** — SQLite-backed persistent memory so the LLM recalls context between sessions

## Quick start

```bash
go build -o locolm.exe ./cmd/locolm/
./locolm.exe
```

That's it. locolm will:

1. Start the MCP server on port 11501
2. Start llama-server with your configured model
3. Wait for llama-server to become ready
4. Open Chrome as a standalone app pointed at llama-server

Point your MCP client (or llama-server itself) at `http://127.0.0.1:11501`.

## Tools

| Tool | Description |
|------|-------------|
| `google_search` | Search the web via Google Custom Search |
| `exa_search` | Search the web via Exa AI (neural search with highlights and synthesis) |
| `web_fetch` | Fetch and read a web page (HTML, PDF, plain text) |\| `sys_info` | Get system information (date, OS, arch, hostname, uptime, etc.) |
| `fs_run` | Execute a command and capture its output |
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
  "llama_server_command": "llama-server -m model.gguf",
  "browser_command": "chrome.exe",
  "web_fetch_max_bytes": 5242880,
  "web_fetch_max_text_bytes": 204800,
  "web_fetch_timeout_seconds": 30
}
```

Third-party API keys are set via environment variables:

| Variable | Description |
|----------|-------------|
| `GOOGLE_API_KEY` | Google Search API key |
| `GOOGLE_CSE_ID` | Google Custom Search Engine ID |
| `EXA_API_KEY` | Exa AI search API key |

## System prompt

See `prompt.md` for the system prompt to use with your LLM. It teaches the model when and how to use each tool.

## Project status

This is a work in progress. Some planned features are not yet implemented:

- Automatic download of llama-cpp binaries
- Model downloading
- First-run setup wizard

## License

MIT
