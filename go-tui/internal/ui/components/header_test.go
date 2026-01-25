// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// MODE TESTS
// =============================================================================

func TestModeString(t *testing.T) {
	tests := []struct {
		mode Mode
		want string
	}{
		{ModeLocal, "LOCAL"},
		{ModeCloud, "CLOUD"},
		{ModeAuto, "AUTO"},
		{Mode(99), "UNKNOWN"}, // Invalid mode
	}

	for _, tc := range tests {
		got := tc.mode.String()
		if got != tc.want {
			t.Errorf("Mode(%d).String() = %q, want %q", tc.mode, got, tc.want)
		}
	}
}

// =============================================================================
// HEADER TESTS
// =============================================================================

func TestNewHeader(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)

	if h == nil {
		t.Fatal("NewHeader() returned nil")
	}

	if h.Title != "rigrun" {
		t.Errorf("NewHeader() Title = %q, want %q", h.Title, "rigrun")
	}

	if h.ModelName != "" {
		t.Errorf("NewHeader() ModelName = %q, want empty string", h.ModelName)
	}

	if h.Mode != ModeLocal {
		t.Errorf("NewHeader() Mode = %v, want %v", h.Mode, ModeLocal)
	}

	if h.Width != 80 {
		t.Errorf("NewHeader() Width = %d, want 80", h.Width)
	}

	if h.OfflineMode {
		t.Error("NewHeader() OfflineMode should be false")
	}

	if h.theme != theme {
		t.Error("NewHeader() did not set theme")
	}
}

func TestHeaderSetWidth(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)

	widths := []int{40, 80, 120, 200}
	for _, width := range widths {
		h.SetWidth(width)
		if h.Width != width {
			t.Errorf("SetWidth(%d) Width = %d, want %d", width, h.Width, width)
		}
	}
}

func TestHeaderSetModel(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)

	model := "qwen2.5-coder:14b"
	h.SetModel(model)

	if h.ModelName != model {
		t.Errorf("SetModel(%q) ModelName = %q, want %q", model, h.ModelName, model)
	}
}

func TestHeaderSetMode(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)

	modes := []Mode{ModeLocal, ModeCloud, ModeAuto}
	for _, mode := range modes {
		h.SetMode(mode)
		if h.Mode != mode {
			t.Errorf("SetMode(%v) Mode = %v, want %v", mode, h.Mode, mode)
		}
	}
}

func TestHeaderSetOfflineMode(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)

	h.SetOfflineMode(true)
	if !h.OfflineMode {
		t.Error("SetOfflineMode(true) did not enable offline mode")
	}

	h.SetOfflineMode(false)
	if h.OfflineMode {
		t.Error("SetOfflineMode(false) did not disable offline mode")
	}
}

func TestHeaderView(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)

	view := h.View()
	if view == "" {
		t.Error("View() should return non-empty string")
	}

	// Should contain the title
	if !strings.Contains(view, "rigrun") {
		t.Error("View() should contain title 'rigrun'")
	}
}

func TestHeaderViewWithModel(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)
	h.SetModel("test-model")

	view := h.View()
	if !strings.Contains(view, "test-model") {
		t.Error("View() should contain model name")
	}
}

func TestHeaderViewWithMode(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)

	tests := []struct {
		mode Mode
		want string
	}{
		{ModeLocal, "LOCAL"},
		{ModeCloud, "CLOUD"},
		{ModeAuto, "AUTO"},
	}

	for _, tc := range tests {
		h.SetMode(tc.mode)
		view := h.View()
		if !strings.Contains(view, tc.want) {
			t.Errorf("View() with mode %v should contain %q", tc.mode, tc.want)
		}
	}
}

func TestHeaderViewWithOfflineMode(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)
	h.SetOfflineMode(true)

	view := h.View()
	if !strings.Contains(view, "OFFLINE") {
		t.Error("View() with offline mode should contain 'OFFLINE'")
	}
}

func TestHeaderViewMinimumWidth(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)
	h.SetWidth(10) // Very narrow

	view := h.View()
	if view == "" {
		t.Error("View() should handle minimum width gracefully")
	}

	// Should still contain title even at minimum width
	if !strings.Contains(view, "rigrun") {
		t.Error("View() should contain title even at minimum width")
	}
}

func TestHeaderViewCompact(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)
	h.SetModel("test-model")
	h.SetMode(ModeCloud)

	view := h.ViewCompact()
	if view == "" {
		t.Error("ViewCompact() should return non-empty string")
	}

	// Should contain key elements
	if !strings.Contains(view, "rigrun") {
		t.Error("ViewCompact() should contain title")
	}
	if !strings.Contains(view, "test-model") {
		t.Error("ViewCompact() should contain model")
	}
	if !strings.Contains(view, "CLOUD") {
		t.Error("ViewCompact() should contain mode")
	}
}

func TestHeaderViewCompactWithOffline(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)
	h.SetOfflineMode(true)

	view := h.ViewCompact()
	if !strings.Contains(view, "OFFLINE") {
		t.Error("ViewCompact() with offline mode should contain 'OFFLINE'")
	}
}

func TestHeaderViewFancy(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)
	h.SetWidth(80)
	h.SetModel("test-model")

	view := h.ViewFancy()
	if view == "" {
		t.Error("ViewFancy() should return non-empty string")
	}

	// Should contain decorative elements
	if !strings.Contains(view, "rigrun") {
		t.Error("ViewFancy() should contain title")
	}
}

func TestHeaderViewFancyNarrowFallback(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)
	h.SetWidth(50) // Too narrow for fancy view

	view := h.ViewFancy()
	// Should fall back to regular view
	if view == "" {
		t.Error("ViewFancy() should return non-empty string even when narrow")
	}
}

func TestHeaderViewFancyWithOffline(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)
	h.SetWidth(80)
	h.SetOfflineMode(true)

	view := h.ViewFancy()
	if !strings.Contains(view, "OFFLINE") {
		t.Error("ViewFancy() with offline mode should contain 'OFFLINE'")
	}
}

func TestHeaderGetModeStyle(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)

	// Test all modes return non-nil styles
	modes := []Mode{ModeLocal, ModeCloud, ModeAuto, Mode(99)}
	for _, mode := range modes {
		h.SetMode(mode)
		style := h.getModeStyle()
		// Just verify it doesn't panic and returns a style
		_ = style
	}
}

func TestHeaderCreateDecorativeLine(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)

	tests := []struct {
		name  string
		width int
	}{
		{"narrow", 10},
		{"medium", 50},
		{"wide", 100},
		{"very small", 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			line := h.createDecorativeLine(tc.width)
			if tc.width >= 10 && line == "" {
				t.Error("createDecorativeLine() should return non-empty for adequate width")
			}
		})
	}
}

// =============================================================================
// GRADIENT TITLE TESTS
// =============================================================================

func TestGradientTitle(t *testing.T) {
	// Use lipgloss.Color directly since GradientTitle expects Color, not AdaptiveColor
	start := lipgloss.Color("#7C3AED") // Purple
	end := lipgloss.Color("#22D3EE")   // Cyan

	tests := []struct {
		name string
		text string
	}{
		{"empty", ""},
		{"single char", "a"},
		{"short", "hi"},
		{"normal", "rigrun"},
		{"long", "This is a longer gradient title"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := GradientTitle(tc.text, start, end)
			if tc.text == "" && result != "" {
				t.Error("GradientTitle() should return empty for empty input")
			}
			if tc.text != "" && result == "" {
				t.Error("GradientTitle() should return non-empty for non-empty input")
			}
		})
	}
}

func TestInterpolateColor(t *testing.T) {
	// Use lipgloss.Color directly since interpolateColor expects Color, not AdaptiveColor
	start := lipgloss.Color("#7C3AED") // Purple
	end := lipgloss.Color("#22D3EE")   // Cyan

	// Test interpolation at different points
	tests := []struct {
		name string
		t    float64
	}{
		{"start", 0.0},
		{"quarter", 0.25},
		{"half", 0.5},
		{"three quarters", 0.75},
		{"end", 1.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			color := interpolateColor(start, end, tc.t)
			if color == "" {
				t.Error("interpolateColor() should return non-empty color")
			}
		})
	}
}

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		hex     string
		wantR   uint8
		wantG   uint8
		wantB   uint8
		wantErr bool
	}{
		{"000000", 0, 0, 0, false},
		{"FFFFFF", 255, 255, 255, false},
		{"FF0000", 255, 0, 0, false},
		{"00FF00", 0, 255, 0, false},
		{"0000FF", 0, 0, 255, false},
		{"7C3AED", 124, 58, 237, false},
		{"22D3EE", 34, 211, 238, false},
		{"", 255, 255, 255, true},       // Empty - defaults to white
		{"FFF", 255, 255, 255, true},    // Too short - defaults to white
		{"GGGGGG", 255, 255, 255, true}, // Invalid hex - defaults to white
	}

	for _, tc := range tests {
		r, g, b := parseHexColor(tc.hex)
		if !tc.wantErr {
			if r != tc.wantR || g != tc.wantG || b != tc.wantB {
				t.Errorf("parseHexColor(%q) = (%d, %d, %d), want (%d, %d, %d)",
					tc.hex, r, g, b, tc.wantR, tc.wantG, tc.wantB)
			}
		} else {
			// For error cases, just check we got white (default)
			if r != 255 || g != 255 || b != 255 {
				t.Errorf("parseHexColor(%q) should return white (255,255,255) for invalid input, got (%d,%d,%d)",
					tc.hex, r, g, b)
			}
		}
	}
}

func TestParseHexByte(t *testing.T) {
	tests := []struct {
		s    string
		want uint8
	}{
		{"00", 0},
		{"FF", 255},
		{"7C", 124},
		{"3A", 58},
		{"ED", 237},
		{"22", 34},
		{"D3", 211},
		{"EE", 238},
		{"", 255},    // Invalid - too short
		{"F", 255},   // Invalid - too short
		{"FFF", 255}, // Invalid - too long
		{"GG", 255},  // Invalid - not hex
	}

	for _, tc := range tests {
		got := parseHexByte(tc.s)
		if got != tc.want {
			t.Errorf("parseHexByte(%q) = %d, want %d", tc.s, got, tc.want)
		}
	}
}

func TestFormatHexColor(t *testing.T) {
	tests := []struct {
		r, g, b uint8
		want    string
	}{
		{0, 0, 0, "#000000"},
		{255, 255, 255, "#FFFFFF"},
		{255, 0, 0, "#FF0000"},
		{0, 255, 0, "#00FF00"},
		{0, 0, 255, "#0000FF"},
		{124, 58, 237, "#7C3AED"},
		{34, 211, 238, "#22D3EE"},
	}

	for _, tc := range tests {
		got := formatHexColor(tc.r, tc.g, tc.b)
		if got != tc.want {
			t.Errorf("formatHexColor(%d, %d, %d) = %q, want %q",
				tc.r, tc.g, tc.b, got, tc.want)
		}
	}
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

func TestHeaderEmptyTitle(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)
	h.Title = ""

	view := h.View()
	if view == "" {
		t.Error("View() should handle empty title gracefully")
	}
}

func TestHeaderVeryWideWidth(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)
	h.SetWidth(10000)

	view := h.View()
	if view == "" {
		t.Error("View() should handle very wide width")
	}
}

func TestHeaderAllFieldsSet(t *testing.T) {
	theme := styles.NewTheme()
	h := NewHeader(theme)
	h.Title = "Custom Title"
	h.SetModel("custom-model:latest")
	h.SetMode(ModeAuto)
	h.SetOfflineMode(true)
	h.SetWidth(100)

	view := h.View()
	if !strings.Contains(view, "Custom Title") {
		t.Error("View() should contain custom title")
	}
	if !strings.Contains(view, "custom-model") {
		t.Error("View() should contain model name")
	}
	if !strings.Contains(view, "AUTO") {
		t.Error("View() should contain mode")
	}
	if !strings.Contains(view, "OFFLINE") {
		t.Error("View() should contain offline indicator")
	}
}

func TestGradientTitleEdgeCases(t *testing.T) {
	// Use lipgloss.Color directly since GradientTitle expects Color, not AdaptiveColor
	start := lipgloss.Color("#7C3AED") // Purple
	end := lipgloss.Color("#22D3EE")   // Cyan

	// Test with special characters
	tests := []string{
		"Hello, World!",
		"123-456",
		"Special@#$%",
		"Unicode: 你好",
	}

	for _, text := range tests {
		result := GradientTitle(text, start, end)
		if result == "" {
			t.Errorf("GradientTitle(%q) should return non-empty result", text)
		}
	}
}
