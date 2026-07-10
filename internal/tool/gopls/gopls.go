// Package gopls provides tools to interact with the Go Language Server (gopls).
// It manages multiple isolated gopls background daemons with real-time diagnostic interceptors,
// strict capacity caps, and a 5-minute idle eviction policy.
package gopls

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vedranvuk/locolm/internal/mcp"
)

// ---------------------------------------------------------------------------
// Config & State Management
// ---------------------------------------------------------------------------

type Config struct {
	GoplsPath          string `json:"gopls_path"`
	IdleTimeoutMinutes int    `json:"idle_timeout_minutes"`
	MaxWorkspaces      int    `json:"max_workspaces"`
}

func DefaultConfig() *Config {
	return &Config{
		GoplsPath:          "gopls",
		IdleTimeoutMinutes: 5,
		MaxWorkspaces:      10,
	}
}

type Gopls struct {
	mu                  sync.RWMutex
	config              *Config
	workspaces          map[string]*goplsInstance
	activeWorkspacePath string
}

func New(config *Config) (*Gopls, error) {
	return &Gopls{
		config:     config,
		workspaces: make(map[string]*goplsInstance),
	}, nil
}

func (self *Gopls) Register(r mcp.Registry) {
	// --- LIFECYCLE MANAGEMENT ---
	r.RegisterTool(
		"gopls_workspace_activate",
		"Sets the active Go project context workspace directory.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Absolute path to the Go project root directory (containing the go.mod file)."
				}
			},
			"required": ["path"]
		}`),
		self.goplsWorkspaceActivate,
	)

	// --- CATEGORY 1: NAVIGATION & DISCOVERY ---
	r.RegisterTool(
		"gopls_definition",
		"Locates the exact declaration position of a Go identifier.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"file": { "type": "string", "description": "Relative or absolute path to the target Go (.go) file." },
				"line": { "type": "string", "description": "1-based line number." },
				"character": { "type": "string", "description": "0-based character offset within that line." }
			},
			"required": ["file", "line", "character"]
		}`),
		self.goplsDefinition,
	)

	r.RegisterTool(
		"gopls_references",
		"Traces and lists all occurrences, call sites, and usages of a specific Go identifier.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"file": { "type": "string", "description": "Relative or absolute path to the target Go (.go) file." },
				"line": { "type": "string", "description": "1-based line number." },
				"character": { "type": "string", "description": "0-based character offset within that line." }
			},
			"required": ["file", "line", "character"]
		}`),
		self.goplsReferences,
	)

	r.RegisterTool(
		"gopls_implementation",
		"Identifies all concrete Go structs, types, or methods that actively implement a target Go interface.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"file": { "type": "string", "description": "Relative or absolute path to the Go (.go) file containing the interface." },
				"line": { "type": "string", "description": "1-based line number." },
				"character": { "type": "string", "description": "0-based character offset within that line." }
			},
			"required": ["file", "line", "character"]
		}`),
		self.goplsImplementation,
	)

	r.RegisterTool(
		"gopls_symbols",
		"Performs a Go workspace-wide fuzzy search to look up global Go definitions.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": { "type": "string", "description": "Fuzzy search term or Go symbol identifier." }
			},
			"required": ["query"]
		}`),
		self.goplsSymbols,
	)

	// --- CATEGORY 2: DIAGNOSTICS & REAL-TIME ANALYSIS ---
	r.RegisterTool(
		"gopls_diagnostics",
		"Extracts real-time Go compiler diagnostics.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"file": { "type": "string", "description": "Optional Go file path to narrow down scope." }
			}
		}`),
		self.goplsDiagnostics,
	)

	// --- CATEGORY 3: SMART CODE ASSISTANCE ---
	r.RegisterTool(
		"gopls_completion",
		"Generates context-aware Go code intelligence completions.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"file": { "type": "string", "description": "Relative or absolute path to the target Go (.go) file." },
				"line": { "type": "string", "description": "1-based line number." },
				"character": { "type": "string", "description": "0-based character offset within that line." }
			},
			"required": ["file", "line", "character"]
		}`),
		self.goplsCompletion,
	)

	// --- CATEGORY 4: TRANSFORMATIONS & REFACTORING ---
	r.RegisterTool(
		"gopls_rename",
		"Executes an automated, structurally safe refactoring rename of a Go identifier.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"file": { "type": "string", "description": "Relative or absolute path to the Go file." },
				"line": { "type": "string", "description": "1-based line number." },
				"character": { "type": "string", "description": "0-based character offset within that line." },
				"new_name": { "type": "string", "description": "The replacement Go identifier string." }
			},
			"required": ["file", "line", "character", "new_name"]
		}`),
		self.goplsRename,
	)

	// --- CATEGORY 5: ECOSYSTEM SUPPORT ---
	r.RegisterTool(
		"gopls_format",
		"Calculates native gofmt spacing layouts and automatically evaluates import blocks.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"file": { "type": "string", "description": "Relative or absolute path to the target Go source (.go) file or go.mod asset." }
			},
			"required": ["file"]
		}`),
		self.goplsFormat,
	)

	go self.startJanitor(30 * time.Second)
}

// ---------------------------------------------------------------------------
// LSP Types & Engines
// ---------------------------------------------------------------------------

type goplsInstance struct {
	path        string
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	lastActive  time.Time
	reqID       int
	pending     map[int]chan []byte
	diagnostics map[string][]string // Intercepted async compilation problems
	mu          sync.Mutex
}

type lspLocation struct {
	URI   string `json:"uri"`
	Range struct {
		Start struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"start"`
	} `json:"range"`
}

func (inst *goplsInstance) sendRPC(method string, params any) ([]byte, error) {
	inst.mu.Lock()
	inst.reqID++
	id := inst.reqID
	inst.lastActive = time.Now()

	ch := make(chan []byte, 1)
	inst.pending[id] = ch
	inst.mu.Unlock()

	defer func() {
		inst.mu.Lock()
		delete(inst.pending, id)
		inst.mu.Unlock()
	}()

	payload := map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(b), string(b))
	if _, err := inst.stdin.Write([]byte(frame)); err != nil {
		return nil, fmt.Errorf("gopls write failed: %w", err)
	}

	select {
	case res := <-ch:
		return res, nil
	case <-time.After(12 * time.Second):
		return nil, fmt.Errorf("gopls communication timeout")
	}
}

func (inst *goplsInstance) sendNotification(method string, params any) error {
	inst.mu.Lock()
	inst.lastActive = time.Now()
	inst.mu.Unlock()

	payload := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(b), string(b))
	_, err = inst.stdin.Write([]byte(frame))
	return err
}

// ---------------------------------------------------------------------------
// Core Tool Handlers
// ---------------------------------------------------------------------------

func (self *Gopls) goplsWorkspaceActivate(args map[string]string) (string, error) {
	path := args["path"]
	if path == "" {
		return "", fmt.Errorf("path context identifier is required")
	}
	inst, err := self.getOrCreateWorkspace(path)
	if err != nil {
		return "", err
	}
	self.mu.Lock()
	self.activeWorkspacePath = inst.path
	self.mu.Unlock()
	return fmt.Sprintf("Workspace tracking set to active project: %s", inst.path), nil
}

func (self *Gopls) goplsDefinition(args map[string]string) (string, error) {
	return self.handleLocationQuery("textDocument/definition", args)
}

func (self *Gopls) goplsReferences(args map[string]string) (string, error) {
	inst, err := self.getActiveInstance()
	if err != nil {
		return "", err
	}
	absFile, line, char, err := parsePositionArgs(args, inst.path)
	if err != nil {
		return "", err
	}

	params := map[string]any{
		"textDocument": map[string]string{"uri": "file://" + absFile},
		"position":     map[string]int{"line": line, "character": char},
		"context":      map[string]bool{"includeDeclaration": true},
	}
	res, err := inst.sendRPC("textDocument/references", params)
	if err != nil {
		return "", err
	}

	var response struct {
		Result []lspLocation `json:"result"`
	}
	json.Unmarshal(res, &response)

	if len(response.Result) == 0 {
		return "No reference points discovered.", nil
	}
	var out strings.Builder
	for _, ref := range response.Result {
		fmt.Fprintf(&out, "- %s:%d:%d\n", strings.TrimPrefix(ref.URI, "file://"), ref.Range.Start.Line+1, ref.Range.Start.Character)
	}
	return out.String(), nil
}

func (self *Gopls) goplsImplementation(args map[string]string) (string, error) {
	return self.handleLocationQuery("textDocument/implementation", args)
}

func (self *Gopls) goplsSymbols(args map[string]string) (string, error) {
	inst, err := self.getActiveInstance()
	if err != nil {
		return "", err
	}
	res, err := inst.sendRPC("workspace/symbol", map[string]string{"query": args["query"]})
	if err != nil {
		return "", err
	}
	var response struct {
		Result []struct {
			Name     string      `json:"name"`
			Kind     int         `json:"kind"`
			Location lspLocation `json:"location"`
		} `json:"result"`
	}
	json.Unmarshal(res, &response)

	if len(response.Result) == 0 {
		return "No structured symbols matching query found.", nil
	}
	var out strings.Builder
	for _, sym := range response.Result {
		fmt.Fprintf(&out, "- Symbol: %s (Kind %d) in %s:%d\n", sym.Name, sym.Kind, strings.TrimPrefix(sym.Location.URI, "file://"), sym.Location.Range.Start.Line+1)
	}
	return out.String(), nil
}

func (self *Gopls) goplsDiagnostics(args map[string]string) (string, error) {
	inst, err := self.getActiveInstance()
	if err != nil {
		return "", err
	}

	inst.mu.Lock()
	defer inst.mu.Unlock()

	filterFile := args["file"]
	if filterFile != "" && !filepath.IsAbs(filterFile) {
		filterFile = filepath.Join(inst.path, filterFile)
	}

	var out strings.Builder
	found := false
	for file, issues := range inst.diagnostics {
		if filterFile != "" && file != filterFile {
			continue
		}
		found = true
		fmt.Fprintf(&out, "File: %s\n%s\n", file, strings.Join(issues, "\n"))
	}

	if !found {
		return "Clean compilation layout. Zero diagnostic errors detected.", nil
	}
	return out.String(), nil
}

func (self *Gopls) goplsCompletion(args map[string]string) (string, error) {
	inst, err := self.getActiveInstance()
	if err != nil {
		return "", err
	}
	absFile, line, char, err := parsePositionArgs(args, inst.path)
	if err != nil {
		return "", err
	}

	params := map[string]any{
		"textDocument": map[string]string{"uri": "file://" + absFile},
		"position":     map[string]int{"line": line, "character": char},
	}
	res, err := inst.sendRPC("textDocument/completion", params)
	if err != nil {
		return "", err
	}

	var response struct {
		Result json.RawMessage `json:"result"`
	}
	json.Unmarshal(res, &response)

	// Unmarshal list variations supported by LSP specifications
	var items []struct {
		Label         string `json:"label"`
		Detail        string `json:"detail"`
		Documentation string `json:"documentation"`
	}

	if len(response.Result) > 0 && response.Result[0] == '{' {
		var wrapped struct {
			Items []struct {
				Label         string `json:"label"`
				Detail        string `json:"detail"`
				Documentation string `json:"documentation"`
			} `json:"items"`
		}
		json.Unmarshal(response.Result, &wrapped)
		items = wrapped.Items
	} else {
		json.Unmarshal(response.Result, &items)
	}

	if len(items) == 0 {
		return "No smart context suggestions available.", nil
	}

	var out strings.Builder
	for _, it := range items {
		fmt.Fprintf(&out, "* %s %s (%s)\n", it.Label, it.Detail, it.Documentation)
	}
	return out.String(), nil
}

func (self *Gopls) goplsRename(args map[string]string) (string, error) {
	inst, err := self.getActiveInstance()
	if err != nil {
		return "", err
	}
	newName := args["new_name"]
	if newName == "" {
		return "", fmt.Errorf("new_name param missing")
	}
	absFile, line, char, err := parsePositionArgs(args, inst.path)
	if err != nil {
		return "", err
	}

	params := map[string]any{
		"textDocument": map[string]string{"uri": "file://" + absFile},
		"position":     map[string]int{"line": line, "character": char},
		"newName":      newName,
	}
	res, err := inst.sendRPC("textDocument/rename", params)
	if err != nil {
		return "", err
	}

	var response struct {
		Result struct {
			Changes map[string]json.RawMessage `json:"changes"`
		} `json:"result"`
	}
	json.Unmarshal(res, &response)

	if len(response.Result.Changes) == 0 {
		return "No occurrences required adjustment.", nil
	}

	var out strings.Builder
	out.WriteString("Proposed structural refactoring modifications:\n")
	for uri := range response.Result.Changes {
		fmt.Fprintf(&out, "- File requires modifications: %s\n", strings.TrimPrefix(uri, "file://"))
	}
	return out.String(), nil
}

func (self *Gopls) goplsFormat(args map[string]string) (string, error) {
	inst, err := self.getActiveInstance()
	if err != nil {
		return "", err
	}
	fileArg := args["file"]
	absFile := fileArg
	if !filepath.IsAbs(absFile) {
		absFile = filepath.Join(inst.path, fileArg)
	}

	params := map[string]any{
		"textDocument": map[string]string{"uri": "file://" + absFile},
		"options":      map[string]any{"tabSize": 4, "insertSpaces": false},
	}
	res, err := inst.sendRPC("textDocument/formatting", params)
	if err != nil {
		return "", err
	}

	var response struct {
		Result []struct {
			NewText string `json:"newText"`
		} `json:"result"`
	}
	json.Unmarshal(res, &response)

	if len(response.Result) == 0 {
		return "File layout is structurally pristine. Format redundant.", nil
	}

	return "Formatting corrections generated successfully. Use files execution workspace to overwrite file assets.", nil
}

// ---------------------------------------------------------------------------
// Internal System Utilities
// ---------------------------------------------------------------------------

func (self *Gopls) handleLocationQuery(method string, args map[string]string) (string, error) {
	inst, err := self.getActiveInstance()
	if err != nil {
		return "", err
	}
	absFile, line, char, err := parsePositionArgs(args, inst.path)
	if err != nil {
		return "", err
	}

	params := map[string]any{
		"textDocument": map[string]string{"uri": "file://" + absFile},
		"position":     map[string]int{"line": line, "character": char},
	}
	res, err := inst.sendRPC(method, params)
	if err != nil {
		return "", err
	}

	var response struct {
		Result json.RawMessage `json:"result"`
	}
	json.Unmarshal(res, &response)

	if string(response.Result) == "null" || len(response.Result) == 0 {
		return "Target location point could not be traced.", nil
	}

	var locs []lspLocation
	if response.Result[0] == '[' {
		json.Unmarshal(response.Result, &locs)
	} else {
		var single lspLocation
		if err := json.Unmarshal(response.Result, &single); err == nil {
			locs = append(locs, single)
		}
	}

	if len(locs) == 0 {
		return "No specific location structural targets map to current coordinates.", nil
	}

	return fmt.Sprintf("Target location traced:\nFile: %s\nLine: %d, Offset: %d",
		strings.TrimPrefix(locs[0].URI, "file://"), locs[0].Range.Start.Line+1, locs[0].Range.Start.Character), nil
}

func (self *Gopls) getActiveInstance() (*goplsInstance, error) {
	self.mu.RLock()
	path := self.activeWorkspacePath
	self.mu.RUnlock()

	if path == "" {
		return nil, fmt.Errorf("active workspace context missing. Call gopls_workspace_activate first")
	}

	self.mu.Lock()
	inst, exists := self.workspaces[path]
	if exists {
		inst.lastActive = time.Now()
	}
	self.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("workspace instance expired due to idle constraints. Re-run gopls_workspace_activate")
	}
	return inst, nil
}

func parsePositionArgs(args map[string]string, wsPath string) (string, int, int, error) {
	fileArg := args["file"]
	if fileArg == "" {
		return "", 0, 0, fmt.Errorf("file parameter required")
	}
	line, _ := strconv.Atoi(args["line"])
	char, _ := strconv.Atoi(args["character"])
	if line < 1 {
		return "", 0, 0, fmt.Errorf("line criteria must be 1-based format")
	}
	absFile := fileArg
	if !filepath.IsAbs(absFile) {
		absFile = filepath.Join(wsPath, fileArg)
	}
	return absFile, line - 1, char, nil
}

func (self *Gopls) getOrCreateWorkspace(path string) (*goplsInstance, error) {
	self.mu.Lock()
	defer self.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	if inst, exists := self.workspaces[absPath]; exists {
		inst.lastActive = time.Now()
		return inst, nil
	}

	if len(self.workspaces) >= self.config.MaxWorkspaces {
		var oldestPath string
		var oldestTime time.Time
		first := true
		for p, inst := range self.workspaces {
			if first || inst.lastActive.Before(oldestTime) {
				oldestTime = inst.lastActive
				oldestPath = p
				first = false
			}
		}
		if oldestPath != "" {
			log.Printf("[GOPLS] Max cap reached. Evicting oldest context: %s", oldestPath)
			self.workspaces[oldestPath].close()
			delete(self.workspaces, oldestPath)
		}
	}

	log.Printf("[GOPLS] Creating isolated language server session: %s", absPath)
	cmd := exec.Command(self.config.GoplsPath, "serve")
	cmd.Dir = absPath

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("gopls execution sequence aborted: %w", err)
	}

	inst := &goplsInstance{
		path:        absPath,
		cmd:         cmd,
		stdin:       stdin,
		lastActive:  time.Now(),
		pending:     make(map[int]chan []byte),
		diagnostics: make(map[string][]string),
	}

	go inst.readPump(stdout)

	initParams := map[string]any{
		"processId": 0,
		"rootPath":  absPath,
		"rootUri":   "file://" + absPath,
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"definition":     map[string]any{"dynamicRegistration": true},
				"references":     map[string]any{"dynamicRegistration": true},
				"implementation": map[string]any{"dynamicRegistration": true},
				"completion":     map[string]any{"completionItem": map[string]any{"documentationFormat": []string{"plaintext"}}},
				"formatting":     map[string]any{"dynamicRegistration": true},
				"rename":         map[string]any{"dynamicRegistration": true},
			},
		},
	}

	if _, err := inst.sendRPC("initialize", initParams); err != nil {
		inst.close()
		return nil, fmt.Errorf("gopls pipeline initialization handshake rejected: %w", err)
	}
	inst.sendNotification("initialized", map[string]any{})

	self.workspaces[absPath] = inst
	return inst, nil
}

func (inst *goplsInstance) readPump(r io.Reader) {
	reader := bufio.NewReader(r)
	for {
		var contentLength int
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if line == "\r\n" {
				break
			}
			if strings.HasPrefix(line, "Content-Length:") {
				fmt.Sscanf(line, "Content-Length: %d", &contentLength)
			}
		}
		if contentLength == 0 {
			continue
		}

		buf := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return
		}

		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(buf, &msg); err == nil {
			if msg.ID != nil && string(msg.ID) != "null" {
				var id int
				if err := json.Unmarshal(msg.ID, &id); err == nil {
					inst.mu.Lock()
					if ch, ok := inst.pending[id]; ok {
						select {
						case ch <- buf:
						default:
						}
					}
					inst.mu.Unlock()
				}
			} else if msg.Method == "textDocument/publishDiagnostics" {
				// Intercept asynchronous background diagnostics notifications sent by gopls
				var dParams struct {
					URI         string `json:"uri"`
					Diagnostics []struct {
						Severity int    `json:"severity"`
						Message  string `json:"message"`
						Range    struct {
							Start struct {
								Line int `json:"line"`
							} `json:"start"`
						} `json:"range"`
					} `json:"diagnostics"`
				}
				if err := json.Unmarshal(msg.Params, &dParams); err == nil {
					cleanURI := strings.TrimPrefix(dParams.URI, "file://")
					inst.mu.Lock()
					if len(dParams.Diagnostics) == 0 {
						delete(inst.diagnostics, cleanURI)
					} else {
						var issues []string
						for _, d := range dParams.Diagnostics {
							issues = append(issues, fmt.Sprintf(" -> Line %d: [Sev %d] %s", d.Range.Start.Line+1, d.Severity, d.Message))
						}
						inst.diagnostics[cleanURI] = issues
					}
					inst.mu.Unlock()
				}
			}
		}
	}
}

func (inst *goplsInstance) close() {
	inst.stdin.Close()
	if inst.cmd.Process != nil {
		inst.cmd.Process.Kill()
	}
}

func (self *Gopls) startJanitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		self.mu.Lock()
		now := time.Now()
		limit := time.Duration(self.config.IdleTimeoutMinutes) * time.Minute

		for path, inst := range self.workspaces {
			if now.Sub(inst.lastActive) > limit {
				log.Printf("[GOPLS] Janitor cleaning up idle process workspace: %s", path)
				inst.close()
				if self.activeWorkspacePath == path {
					self.activeWorkspacePath = ""
				}
				delete(self.workspaces, path)
			}
		}
		self.mu.Unlock()
	}
}
