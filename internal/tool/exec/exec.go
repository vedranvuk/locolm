package exec

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"time"

	"github.com/vedranvuk/locolm/internal/tool"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

// ExecConfig holds all configuration for the exec tool.
type ExecConfig struct {
	// AllowedCommands is a list of regex patterns. If non-empty, each command
	// must match at least one pattern to be allowed. If empty, all commands are allowed.
	AllowedCommands []string `json:"allowed_commands"`
	// TimeoutSec is the default command execution timeout in seconds.
	TimeoutSec int `json:"timeout_sec"`
	// MaxOutputBytes caps the combined stdout/stderr output.
	MaxOutputBytes int `json:"max_output_bytes"`
}

// execCfg is the package-level config with safe defaults.
var execCfg = ExecConfig{
	TimeoutSec:     30,
	MaxOutputBytes: 102400, // 100 KB
}

var execAllowedPatterns []*regexp.Regexp

func init() {
	// Register config loader
	tool.RegisterConfig("exec", func(raw json.RawMessage) error {
		if len(raw) == 0 {
			return nil
		}
		if err := json.Unmarshal(raw, &execCfg); err != nil {
			return err
		}
		// Compile allowed command patterns
		execAllowedPatterns = nil
		for _, pattern := range execCfg.AllowedCommands {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return fmt.Errorf("invalid allowed_commands regex %q: %w", pattern, err)
			}
			execAllowedPatterns = append(execAllowedPatterns, re)
		}
		return nil
	})

	// Register tool
	tool.Register("fs_run", tool.Tool{
		Name:        "fs_run",
		Description: "Execute a command and capture its output. Runs via cmd /C on Windows. Commands may be restricted by the allowed_commands config.",
		InputSchema: json.RawMessage(`{
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
		Func: runCommand,
	})
}

// ---------------------------------------------------------------------------
// Security check
// ---------------------------------------------------------------------------

func isCommandAllowed(command string) bool {
	if len(execAllowedPatterns) == 0 {
		return true // no restrictions
	}
	for _, re := range execAllowedPatterns {
		if re.MatchString(command) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// fs_run
// ---------------------------------------------------------------------------

func runCommand(args map[string]string) (string, error) {
	command := args["command"]
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	if !isCommandAllowed(command) {
		return "", fmt.Errorf("command not allowed by config: %q", command)
	}

	timeoutSec := execCfg.TimeoutSec
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
		return formatOutput(exitCode, stdout, stderr, false), nil
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		cmd.Process.Kill()
		return formatOutput(-1, stdout, stderr, true), nil
	}
}

func formatOutput(exitCode int, stdout, stderr []byte, timedOut bool) string {
	maxOut := execCfg.MaxOutputBytes
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
		"stdout":   string(stdout),
		"stderr":   string(stderr),
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
