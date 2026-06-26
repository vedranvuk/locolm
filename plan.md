# Plan: Memory System Rewrite

## Goal

Replace the flat key/value memory store with a bucketed CRUD system.

## Schema

```sql
CREATE TABLE IF NOT EXISTS memories (
    key         TEXT NOT NULL,
    value       TEXT NOT NULL,
    project     TEXT NOT NULL,
    keywords    TEXT DEFAULT '',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (key, project)
);
```

- `project` = bucket name (free-form string, required)
- `keywords` = comma-separated tags for future FTS search
- Same key can exist in different buckets

## Tool Design (7 tools)

| Tool | Required args | Optional args | Purpose |
|------|--------------|---------------|---------|
| `memory_save` | `bucket`, `key`, `value` | `keywords` | Upsert into bucket |
| `memory_edit` | `bucket`, `key`, `value` | — | Update (error if missing) |
| `memory_delete` | `bucket`, `key` | — | Delete one |
| `memory_load` | `bucket`, `key` | — | Read one |
| `memory_list` | — | `bucket` | List keys+values; omit = all |
| `memory_delete_bucket` | `bucket` | — | Delete bucket + contents |
| `memory_list_buckets` | — | — | List all buckets + counts |

## Files to change

1. `memory.go` — full rewrite
2. `tools.go` — update registry + definitions
3. `prompt.md` — update memory section
4. `memory.md` — update docs
