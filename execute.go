package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

const maxOutputBytes = 102400 // 100 KB

func runCommand(args map[string]string) (string, error) {
	command := args["command"]
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	timeoutSec := 30
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
	if len(stdout) > maxOutputBytes {
		stdout = stdout[:maxOutputBytes]
		stdout = append(stderr, []byte("\n...[truncated]")...)
	}
	if len(stderr) > maxOutputBytes {
		stderr = stderr[:maxOutputBytes]
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
