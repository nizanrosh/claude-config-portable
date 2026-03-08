package payload

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodeDecode_RoundTrip(t *testing.T) {
	t.Parallel()

	bundle := &ConfigBundle{
		Version:         SchemaVersion,
		CreatedAt:       "2026-03-08T00:00:00Z",
		Platform:        "darwin",
		SecretsIncluded: false,
		Settings:        json.RawMessage(`{"model":"opus"}`),
		Plugins: PluginManifest{
			Version: 1,
			Plugins: map[string][]PluginInstall{
				"test-plugin": {{Scope: "user", Version: "1.0.0"}},
			},
		},
		Marketplaces: json.RawMessage(`[]`),
		MCPConfigs:   map[string]json.RawMessage{"test": json.RawMessage(`{"type":"stdio"}`)},
		Skills: []SkillEntry{
			{Name: "my-skill", Files: map[string]string{"index.md": "# Hello"}},
		},
	}

	encoded, err := Encode(bundle)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	if !strings.HasPrefix(encoded, "ccfg:1:") {
		t.Fatalf("encoded string missing prefix, got: %s", encoded[:20])
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
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
	if len(decoded.Plugins.Plugins) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(decoded.Plugins.Plugins))
	}
	if len(decoded.Skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(decoded.Skills))
	}
	if decoded.Skills[0].Name != "my-skill" {
		t.Errorf("skill name mismatch: got %q", decoded.Skills[0].Name)
	}
	if decoded.Skills[0].Files["index.md"] != "# Hello" {
		t.Errorf("skill file content mismatch: got %q", decoded.Skills[0].Files["index.md"])
	}
}

func TestDecode_InvalidPrefix(t *testing.T) {
	t.Parallel()
	_, err := Decode("invalid:data")
	if err == nil {
		t.Fatal("expected error for invalid prefix")
	}
}

func TestDecode_UnsupportedVersion(t *testing.T) {
	t.Parallel()
	_, err := Decode("ccfg:99:data")
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestDecode_InvalidBase64(t *testing.T) {
	t.Parallel()
	_, err := Decode("ccfg:1:not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}
