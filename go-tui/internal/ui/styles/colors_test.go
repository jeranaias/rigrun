// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package styles provides the visual styling system for rigrun TUI.
package styles

import (
	"strings"
	"testing"
)

// =============================================================================
// COLOR DEFINITION TESTS
// =============================================================================

func TestPrimaryColors(t *testing.T) {
	// Test that primary colors are defined (non-empty)
	colors := []struct {
		name  string
		color interface{}
	}{
		{"Purple", Purple},
		{"PurpleDeep", PurpleDeep},
		{"Cyan", Cyan},
		{"CyanDeep", CyanDeep},
		{"Emerald", Emerald},
		{"EmeraldDeep", EmeraldDeep},
	}

	for _, c := range colors {
		// AdaptiveColor should have Light and Dark fields
		// Just verify they're non-zero values
		if c.color == nil {
			t.Errorf("%s color should be defined", c.name)
		}
	}
}

func TestSemanticColors(t *testing.T) {
	colors := []struct {
		name  string
		color interface{}
	}{
		{"Rose", Rose},
		{"RoseDeep", RoseDeep},
		{"Amber", Amber},
		{"AmberDeep", AmberDeep},
	}

	for _, c := range colors {
		if c.color == nil {
			t.Errorf("%s color should be defined", c.name)
		}
	}
}

func TestSurfaceColors(t *testing.T) {
	colors := []struct {
		name  string
		color interface{}
	}{
		{"Surface", Surface},
		{"SurfaceDim", SurfaceDim},
		{"SurfaceBright", SurfaceBright},
		{"Overlay", Overlay},
		{"OverlayDim", OverlayDim},
	}

	for _, c := range colors {
		if c.color == nil {
			t.Errorf("%s color should be defined", c.name)
		}
	}
}

func TestTextColors(t *testing.T) {
	colors := []struct {
		name  string
		color interface{}
	}{
		{"TextPrimary", TextPrimary},
		{"TextSecondary", TextSecondary},
		{"TextMuted", TextMuted},
		{"TextInverse", TextInverse},
	}

	for _, c := range colors {
		if c.color == nil {
			t.Errorf("%s color should be defined", c.name)
		}
	}
}

func TestMessageBubbleColors(t *testing.T) {
	colors := []struct {
		name  string
		color interface{}
	}{
		{"UserBubbleBg", UserBubbleBg},
		{"UserBubbleFg", UserBubbleFg},
		{"UserBubbleBorder", UserBubbleBorder},
		{"AssistantBubbleBg", AssistantBubbleBg},
		{"AssistantBubbleFg", AssistantBubbleFg},
		{"AssistantBubbleBorder", AssistantBubbleBorder},
		{"SystemBubbleBg", SystemBubbleBg},
		{"SystemBubbleFg", SystemBubbleFg},
		{"SystemBubbleBorder", SystemBubbleBorder},
		{"ToolSuccessBg", ToolSuccessBg},
		{"ToolSuccessFg", ToolSuccessFg},
		{"ToolErrorBg", ToolErrorBg},
		{"ToolErrorFg", ToolErrorFg},
	}

	for _, c := range colors {
		if c.color == nil {
			t.Errorf("%s color should be defined", c.name)
		}
	}
}

func TestSyntaxColors(t *testing.T) {
	colors := []struct {
		name  string
		color interface{}
	}{
		{"SyntaxKeyword", SyntaxKeyword},
		{"SyntaxString", SyntaxString},
		{"SyntaxNumber", SyntaxNumber},
		{"SyntaxComment", SyntaxComment},
		{"SyntaxFunction", SyntaxFunction},
		{"SyntaxType", SyntaxType},
		{"SyntaxOperator", SyntaxOperator},
		{"SyntaxVariable", SyntaxVariable},
		{"SyntaxConstant", SyntaxConstant},
		{"SyntaxBuiltin", SyntaxBuiltin},
	}

	for _, c := range colors {
		if c.color == nil {
			t.Errorf("%s color should be defined", c.name)
		}
	}
}

func TestSpecialEffectColors(t *testing.T) {
	colors := []struct {
		name  string
		color interface{}
	}{
		{"GradientStart", GradientStart},
		{"GradientEnd", GradientEnd},
		{"FocusRing", FocusRing},
		{"SelectionBg", SelectionBg},
	}

	for _, c := range colors {
		if c.color == nil {
			t.Errorf("%s color should be defined", c.name)
		}
	}
}

// =============================================================================
// ACCESSIBILITY COLOR TESTS
// =============================================================================

func TestAccessibilityColors(t *testing.T) {
	colors := []struct {
		name  string
		color interface{}
	}{
		{"SuccessHighContrast", SuccessHighContrast},
		{"ErrorHighContrast", ErrorHighContrast},
		{"WarningHighContrast", WarningHighContrast},
		{"InfoHighContrast", InfoHighContrast},
		{"DeuteranopiaSafeSuccess", DeuteranopiaSafeSuccess},
		{"DeuteranopiaSafeError", DeuteranopiaSafeError},
		{"LinkColor", LinkColor},
	}

	for _, c := range colors {
		if c.color == nil {
			t.Errorf("%s accessibility color should be defined", c.name)
		}
	}
}

// =============================================================================
// STATUS INDICATORS TESTS
// =============================================================================

func TestStatusIndicators(t *testing.T) {
	if StatusIndicators.Success == "" {
		t.Error("StatusIndicators.Success should be defined")
	}
	if StatusIndicators.Error == "" {
		t.Error("StatusIndicators.Error should be defined")
	}
	if StatusIndicators.Warning == "" {
		t.Error("StatusIndicators.Warning should be defined")
	}
	if StatusIndicators.Info == "" {
		t.Error("StatusIndicators.Info should be defined")
	}
	if StatusIndicators.Pending == "" {
		t.Error("StatusIndicators.Pending should be defined")
	}
	if StatusIndicators.Active == "" {
		t.Error("StatusIndicators.Active should be defined")
	}

	// Verify indicators are distinct
	indicators := []string{
		StatusIndicators.Success,
		StatusIndicators.Error,
		StatusIndicators.Warning,
		StatusIndicators.Info,
		StatusIndicators.Pending,
		StatusIndicators.Active,
	}

	seen := make(map[string]bool)
	for _, ind := range indicators {
		if seen[ind] {
			t.Errorf("Duplicate status indicator: %q", ind)
		}
		seen[ind] = true
	}
}

// =============================================================================
// ACCESSIBILITY TESTS
// =============================================================================

// Note: AccessibilityConfig was removed as accessibility features are always enabled.
// StatusIndicators provide shape-based indicators for all users by default.

// =============================================================================
// RENDER FUNCTION TESTS
// =============================================================================

func TestRenderSuccess(t *testing.T) {
	msg := "Operation completed"
	result := RenderSuccess(msg)

	if result == "" {
		t.Error("RenderSuccess() should return non-empty string")
	}

	if !strings.Contains(result, msg) {
		t.Errorf("RenderSuccess() = %q, should contain %q", result, msg)
	}

	// Should contain success indicator
	if !strings.Contains(result, StatusIndicators.Success) {
		t.Error("RenderSuccess() should contain success indicator")
	}
}

func TestRenderError(t *testing.T) {
	msg := "Operation failed"
	result := RenderError(msg)

	if result == "" {
		t.Error("RenderError() should return non-empty string")
	}

	if !strings.Contains(result, msg) {
		t.Errorf("RenderError() = %q, should contain %q", result, msg)
	}

	// Should contain error indicator
	if !strings.Contains(result, StatusIndicators.Error) {
		t.Error("RenderError() should contain error indicator")
	}
}

func TestRenderWarning(t *testing.T) {
	msg := "Potential issue detected"
	result := RenderWarning(msg)

	if result == "" {
		t.Error("RenderWarning() should return non-empty string")
	}

	if !strings.Contains(result, msg) {
		t.Errorf("RenderWarning() = %q, should contain %q", result, msg)
	}

	// Should contain warning indicator
	if !strings.Contains(result, StatusIndicators.Warning) {
		t.Error("RenderWarning() should contain warning indicator")
	}
}

func TestRenderInfo(t *testing.T) {
	msg := "Information available"
	result := RenderInfo(msg)

	if result == "" {
		t.Error("RenderInfo() should return non-empty string")
	}

	if !strings.Contains(result, msg) {
		t.Errorf("RenderInfo() = %q, should contain %q", result, msg)
	}

	// Should contain info indicator
	if !strings.Contains(result, StatusIndicators.Info) {
		t.Error("RenderInfo() should contain info indicator")
	}
}

func TestRenderStatus(t *testing.T) {
	msg := "Status message"

	// Test success case
	result := RenderStatus(true, msg)
	if !strings.Contains(result, StatusIndicators.Success) {
		t.Error("RenderStatus(true, msg) should use success indicator")
	}

	// Test error case
	result = RenderStatus(false, msg)
	if !strings.Contains(result, StatusIndicators.Error) {
		t.Error("RenderStatus(false, msg) should use error indicator")
	}
}

func TestRenderLink(t *testing.T) {
	text := "Click here"
	result := RenderLink(text)

	if result == "" {
		t.Error("RenderLink() should return non-empty string")
	}

	if !strings.Contains(result, text) {
		t.Errorf("RenderLink() = %q, should contain %q", result, text)
	}

	// Note: Can't easily test for underline in plain text output
	// but we can verify the function runs without error
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

func TestRenderFunctionsEmptyString(t *testing.T) {
	msg := ""

	// Status render functions (with indicators) should handle empty strings
	statusFuncs := []struct {
		name   string
		result string
	}{
		{"RenderSuccess", RenderSuccess(msg)},
		{"RenderError", RenderError(msg)},
		{"RenderWarning", RenderWarning(msg)},
		{"RenderInfo", RenderInfo(msg)},
	}

	for _, f := range statusFuncs {
		// Should still contain the indicator even with empty message
		if f.result == "" {
			t.Errorf("%s(\"\") should return non-empty (at least the indicator)", f.name)
		}
	}

	// RenderLink doesn't add an indicator, so empty input may produce empty output
	// Just verify it doesn't panic
	_ = RenderLink(msg)
}

func TestRenderFunctionsLongString(t *testing.T) {
	msg := strings.Repeat("Very long message ", 100)

	// All render functions should handle long strings
	funcs := []struct {
		name   string
		result string
	}{
		{"RenderSuccess", RenderSuccess(msg)},
		{"RenderError", RenderError(msg)},
		{"RenderWarning", RenderWarning(msg)},
		{"RenderInfo", RenderInfo(msg)},
		{"RenderLink", RenderLink(msg)},
	}

	for _, f := range funcs {
		if !strings.Contains(f.result, msg) {
			t.Errorf("%s() should handle long messages", f.name)
		}
	}
}

func TestRenderFunctionsSpecialCharacters(t *testing.T) {
	messages := []string{
		"Message with émojis 🎉",
		"Message with Unicode: 你好",
		"Message with symbols: @#$%^&*()",
	}

	for _, msg := range messages {
		// All render functions should handle special characters without panicking
		// and produce non-empty output
		if result := RenderSuccess(msg); len(result) == 0 {
			t.Errorf("RenderSuccess() should produce output for %q", msg)
		}
		if result := RenderError(msg); len(result) == 0 {
			t.Errorf("RenderError() should produce output for %q", msg)
		}
		if result := RenderWarning(msg); len(result) == 0 {
			t.Errorf("RenderWarning() should produce output for %q", msg)
		}
		if result := RenderInfo(msg); len(result) == 0 {
			t.Errorf("RenderInfo() should produce output for %q", msg)
		}
		if result := RenderLink(msg); len(result) == 0 {
			t.Errorf("RenderLink() should produce output for %q", msg)
		}
	}
}

// =============================================================================
// SECURITY TESTS
// =============================================================================

func TestRenderFunctionsNoScriptInjection(t *testing.T) {
	// Test that render functions don't interpret control sequences
	malicious := "\x1b[31m<script>alert('xss')</script>"

	result := RenderSuccess(malicious)
	// Should contain the message as-is (lipgloss handles escaping)
	if !strings.Contains(result, "script") {
		t.Error("RenderSuccess() should include script tag as text, not execute it")
	}
}

func TestStatusIndicatorsUniqueness(t *testing.T) {
	// Verify all indicators are unique for better accessibility
	indicators := map[string]string{
		"Success": StatusIndicators.Success,
		"Error":   StatusIndicators.Error,
		"Warning": StatusIndicators.Warning,
		"Info":    StatusIndicators.Info,
		"Pending": StatusIndicators.Pending,
		"Active":  StatusIndicators.Active,
	}

	seen := make(map[string]string)
	for name, indicator := range indicators {
		if existingName, exists := seen[indicator]; exists {
			t.Errorf("Duplicate indicator %q used for both %s and %s", indicator, name, existingName)
		}
		seen[indicator] = name
	}
}
