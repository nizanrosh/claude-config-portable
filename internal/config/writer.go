package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

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
	FilesWritten      []string
	SkillsWritten     []string
	PluginsInstalled  []string
	PluginsFailed     []string
	RedactedServers   []string
	Warnings          []string
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

	// Install plugins via `claude plugin install` CLI.
	// Just writing settings.json isn't enough — Claude Code doesn't auto-download
	// plugins from enabledPlugins. We must explicitly trigger installation.
	if !opts.DryRun {
		pluginNames := extractEnabledPlugins(bundle.Settings)
		if len(pluginNames) > 0 {
			installPlugins(pluginNames, result)
		}
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
			// Resolve symlink target relative to the skills dir
			targetPath := skill.LinkTarget
			if !filepath.IsAbs(targetPath) {
				targetPath = filepath.Join(paths.SkillsDir, targetPath)
			}
			if _, err := os.Stat(targetPath); err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("skill %q is a symlink to %q which does not exist on this machine — skipping", skill.Name, skill.LinkTarget))
				continue
			}
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
// Format: { "marketplace-name": { "source": {...} }, ... }
func reconstructMarketplaces(data json.RawMessage, paths Paths) (json.RawMessage, error) {
	var raw map[string]map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return data, nil
	}

	now, _ := json.Marshal(time.Now().UTC().Format(time.RFC3339Nano))
	for name, entry := range raw {
		loc := filepath.Join(paths.ClaudeDir, "plugins", "marketplaces", name)
		locJSON, _ := json.Marshal(loc)
		entry["installLocation"] = locJSON
		entry["lastUpdated"] = now
	}

	return json.Marshal(raw)
}

// extractEnabledPlugins parses the enabledPlugins map from settings JSON.
// Returns plugin specs like "superpowers@claude-plugins-official".
func extractEnabledPlugins(settings json.RawMessage) []string {
	var s map[string]json.RawMessage
	if json.Unmarshal(settings, &s) != nil {
		return nil
	}
	epRaw, ok := s["enabledPlugins"]
	if !ok {
		return nil
	}
	var enabled map[string]bool
	if json.Unmarshal(epRaw, &enabled) != nil {
		return nil
	}
	var names []string
	for name, on := range enabled {
		if on {
			names = append(names, name)
		}
	}
	return names
}

// installPlugins updates marketplaces first, then calls `claude plugin install`
// for each plugin.
func installPlugins(plugins []string, result *WriteResult) {
	claudeBin, err := exec.LookPath("claude")
	if err != nil {
		result.Warnings = append(result.Warnings,
			"claude CLI not found in PATH — skipping plugin installation. Install plugins manually with: claude plugin install <name>")
		return
	}

	// Update all marketplaces first so plugin source paths exist
	fmt.Println("Updating marketplaces...")
	cmd := exec.Command(claudeBin, "plugin", "marketplace", "update")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("marketplace update failed: %v — some plugins may fail to install", err))
	}

	for _, plugin := range plugins {
		cmd := exec.Command(claudeBin, "plugin", "install", plugin, "--scope", "user")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			result.PluginsFailed = append(result.PluginsFailed, plugin)
		} else {
			result.PluginsInstalled = append(result.PluginsInstalled, plugin)
		}
	}
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
