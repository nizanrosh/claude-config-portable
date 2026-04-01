package cursor

import (
	"testing"
)

func TestValidateComponents_ValidNames(t *testing.T) {
	t.Parallel()
	err := ValidateComponents([]string{"settings", "rules", "mcp"}, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateComponents_InvalidOnly(t *testing.T) {
	t.Parallel()
	err := ValidateComponents([]string{"setitngs"}, nil)
	if err == nil {
		t.Fatal("expected error for typo in --only")
	}
}

func TestValidateComponents_InvalidSkip(t *testing.T) {
	t.Parallel()
	err := ValidateComponents(nil, []string{"extnsions"})
	if err == nil {
		t.Fatal("expected error for typo in --skip")
	}
}

func TestValidateComponents_AllValid(t *testing.T) {
	t.Parallel()
	var all []string
	for _, c := range AllCursorComponents() {
		all = append(all, string(c))
	}
	err := ValidateComponents(all, nil)
	if err != nil {
		t.Fatalf("all valid components should pass: %v", err)
	}
}

func TestShouldImport_OnlyFilter(t *testing.T) {
	t.Parallel()
	opts := WriteOptions{Only: []string{"settings", "mcp"}}

	if !shouldImport(CompSettings, opts) {
		t.Error("settings should be included")
	}
	if !shouldImport(CompMCP, opts) {
		t.Error("mcp should be included")
	}
	if shouldImport(CompRules, opts) {
		t.Error("rules should be excluded")
	}
}

func TestShouldImport_SkipFilter(t *testing.T) {
	t.Parallel()
	opts := WriteOptions{Skip: []string{"extensions", "skills"}}

	if !shouldImport(CompSettings, opts) {
		t.Error("settings should be included")
	}
	if shouldImport(CompExtensions, opts) {
		t.Error("extensions should be excluded")
	}
	if shouldImport(CompSkills, opts) {
		t.Error("skills should be excluded")
	}
}

func TestShouldImport_NoFilters(t *testing.T) {
	t.Parallel()
	opts := WriteOptions{}

	for _, c := range AllCursorComponents() {
		if !shouldImport(c, opts) {
			t.Errorf("component %q should be included with no filters", c)
		}
	}
}
