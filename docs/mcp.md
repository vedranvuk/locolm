# Communicating over MCP Protocol using curl

The Model Context Protocol (MCP) uses JSON-RPC 2.0 as its communication backbone. This allows for structured interaction between a client and a server through standard HTTP methods or stdio streams.

## 1. Core Communication Basics
Every message in the MCP protocol follows the JSON-RPC 2.0 format:
```json
{
  "jsonrpc": "2.0",
  "id": <integer|string>,
  "method": "<method-name>",
  "params": { ... }
}
```

## 2. Initializing a Session
To start an MCP session, you must first perform an `initialize` handshake to exchange capabilities between the client and server.

**Example curl command:**
```bash
curl -X POST http://127.0.0.1:8000/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {
        "tools": {},
        "resources": {},
        "prompts": {}
      },
      "clientInfo": {
        "name": "curl-client",
        "version": "1.0.0"
      }
    }
  }'
```

## 3. Operational Commands

### List Available Tools
To see what tools the server offers:
```bash
curl -X POST http://127.0.0.1:8000/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list"
  }'
```

### Call a Tool
To execute a specific tool with arguments:
```bash
curl -X POST http://127.0.0.1:8000/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "add",
      "arguments": {
        "a": 5,
        "b": 3
      }
    }
  }'
```

### List Resources
To discover data resources:
```bash
curl -X POST http://127.0.0.1:8000/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 4,
    "method": "resources/list"
  }'
```

## 4. Advanced Transport: SSE (Server-Sent Events)
For real-time streaming or complex sessions, MCP often uses the **SSE transport**. In this mode, you can keep a connection open and send messages to it.

**Example for calling a tool over an SSE endpoint:**
```bash
curl -X POST http://127.0.0.1:8000/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -d '{
    "jsonrpc": "2.0",
    "id": 5,
    "method": "tools/call",
    "params": {
      "name": "get_weather",
      "arguments": {
        "location": "London"
      }
    }
  }'
```

## Debugging Tips
- **Use `jq`**: Pipe your curl results into `jq` to make the JSON output readable.
  Example: `curl ... | jq .`
- **Session IDs**: While some servers are stateless, others require a consistent `id` in the JSON-RPC object to track asynchronous responses.
- **Headers**: Many MCP servers support an `Mcp-Session-Id` header or standard `Authorization` headers for multi-session tracking.
