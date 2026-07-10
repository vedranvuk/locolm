package exec

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"time"

	"github.com/vedranvuk/locolm/internal/mcp"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

// Config holds all configuration for the exec tool.
type Config struct {
	// AllowedCommands is a list of regex patterns. If non-empty, each command
	// must match at least one pattern to be allowed. If empty, all commands are allowed.
	AllowedCommands []string `json:"allowed_commands"`
	// TimeoutSec is the default command execution timeout in seconds.
	TimeoutSec int `json:"timeout_sec"`
	// MaxOutputBytes caps the combined stdout/stderr output.
	MaxOutputBytes int `json:"max_output_bytes"`
}

func DefaultConfig() *Config {
	return &Config{
		TimeoutSec:     30,
		MaxOutputBytes: 102400, // 100 KB
	}
}

// ---------------------------------------------------------------------------
// Tool
// ---------------------------------------------------------------------------

type ExecTool struct {
	config *Config
	allowedPatterns []*regexp.Regexp
}

func New(config *Config) (*ExecTool, error) {
	if config == nil {
		config = DefaultConfig()
	}
	tool := &ExecTool{config: config}
	tool.compilePatterns()
	return tool, nil
}

func (self *ExecTool) compilePatterns() {
	self.allowedPatterns = nil
	for _, pattern := range self.config.AllowedCommands {
		re, err := regexp.Compile(pattern)
		if err != nil {
			log.Printf("[EXEC] Warning: invalid allowed_commands regex %q: %v", pattern, err)
			continue
		}
		self.allowedPatterns = append(self.allowedPatterns, re)
	}
	log.Printf("[EXEC] Config loaded: timeout=%d, max_output=%d, allowed_patterns=%d",
		self.config.TimeoutSec, self.config.MaxOutputBytes, len(self.allowedPatterns))
}

func (self *ExecTool) Register(r mcp.Registry) {
	r.RegisterTool(
		"fs_run",
		"Execute a command and capture its output. Runs via cmd /C on Windows. Commands may be restricted by the allowed_commands config.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {
					"type": "string",
					"description": "The command to execute (e.g. 'dir', 'git status', 'python script.py')"
				},
				"timeout": {
					"type": "string",
					"description": "Optional timeout in seconds (default from config)"
				}
			},
			"required": ["command"]
		}`),
		self.runCommand,
	)
}

// ---------------------------------------------------------------------------
// Security check
// ---------------------------------------------------------------------------

func (self *ExecTool) isCommandAllowed(command string) bool {
	if len(self.allowedPatterns) == 0 {
		return true // no restrictions
	}
	for _, re := range self.allowedPatterns {
		if re.MatchString(command) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// fs_run
// ---------------------------------------------------------------------------

func (self *ExecTool) runCommand(args map[string]string) (string, error) {
	command := args["command"]
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	if !self.isCommandAllowed(command) {
		return "", fmt.Errorf("command not allowed by config: %q", command)
	}

	timeoutSec := self.config.TimeoutSec
	if v := args["timeout"]; v != "" {
		if d, err := time.ParseDuration(v + "s"); err == nil {
			timeoutSec = int(d.Seconds())
		}
	}

	cmd := exec.Command("cmd", "/C", command)

	done := make(chan error, 1)
	var stdout, stderr []byte
	cmd.Stdout = &writeCollector{}
	cmd.Stderr = &writeCollector{}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		stdout = cmd.Stdout.(*writeCollector).bytes()
		stderr = cmd.Stderr.(*writeCollector).bytes()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}
		return self.formatOutput(exitCode, stdout, stderr, false), nil
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		cmd.Process.Kill()
		return self.formatOutput(-1, stdout, stderr, true), nil
	}
}

func (self *ExecTool) formatOutput(exitCode int, stdout, stderr []byte, timedOut bool) string {
	maxOut := self.config.MaxOutputBytes
	if len(stdout) > maxOut {
		stdout = stdout[:maxOut]
		stdout = append(stdout, []byte("\n...[truncated]")...)
	}
	if len(stderr) > maxOut {
		stderr = stderr[:maxOut]
		stderr = append(stderr, []byte("\n...[truncated]")...)
	}

	out := map[string]interface{}{
		"exit_code": exitCode,
		"stdout":    string(stdout),
		"stderr":    string(stderr),
	}
	if timedOut {
		out["timed_out"] = true
	}

	b, _ := json.Marshal(out)
	return string(b)
}

type writeCollector struct {
	data []byte
}

func (w *writeCollector) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	return len(p), nil
}

func (w *writeCollector) bytes() []byte {
	return w.data
}
