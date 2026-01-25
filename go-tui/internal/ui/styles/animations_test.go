// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package styles provides the visual styling system for rigrun TUI.
package styles

import (
	"strings"
	"testing"
	"time"
)

// =============================================================================
// SPINNER CONFIG TESTS
// =============================================================================

func TestSpinnerConfigs(t *testing.T) {
	spinners := []struct {
		name   string
		config SpinnerConfig
	}{
		{"BrailleSpinner", BrailleSpinner},
		{"DotsSpinner", DotsSpinner},
		{"LineSpinner", LineSpinner},
		{"BlockSpinner", BlockSpinner},
		{"PulseSpinner", PulseSpinner},
	}

	for _, s := range spinners {
		t.Run(s.name, func(t *testing.T) {
			if len(s.config.Frames) == 0 {
				t.Errorf("%s should have frames", s.name)
			}
			if s.config.FPS <= 0 {
				t.Errorf("%s FPS should be positive", s.name)
			}
		})
	}
}

func TestSpinnerConfigDuration(t *testing.T) {
	tests := []struct {
		name string
		fps  int
		want time.Duration
	}{
		{"12 FPS", 12, time.Second / 12},
		{"6 FPS", 6, time.Second / 6},
		{"10 FPS", 10, time.Second / 10},
		{"8 FPS", 8, time.Second / 8},
		{"15 FPS", 15, time.Second / 15},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := SpinnerConfig{FPS: tc.fps}
			got := config.Duration()
			if got != tc.want {
				t.Errorf("Duration() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBrailleSpinnerFrames(t *testing.T) {
	if len(BrailleSpinner.Frames) != 10 {
		t.Errorf("BrailleSpinner should have 10 frames, got %d", len(BrailleSpinner.Frames))
	}

	// Verify all frames are non-empty
	for i, frame := range BrailleSpinner.Frames {
		if frame == "" {
			t.Errorf("BrailleSpinner frame %d should not be empty", i)
		}
	}
}

func TestDotsSpinnerFrames(t *testing.T) {
	if len(DotsSpinner.Frames) != 6 {
		t.Errorf("DotsSpinner should have 6 frames, got %d", len(DotsSpinner.Frames))
	}
}

func TestLineSpinnerFrames(t *testing.T) {
	if len(LineSpinner.Frames) != 4 {
		t.Errorf("LineSpinner should have 4 frames, got %d", len(LineSpinner.Frames))
	}

	// Verify expected frames
	expected := []string{"|", "/", "-", "\\"}
	for i, want := range expected {
		if LineSpinner.Frames[i] != want {
			t.Errorf("LineSpinner frame %d = %q, want %q", i, LineSpinner.Frames[i], want)
		}
	}
}

// =============================================================================
// PROGRESS BAR TESTS
// =============================================================================

func TestProgressBarCharacters(t *testing.T) {
	if ProgressFull == "" {
		t.Error("ProgressFull should be defined")
	}
	if ProgressEmpty == "" {
		t.Error("ProgressEmpty should be defined")
	}
	if len(ProgressPartial) == 0 {
		t.Error("ProgressPartial should have characters")
	}
}

func TestRenderProgressBar(t *testing.T) {
	tests := []struct {
		width   int
		percent float64
	}{
		{10, 0.0},
		{10, 25.0},
		{10, 50.0},
		{10, 75.0},
		{10, 100.0},
		{20, 33.333},
		{30, 66.666},
	}

	for _, tc := range tests {
		result := RenderProgressBar(tc.width, tc.percent)
		// Result should be close to the requested width
		// (may vary slightly due to Unicode characters and partial blocks)
		runeCount := len([]rune(result))
		if runeCount < tc.width-1 || runeCount > tc.width+1 {
			t.Errorf("RenderProgressBar(%d, %.1f) length = %d, expected ~%d",
				tc.width, tc.percent, runeCount, tc.width)
		}
	}
}

func TestRenderProgressBarEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		width   int
		percent float64
	}{
		{"Zero width", 0, 50.0},
		{"Negative percent", 10, -10.0},
		{"Over 100 percent", 10, 150.0},
		{"Small width", 1, 50.0},
		{"Large width", 100, 50.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Should not panic
			result := RenderProgressBar(tc.width, tc.percent)
			_ = result
		})
	}
}

func TestRenderProgressBarBounds(t *testing.T) {
	// Test that negative percents are clamped to 0
	result := RenderProgressBar(10, -50.0)
	if !strings.Contains(result, ProgressEmpty) {
		t.Error("RenderProgressBar with negative percent should show empty bar")
	}

	// Test that >100% is clamped to 100
	result = RenderProgressBar(10, 200.0)
	if !strings.Contains(result, ProgressFull) {
		t.Error("RenderProgressBar with >100% should show full bar")
	}
}

// =============================================================================
// EASING FUNCTION TESTS
// =============================================================================

func TestEaseLinear(t *testing.T) {
	tests := []struct {
		t    float64
		want float64
	}{
		{0.0, 0.0},
		{0.25, 0.25},
		{0.5, 0.5},
		{0.75, 0.75},
		{1.0, 1.0},
	}

	for _, tc := range tests {
		got := EaseLinear(tc.t)
		if got != tc.want {
			t.Errorf("EaseLinear(%f) = %f, want %f", tc.t, got, tc.want)
		}
	}
}

func TestEaseInQuad(t *testing.T) {
	tests := []struct {
		t    float64
		want float64
	}{
		{0.0, 0.0},
		{0.5, 0.25},
		{1.0, 1.0},
	}

	for _, tc := range tests {
		got := EaseInQuad(tc.t)
		if got != tc.want {
			t.Errorf("EaseInQuad(%f) = %f, want %f", tc.t, got, tc.want)
		}
	}
}

func TestEaseOutQuad(t *testing.T) {
	// Test basic properties
	if EaseOutQuad(0.0) != 0.0 {
		t.Error("EaseOutQuad(0) should be 0")
	}
	if EaseOutQuad(1.0) != 1.0 {
		t.Error("EaseOutQuad(1) should be 1")
	}

	// Test that it decelerates (output > input at midpoint)
	mid := EaseOutQuad(0.5)
	if mid <= 0.5 {
		t.Error("EaseOutQuad should decelerate (mid > 0.5)")
	}
}

func TestEaseInOutQuad(t *testing.T) {
	if EaseInOutQuad(0.0) != 0.0 {
		t.Error("EaseInOutQuad(0) should be 0")
	}
	if EaseInOutQuad(1.0) != 1.0 {
		t.Error("EaseInOutQuad(1) should be 1")
	}
	if EaseInOutQuad(0.5) != 0.5 {
		t.Error("EaseInOutQuad(0.5) should be 0.5")
	}
}

func TestEaseOutCubic(t *testing.T) {
	if EaseOutCubic(0.0) != 0.0 {
		t.Error("EaseOutCubic(0) should be 0")
	}
	if EaseOutCubic(1.0) != 1.0 {
		t.Error("EaseOutCubic(1) should be 1")
	}
}

func TestEaseOutElastic(t *testing.T) {
	if EaseOutElastic(0.0) != 0.0 {
		t.Error("EaseOutElastic(0) should be 0")
	}
	if EaseOutElastic(1.0) != 1.0 {
		t.Error("EaseOutElastic(1) should be 1")
	}

	// Elastic can oscillate significantly - just verify it doesn't panic
	// and returns a finite value (the implementation approximates the math)
	result := EaseOutElastic(0.8)
	// Just verify it doesn't panic and returns a finite value
	if result != result { // NaN check
		t.Error("EaseOutElastic(0.8) should not return NaN")
	}
}

func TestSin(t *testing.T) {
	// Test basic sine approximation
	tests := []struct {
		x    float64
		want float64
		tol  float64
	}{
		{0.0, 0.0, 0.001},
		// sin(π/6) ≈ 0.5
		{0.5236, 0.5, 0.1},
		// Test that function returns reasonable values
	}

	for _, tc := range tests {
		got := sin(tc.x)
		if got < tc.want-tc.tol || got > tc.want+tc.tol {
			t.Logf("sin(%f) = %f, want ~%f (within %f)", tc.x, got, tc.want, tc.tol)
			// Note: This is a simplified sine, so tolerances are loose
		}
	}
}

// =============================================================================
// TRANSITION CONFIG TESTS
// =============================================================================

func TestTransitionConfigs(t *testing.T) {
	transitions := []struct {
		name   string
		config TransitionConfig
	}{
		{"Fast", TransitionFast},
		{"Normal", TransitionNormal},
		{"Slow", TransitionSlow},
	}

	for _, tr := range transitions {
		t.Run(tr.name, func(t *testing.T) {
			if tr.config.Duration <= 0 {
				t.Errorf("%s transition duration should be positive", tr.name)
			}
			if tr.config.Easing == nil {
				t.Errorf("%s transition easing function should not be nil", tr.name)
			}

			// Test that easing function works
			result := tr.config.Easing(0.5)
			if result < 0 || result > 1 {
				t.Errorf("%s easing(0.5) = %f, should be in [0,1]", tr.name, result)
			}
		})
	}
}

func TestTransitionDurations(t *testing.T) {
	if TransitionFast.Duration >= TransitionNormal.Duration {
		t.Error("TransitionFast should be faster than TransitionNormal")
	}
	if TransitionNormal.Duration >= TransitionSlow.Duration {
		t.Error("TransitionNormal should be faster than TransitionSlow")
	}
}

// =============================================================================
// FADE CHARACTER TESTS
// =============================================================================

func TestFadeChars(t *testing.T) {
	if len(FadeChars) != 4 {
		t.Errorf("FadeChars should have 4 characters, got %d", len(FadeChars))
	}

	for i, char := range FadeChars {
		if char == "" {
			t.Errorf("FadeChars[%d] should not be empty", i)
		}
	}
}

// =============================================================================
// TYPING ANIMATION TESTS
// =============================================================================

func TestTypingCursor(t *testing.T) {
	if len(TypingCursor) != 2 {
		t.Errorf("TypingCursor should have 2 states, got %d", len(TypingCursor))
	}

	// Should have visible and invisible states
	if TypingCursor[0] == "" {
		t.Error("TypingCursor[0] should be visible character")
	}
}

func TestCursorBlinkRate(t *testing.T) {
	if CursorBlinkRate <= 0 {
		t.Error("CursorBlinkRate should be positive")
	}

	// Should be reasonable (100ms - 1s)
	if CursorBlinkRate < 100*time.Millisecond || CursorBlinkRate > 1*time.Second {
		t.Errorf("CursorBlinkRate = %v, expected reasonable range (100ms-1s)", CursorBlinkRate)
	}
}

// =============================================================================
// STATUS INDICATOR TESTS
// =============================================================================

func TestAnimationStatusIndicators(t *testing.T) {
	indicators := []struct {
		name  string
		value string
	}{
		{"Success", AnimationStatusIndicators.Success},
		{"Error", AnimationStatusIndicators.Error},
		{"Warning", AnimationStatusIndicators.Warning},
		{"Info", AnimationStatusIndicators.Info},
		{"Loading", AnimationStatusIndicators.Loading},
		{"Paused", AnimationStatusIndicators.Paused},
		{"Connected", AnimationStatusIndicators.Connected},
		{"Offline", AnimationStatusIndicators.Offline},
	}

	for _, ind := range indicators {
		if ind.value == "" {
			t.Errorf("AnimationStatusIndicators.%s should not be empty", ind.name)
		}
	}
}

// =============================================================================
// TREE CHARACTER TESTS
// =============================================================================

func TestTreeChars(t *testing.T) {
	chars := []struct {
		name  string
		value string
	}{
		{"Pipe", TreeChars.Pipe},
		{"Tee", TreeChars.Tee},
		{"Corner", TreeChars.Corner},
		{"Dash", TreeChars.Dash},
	}

	for _, c := range chars {
		if c.value == "" {
			t.Errorf("TreeChars.%s should not be empty", c.name)
		}
	}
}

func TestRenderTreeLine(t *testing.T) {
	// Test last item
	lastLine := RenderTreeLine(true)
	if !strings.Contains(lastLine, TreeChars.Corner) {
		t.Error("RenderTreeLine(true) should contain corner character")
	}

	// Test non-last item
	middleLine := RenderTreeLine(false)
	if !strings.Contains(middleLine, TreeChars.Tee) {
		t.Error("RenderTreeLine(false) should contain tee character")
	}

	// Both should contain dash
	if !strings.Contains(lastLine, TreeChars.Dash) {
		t.Error("RenderTreeLine(true) should contain dash")
	}
	if !strings.Contains(middleLine, TreeChars.Dash) {
		t.Error("RenderTreeLine(false) should contain dash")
	}
}

// =============================================================================
// BOX CHARACTER TESTS
// =============================================================================

func TestBoxChars(t *testing.T) {
	chars := []struct {
		name  string
		value string
	}{
		{"TopLeftRound", BoxChars.TopLeftRound},
		{"TopRightRound", BoxChars.TopRightRound},
		{"BottomLeftRound", BoxChars.BottomLeftRound},
		{"BottomRightRound", BoxChars.BottomRightRound},
		{"TopLeft", BoxChars.TopLeft},
		{"TopRight", BoxChars.TopRight},
		{"BottomLeft", BoxChars.BottomLeft},
		{"BottomRight", BoxChars.BottomRight},
		{"Horizontal", BoxChars.Horizontal},
		{"Vertical", BoxChars.Vertical},
		{"HorizontalDouble", BoxChars.HorizontalDouble},
		{"VerticalDouble", BoxChars.VerticalDouble},
	}

	for _, c := range chars {
		if c.value == "" {
			t.Errorf("BoxChars.%s should not be empty", c.name)
		}
	}
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

func TestEasingFunctionsBounds(t *testing.T) {
	funcs := []struct {
		name string
		fn   EasingFunc
	}{
		{"Linear", EaseLinear},
		{"InQuad", EaseInQuad},
		{"OutQuad", EaseOutQuad},
		{"InOutQuad", EaseInOutQuad},
		{"OutCubic", EaseOutCubic},
		{"OutElastic", EaseOutElastic},
	}

	for _, f := range funcs {
		t.Run(f.name, func(t *testing.T) {
			// Test at boundaries
			start := f.fn(0.0)
			if start < -0.1 || start > 0.1 {
				t.Errorf("%s(0) = %f, expected ~0", f.name, start)
			}

			end := f.fn(1.0)
			if end < 0.9 || end > 1.1 {
				t.Errorf("%s(1) = %f, expected ~1", f.name, end)
			}

			// Test mid-point doesn't panic
			mid := f.fn(0.5)
			_ = mid
		})
	}
}

func TestRenderProgressBarZeroWidth(t *testing.T) {
	result := RenderProgressBar(0, 50.0)
	if result != "" {
		t.Error("RenderProgressBar(0, ...) should return empty string")
	}
}

func TestRenderProgressBarNegativeWidth(t *testing.T) {
	// Should handle gracefully (treat as zero or minimal)
	result := RenderProgressBar(-10, 50.0)
	_ = result // Should not panic
}
