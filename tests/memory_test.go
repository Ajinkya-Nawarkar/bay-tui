package tests

import (
	"strings"
	"testing"

	"bay/internal/db"
	"bay/internal/memory"
)

func TestContextRenderingWithChecklist(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	memory.CreateTaskDB(d, "s1", "Reproduce the bug")
	id2, _ := memory.CreateTaskDB(d, "s1", "Fix redirect URL")
	memory.SetTaskStatusDB(d, id2, "done")
	memory.CreateTaskDB(d, "s1", "Add tests")

	ctx, err := memory.RenderContextDB(d, "s1", "myrepo", "fix/auth", "Fix login redirect bug")
	if err != nil {
		t.Fatalf("RenderContextDB failed: %v", err)
	}

	if !strings.Contains(ctx, "# Session: s1") {
		t.Error("expected session name in header")
	}
	if !strings.Contains(ctx, "Repo: myrepo") {
		t.Error("expected repo in header")
	}
	if !strings.Contains(ctx, "Branch: fix/auth") {
		t.Error("expected branch in header")
	}
	if !strings.Contains(ctx, "## Purpose") {
		t.Error("expected Purpose section")
	}
	if !strings.Contains(ctx, "Fix login redirect bug") {
		t.Error("expected purpose text")
	}
	if !strings.Contains(ctx, "## Checklist") {
		t.Error("expected Checklist section")
	}
	if !strings.Contains(ctx, "- [ ] Reproduce the bug") {
		t.Error("expected todo item")
	}
	if !strings.Contains(ctx, "- [x] Fix redirect URL") {
		t.Error("expected done item")
	}
}

func TestContextRenderingNoPurpose(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	ctx, err := memory.RenderContextDB(d, "s1", "myrepo", "", "")
	if err != nil {
		t.Fatalf("RenderContextDB failed: %v", err)
	}

	if !strings.Contains(ctx, "# Session: s1") {
		t.Error("expected header")
	}
	if strings.Contains(ctx, "## Purpose") {
		t.Error("purpose section should be absent when empty")
	}
	if strings.Contains(ctx, "## Checklist") {
		t.Error("checklist section should be absent when no items")
	}
}

func TestContextRenderingNoChecklist(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	ctx, err := memory.RenderContextDB(d, "s1", "myrepo", "main", "Working on feature X")
	if err != nil {
		t.Fatalf("RenderContextDB failed: %v", err)
	}

	if !strings.Contains(ctx, "## Purpose") {
		t.Error("expected Purpose section")
	}
	if strings.Contains(ctx, "## Checklist") {
		t.Error("checklist section should be absent when no items")
	}
}
