# Changelog

## v0.4.0 — Security hardening

- Strip hooks and statusLine from `settings.local.json` on import by default (`--with-hooks` to include)
- Add `--only` and `--skip` flags for selective component import
- Print security summary before every import (hooks, skills, MCP servers)
- Enhance `inspect` to show hook commands, statusLine, skill files, and MCP URLs with `[SECURITY RISK]` labels
- Fix MCP secret filtering for plugin configs using named-server format
- Add tests for named-server format filtering

## v0.3.0

- Skip dangling skill symlinks with warning instead of failing
- Add `--copy`/`-c` flag to copy export to clipboard

## v0.2.0

- Add `lastUpdated` to reconstructed marketplace config (fixes Claude Code crash)
- Fix marketplace JSON parsing (map format, not array)
- Run `claude plugin marketplace update` before installing plugins
- Install plugins via `claude plugin install` CLI instead of relying on settings sync

## v0.1.1

- Version bump for initial testing

## v0.1.0

- Initial release
- Export/import Claude Code config as portable `ccfg:1:` strings
- Secret stripping for MCP headers, env, oauth, URL query params
- Plugin installation via Claude CLI on import
- Skill export with symlink support
- `inspect` command for auditing config strings
- `--dry-run`, `--force`, `--merge` import modes
- Cross-platform install script (`curl | bash`)
