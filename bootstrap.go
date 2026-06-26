package main

import (
	"log"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"
)

const llamaURL = "http://127.0.0.1:11500"

var llamaProcess *os.Process

// Bootstrap starts llama-server, waits for it to be ready, then launches the browser.
func Bootstrap() {
	llamaCmd := os.Getenv("LOCOLM_BOOTSTRAP_LLAMA_SERVER_COMMAND")
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
	llamaProcess = llama.Process

	log.Printf("[BOOTSTRAP] Waiting for llama-server to be ready...")
	if !waitForServer(llamaURL+"/health", 120*time.Second) {
		llamaProcess.Kill()
		log.Fatalf("[BOOTSTRAP] llama-server did not become ready in time")
	}

	browserCmd := os.Getenv("LOCOLM_BOOTSTRAP_BROWSER_COMMAND")
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
	if llamaProcess == nil {
		return
	}
	log.Printf("[BOOTSTRAP] Stopping llama-server (PID %d)...", llamaProcess.Pid)
	llamaProcess.Signal(syscall.SIGTERM)
	llamaProcess.Wait()
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
