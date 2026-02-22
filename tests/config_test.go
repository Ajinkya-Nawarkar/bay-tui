package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anawarkar/bay/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
	if cfg.Defaults.Shell != "zsh" {
		t.Errorf("expected shell zsh, got %s", cfg.Defaults.Shell)
	}
	if cfg.Defaults.Agent != "claude" {
		t.Errorf("expected agent claude, got %s", cfg.Defaults.Agent)
	}
	if cfg.Defaults.WorktreeLocation != "managed" {
		t.Errorf("expected worktree_location managed, got %s", cfg.Defaults.WorktreeLocation)
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Use a temp dir as home
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfg := config.DefaultConfig()
	cfg.ScanDirs = []string{"/tmp/test-workspace"}

	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	cfgPath := filepath.Join(tmpDir, ".bay", "config.yaml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Fatal("config.yaml not created")
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Version != 1 {
		t.Errorf("expected version 1, got %d", loaded.Version)
	}
	if len(loaded.ScanDirs) != 1 || loaded.ScanDirs[0] != "/tmp/test-workspace" {
		t.Errorf("unexpected scan_dirs: %v", loaded.ScanDirs)
	}
}

func TestEnsureDirs(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	if err := config.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs failed: %v", err)
	}

	expectedDirs := []string{
		".bay",
		".bay/sessions",
		".bay/worktrees",
		".bay/agents",
		".bay/logs",
		".bay/plugins",
	}

	for _, d := range expectedDirs {
		full := filepath.Join(tmpDir, d)
		info, err := os.Stat(full)
		if os.IsNotExist(err) {
			t.Errorf("directory not created: %s", d)
		} else if !info.IsDir() {
			t.Errorf("not a directory: %s", d)
		}
	}
}
