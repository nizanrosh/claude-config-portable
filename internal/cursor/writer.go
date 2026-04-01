package cursor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

// CursorComponent identifies a part of the Cursor config bundle that can be
// selectively included or excluded during import.
type CursorComponent string

const (
	CompSettings    CursorComponent = "settings"
	CompKeybindings CursorComponent = "keybindings"
	CompSnippets    CursorComponent = "snippets"
	CompRules       CursorComponent = "rules"
	CompMCP         CursorComponent = "mcp"
	CompExtensions  CursorComponent = "extensions"
	CompSkills      CursorComponent = "skills"
	CompCommands    CursorComponent = "commands"
	CompCLIConfig   CursorComponent = "cli-config"
)

// AllCursorComponents returns the list of all importable Cursor components.
func AllCursorComponents() []CursorComponent {
	return []CursorComponent{
		CompSettings, CompKeybindings, CompSnippets, CompRules,
		CompMCP, CompExtensions, CompSkills, CompCommands, CompCLIConfig,
	}
}

// WriteOptions controls the Cursor import behavior.
type WriteOptions struct {
	Mode   WriteMode
	DryRun bool
	Only   []string
	Skip   []string
}

// WriteResult summarizes what was written during Cursor import.
type WriteResult struct {
	FilesWritten       []string
	SkillsWritten      []string
	CommandsWritten    []string
	ExtensionsInstalled []string
	ExtensionsFailed   []string
	RedactedServers    []string
	Warnings           []string
}

func shouldImport(comp CursorComponent, opts WriteOptions) bool {
	if len(opts.Only) > 0 {
		for _, c := range opts.Only {
			if CursorComponent(c) == comp {
				return true
			}
		}
		return false
	}
	for _, c := range opts.Skip {
		if CursorComponent(c) == comp {
			return false
		}
	}
	return true
}

// ValidateComponents checks that all values in only/skip are recognized component names.
func ValidateComponents(only, skip []string) error {
	valid := make(map[string]bool)
	for _, c := range AllCursorComponents() {
		valid[string(c)] = true
	}
	for _, c := range only {
		if !valid[c] {
			return fmt.Errorf("unknown component %q in --only (valid: %s)", c, componentList())
		}
	}
	for _, c := range skip {
		if !valid[c] {
			return fmt.Errorf("unknown component %q in --skip (valid: %s)", c, componentList())
		}
	}
	return nil
}

func componentList() string {
	var names []string
	for _, c := range AllCursorComponents() {
		names = append(names, string(c))
	}
	return strings.Join(names, ", ")
}

// WriteCursorBundle writes a CursorConfigBundle to disk.
func WriteCursorBundle(bundle *payload.CursorConfigBundle, opts WriteOptions) (*WriteResult, error) {
	if err := ValidateComponents(opts.Only, opts.Skip); err != nil {
		return nil, err
	}

	paths, err := ResolveCursorPaths()
	if err != nil {
		return nil, err
	}

	result := &WriteResult{}

	if !opts.DryRun {
		for _, dir := range []string{
			paths.UserDir,
			paths.SnippetsDir,
			paths.RulesDir,
			paths.SkillsDir,
			paths.CommandsDir,
			filepath.Dir(paths.ExtensionsDB),
		} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("creating directory %s: %w", dir, err)
			}
		}
	}

	if len(bundle.Settings) > 0 && shouldImport(CompSettings, opts) {
		if err := writeJSONFile(paths.Settings, bundle.Settings, opts, result); err != nil {
			return nil, fmt.Errorf("writing settings.json: %w", err)
		}
	}

	if len(bundle.Keybindings) > 0 && shouldImport(CompKeybindings, opts) {
		if err := writeJSONFile(paths.Keybindings, bundle.Keybindings, opts, result); err != nil {
			return nil, fmt.Errorf("writing keybindings.json: %w", err)
		}
	}

	if len(bundle.Snippets) > 0 && shouldImport(CompSnippets, opts) {
		for name, data := range bundle.Snippets {
			snippetPath := filepath.Join(paths.SnippetsDir, name)
			if err := writeJSONFile(snippetPath, data, opts, result); err != nil {
				return nil, fmt.Errorf("writing snippet %q: %w", name, err)
			}
		}
	}

	if shouldImport(CompRules, opts) {
		for _, rule := range bundle.Rules {
			rulePath := filepath.Join(paths.RulesDir, rule.Name)
			if opts.DryRun {
				result.FilesWritten = append(result.FilesWritten, rulePath+" (dry run)")
				continue
			}
			if err := os.WriteFile(rulePath, []byte(rule.Content), 0644); err != nil {
				return nil, fmt.Errorf("writing rule %q: %w", rule.Name, err)
			}
			result.FilesWritten = append(result.FilesWritten, rulePath)
		}
	}

	if len(bundle.MCPConfig) > 0 && shouldImport(CompMCP, opts) {
		if err := writeJSONFile(paths.MCP, bundle.MCPConfig, opts, result); err != nil {
			return nil, fmt.Errorf("writing mcp.json: %w", err)
		}
		if secrets.HasRedactedValues(bundle.MCPConfig) {
			result.RedactedServers = append(result.RedactedServers, "~/.cursor/mcp.json")
		}
	}

	if len(bundle.Extensions) > 0 && shouldImport(CompExtensions, opts) {
		if err := writeJSONFile(paths.ExtensionsDB, bundle.Extensions, opts, result); err != nil {
			return nil, fmt.Errorf("writing extensions.json: %w", err)
		}
	}

	if !opts.DryRun && shouldImport(CompExtensions, opts) {
		extIDs := extractExtensionIDs(bundle.Extensions)
		if len(extIDs) > 0 {
			installExtensions(extIDs, result)
		}
	}

	if shouldImport(CompSkills, opts) {
		for _, skill := range bundle.Skills {
			skillPath := filepath.Join(paths.SkillsDir, skill.Name)
			if skill.IsSymlink {
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

		if opts.DryRun {
			result.SkillsWritten = append(result.SkillsWritten, skill.Name+" (dry run)")
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
	}

	if shouldImport(CompCommands, opts) {
		for _, cmd := range bundle.Commands {
			cmdPath := filepath.Join(paths.CommandsDir, cmd.Name)
			if opts.DryRun {
				result.CommandsWritten = append(result.CommandsWritten, cmd.Name+" (dry run)")
				continue
			}
			if err := os.WriteFile(cmdPath, []byte(cmd.Content), 0644); err != nil {
				return nil, fmt.Errorf("writing command %q: %w", cmd.Name, err)
			}
			result.CommandsWritten = append(result.CommandsWritten, cmd.Name)
		}
	}

	if len(bundle.CLIConfig) > 0 && shouldImport(CompCLIConfig, opts) {
		if err := writeJSONFile(paths.CLIConfig, bundle.CLIConfig, opts, result); err != nil {
			return nil, fmt.Errorf("writing cli-config.json: %w", err)
		}
	}

	return result, nil
}

// CheckCursorConflicts returns an error if any target files already exist
// and neither force nor merge mode is set.
func CheckCursorConflicts(bundle *payload.CursorConfigBundle) error {
	paths, err := ResolveCursorPaths()
	if err != nil {
		return err
	}

	var conflicts []string
	for _, path := range []string{paths.Settings, paths.Keybindings, paths.MCP, paths.CLIConfig} {
		if _, err := os.Stat(path); err == nil {
			conflicts = append(conflicts, path)
		}
	}

	if len(conflicts) > 0 {
		msg := "existing config files found:\n"
		for i, c := range conflicts {
			if i > 0 {
				msg += "\n"
			}
			msg += "  " + c
		}
		return fmt.Errorf("%s\nUse --force to overwrite or --merge to deep-merge", msg)
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

// extractExtensionIDs pulls "identifier.id" values from the extensions manifest.
func extractExtensionIDs(data json.RawMessage) []string {
	if len(data) == 0 {
		return nil
	}
	var extensions []struct {
		Identifier struct {
			ID string `json:"id"`
		} `json:"identifier"`
	}
	if json.Unmarshal(data, &extensions) != nil {
		return nil
	}
	var ids []string
	for _, ext := range extensions {
		if ext.Identifier.ID != "" {
			ids = append(ids, ext.Identifier.ID)
		}
	}
	return ids
}

func installExtensions(ids []string, result *WriteResult) {
	cursorBin, err := exec.LookPath("cursor")
	if err != nil {
		result.Warnings = append(result.Warnings,
			"cursor CLI not found in PATH — skipping extension installation. Install extensions manually with: cursor --install-extension <id>")
		return
	}

	for _, id := range ids {
		cmd := exec.Command(cursorBin, "--install-extension", id)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			result.ExtensionsFailed = append(result.ExtensionsFailed, id)
		} else {
			result.ExtensionsInstalled = append(result.ExtensionsInstalled, id)
		}
	}
}
