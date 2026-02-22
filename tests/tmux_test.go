package tests

import (
	"testing"

	baytmux "github.com/anawarkar/bay/internal/tmux"
)

func TestConstants(t *testing.T) {
	if baytmux.MainSession != "bay" {
		t.Errorf("expected main session 'bay', got '%s'", baytmux.MainSession)
	}
	if baytmux.SidebarWidth != "35" {
		t.Errorf("expected sidebar width '35', got '%s'", baytmux.SidebarWidth)
	}
}
