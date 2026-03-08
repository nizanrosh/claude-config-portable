package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/nizanrosh/claude-config-portable/internal/payload"
	"github.com/nizanrosh/claude-config-portable/internal/secrets"
	"github.com/nizanrosh/claude-config-portable/internal/skills"
)

// ReadOptions controls what gets included in the export.
type ReadOptions struct {
	WithSecrets bool
	NoSkills    bool
}

// ReadBundle reads all Claude config files and assembles a ConfigBundle.
func ReadBundle(opts ReadOptions) (*payload.ConfigBundle, error) {
	paths, err := ResolvePaths()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(paths.Settings); err != nil {
		return nil, fmt.Errorf("settings.json not found at %s — is Claude Code installed?", paths.Settings)
	}

	bundle := &payload.ConfigBundle{
		Version:         payload.SchemaVersion,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		Platform:        runtime.GOOS,
		SecretsIncluded: opts.WithSecrets,
		MCPConfigs:      make(map[string]json.RawMessage),
	}

	// Read settings
	bundle.Settings, err = os.ReadFile(paths.Settings)
	if err != nil {
		return nil, fmt.Errorf("reading settings.json: %w", err)
	}

	// Read settings.local.json (optional)
	if data, err := os.ReadFile(paths.SettingsLocal); err == nil {
		bundle.SettingsLocal = data
	}

	// Read installed plugins and strip machine-specific fields
	if data, err := os.ReadFile(paths.InstalledPlugins); err == nil {
		manifest, err := parseAndStripPlugins(data)
		if err != nil {
			return nil, fmt.Errorf("parsing installed plugins: %w", err)
		}
		bundle.Plugins = manifest
	}

	// Read marketplaces and strip machine-specific fields
	if data, err := os.ReadFile(paths.Marketplaces); err == nil {
		stripped, err := stripMarketplaceFields(data)
		if err != nil {
			return nil, fmt.Errorf("stripping marketplace fields: %w", err)
		}
		bundle.Marketplaces = stripped
	}

	// Read MCP configs from plugin install paths
	if err := collectPluginMCPConfigs(bundle, paths, opts.WithSecrets); err != nil {
		return nil, err
	}

	// Read user-level MCP config
	if data, err := os.ReadFile(paths.UserMCP); err == nil {
		if opts.WithSecrets {
			bundle.UserMCPConfig = data
		} else {
			filtered, err := secrets.FilterMCPServers(data)
			if err != nil {
				return nil, fmt.Errorf("filtering user MCP config: %w", err)
			}
			bundle.UserMCPConfig = filtered
		}
	}

	// Collect skills
	if !opts.NoSkills {
		skillEntries, err := skills.Collect(paths.SkillsDir)
		if err != nil {
			return nil, fmt.Errorf("collecting skills: %w", err)
		}
		bundle.Skills = skillEntries
	}
	if bundle.Skills == nil {
		bundle.Skills = []payload.SkillEntry{}
	}

	return bundle, nil
}

// parseAndStripPlugins reads the installed plugins manifest and strips
// machine-specific fields (installPath, installTimestamp, lastUpdated).
func parseAndStripPlugins(data []byte) (payload.PluginManifest, error) {
	// Parse the raw structure to access all fields
	var raw struct {
		Version int                              `json:"version"`
		Plugins map[string][]json.RawMessage     `json:"plugins"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return payload.PluginManifest{}, fmt.Errorf("unmarshaling: %w", err)
	}

	result := payload.PluginManifest{
		Version: raw.Version,
		Plugins: make(map[string][]payload.PluginInstall),
	}

	for name, installs := range raw.Plugins {
		for _, installRaw := range installs {
			var full map[string]json.RawMessage
			if err := json.Unmarshal(installRaw, &full); err != nil {
				return payload.PluginManifest{}, fmt.Errorf("unmarshaling plugin %q: %w", name, err)
			}

			pi := payload.PluginInstall{}
			if v, ok := full["scope"]; ok {
				json.Unmarshal(v, &pi.Scope)
			}
			if v, ok := full["version"]; ok {
				json.Unmarshal(v, &pi.Version)
			}
			if v, ok := full["gitCommitSha"]; ok {
				json.Unmarshal(v, &pi.GitCommitSha)
			}
			result.Plugins[name] = append(result.Plugins[name], pi)
		}
	}

	return result, nil
}

// stripMarketplaceFields removes installLocation and lastUpdated from marketplace entries.
// The file format is: { "marketplace-name": { "source": {...}, "installLocation": "...", "lastUpdated": "..." }, ... }
func stripMarketplaceFields(data []byte) (json.RawMessage, error) {
	var raw map[string]map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return data, nil // return as-is if we can't parse
	}

	for _, entry := range raw {
		delete(entry, "installLocation")
		delete(entry, "lastUpdated")
	}

	out, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("re-marshaling marketplaces: %w", err)
	}
	return out, nil
}

// collectPluginMCPConfigs reads .mcp.json from each installed plugin's directory.
func collectPluginMCPConfigs(bundle *payload.ConfigBundle, paths Paths, withSecrets bool) error {
	// Walk the plugins cache directory looking for .mcp.json files
	cacheDir := paths.PluginsCacheDir
	if _, err := os.Stat(cacheDir); err != nil {
		return nil // no cache dir, nothing to do
	}

	err := filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() || info.Name() != ".mcp.json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable files
		}

		// Use relative path from cache dir as the key
		relPath, _ := filepath.Rel(cacheDir, path)
		key := filepath.Dir(relPath)

		if withSecrets {
			bundle.MCPConfigs[key] = data
		} else {
			filtered, err := secrets.FilterMCPConfig(data)
			if err != nil {
				return fmt.Errorf("filtering MCP config %q: %w", key, err)
			}
			bundle.MCPConfigs[key] = filtered
		}

		return nil
	})

	return err
}
