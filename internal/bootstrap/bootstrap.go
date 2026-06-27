package bootstrap

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"
)

const llamaURL = "http://127.0.0.1:11500"

// LlamaProcess holds the handle for the llama-server process so we can
// gracefully shut it down on exit.
var LlamaProcess *os.Process

// Bootstrap starts llama-server, waits for it to be ready, then launches
// the browser.
func Bootstrap(llamaCmd, browserCmd string) {
	if llamaCmd == "" {
		log.Fatalf("[BOOTSTRAP] llama_server_command is required in locolm.json")
	}

	log.Printf("[BOOTSTRAP] Starting llama-server...")
	llama := exec.Command("cmd", "/C", llamaCmd)
	llama.Stdout = os.Stdout
	llama.Stderr = os.Stderr
	llama.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	if err := llama.Start(); err != nil {
		log.Fatalf("[BOOTSTRAP] Failed to start llama-server: %v", err)
	}
	LlamaProcess = llama.Process

	log.Printf("[BOOTSTRAP] Waiting for llama-server to be ready...")
	if !waitForServer(llamaURL+"/health", 120*time.Second) {
		LlamaProcess.Kill()
		log.Fatalf("[BOOTSTRAP] llama-server did not become ready in time")
	}

	log.Printf("[BOOTSTRAP] Waiting for model to be loaded...")
	if !waitForModelReady(llamaURL, 120*time.Second) {
		LlamaProcess.Kill()
		log.Fatalf("[BOOTSTRAP] model did not become ready in time")
	}

	if browserCmd == "" {
		log.Fatalf("[BOOTSTRAP] browser_command is required in locolm.json")
	}

	log.Printf("[BOOTSTRAP] Starting browser...")
	browser := exec.Command("cmd", "/C", browserCmd, "--app="+llamaURL)
	browser.Stdout = os.Stdout
	browser.Stderr = os.Stderr
	if err := browser.Start(); err != nil {
		log.Printf("[BOOTSTRAP] Failed to start browser: %v", err)
	}

	log.Printf("[BOOTSTRAP] All services started.")
}

// StopLlama sends SIGTERM to llama-server and waits for it to exit.
func StopLlama() {
	if LlamaProcess == nil {
		return
	}
	log.Printf("[BOOTSTRAP] Stopping llama-server (PID %d)...", LlamaProcess.Pid)
	LlamaProcess.Signal(syscall.SIGTERM)
	LlamaProcess.Wait()
	log.Printf("[BOOTSTRAP] llama-server stopped.")
}

func waitForServer(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// waitForModelReady sends a minimal chat completion request and waits for a
// successful response. This confirms the model is fully loaded and the
// inference engine is ready, not just that the HTTP server is listening.
func waitForModelReady(baseURL string, timeout time.Duration) bool {
	probe := map[string]interface{}{
		"model":    "unused",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
		"max_tokens": 1,
		"stream":     false,
	}
	body, _ := json.Marshal(probe)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
		if err == nil {
			respBody, ioErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if ioErr == nil && resp.StatusCode == http.StatusOK {
				// Verify the response is a valid chat completion, not an error.
				var result struct {
					Choices []struct {
						Message struct {
							Content string `json:"content"`
						} `json:"message"`
					} `json:"choices"`
				}
				if json.Unmarshal(respBody, &result) == nil && len(result.Choices) > 0 {
					return true
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
	return false
}
