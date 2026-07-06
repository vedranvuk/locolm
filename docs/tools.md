# Complete Tool Reference

## Workspace & File Tools

### create_directory
**Description:** Create a new directory structure in the workspace. Will recursively create all directories in the path, like mkdir -p.

**Parameters:**
- `dirPath` (required, string): The absolute path to the directory to create

**Return Value:** Directory creation result

---

### create_file
**Description:** Create a new file in the workspace with specified content. The directory will be created if it does not already exist. Never use this tool to edit a file that already exists.

**Parameters:**
- `filePath` (required, string): The absolute path to the file to create
- `content` (required, string): The content to write to the file

**Return Value:** File creation confirmation

---

### list_dir
**Description:** List the contents of a directory. Result will have the name of the child. If the name ends in /, it's a folder, otherwise a file.

**Parameters:**
- `path` (required, string): The absolute path to the directory to list

**Return Value:** Array of child names (files and directories)

---

### read_file
**Description:** Read the contents of a file. You must specify the line range you're interested in. Line numbers are 1-indexed. If the file contents returned are insufficient, you may call this tool again to retrieve more content. For binary files, startLine/endLine are byte offsets.

**Parameters:**
- `filePath` (required, string): The absolute path of the file to read
- `startLine` (required, number): The line number to start reading from (1-based)
- `endLine` (required, number): The inclusive line number to end reading at (1-based)

**Return Value:** File contents for specified line range

---

### replace_string_in_file
**Description:** Make edits in an existing file. For moving or renaming files, use terminal commands instead. For larger edits, split into smaller edits and call multiple times. Before editing, ensure you have context to understand the file's contents.

**Parameters:**
- `filePath` (required, string): An absolute path to the file to edit
- `oldString` (required, string): The exact literal text to replace (must include at least 3 lines of context BEFORE and AFTER the target text, matching whitespace and indentation precisely)
- `newString` (required, string): The exact literal text to replace `oldString` with (also including all whitespace, indentation, newlines, and surrounding code)

**Return Value:** Edit confirmation

---

### multi_replace_string_in_file
**Description:** Apply multiple replace_string_in_file operations in a single call, which is more efficient than calling replace_string_in_file multiple times.

**Parameters:**
- `explanation` (required, string): A brief explanation of what the multi-replace operation will accomplish
- `replacements` (required, array of objects): Array of replacement operations to apply sequentially
  - Each object contains:
    - `filePath` (required, string): An absolute path to the file to edit
    - `oldString` (required, string): The exact literal text to replace
    - `newString` (required, string): The exact literal text to replace oldString with

**Return Value:** Summary of successful and failed operations

---

## Search & Navigation Tools

### file_search
**Description:** Search for files in the workspace by glob pattern. This only returns the paths of matching files. Glob patterns match from the root of the workspace folder.

**Parameters:**
- `query` (required, string): Search for files with names or paths matching this glob pattern (e.g., `**/*.{js,ts}`, `src/**`, `**/foo/**/*.js`)
- `maxResults` (optional, number): The maximum number of results to return. Default returns some matches; if not seeing what you need, try again with a more specific query or larger maxResults

**Return Value:** Array of matching file paths

---

### grep_search
**Description:** Do a fast text search in the workspace using exact string or regex. Useful for searching code content, not file names.

**Parameters:**
- `query` (required, string): The pattern to search for (plain text or regex). Use regex with alternation (e.g., 'word1|word2|word3') or character classes to find multiple words in one search. Case-insensitive
- `isRegexp` (required, boolean): Whether the pattern is a regex
- `includePattern` (optional, string): Search files matching this glob pattern (e.g., `src/folder/**`). Applies to relative paths within the workspace
- `includeIgnoredFiles` (optional, boolean): Whether to include files normally ignored (.gitignore, ignore files, `files.exclude`, `search.exclude` settings). Warning: may slow things down
- `maxResults` (optional, number): The maximum number of results to return

**Return Value:** Array of matching code snippets with file locations

---

### semantic_search
**Description:** Run a natural language search for relevant code or documentation comments from the user's current workspace. Returns relevant code snippets if workspace is large, or full contents if small.

**Parameters:**
- `query` (required, string): The query to search the codebase for (e.g., function names, variable names, or comments that might appear in the codebase)

**Return Value:** Relevant code snippets or full workspace contents

---

## Code Analysis Tools

### get_errors
**Description:** Get any compile or lint errors in a specific file or across all files. Use when user mentions errors or problems in files.

**Parameters:**
- `filePaths` (optional, array of strings): The absolute paths to the files or folders to check for errors. Omit this when retrieving all errors

**Return Value:** Array of compile/lint errors with file locations and descriptions

---

### vscode_listCodeUsages
**Description:** Find all usages (references, definitions, and implementations) of a code symbol across the workspace. The file and line do NOT need to be the definition of the symbol—any occurrence works (usage, import, call site, etc.).

**Parameters:**
- `symbol` (required, string): The exact name of the symbol (function, class, method, variable, type, etc.)
- `filePath` (optional, string): A workspace-relative file path where the symbol appears (e.g., `src/utils/helpers.ts`). Provide either `uri` or `filePath`
- `uri` (optional, string): A full URI of a file where the symbol appears (e.g., `file:///path/to/file.ts`). Provide either `uri` or `filePath`
- `lineContent` (required, string): A substring of the line of code where the symbol appears (must be actual text from the file)

**Return Value:** Array of all usages of the symbol with file locations and context

---

### vscode_renameSymbol
**Description:** Rename a code symbol across the workspace using the language server's rename functionality. This performs a precise, semantics-aware rename that updates all references.

**Parameters:**
- `symbol` (required, string): The exact current name of the symbol to rename
- `newName` (required, string): The new name for the symbol
- `filePath` (optional, string): A workspace-relative file path where the symbol appears (e.g., `src/utils/helpers.ts`). Provide either `uri` or `filePath`
- `uri` (optional, string): A full URI of a file where the symbol appears (e.g., `file:///path/to/file.ts`). Provide either `uri` or `filePath`
- `lineContent` (required, string): A substring of the line of code where the symbol appears (must be actual text from the file)

**Return Value:** Confirmation of symbol renaming with all locations updated

---

## Code Intelligence Tools

### runSubagent
**Description:** Launch a new agent to handle complex, multi-step tasks autonomously. Good at researching complex questions, searching for code, and executing multi-step tasks. Agents run synchronously—you will wait for the result. Each agent invocation is stateless.

**Parameters:**
- `prompt` (required, string): A detailed description of the task for the agent to perform
- `description` (required, string): A short (3-5 word) description of the task
- `agentName` (optional, string): Optional name of a specific agent to invoke (case-sensitive). If not provided, uses the current agent
- `model` (optional, string): Optional model for the subagent (format: "Model Name (Vendor)"). Only use to enforce a specific model

**Return Value:** A single message with results of the agent's work

---

## Web & Browser Tools

### open_browser_page
**Description:** Open a new browser page in the integrated browser at the given URL. May prompt user to share a page if a similar one already exists, unless "forceNew" is true.

**Parameters:**
- `url` (optional, string): The URL to open (must be absolute URI with scheme like file:, http:, https:). For local files, use canonical absolute form (e.g., `file:///path/to/file`)
- `forceNew` (optional, boolean): Whether to force opening a new page even if a page with the same host already exists. Default is false

**Return Value:** Page ID and accessibility snapshot of the page

---

### fetch_webpage
**Description:** Fetch the main content from a web page. Useful for summarizing or analyzing webpage content.

**Parameters:**
- `urls` (required, array of strings): Array of URLs to fetch content from
- `query` (required, string): The query to search for in the web page's content (clear and concise description of content you want to find)

**Return Value:** Content from the web pages matching the query

---

### navigate_page
**Description:** Navigate a browser page by URL, history, or reload.

**Parameters:**
- `pageId` (required, string): The browser page ID to navigate (from context or open tool)
- `type` (optional, string): Navigation type—"url" (navigate to URL, requires `url` param), "back" or "forward" (history), "reload" (refresh). Default is "url"
- `url` (optional, string): The URL to navigate to. Required when type is "url"

**Return Value:** Navigation confirmation

---

### read_page
**Description:** Get a snapshot of the current browser page state. Better than screenshot for understanding page content and structure.

**Parameters:**
- `pageId` (required, string): The browser page ID to read (from context or open tool)

**Return Value:** Accessibility snapshot with page structure and content

---

### screenshot_page
**Description:** Capture a screenshot of the current browser page. You can't perform actions based on the screenshot; use read_page for actions.

**Parameters:**
- `pageId` (required, string): The browser page ID to capture (from context or open tool)
- `element` (optional, string): Human-readable description of element to capture (e.g., "chart diagram", "product image")
- `ref` (optional, string): Element reference to capture. If omitted, captures whole viewport
- `selector` (optional, string): Playwright selector of element to capture when "ref" is not available. If omitted, captures whole viewport
- `scrollIntoViewIfNeeded` (optional, boolean): Whether to scroll element into view before capturing. Defaults to false

**Return Value:** Screenshot image

---

### click_element
**Description:** Click on an element in a browser page.

**Parameters:**
- `pageId` (required, string): The browser page ID (from context or open tool)
- `element` (required, string): Human-readable description of element to click (e.g., "submit button", "search icon")
- `ref` (optional, string): Element reference to click. One of "ref" or "selector" is required
- `selector` (optional, string): Playwright selector of element to click when "ref" is not available. One of "ref" or "selector" is required
- `button` (optional, string): Mouse button to click with ("left", "right", "middle"). Default is "left"
- `dblClick` (optional, boolean): Set to true for double clicks. Default is false

**Return Value:** Click confirmation

---

### hover_element
**Description:** Hover over an element in a browser page.

**Parameters:**
- `pageId` (required, string): The browser page ID (from context or open tool)
- `element` (required, string): Human-readable description of element to hover over (e.g., "navigation menu", "tooltip trigger")
- `ref` (optional, string): Element reference to hover over. One of "ref" or "selector" is required
- `selector` (optional, string): Playwright selector of element to hover over when "ref" is not available. One of "ref" or "selector" is required

**Return Value:** Hover confirmation

---

### type_in_page
**Description:** Type text or press keys in a browser page.

**Parameters:**
- `pageId` (required, string): The browser page ID (from context or open tool)
- `text` (optional, string): The text to type. One of "text" or "key" must be provided
- `key` (optional, string): A key or key combination to press (e.g., "Enter", "Tab", "Control+c"). One of "text" or "key" must be provided
- `ref` (optional, string): Element reference to focus and type into. If omitted, types into focused element
- `selector` (optional, string): Playwright selector of element to focus and type into. Use if "ref" not available. If omitted, types into focused element
- `element` (optional, string): Human-readable description of element to type into (e.g., "search box", "comment field"). Required when "ref" or "selector" is specified
- `submit` (optional, boolean): Whether to press Enter after typing text. Ignored when "key" is provided. Default is false

**Return Value:** Typing confirmation

---

### drag_element
**Description:** Drag an element over another element in a browser page.

**Parameters:**
- `pageId` (required, string): The browser page ID (from context or open tool)
- `fromElement` (required, string): Human-readable description of element to drag (e.g., "file item", "draggable card")
- `toElement` (required, string): Human-readable description of element to drop onto (e.g., "drop zone", "target folder")
- `fromRef` (optional, string): Element reference of element to drag. One of "fromRef" or "fromSelector" is required
- `fromSelector` (optional, string): Playwright selector of element to drag when "fromRef" is not available. One of "fromRef" or "fromSelector" is required
- `toRef` (optional, string): Element reference of element to drop onto. One of "toRef" or "toSelector" is required
- `toSelector` (optional, string): Playwright selector of element to drop onto when "toRef" is not available. One of "toRef" or "toSelector" is required

**Return Value:** Drag confirmation

---

### handle_dialog
**Description:** Respond to a pending modal (alert, confirm, prompt) or file chooser dialog on a browser page.

**Parameters:**
- `pageId` (required, string): The browser page ID (from context or open tool)
- `acceptModal` (optional, boolean): Whether to accept (true) or dismiss (false) a modal dialog
- `promptText` (optional, string): Text to enter into a prompt dialog
- `selectFiles` (optional, array of strings): Absolute paths of files to select, or empty to dismiss. Required for file chooser dialogs

**Return Value:** Dialog response confirmation

---

### run_playwright_code
**Description:** Run a Playwright code snippet to control a browser page. Only use if other browser tools are insufficient. You must not directly access `document` or `window`—access via the provided `page` object.

**Parameters:**
- `pageId` (required, string): The browser page ID (from context or open tool)
- `code` (optional, string): The Playwright code to execute (must be concise, serve one clear purpose, and be self-contained). Must use the provided `page` object, e.g., `return page.evaluate(() => document.title)`. Omit when resuming deferred execution
- `deferredResultId` (optional, string): If a previous call returned a deferredResultId, pass it here to continue waiting for execution to complete
- `timeoutMs` (optional, number): Maximum time in milliseconds to wait for code to complete. Defaults to 5000 (5 seconds)

**Return Value:** Code execution result

---

## Terminal & Task Tools

### run_in_terminal
**Description:** Execute PowerShell commands in a persistent terminal session, preserving environment variables, working directory, and other context across multiple commands.

**Parameters:**
- `command` (required, string): The command to run in the terminal
- `explanation` (required, string): A one-sentence description of what the command does (shown to user before execution)
- `goal` (required, string): A short description of the goal or purpose (e.g., "Install dependencies", "Start development server")
- `mode` (required, string): Execution mode—"sync" (wait for completion up to timeout) or "async" (wait for initial idle/output signal, then return with terminal ID)
- `timeout` (optional, number): Hard cap in milliseconds before tool returns. Use generous values (600000 = 10 min for installs, 900000 = 15 min for big builds). Omit to let command run to completion

**Return Value:** Command output and exit code (for sync), or terminal ID and output snapshot (for async)

---

### get_terminal_output
**Description:** Get output from an active terminal execution (identified by the `id` returned from run_in_terminal). Use for async executions or sync commands that timed out and moved to background.

**Parameters:**
- `id` (required, string): The ID of an active terminal execution (UUID format from run_in_terminal for async executions or timed-out sync)

**Return Value:** Terminal output

---

### kill_terminal
**Description:** Kill a terminal by its ID. Use this to clean up terminals that are no longer needed (e.g., after stopping a server).

**Parameters:**
- `id` (required, string): The ID of the persistent terminal to kill (UUID format, returned by run_in_terminal in async mode)

**Return Value:** Kill confirmation

---

### terminal_last_command
**Description:** Get the last command run in the active terminal.

**Parameters:** (none)

**Return Value:** Last command text

---

### terminal_selection
**Description:** Get the current selection in the active terminal.

**Parameters:** (none)

**Return Value:** Selected text

---

### create_and_run_task
**Description:** Create and run a build, run, or custom task for the workspace by generating or adding to a tasks.json file based on project structure (package.json, README.md, etc.).

**Parameters:**
- `workspaceFolder` (required, string): The absolute path of the workspace folder where tasks.json will be created
- `task` (required, object): The task to add to tasks.json
  - `label` (required, string): The label of the task
  - `type` (required, string): The type of the task (only "shell" is supported)
  - `command` (required, string): The shell command to run
  - `args` (optional, array of strings): Arguments to pass to the command
  - `group` (optional, string): The group to which the task belongs
  - `isBackground` (optional, boolean): Whether the task runs in background without blocking UI
  - `problemMatcher` (optional, array of strings): Problem matcher to parse task output (e.g., '$tsc', '$eslint - stylish', '$gcc')

**Return Value:** Task creation and execution confirmation

---

### get_task_output
**Description:** Get the output of a task.

**Parameters:**
- `id` (required, string): The task ID for which to get output
- `workspaceFolder` (required, string): The workspace folder path containing the task

**Return Value:** Task output

---

## Project Management Tools

### manage_todo_list
**Description:** Manage a structured todo list to track progress and plan tasks throughout your coding session. Use VERY frequently to ensure task visibility and proper planning.

**Parameters:**
- `todoList` (required, array of objects): Complete array of all todo items (must include ALL items—both existing and new)
  - Each object contains:
    - `id` (required, number): Unique identifier (use sequential numbers starting from 1)
    - `title` (required, string): Concise action-oriented label (3-7 words)
    - `status` (required, string): "not-started", "in-progress" (max 1 at a time), or "completed"

**Return Value:** Todo list update confirmation

---

## Database & Session Tools

### session_store_sql
**Description:** Query the local session store containing history from past coding sessions. Uses SQLite syntax (SELECT and WITH only—read-only). Supports JOINs, FTS5 MATCH, aggregations.

**Parameters:**
- `description` (required, string): A 2-5 word summary of what this call does
- `action` (optional, string): The action to perform—"query" (default, execute SQL) or "reindex" (rebuild local session index)
- `query` (optional, string): A single read-only SQL query to execute. Required when action is "query"
- `force` (optional, boolean): When true with action "reindex", re-processes all sessions. Default false (skips already-indexed)
- `subcommand` (optional, string): The chronicle subcommand that triggered this call (for telemetry)

**Return Value:** Query results or reindex confirmation