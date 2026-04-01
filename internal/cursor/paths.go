// Package cursor handles reading and writing Cursor IDE configuration files
// from ~/.cursor/ and the platform-specific Application Support directory.
package cursor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// CursorPaths holds all resolved Cursor config file paths.
type CursorPaths struct {
	CursorDir    string // ~/.cursor
	UserDir      string // platform-specific User settings directory
	Settings     string // User/settings.json
	Keybindings  string // User/keybindings.json
	SnippetsDir  string // User/snippets/
	RulesDir     string // ~/.cursor/rules/
	MCP          string // ~/.cursor/mcp.json
	ExtensionsDB string // ~/.cursor/extensions/extensions.json
	SkillsDir    string // ~/.cursor/skills-cursor/
	SkillsManif  string // ~/.cursor/skills-cursor/.cursor-managed-skills-manifest.json
	CommandsDir  string // ~/.cursor/commands/
	CLIConfig    string // ~/.cursor/cli-config.json
}

// ResolveCursorPaths builds all Cursor config file paths.
func ResolveCursorPaths() (CursorPaths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return CursorPaths{}, fmt.Errorf("resolving home directory: %w", err)
	}

	cursorDir := filepath.Join(home, ".cursor")
	userDir, err := resolveUserDir(home)
	if err != nil {
		return CursorPaths{}, err
	}

	skillsDir := filepath.Join(cursorDir, "skills-cursor")

	return CursorPaths{
		CursorDir:    cursorDir,
		UserDir:      userDir,
		Settings:     filepath.Join(userDir, "settings.json"),
		Keybindings:  filepath.Join(userDir, "keybindings.json"),
		SnippetsDir:  filepath.Join(userDir, "snippets"),
		RulesDir:     filepath.Join(cursorDir, "rules"),
		MCP:          filepath.Join(cursorDir, "mcp.json"),
		ExtensionsDB: filepath.Join(cursorDir, "extensions", "extensions.json"),
		SkillsDir:    skillsDir,
		SkillsManif:  filepath.Join(skillsDir, ".cursor-managed-skills-manifest.json"),
		CommandsDir:  filepath.Join(cursorDir, "commands"),
		CLIConfig:    filepath.Join(cursorDir, "cli-config.json"),
	}, nil
}

func resolveUserDir(home string) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Cursor", "User"), nil
	case "linux":
		return filepath.Join(home, ".config", "Cursor", "User"), nil
	default:
		return "", fmt.Errorf("unsupported platform %q for Cursor paths", runtime.GOOS)
	}
}
