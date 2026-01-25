// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package styles provides the visual styling system for rigrun TUI.
package styles

import (
	"strings"
	"time"
)

// =============================================================================
// SPINNER ANIMATIONS
// =============================================================================

// BrailleSpinner - Smooth ASCII spinner (previously braille)
var BrailleSpinner = SpinnerConfig{
	Frames: []string{"|", "/", "-", "\\", "|", "/", "-", "\\", "|", "/"},
	FPS:    12,
}

// DotsSpinner - Classic three-dot animation
var DotsSpinner = SpinnerConfig{
	Frames: []string{".  ", ".. ", "...", " ..", "  .", "   "},
	FPS:    6,
}

// LineSpinner - Simple line rotation
var LineSpinner = SpinnerConfig{
	Frames: []string{"|", "/", "-", "\\"},
	FPS:    10,
}

// BlockSpinner - Growing/shrinking progress
var BlockSpinner = SpinnerConfig{
	Frames: []string{"[", "[=", "[==", "[===", "[====", "[=====", "[====", "[===", "[==", "[="},
	FPS:    15,
}

// PulseSpinner - Pulsing indicator
var PulseSpinner = SpinnerConfig{
	Frames: []string{"( )", "(.)", "(o)", "(O)", "(o)", "(.)", "( )", "   "},
	FPS:    8,
}

// ArrowSpinner - Rotating arrow (ASCII-safe alternative to moon phases)
var ArrowSpinner = SpinnerConfig{
	Frames: []string{"<", "^", ">", "v"},
	FPS:    5,
}

// ProgressSpinner - Progress dots (ASCII-safe alternative to clock)
var ProgressSpinner = SpinnerConfig{
	Frames: []string{"[    ]", "[=   ]", "[==  ]", "[=== ]", "[====]", "[ ===]", "[  ==]", "[   =]"},
	FPS:    4,
}

// SpinnerConfig holds the configuration for a spinner animation.
type SpinnerConfig struct {
	Frames []string
	FPS    int
}

// Duration returns the duration for each frame.
func (s SpinnerConfig) Duration() time.Duration {
	return time.Second / time.Duration(s.FPS)
}

// =============================================================================
// PROGRESS INDICATORS
// =============================================================================

// ProgressBar characters for context bar and other progress displays.
var (
	ProgressFull    = "#"
	ProgressEmpty   = "-"
	ProgressPartial = []string{".", ":", "+", "#", "#", "#", "#"}
)

// RenderProgressBar creates a progress bar string.
// width: total width of the bar in characters
// percent: 0-100 percentage complete
func RenderProgressBar(width int, percent float64) string {
	// Handle invalid width
	if width <= 0 {
		return ""
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	filledWidth := float64(width) * percent / 100
	fullBlocks := int(filledWidth)
	partialIndex := int((filledWidth - float64(fullBlocks)) * float64(len(ProgressPartial)))

	// PERFORMANCE: strings.Builder avoids quadratic allocations
	var sb strings.Builder
	sb.Grow(width * 3) // Pre-allocate for Unicode characters (up to 3 bytes each)

	for i := 0; i < fullBlocks && i < width; i++ {
		sb.WriteString(ProgressFull)
	}

	if fullBlocks < width && partialIndex > 0 {
		sb.WriteString(ProgressPartial[partialIndex-1])
		fullBlocks++
	}

	for i := fullBlocks; i < width; i++ {
		sb.WriteString(ProgressEmpty)
	}

	return sb.String()
}

// =============================================================================
// TRANSITION EFFECTS
// =============================================================================

// FadeChars for fade-in/fade-out transitions
var FadeChars = []string{".", ":", "+", "#"}

// TransitionConfig defines a transition animation.
type TransitionConfig struct {
	Duration time.Duration
	Easing   EasingFunc
}

// EasingFunc is a function that maps progress (0-1) to output (0-1).
type EasingFunc func(t float64) float64

// Linear easing - constant speed
func EaseLinear(t float64) float64 {
	return t
}

// EaseInQuad - accelerating from zero
func EaseInQuad(t float64) float64 {
	return t * t
}

// EaseOutQuad - decelerating to zero
func EaseOutQuad(t float64) float64 {
	return t * (2 - t)
}

// EaseInOutQuad - acceleration until halfway, then deceleration
func EaseInOutQuad(t float64) float64 {
	if t < 0.5 {
		return 2 * t * t
	}
	return -1 + (4-2*t)*t
}

// EaseOutCubic - decelerating to zero (smoother)
func EaseOutCubic(t float64) float64 {
	t--
	return t*t*t + 1
}

// EaseOutElastic - overshoot with elastic bounce
func EaseOutElastic(t float64) float64 {
	if t == 0 || t == 1 {
		return t
	}
	p := 0.3
	s := p / 4
	// Calculate 2^(10*(t-1)) using repeated multiplication
	power := 1.0
	exponent := 10 * (t - 1)
	// Approximate 2^exponent using multiplication
	base := 2.0
	for exponent > 1 {
		power *= base
		exponent--
	}
	if exponent > 0 {
		power *= (1 + exponent*(base-1)) // Linear interpolation for fractional part
	}
	// Simplified elastic formula
	return 1 + (-power * sin((t-1-s)*(2*3.14159)/p))
}

func sin(x float64) float64 {
	// Simple sine approximation for animation
	// Using Taylor series: sin(x) ≈ x - x³/6 + x⁵/120
	x3 := x * x * x
	x5 := x3 * x * x
	return x - x3/6 + x5/120
}

// Default transitions
var (
	TransitionFast = TransitionConfig{
		Duration: 150 * time.Millisecond,
		Easing:   EaseOutQuad,
	}
	TransitionNormal = TransitionConfig{
		Duration: 300 * time.Millisecond,
		Easing:   EaseOutCubic,
	}
	TransitionSlow = TransitionConfig{
		Duration: 500 * time.Millisecond,
		Easing:   EaseInOutQuad,
	}
)

// =============================================================================
// TYPING ANIMATION
// =============================================================================

// TypingCursor characters for blinking cursor
var TypingCursor = []string{"_", " "}

// CursorBlinkRate is the rate at which the cursor blinks
var CursorBlinkRate = 530 * time.Millisecond

// =============================================================================
// STATUS INDICATORS
// =============================================================================

// AnimationStatusIndicators for various states (ASCII-only for compatibility)
// Note: AccessibilityIndicators in colors.go provides the primary shape indicators
var AnimationStatusIndicators = struct {
	Success   string
	Error     string
	Warning   string
	Info      string
	Loading   string
	Paused    string
	Connected string
	Offline   string
}{
	Success:   "[OK]",
	Error:     "[X]",
	Warning:   "[!]",
	Info:      "[i]",
	Loading:   "[.]",
	Paused:    "||",
	Connected: "(+)",
	Offline:   "(-)",
}

// =============================================================================
// TREE CONNECTORS
// =============================================================================

// TreeChars for rendering tree structures (like thinking details)
var TreeChars = struct {
	Pipe     string
	Tee      string
	Corner   string
	Dash     string
}{
	Pipe:   "|",
	Tee:    "+",
	Corner: "`",
	Dash:   "-",
}

// RenderTreeLine creates a tree line prefix.
// isLast: true if this is the last item in the list
func RenderTreeLine(isLast bool) string {
	if isLast {
		return TreeChars.Corner + TreeChars.Dash + " "
	}
	return TreeChars.Tee + TreeChars.Dash + " "
}

// =============================================================================
// BORDER CHARACTERS (for custom borders)
// =============================================================================

// BoxChars for custom box drawing (ASCII-safe)
var BoxChars = struct {
	// Rounded corners
	TopLeftRound     string
	TopRightRound    string
	BottomLeftRound  string
	BottomRightRound string
	// Sharp corners
	TopLeft     string
	TopRight    string
	BottomLeft  string
	BottomRight string
	// Lines
	Horizontal string
	Vertical   string
	// Double lines
	HorizontalDouble string
	VerticalDouble   string
}{
	TopLeftRound:     "+",
	TopRightRound:    "+",
	BottomLeftRound:  "+",
	BottomRightRound: "+",
	TopLeft:          "+",
	TopRight:         "+",
	BottomLeft:       "+",
	BottomRight:      "+",
	Horizontal:       "-",
	Vertical:         "|",
	HorizontalDouble: "=",
	VerticalDouble:   "|",
}
