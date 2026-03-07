package tests

import (
	"testing"

	baytmux "bay/internal/tmux"
)

func TestConstants(t *testing.T) {
	if baytmux.MainSession != "bay" {
		t.Errorf("expected main session 'bay', got '%s'", baytmux.MainSession)
	}
	if baytmux.TopbarHeight != "5" {
		t.Errorf("expected topbar height '5', got '%s'", baytmux.TopbarHeight)
	}
}
