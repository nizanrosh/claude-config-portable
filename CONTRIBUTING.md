# Contributing

Thanks for your interest in contributing to claude-config-portable!

## Development

```bash
git clone https://github.com/nizanrosh/claude-config-portable.git
cd claude-config-portable
go build -o bin/claude-config ./cmd/claude-config
go test ./...
```

Requires Go 1.22+.

## Making changes

1. Fork the repo and create a branch from `main`
2. Make your changes
3. Add or update tests as needed
4. Run `go test ./...` and `go vet ./...`
5. Open a pull request

## Guidelines

- Keep PRs focused — one feature or fix per PR
- Follow existing code style (standard Go conventions)
- Add tests for new functionality
- Update the README if you're adding user-facing features

## Reporting issues

Open an issue on GitHub with:
- What you expected to happen
- What actually happened
- Steps to reproduce
- `claude-config version` output

## Security

If you find a security vulnerability, please see [SECURITY.md](SECURITY.md) instead of opening a public issue.
