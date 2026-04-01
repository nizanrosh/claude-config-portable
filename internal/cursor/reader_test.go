package cursor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nizanrosh/claude-config-portable/internal/payload"
)

func TestStripJSONComments_LineComments(t *testing.T) {
	t.Parallel()
	input := []byte("// top comment\n[{\"key\": \"value\"}]\n")
	got := stripJSONComments(input)

	var arr []map[string]string
	if err := json.Unmarshal(got, &arr); err != nil {
		t.Fatalf("result is not valid JSON: %v\nraw: %s", err, string(got))
	}
	if arr[0]["key"] != "value" {
		t.Errorf("unexpected value: %q", arr[0]["key"])
	}
}

func TestStripJSONComments_InlineComments(t *testing.T) {
	t.Parallel()
	input := []byte(`{
  "a": 1, // inline comment
  "b": 2
}`)
	got := stripJSONComments(input)

	var obj map[string]int
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("result is not valid JSON: %v\nraw: %s", err, string(got))
	}
	if obj["a"] != 1 || obj["b"] != 2 {
		t.Errorf("unexpected values: %v", obj)
	}
}

func TestStripJSONComments_BlockComments(t *testing.T) {
	t.Parallel()
	input := []byte(`{
  /* block comment */
  "x": true
}`)
	got := stripJSONComments(input)

	var obj map[string]bool
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("result is not valid JSON: %v\nraw: %s", err, string(got))
	}
	if !obj["x"] {
		t.Error("expected x to be true")
	}
}

func TestStripJSONComments_TrailingCommas(t *testing.T) {
	t.Parallel()
	input := []byte(`{
  "a": [1, 2, 3,],
  "b": {"nested": true,},
}`)
	got := stripJSONComments(input)

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("result is not valid JSON: %v\nraw: %s", err, string(got))
	}
}

func TestStripJSONComments_StringsWithSlashes(t *testing.T) {
	t.Parallel()
	input := []byte(`{"url": "https://example.com/path"}`)
	got := stripJSONComments(input)

	var obj map[string]string
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("result is not valid JSON: %v\nraw: %s", err, string(got))
	}
	if obj["url"] != "https://example.com/path" {
		t.Errorf("URL was corrupted: %q", obj["url"])
	}
}

func TestStripJSONComments_CommentLikeStrings(t *testing.T) {
	t.Parallel()
	input := []byte(`{"msg": "this has // slashes and /* stars */"}`)
	got := stripJSONComments(input)

	var obj map[string]string
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("result is not valid JSON: %v\nraw: %s", err, string(got))
	}
	if obj["msg"] != "this has // slashes and /* stars */" {
		t.Errorf("string content was corrupted: %q", obj["msg"])
	}
}

func TestFilterUserSkills_ExcludesBuiltins(t *testing.T) {
	t.Parallel()
	all := []payload.SkillEntry{
		{Name: "create-rule"},
		{Name: "create-skill"},
		{Name: "canvas"},
		{Name: "my-custom-skill"},
	}
	builtins := map[string]bool{
		"create-rule":  true,
		"create-skill": true,
	}

	got := filterUserSkills(all, builtins)
	if len(got) != 2 {
		t.Fatalf("expected 2 user skills, got %d", len(got))
	}
	if got[0].Name != "canvas" {
		t.Errorf("expected canvas, got %q", got[0].Name)
	}
	if got[1].Name != "my-custom-skill" {
		t.Errorf("expected my-custom-skill, got %q", got[1].Name)
	}
}

func TestFilterUserSkills_NilBuiltins(t *testing.T) {
	t.Parallel()
	all := []payload.SkillEntry{{Name: "a"}, {Name: "b"}}
	got := filterUserSkills(all, nil)
	if len(got) != 2 {
		t.Fatalf("expected all skills returned when builtins is nil, got %d", len(got))
	}
}

func TestStripExtensionLocalFields(t *testing.T) {
	t.Parallel()
	input := []byte(`[{"identifier":{"id":"ext.test"},"version":"1.0","location":"/long/path","relativeLocation":"rel"}]`)

	got, err := stripExtensionLocalFields(input)
	if err != nil {
		t.Fatalf("stripExtensionLocalFields failed: %v", err)
	}

	var exts []map[string]json.RawMessage
	if err := json.Unmarshal(got, &exts); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if _, ok := exts[0]["location"]; ok {
		t.Error("location should have been stripped")
	}
	if _, ok := exts[0]["relativeLocation"]; ok {
		t.Error("relativeLocation should have been stripped")
	}
	if _, ok := exts[0]["identifier"]; !ok {
		t.Error("identifier should have been preserved")
	}
}

func TestCollectRules_MissingDir(t *testing.T) {
	t.Parallel()
	got, err := collectRules("/nonexistent/path")
	if err != nil {
		t.Fatalf("expected nil error for missing dir, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestCollectRules_ReadsMDCFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "test.mdc"), []byte("rule content"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("not a rule"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "old.cursorrules"), []byte("old rule"), 0644)

	got, err := collectRules(dir)
	if err != nil {
		t.Fatalf("collectRules failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(got))
	}
}

func TestCollectCommands_MissingDir(t *testing.T) {
	t.Parallel()
	got, err := collectCommands("/nonexistent/path")
	if err != nil {
		t.Fatalf("expected nil error for missing dir, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestCollectCommands_ReadsMDFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "MyCmd.md"), []byte("# Command"), 0644)
	os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("nope"), 0644)

	got, err := collectCommands(dir)
	if err != nil {
		t.Fatalf("collectCommands failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 command, got %d", len(got))
	}
	if got[0].Name != "MyCmd.md" {
		t.Errorf("expected MyCmd.md, got %q", got[0].Name)
	}
}
