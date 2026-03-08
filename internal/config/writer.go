package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nizanrosh/claude-config-portable/internal/merge"
	"github.com/nizanrosh/claude-config-portable/internal/payload"
	"github.com/nizanrosh/claude-config-portable/internal/secrets"
)

// WriteMode controls how conflicts are handled during import.
type WriteMode int

const (
	WriteModeForce WriteMode = iota
	WriteModeMerge
)

// WriteOptions controls the import behavior.
type WriteOptions struct {
	Mode   WriteMode
	DryRun bool
}

// WriteResult summarizes what was written during import.
type WriteResult struct {
	FilesWritten    []string
	SkillsWritten   []string
	PluginsToSync   int
	RedactedServers []string
	Warnings        []string
}

// WriteBundle writes a ConfigBundle to disk.
func WriteBundle(bundle *payload.ConfigBundle, opts WriteOptions) (*WriteResult, error) {
	paths, err := ResolvePaths()
	if err != nil {
		return nil, err
	}

	result := &WriteResult{}

	// Ensure base directories exist
	for _, dir := range []string{
		paths.ClaudeDir,
		filepath.Dir(paths.InstalledPlugins),
		paths.SkillsDir,
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	// Write settings.json
	if len(bundle.Settings) > 0 {
		if err := writeJSONFile(paths.Settings, bundle.Settings, opts, result); err != nil {
			return nil, fmt.Errorf("writing settings.json: %w", err)
		}
	}

	// Write settings.local.json
	if len(bundle.SettingsLocal) > 0 {
		if err := writeJSONFile(paths.SettingsLocal, bundle.SettingsLocal, opts, result); err != nil {
			return nil, fmt.Errorf("writing settings.local.json: %w", err)
		}
	}

	// Write marketplaces with reconstructed installLocation
	if len(bundle.Marketplaces) > 0 {
		reconstructed, err := reconstructMarketplaces(bundle.Marketplaces, paths)
		if err != nil {
			return nil, fmt.Errorf("reconstructing marketplaces: %w", err)
		}
		if err := writeJSONFile(paths.Marketplaces, reconstructed, opts, result); err != nil {
			return nil, fmt.Errorf("writing marketplaces: %w", err)
		}
	}

	// NOTE: We intentionally do NOT write installed_plugins.json or plugin MCP configs.
	// The settings.json contains enabledPlugins which tells Claude Code what to install.
	// Claude Code will download and install the plugins from the marketplace on next launch.
	// Writing empty cache dirs would cause "plugin may not exist in marketplace" errors.
	if len(bundle.Plugins.Plugins) > 0 {
		result.PluginsToSync = countPluginEntries(bundle)
	}

	// Write user MCP config
	if len(bundle.UserMCPConfig) > 0 {
		if err := writeJSONFile(paths.UserMCP, bundle.UserMCPConfig, opts, result); err != nil {
			return nil, fmt.Errorf("writing user MCP config: %w", err)
		}
		if secrets.HasRedactedValues(bundle.UserMCPConfig) {
			result.RedactedServers = append(result.RedactedServers, "~/.claude/.mcp.json")
		}
	}

	// Write skills
	for _, skill := range bundle.Skills {
		skillPath := filepath.Join(paths.SkillsDir, skill.Name)
		if skill.IsSymlink {
			if _, err := os.Lstat(skillPath); err == nil {
				if opts.Mode == WriteModeForce {
					os.Remove(skillPath)
				} else {
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("skill symlink %q already exists, skipping", skill.Name))
					continue
				}
			}
			if err := os.Symlink(skill.LinkTarget, skillPath); err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("failed to create symlink for skill %q: %v", skill.Name, err))
				continue
			}
			result.SkillsWritten = append(result.SkillsWritten, skill.Name+" (symlink)")
			continue
		}

		if err := os.MkdirAll(skillPath, 0755); err != nil {
			return nil, fmt.Errorf("creating skill directory %q: %w", skill.Name, err)
		}
		for fileName, content := range skill.Files {
			filePath := filepath.Join(skillPath, fileName)
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				return nil, fmt.Errorf("writing skill file %q: %w", filePath, err)
			}
		}
		result.SkillsWritten = append(result.SkillsWritten, skill.Name)
	}

	return result, nil
}

// CheckConflicts returns an error if any target files already exist
// and neither force nor merge mode is set.
func CheckConflicts(bundle *payload.ConfigBundle) error {
	paths, err := ResolvePaths()
	if err != nil {
		return err
	}

	var conflicts []string
	for _, path := range []string{paths.Settings, paths.SettingsLocal, paths.Marketplaces} {
		if _, err := os.Stat(path); err == nil {
			conflicts = append(conflicts, path)
		}
	}

	if len(conflicts) > 0 {
		return fmt.Errorf("existing config files found:\n  %s\nUse --force to overwrite or --merge to deep-merge",
			joinLines(conflicts))
	}
	return nil
}

func writeJSONFile(path string, data json.RawMessage, opts WriteOptions, result *WriteResult) error {
	if opts.DryRun {
		result.FilesWritten = append(result.FilesWritten, path+" (dry run)")
		return nil
	}

	if opts.Mode == WriteModeMerge {
		existing, err := os.ReadFile(path)
		if err == nil {
			merged, err := merge.DeepMerge(json.RawMessage(existing), data)
			if err != nil {
				return fmt.Errorf("merging %s: %w", path, err)
			}
			data = merged
		}
	}

	// Pretty-print JSON for human readability
	var pretty json.RawMessage
	if err := json.Unmarshal(data, &pretty); err == nil {
		if formatted, err := json.MarshalIndent(pretty, "", "  "); err == nil {
			data = formatted
		}
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	result.FilesWritten = append(result.FilesWritten, path)
	return nil
}

// reconstructMarketplaces adds installLocation back to marketplace entries.
func reconstructMarketplaces(data json.RawMessage, paths Paths) (json.RawMessage, error) {
	var raw []map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return data, nil
	}

	for _, entry := range raw {
		// Extract the id to build the installLocation
		var id string
		if idRaw, ok := entry["id"]; ok {
			json.Unmarshal(idRaw, &id)
		}
		if id != "" {
			loc := filepath.Join(paths.ClaudeDir, "plugins", "marketplaces", id)
			locJSON, _ := json.Marshal(loc)
			entry["installLocation"] = locJSON
		}
	}

	return json.Marshal(raw)
}

func countPluginEntries(bundle *payload.ConfigBundle) int {
	count := 0
	for _, installs := range bundle.Plugins.Plugins {
		count += len(installs)
	}
	return count
}

func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n  "
		}
		result += l
	}
	return result
}
