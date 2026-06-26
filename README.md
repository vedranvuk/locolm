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
go build -o locolm.exe .
./locolm.exe
```

That's it. locolm will:

1. Start llama-server with the default model
2. Wait for the model to load
3. Open Chrome as a standalone app pointed at llama-server
4. Start the MCP server on port 11501

Point your MCP client (or llama-server itself) at `http://127.0.0.1:11501`.

## Tools

| Tool | Description |
|------|-------------|
| `google_search` | Search the web via Google Custom Search |
| `web_fetch` | Fetch and read a web page |
| `memory_save` | Save a memory entry |
| `memory_load` | Load all memories |
| `memory_forget` | Delete a memory by key |
| `memory_list` | List all memory keys |

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `LOCOLM_MCP_PORT` | `11501` | MCP server port |
| `LOCOLM_BOOTSTRAP_LLAMA_SERVER_COMMAND` | (see source) | Full llama-server command |
| `LOCOLM_BOOTSTRAP_BROWSER_COMMAND` | Chrome path | Browser executable |
| `GOOGLE_API_KEY` | — | Google Search API key |
| `GOOGLE_CSE_ID` | — | Google Custom Search Engine ID |

## System prompt

See `prompt.md` for the system prompt to use with your LLM. It teaches the model when and how to use each tool.

## Project status

This is a work in progress. Some planned features are not yet implemented:

- Configuration file (`locolm.json`)
- Automatic download of llama-cpp binaries
- Model downloading
- First-run setup wizard

## License

MIT
