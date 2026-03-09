package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/nizanrosh/claude-config-portable/internal/config"
	"github.com/nizanrosh/claude-config-portable/internal/payload"
	"github.com/nizanrosh/claude-config-portable/internal/secrets"
	"github.com/spf13/cobra"
)

// Color helpers for terminal output.
var (
	cBold    = color.New(color.Bold).SprintFunc()
	cCyan    = color.New(color.FgCyan, color.Bold).SprintFunc()
	cGreen   = color.New(color.FgGreen).SprintFunc()
	cYellow  = color.New(color.FgYellow, color.Bold).SprintFunc()
	cRed     = color.New(color.FgRed, color.Bold).SprintFunc()
	cDim     = color.New(color.Faint).SprintFunc()
	cRedDim  = color.New(color.FgRed).SprintFunc()
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:   "claude-config",
		Short: "Export and import Claude Code configuration",
		Long:  "A portable tool to share Claude Code setup (plugins, skills, MCPs, settings) between machines.",
	}

	root.AddCommand(exportCmd(), importCmd(), inspectCmd(), versionCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func exportCmd() *cobra.Command {
	var (
		withSecrets bool
		noSkills    bool
		noAgents    bool
		output      string
		copyClip    bool
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export Claude Code config as a portable string",
		RunE: func(cmd *cobra.Command, args []string) error {
			bundle, err := config.ReadBundle(config.ReadOptions{
				WithSecrets: withSecrets,
				NoSkills:    noSkills,
				NoAgents:    noAgents,
			})
			if err != nil {
				return err
			}

			encoded, err := payload.Encode(bundle)
			if err != nil {
				return fmt.Errorf("encoding bundle: %w", err)
			}

			if output != "" {
				if err := os.WriteFile(output, []byte(encoded), 0644); err != nil {
					return fmt.Errorf("writing to %s: %w", output, err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Config exported to %s\n", cGreen(output))
			} else if copyClip {
				if err := copyToClipboard(encoded); err != nil {
					return fmt.Errorf("copying to clipboard: %w", err)
				}
				fmt.Fprintln(cmd.ErrOrStderr(), cGreen("Config copied to clipboard!"))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), encoded)
			}

			printExportSummary(cmd, bundle)
			return nil
		},
	}

	cmd.Flags().BoolVar(&withSecrets, "with-secrets", false, "Include MCP server secrets (headers, env vars, OAuth tokens)")
	cmd.Flags().BoolVar(&noSkills, "no-skills", false, "Exclude user-created skills")
	cmd.Flags().BoolVar(&noAgents, "no-agents", false, "Exclude user-created agents")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write to file instead of stdout")
	cmd.Flags().BoolVarP(&copyClip, "copy", "c", false, "Copy to clipboard instead of printing")

	return cmd
}

func importCmd() *cobra.Command {
	var (
		force         bool
		doMerge       bool
		dryRun        bool
		withHooks     bool
		fromClipboard bool
		only          []string
		skip          []string
	)

	cmd := &cobra.Command{
		Use:   "import [string-or-file]",
		Short: "Import Claude Code config from a portable string, file, or clipboard",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var input string
			var err error
			if fromClipboard {
				input, err = readFromClipboard()
				if err != nil {
					return fmt.Errorf("reading from clipboard: %w", err)
				}
				input = strings.TrimSpace(input)
			} else {
				if len(args) == 0 {
					return fmt.Errorf("provide a config string, file path, or use --from-clipboard")
				}
				input, err = resolveInput(args[0])
			}
			if err != nil {
				return err
			}

			bundle, err := payload.Decode(input)
			if err != nil {
				return fmt.Errorf("decoding config: %w", err)
			}

			// Print security summary before import
			printSecuritySummary(cmd, bundle, withHooks)

			mode := config.WriteModeForce
			if doMerge {
				mode = config.WriteModeMerge
			}

			writeOpts := config.WriteOptions{
				Mode:      mode,
				DryRun:    dryRun,
				WithHooks: withHooks,
				Only:      only,
				Skip:      skip,
			}

			if dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), cYellow("=== DRY RUN — no changes will be written ==="))
				printInspection(cmd, bundle)
				result, err := config.WriteBundle(bundle, writeOpts)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "\nFiles that would be written:")
				for _, f := range result.FilesWritten {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", f)
				}
				return nil
			}

			if !force && !doMerge {
				if err := config.CheckConflicts(bundle); err != nil {
					return err
				}
			}

			result, err := config.WriteBundle(bundle, writeOpts)
			if err != nil {
				return err
			}

			printImportResult(cmd, result, bundle)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config files")
	cmd.Flags().BoolVar(&doMerge, "merge", false, "Deep-merge with existing config (incoming wins on conflicts)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would change without writing")
	cmd.Flags().BoolVar(&withHooks, "with-hooks", false, "Import hooks from settings.local.json (stripped by default for security)")
	cmd.Flags().BoolVar(&fromClipboard, "from-clipboard", false, "Read config from clipboard instead of argument")
	cmd.Flags().StringSliceVar(&only, "only", nil, "Only import these components (settings,hooks,permissions,plugins,marketplaces,mcp,skills,agents)")
	cmd.Flags().StringSliceVar(&skip, "skip", nil, "Skip these components during import")

	return cmd
}

func inspectCmd() *cobra.Command {
	var fromClipboard bool

	cmd := &cobra.Command{
		Use:   "inspect [string-or-file]",
		Short: "Show a human-readable summary of a config export",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var input string
			var err error
			if fromClipboard {
				input, err = readFromClipboard()
				if err != nil {
					return fmt.Errorf("reading from clipboard: %w", err)
				}
				input = strings.TrimSpace(input)
			} else {
				if len(args) == 0 {
					return fmt.Errorf("provide a config string, file path, or use --from-clipboard")
				}
				input, err = resolveInput(args[0])
			}
			if err != nil {
				return err
			}

			bundle, err := payload.Decode(input)
			if err != nil {
				return fmt.Errorf("decoding config: %w", err)
			}

			printInspection(cmd, bundle)
			return nil
		},
	}

	cmd.Flags().BoolVar(&fromClipboard, "from-clipboard", false, "Read config from clipboard instead of argument")
	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "claude-config %s\n", version)
		},
	}
}

// resolveInput reads from a file path or returns the string directly.
func resolveInput(arg string) (string, error) {
	if strings.HasPrefix(arg, "ccfg:") {
		return arg, nil
	}

	data, err := os.ReadFile(arg)
	if err != nil {
		return "", fmt.Errorf("reading file %s: %w", arg, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func printExportSummary(cmd *cobra.Command, bundle *payload.ConfigBundle) {
	w := cmd.ErrOrStderr()
	fmt.Fprintf(w, "\n%s\n", cCyan("--- Export Summary ---"))
	fmt.Fprintf(w, "Plugins:     %s\n", cBold(countPlugins(bundle)))
	fmt.Fprintf(w, "Skills:      %s\n", cBold(len(bundle.Skills)))
	fmt.Fprintf(w, "Agents:      %s\n", cBold(len(bundle.Agents)))
	fmt.Fprintf(w, "MCP configs: %s\n", cBold(len(bundle.MCPConfigs)))
	if len(bundle.UserMCPConfig) > 0 {
		fmt.Fprintf(w, "User MCP:    %s\n", cGreen("included"))
	}
	if bundle.SecretsIncluded {
		fmt.Fprintf(w, "Secrets:     %s\n", cRed("INCLUDED (handle with care!)"))
	} else {
		fmt.Fprintf(w, "Secrets:     %s\n", cGreen("stripped"))
	}
}

func printInspection(cmd *cobra.Command, bundle *payload.ConfigBundle) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s\n", cCyan("=== Claude Config Inspection ==="))
	fmt.Fprintf(w, "Schema version: %s\n", cBold(bundle.Version))
	fmt.Fprintf(w, "Created:        %s\n", cDim(bundle.CreatedAt))
	fmt.Fprintf(w, "Platform:       %s\n", cDim(bundle.Platform))
	if bundle.SecretsIncluded {
		fmt.Fprintf(w, "Secrets:        %s\n", cRed("true"))
	} else {
		fmt.Fprintf(w, "Secrets:        %s\n", cGreen("false"))
	}

	fmt.Fprintf(w, "\n%s\n", cYellow(fmt.Sprintf("Plugins (%d):", countPlugins(bundle))))
	for name, installs := range bundle.Plugins.Plugins {
		for _, pi := range installs {
			fmt.Fprintf(w, "  - %s %s\n", name, cDim(fmt.Sprintf("(v%s, scope: %s)", pi.Version, pi.Scope)))
		}
	}

	if len(bundle.MCPConfigs) > 0 {
		fmt.Fprintf(w, "\n%s\n", cYellow(fmt.Sprintf("MCP Configs (%d):", len(bundle.MCPConfigs))))
		for key, cfg := range bundle.MCPConfigs {
			redacted := ""
			if secrets.HasRedactedValues(cfg) {
				redacted = " " + cRedDim("[secrets redacted]")
			}
			fmt.Fprintf(w, "  - %s%s\n", key, redacted)
		}
	}

	if len(bundle.UserMCPConfig) > 0 {
		fmt.Fprintf(w, "\n%s\n", cYellow("User MCP Config:"))
		var obj map[string]json.RawMessage
		if json.Unmarshal(bundle.UserMCPConfig, &obj) == nil {
			if servers, ok := obj["mcpServers"]; ok {
				var srvMap map[string]json.RawMessage
				if json.Unmarshal(servers, &srvMap) == nil {
					for name, cfg := range srvMap {
						redacted := ""
						if secrets.HasRedactedValues(cfg) {
							redacted = " " + cRedDim("[secrets redacted]")
						}
						fmt.Fprintf(w, "  - %s %s%s\n", name, cDim(extractMCPType(cfg)), redacted)
					}
				}
			}
		}
	}

	// Show hooks detail in inspect
	if len(bundle.SettingsLocal) > 0 {
		var local map[string]json.RawMessage
		if json.Unmarshal(bundle.SettingsLocal, &local) == nil {
			if hooksRaw, ok := local["hooks"]; ok {
				var hooks map[string]json.RawMessage
				if json.Unmarshal(hooksRaw, &hooks) == nil && len(hooks) > 0 {
					fmt.Fprintf(w, "\n%s %s\n",
						cYellow(fmt.Sprintf("Hooks (%d event types)", len(hooks))),
						cRed("[SECURITY RISK]"))
					printHookDetails(w, hooks)
				}
			}
			if statusRaw, ok := local["statusLine"]; ok {
				var status map[string]json.RawMessage
				if json.Unmarshal(statusRaw, &status) == nil {
					if cmdRaw, ok := status["command"]; ok {
						var c string
						_ = json.Unmarshal(cmdRaw, &c)
						fmt.Fprintf(w, "\n%s %s: %s\n",
							cYellow("statusLine command"),
							cRed("[SECURITY RISK]"),
							cDim(c))
					}
				}
			}
		}
	}

	if len(bundle.Skills) > 0 {
		fmt.Fprintf(w, "\n%s\n", cYellow(fmt.Sprintf("Skills (%d):", len(bundle.Skills))))
		for _, skill := range bundle.Skills {
			var kind string
			if skill.IsSymlink {
				kind = fmt.Sprintf("symlink → %s", skill.LinkTarget)
			} else {
				kind = fmt.Sprintf("%d files", len(skill.Files))
			}
			fmt.Fprintf(w, "  - %s %s\n", skill.Name, cDim("("+kind+")"))
			// Show file names for non-symlink skills
			if !skill.IsSymlink {
				for fileName := range skill.Files {
					fmt.Fprintf(w, "      %s\n", cDim(fileName))
				}
			}
		}
	}

	if len(bundle.Agents) > 0 {
		fmt.Fprintf(w, "\n%s\n", cYellow(fmt.Sprintf("Agents (%d):", len(bundle.Agents))))
		for _, agent := range bundle.Agents {
			fmt.Fprintf(w, "  - %s\n", agent.Name)
		}
	}

	// Settings highlights
	if len(bundle.Settings) > 0 {
		var settings map[string]json.RawMessage
		if json.Unmarshal(bundle.Settings, &settings) == nil {
			fmt.Fprintf(w, "\n%s\n", cYellow("Settings highlights:"))
			for _, key := range []string{"model", "effortLevel", "enabledPlugins"} {
				if val, ok := settings[key]; ok {
					fmt.Fprintf(w, "  %s: %s\n", key, cGreen(string(val)))
				}
			}
		}
	}

	if len(bundle.Marketplaces) > 0 {
		var marketplaces []map[string]json.RawMessage
		if json.Unmarshal(bundle.Marketplaces, &marketplaces) == nil && len(marketplaces) > 0 {
			fmt.Fprintf(w, "\n%s\n", cYellow(fmt.Sprintf("Marketplaces (%d):", len(marketplaces))))
			for _, m := range marketplaces {
				var name string
				if n, ok := m["name"]; ok {
					_ = json.Unmarshal(n, &name)
				}
				var id string
				if i, ok := m["id"]; ok {
					_ = json.Unmarshal(i, &id)
				}
				if name != "" {
					fmt.Fprintf(w, "  - %s %s\n", name, cDim("("+id+")"))
				} else if id != "" {
					fmt.Fprintf(w, "  - %s\n", id)
				}
			}
		}
	}
}

func printSecuritySummary(cmd *cobra.Command, bundle *payload.ConfigBundle, withHooks bool) {
	w := cmd.ErrOrStderr()
	fmt.Fprintf(w, "\n%s\n", cCyan("=== Security Summary ==="))

	// Check for hooks
	if len(bundle.SettingsLocal) > 0 {
		var local map[string]json.RawMessage
		if json.Unmarshal(bundle.SettingsLocal, &local) == nil {
			if hooksRaw, ok := local["hooks"]; ok {
				var hooks map[string]json.RawMessage
				if json.Unmarshal(hooksRaw, &hooks) == nil && len(hooks) > 0 {
					if withHooks {
						fmt.Fprintf(w, "%s Hooks will be imported (%d event types):\n",
							cRed("WARNING:"), len(hooks))
						printHookDetails(w, hooks)
					} else {
						fmt.Fprintf(w, "Hooks detected (%d event types) — %s:\n",
							len(hooks), cGreen("STRIPPED for safety"))
						printHookDetails(w, hooks)
						fmt.Fprintf(w, "  %s\n", cDim("Use --with-hooks to include them (only if you trust the source)."))
					}
				}
			}
			if _, ok := local["statusLine"]; ok {
				if withHooks {
					fmt.Fprintf(w, "%s statusLine command will be imported.\n", cRed("WARNING:"))
				} else {
					fmt.Fprintf(w, "statusLine command detected — %s.\n", cGreen("STRIPPED for safety"))
				}
			}
		}
	}

	// Check for skills (prompt injection risk)
	if len(bundle.Skills) > 0 {
		fmt.Fprintf(w, "\n%s\n",
			cYellow(fmt.Sprintf("Skills (%d) — these inject prompts into Claude's context:", len(bundle.Skills))))
		for _, skill := range bundle.Skills {
			if skill.IsSymlink {
				fmt.Fprintf(w, "  - %s %s\n", skill.Name, cDim("(symlink)"))
			} else {
				fmt.Fprintf(w, "  - %s %s\n", skill.Name, cDim(fmt.Sprintf("(%d files)", len(skill.Files))))
			}
		}
	}

	// Check for agents (prompt injection risk)
	if len(bundle.Agents) > 0 {
		fmt.Fprintf(w, "\n%s\n",
			cYellow(fmt.Sprintf("Agents (%d) — these inject prompts into Claude's context:", len(bundle.Agents))))
		for _, agent := range bundle.Agents {
			fmt.Fprintf(w, "  - %s\n", agent.Name)
		}
	}

	// Check for MCP servers (traffic redirect risk)
	if len(bundle.UserMCPConfig) > 0 {
		var obj map[string]json.RawMessage
		if json.Unmarshal(bundle.UserMCPConfig, &obj) == nil {
			if servers, ok := obj["mcpServers"]; ok {
				var srvMap map[string]json.RawMessage
				if json.Unmarshal(servers, &srvMap) == nil && len(srvMap) > 0 {
					fmt.Fprintf(w, "\n%s\n",
						cYellow(fmt.Sprintf("User MCP servers (%d) — these handle tool calls:", len(srvMap))))
					for name, cfg := range srvMap {
						fmt.Fprintf(w, "  - %s %s\n", name, cDim(extractMCPType(cfg)))
					}
				}
			}
		}
	}

	fmt.Fprintf(w, "%s\n", cCyan("========================"))
}

func printHookDetails(w io.Writer, hooks map[string]json.RawMessage) {
	for eventType, hookList := range hooks {
		var entries []struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		}
		if json.Unmarshal(hookList, &entries) == nil {
			for _, entry := range entries {
				for _, h := range entry.Hooks {
					preview := h.Command
					if len(preview) > 80 {
						preview = preview[:80] + "..."
					}
					fmt.Fprintf(w, "  %s %s: %s\n",
						cRedDim("["+eventType+"]"),
						entry.Matcher,
						cDim(preview))
				}
			}
		}
	}
}

func extractMCPType(cfg json.RawMessage) string {
	var obj map[string]json.RawMessage
	if json.Unmarshal(cfg, &obj) != nil {
		return ""
	}
	var typ string
	if t, ok := obj["type"]; ok {
		_ = json.Unmarshal(t, &typ)
	}
	info := "(" + typ
	if urlRaw, ok := obj["url"]; ok {
		var u string
		_ = json.Unmarshal(urlRaw, &u)
		if u != "" {
			info += " " + u
		}
	}
	if cmdRaw, ok := obj["command"]; ok {
		var c string
		_ = json.Unmarshal(cmdRaw, &c)
		if c != "" {
			info += " " + c
		}
	}
	info += ")"
	return info
}

func printImportResult(cmd *cobra.Command, result *config.WriteResult, bundle *payload.ConfigBundle) {
	w := cmd.ErrOrStderr()
	fmt.Fprintf(w, "\n%s\n", cCyan("--- Import Complete ---"))
	fmt.Fprintf(w, "Files written: %s\n", cBold(len(result.FilesWritten)))
	for _, f := range result.FilesWritten {
		fmt.Fprintf(w, "  %s\n", cGreen(f))
	}

	if len(result.SkillsWritten) > 0 {
		fmt.Fprintf(w, "Skills written: %s\n", cBold(len(result.SkillsWritten)))
		for _, s := range result.SkillsWritten {
			fmt.Fprintf(w, "  %s\n", cGreen(s))
		}
	}

	if len(result.AgentsWritten) > 0 {
		fmt.Fprintf(w, "Agents written: %s\n", cBold(len(result.AgentsWritten)))
		for _, a := range result.AgentsWritten {
			fmt.Fprintf(w, "  %s\n", cGreen(a))
		}
	}

	if len(result.PluginsInstalled) > 0 {
		fmt.Fprintf(w, "\nPlugins installed: %s\n", cGreen(fmt.Sprintf("%d", len(result.PluginsInstalled))))
	}
	if len(result.PluginsFailed) > 0 {
		fmt.Fprintf(w, "\n%s\n", cRed(fmt.Sprintf("Plugins failed to install (%d):", len(result.PluginsFailed))))
		for _, p := range result.PluginsFailed {
			fmt.Fprintf(w, "  - %s\n", p)
		}
		fmt.Fprintf(w, "%s\n", cDim("Try installing manually: claude plugin install <name>"))
	}

	if len(result.HooksStripped) > 0 {
		fmt.Fprintf(w, "\nHooks stripped for safety: %s\n", cGreen(strings.Join(result.HooksStripped, ", ")))
		fmt.Fprintf(w, "%s\n", cDim("Re-run with --with-hooks if you trust this config source."))
	}

	if len(result.RedactedServers) > 0 {
		fmt.Fprintf(w, "\n%s\n", cRed("WARNING: The following MCP servers have redacted credentials:"))
		for _, s := range result.RedactedServers {
			fmt.Fprintf(w, "  - %s\n", cYellow(s))
		}
		fmt.Fprintf(w, "%s\n", cDim("You must manually configure their secrets before they will work."))
	}

	if len(result.Warnings) > 0 {
		fmt.Fprintf(w, "\n%s\n", cYellow("Warnings:"))
		for _, warn := range result.Warnings {
			fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", warn)
		}
	}

	fmt.Fprintf(w, "\n%s\n", cGreen("Restart Claude Code to pick up changes."))
}

func countPlugins(bundle *payload.ConfigBundle) int {
	count := 0
	for _, installs := range bundle.Plugins.Plugins {
		count += len(installs)
	}
	return count
}

func readFromClipboard() (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbpaste")
	case "linux":
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard", "-o")
		} else {
			cmd = exec.Command("xsel", "--clipboard", "--output")
		}
	default:
		return "", fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		// Try xclip first, fall back to xsel
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		}
	default:
		return fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
