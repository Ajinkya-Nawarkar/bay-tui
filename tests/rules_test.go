package tests

import (
	"os"
	"path/filepath"
	"testing"

	bayctx "bay/internal/context"
	"bay/internal/db"
)

func TestContextFileAddAndList(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	if err := bayctx.AddDB(d, "go-standards", "/home/user/.claude/docs/go-standards.md", "global", "rules"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := bayctx.AddDB(d, "bay-conv", "/home/user/.claude/docs/bay/DESIGN.md", "repo:bay", "docs"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	list, err := bayctx.ListDB(d)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 context files, got %d", len(list))
	}

	// Should be sorted by name
	if list[0].Name != "bay-conv" {
		t.Errorf("expected first entry 'bay-conv', got '%s'", list[0].Name)
	}
	if list[0].Category != "docs" {
		t.Errorf("expected category 'docs', got '%s'", list[0].Category)
	}
	if list[1].Name != "go-standards" {
		t.Errorf("expected second entry 'go-standards', got '%s'", list[1].Name)
	}
	if list[1].Category != "rules" {
		t.Errorf("expected category 'rules', got '%s'", list[1].Category)
	}
}

func TestContextFileRemove(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	bayctx.AddDB(d, "test-rule", "/tmp/test.md", "global", "rules")

	if err := bayctx.RemoveDB(d, "test-rule"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	list, _ := bayctx.ListDB(d)
	if len(list) != 0 {
		t.Errorf("expected 0 entries after remove, got %d", len(list))
	}
}

func TestContextFileToggle(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	bayctx.AddDB(d, "test-rule", "/tmp/test.md", "global", "rules")

	// Initially enabled
	list, _ := bayctx.ListDB(d)
	if !list[0].Enabled {
		t.Error("expected entry to be enabled initially")
	}

	// Toggle off
	if err := bayctx.ToggleDB(d, "test-rule"); err != nil {
		t.Fatalf("Toggle failed: %v", err)
	}

	list, _ = bayctx.ListDB(d)
	if list[0].Enabled {
		t.Error("expected entry to be disabled after toggle")
	}

	// Toggle back on
	bayctx.ToggleDB(d, "test-rule")
	list, _ = bayctx.ListDB(d)
	if !list[0].Enabled {
		t.Error("expected entry to be enabled after second toggle")
	}
}

func TestActiveRules(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	bayctx.AddDB(d, "global-rule", "/tmp/global.md", "global", "rules")
	bayctx.AddDB(d, "bay-rule", "/tmp/bay.md", "repo:bay", "rules")
	bayctx.AddDB(d, "other-rule", "/tmp/other.md", "repo:other-project", "rules")
	bayctx.AddDB(d, "disabled-global", "/tmp/disabled.md", "global", "rules")
	bayctx.ToggleDB(d, "disabled-global")

	active, err := bayctx.ActiveRulesDB(d, "bay")
	if err != nil {
		t.Fatalf("ActiveRules failed: %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("expected 2 active entries for 'bay', got %d", len(active))
	}

	// Should include global + bay-scoped, but not other-project or disabled
	names := map[string]bool{}
	for _, f := range active {
		names[f.Name] = true
	}
	if !names["global-rule"] {
		t.Error("expected global-rule in active entries")
	}
	if !names["bay-rule"] {
		t.Error("expected bay-rule in active entries")
	}
}

func TestContextFileReadContent(t *testing.T) {
	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "test.md")
	content := "# Test Rule\nThis is a test rule."
	os.WriteFile(mdPath, []byte(content), 0644)

	f := bayctx.ContextFile{Name: "test", Path: mdPath}
	got, err := bayctx.ReadContent(f)
	if err != nil {
		t.Fatalf("ReadContent failed: %v", err)
	}
	if got != content {
		t.Errorf("expected content '%s', got '%s'", content, got)
	}
}

func TestContextFileReadContentMissing(t *testing.T) {
	f := bayctx.ContextFile{Name: "missing", Path: "/nonexistent/file.md"}
	_, err := bayctx.ReadContent(f)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestContextFileUpsert(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	bayctx.AddDB(d, "test", "/path/v1.md", "global", "rules")
	bayctx.AddDB(d, "test", "/path/v2.md", "repo:bay", "docs") // upsert

	list, _ := bayctx.ListDB(d)
	if len(list) != 1 {
		t.Fatalf("expected 1 entry after upsert, got %d", len(list))
	}
	if list[0].Path != "/path/v2.md" {
		t.Errorf("expected updated path, got '%s'", list[0].Path)
	}
	if list[0].Scope != "repo:bay" {
		t.Errorf("expected updated scope, got '%s'", list[0].Scope)
	}
	if list[0].Category != "docs" {
		t.Errorf("expected updated category 'docs', got '%s'", list[0].Category)
	}
}

func TestContextFileCategory(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	bayctx.AddDB(d, "coding-std", "/tmp/std.md", "global", "standards")
	bayctx.AddDB(d, "api-ref", "/tmp/api.md", "global", "docs")
	bayctx.AddDB(d, "lint-rules", "/tmp/lint.md", "global", "rules")

	list, _ := bayctx.ListDB(d)
	if len(list) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(list))
	}

	cats := map[string]string{}
	for _, f := range list {
		cats[f.Name] = f.Category
	}
	if cats["coding-std"] != "standards" {
		t.Errorf("expected 'standards', got '%s'", cats["coding-std"])
	}
	if cats["api-ref"] != "docs" {
		t.Errorf("expected 'docs', got '%s'", cats["api-ref"])
	}
	if cats["lint-rules"] != "rules" {
		t.Errorf("expected 'rules', got '%s'", cats["lint-rules"])
	}
}
