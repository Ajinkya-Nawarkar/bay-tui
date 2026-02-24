package tests

import (
	"os"
	"path/filepath"
	"testing"

	"bay/internal/db"
	"bay/internal/rules"
)

func TestRulesAddAndList(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	if err := rules.AddDB(d, "go-standards", "/home/user/.claude/docs/go-standards.md", "global"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := rules.AddDB(d, "bay-conv", "/home/user/.claude/docs/bay/DESIGN.md", "repo:bay"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	list, err := rules.ListDB(d)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(list))
	}

	// Should be sorted by name
	if list[0].Name != "bay-conv" {
		t.Errorf("expected first rule 'bay-conv', got '%s'", list[0].Name)
	}
	if list[1].Name != "go-standards" {
		t.Errorf("expected second rule 'go-standards', got '%s'", list[1].Name)
	}
}

func TestRulesRemove(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	rules.AddDB(d, "test-rule", "/tmp/test.md", "global")

	if err := rules.RemoveDB(d, "test-rule"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	list, _ := rules.ListDB(d)
	if len(list) != 0 {
		t.Errorf("expected 0 rules after remove, got %d", len(list))
	}
}

func TestRulesToggle(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	rules.AddDB(d, "test-rule", "/tmp/test.md", "global")

	// Initially enabled
	list, _ := rules.ListDB(d)
	if !list[0].Enabled {
		t.Error("expected rule to be enabled initially")
	}

	// Toggle off
	if err := rules.ToggleDB(d, "test-rule"); err != nil {
		t.Fatalf("Toggle failed: %v", err)
	}

	list, _ = rules.ListDB(d)
	if list[0].Enabled {
		t.Error("expected rule to be disabled after toggle")
	}

	// Toggle back on
	rules.ToggleDB(d, "test-rule")
	list, _ = rules.ListDB(d)
	if !list[0].Enabled {
		t.Error("expected rule to be enabled after second toggle")
	}
}

func TestActiveRules(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	rules.AddDB(d, "global-rule", "/tmp/global.md", "global")
	rules.AddDB(d, "bay-rule", "/tmp/bay.md", "repo:bay")
	rules.AddDB(d, "other-rule", "/tmp/other.md", "repo:other-project")
	rules.AddDB(d, "disabled-global", "/tmp/disabled.md", "global")
	rules.ToggleDB(d, "disabled-global")

	active, err := rules.ActiveRulesDB(d, "bay")
	if err != nil {
		t.Fatalf("ActiveRules failed: %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("expected 2 active rules for 'bay', got %d", len(active))
	}

	// Should include global + bay-scoped, but not other-project or disabled
	names := map[string]bool{}
	for _, r := range active {
		names[r.Name] = true
	}
	if !names["global-rule"] {
		t.Error("expected global-rule in active rules")
	}
	if !names["bay-rule"] {
		t.Error("expected bay-rule in active rules")
	}
}

func TestRulesReadContent(t *testing.T) {
	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "test.md")
	content := "# Test Rule\nThis is a test rule."
	os.WriteFile(mdPath, []byte(content), 0644)

	r := rules.Rule{Name: "test", Path: mdPath}
	got, err := rules.ReadContent(r)
	if err != nil {
		t.Fatalf("ReadContent failed: %v", err)
	}
	if got != content {
		t.Errorf("expected content '%s', got '%s'", content, got)
	}
}

func TestRulesReadContentMissing(t *testing.T) {
	r := rules.Rule{Name: "missing", Path: "/nonexistent/file.md"}
	_, err := rules.ReadContent(r)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestRulesUpsert(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	rules.AddDB(d, "test", "/path/v1.md", "global")
	rules.AddDB(d, "test", "/path/v2.md", "repo:bay") // upsert

	list, _ := rules.ListDB(d)
	if len(list) != 1 {
		t.Fatalf("expected 1 rule after upsert, got %d", len(list))
	}
	if list[0].Path != "/path/v2.md" {
		t.Errorf("expected updated path, got '%s'", list[0].Path)
	}
	if list[0].Scope != "repo:bay" {
		t.Errorf("expected updated scope, got '%s'", list[0].Scope)
	}
}
