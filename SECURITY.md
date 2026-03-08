# Security Policy

## Reporting a vulnerability

If you discover a security vulnerability in claude-config-portable, please report it responsibly.

**Do not open a public GitHub issue.**

Instead, email **nizanrosh@gmail.com** with:
- A description of the vulnerability
- Steps to reproduce
- Potential impact

You should receive a response within 48 hours.

## Security considerations

This tool handles Claude Code configuration which may include:

- **Hooks** — shell commands that execute automatically. Stripped on import by default.
- **Skills** — prompt text injected into Claude's context. Inspect before importing from untrusted sources.
- **MCP server configs** — define endpoints that handle tool calls. Could redirect traffic to malicious servers.
- **MCP credentials** — headers, env vars, OAuth tokens. Stripped by default unless `--with-secrets` is used.

### Recommendations

- Always use `claude-config inspect` before importing config from others
- Never use `--with-hooks` on config from untrusted sources
- Use `--only settings,plugins` when importing from unfamiliar sources
- Use `--with-secrets` only over secure channels (e.g., between your own machines)
