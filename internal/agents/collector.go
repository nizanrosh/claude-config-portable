// Package agents collects user-created agent files from ~/.claude/agents/.
package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nizanrosh/claude-config-portable/internal/payload"
)

// Collect walks the agents directory and returns all agent entries.
// Only files with a ".md" extension are collected; backup files are skipped.
func Collect(agentsDir string) ([]payload.AgentEntry, error) {
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading agents directory: %w", err)
	}

	var result []payload.AgentEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		fullPath := filepath.Join(agentsDir, name)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("reading agent file %q: %w", name, err)
		}
		result = append(result, payload.AgentEntry{
			Name:    name,
			Content: string(data),
		})
	}

	return result, nil
}
