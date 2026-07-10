package client

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ---------------------------------------------------------------------------
// Props — GET /props, POST /props
// ---------------------------------------------------------------------------

// Props retrieves server global properties (GET /props).
func (c *Client) Props(ctx context.Context) (*ServerProps, error) {
	resp, err := c.do(ctx, http.MethodGet, "/props", nil, nil, nil)
	if err != nil {
		return nil, err
	}
	var out ServerProps
	if err := decodeOK(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetProps changes server global properties (POST /props). Requires the server
// to be started with --props.
func (c *Client) SetProps(ctx context.Context, props any) error {
	resp, err := c.do(ctx, http.MethodPost, "/props", nil, props, nil)
	if err != nil {
		return err
	}
	return decodeOK(resp, nil)
}

// ServerProps is the response from GET /props.
type ServerProps struct {
	DefaultGenerationSettings map[string]any `json:"default_generation_settings"`
	TotalSlots                 int           `json:"total_slots"`
	ModelPath                  string        `json:"model_path"`
	ChatTemplate               string        `json:"chat_template"`
	ChatTemplateCaps           any           `json:"chat_template_caps,omitempty"`
	Modalities                 []string      `json:"modalities,omitempty"`
	IsSleeping                 bool          `json:"is_sleeping"`
}

// ---------------------------------------------------------------------------
// Slots — GET /slots, POST /slots/{id}?action=...
// ---------------------------------------------------------------------------

// Slots retrieves the current slot processing state (GET /slots).
func (c *Client) Slots(ctx context.Context, failOnNoSlot bool) ([]Slot, error) {
	q := url.Values{}
	if failOnNoSlot {
		q.Set("fail_on_no_slot", "1")
	}
	resp, err := c.do(ctx, http.MethodGet, "/slots", q, nil, nil)
	if err != nil {
		return nil, err
	}
	var out []Slot
	if err := decodeOK(resp, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SlotSave persists a slot's prompt cache to a file.
func (c *Client) SlotSave(ctx context.Context, idSlot int, filename string) error {
	return c.slotAction(ctx, idSlot, "save", filename)
}

// SlotRestore restores a slot's prompt cache from a file.
func (c *Client) SlotRestore(ctx context.Context, idSlot int, filename string) error {
	return c.slotAction(ctx, idSlot, "restore", filename)
}

// SlotErase erases a slot's prompt cache.
func (c *Client) SlotErase(ctx context.Context, idSlot int) error {
	return c.slotAction(ctx, idSlot, "erase", "")
}

func (c *Client) slotAction(ctx context.Context, idSlot int, action, filename string) error {
	q := url.Values{}
	q.Set("action", action)
	path := "/slots/" + itoa(idSlot)
	var body any
	if filename != "" {
		body = map[string]string{"filename": filename}
	}
	resp, err := c.do(ctx, http.MethodPost, path, q, body, nil)
	if err != nil {
		return err
	}
	return decodeOK(resp, nil)
}

// Slot describes a single server slot.
type Slot struct {
	ID              int    `json:"id"`
	State           string `json:"state"`
	Model           string `json:"model"`
	PromptTokens    int    `json:"prompt_tokens"`
	GeneratedTokens int    `json:"generated_tokens"`
}

// ---------------------------------------------------------------------------
// Metrics — GET /metrics
// ---------------------------------------------------------------------------

// Metrics retrieves the Prometheus-compatible metrics text (GET /metrics).
// Requires the server to be started with --metrics.
func (c *Client) Metrics(ctx context.Context) (string, error) {
	resp, err := c.do(ctx, http.MethodGet, "/metrics", nil, nil, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", &Error{Status: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ---------------------------------------------------------------------------
// LoRA adapters — GET /lora-adapters, POST /lora-adapters
// ---------------------------------------------------------------------------

// LoraAdapters retrieves the list of loaded LoRA adapters.
func (c *Client) LoraAdapters(ctx context.Context) ([]LoraAdapter, error) {
	resp, err := c.do(ctx, http.MethodGet, "/lora-adapters", nil, nil, nil)
	if err != nil {
		return nil, err
	}
	var out []LoraAdapter
	if err := decodeOK(resp, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SetLoraAdapters sets the global LoRA adapter scales.
func (c *Client) SetLoraAdapters(ctx context.Context, adapters []LoraAdapter) error {
	resp, err := c.do(ctx, http.MethodPost, "/lora-adapters", nil, adapters, nil)
	if err != nil {
		return err
	}
	return decodeOK(resp, nil)
}

// LoraAdapter describes a single LoRA adapter and its scale.
type LoraAdapter struct {
	ID    int     `json:"id"`
	Scale float64 `json:"scale"`
}

// ---------------------------------------------------------------------------
// Chat control — POST /v1/chat/completions/control
// ---------------------------------------------------------------------------

// ChatControl sends a realtime reasoning control action for an in-flight
// completion. action is typically "stop" or "continue".
func (c *Client) ChatControl(ctx context.Context, id string, action string) error {
	body := map[string]string{
		"id":     id,
		"action": action,
	}
	resp, err := c.do(ctx, http.MethodPost, "/v1/chat/completions/control", nil, body, nil)
	if err != nil {
		return err
	}
	return decodeOK(resp, nil)
}
