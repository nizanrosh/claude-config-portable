package agents_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nizanrosh/claude-config-portable/internal/agents"
)

func TestCollect_ReturnsAgentFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "my-agent.md"), []byte("# Agent content"), 0644); err != nil {
		t.Fatal(err)
	}
	// Non-.md file — should be skipped
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0644); err != nil {
		t.Fatal(err)
	}
	// Backup file — should be skipped
	if err := os.WriteFile(filepath.Join(dir, "my-agent.md.backup-20260301-150624"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := agents.Collect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(result))
	}
	if result[0].Name != "my-agent.md" {
		t.Errorf("name mismatch: got %q", result[0].Name)
	}
	if result[0].Content != "# Agent content" {
		t.Errorf("content mismatch: got %q", result[0].Content)
	}
}

func TestCollect_MissingDir_ReturnsNil(t *testing.T) {
	t.Parallel()

	result, err := agents.Collect("/nonexistent/path/agents")
	if err != nil {
		t.Fatalf("expected nil error for missing dir, got: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for missing dir, got: %v", result)
	}
}
