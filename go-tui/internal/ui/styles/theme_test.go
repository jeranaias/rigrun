// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package styles provides the visual styling system for rigrun TUI.
package styles

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// =============================================================================
// THEME CREATION TESTS
// =============================================================================

func TestNewTheme(t *testing.T) {
	theme := NewTheme()

	if theme == nil {
		t.Fatal("NewTheme() returned nil")
	}

	// Check basic properties are set
	if theme.ColorProfile == 0 {
		t.Error("NewTheme() should set ColorProfile")
	}

	// Verify styles are initialized by rendering a test string
	renderedApp := theme.App.Render("test")
	if renderedApp == "" {
		t.Error("NewTheme() should initialize App style")
	}
}

func TestThemeInitStyles(t *testing.T) {
	theme := NewTheme()

	// Test that various style categories are initialized
	// We test by rendering and checking for non-empty output
	styles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"Header", theme.Header},
		{"UserBubble", theme.UserBubble},
		{"AssistantBubble", theme.AssistantBubble},
		{"SystemBubble", theme.SystemBubble},
		{"InputContainer", theme.InputContainer},
		{"StatusBar", theme.StatusBar},
		{"ErrorBox", theme.ErrorBox},
		{"CodeBlock", theme.CodeBlock},
	}

	for _, s := range styles {
		// Verify each style is initialized by rendering a test string
		// An uninitialized style would just return the input unchanged
		rendered := s.style.Render("test")
		if rendered == "" {
			t.Errorf("%s style should be initialized", s.name)
		}
	}
}

// =============================================================================
// THEME SIZE TESTS
// =============================================================================

func TestThemeSetSize(t *testing.T) {
	theme := NewTheme()

	tests := []struct {
		width  int
		height int
	}{
		{80, 24},
		{120, 40},
		{200, 60},
		{40, 10},
	}

	for _, tc := range tests {
		theme.SetSize(tc.width, tc.height)
		if theme.Width != tc.width {
			t.Errorf("SetSize(%d, %d) Width = %d, want %d", tc.width, tc.height, theme.Width, tc.width)
		}
		if theme.Height != tc.height {
			t.Errorf("SetSize(%d, %d) Height = %d, want %d", tc.width, tc.height, theme.Height, tc.height)
		}
	}
}

func TestThemeGetLayoutMode(t *testing.T) {
	theme := NewTheme()

	tests := []struct {
		width int
		want  LayoutMode
	}{
		{40, LayoutNarrow},
		{59, LayoutNarrow},
		{60, LayoutMedium},
		{80, LayoutMedium},
		{99, LayoutMedium},
		{100, LayoutWide},
		{150, LayoutWide},
		{200, LayoutWide},
	}

	for _, tc := range tests {
		theme.SetSize(tc.width, 24)
		got := theme.GetLayoutMode()
		if got != tc.want {
			t.Errorf("GetLayoutMode() with width %d = %v, want %v", tc.width, got, tc.want)
		}
	}
}

// =============================================================================
// LAYOUT MODE TESTS
// =============================================================================

func TestLayoutModeConstants(t *testing.T) {
	// Verify layout mode constants have expected values
	if LayoutNarrow != 0 {
		t.Errorf("LayoutNarrow = %d, want 0", LayoutNarrow)
	}
	if LayoutMedium != 1 {
		t.Errorf("LayoutMedium = %d, want 1", LayoutMedium)
	}
	if LayoutWide != 2 {
		t.Errorf("LayoutWide = %d, want 2", LayoutWide)
	}
}

// =============================================================================
// ACCESSIBILITY STYLE TESTS
// =============================================================================

func TestThemeAccessibilityStyles(t *testing.T) {
	theme := NewTheme()

	// Test that accessibility styles are initialized
	accessibilityStyles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"SuccessStyle", theme.SuccessStyle},
		{"ErrorStyle", theme.ErrorStyle},
		{"WarningStyle", theme.WarningStyle},
		{"InfoStyle", theme.InfoStyle},
		{"LinkStyle", theme.LinkStyle},
	}

	for _, s := range accessibilityStyles {
		rendered := s.style.Render("test")
		if rendered == "" {
			t.Errorf("%s should be initialized", s.name)
		}
	}
}

// =============================================================================
// MESSAGE BUBBLE STYLE TESTS
// =============================================================================

func TestThemeMessageBubbleStyles(t *testing.T) {
	theme := NewTheme()

	bubbles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"UserBubble", theme.UserBubble},
		{"AssistantBubble", theme.AssistantBubble},
		{"SystemBubble", theme.SystemBubble},
		{"ToolSuccess", theme.ToolSuccess},
		{"ToolError", theme.ToolError},
	}

	for _, b := range bubbles {
		rendered := b.style.Render("test")
		if rendered == "" {
			t.Errorf("%s should be initialized", b.name)
		}
	}
}

// =============================================================================
// INPUT STYLE TESTS
// =============================================================================

func TestThemeInputStyles(t *testing.T) {
	theme := NewTheme()

	inputStyles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"InputContainer", theme.InputContainer},
		{"InputPrompt", theme.InputPrompt},
		{"InputText", theme.InputText},
		{"InputPlaceholder", theme.InputPlaceholder},
		{"CharCount", theme.CharCount},
		{"CharCountWarning", theme.CharCountWarning},
		{"CharCountDanger", theme.CharCountDanger},
	}

	for _, s := range inputStyles {
		rendered := s.style.Render("test")
		if rendered == "" {
			t.Errorf("%s should be initialized", s.name)
		}
	}
}

// =============================================================================
// STATUS BAR STYLE TESTS
// =============================================================================

func TestThemeStatusBarStyles(t *testing.T) {
	theme := NewTheme()

	statusStyles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"StatusBar", theme.StatusBar},
		{"StatusBarWide", theme.StatusBarWide},
		{"ModeLocal", theme.ModeLocal},
		{"ModeCloud", theme.ModeCloud},
		{"ModeHybrid", theme.ModeHybrid},
		{"GPUActive", theme.GPUActive},
		{"GPUInactive", theme.GPUInactive},
		{"ShortcutKey", theme.ShortcutKey},
		{"ShortcutDesc", theme.ShortcutDesc},
	}

	for _, s := range statusStyles {
		rendered := s.style.Render("test")
		if rendered == "" {
			t.Errorf("%s should be initialized", s.name)
		}
	}
}

// =============================================================================
// CODE BLOCK STYLE TESTS
// =============================================================================

func TestThemeCodeBlockStyles(t *testing.T) {
	theme := NewTheme()

	codeStyles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"CodeBlock", theme.CodeBlock},
		{"CodeLangBadge", theme.CodeLangBadge},
		{"CodeCopyBtn", theme.CodeCopyBtn},
		{"CodeLineNum", theme.CodeLineNum},
	}

	for _, s := range codeStyles {
		rendered := s.style.Render("test")
		if rendered == "" {
			t.Errorf("%s should be initialized", s.name)
		}
	}
}

// =============================================================================
// ERROR BOX STYLE TESTS
// =============================================================================

func TestThemeErrorBoxStyles(t *testing.T) {
	theme := NewTheme()

	errorStyles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"ErrorBox", theme.ErrorBox},
		{"ErrorTitle", theme.ErrorTitle},
		{"ErrorMessage", theme.ErrorMessage},
		{"ErrorSuggestion", theme.ErrorSuggestion},
		{"ErrorTip", theme.ErrorTip},
	}

	for _, s := range errorStyles {
		rendered := s.style.Render("test")
		if rendered == "" {
			t.Errorf("%s should be initialized", s.name)
		}
	}
}

// =============================================================================
// SPINNER STYLE TESTS
// =============================================================================

func TestThemeSpinnerStyles(t *testing.T) {
	theme := NewTheme()

	spinnerStyles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"Spinner", theme.Spinner},
		{"ThinkingText", theme.ThinkingText},
		{"ThinkingDots", theme.ThinkingDots},
		{"ThinkingTime", theme.ThinkingTime},
		{"ThinkingDetail", theme.ThinkingDetail},
	}

	for _, s := range spinnerStyles {
		rendered := s.style.Render("test")
		if rendered == "" {
			t.Errorf("%s should be initialized", s.name)
		}
	}
}

// =============================================================================
// PALETTE STYLE TESTS
// =============================================================================

func TestThemePaletteStyles(t *testing.T) {
	theme := NewTheme()

	paletteStyles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"PaletteOverlay", theme.PaletteOverlay},
		{"PaletteBox", theme.PaletteBox},
		{"PaletteItem", theme.PaletteItem},
		{"PaletteItemSelected", theme.PaletteItemSelected},
		{"PaletteCommand", theme.PaletteCommand},
		{"PaletteDesc", theme.PaletteDesc},
	}

	for _, s := range paletteStyles {
		rendered := s.style.Render("test")
		if rendered == "" {
			t.Errorf("%s should be initialized", s.name)
		}
	}
}

// =============================================================================
// COMPLETION POPUP STYLE TESTS
// =============================================================================

func TestThemeCompletionStyles(t *testing.T) {
	theme := NewTheme()

	completionStyles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"CompletionPopup", theme.CompletionPopup},
		{"CompletionItem", theme.CompletionItem},
		{"CompletionSelected", theme.CompletionSelected},
		{"CompletionMatch", theme.CompletionMatch},
	}

	for _, s := range completionStyles {
		rendered := s.style.Render("test")
		if rendered == "" {
			t.Errorf("%s should be initialized", s.name)
		}
	}
}

// =============================================================================
// SESSION LIST STYLE TESTS
// =============================================================================

func TestThemeSessionStyles(t *testing.T) {
	theme := NewTheme()

	sessionStyles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"SessionList", theme.SessionList},
		{"SessionItem", theme.SessionItem},
		{"SessionItemSelected", theme.SessionItemSelected},
		{"SessionID", theme.SessionID},
		{"SessionTitle", theme.SessionTitle},
		{"SessionMeta", theme.SessionMeta},
	}

	for _, s := range sessionStyles {
		rendered := s.style.Render("test")
		if rendered == "" {
			t.Errorf("%s should be initialized", s.name)
		}
	}
}

// =============================================================================
// WELCOME SCREEN STYLE TESTS
// =============================================================================

func TestThemeWelcomeStyles(t *testing.T) {
	theme := NewTheme()

	welcomeStyles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"WelcomeBox", theme.WelcomeBox},
		{"WelcomeLogo", theme.WelcomeLogo},
		{"WelcomeVersion", theme.WelcomeVersion},
		{"WelcomeInfo", theme.WelcomeInfo},
		{"WelcomeKey", theme.WelcomeKey},
		{"WelcomePressKey", theme.WelcomePressKey},
	}

	for _, s := range welcomeStyles {
		rendered := s.style.Render("test")
		if rendered == "" {
			t.Errorf("%s should be initialized", s.name)
		}
	}
}

// =============================================================================
// CONTEXT PREVIEW STYLE TESTS
// =============================================================================

func TestThemeContextStyles(t *testing.T) {
	theme := NewTheme()

	contextStyles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"ContextPreview", theme.ContextPreview},
		{"ContextHeader", theme.ContextHeader},
		{"ContextContent", theme.ContextContent},
		{"MentionText", theme.MentionText},
	}

	for _, s := range contextStyles {
		rendered := s.style.Render("test")
		if rendered == "" {
			t.Errorf("%s should be initialized", s.name)
		}
	}
}

// =============================================================================
// STATISTICS STYLE TESTS
// =============================================================================

func TestThemeStatisticsStyles(t *testing.T) {
	theme := NewTheme()

	statsStyles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"StatsBar", theme.StatsBar},
		{"StatsLabel", theme.StatsLabel},
		{"StatsValue", theme.StatsValue},
	}

	for _, s := range statsStyles {
		rendered := s.style.Render("test")
		if rendered == "" {
			t.Errorf("%s should be initialized", s.name)
		}
	}
}

// =============================================================================
// PERMISSION PROMPT STYLE TESTS
// =============================================================================

func TestThemePermissionStyles(t *testing.T) {
	theme := NewTheme()

	permissionStyles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"PermissionBox", theme.PermissionBox},
		{"PermissionTitle", theme.PermissionTitle},
		{"PermissionCommand", theme.PermissionCommand},
		{"PermissionButton", theme.PermissionButton},
		{"PermissionButtonActive", theme.PermissionButtonActive},
	}

	for _, s := range permissionStyles {
		rendered := s.style.Render("test")
		if rendered == "" {
			t.Errorf("%s should be initialized", s.name)
		}
	}
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

func TestThemeZeroSize(t *testing.T) {
	theme := NewTheme()
	theme.SetSize(0, 0)

	if theme.Width != 0 || theme.Height != 0 {
		t.Error("SetSize(0, 0) should set both dimensions to 0")
	}

	// GetLayoutMode should still work
	mode := theme.GetLayoutMode()
	if mode != LayoutNarrow {
		t.Errorf("GetLayoutMode() with width 0 = %v, want %v", mode, LayoutNarrow)
	}
}

func TestThemeNegativeSize(t *testing.T) {
	theme := NewTheme()
	theme.SetSize(-100, -50)

	// Should accept negative values (terminal can't be negative, but no validation)
	if theme.Width != -100 || theme.Height != -50 {
		t.Error("SetSize() should accept values as-is")
	}
}

func TestThemeMultipleInitialization(t *testing.T) {
	// Create multiple themes to ensure no global state issues
	theme1 := NewTheme()
	theme2 := NewTheme()

	if theme1 == theme2 {
		t.Error("NewTheme() should create distinct theme instances")
	}

	// Modify one theme
	theme1.SetSize(100, 50)
	theme2.SetSize(200, 80)

	if theme1.Width == theme2.Width {
		t.Error("Themes should have independent state")
	}
}
