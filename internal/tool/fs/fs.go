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
	"sort"
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
				}
			}
		}`),
		fsList,
	)

	mcp.RegisterTool(
		"fs_read",
		"Read a text file's content. File must be within an allowed path and under the read size limit.",
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
		"Create or overwrite a file with text content. File must be within an allowed path.",
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
		"Delete a single file. The file must be within an allowed path. Cannot delete directories.",
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
		"Display a directory tree structure as indented text. Depth-limited for safety.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Root directory for the tree. Defaults to current directory."
				},
				"depth": {
					"type": "string",
					"description": "Maximum depth to traverse. Defaults to configured limit (3)."
				},
				"exclude": {
					"type": "string",
					"description": "Comma-separated list of directory names to exclude (e.g. 'node_modules,.git,vendor')"
				}
			}
		}`),
		fsTree,
	)
}

// ---------------------------------------------------------------------------
// Sandbox
// ---------------------------------------------------------------------------

// resolveAndValidate resolves a path (expanding "~" to home) and verifies
// it falls within one of the configured allowed base directories.
// Returns the cleaned absolute path or an error.
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

		// Check if absPath is within or equal to baseAbs.
		// Use filepath.Rel to properly compute the relationship:
		// if the relative path doesn't start with "..", it's inside.
		if absPath == baseAbs {
			return absPath, nil
		}
		rel, err := filepath.Rel(baseAbs, absPath)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return absPath, nil
		}
	}

	return "", fmt.Errorf("path %q is outside allowed directories (allowed: %v)", inputPath, fsCfg.AllowedPaths)
}

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

	var results []listEntry
	for _, entry := range entries {
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

	content := string(data)

	// Apply offset/limit
	offset := 1
	if v := args["offset"]; v != "" {
		fmt.Sscanf(v, "%d", &offset)
	}
	limit := 500
	if v := args["limit"]; v != "" {
		fmt.Sscanf(v, "%d", &limit)
	}

	lines := strings.Split(content, "\n")
	if offset > len(lines) {
		return "", nil
	}
	end := offset + limit
	if end > len(lines) || limit <= 0 {
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
		fmt.Sscanf(v, "%d", &maxResults)
	}

	var results []string

	err = filepath.WalkDir(resolved, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}

		// Match against the pattern using the file's base name
		matched, matchErr := filepath.Match(pattern, d.Name())
		if matchErr != nil {
			return nil
		}
		// Also try matching against relative path for patterns like "**/*.go"
		if !matched {
			rel, relErr := filepath.Rel(resolved, path)
			if relErr == nil {
				matched, _ = filepath.Match(pattern, rel)
			}
		}

		if matched {
			results = append(results, path)
			if len(results) >= maxResults {
				return filepath.SkipAll
			}
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return "", fmt.Errorf("search failed: %w", err)
	}

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
		fmt.Sscanf(v, "%d", &maxDepth)
	}

	excludeSet := map[string]bool{}
	if v := args["exclude"]; v != "" {
		for _, name := range strings.Split(v, ",") {
			excludeSet[strings.TrimSpace(name)] = true
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n", resolved)

	buildTree(&sb, resolved, "", 0, maxDepth, excludeSet)

	return sb.String(), nil
}

func buildTree(sb *strings.Builder, dirPath string, prefix string, depth int, maxDepth int, excludeSet map[string]bool) {
	if depth >= maxDepth {
		return
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return
	}

	// Sort: directories first, then by name
	sort.Slice(entries, func(i, j int) bool {
		iIsDir := entries[i].IsDir()
		jIsDir := entries[j].IsDir()
		if iIsDir != jIsDir {
			return iIsDir // directories first
		}
		return entries[i].Name() < entries[j].Name()
	})

	for i, entry := range entries {
		if excludeSet[entry.Name()] {
			continue
		}

		isLast := i == len(entries)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		fmt.Fprintf(sb, "%s%s%s\n", prefix, connector, entry.Name())

		if entry.IsDir() {
			extension := "│   "
			if isLast {
				extension = "    "
			}
			buildTree(sb, filepath.Join(dirPath, entry.Name()), prefix+extension, depth+1, maxDepth, excludeSet)
		}
	}
}
