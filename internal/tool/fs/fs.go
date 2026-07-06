// Package fs provides filesystem tools for the MCP server.
// All paths are sandboxed to configured allowed base directories.
// Config is self-registered via init() using tool.RegisterConfig.
package fs

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vedranvuk/locolm/internal/mcp"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

// FSConfig holds all configuration for the filesystem tools.
// Defined here — not in config.go — to maintain separation of concerns.
type FSConfig struct {
	AllowedPaths   []string `json:"allowed_paths"`
	ReadMaxBytes   int64    `json:"read_max_bytes"`
	WriteMaxBytes  int64    `json:"write_max_bytes"`
	FindMaxResults int      `json:"find_max_results"`
	TreeMaxDepth   int      `json:"tree_max_depth"`
}

// fsCfg is the package-level config with safe defaults.
var fsCfg = FSConfig{
	AllowedPaths:   []string{"."},
	ReadMaxBytes:   1048576, // 1 MB
	WriteMaxBytes:  1048576, // 1 MB
	FindMaxResults: 200,
	TreeMaxDepth:   3,
}

func init() {

	// Register tools


mcp.RegisterTool(
		"fs_list",
		"List directory contents. Returns name, size, type, and modification time for each entry.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Directory path to list (relative or absolute). Defaults to current directory."
				},
				"sort_by": {
					"type": "string",
					"description": "Sort field: 'name', 'size', 'modified'. Defaults to 'name'.",
					"enum": ["name", "size", "modified"]
				},
				"sort_order": {
					"type": "string",
					"description": "Sort order: 'asc' or 'desc'. Defaults to 'asc'.",
					"enum": ["asc", "desc"]
				},
				"show_hidden": {
					"type": "string",
					"description": "Set to 'true' to include hidden files/directories (starts with dot). Defaults to 'false'."
				}
			}
		}`),
		fsList,
	)

	mcp.RegisterTool(
		"fs_read",
		"Read a text file's content.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Path to the file to read (relative or absolute)."
				},
				"offset": {
					"type": "string",
					"description": "Line number to start reading from (1-based). Defaults to 1."
				},
				"limit": {
					"type": "string",
					"description": "Maximum number of lines to read. Defaults to 500."
				}
			},
			"required": ["path"]
		}`),
		fsRead,
	)

	mcp.RegisterTool(
		"fs_write",
		"Create or overwrite a file with text content.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Path to the file to write (relative or absolute). Parent directory must exist."
				},
				"content": {
					"type": "string",
					"description": "The text content to write to the file."
				}
			},
			"required": ["path", "content"]
		}`),
		fsWrite,
	)

	mcp.RegisterTool(
		"fs_delete",
		"Delete a single file. Cannot delete directories.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Path to the file to delete (relative or absolute)."
				}
			},
			"required": ["path"]
		}`),
		fsDelete,
	)

	mcp.RegisterTool(
		"fs_find",
		"Find files by name pattern (glob). Returns matching file paths.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"pattern": {
					"type": "string",
					"description": "Glob pattern to match (e.g. '*.go', '**/*.json', 'src/*.ts')"
				},
				"path": {
					"type": "string",
					"description": "Base directory to search in. Defaults to current directory."
				},
				"max_results": {
					"type": "string",
					"description": "Maximum number of results to return. Defaults to configured limit (200)."
				}
			},
			"required": ["pattern"]
		}`),
		fsFind,
	)

mcp.RegisterTool(
		"fs_tree",
		"Display a directory tree structure as indented text.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Root directory for the tree. Defaults to current directory."
				},
				"depth": {
					"type": "string",
					"description": "Maximum depth to traverse. Defaults to (3)."
				},
				"exclude": {
					"type": "string",
					"description": "Comma-separated list of directory names to exclude (e.g. 'node_modules,.git,vendor')"
				},
				"show_hidden": {
					"type": "string",
					"description": "Set to 'true' to include hidden files/directories (starts with dot). Defaults to 'false'."
				}
			}
		}`),
		fsTree,
	)
	
	mcp.RegisterTool(
		"fs_replace",
		"Replace exact literal string or regex pattern in a file. Use this tool to edit source files.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": { "type": "string", "description": "Path to the file." },
				"old_content": { "type": "string", "description": "The exact literal string or regex pattern to replace. Must match exactly, including whitespace." },
				"new_content": { "type": "string", "description": "The replacement string." },
				"is_regex": { "type": "string", "description": "Set to 'true' to evaluate old_content as a regular expression. Defaults to 'false' (literal match)." }
			},
			"required": ["path", "old_content", "new_content"]
		}`),
		fsReplace,
	)

	mcp.RegisterTool(
		"fs_append",
		"Append exact content to the end of a file. Escape special characters (Go syntax).",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": { "type": "string" },
				"content": { "type": "string", "description": "Exact string to append. Include \\n if needed." }
			},
			"required": ["path", "content"]
		}`),
		fsAppend,
	)

	mcp.RegisterTool(
		"fs_prepend",
		"Prepend exact content to the beginning of a file. Escape special characters (Go syntax).",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": { "type": "string" },
				"content": { "type": "string", "description": "Exact string to prepend. Include \\n if needed." }
			},
			"required": ["path", "content"]
		}`),
		fsPrepend,
	)

	mcp.RegisterTool(
		"fs_move",
		"Move or rename a file. Creates parent directories if missing.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": { "type": "string", "description": "Current file path." },
				"new_path": { "type": "string", "description": "New file path." }
			},
			"required": ["path", "new_path"]
		}`),
		fsMove,
	)
}

// ---------------------------------------------------------------------------
// Sandbox
// ---------------------------------------------------------------------------

// LoadFSConfig unmarshals the fs JSON config into fsCfg.
// Call this from main after LoadConfig.
func LoadFSConfig(raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}
	json.Unmarshal(raw, &fsCfg)
	log.Printf("[FS] Config loaded: allowed_paths=%v, read_max=%d, write_max=%d, find_max=%d, tree_depth=%d",
		fsCfg.AllowedPaths, fsCfg.ReadMaxBytes, fsCfg.WriteMaxBytes, fsCfg.FindMaxResults, fsCfg.TreeMaxDepth)
}

// resolveAndValidate resolves a path (expanding "~" to home) and verifies
// it falls within one of the configured allowed base directories.
// Returns the cleaned absolute path or an error.
// Resolves symlinks to prevent sandbox escape.
func resolveAndValidate(inputPath string) (string, error) {
	if inputPath == "" {
		inputPath = "."
	}

	// Expand ~ to home directory
	if strings.HasPrefix(inputPath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not resolve home directory: %w", err)
		}
		inputPath = filepath.Join(home, inputPath[1:])
	}

	// Resolve to absolute path (relative to CWD)
	absPath, err := filepath.Abs(inputPath)
	if err != nil {
		return "", fmt.Errorf("invalid path %q: %w", inputPath, err)
	}

	// Resolve symlinks to get the real path
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If the path doesn't exist (e.g. fs_write creating a new file),
		// use the absolute path as-is for validation
		realPath = absPath
	}

	// Check against each allowed base
	for _, base := range fsCfg.AllowedPaths {
		if base == "" {
			continue
		}

		// Expand ~ in base too
		if strings.HasPrefix(base, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				continue
			}
			base = filepath.Join(home, base[1:])
		}

		baseAbs, err := filepath.Abs(base)
		if err != nil {
			continue
		}

		// Resolve symlinks in base as well
		baseReal, err := filepath.EvalSymlinks(baseAbs)
		if err != nil {
			baseReal = baseAbs
		}

		// Check if realPath is within or equal to baseReal.
		if realPath == baseReal {
			return realPath, nil
		}
		rel, err := filepath.Rel(baseReal, realPath)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return realPath, nil
		}
	}

	return "", fmt.Errorf("path %q is outside allowed directories (allowed: %v)", inputPath, fsCfg.AllowedPaths)
}

// ---------------------------------------------------------------------------
// fs_list
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// fs_list
// ---------------------------------------------------------------------------

type listEntry struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	IsDir    bool   `json:"is_dir"`
	Modified string `json:"modified"`
}

func fsList(args map[string]string) (string, error) {
	dirPath := args["path"]
	if dirPath == "" {
		dirPath = "."
	}

	resolved, err := resolveAndValidate(dirPath)
	if err != nil {
		return "", err
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return "", fmt.Errorf("failed to read directory %q: %w", resolved, err)
	}

	showHidden := args["show_hidden"] == "true"

	log.Printf("[FS] Listing %s (read %d raw entries, show_hidden %v)", resolved, len(entries), showHidden)

	var results []listEntry
	for _, entry := range entries {
		if !showHidden && isHidden(entry.Name()) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue // skip entries we can't stat
		}
		results = append(results, listEntry{
			Name:     entry.Name(),
			Size:     info.Size(),
			IsDir:    entry.IsDir(),
			Modified: info.ModTime().Format(time.RFC3339),
		})
	}

	// Sort
	sortBy := args["sort_by"]
	if sortBy == "" {
		sortBy = "name"
	}
	sortOrder := args["sort_order"]
	if sortOrder == "" {
		sortOrder = "asc"
	}

	sort.Slice(results, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "size":
			less = results[i].Size < results[j].Size
		case "modified":
			less = results[i].Modified < results[j].Modified
		default: // name
			less = results[i].Name < results[j].Name
		}
		if sortOrder == "desc" {
			return !less
		}
		return less
	})

	b, _ := json.MarshalIndent(results, "", "  ")
	return string(b), nil
}

// ---------------------------------------------------------------------------
// fs_read
// ---------------------------------------------------------------------------

func fsRead(args map[string]string) (string, error) {
	filePath := args["path"]
	if filePath == "" {
		return "", fmt.Errorf("path is required")
	}

	resolved, err := resolveAndValidate(filePath)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("file not found: %q", resolved)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %q", resolved)
	}
	if info.Size() > fsCfg.ReadMaxBytes {
		return "", fmt.Errorf("file size %d exceeds read limit of %d bytes", info.Size(), fsCfg.ReadMaxBytes)
	}

	f, err := os.Open(resolved)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	// Read entire file (size already checked)
	data, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	log.Printf("[FS] Read %d bytes from %s", len(data), resolved)

	content := string(data)

	// Apply offset/limit
	offset := 1
	if v := args["offset"]; v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return "", fmt.Errorf("invalid offset %q: must be a positive integer", v)
		}
		offset = n
	}
	limit := 500
	if v := args["limit"]; v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return "", fmt.Errorf("invalid limit %q: must be a positive integer", v)
		}
		limit = n
	}

	lines := strings.Split(content, "\n")
	if offset > len(lines) {
		return "", nil
	}
	end := offset + limit
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[offset-1:end], "\n"), nil
}

// ---------------------------------------------------------------------------
// fs_write
// ---------------------------------------------------------------------------

func fsWrite(args map[string]string) (string, error) {
	filePath := args["path"]
	content := args["content"]

	if filePath == "" {
		return "", fmt.Errorf("path is required")
	}

	if int64(len(content)) > fsCfg.WriteMaxBytes {
		return "", fmt.Errorf("content size %d exceeds write limit of %d bytes", len(content), fsCfg.WriteMaxBytes)
	}

	resolved, err := resolveAndValidate(filePath)
	if err != nil {
		return "", err
	}

	// Verify parent directory exists
	parentDir := filepath.Dir(resolved)
	if _, err := os.Stat(parentDir); err != nil {
		return "", fmt.Errorf("parent directory does not exist: %q", parentDir)
	}

	if err := os.WriteFile(resolved, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	log.Printf("[FS] Wrote %d bytes to %s", len(content), resolved)
	return fmt.Sprintf("File written successfully (%d bytes)", len(content)), nil
}

// ---------------------------------------------------------------------------
// fs_delete
// ---------------------------------------------------------------------------

func fsDelete(args map[string]string) (string, error) {
	filePath := args["path"]
	if filePath == "" {
		return "", fmt.Errorf("path is required")
	}

	resolved, err := resolveAndValidate(filePath)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("file not found: %q", resolved)
	}
	if info.IsDir() {
		return "", fmt.Errorf("cannot delete directories with fs_delete, only files")
	}

	if err := os.Remove(resolved); err != nil {
		return "", fmt.Errorf("failed to delete file: %w", err)
	}

	log.Printf("[FS] Deleted file: %s", resolved)
	return "File deleted successfully", nil
}

// ---------------------------------------------------------------------------
// fs_find
// ---------------------------------------------------------------------------

func fsFind(args map[string]string) (string, error) {
	pattern := args["pattern"]
	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	basePath := args["path"]
	if basePath == "" {
		basePath = "."
	}

	resolved, err := resolveAndValidate(basePath)
	if err != nil {
		return "", err
	}

	maxResults := fsCfg.FindMaxResults
	if v := args["max_results"]; v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return "", fmt.Errorf("invalid max_results %q: must be a positive integer", v)
		}
		maxResults = n
	}

	log.Printf("[FS] Finding %q in %s (max %d)", pattern, resolved, maxResults)

	var results []string

	err = filepath.WalkDir(resolved, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}

		// Resolve symlinks to prevent sandbox escape
		realPath := path
		if rp, evalErr := filepath.EvalSymlinks(path); evalErr == nil {
			realPath = rp
		}

		// Check that the real path is still within allowed directories
		valid := false
		for _, base := range fsCfg.AllowedPaths {
			if base == "" {
				continue
			}
			baseAbs, _ := filepath.Abs(base)
			baseReal, evalErr := filepath.EvalSymlinks(baseAbs)
			if evalErr != nil {
				baseReal = baseAbs
			}
			if realPath == baseReal {
				valid = true
				break
			}
			rel, relErr := filepath.Rel(baseReal, realPath)
			if relErr == nil && !strings.HasPrefix(rel, "..") {
				valid = true
				break
			}
		}
		if !valid {
			return nil
		}

		// Match against the pattern using the file's base name
		matched, matchErr := filepath.Match(pattern, d.Name())
		if matchErr != nil {
			return nil
		}
		// Also try matching against relative path for patterns like "src/*.ts"
		if !matched {
			rel, relErr := filepath.Rel(resolved, path)
			if relErr == nil {
				matched, _ = filepath.Match(pattern, rel)
			}
		}

		if matched {
			rel, _ := filepath.Rel(resolved, path)
			results = append(results, rel)
			if len(results) >= maxResults {
				return filepath.SkipAll
			}
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return "", fmt.Errorf("search failed: %w", err)
	}

	log.Printf("[FS] Found %d results for %q in %s", len(results), pattern, resolved)
	b, _ := json.MarshalIndent(results, "", "  ")
	return string(b), nil
}

// ---------------------------------------------------------------------------
// fs_tree
// ---------------------------------------------------------------------------

func fsTree(args map[string]string) (string, error) {
	basePath := args["path"]
	if basePath == "" {
		basePath = "."
	}

	resolved, err := resolveAndValidate(basePath)
	if err != nil {
		return "", err
	}

	maxDepth := fsCfg.TreeMaxDepth
	if v := args["depth"]; v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return "", fmt.Errorf("invalid depth %q: must be a positive integer", v)
		}
		maxDepth = n
	}

	excludeSet := map[string]bool{}
	if v := args["exclude"]; v != "" {
		for _, name := range strings.Split(v, ",") {
			excludeSet[strings.TrimSpace(name)] = true
		}
	}

	showHidden := args["show_hidden"] == "true"

	log.Printf("[FS] Tree of %s (depth %d, exclude %v, show_hidden %v)", resolved, maxDepth, excludeSet, showHidden)

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n", resolved)

	treeMaxEntries := fsCfg.FindMaxResults // reuse find limit as tree entry cap
	buildTree(&sb, resolved, "", 0, maxDepth, excludeSet, &treeMaxEntries, showHidden)

	log.Printf("[FS] Tree output: %d lines", strings.Count(sb.String(), "\n"))
	return sb.String(), nil
}

func isHidden(name string) bool {
	// A simple dot-prefix check covers Unix hidden files and standard developer
	// configurations (like .git, .vscode, .env) cross-platform.
	return strings.HasPrefix(name, ".")
}

func buildTree(sb *strings.Builder, dirPath string, prefix string, depth int, maxDepth int, excludeSet map[string]bool, remaining *int, showHidden bool) {
	if depth >= maxDepth || *remaining <= 0 {
		return
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return
	}

	// Filter entries early so the last-item connector logic (└──) is calculated accurately
	var filtered []os.DirEntry
	for _, entry := range entries {
		if excludeSet[entry.Name()] {
			continue
		}
		if !showHidden && isHidden(entry.Name()) {
			continue
		}
		filtered = append(filtered, entry)
	}

	// Sort: directories first, then by name
	sort.Slice(filtered, func(i, j int) bool {
		iIsDir := filtered[i].IsDir()
		jIsDir := filtered[j].IsDir()
		if iIsDir != jIsDir {
			return iIsDir // directories first
		}
		return filtered[i].Name() < filtered[j].Name()
	})

	for i, entry := range filtered {
		if *remaining <= 0 {
			return
		}

		isLast := i == len(filtered)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		fmt.Fprintf(sb, "%s%s%s\n", prefix, connector, entry.Name())
		*remaining--

		if entry.IsDir() {
			extension := "│   "
			if isLast {
				extension = "    "
			}
			buildTree(sb, filepath.Join(dirPath, entry.Name()), prefix+extension, depth+1, maxDepth, excludeSet, remaining, showHidden)
		}
	}
}

// ---------------------------------------------------------------------------
// fs_replace
// ---------------------------------------------------------------------------

func fsReplace(args map[string]string) (string, error) {
	resolved, err := resolveAndValidate(args["path"])
	if err != nil {
		return "", err
	}

	contentBytes, err := os.ReadFile(resolved)
	if err != nil {
		return "", err
	}

	content := string(contentBytes)
	oldContent := args["old_content"]
	newContent := args["new_content"]
	isRegex := args["is_regex"] == "true"

	var finalContent string
	if isRegex {
		re, err := regexp.Compile(oldContent)
		if err != nil {
			return "", fmt.Errorf("invalid regex pattern: %w", err)
		}
		finalContent = re.ReplaceAllString(content, newContent)
	} else {
		if !strings.Contains(content, oldContent) {
			return "", fmt.Errorf("exact match for old_content not found in file")
		}
		// Replace the first exact match to allow sequential edits of identical blocks
		finalContent = strings.Replace(content, oldContent, newContent, 1)
	}

	if int64(len(finalContent)) > fsCfg.WriteMaxBytes {
		return "", fmt.Errorf("content size exceeds write limit")
	}

	if err := os.WriteFile(resolved, []byte(finalContent), 0644); err != nil {
		return "", err
	}
	return "Content replaced successfully", nil
}

// ---------------------------------------------------------------------------
// fs_append & fs_prepend
// ---------------------------------------------------------------------------

func fsAppend(args map[string]string) (string, error) {
	resolved, err := resolveAndValidate(args["path"])
	if err != nil { return "", err }

	existing, err := os.ReadFile(resolved)
	if err != nil && !os.IsNotExist(err) { return "", err }

	// Strictly append exact bytes provided
	finalContent := string(existing) + args["content"]

	if int64(len(finalContent)) > fsCfg.WriteMaxBytes {
		return "", fmt.Errorf("content size exceeds write limit")
	}
	if err := os.WriteFile(resolved, []byte(finalContent), 0644); err != nil {
		return "", err
	}
	return "Appended exactly as requested", nil
}

func fsPrepend(args map[string]string) (string, error) {
	resolved, err := resolveAndValidate(args["path"])
	if err != nil { return "", err }

	existing, err := os.ReadFile(resolved)
	if err != nil && !os.IsNotExist(err) { return "", err }

	// Strictly prepend exact bytes provided
	finalContent := args["content"] + string(existing)

	if int64(len(finalContent)) > fsCfg.WriteMaxBytes {
		return "", fmt.Errorf("content size exceeds write limit")
	}
	if err := os.WriteFile(resolved, []byte(finalContent), 0644); err != nil {
		return "", err
	}
	return "Prepended exactly as requested", nil
}

// ---------------------------------------------------------------------------
// fs_move
// ---------------------------------------------------------------------------

func fsMove(args map[string]string) (string, error) {
	resolvedOld, err := resolveAndValidate(args["path"])
	if err != nil {
		return "", err
	}

	resolvedNew, err := resolveAndValidate(args["new_path"])
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(resolvedNew), 0755); err != nil {
		return "", err
	}
	if err := os.Rename(resolvedOld, resolvedNew); err != nil {
		return "", err
	}
	return "File moved/renamed successfully", nil
}