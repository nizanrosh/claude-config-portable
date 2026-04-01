// Package payload — Cursor IDE config bundle types.
// These are separate from the Claude Code types to keep the two implementations independent.
package payload

import "encoding/json"

const CursorSchemaVersion = 1

// CursorConfigBundle is the top-level structure exported/imported for Cursor IDE.
type CursorConfigBundle struct {
	Version         int                        `json:"version"`
	CreatedAt       string                     `json:"createdAt"`
	Platform        string                     `json:"platform"`
	SecretsIncluded bool                       `json:"secretsIncluded"`
	Settings        json.RawMessage            `json:"settings,omitempty"`
	Keybindings     json.RawMessage            `json:"keybindings,omitempty"`
	Snippets        map[string]json.RawMessage `json:"snippets,omitempty"`
	Rules           []RuleEntry                `json:"rules"`
	MCPConfig       json.RawMessage            `json:"mcpConfig,omitempty"`
	Extensions      json.RawMessage            `json:"extensions,omitempty"`
	Skills          []SkillEntry               `json:"skills"`
	Commands        []CommandEntry             `json:"commands"`
	CLIConfig       json.RawMessage            `json:"cliConfig,omitempty"`
}

// RuleEntry represents a Cursor rule file from ~/.cursor/rules/.
type RuleEntry struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// CommandEntry represents a custom command file from ~/.cursor/commands/.
type CommandEntry struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}
