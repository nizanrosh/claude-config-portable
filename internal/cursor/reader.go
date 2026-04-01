package cursor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/nizanrosh/claude-config-portable/internal/payload"
	"github.com/nizanrosh/claude-config-portable/internal/secrets"
	"github.com/nizanrosh/claude-config-portable/internal/skills"
)

// ReadOptions controls what gets included in the Cursor export.
type ReadOptions struct {
	WithSecrets bool
	NoSkills    bool
	NoCommands  bool
}

// ReadCursorBundle reads all Cursor config files and assembles a CursorConfigBundle.
func ReadCursorBundle(opts ReadOptions) (*payload.CursorConfigBundle, error) {
	paths, err := ResolveCursorPaths()
	if err != nil {
		return nil, err
	}

	bundle := &payload.CursorConfigBundle{
		Version:         payload.CursorSchemaVersion,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		Platform:        runtime.GOOS,
		SecretsIncluded: opts.WithSecrets,
	}

	if data, err := os.ReadFile(paths.Settings); err == nil {
		bundle.Settings = data
	}

	if data, err := os.ReadFile(paths.Keybindings); err == nil {
		bundle.Keybindings = stripJSONComments(data)
	}

	snippets, err := collectSnippets(paths.SnippetsDir)
	if err != nil {
		return nil, fmt.Errorf("collecting snippets: %w", err)
	}
	bundle.Snippets = snippets

	rules, err := collectRules(paths.RulesDir)
	if err != nil {
		return nil, fmt.Errorf("collecting rules: %w", err)
	}
	bundle.Rules = rules

	if data, err := os.ReadFile(paths.MCP); err == nil {
		if opts.WithSecrets {
			bundle.MCPConfig = data
		} else {
			filtered, err := secrets.FilterMCPServers(data)
			if err != nil {
				return nil, fmt.Errorf("filtering MCP config: %w", err)
			}
			bundle.MCPConfig = filtered
		}
	}

	if data, err := os.ReadFile(paths.ExtensionsDB); err == nil {
		stripped, err := stripExtensionLocalFields(data)
		if err != nil {
			return nil, fmt.Errorf("stripping extension fields: %w", err)
		}
		bundle.Extensions = stripped
	}

	if !opts.NoSkills {
		builtinIDs := loadBuiltinSkillIDs(paths.SkillsManif)
		allSkills, err := skills.Collect(paths.SkillsDir)
		if err != nil {
			return nil, fmt.Errorf("collecting skills: %w", err)
		}
		bundle.Skills = filterUserSkills(allSkills, builtinIDs)
	}
	if bundle.Skills == nil {
		bundle.Skills = []payload.SkillEntry{}
	}

	if !opts.NoCommands {
		commands, err := collectCommands(paths.CommandsDir)
		if err != nil {
			return nil, fmt.Errorf("collecting commands: %w", err)
		}
		bundle.Commands = commands
	}
	if bundle.Commands == nil {
		bundle.Commands = []payload.CommandEntry{}
	}

	if data, err := os.ReadFile(paths.CLIConfig); err == nil {
		bundle.CLIConfig = data
	}

	return bundle, nil
}

func collectSnippets(dir string) (map[string]json.RawMessage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	result := make(map[string]json.RawMessage)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading snippet %q: %w", entry.Name(), err)
		}
		result[entry.Name()] = data
	}

	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func collectRules(dir string) ([]payload.RuleEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var rules []payload.RuleEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".mdc") && !strings.HasSuffix(name, ".cursorrules") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("reading rule %q: %w", name, err)
		}
		rules = append(rules, payload.RuleEntry{
			Name:    name,
			Content: string(data),
		})
	}
	return rules, nil
}

func collectCommands(dir string) ([]payload.CommandEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var commands []payload.CommandEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("reading command %q: %w", name, err)
		}
		commands = append(commands, payload.CommandEntry{
			Name:    name,
			Content: string(data),
		})
	}
	return commands, nil
}

type skillsManifest struct {
	BuiltinSkillIDs []string `json:"builtinSkillIds"`
	ManagedSkillIDs []string `json:"managedSkillIds"`
}

func loadBuiltinSkillIDs(manifestPath string) map[string]bool {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil
	}
	var manifest skillsManifest
	if json.Unmarshal(data, &manifest) != nil {
		return nil
	}
	ids := make(map[string]bool, len(manifest.BuiltinSkillIDs)+len(manifest.ManagedSkillIDs))
	for _, id := range manifest.BuiltinSkillIDs {
		ids[id] = true
	}
	for _, id := range manifest.ManagedSkillIDs {
		ids[id] = true
	}
	return ids
}

func filterUserSkills(allSkills []payload.SkillEntry, builtinIDs map[string]bool) []payload.SkillEntry {
	if builtinIDs == nil {
		return allSkills
	}
	var userSkills []payload.SkillEntry
	for _, s := range allSkills {
		if !builtinIDs[s.Name] {
			userSkills = append(userSkills, s)
		}
	}
	return userSkills
}

// stripJSONComments removes single-line // comments and trailing commas from
// JSONC files (the format VS Code/Cursor uses for keybindings.json etc.).
func stripJSONComments(data []byte) json.RawMessage {
	var result []byte
	inString := false
	escaped := false
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inString && strings.HasPrefix(trimmed, "//") {
			continue
		}
		for i := 0; i < len(line); i++ {
			if escaped {
				escaped = false
				continue
			}
			if line[i] == '\\' && inString {
				escaped = true
				continue
			}
			if line[i] == '"' {
				inString = !inString
			}
		}
		result = append(result, []byte(line)...)
		result = append(result, '\n')
	}
	return json.RawMessage(result)
}

// stripExtensionLocalFields removes machine-specific fields from each extension entry.
// Keeps: identifier, version, metadata. Strips: location, relativeLocation.
func stripExtensionLocalFields(data []byte) (json.RawMessage, error) {
	var extensions []map[string]json.RawMessage
	if err := json.Unmarshal(data, &extensions); err != nil {
		return data, nil
	}

	for _, ext := range extensions {
		delete(ext, "location")
		delete(ext, "relativeLocation")
	}

	out, err := json.Marshal(extensions)
	if err != nil {
		return nil, fmt.Errorf("re-marshaling extensions: %w", err)
	}
	return out, nil
}
