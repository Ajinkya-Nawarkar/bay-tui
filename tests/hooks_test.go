package tests

import (
	"testing"
	"time"

	"bay/internal/hooks"
)

func TestDeactivateDebounce(t *testing.T) {
	hooks.ResetDebounce()

	// First call should allow capture
	if !hooks.ShouldCapture("test-session") {
		t.Error("first call should allow capture")
	}

	// Immediate second call should be debounced
	if hooks.ShouldCapture("test-session") {
		t.Error("rapid second call should be debounced")
	}

	// Different session should not be affected
	if !hooks.ShouldCapture("other-session") {
		t.Error("different session should not be debounced")
	}
}

func TestDeactivateDebounceExpiry(t *testing.T) {
	hooks.ResetDebounce()

	// Use a very short duration for testing
	dur := 50 * time.Millisecond

	if !hooks.ShouldCaptureWithDuration("test-session", dur) {
		t.Error("first call should allow capture")
	}

	if hooks.ShouldCaptureWithDuration("test-session", dur) {
		t.Error("immediate second call should be debounced")
	}

	// Wait for debounce to expire
	time.Sleep(60 * time.Millisecond)

	if !hooks.ShouldCaptureWithDuration("test-session", dur) {
		t.Error("call after debounce expiry should allow capture")
	}
}
