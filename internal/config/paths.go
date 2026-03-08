package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// ClaudeDir returns the absolute path to ~/.claude/.
func ClaudeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".claude"), nil
}

// Paths holds all resolved config file paths.
type Paths struct {
	ClaudeDir        string
	Settings         string
	SettingsLocal    string
	InstalledPlugins string
	Marketplaces     string
	UserMCP          string
	SkillsDir        string
	PluginsCacheDir  string
}

// ResolvePaths builds all config file paths from the Claude base directory.
func ResolvePaths() (Paths, error) {
	base, err := ClaudeDir()
	if err != nil {
		return Paths{}, err
	}
	return Paths{
		ClaudeDir:        base,
		Settings:         filepath.Join(base, "settings.json"),
		SettingsLocal:    filepath.Join(base, "settings.local.json"),
		InstalledPlugins: filepath.Join(base, "plugins", "installed_plugins.json"),
		Marketplaces:     filepath.Join(base, "plugins", "known_marketplaces.json"),
		UserMCP:          filepath.Join(base, ".mcp.json"),
		SkillsDir:        filepath.Join(base, "skills"),
		PluginsCacheDir:  filepath.Join(base, "plugins", "cache"),
	}, nil
}
