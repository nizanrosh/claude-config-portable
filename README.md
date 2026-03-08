# claude-config-portable

Portable CLI to export and import your entire Claude Code setup (plugins, skills, MCPs, settings) as a single string.

Share your Claude Code configuration between machines or teammates — no manual copying of dotfiles required.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/nizanrosh/claude-config-portable/main/install.sh | bash
```

## What Gets Exported

| Config | Source |
|--------|--------|
| Global settings (model, plugins, effort level) | `~/.claude/settings.json` |
| Permissions & hooks | `~/.claude/settings.local.json` |
| Installed plugins | `~/.claude/plugins/installed_plugins.json` |
| Custom marketplaces | `~/.claude/plugins/known_marketplaces.json` |
| MCP server configs | `.mcp.json` inside each plugin install path |
| User-level MCP config | `~/.claude/.mcp.json` |
| User-created skills | `~/.claude/skills/*/` |

MCP credentials (headers, env vars, OAuth tokens) are **stripped by default**. Use `--with-secrets` to include them.

## Usage

### Export

```bash
# Print portable config string to stdout
claude-config export

# Save to file
claude-config export -o my-setup.txt

# Include MCP secrets (handle with care)
claude-config export --with-secrets

# Exclude skills
claude-config export --no-skills
```

### Import

```bash
# Import from string
claude-config import 'ccfg:1:...'

# Import from file
claude-config import my-setup.txt

# Preview what would change
claude-config import my-setup.txt --dry-run

# Overwrite existing config
claude-config import my-setup.txt --force

# Deep-merge with existing config (incoming wins on conflicts)
claude-config import my-setup.txt --merge
```

### Inspect

```bash
# See what's inside a config string without importing
claude-config inspect my-setup.txt
```

Example output:

```
=== Claude Config Inspection ===
Schema version: 1
Created:        2026-03-08T15:28:04Z
Platform:       darwin
Secrets:        false

Plugins (36):
  - superpowers@claude-plugins-official (v4.3.1, scope: user)
  - github@claude-plugins-official (v205b6e0b3036, scope: user)
  ...

MCP Configs (15):
  - claude-plugins-official/slack/1.0.0 [secrets redacted]
  ...

Skills (4):
  - golang-backend (1 files)
  - find-skills (symlink → ../../.agents/skills/find-skills)
  ...

Settings highlights:
  model: "opus"
  effortLevel: "high"
```

## Wire Format

```
ccfg:1:<base64-encoded gzipped JSON>
```

- `ccfg:` — magic prefix for recognition
- `1:` — schema version for forward compatibility
- Payload: JSON → gzip → base64

## Secret Handling

By default, MCP configs are exported with sensitive blocks replaced by `__CONFIGURE_AFTER_IMPORT__`:

- `headers` — often contains `Authorization: Bearer ...`
- `env` — environment variables with API keys
- `oauth` — OAuth tokens
- URL query parameters — may contain `?token=...`

On import, you'll see a warning listing which MCP servers need manual credential setup.

Use `--with-secrets` to preserve credentials in the export (e.g., for your own machines over a secure channel).

## Build from Source

```bash
git clone https://github.com/nizanrosh/claude-config-portable.git
cd claude-config-portable
make build
./bin/claude-config --help
```

## License

MIT
