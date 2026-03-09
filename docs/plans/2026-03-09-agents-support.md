# Agent Export/Import Support Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add export and import of `~/.claude/agents/*.md` files to `claude-config-portable`, with a `--no-agents` flag to exclude them.

**Architecture:** Mirror the existing `internal/skills` pattern exactly. Add an `internal/agents` package with a `Collect` function, add `AgentEntry` to the payload types, wire into `ReadBundle`/`WriteBundle`, and expose `--no-agents` on the export CLI command.

**Tech Stack:** Go 1.22+, cobra (CLI), standard library only (no new deps)

---

### Task 1: Add `AgentEntry` to payload types and update the round-trip test

**Files:**
- Modify: `internal/payload/types.go`
- Modify: `internal/payload/encode_test.go`

**Step 1: Add `AgentEntry` struct and `Agents` field to `ConfigBundle`**

In `internal/payload/types.go`, add after the `SkillEntry` struct:

```go
// AgentEntry represents a user-created agent file from ~/.claude/agents/.
type AgentEntry struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}
```

And add `Agents []AgentEntry` to `ConfigBundle` after the `Skills` field:

```go
Agents []AgentEntry `json:"agents,omitempty"`
```

**Step 2: Run existing tests to confirm nothing is broken**

```bash
go test ./internal/payload/... -v
```
Expected: all PASS

**Step 3: Extend the round-trip test to include an agent**

In `internal/payload/encode_test.go`, add an agent to the bundle in `TestEncodeDecode_RoundTrip`:

```go
Agents: []AgentEntry{
    {Name: "my-agent.md", Content: "---\nname: my-agent\n---\n# Agent"},
},
```

Add assertions after the skills assertions:

```go
if len(decoded.Agents) != 1 {
    t.Errorf("expected 1 agent, got %d", len(decoded.Agents))
}
if decoded.Agents[0].Name != "my-agent.md" {
    t.Errorf("agent name mismatch: got %q", decoded.Agents[0].Name)
}
if decoded.Agents[0].Content != "---\nname: my-agent\n---\n# Agent" {
    t.Errorf("agent content mismatch: got %q", decoded.Agents[0].Content)
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/payload/... -v
```
Expected: PASS — `AgentEntry` round-trips correctly through encode/decode.

**Step 5: Commit**

```bash
git add internal/payload/types.go internal/payload/encode_test.go
git commit -m "feat: add AgentEntry type to payload and extend round-trip test"
```

---

### Task 2: Create `internal/agents` package

**Files:**
- Create: `internal/agents/collector.go`
- Create: `internal/agents/collector_test.go`

**Step 1: Write the failing test**

Create `internal/agents/collector_test.go`:

```go
package agents_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nizanrosh/claude-config-portable/internal/agents"
)

func TestCollect_ReturnsAgentFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "my-agent.md"), []byte("# Agent content"), 0644); err != nil {
		t.Fatal(err)
	}
	// Non-.md file — should be skipped
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0644); err != nil {
		t.Fatal(err)
	}
	// Backup file — should be skipped
	if err := os.WriteFile(filepath.Join(dir, "my-agent.md.backup-20260301-150624"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := agents.Collect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(result))
	}
	if result[0].Name != "my-agent.md" {
		t.Errorf("name mismatch: got %q", result[0].Name)
	}
	if result[0].Content != "# Agent content" {
		t.Errorf("content mismatch: got %q", result[0].Content)
	}
}

func TestCollect_MissingDir_ReturnsNil(t *testing.T) {
	t.Parallel()

	result, err := agents.Collect("/nonexistent/path/agents")
	if err != nil {
		t.Fatalf("expected nil error for missing dir, got: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for missing dir, got: %v", result)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/agents/... -v
```
Expected: FAIL — package does not exist yet.

**Step 3: Implement `internal/agents/collector.go`**

```go
// Package agents collects user-created agent files from ~/.claude/agents/.
package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nizanrosh/claude-config-portable/internal/payload"
)

// Collect walks the agents directory and returns all agent entries.
// Only files with a ".md" extension are collected; backup files are skipped.
func Collect(agentsDir string) ([]payload.AgentEntry, error) {
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading agents directory: %w", err)
	}

	var result []payload.AgentEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		fullPath := filepath.Join(agentsDir, name)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("reading agent file %q: %w", name, err)
		}
		result = append(result, payload.AgentEntry{
			Name:    name,
			Content: string(data),
		})
	}

	return result, nil
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/agents/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agents/collector.go internal/agents/collector_test.go
git commit -m "feat: add internal/agents package with Collect function"
```

---

### Task 3: Add `AgentsDir` to `Paths` and wire into `ReadBundle`

**Files:**
- Modify: `internal/config/paths.go`
- Modify: `internal/config/reader.go`

**Step 1: Add `AgentsDir` to `Paths`**

In `internal/config/paths.go`, add `AgentsDir string` to the `Paths` struct after `SkillsDir`:

```go
AgentsDir string
```

And set it in `ResolvePaths()` after the `SkillsDir` line:

```go
AgentsDir: filepath.Join(base, "agents"),
```

**Step 2: Add `NoAgents` to `ReadOptions` and collect agents in `ReadBundle`**

In `internal/config/reader.go`:

Add `NoAgents bool` to `ReadOptions`:
```go
type ReadOptions struct {
	WithSecrets bool
	NoSkills    bool
	NoAgents    bool
}
```

Add the import for the agents package at the top:
```go
"github.com/nizanrosh/claude-config-portable/internal/agents"
```

After the skills collection block (around line 94), add:
```go
// Collect agents
if !opts.NoAgents {
    agentEntries, err := agents.Collect(paths.AgentsDir)
    if err != nil {
        return nil, fmt.Errorf("collecting agents: %w", err)
    }
    bundle.Agents = agentEntries
}
if bundle.Agents == nil {
    bundle.Agents = []payload.AgentEntry{}
}
```

**Step 3: Build to verify no compile errors**

```bash
go build ./...
```
Expected: no errors

**Step 4: Commit**

```bash
git add internal/config/paths.go internal/config/reader.go
git commit -m "feat: collect agents in ReadBundle, add NoAgents option"
```

---

### Task 4: Wire agents into `WriteBundle`

**Files:**
- Modify: `internal/config/writer.go`

**Step 1: Add `ComponentAgents` constant and `AgentsWritten` to `WriteResult`**

In `internal/config/writer.go`:

Add to the `ImportComponent` constants block:
```go
ComponentAgents ImportComponent = "agents"
```

Update `AllComponents()` to include it:
```go
func AllComponents() []ImportComponent {
	return []ImportComponent{
		ComponentSettings, ComponentHooks, ComponentPermissions,
		ComponentPlugins, ComponentMarketplaces, ComponentMCP, ComponentSkills,
		ComponentAgents,
	}
}
```

Add `AgentsWritten []string` to `WriteResult`:
```go
type WriteResult struct {
	FilesWritten     []string
	SkillsWritten    []string
	AgentsWritten    []string
	PluginsInstalled []string
	PluginsFailed    []string
	RedactedServers  []string
	HooksStripped    []string
	Warnings         []string
}
```

**Step 2: Ensure agents dir exists and write agents on import**

In `WriteBundle`, add `paths.AgentsDir` to the directory creation block:
```go
for _, dir := range []string{
    paths.ClaudeDir,
    filepath.Dir(paths.InstalledPlugins),
    paths.SkillsDir,
    paths.AgentsDir,
} {
```

After the skills writing block (end of `WriteBundle`), add:
```go
// Write agents
if shouldImport(ComponentAgents, opts) {
    for _, agent := range bundle.Agents {
        agentPath := filepath.Join(paths.AgentsDir, agent.Name)
        if opts.DryRun {
            result.AgentsWritten = append(result.AgentsWritten, agent.Name+" (dry run)")
            continue
        }
        if err := os.WriteFile(agentPath, []byte(agent.Content), 0644); err != nil {
            return nil, fmt.Errorf("writing agent file %q: %w", agent.Name, err)
        }
        result.AgentsWritten = append(result.AgentsWritten, agent.Name)
    }
}
```

**Step 3: Build to verify no compile errors**

```bash
go build ./...
```
Expected: no errors

**Step 4: Run all tests**

```bash
go test ./...
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/writer.go
git commit -m "feat: write agents on import, add ComponentAgents constant"
```

---

### Task 5: Add `--no-agents` CLI flag and update output

**Files:**
- Modify: `cmd/claude-config/main.go`

**Step 1: Add `--no-agents` flag to export command**

In `exportCmd()`, add `noAgents bool` to the var block:
```go
var (
    withSecrets bool
    noSkills    bool
    noAgents    bool
    output      string
    copyClip    bool
)
```

Pass it to `ReadOptions`:
```go
bundle, err := config.ReadBundle(config.ReadOptions{
    WithSecrets: withSecrets,
    NoSkills:    noSkills,
    NoAgents:    noAgents,
})
```

Register the flag:
```go
cmd.Flags().BoolVar(&noAgents, "no-agents", false, "Exclude user-created agents")
```

**Step 2: Add agents count to export summary**

In `printExportSummary`, add after the Skills line:
```go
fmt.Fprintf(w, "Agents:      %s\n", cBold(len(bundle.Agents)))
```

**Step 3: Add agents to inspect output**

In `printInspection`, add after the skills block:
```go
if len(bundle.Agents) > 0 {
    fmt.Fprintf(w, "\n%s\n", cYellow(fmt.Sprintf("Agents (%d):", len(bundle.Agents))))
    for _, agent := range bundle.Agents {
        fmt.Fprintf(w, "  - %s\n", agent.Name)
    }
}
```

**Step 4: Add agents to import result output**

In `printImportResult`, add after the skills written block:
```go
if len(result.AgentsWritten) > 0 {
    fmt.Fprintf(w, "Agents written: %s\n", cBold(len(result.AgentsWritten)))
    for _, a := range result.AgentsWritten {
        fmt.Fprintf(w, "  %s\n", cGreen(a))
    }
}
```

**Step 5: Update the `--only`/`--skip` help text in import command**

Change:
```go
cmd.Flags().StringSliceVar(&only, "only", nil, "Only import these components (settings,hooks,permissions,plugins,marketplaces,mcp,skills)")
```
To:
```go
cmd.Flags().StringSliceVar(&only, "only", nil, "Only import these components (settings,hooks,permissions,plugins,marketplaces,mcp,skills,agents)")
```

**Step 6: Build and run all tests**

```bash
go build ./... && go test ./...
```
Expected: PASS, binary builds cleanly

**Step 7: Commit**

```bash
git add cmd/claude-config/main.go
git commit -m "feat: add --no-agents flag and show agents in export/import/inspect output"
```

---

### Task 6: Manual smoke test

**Step 1: Build the binary**

```bash
go build -o bin/claude-config ./cmd/claude-config
```

**Step 2: Export and inspect**

```bash
./bin/claude-config export | ./bin/claude-config inspect
```
Expected: "Agents (N):" section shows your agents from `~/.claude/agents/`

**Step 3: Test --no-agents excludes agents**

```bash
./bin/claude-config export --no-agents | ./bin/claude-config inspect
```
Expected: no "Agents" section in output

**Step 4: Test dry-run import**

```bash
./bin/claude-config export | ./bin/claude-config import --dry-run --force
```
Expected: agent `.md` files listed under "Agents written (dry run)"

**Step 5: Commit smoke test confirmation (no code change needed)**

If all looks good, no commit needed. If you found issues, fix and commit with:
```bash
git commit -m "fix: <describe what you fixed>"
```
