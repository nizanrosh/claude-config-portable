package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/nizanrosh/claude-config-portable/internal/config"
	"github.com/nizanrosh/claude-config-portable/internal/payload"
	"github.com/nizanrosh/claude-config-portable/internal/secrets"
	"github.com/spf13/cobra"
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
				fmt.Fprintf(cmd.ErrOrStderr(), "Config exported to %s\n", output)
			} else if copyClip {
				if err := copyToClipboard(encoded); err != nil {
					return fmt.Errorf("copying to clipboard: %w", err)
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "Config copied to clipboard!")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), encoded)
			}

			printExportSummary(cmd, bundle)
			return nil
		},
	}

	cmd.Flags().BoolVar(&withSecrets, "with-secrets", false, "Include MCP server secrets (headers, env vars, OAuth tokens)")
	cmd.Flags().BoolVar(&noSkills, "no-skills", false, "Exclude user-created skills")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write to file instead of stdout")
	cmd.Flags().BoolVarP(&copyClip, "copy", "c", false, "Copy to clipboard instead of printing")

	return cmd
}

func importCmd() *cobra.Command {
	var (
		force  bool
		doMerge bool
		dryRun bool
	)

	cmd := &cobra.Command{
		Use:   "import <string-or-file>",
		Short: "Import Claude Code config from a portable string or file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input, err := resolveInput(args[0])
			if err != nil {
				return err
			}

			bundle, err := payload.Decode(input)
			if err != nil {
				return fmt.Errorf("decoding config: %w", err)
			}

			if dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "=== DRY RUN — no changes will be written ===")
				printInspection(cmd, bundle)
				result, err := config.WriteBundle(bundle, config.WriteOptions{
					Mode:   config.WriteModeForce,
					DryRun: true,
				})
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

			mode := config.WriteModeForce
			if doMerge {
				mode = config.WriteModeMerge
			}

			result, err := config.WriteBundle(bundle, config.WriteOptions{Mode: mode})
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

	return cmd
}

func inspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <string-or-file>",
		Short: "Show a human-readable summary of a config export",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input, err := resolveInput(args[0])
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
	fmt.Fprintln(w, "\n--- Export Summary ---")
	fmt.Fprintf(w, "Plugins:  %d\n", countPlugins(bundle))
	fmt.Fprintf(w, "Skills:   %d\n", len(bundle.Skills))
	fmt.Fprintf(w, "MCP configs: %d\n", len(bundle.MCPConfigs))
	if len(bundle.UserMCPConfig) > 0 {
		fmt.Fprintln(w, "User MCP: included")
	}
	if bundle.SecretsIncluded {
		fmt.Fprintln(w, "Secrets:  INCLUDED (handle with care!)")
	} else {
		fmt.Fprintln(w, "Secrets:  stripped")
	}
}

func printInspection(cmd *cobra.Command, bundle *payload.ConfigBundle) {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "=== Claude Config Inspection ===")
	fmt.Fprintf(w, "Schema version: %d\n", bundle.Version)
	fmt.Fprintf(w, "Created:        %s\n", bundle.CreatedAt)
	fmt.Fprintf(w, "Platform:       %s\n", bundle.Platform)
	fmt.Fprintf(w, "Secrets:        %v\n", bundle.SecretsIncluded)

	fmt.Fprintf(w, "\nPlugins (%d):\n", countPlugins(bundle))
	for name, installs := range bundle.Plugins.Plugins {
		for _, pi := range installs {
			fmt.Fprintf(w, "  - %s (v%s, scope: %s)\n", name, pi.Version, pi.Scope)
		}
	}

	if len(bundle.MCPConfigs) > 0 {
		fmt.Fprintf(w, "\nMCP Configs (%d):\n", len(bundle.MCPConfigs))
		for key, cfg := range bundle.MCPConfigs {
			redacted := ""
			if secrets.HasRedactedValues(cfg) {
				redacted = " [secrets redacted]"
			}
			fmt.Fprintf(w, "  - %s%s\n", key, redacted)
		}
	}

	if len(bundle.UserMCPConfig) > 0 {
		fmt.Fprintln(w, "\nUser MCP Config: present")
		var obj map[string]json.RawMessage
		if json.Unmarshal(bundle.UserMCPConfig, &obj) == nil {
			if servers, ok := obj["mcpServers"]; ok {
				var srvMap map[string]json.RawMessage
				if json.Unmarshal(servers, &srvMap) == nil {
					for name, cfg := range srvMap {
						redacted := ""
						if secrets.HasRedactedValues(cfg) {
							redacted = " [secrets redacted]"
						}
						fmt.Fprintf(w, "  - %s%s\n", name, redacted)
					}
				}
			}
		}
	}

	if len(bundle.Skills) > 0 {
		fmt.Fprintf(w, "\nSkills (%d):\n", len(bundle.Skills))
		for _, skill := range bundle.Skills {
			kind := "dir"
			if skill.IsSymlink {
				kind = fmt.Sprintf("symlink → %s", skill.LinkTarget)
			} else {
				kind = fmt.Sprintf("%d files", len(skill.Files))
			}
			fmt.Fprintf(w, "  - %s (%s)\n", skill.Name, kind)
		}
	}

	// Settings highlights
	if len(bundle.Settings) > 0 {
		var settings map[string]json.RawMessage
		if json.Unmarshal(bundle.Settings, &settings) == nil {
			fmt.Fprintln(w, "\nSettings highlights:")
			for _, key := range []string{"model", "effortLevel", "enabledPlugins"} {
				if val, ok := settings[key]; ok {
					fmt.Fprintf(w, "  %s: %s\n", key, string(val))
				}
			}
		}
	}

	if len(bundle.Marketplaces) > 0 {
		var marketplaces []map[string]json.RawMessage
		if json.Unmarshal(bundle.Marketplaces, &marketplaces) == nil && len(marketplaces) > 0 {
			fmt.Fprintf(w, "\nMarketplaces (%d):\n", len(marketplaces))
			for _, m := range marketplaces {
				var name string
				if n, ok := m["name"]; ok {
					json.Unmarshal(n, &name)
				}
				var id string
				if i, ok := m["id"]; ok {
					json.Unmarshal(i, &id)
				}
				if name != "" {
					fmt.Fprintf(w, "  - %s (%s)\n", name, id)
				} else if id != "" {
					fmt.Fprintf(w, "  - %s\n", id)
				}
			}
		}
	}
}

func printImportResult(cmd *cobra.Command, result *config.WriteResult, bundle *payload.ConfigBundle) {
	w := cmd.ErrOrStderr()
	fmt.Fprintln(w, "\n--- Import Complete ---")
	fmt.Fprintf(w, "Files written: %d\n", len(result.FilesWritten))
	for _, f := range result.FilesWritten {
		fmt.Fprintf(w, "  %s\n", f)
	}

	if len(result.SkillsWritten) > 0 {
		fmt.Fprintf(w, "Skills written: %d\n", len(result.SkillsWritten))
		for _, s := range result.SkillsWritten {
			fmt.Fprintf(w, "  %s\n", s)
		}
	}

	if len(result.PluginsInstalled) > 0 {
		fmt.Fprintf(w, "\nPlugins installed: %d\n", len(result.PluginsInstalled))
	}
	if len(result.PluginsFailed) > 0 {
		fmt.Fprintf(w, "\nPlugins failed to install (%d):\n", len(result.PluginsFailed))
		for _, p := range result.PluginsFailed {
			fmt.Fprintf(w, "  - %s\n", p)
		}
		fmt.Fprintln(w, "Try installing manually: claude plugin install <name>")
	}

	if len(result.RedactedServers) > 0 {
		fmt.Fprintln(w, "\nWARNING: The following MCP servers have redacted credentials:")
		for _, s := range result.RedactedServers {
			fmt.Fprintf(w, "  - %s\n", s)
		}
		fmt.Fprintln(w, "You must manually configure their secrets before they will work.")
	}

	if len(result.Warnings) > 0 {
		fmt.Fprintln(w, "\nWarnings:")
		for _, w := range result.Warnings {
			fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", w)
		}
	}

	fmt.Fprintln(w, "\nRestart Claude Code to pick up changes.")
}

func countPlugins(bundle *payload.ConfigBundle) int {
	count := 0
	for _, installs := range bundle.Plugins.Plugins {
		count += len(installs)
	}
	return count
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
