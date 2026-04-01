package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nizanrosh/claude-config-portable/internal/cursor"
	"github.com/nizanrosh/claude-config-portable/internal/payload"
	"github.com/nizanrosh/claude-config-portable/internal/secrets"
	"github.com/spf13/cobra"
)

func cursorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cursor",
		Short: "Export and import Cursor IDE configuration",
		Long:  "Manage Cursor IDE setup (settings, rules, extensions, MCPs, skills, commands) as a portable string.",
	}

	cmd.AddCommand(cursorExportCmd(), cursorImportCmd(), cursorInspectCmd())
	return cmd
}

func cursorExportCmd() *cobra.Command {
	var (
		withSecrets bool
		noSkills    bool
		noCommands  bool
		output      string
		copyClip    bool
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export Cursor IDE config as a portable string",
		RunE: func(cmd *cobra.Command, args []string) error {
			bundle, err := cursor.ReadCursorBundle(cursor.ReadOptions{
				WithSecrets: withSecrets,
				NoSkills:    noSkills,
				NoCommands:  noCommands,
			})
			if err != nil {
				return err
			}

			encoded, err := payload.EncodeCursor(bundle)
			if err != nil {
				return fmt.Errorf("encoding bundle: %w", err)
			}

			if output != "" {
				if err := writeToFile(output, encoded); err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Cursor config exported to %s\n", cGreen(output))
			} else if copyClip {
				if err := copyToClipboard(encoded); err != nil {
					return fmt.Errorf("copying to clipboard: %w", err)
				}
				fmt.Fprintln(cmd.ErrOrStderr(), cGreen("Cursor config copied to clipboard!"))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), encoded)
			}

			printCursorExportSummary(cmd, bundle)
			return nil
		},
	}

	cmd.Flags().BoolVar(&withSecrets, "with-secrets", false, "Include MCP server secrets (headers, env vars, OAuth tokens)")
	cmd.Flags().BoolVar(&noSkills, "no-skills", false, "Exclude user-created skills")
	cmd.Flags().BoolVar(&noCommands, "no-commands", false, "Exclude custom commands")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write to file instead of stdout")
	cmd.Flags().BoolVarP(&copyClip, "copy", "c", false, "Copy to clipboard instead of printing")

	return cmd
}

func cursorImportCmd() *cobra.Command {
	var (
		force         bool
		doMerge       bool
		dryRun        bool
		fromClipboard bool
		only          []string
		skip          []string
	)

	cmd := &cobra.Command{
		Use:   "import [string-or-file]",
		Short: "Import Cursor IDE config from a portable string, file, or clipboard",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input, err := resolveCursorInput(args, fromClipboard)
			if err != nil {
				return err
			}

			bundle, err := payload.DecodeCursor(input)
			if err != nil {
				return fmt.Errorf("decoding config: %w", err)
			}

			printCursorSecuritySummary(cmd, bundle)

			mode := cursor.WriteModeForce
			if doMerge {
				mode = cursor.WriteModeMerge
			}

			writeOpts := cursor.WriteOptions{
				Mode:   mode,
				DryRun: dryRun,
				Only:   only,
				Skip:   skip,
			}

			if dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), cYellow("=== DRY RUN -- no changes will be written ==="))
				printCursorInspection(cmd, bundle)
				result, err := cursor.WriteCursorBundle(bundle, writeOpts)
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
				if err := cursor.CheckCursorConflicts(bundle); err != nil {
					return err
				}
			}

			result, err := cursor.WriteCursorBundle(bundle, writeOpts)
			if err != nil {
				return err
			}

			printCursorImportResult(cmd, result, bundle)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config files")
	cmd.Flags().BoolVar(&doMerge, "merge", false, "Deep-merge with existing config (incoming wins on conflicts)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would change without writing")
	cmd.Flags().BoolVar(&fromClipboard, "from-clipboard", false, "Read config from clipboard instead of argument")
	cmd.Flags().StringSliceVar(&only, "only", nil, "Only import these components (settings,keybindings,snippets,rules,mcp,extensions,skills,commands,cli-config)")
	cmd.Flags().StringSliceVar(&skip, "skip", nil, "Skip these components during import")

	return cmd
}

func cursorInspectCmd() *cobra.Command {
	var fromClipboard bool

	cmd := &cobra.Command{
		Use:   "inspect [string-or-file]",
		Short: "Show a human-readable summary of a Cursor config export",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input, err := resolveCursorInput(args, fromClipboard)
			if err != nil {
				return err
			}

			bundle, err := payload.DecodeCursor(input)
			if err != nil {
				return fmt.Errorf("decoding config: %w", err)
			}

			printCursorInspection(cmd, bundle)
			return nil
		},
	}

	cmd.Flags().BoolVar(&fromClipboard, "from-clipboard", false, "Read config from clipboard instead of argument")
	return cmd
}

func resolveCursorInput(args []string, fromClipboard bool) (string, error) {
	if fromClipboard {
		input, err := readFromClipboard()
		if err != nil {
			return "", fmt.Errorf("reading from clipboard: %w", err)
		}
		return strings.TrimSpace(input), nil
	}
	if len(args) == 0 {
		return "", fmt.Errorf("provide a config string, file path, or use --from-clipboard")
	}
	arg := args[0]
	if strings.HasPrefix(arg, payload.CursorPrefix) {
		return arg, nil
	}
	data, err := os.ReadFile(arg)
	if err != nil {
		return "", fmt.Errorf("reading file %s: %w", arg, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func printCursorExportSummary(cmd *cobra.Command, bundle *payload.CursorConfigBundle) {
	w := cmd.ErrOrStderr()
	fmt.Fprintf(w, "\n%s\n", cCyan("--- Cursor Export Summary ---"))
	fmt.Fprintf(w, "Rules:       %s\n", cBold(len(bundle.Rules)))
	fmt.Fprintf(w, "Skills:      %s\n", cBold(len(bundle.Skills)))
	fmt.Fprintf(w, "Commands:    %s\n", cBold(len(bundle.Commands)))
	fmt.Fprintf(w, "Extensions:  %s\n", cBold(countCursorExtensions(bundle)))

	hasMCP := len(bundle.MCPConfig) > 0
	if hasMCP {
		fmt.Fprintf(w, "MCP config:  %s\n", cGreen("included"))
	}
	if bundle.SecretsIncluded {
		fmt.Fprintf(w, "Secrets:     %s\n", cRed("INCLUDED (handle with care!)"))
	} else {
		fmt.Fprintf(w, "Secrets:     %s\n", cGreen("stripped"))
	}
}

func printCursorInspection(cmd *cobra.Command, bundle *payload.CursorConfigBundle) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s\n", cCyan("=== Cursor Config Inspection ==="))
	fmt.Fprintf(w, "Schema version: %s\n", cBold(bundle.Version))
	fmt.Fprintf(w, "Created:        %s\n", cDim(bundle.CreatedAt))
	fmt.Fprintf(w, "Platform:       %s\n", cDim(bundle.Platform))
	if bundle.SecretsIncluded {
		fmt.Fprintf(w, "Secrets:        %s\n", cRed("true"))
	} else {
		fmt.Fprintf(w, "Secrets:        %s\n", cGreen("false"))
	}

	if len(bundle.Settings) > 0 {
		printCursorSettingsHighlights(w, bundle.Settings)
	}

	if len(bundle.Keybindings) > 0 {
		var bindings []json.RawMessage
		if json.Unmarshal(bundle.Keybindings, &bindings) == nil {
			fmt.Fprintf(w, "\n%s\n", cYellow(fmt.Sprintf("Keybindings (%d custom):", len(bindings))))
		}
	}

	if len(bundle.Snippets) > 0 {
		fmt.Fprintf(w, "\n%s\n", cYellow(fmt.Sprintf("Snippets (%d files):", len(bundle.Snippets))))
		for name := range bundle.Snippets {
			fmt.Fprintf(w, "  - %s\n", name)
		}
	}

	if len(bundle.Rules) > 0 {
		fmt.Fprintf(w, "\n%s\n", cYellow(fmt.Sprintf("Rules (%d):", len(bundle.Rules))))
		for _, rule := range bundle.Rules {
			fmt.Fprintf(w, "  - %s\n", rule.Name)
		}
	}

	if len(bundle.MCPConfig) > 0 {
		printCursorMCPDetails(w, bundle.MCPConfig)
	}

	extCount := countCursorExtensions(bundle)
	if extCount > 0 {
		fmt.Fprintf(w, "\n%s\n", cYellow(fmt.Sprintf("Extensions (%d):", extCount)))
		printExtensionList(w, bundle.Extensions)
	}

	if len(bundle.Skills) > 0 {
		fmt.Fprintf(w, "\n%s\n", cYellow(fmt.Sprintf("Skills (%d):", len(bundle.Skills))))
		for _, skill := range bundle.Skills {
			var kind string
			if skill.IsSymlink {
				kind = fmt.Sprintf("symlink -> %s", skill.LinkTarget)
			} else {
				kind = fmt.Sprintf("%d files", len(skill.Files))
			}
			fmt.Fprintf(w, "  - %s %s\n", skill.Name, cDim("("+kind+")"))
		}
	}

	if len(bundle.Commands) > 0 {
		fmt.Fprintf(w, "\n%s\n", cYellow(fmt.Sprintf("Commands (%d):", len(bundle.Commands))))
		for _, cmd := range bundle.Commands {
			fmt.Fprintf(w, "  - %s\n", cmd.Name)
		}
	}

	if len(bundle.CLIConfig) > 0 {
		fmt.Fprintf(w, "\n%s\n", cYellow("CLI Config:"))
		fmt.Fprintf(w, "  %s\n", cGreen("included"))
	}
}

func printCursorSecuritySummary(cmd *cobra.Command, bundle *payload.CursorConfigBundle) {
	w := cmd.ErrOrStderr()
	fmt.Fprintf(w, "\n%s\n", cCyan("=== Security Summary ==="))

	if len(bundle.Rules) > 0 {
		fmt.Fprintf(w, "%s\n",
			cYellow(fmt.Sprintf("Rules (%d) -- these inject prompts into Cursor's context:", len(bundle.Rules))))
		for _, rule := range bundle.Rules {
			fmt.Fprintf(w, "  - %s\n", rule.Name)
		}
	}

	if len(bundle.Skills) > 0 {
		fmt.Fprintf(w, "\n%s\n",
			cYellow(fmt.Sprintf("Skills (%d) -- these inject prompts into Cursor's context:", len(bundle.Skills))))
		for _, skill := range bundle.Skills {
			if skill.IsSymlink {
				fmt.Fprintf(w, "  - %s %s\n", skill.Name, cDim("(symlink)"))
			} else {
				fmt.Fprintf(w, "  - %s %s\n", skill.Name, cDim(fmt.Sprintf("(%d files)", len(skill.Files))))
			}
		}
	}

	if len(bundle.Commands) > 0 {
		fmt.Fprintf(w, "\n%s\n",
			cYellow(fmt.Sprintf("Commands (%d) -- these inject prompts into Cursor's context:", len(bundle.Commands))))
		for _, cmd := range bundle.Commands {
			fmt.Fprintf(w, "  - %s\n", cmd.Name)
		}
	}

	if len(bundle.MCPConfig) > 0 {
		var obj map[string]json.RawMessage
		if json.Unmarshal(bundle.MCPConfig, &obj) == nil {
			if servers, ok := obj["mcpServers"]; ok {
				var srvMap map[string]json.RawMessage
				if json.Unmarshal(servers, &srvMap) == nil && len(srvMap) > 0 {
					fmt.Fprintf(w, "\n%s\n",
						cYellow(fmt.Sprintf("MCP servers (%d) -- these handle tool calls:", len(srvMap))))
					for name, cfg := range srvMap {
						fmt.Fprintf(w, "  - %s %s\n", name, cDim(extractMCPType(cfg)))
					}
				}
			}
		}
	}

	fmt.Fprintf(w, "%s\n", cCyan("========================"))
}

func printCursorImportResult(cmd *cobra.Command, result *cursor.WriteResult, bundle *payload.CursorConfigBundle) {
	w := cmd.ErrOrStderr()
	fmt.Fprintf(w, "\n%s\n", cCyan("--- Cursor Import Complete ---"))
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

	if len(result.CommandsWritten) > 0 {
		fmt.Fprintf(w, "Commands written: %s\n", cBold(len(result.CommandsWritten)))
		for _, c := range result.CommandsWritten {
			fmt.Fprintf(w, "  %s\n", cGreen(c))
		}
	}

	if len(result.ExtensionsInstalled) > 0 {
		fmt.Fprintf(w, "\nExtensions installed: %s\n", cGreen(fmt.Sprintf("%d", len(result.ExtensionsInstalled))))
	}
	if len(result.ExtensionsFailed) > 0 {
		fmt.Fprintf(w, "\n%s\n", cRed(fmt.Sprintf("Extensions failed to install (%d):", len(result.ExtensionsFailed))))
		for _, e := range result.ExtensionsFailed {
			fmt.Fprintf(w, "  - %s\n", e)
		}
		fmt.Fprintf(w, "%s\n", cDim("Try installing manually: cursor --install-extension <id>"))
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
			fmt.Fprintf(w, "  %s\n", warn)
		}
	}

	fmt.Fprintf(w, "\n%s\n", cGreen("Restart Cursor to pick up changes."))
}

func printCursorSettingsHighlights(w io.Writer, data json.RawMessage) {
	var settings map[string]json.RawMessage
	if json.Unmarshal(data, &settings) != nil {
		return
	}
	fmt.Fprintf(w, "\n%s\n", cYellow("Settings highlights:"))
	for _, key := range []string{
		"editor.fontSize",
		"editor.fontFamily",
		"workbench.colorTheme",
		"cursor.composer.model",
		"cursor.cpp.enablePartialAccepts",
	} {
		if val, ok := settings[key]; ok {
			fmt.Fprintf(w, "  %s: %s\n", key, cGreen(string(val)))
		}
	}
}

func printCursorMCPDetails(w io.Writer, data json.RawMessage) {
	var obj map[string]json.RawMessage
	if json.Unmarshal(data, &obj) != nil {
		return
	}
	servers, ok := obj["mcpServers"]
	if !ok {
		return
	}
	var srvMap map[string]json.RawMessage
	if json.Unmarshal(servers, &srvMap) != nil {
		return
	}
	fmt.Fprintf(w, "\n%s\n", cYellow(fmt.Sprintf("MCP Servers (%d):", len(srvMap))))
	for name, cfg := range srvMap {
		redacted := ""
		if secrets.HasRedactedValues(cfg) {
			redacted = " " + cRedDim("[secrets redacted]")
		}
		fmt.Fprintf(w, "  - %s %s%s\n", name, cDim(extractMCPType(cfg)), redacted)
	}
}

func printExtensionList(w io.Writer, data json.RawMessage) {
	var extensions []struct {
		Identifier struct {
			ID string `json:"id"`
		} `json:"identifier"`
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &extensions) != nil {
		return
	}
	for _, ext := range extensions {
		if ext.Identifier.ID != "" {
			fmt.Fprintf(w, "  - %s %s\n", ext.Identifier.ID, cDim("(v"+ext.Version+")"))
		}
	}
}

func countCursorExtensions(bundle *payload.CursorConfigBundle) int {
	if len(bundle.Extensions) == 0 {
		return 0
	}
	var extensions []json.RawMessage
	if json.Unmarshal(bundle.Extensions, &extensions) != nil {
		return 0
	}
	return len(extensions)
}

func writeToFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
