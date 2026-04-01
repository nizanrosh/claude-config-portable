package payload

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCursorEncodeDecode_RoundTrip(t *testing.T) {
	t.Parallel()

	bundle := &CursorConfigBundle{
		Version:         CursorSchemaVersion,
		CreatedAt:       "2026-03-30T00:00:00Z",
		Platform:        "darwin",
		SecretsIncluded: false,
		Settings:        json.RawMessage(`{"editor.fontSize":16}`),
		Keybindings:     json.RawMessage(`[{"key":"cmd+i","command":"test"}]`),
		Rules: []RuleEntry{
			{Name: "test.mdc", Content: "---\ndescription: test\n---\nrule content"},
		},
		MCPConfig:  json.RawMessage(`{"mcpServers":{"test":{"command":"echo"}}}`),
		Extensions: json.RawMessage(`[{"identifier":{"id":"ext.test"},"version":"1.0.0"}]`),
		Skills: []SkillEntry{
			{Name: "my-skill", Files: map[string]string{"SKILL.md": "# Skill"}},
		},
		Commands: []CommandEntry{
			{Name: "DoSomething.md", Content: "# Command\nDo the thing."},
		},
		CLIConfig: json.RawMessage(`{"version":1}`),
	}

	encoded, err := EncodeCursor(bundle)
	if err != nil {
		t.Fatalf("EncodeCursor failed: %v", err)
	}

	if !strings.HasPrefix(encoded, "ccur:1:") {
		t.Fatalf("encoded string missing prefix, got: %s", encoded[:20])
	}

	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeCursor failed: %v", err)
	}

	if decoded.Version != bundle.Version {
		t.Errorf("version mismatch: got %d, want %d", decoded.Version, bundle.Version)
	}
	if decoded.Platform != bundle.Platform {
		t.Errorf("platform mismatch: got %q, want %q", decoded.Platform, bundle.Platform)
	}
	if decoded.SecretsIncluded != bundle.SecretsIncluded {
		t.Errorf("secretsIncluded mismatch: got %v, want %v", decoded.SecretsIncluded, bundle.SecretsIncluded)
	}
	if len(decoded.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(decoded.Rules))
	}
	if decoded.Rules[0].Name != "test.mdc" {
		t.Errorf("rule name mismatch: got %q", decoded.Rules[0].Name)
	}
	if len(decoded.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(decoded.Skills))
	}
	if decoded.Skills[0].Files["SKILL.md"] != "# Skill" {
		t.Errorf("skill file content mismatch: got %q", decoded.Skills[0].Files["SKILL.md"])
	}
	if len(decoded.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(decoded.Commands))
	}
	if decoded.Commands[0].Name != "DoSomething.md" {
		t.Errorf("command name mismatch: got %q", decoded.Commands[0].Name)
	}
	if string(decoded.CLIConfig) != `{"version":1}` {
		t.Errorf("CLIConfig mismatch: got %q", string(decoded.CLIConfig))
	}
}

func TestDecodeCursor_InvalidPrefix(t *testing.T) {
	t.Parallel()
	_, err := DecodeCursor("ccfg:1:data")
	if err == nil {
		t.Fatal("expected error for wrong prefix (ccfg instead of ccur)")
	}
}

func TestDecodeCursor_UnsupportedVersion(t *testing.T) {
	t.Parallel()
	_, err := DecodeCursor("ccur:99:data")
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestDecodeCursor_InvalidBase64(t *testing.T) {
	t.Parallel()
	_, err := DecodeCursor("ccur:1:not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}
