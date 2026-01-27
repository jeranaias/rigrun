// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package components

import (
	"testing"
	"time"
)

func TestNewErrorToast(t *testing.T) {
	toast := NewErrorToast("Test error message")

	if toast.Message != "Test error message" {
		t.Errorf("Expected message 'Test error message', got '%s'", toast.Message)
	}
	if toast.Kind != ToastKindError {
		t.Errorf("Expected ToastKindError, got %d", toast.Kind)
	}
	if toast.Duration != ErrorToastDuration {
		t.Errorf("Expected duration %v, got %v", ErrorToastDuration, toast.Duration)
	}
	if !toast.Dismissible {
		t.Error("Expected toast to be dismissible")
	}
	if toast.ID == 0 {
		t.Error("Expected non-zero toast ID")
	}
}

func TestNewWarningToast(t *testing.T) {
	toast := NewWarningToast("Test warning")

	if toast.Kind != ToastKindWarning {
		t.Errorf("Expected ToastKindWarning, got %d", toast.Kind)
	}
	if toast.Duration != WarningToastDuration {
		t.Errorf("Expected duration %v, got %v", WarningToastDuration, toast.Duration)
	}
}

func TestToastIsExpired(t *testing.T) {
	// Create a toast with very short duration
	toast := NewStatusToast("Test")
	toast.Duration = 10 * time.Millisecond
	toast.CreatedAt = time.Now().Add(-20 * time.Millisecond)

	if !toast.IsExpired() {
		t.Error("Toast should be expired")
	}

	// Fresh toast should not be expired
	freshToast := NewStatusToast("Fresh")
	if freshToast.IsExpired() {
		t.Error("Fresh toast should not be expired")
	}
}

func TestToastManager(t *testing.T) {
	manager := NewToastManager()

	if manager.HasToasts() {
		t.Error("New manager should have no toasts")
	}

	// Add toasts
	id1 := manager.AddError("Error 1")
	id2 := manager.AddWarning("Warning 1")

	if !manager.HasToasts() {
		t.Error("Manager should have toasts after adding")
	}

	toasts := manager.GetToasts()
	if len(toasts) != 2 {
		t.Errorf("Expected 2 toasts, got %d", len(toasts))
	}

	// Remove a toast
	manager.RemoveToast(id1)
	toasts = manager.GetToasts()
	if len(toasts) != 1 {
		t.Errorf("Expected 1 toast after removal, got %d", len(toasts))
	}

	// Clear all toasts
	manager.Clear()
	if manager.HasToasts() {
		t.Error("Manager should have no toasts after clear")
	}

	// Silence unused warning
	_ = id2
}

func TestToastManagerMaxToasts(t *testing.T) {
	manager := NewToastManager()
	manager.maxToasts = 3

	// Add more than max toasts
	manager.AddStatus("Toast 1")
	manager.AddStatus("Toast 2")
	manager.AddStatus("Toast 3")
	manager.AddStatus("Toast 4")
	manager.AddStatus("Toast 5")

	toasts := manager.GetToasts()
	if len(toasts) != 3 {
		t.Errorf("Expected max 3 toasts, got %d", len(toasts))
	}

	// Newest toasts should be first
	if toasts[0].Message != "Toast 5" {
		t.Error("Newest toast should be first")
	}
}

func TestToastTickExpiry(t *testing.T) {
	manager := NewToastManager()

	// Add a toast that's already expired
	expiredToast := NewStatusToast("Expired")
	expiredToast.Duration = 10 * time.Millisecond
	expiredToast.CreatedAt = time.Now().Add(-100 * time.Millisecond)
	manager.AddToast(expiredToast)

	// Add a fresh toast
	manager.AddStatus("Fresh")

	// Tick should remove expired toast
	remaining := manager.TickToasts()
	if len(remaining) != 1 {
		t.Errorf("Expected 1 remaining toast after tick, got %d", len(remaining))
	}
	if remaining[0].Message != "Fresh" {
		t.Error("Fresh toast should remain")
	}
}

func TestFormatSeconds(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{-1, "0"},
		{1, "1"},
		{5, "5"},
		{10, "10"},
		{59, "59"},
		{100, "100"},
	}

	for _, tc := range tests {
		result := formatSeconds(tc.input)
		if result != tc.expected {
			t.Errorf("formatSeconds(%d) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}

func TestRenderToast(t *testing.T) {
	toast := NewErrorToast("Test error message")
	rendered := RenderToast(toast, 80)

	if rendered == "" {
		t.Error("Rendered toast should not be empty")
	}

	// Should contain the message
	if len(rendered) < len(toast.Message) {
		t.Error("Rendered toast should contain the message")
	}
}

func TestRenderToastStack(t *testing.T) {
	toasts := []ErrorToast{
		NewErrorToast("Error 1"),
		NewWarningToast("Warning 1"),
	}

	rendered := RenderToastStack(toasts, 100, 40)

	if rendered == "" {
		t.Error("Rendered toast stack should not be empty")
	}
}

func TestRenderToastStackEmpty(t *testing.T) {
	rendered := RenderToastStack([]ErrorToast{}, 100, 40)

	if rendered != "" {
		t.Error("Empty toast stack should render empty string")
	}
}
