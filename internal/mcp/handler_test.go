package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// sanitizeRawJSON — unit tests
// ---------------------------------------------------------------------------

func TestSanitizeRawJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		// want is the exact sanitized output.
		want string
		// valid reports whether the sanitized output is expected to be valid
		// JSON. Cases that are intentionally invalid/non-JSON (e.g. a trailing
		// backslash, an unescaped quote) are left as-is and must NOT be valid.
		valid bool
	}{
		// --- valid JSON passes through unchanged ---
		{
			name:  "valid json object unchanged",
			input: `{"query":"SELECT * WHERE { ?s ?p ?o }"}`,
			want:  `{"query":"SELECT * WHERE { ?s ?p ?o }"}`,
			valid: true,
		},
		{
			name:  "valid json with escaped sequences unchanged",
			input: `{"a":"line1\nline2","b":"tab\there","c":"\t\n\r"}`,
			want:  `{"a":"line1\nline2","b":"tab\there","c":"\t\n\r"}`,
			valid: true,
		},
		{
			name:  "valid json array unchanged",
			input: `[1,2,3,"x"]`,
			want:  `[1,2,3,"x"]`,
			valid: true,
		},
		{
			name:  "empty input unchanged",
			input: ``,
			want:  ``,
			valid: false,
		},
		{
			name:  "non json text unchanged",
			input: `just some plain text with no json`,
			want:  `just some plain text with no json`,
			valid: false,
		},

		// --- literal control chars inside strings get escaped ---
		{
			name:  "literal newline inside string",
			input: "{\"q\":\"line1\nline2\"}",
			want:  "{\"q\":\"line1\\nline2\"}",
			valid: true,
		},
		{
			name:  "literal carriage return inside string",
			input: "{\"q\":\"a\rb\"}",
			want:  "{\"q\":\"a\\rb\"}",
			valid: true,
		},
		{
			name:  "literal tab inside string",
			input: "{\"q\":\"a\tb\"}",
			want:  "{\"q\":\"a\\tb\"}",
			valid: true,
		},
		{
			name:  "literal backspace inside string",
			input: "{\"q\":\"a\bb\"}",
			want:  "{\"q\":\"a\\bb\"}",
			valid: true,
		},
		{
			name:  "literal form feed inside string",
			input: "{\"q\":\"a\fb\"}",
			want:  "{\"q\":\"a\\fb\"}",
			valid: true,
		},
		{
			name:  "literal vertical tab (0x0b) inside string",
			input: "{\"q\":\"a\x0bb\"}",
			want:  "{\"q\":\"a\\u000bb\"}",
			valid: true,
		},
		{
			name:  "literal null (0x00) inside string",
			input: "{\"q\":\"a\x00b\"}",
			want:  "{\"q\":\"a\\u0000b\"}",
			valid: true,
		},
		{
			name:  "multiple control chars inside string",
			input: "{\"q\":\"a\nb\rc\td\"}",
			want:  "{\"q\":\"a\\nb\\rc\\td\"}",
			valid: true,
		},
		{
			name:  "control char at string start",
			input: "{\"q\":\"\nstart\"}",
			want:  "{\"q\":\"\\nstart\"}",
			valid: true,
		},
		{
			name:  "control char at string end",
			input: "{\"q\":\"end\n\"}",
			want:  "{\"q\":\"end\\n\"}",
			valid: true,
		},

		// --- Windows paths / double-escaped sequences preserved ---
		{
			name:  "windows path double backslash preserved",
			input: `{"p":"C:\\new\\temp"}`,
			want:  `{"p":"C:\\new\\temp"}`,
			valid: true,
		},
		{
			name:  "double backslash then n preserved",
			input: `{"p":"D:\\root\\nope"}`,
			want:  `{"p":"D:\\root\\nope"}`,
			valid: true,
		},
		{
			name:  "single backslash before quote preserved",
			input: `{"p":"ends with \\"}`,
			want:  `{"p":"ends with \\"}`,
			valid: true,
		},

		// --- malformed escapes normalized ---
		{
			name:  "malformed escape backslash then raw newline",
			input: "{\"q\":\"a\\" + "\n" + "b\"}",
			want:  "{\"q\":\"a\\" + "\\n" + "b\"}",
			valid: true,
		},
		{
			name:  "malformed escape backslash then raw tab",
			input: "{\"q\":\"a\\" + "\t" + "b\"}",
			want:  "{\"q\":\"a\\" + "\\t" + "b\"}",
			valid: true,
		},
		{
			name:  "backslash as final byte is preserved",
			input: "{\"q\":\"trailing \\",
			want:  "{\"q\":\"trailing \\",
			valid: false,
		},

		// --- control chars OUTSIDE strings are left untouched ---
		{
			name:  "newline outside string preserved",
			input: "{\n  \"q\":\"ok\"\n}",
			want:  "{\n  \"q\":\"ok\"\n}",
			valid: true,
		},
		{
			name:  "tab outside string preserved",
			input: "{\t\"q\":\"ok\"\t}",
			want:  "{\t\"q\":\"ok\"\t}",
			valid: true,
		},

		// --- string state tracking ---
		{
			name:  "quote inside string does not end string when escaped",
			input: `{"q":"he said \"hi\" there"}`,
			want:  `{"q":"he said \"hi\" there"}`,
			valid: true,
		},
		{
			name:  "literal quote inside string left as-is",
			input: "{\"q\":\"a\"b\"}",
			want:  "{\"q\":\"a\"b\"}",
			valid: false,
		},
		{
			name:  "multiple strings each sanitized independently",
			input: "{\"a\":\"x\ny\",\"b\":\"p\tq\"}",
			want:  "{\"a\":\"x\\ny\",\"b\":\"p\\tq\"}",
			valid: true,
		},
		{
			name:  "empty string value",
			input: `{"a":""}`,
			want:  `{"a":""}`,
			valid: true,
		},

		// --- unicode / multibyte content preserved ---
		{
			name:  "utf8 multibyte preserved",
			input: `{"q":"café — 日本語 🚀"}`,
			want:  `{"q":"café — 日本語 🚀"}`,
			valid: true,
		},
		{
			name:  "control char after multibyte preserved correctly",
			input: "{\"q\":\"café\n日本語\"}",
			want:  "{\"q\":\"café\\n日本語\"}",
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(sanitizeRawJSON([]byte(tt.input)))
			if got != tt.want {
				t.Fatalf("sanitizeRawJSON() =\n  %q\nwant\n  %q", got, tt.want)
			}
			if tt.valid != json.Valid([]byte(got)) {
				t.Fatalf("json.Valid(%q) = %v, want %v", got, json.Valid([]byte(got)), tt.valid)
			}
		})
	}
}

// TestSanitizeRawJSON_OutputParsesToSameStringValues verifies that after
// sanitization, the string values decode back to the intended content (with
// the control characters intact as actual characters).
func TestSanitizeRawJSON_OutputParsesToSameStringValues(t *testing.T) {
	cases := []struct {
		input   string
		wantVal string
	}{
		{"{\"q\":\"a\nb\"}", "a\nb"},
		{"{\"q\":\"a\tb\"}", "a\tb"},
		{"{\"q\":\"a\rb\"}", "a\rb"},
		{"{\"q\":\"a\bb\"}", "a\bb"},
		{"{\"q\":\"a\fb\"}", "a\fb"},
		{"{\"q\":\"a\x00b\"}", "a\x00b"},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			sanitized := sanitizeRawJSON([]byte(c.input))
			var m map[string]string
			if err := json.Unmarshal(sanitized, &m); err != nil {
				t.Fatalf("unmarshal sanitized: %v", err)
			}
			if m["q"] != c.wantVal {
				t.Fatalf("decoded value = %q, want %q", m["q"], c.wantVal)
			}
		})
	}
}

// TestSanitizeRawJSON_Idempotent verifies that sanitizing already-valid JSON
// (including previously-escaped control chars) is a no-op on the second pass.
func TestSanitizeRawJSON_Idempotent(t *testing.T) {
	inputs := []string{
		`{"q":"line1\nline2"}`,
		`{"p":"C:\\new"}`,
		`{"q":"a\tb\rc"}`,
		`{"q":"unicode — 🚀"}`,
	}
	for _, in := range inputs {
		first := sanitizeRawJSON([]byte(in))
		second := sanitizeRawJSON(first)
		if string(first) != string(second) {
			t.Fatalf("not idempotent for %q:\n first=%q\nsecond=%q", in, first, second)
		}
	}
}

// TestSanitizeRawJSON_NoDoubleEscape verifies that an already-escaped sequence
// is never turned into a double escape (e.g. \\n must not become \\\\n).
func TestSanitizeRawJSON_NoDoubleEscape(t *testing.T) {
	input := `{"p":"C:\\new\\temp","q":"a\nb"}`
	got := string(sanitizeRawJSON([]byte(input)))
	if strings.Contains(got, `\\\\`) {
		t.Fatalf("produced double backslash escape: %q", got)
	}
	if got != input {
		t.Fatalf("got %q, want %q", got, input)
	}
}

// ---------------------------------------------------------------------------
// RegisterTool argument parsing — integration tests
// ---------------------------------------------------------------------------

// parseArgs mirrors the exact decode path used inside RegisterTool, so the
// tests exercise the same logic the server applies to raw arguments. A nil
// payload (no arguments) yields an empty map, matching the handler's guard.
func parseArgs(raw []byte) (map[string]string, error) {
	parsed := make(map[string]string)
	if raw == nil {
		return parsed, nil
	}
	sanitized := sanitizeRawJSON(raw)
	var rawArgs map[string]json.RawMessage
	if err := json.Unmarshal(sanitized, &rawArgs); err != nil {
		return nil, err
	}
	for k, v := range rawArgs {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			parsed[k] = s
		} else {
			parsed[k] = string(v)
		}
	}
	return parsed, nil
}

// TestRegisterTool_ParsesStringArgs verifies that a normal JSON object with
// string values is parsed into the args map.
func TestRegisterTool_ParsesStringArgs(t *testing.T) {
	parsed, err := parseArgs([]byte(`{"name":"vedran","role":"admin"}`))
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if parsed["name"] != "vedran" || parsed["role"] != "admin" {
		t.Fatalf("parsed = %v", parsed)
	}
}

// TestRegisterTool_NonStringArgsStoredAsRawJSON verifies that non-string values
// are stored as their raw JSON text (the documented flattening behavior).
func TestRegisterTool_NonStringArgsStoredAsRawJSON(t *testing.T) {
	parsed, err := parseArgs([]byte(`{"count":42,"flag":true,"list":[1,2,3]}`))
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if parsed["count"] != "42" {
		t.Fatalf("count = %q, want 42", parsed["count"])
	}
	if parsed["flag"] != "true" {
		t.Fatalf("flag = %q, want true", parsed["flag"])
	}
	if parsed["list"] != "[1,2,3]" {
		t.Fatalf("list = %q, want [1,2,3]", parsed["list"])
	}
}

// TestRegisterTool_LiteralNewlineInArgIsParsed verifies the end-to-end path:
// a raw newline inside a string argument is sanitized and the value decodes
// with the newline intact.
func TestRegisterTool_LiteralNewlineInArgIsParsed(t *testing.T) {
	parsed, err := parseArgs([]byte("{\"text\":\"line1\nline2\"}"))
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if parsed["text"] != "line1\nline2" {
		t.Fatalf("text = %q, want %q", parsed["text"], "line1\nline2")
	}
}

// TestRegisterTool_EmptyArguments verifies that a nil/empty arguments payload
// yields an empty args map without error.
func TestRegisterTool_EmptyArguments(t *testing.T) {
	parsed, err := parseArgs(nil)
	if err != nil {
		t.Fatalf("parseArgs(nil): %v", err)
	}
	if len(parsed) != 0 {
		t.Fatalf("expected empty map, got %v", parsed)
	}
}

// TestRegisterTool_InvalidJSONStillFails confirms that structurally broken JSON
// (not just control chars) still returns an error after sanitization, so the
// tool handler's error path is reachable.
func TestRegisterTool_InvalidJSONStillFails(t *testing.T) {
	_, err := parseArgs([]byte(`{"name":`))
	if err == nil {
		t.Fatal("expected error for structurally invalid JSON, got nil")
	}
}
