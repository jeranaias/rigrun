// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package styles provides the visual styling system for rigrun TUI.
// All colors use Lip Gloss AdaptiveColor for automatic light/dark detection.
package styles

import "github.com/charmbracelet/lipgloss"

// =============================================================================
// PRIMARY ACCENT COLORS
// =============================================================================

// Purple - Primary accent, assistant messages, selections
var Purple = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"}

// PurpleDeep - Darker purple for backgrounds
var PurpleDeep = lipgloss.AdaptiveColor{Light: "#5B21B6", Dark: "#4C1D95"}

// Cyan - Brand color, info, commands, user highlights
var Cyan = lipgloss.AdaptiveColor{Light: "#0891B2", Dark: "#22D3EE"}

// CyanDeep - Darker cyan for backgrounds
var CyanDeep = lipgloss.AdaptiveColor{Light: "#0E7490", Dark: "#164E63"}

// Emerald - Success states, local mode indicator
var Emerald = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"}

// EmeraldDeep - Darker emerald for backgrounds
var EmeraldDeep = lipgloss.AdaptiveColor{Light: "#047857", Dark: "#064E3B"}

// =============================================================================
// SEMANTIC COLORS
// =============================================================================

// Rose - Errors, critical alerts, danger states
var Rose = lipgloss.AdaptiveColor{Light: "#E11D48", Dark: "#FB7185"}

// RoseDeep - Darker rose for backgrounds
var RoseDeep = lipgloss.AdaptiveColor{Light: "#BE123C", Dark: "#881337"}

// Amber - Warnings, cloud mode, caution states
var Amber = lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#FBBF24"}

// AmberDeep - Darker amber for backgrounds
var AmberDeep = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#78350F"}

// =============================================================================
// SURFACE COLORS
// =============================================================================

// Surface - Main background
var Surface = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#1E1E2E"}

// SurfaceDim - Slightly darker/lighter surface for headers/footers
var SurfaceDim = lipgloss.AdaptiveColor{Light: "#F5F5F5", Dark: "#181825"}

// SurfaceBright - Slightly lighter/darker surface for highlights
var SurfaceBright = lipgloss.AdaptiveColor{Light: "#FAFAFA", Dark: "#313244"}

// Overlay - Borders, separators, subtle backgrounds
var Overlay = lipgloss.AdaptiveColor{Light: "#E5E5E5", Dark: "#313244"}

// OverlayDim - Dimmer overlay for less prominent elements
var OverlayDim = lipgloss.AdaptiveColor{Light: "#D4D4D4", Dark: "#45475A"}

// =============================================================================
// TEXT COLORS
// =============================================================================

// TextPrimary - Main body text
var TextPrimary = lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#CDD6F4"}

// TextSecondary - Labels, less prominent text
var TextSecondary = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#A6ADC8"}

// TextMuted - Hints, timestamps, very subtle text
var TextMuted = lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6C7086"}

// TextInverse - Text on colored backgrounds
var TextInverse = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#1E1E2E"}

// =============================================================================
// MESSAGE BUBBLE COLORS
// =============================================================================

// User message bubble - Blue tones
var UserBubbleBg = lipgloss.AdaptiveColor{Light: "#DBEAFE", Dark: "#1D4ED8"}
var UserBubbleFg = lipgloss.AdaptiveColor{Light: "#1E40AF", Dark: "#E0F2FE"}
var UserBubbleBorder = lipgloss.AdaptiveColor{Light: "#3B82F6", Dark: "#3B82F6"}

// Assistant message bubble - Soft purple/violet tones (muted, not saturated)
var AssistantBubbleBg = lipgloss.AdaptiveColor{Light: "#F5F3FF", Dark: "#3B3655"}
var AssistantBubbleFg = lipgloss.AdaptiveColor{Light: "#5B4B8A", Dark: "#E9E4F5"}
var AssistantBubbleBorder = lipgloss.AdaptiveColor{Light: "#C4B5FD", Dark: "#A78BFA"}

// System message bubble - Amber/yellow tones
var SystemBubbleBg = lipgloss.AdaptiveColor{Light: "#FEF3C7", Dark: "#78350F"}
var SystemBubbleFg = lipgloss.AdaptiveColor{Light: "#92400E", Dark: "#FEF3C7"}
var SystemBubbleBorder = lipgloss.AdaptiveColor{Light: "#F59E0B", Dark: "#F59E0B"}

// Tool result - Emerald for success, Rose for error
var ToolSuccessBg = lipgloss.AdaptiveColor{Light: "#D1FAE5", Dark: "#064E3B"}
var ToolSuccessFg = lipgloss.AdaptiveColor{Light: "#065F46", Dark: "#A7F3D0"}
var ToolErrorBg = lipgloss.AdaptiveColor{Light: "#FEE2E2", Dark: "#881337"}
var ToolErrorFg = lipgloss.AdaptiveColor{Light: "#991B1B", Dark: "#FECACA"}

// =============================================================================
// SYNTAX HIGHLIGHTING (Catppuccin Latte/Mocha)
// =============================================================================

var SyntaxKeyword = lipgloss.AdaptiveColor{Light: "#8839EF", Dark: "#CBA6F7"}   // Mauve
var SyntaxString = lipgloss.AdaptiveColor{Light: "#40A02B", Dark: "#A6E3A1"}    // Green
var SyntaxNumber = lipgloss.AdaptiveColor{Light: "#FE640B", Dark: "#FAB387"}    // Peach
var SyntaxComment = lipgloss.AdaptiveColor{Light: "#9CA0B0", Dark: "#6C7086"}   // Overlay0
var SyntaxFunction = lipgloss.AdaptiveColor{Light: "#1E66F5", Dark: "#89B4FA"}  // Blue
var SyntaxType = lipgloss.AdaptiveColor{Light: "#DF8E1D", Dark: "#F9E2AF"}      // Yellow
var SyntaxOperator = lipgloss.AdaptiveColor{Light: "#04A5E5", Dark: "#89DCEB"}  // Sky
var SyntaxVariable = lipgloss.AdaptiveColor{Light: "#EA76CB", Dark: "#F5C2E7"}  // Pink
var SyntaxConstant = lipgloss.AdaptiveColor{Light: "#FE640B", Dark: "#FAB387"}  // Peach
var SyntaxBuiltin = lipgloss.AdaptiveColor{Light: "#D20F39", Dark: "#F38BA8"}   // Red

// =============================================================================
// SPECIAL EFFECTS
// =============================================================================

// Gradient start/end for header effects
var GradientStart = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"} // Purple
var GradientEnd = lipgloss.AdaptiveColor{Light: "#0891B2", Dark: "#22D3EE"}   // Cyan

// Focus ring color
var FocusRing = Cyan

// Selection highlight
var SelectionBg = lipgloss.AdaptiveColor{Light: "#BFDBFE", Dark: "#1E3A5F"}

// =============================================================================
// ACCESSIBILITY: Shapes and high contrast for colorblind users
// =============================================================================

// StatusIndicatorSet contains text/shape indicators for status states.
// These symbols provide visual cues beyond color for colorblind accessibility.
type StatusIndicatorSet struct {
	Success string // Checkmark for success states
	Error   string // X mark for error states
	Warning string // Warning triangle for caution states
	Info    string // Info circle for informational states
	Pending string // Clock for pending/loading states
	Active  string // Dot for active/online states
}

// StatusIndicators provides accessible shape/text indicators alongside colors.
// ACCESSIBILITY: ASCII-only indicators for maximum compatibility and colorblind users.
var StatusIndicators = StatusIndicatorSet{
	Success: "[OK]",   // ASCII checkmark alternative
	Error:   "[X]",    // ASCII X mark alternative
	Warning: "[!]",    // ASCII warning alternative
	Info:    "[i]",    // ASCII info alternative
	Pending: "[ ]",    // ASCII empty for pending
	Active:  "[*]",    // ASCII filled for active
}

// NOTE: AccessibilityConfig is not currently implemented for runtime switching.
// The StatusIndicators system is always enabled and provides shape-based indicators
// for colorblind accessibility. If runtime accessibility mode switching is needed in
// the future, implement it as part of the config package with persistence.
//
// For now, accessibility features are always on:
// - StatusIndicators always include ASCII shape indicators ([OK], [X], [!], [i])
// - High contrast colors are used throughout the theme
// - No runtime toggling is needed

// =============================================================================
// ACCESSIBILITY: High-contrast color pairs for colorblind users
// =============================================================================

// High contrast success - Bright green with bold, works for most color blindness types
var SuccessHighContrast = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#22C55E"}

// High contrast error - Bright red with bold, distinct from green even for colorblind
var ErrorHighContrast = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#EF4444"}

// High contrast warning - Bright amber/orange, deuteranopia-friendly
var WarningHighContrast = lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#F59E0B"}

// High contrast info - Bright blue, distinct from red/green spectrum
var InfoHighContrast = lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#3B82F6"}

// =============================================================================
// ACCESSIBILITY: Deuteranopia-friendly alternative color pairs
// Uses blue for success and orange for error (avoids red-green confusion)
// =============================================================================

// DeuteranopiaSafeSuccess - Blue instead of green for deuteranopia users
var DeuteranopiaSafeSuccess = lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#60A5FA"}

// DeuteranopiaSafeError - Orange instead of red for deuteranopia users
var DeuteranopiaSafeError = lipgloss.AdaptiveColor{Light: "#EA580C", Dark: "#FB923C"}

// =============================================================================
// ACCESSIBILITY: Link style with underline for visual distinction
// =============================================================================

// LinkColor - Accessible link color with sufficient contrast
var LinkColor = lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#60A5FA"}

// =============================================================================
// ACCESSIBILITY: Helper functions for rendering accessible status messages
// =============================================================================

// RenderSuccess renders a success message with checkmark indicator and high contrast green.
// ACCESSIBILITY: Includes shape indicator for colorblind users.
func RenderSuccess(message string) string {
	style := lipgloss.NewStyle().
		Foreground(SuccessHighContrast).
		Bold(true)
	return style.Render(StatusIndicators.Success + " " + message)
}

// RenderError renders an error message with X mark indicator and high contrast red.
// ACCESSIBILITY: Includes shape indicator for colorblind users.
func RenderError(message string) string {
	style := lipgloss.NewStyle().
		Foreground(ErrorHighContrast).
		Bold(true)
	return style.Render(StatusIndicators.Error + " " + message)
}

// RenderWarning renders a warning message with warning triangle and high contrast amber.
// ACCESSIBILITY: Includes shape indicator for colorblind users.
func RenderWarning(message string) string {
	style := lipgloss.NewStyle().
		Foreground(WarningHighContrast).
		Bold(true)
	return style.Render(StatusIndicators.Warning + " " + message)
}

// RenderInfo renders an info message with info circle and high contrast blue.
// ACCESSIBILITY: Includes shape indicator for colorblind users.
func RenderInfo(message string) string {
	style := lipgloss.NewStyle().
		Foreground(InfoHighContrast).
		Bold(true)
	return style.Render(StatusIndicators.Info + " " + message)
}

// RenderStatus renders a status message based on success/failure with appropriate indicator.
// ACCESSIBILITY: Uses shapes and high contrast colors for colorblind users.
func RenderStatus(success bool, message string) string {
	if success {
		return RenderSuccess(message)
	}
	return RenderError(message)
}

// RenderLink renders text as an accessible link with underline.
// ACCESSIBILITY: Underline provides visual cue beyond color for links.
func RenderLink(text string) string {
	style := lipgloss.NewStyle().
		Foreground(LinkColor).
		Underline(true)
	return style.Render(text)
}
