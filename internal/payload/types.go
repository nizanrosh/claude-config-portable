// Package payload defines the ConfigBundle wire format and handles
// encoding (JSON → gzip → base64) and decoding of portable config strings.
package payload

import "encoding/json"

// ConfigBundle is the top-level structure exported/imported by claude-config.
type ConfigBundle struct {
	Version         int                        `json:"version"`
	CreatedAt       string                     `json:"createdAt"`
	Platform        string                     `json:"platform"`
	SecretsIncluded bool                       `json:"secretsIncluded"`
	Settings        json.RawMessage            `json:"settings"`
	SettingsLocal   json.RawMessage            `json:"settingsLocal,omitempty"`
	Plugins         PluginManifest             `json:"plugins"`
	Marketplaces    json.RawMessage            `json:"marketplaces"`
	MCPConfigs      map[string]json.RawMessage `json:"mcpConfigs"`
	UserMCPConfig   json.RawMessage            `json:"userMcpConfig,omitempty"`
	Skills          []SkillEntry               `json:"skills"`
}

// PluginManifest mirrors the structure of installed_plugins.json.
type PluginManifest struct {
	Version int                        `json:"version"`
	Plugins map[string][]PluginInstall `json:"plugins"`
}

// PluginInstall represents a single installed plugin entry.
// Machine-specific fields (InstallPath, timestamps) are stripped on export.
type PluginInstall struct {
	Scope        string `json:"scope"`
	Version      string `json:"version"`
	GitCommitSha string `json:"gitCommitSha,omitempty"`
}

// SkillEntry represents a user-created skill directory.
type SkillEntry struct {
	Name       string            `json:"name"`
	IsSymlink  bool              `json:"isSymlink"`
	LinkTarget string            `json:"linkTarget,omitempty"`
	Files      map[string]string `json:"files,omitempty"`
}
