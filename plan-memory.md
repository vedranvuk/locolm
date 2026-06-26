# Memory System Reimplementation Plan

## Overview

Replace the current flat key-value memory system with a searchable, categorized knowledge base that the LLM queries on-demand rather than loading entirely into context. The goal: the LLM should only load relevant memories, keeping context windows small and responses focused.

## Design Principles

1. **SQLite only** — no new storage dependencies. modernc.org/sqlite already provides FTS5 support.
2. **Simple structures** — tabular with JSON columns for arrays. No graph database, no vector DB.
3. **LLM-driven retrieval** — the LLM decides when to search, what keywords to use, and which memories are relevant.
4. **Two-tier loading** — automatic high-priority recall at conversation start, plus on-demand keyword search during conversation.
5. **Minimal tool surface** — four tools: `memory_recall`, `memory_search`, `memory_save`, `memory_forget`.

---

## Database Schema

### migrations

Run on startup. Use `user_version` pragma to track schema version.

```sql
PRAGMA user_version = 1;
```

### memories table

```sql
CREATE TABLE IF NOT EXISTS memories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT UNIQUE NOT NULL,
    value TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'general',
    priority INTEGER NOT NULL DEFAULT 5 CHECK(priority >= 1 AND priority <= 9),
    keywords TEXT NOT NULL DEFAULT '[]',
    source TEXT DEFAULT 'conversation',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

| Column | Purpose |
|---|---|
| `id` | Auto-increment primary key |
| `key` | Unique identifier (e.g. `user_prefers_go`) |
| `value` | The actual memory content |
| `category` | Grouping: `user`, `project`, `preference`, `fact`, `decision`, `context` |
| `priority` | 1=critical (always recall), 5=normal, 9=niche (search only) |
| `keywords` | JSON array of searchable terms, e.g. `["go", "language", "preference"]` |
| `source` | How it was created: `conversation`, `tool`, `system` |
| `created_at` / `updated_at` | Timestamps for ordering and staleness detection |

### FTS5 virtual table

```sql
CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
    key,
    value,
    keywords,
    content='memories',
    content_rowid='id'
);
```

This enables full-text search across key, value, and keywords columns. Queries use `MATCH` operator.

### Indexes

```sql
CREATE INDEX IF NOT EXISTS idx_memories_category ON memories(category);
CREATE INDEX IF NOT EXISTS idx_memories_priority ON memories(priority);
CREATE INDEX IF NOT EXISTS idx_memories_updated ON memories(updated_at DESC);
```

### Triggers to keep FTS in sync

```sql
CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
    INSERT INTO memories_fts(rowid, key, value, keywords)
    VALUES (new.id, new.key, new.value, new.keywords);
END;

CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
    INSERT INTO memories_fts(memories_fts, rowid, key, value, keywords)
    VALUES ('delete', old.id, old.key, old.value, old.keywords);
END;

CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
    INSERT INTO memories_fts(memories_fts, rowid, key, value, keywords)
    VALUES ('delete', old.id, old.key, old.value, old.keywords);
    INSERT INTO memories_fts(rowid, key, value, keywords)
    VALUES (new.id, new.key, new.value, new.keywords);
END;
```

---

## Tool Definitions

### memory_recall

Called automatically at conversation start. Loads high-priority memories that should always be in context.

**Input:**
```json
{
  "limit": 10
}
```

**Behavior:**
- Query: `SELECT key, value, category, priority FROM memories WHERE priority <= 3 ORDER BY priority ASC, updated_at DESC LIMIT ?`
- Returns compact text block for injection into LLM context
- The `limit` parameter is optional, default 10

**Response format:**
```
High-priority memories:

[prefers_go] User prefers Go for systems programming (category: preference, priority: 1)
[project_locolm] locolm is an MCP server for local LLM interaction (category: project, priority: 2)
```

### memory_search

Called on-demand during conversation when the LLM needs to check if it knows something relevant.

**Input:**
```json
{
  "query": "MCP transport",
  "category": "project",
  "limit": 5
}
```

| Parameter | Required | Default | Description |
|---|---|---|---|
| `query` | yes | — | Keywords to search for |
| `category` | no | all | Filter by category |
| `limit` | no | 5 | Max results |

**Behavior:**
- Uses FTS5: `SELECT m.key, m.value, m.category, m.priority FROM memories_fts f JOIN memories m ON m.id = f.rowid WHERE memories_fts MATCH ? ORDER BY rank LIMIT ?`
- If `category` is provided, add `AND m.category = ?`
- Keywords are extracted from `query` string (split on spaces, each term becomes a MATCH term)
- Results ordered by FTS5 rank (relevance), then by priority

**Response format:**
```
Found 3 memories matching "MCP transport":

[project_mcp_version] MCP protocol version is 2024-11-05 (category: project, priority: 2)
[project_transport] User prefers stdio transport for local tools (category: preference, priority: 1)
[mcp_config] MCP server listens on 127.0.0.1:11501 (category: project, priority: 3)
```

### memory_save

Saves or updates a memory entry.

**Input:**
```json
{
  "key": "user_prefers_tabs",
  "value": "User prefers tabs over spaces for indentation",
  "category": "preference",
  "priority": 2,
  "keywords": ["tabs", "spaces", "indentation", "formatting"]
}
```

| Parameter | Required | Default |
|---|---|---|
| `key` | yes | — |
| `value` | yes | — |
| `category` | no | `general` |
| `priority` | no | 5 |
| `keywords` | no | auto-extracted from key + value |

**Behavior:**
- If `keywords` not provided: split `key` on `_` and combine with first 20 words of `value`, deduplicate, store as JSON array
- Upsert on `key` conflict (same as current behavior)
- Updates `updated_at` timestamp
- FTS triggers handle index updates automatically

### memory_forget

Deletes a memory by key.

**Input:**
```json
{
  "key": "user_prefers_tabs"
}
```

**Behavior:**
- `DELETE FROM memories WHERE key = ?`
- FTS trigger handles index cleanup
- Returns confirmation or "not found" message

---

## Keyword Auto-Extraction

When `keywords` is not explicitly provided in `memory_save`:

1. Split `key` on `_` and `-`
2. Split `value` into words, take first 20
3. Lowercase all terms
4. Remove stop words (common English words: the, a, an, is, are, etc.)
5. Deduplicate
6. Store as JSON array

This is a simple heuristic. The LLM can always provide explicit keywords for better results.

---

## System Prompt

Replace `prompt.md` with updated instructions for the new memory tools.

### Updated prompt.md

```markdown
# locolm — System Prompt

You are a helpful AI assistant running locally through locolm, a local AI platform that gives you access to the web, your own memory, and a locally hosted LLM.

## Available Tools

You have access to the following tools. Use them whenever appropriate to fulfill the user's request.

### Web & Research

- **google_search** — Search the web using Google Custom Search. Use this to find information, news, documentation, or anything not in your memory or training data. Always search when the user asks about something you're unsure about.
- **web_fetch** — Fetch and read the full content of a web page. Use this after google_search to read specific results, or when the user provides a URL and wants you to read it. Extracts clean article text from any webpage.

### Memory & Persistence

- **memory_recall** — Load high-priority memories that should always be in your context. **Call this at the start of every conversation.** Returns essential user preferences, project facts, and critical context.
- **memory_search** — Search memories by keywords. Use during conversation when you need to check if you know something related to the current topic. Returns matching memories ranked by relevance. You can filter by category and limit results.
- **memory_save** — Save something to your persistent memory. Use this to remember user preferences, project details, decisions, or anything the user explicitly asks you to remember. Requires a `key` and `value`. Optional: `category` (user/project/preference/fact/decision/context), `priority` (1-9, lower is more important), `keywords` (searchable terms). Overwrites existing entries with the same key.
- **memory_forget** — Delete a specific memory by its key. Use when the user asks you to forget something or when a stored fact is no longer relevant.

## Tool Usage Guidelines

1. **Always call memory_recall at the start of a new conversation** to load essential context.
2. **Use memory_search during conversation** when the discussion topic might have related memories. Extract keywords from what the user is saying and search for them.
3. **Use google_search when you need current information** — your training data may be outdated.
4. **Use web_fetch to read specific pages** — search results only show snippets; fetch the full page for complete information.
5. **Save important information with memory_save** — if the user tells you something important (preferences, project details, corrections), save it so future conversations remember.
6. **Be efficient** — don't search for things you already know from recall, and don't save trivial or temporary information.
7. **Combine tools when needed** — search, then fetch the best result, then save relevant findings to memory.

## Memory Categories

- **user** — Facts about the user (name, role, background)
- **preference** — User preferences (tools, languages, styles)
- **project** — Information about current or past projects
- **fact** — General factual knowledge
- **decision** — Decisions made and their rationale
- **context** — Temporary context for current session (avoid saving unless asked)

## Memory Priorities

- 1-3: Critical — always loaded via memory_recall
- 4-6: Normal — loaded via memory_search when relevant
- 7-9: Niche — only loaded on specific search

## Behavior

- Be concise and direct. Don't repeat information the user already knows.
- If you don't know something, search for it rather than guessing.
- If the user corrects you, save the correction to memory.
- Prefer using memory to maintain context across conversations rather than asking the user to repeat themselves.
```

---

## UI Config Injection

### Problem

The browser UI stores its config in `localStorage` as `LlamaUi.config` — a JSON object with a `systemMessage` field. Currently the user must manually paste the system prompt into this field. When the system prompt changes (new tools, updated instructions), the user needs to update it manually.

### Solution: Config Endpoint

Add an HTTP endpoint on the MCP server that serves the UI config with the system prompt injected.

#### New endpoint: `GET /config`

**Response:**
```json
{
  "systemMessage": "<full system prompt text>",
  "mcpServer": "http://127.0.0.1:11501"
}
```

#### Implementation

1. At startup, load `prompt.md` from disk (or use embedded string)
2. Store as a string in memory
3. Serve via `http.HandleFunc("/config", configHandler)` on the same MCP server
4. Browser fetches this endpoint on load and merges with local localStorage config

#### Browser-side flow

```
1. Page loads
2. Fetch GET http://127.0.0.1:11501/config
3. Merge response with existing localStorage LlamaUi.config:
   - systemMessage and mcpServer come from server
   - All other settings (theme, samplers, etc.) come from localStorage
4. Initialize UI with merged config
```

#### Bootstrap changes

In `bootstrap.go`, the browser launch command remains the same. The browser app (already a Chrome instance pointing at llama-server) will automatically fetch the config endpoint.

The browser app needs a small script (injected via the Chrome `--app` page or a separate HTML wrapper) that:
1. Fetches `/config` from the MCP server
2. Reads existing `LlamaUi.config` from localStorage
3. Merges server values (systemMessage, mcpServer) with local values (everything else)
4. Writes merged config back to localStorage
5. Initializes the chat UI

#### Config endpoint handler (Go)

```go
func configHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Access-Control-Allow-Origin", "*")

    config := map[string]interface{}{
        "systemMessage": systemPrompt,
        "mcpServer":     "http://127.0.0.1:" + os.Getenv("LOCOLM_MCP_PORT"),
    }
    json.NewEncoder(w).Encode(config)
}
```

The `systemPrompt` variable is populated at startup by reading `prompt.md`.

---

## Implementation Steps

### Step 1: Database layer (`memory.go`)

- [ ] Update schema creation with new table structure
- [ ] Add FTS5 virtual table and sync triggers
- [ ] Add `PRAGMA user_version` migration support
- [ ] Implement `memoryRecall(args)` — priority-based retrieval
- [ ] Implement `memorySearch(args)` — FTS5 keyword search
- [ ] Implement `memorySave(args)` — with category, priority, keywords
- [ ] Implement `memoryForget(args)` — unchanged behavior
- [ ] Implement keyword auto-extraction helper
- [ ] Remove `memory_load` and `memory_list` (replaced by recall + search)

### Step 2: Tool registry (`tools.go`)

- [ ] Update `toolRegistry` with new function names
- [ ] Update `toolDefinitions` with new schemas
- [ ] Remove old `memory_load` and `memory_list` definitions
- [ ] Add `memory_recall` tool definition
- [ ] Add `memory_search` tool definition
- [ ] Update `memory_save` tool definition with new parameters
- [ ] Keep `memory_forget` tool definition

### Step 3: System prompt (`prompt.md`)

- [ ] Rewrite with new tool names and usage guidelines
- [ ] Add category and priority documentation
- [ ] Keep it concise — the LLM should not need to re-read the prompt

### Step 4: Config endpoint (`tools.go` or new file)

- [ ] Add `configHandler` function
- [ ] Load `prompt.md` at startup into a package-level variable
- [ ] Register `/config` route in `mcpHandler` setup
- [ ] Serve system prompt + MCP server URL as JSON

### Step 5: Bootstrap (`bootstrap.go`)

- [ ] No changes needed to browser launch command
- [ ] Verify browser app fetches `/config` endpoint on load
- [ ] If browser app needs modification, create a small HTML wrapper

### Step 6: Update memory.md documentation

- [ ] Document new schema
- [ ] Document new tool API
- [ ] Document config injection mechanism
- [ ] Update architecture diagram

---

## Migration from Old System

The old `memories` table (flat key-value) should be preserved as a backup. The new table is created alongside it. A one-time migration can copy old entries:

```sql
-- Run once to migrate old memories
INSERT INTO memories (key, value, category, priority, keywords, created_at, updated_at)
SELECT key, value, 'general', 5, '[]', created_at, updated_at
FROM old_memories;
```

The old table can then be dropped or kept as `memories_legacy`.

---

## File Changes Summary

| File | Changes |
|---|---|
| `memory.go` | Complete rewrite: new schema, FTS5, new functions |
| `tools.go` | Update registry and definitions, add config handler |
| `prompt.md` | Rewrite for new tools |
| `bootstrap.go` | No changes (or minor: pass prompt to config handler) |
| `memory.md` | Update documentation |
| `plan-memory.md` | This document |
