# Design: Agent Export/Import Support

**Date:** 2026-03-09
**Branch:** support_subagents

## Problem

`claude-config-portable` exports and imports Claude Code configuration (settings, plugins, skills, MCP configs) but does not include sub-agents defined in `~/.claude/agents/`. Users who have custom agents lose them when moving between machines.

## Decision

Export all agents from `~/.claude/agents/*.md`. Allow exclusion via `--no-agents` flag on export. Mirror the existing skills pattern exactly.

## Architecture

Add an `internal/agents` package mirroring `internal/skills`. Add `AgentEntry` to `payload.ConfigBundle`. Wire into `ReadBundle` (export) and `WriteBundle` (import). Add `--no-agents` flag to the export CLI command.

## Components

### `internal/agents/collector.go`
- `Collect(agentsDir string) ([]AgentEntry, error)` — walks `~/.claude/agents/`, reads all `.md` files
- Only collects files with `.md` extension (excludes `.md.backup-*` and other non-agent files)
- Returns nil if directory does not exist (same as skills)

### `internal/payload/types.go`
- Add `AgentEntry` struct: `Name string`, `Content string`
- Add `Agents []AgentEntry` to `ConfigBundle`

### `internal/config/paths.go`
- Add `AgentsDir string` to `Paths`, set to `~/.claude/agents`

### `internal/config/reader.go`
- Add `NoAgents bool` to `ReadOptions`
- Call `agents.Collect(paths.AgentsDir)` when `!opts.NoAgents`

### `internal/config/writer.go`
- Add `ComponentAgents ImportComponent = "agents"` constant
- Include `ComponentAgents` in `AllComponents()`
- Add `AgentsWritten []string` to `WriteResult`
- On import: write each agent's content to `~/.claude/agents/<Name>`

### `cmd/claude-config` (export command)
- Add `--no-agents` flag, pass through to `ReadOptions`

## Data Flow

**Export:**
```
~/.claude/agents/*.md → agents.Collect() → ConfigBundle.Agents → encoded string
```

**Import:**
```
encoded string → ConfigBundle.Agents → write .md files → ~/.claude/agents/
```

## Edge Cases

| Case | Behavior |
|------|----------|
| `~/.claude/agents/` does not exist | Return nil, no error |
| Backup files (`*.backup-*`) | Skip — only collect `*.md` |
| Agent already exists on import | Overwrite (force) or merge per WriteMode |
| `--only agents` / `--skip agents` | Respected via `shouldImport(ComponentAgents, opts)` |

## What Is NOT Included

- No filtering of plugin-provided agents — all `.md` files in `~/.claude/agents/` are exported
- No secrets handling — agent `.md` files are treated as plain text
