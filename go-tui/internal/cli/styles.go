// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// styles.go - Centralized styling for all CLI commands in rigrun.
//
// USABILITY: TTY detection for proper terminal handling
//
// This file eliminates the duplication of style definitions across
// 23+ command files. All CLI commands should import and use these
// shared styles instead of defining their own.
//
// Color handling:
// - Colors are automatically disabled for non-TTY output (piped, redirected)
// - Respects NO_COLOR environment variable (https://no-color.org/)
// - Supports FORCE_COLOR environment variable to override detection
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// init configures lipgloss color profile based on terminal capabilities.
// USABILITY: TTY detection for proper terminal handling
func init() {
	// Configure lipgloss to use the appropriate color profile
	// This respects NO_COLOR, FORCE_COLOR, and TTY detection
	lipgloss.SetColorProfile(GetColorProfile())
}

// =============================================================================
// SHARED STYLES FOR ALL CLI COMMANDS
// =============================================================================

var (
	// TitleStyle is used for command titles and headers
	// Color: Cyan (#39) - consistent across all commands
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")). // Cyan
			MarginBottom(1)

	// SectionStyle is used for section headers within commands
	// Color: White (#255) - bold section dividers
	SectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")). // White
			MarginTop(1)

	// LabelStyle is used for field labels (left-aligned prompts)
	// Color: Light gray (#245) - subtle but readable
	// Width: 20 characters by default (can be overridden inline)
	LabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")). // Light gray
			Width(20)

	// ValueStyle is used for regular values and text
	// Color: White (#252) - slightly dimmer than section headers
	ValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")) // Off-white

	// SuccessStyle is used for success messages and OK statuses
	// Color: Green (#42) - indicates successful operations
	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")). // Green
			Bold(true)

	// ErrorStyle is used for error messages and failures
	// Color: Red (#196) - indicates errors and failures
	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")). // Red
			Bold(true)

	// WarningStyle is used for warnings and cautions
	// Color: Yellow/Orange (#214) - indicates warnings
	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")) // Yellow/Orange

	// DimStyle is used for secondary information and hints
	// Color: Dim gray (#242) - de-emphasized text
	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")) // Dim gray

	// SeparatorStyle is used for visual separators
	// Color: Dark gray (#240) - subtle dividers
	SeparatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")) // Dark gray

	// HighlightStyle is used for highlighted text and emphasis
	// Color: Bright green (#82) - draws attention without being alarming
	HighlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")) // Bright green

	// InfoStyle is used for informational messages
	// Color: Blue (#75) - neutral information
	InfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("75")) // Blue
)

// =============================================================================
// SEMANTIC STATUS STYLES
// =============================================================================
// These provide semantic meaning for consistent status display

var (
	// StatusOKStyle - for successful status checks
	StatusOKStyle = SuccessStyle

	// StatusFailStyle - for failed status checks
	StatusFailStyle = ErrorStyle

	// StatusPendingStyle - for pending/in-progress operations
	StatusPendingStyle = WarningStyle

	// StatusUnknownStyle - for unknown or N/A status
	StatusUnknownStyle = DimStyle
)

// =============================================================================
// HELPER FUNCTIONS FOR COMMON PATTERNS
// =============================================================================

// RenderSeparator renders a horizontal separator line of the specified width.
// Default width is 70 characters if not specified.
func RenderSeparator(width ...int) string {
	w := 70
	if len(width) > 0 && width[0] > 0 {
		w = width[0]
	}
	return SeparatorStyle.Render(strings.Repeat("=", w))
}

// RenderStatus renders a status indicator with appropriate color.
// status should be one of: "ok", "success", "error", "fail", "warning", "pending", "unknown"
func RenderStatus(status string) string {
	switch strings.ToLower(status) {
	case "ok", "success", "pass", "compliant":
		return StatusOKStyle.Render("[OK]")
	case "error", "fail", "failed", "non-compliant":
		return StatusFailStyle.Render("[FAIL]")
	case "warning", "warn", "pending":
		return StatusPendingStyle.Render("[WARN]")
	default:
		return StatusUnknownStyle.Render("[" + strings.ToUpper(status) + "]")
	}
}

// RenderLabel renders a label with consistent width.
// If width is specified, it overrides the default 20 characters.
func RenderLabel(label string, width ...int) string {
	if len(width) > 0 && width[0] > 0 {
		return LabelStyle.Copy().Width(width[0]).Render(label)
	}
	return LabelStyle.Render(label)
}

// =============================================================================
// TTY-AWARE STYLING HELPERS
// USABILITY: TTY detection for proper terminal handling
// =============================================================================

// RenderConditional renders text with style if colors are enabled,
// otherwise returns the text unmodified.
// Use this when you need explicit control over when styling is applied.
func RenderConditional(style lipgloss.Style, text string) string {
	if !ColorsEnabled() {
		return text
	}
	return style.Render(text)
}

// PlainStyle returns an unstyled lipgloss.Style (no colors, no formatting).
// Use this as a fallback when colors should be disabled.
func PlainStyle() lipgloss.Style {
	return lipgloss.NewStyle()
}

// GetStyleForTTY returns the provided style if colors are enabled,
// otherwise returns a plain style.
// USABILITY: TTY detection for proper terminal handling
func GetStyleForTTY(coloredStyle lipgloss.Style) lipgloss.Style {
	if !ColorsEnabled() {
		return PlainStyle()
	}
	return coloredStyle
}

// RenderSeparatorAdaptive renders a separator that adapts to terminal width.
// Uses GetTerminalWidth() to determine the appropriate width.
func RenderSeparatorAdaptive() string {
	width := GetTerminalWidth()
	// Leave some margin
	if width > 4 {
		width -= 4
	}
	// Cap at reasonable maximum
	if width > 80 {
		width = 80
	}
	return RenderSeparator(width)
}

// RenderWrapped renders text wrapped to terminal width with optional style.
// USABILITY: TTY detection for proper terminal handling
func RenderWrapped(style lipgloss.Style, text string) string {
	wrapped := WrapText(text, GetTerminalWidth())
	if !ColorsEnabled() {
		return wrapped
	}
	return style.Render(wrapped)
}
