package secrets

import (
	"encoding/json"
	"testing"
)

func TestFilterMCPConfig_StripsHeaders(t *testing.T) {
	t.Parallel()
	input := json.RawMessage(`{"type":"sse","url":"https://api.example.com","headers":{"Authorization":"Bearer secret123"}}`)

	filtered, err := FilterMCPConfig(input)
	if err != nil {
		t.Fatalf("FilterMCPConfig failed: %v", err)
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(filtered, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	var headers string
	if err := json.Unmarshal(obj["headers"], &headers); err != nil {
		t.Fatalf("headers should be a string: %v", err)
	}
	if headers != redactedPlaceholder {
		t.Errorf("headers not redacted: got %q", headers)
	}

	// type should be preserved
	var typ string
	json.Unmarshal(obj["type"], &typ)
	if typ != "sse" {
		t.Errorf("type not preserved: got %q", typ)
	}
}

func TestFilterMCPConfig_StripsEnvAndOAuth(t *testing.T) {
	t.Parallel()
	input := json.RawMessage(`{"type":"stdio","command":"node","env":{"API_KEY":"secret"},"oauth":{"token":"secret"}}`)

	filtered, err := FilterMCPConfig(input)
	if err != nil {
		t.Fatalf("FilterMCPConfig failed: %v", err)
	}

	var obj map[string]json.RawMessage
	json.Unmarshal(filtered, &obj)

	for _, key := range []string{"env", "oauth"} {
		var val string
		if err := json.Unmarshal(obj[key], &val); err != nil {
			t.Fatalf("%s should be a string: %v", key, err)
		}
		if val != redactedPlaceholder {
			t.Errorf("%s not redacted: got %q", key, val)
		}
	}

	// command should be preserved
	var cmd string
	json.Unmarshal(obj["command"], &cmd)
	if cmd != "node" {
		t.Errorf("command not preserved: got %q", cmd)
	}
}

func TestFilterMCPConfig_StripsURLQueryParams(t *testing.T) {
	t.Parallel()
	input := json.RawMessage(`{"type":"sse","url":"https://api.example.com/v1?token=secret&key=abc"}`)

	filtered, err := FilterMCPConfig(input)
	if err != nil {
		t.Fatalf("FilterMCPConfig failed: %v", err)
	}

	var obj map[string]json.RawMessage
	json.Unmarshal(filtered, &obj)

	var url string
	json.Unmarshal(obj["url"], &url)
	if url != "https://api.example.com/v1" {
		t.Errorf("URL query params not stripped: got %q", url)
	}
}

func TestFilterMCPConfig_PreservesCleanConfig(t *testing.T) {
	t.Parallel()
	input := json.RawMessage(`{"type":"stdio","command":"npx","args":["server"]}`)

	filtered, err := FilterMCPConfig(input)
	if err != nil {
		t.Fatalf("FilterMCPConfig failed: %v", err)
	}

	// Should be unchanged
	if string(filtered) != string(input) {
		t.Errorf("clean config was modified:\n  got:  %s\n  want: %s", filtered, input)
	}
}

func TestHasRedactedValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		json string
		want bool
	}{
		{
			name: "redacted headers",
			json: `{"headers":"__CONFIGURE_AFTER_IMPORT__"}`,
			want: true,
		},
		{
			name: "no redaction",
			json: `{"type":"stdio","command":"node"}`,
			want: false,
		},
		{
			name: "redacted env",
			json: `{"env":"__CONFIGURE_AFTER_IMPORT__"}`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := HasRedactedValues(json.RawMessage(tt.json))
			if got != tt.want {
				t.Errorf("HasRedactedValues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterMCPServers_NestedServers(t *testing.T) {
	t.Parallel()
	input := json.RawMessage(`{"mcpServers":{"my-server":{"type":"sse","url":"https://example.com?key=secret","headers":{"Auth":"Bearer tok"}}}}`)

	filtered, err := FilterMCPServers(input)
	if err != nil {
		t.Fatalf("FilterMCPServers failed: %v", err)
	}

	var obj map[string]map[string]map[string]json.RawMessage
	json.Unmarshal(filtered, &obj)

	server := obj["mcpServers"]["my-server"]
	var headers string
	json.Unmarshal(server["headers"], &headers)
	if headers != redactedPlaceholder {
		t.Errorf("nested headers not redacted: got %q", headers)
	}

	var url string
	json.Unmarshal(server["url"], &url)
	if url != "https://example.com" {
		t.Errorf("nested URL query params not stripped: got %q", url)
	}
}
