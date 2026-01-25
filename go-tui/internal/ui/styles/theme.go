// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package styles provides the visual styling system for rigrun TUI.
package styles

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Theme holds all the styled components for the application.
// It detects the terminal's color capability and adjusts accordingly.
type Theme struct {
	// Terminal capabilities
	IsDark         bool
	HasTrueColor   bool
	ColorProfile   termenv.Profile

	// Layout dimensions
	Width  int
	Height int

	// ==========================================================================
	// APPLICATION CONTAINER STYLES
	// ==========================================================================

	App       lipgloss.Style
	Container lipgloss.Style

	// ==========================================================================
	// HEADER STYLES
	// ==========================================================================

	Header         lipgloss.Style
	HeaderTitle    lipgloss.Style
	HeaderSubtitle lipgloss.Style
	HeaderBrand    lipgloss.Style

	// ==========================================================================
	// MESSAGE BUBBLE STYLES
	// ==========================================================================

	UserBubble      lipgloss.Style
	AssistantBubble lipgloss.Style
	SystemBubble    lipgloss.Style
	ToolSuccess     lipgloss.Style
	ToolError       lipgloss.Style

	// ==========================================================================
	// INPUT AREA STYLES
	// ==========================================================================

	InputContainer    lipgloss.Style
	InputPrompt       lipgloss.Style
	InputText         lipgloss.Style
	InputPlaceholder  lipgloss.Style
	CharCount         lipgloss.Style
	CharCountWarning  lipgloss.Style
	CharCountDanger   lipgloss.Style

	// ==========================================================================
	// STATUS BAR STYLES
	// ==========================================================================

	StatusBar      lipgloss.Style
	StatusBarWide  lipgloss.Style
	ModeLocal      lipgloss.Style
	ModeCloud      lipgloss.Style
	ModeHybrid     lipgloss.Style
	GPUActive      lipgloss.Style
	GPUInactive    lipgloss.Style
	ShortcutKey    lipgloss.Style
	ShortcutDesc   lipgloss.Style

	// ==========================================================================
	// COMMAND PALETTE STYLES
	// ==========================================================================

	PaletteOverlay      lipgloss.Style
	PaletteBox          lipgloss.Style
	PaletteItem         lipgloss.Style
	PaletteItemSelected lipgloss.Style
	PaletteCommand      lipgloss.Style
	PaletteDesc         lipgloss.Style

	// ==========================================================================
	// COMPLETION POPUP STYLES
	// ==========================================================================

	CompletionPopup    lipgloss.Style
	CompletionItem     lipgloss.Style
	CompletionSelected lipgloss.Style
	CompletionMatch    lipgloss.Style

	// ==========================================================================
	// SPINNER AND LOADING STYLES
	// ==========================================================================

	Spinner        lipgloss.Style
	ThinkingText   lipgloss.Style
	ThinkingDots   lipgloss.Style
	ThinkingTime   lipgloss.Style
	ThinkingDetail lipgloss.Style

	// ==========================================================================
	// CODE BLOCK STYLES
	// ==========================================================================

	CodeBlock     lipgloss.Style
	CodeLangBadge lipgloss.Style
	CodeCopyBtn   lipgloss.Style
	CodeLineNum   lipgloss.Style

	// ==========================================================================
	// ERROR BOX STYLES
	// ==========================================================================

	ErrorBox        lipgloss.Style
	ErrorTitle      lipgloss.Style
	ErrorMessage    lipgloss.Style
	ErrorSuggestion lipgloss.Style
	ErrorTip        lipgloss.Style

	// ==========================================================================
	// PERMISSION PROMPT STYLES
	// ==========================================================================

	PermissionBox          lipgloss.Style
	PermissionTitle        lipgloss.Style
	PermissionCommand      lipgloss.Style
	PermissionButton       lipgloss.Style
	PermissionButtonActive lipgloss.Style

	// ==========================================================================
	// SESSION LIST STYLES
	// ==========================================================================

	SessionList         lipgloss.Style
	SessionItem         lipgloss.Style
	SessionItemSelected lipgloss.Style
	SessionID           lipgloss.Style
	SessionTitle        lipgloss.Style
	SessionMeta         lipgloss.Style

	// ==========================================================================
	// WELCOME SCREEN STYLES
	// ==========================================================================

	WelcomeBox      lipgloss.Style
	WelcomeLogo     lipgloss.Style
	WelcomeVersion  lipgloss.Style
	WelcomeInfo     lipgloss.Style
	WelcomeKey      lipgloss.Style
	WelcomePressKey lipgloss.Style

	// ==========================================================================
	// CONTEXT PREVIEW STYLES
	// ==========================================================================

	ContextPreview lipgloss.Style
	ContextHeader  lipgloss.Style
	ContextContent lipgloss.Style
	MentionText    lipgloss.Style

	// ==========================================================================
	// STATISTICS STYLES
	// ==========================================================================

	StatsBar   lipgloss.Style
	StatsLabel lipgloss.Style
	StatsValue lipgloss.Style

	// ==========================================================================
	// ACCESSIBILITY: Status indicator styles with shapes and high contrast
	// ==========================================================================

	// SuccessStyle - Used for success states with checkmark indicator
	SuccessStyle lipgloss.Style
	// ErrorStyle - Used for error states with X mark indicator
	ErrorStyle lipgloss.Style
	// WarningStyle - Used for warning states with warning triangle indicator
	WarningStyle lipgloss.Style
	// InfoStyle - Used for info states with info circle indicator
	InfoStyle lipgloss.Style
	// LinkStyle - Used for links with underline for visual distinction
	LinkStyle lipgloss.Style
}

// NewTheme creates a new theme with all styles configured.
func NewTheme() *Theme {
	// Detect terminal capabilities
	colorProfile := termenv.ColorProfile()
	hasTrueColor := colorProfile == termenv.TrueColor
	isDark := termenv.HasDarkBackground()

	t := &Theme{
		IsDark:       isDark,
		HasTrueColor: hasTrueColor,
		ColorProfile: colorProfile,
	}

	t.initStyles()
	return t
}

// initStyles initializes all the lip gloss styles.
func (t *Theme) initStyles() {
	// App container
	t.App = lipgloss.NewStyle()
	t.Container = lipgloss.NewStyle().Padding(0, 1)

	// Header
	t.Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(Cyan).
		Background(SurfaceDim).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Purple).
		Padding(0, 2).
		Align(lipgloss.Center)

	t.HeaderTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(Purple)

	t.HeaderSubtitle = lipgloss.NewStyle().
		Foreground(TextSecondary).
		Italic(true)

	t.HeaderBrand = lipgloss.NewStyle().
		Bold(true).
		Foreground(Cyan)

	// Message bubbles
	t.UserBubble = lipgloss.NewStyle().
		Foreground(UserBubbleFg).
		Background(UserBubbleBg).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(UserBubbleBorder).
		Padding(0, 2).
		MarginLeft(4)

	t.AssistantBubble = lipgloss.NewStyle().
		Foreground(AssistantBubbleFg).
		Background(AssistantBubbleBg).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(AssistantBubbleBorder).
		Padding(0, 2).
		MarginRight(4)

	t.SystemBubble = lipgloss.NewStyle().
		Foreground(SystemBubbleFg).
		Background(SystemBubbleBg).
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(SystemBubbleBorder).
		Padding(0, 2).
		Align(lipgloss.Center)

	t.ToolSuccess = lipgloss.NewStyle().
		Foreground(ToolSuccessFg).
		Background(ToolSuccessBg).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(Emerald).
		BorderLeft(true).
		PaddingLeft(2)

	t.ToolError = lipgloss.NewStyle().
		Foreground(ToolErrorFg).
		Background(ToolErrorBg).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(Rose).
		BorderLeft(true).
		PaddingLeft(2)

	// Input area
	t.InputContainer = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderBottom(true).
		BorderForeground(Overlay).
		Padding(0, 1)

	t.InputPrompt = lipgloss.NewStyle().
		Foreground(Cyan).
		Bold(true)

	t.InputText = lipgloss.NewStyle().
		Foreground(TextPrimary)

	t.InputPlaceholder = lipgloss.NewStyle().
		Foreground(TextMuted).
		Italic(true)

	t.CharCount = lipgloss.NewStyle().
		Foreground(TextMuted).
		Align(lipgloss.Right)

	t.CharCountWarning = lipgloss.NewStyle().
		Foreground(Amber).
		Align(lipgloss.Right)

	t.CharCountDanger = lipgloss.NewStyle().
		Foreground(Rose).
		Align(lipgloss.Right)

	// Status bar
	t.StatusBar = lipgloss.NewStyle().
		Background(SurfaceDim).
		Foreground(TextSecondary).
		Padding(0, 1)

	t.StatusBarWide = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Overlay).
		Padding(0, 2)

	t.ModeLocal = lipgloss.NewStyle().
		Foreground(Emerald).
		Bold(true)

	t.ModeCloud = lipgloss.NewStyle().
		Foreground(Amber).
		Bold(true)

	t.ModeHybrid = lipgloss.NewStyle().
		Foreground(Purple).
		Bold(true)

	t.GPUActive = lipgloss.NewStyle().
		Foreground(Emerald)

	t.GPUInactive = lipgloss.NewStyle().
		Foreground(TextMuted)

	t.ShortcutKey = lipgloss.NewStyle().
		Foreground(Cyan).
		Bold(true)

	t.ShortcutDesc = lipgloss.NewStyle().
		Foreground(TextMuted)

	// Command palette
	t.PaletteOverlay = lipgloss.NewStyle().
		Background(Overlay)

	t.PaletteBox = lipgloss.NewStyle().
		Background(Surface).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Purple).
		Padding(1, 2)

	t.PaletteItem = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Padding(0, 1)

	t.PaletteItemSelected = lipgloss.NewStyle().
		Background(Purple).
		Foreground(TextInverse).
		Bold(true).
		Padding(0, 1)

	t.PaletteCommand = lipgloss.NewStyle().
		Foreground(Cyan).
		Width(12)

	t.PaletteDesc = lipgloss.NewStyle().
		Foreground(TextMuted)

	// Completion popup
	t.CompletionPopup = lipgloss.NewStyle().
		Background(Surface).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Overlay).
		Padding(0, 1)

	t.CompletionItem = lipgloss.NewStyle().
		Foreground(TextPrimary)

	t.CompletionSelected = lipgloss.NewStyle().
		Background(Purple).
		Foreground(TextInverse).
		Bold(true)

	t.CompletionMatch = lipgloss.NewStyle().
		Foreground(Cyan).
		Bold(true)

	// Spinner and loading
	t.Spinner = lipgloss.NewStyle().
		Foreground(Purple)

	t.ThinkingText = lipgloss.NewStyle().
		Foreground(TextSecondary)

	t.ThinkingDots = lipgloss.NewStyle().
		Foreground(Purple)

	t.ThinkingTime = lipgloss.NewStyle().
		Foreground(TextMuted)

	t.ThinkingDetail = lipgloss.NewStyle().
		Foreground(TextMuted).
		PaddingLeft(2)

	// Code blocks
	t.CodeBlock = lipgloss.NewStyle().
		Background(SurfaceDim).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Overlay).
		Padding(1, 2)

	t.CodeLangBadge = lipgloss.NewStyle().
		Foreground(TextMuted).
		Background(Overlay).
		Padding(0, 1).
		Bold(true)

	t.CodeCopyBtn = lipgloss.NewStyle().
		Foreground(Cyan).
		Background(Overlay).
		Padding(0, 1)

	t.CodeLineNum = lipgloss.NewStyle().
		Foreground(TextMuted).
		Width(4).
		Align(lipgloss.Right).
		MarginRight(1)

	// Error boxes
	t.ErrorBox = lipgloss.NewStyle().
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(Rose).
		Background(RoseDeep).
		Padding(1, 2)

	t.ErrorTitle = lipgloss.NewStyle().
		Foreground(Rose).
		Bold(true)

	t.ErrorMessage = lipgloss.NewStyle().
		Foreground(TextPrimary)

	t.ErrorSuggestion = lipgloss.NewStyle().
		Foreground(TextSecondary).
		PaddingLeft(2)

	t.ErrorTip = lipgloss.NewStyle().
		Foreground(Cyan).
		Italic(true)

	// Permission prompts
	t.PermissionBox = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Amber).
		Background(Surface).
		Padding(1, 2)

	t.PermissionTitle = lipgloss.NewStyle().
		Foreground(Amber).
		Bold(true)

	t.PermissionCommand = lipgloss.NewStyle().
		Background(SurfaceDim).
		Foreground(TextPrimary).
		Padding(0, 1)

	t.PermissionButton = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Background(Overlay).
		Padding(0, 2).
		MarginRight(1)

	t.PermissionButtonActive = lipgloss.NewStyle().
		Foreground(TextInverse).
		Background(Purple).
		Bold(true).
		Padding(0, 2).
		MarginRight(1)

	// Session list
	t.SessionList = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Purple).
		Padding(1, 2)

	t.SessionItem = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Padding(0, 1)

	t.SessionItemSelected = lipgloss.NewStyle().
		Background(Purple).
		Foreground(TextInverse).
		Bold(true).
		Padding(0, 1)

	t.SessionID = lipgloss.NewStyle().
		Foreground(TextMuted).
		Width(12)

	t.SessionTitle = lipgloss.NewStyle().
		Foreground(Cyan).
		Bold(true)

	t.SessionMeta = lipgloss.NewStyle().
		Foreground(TextMuted).
		Italic(true)

	// Welcome screen
	t.WelcomeBox = lipgloss.NewStyle().
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(Purple).
		Padding(2, 4).
		Align(lipgloss.Center)

	t.WelcomeLogo = lipgloss.NewStyle().
		Foreground(Cyan).
		Bold(true)

	t.WelcomeVersion = lipgloss.NewStyle().
		Foreground(TextMuted).
		Italic(true)

	t.WelcomeInfo = lipgloss.NewStyle().
		Foreground(TextSecondary)

	t.WelcomeKey = lipgloss.NewStyle().
		Foreground(Cyan).
		Bold(true)

	t.WelcomePressKey = lipgloss.NewStyle().
		Foreground(Purple).
		Blink(true)

	// Context preview
	t.ContextPreview = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Overlay).
		Padding(0, 1)

	t.ContextHeader = lipgloss.NewStyle().
		Foreground(TextSecondary).
		Bold(true)

	t.ContextContent = lipgloss.NewStyle().
		Foreground(TextMuted)

	t.MentionText = lipgloss.NewStyle().
		Foreground(Cyan).
		Bold(true)

	// Statistics
	t.StatsBar = lipgloss.NewStyle().
		Foreground(TextMuted).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(Overlay).
		Padding(0, 1)

	t.StatsLabel = lipgloss.NewStyle().
		Foreground(TextMuted)

	t.StatsValue = lipgloss.NewStyle().
		Foreground(TextSecondary).
		Bold(true)

	// ==========================================================================
	// ACCESSIBILITY: Status indicator styles with shapes and high contrast
	// ==========================================================================

	// SuccessStyle - High contrast green with bold for colorblind accessibility
	// ACCESSIBILITY: Use with StatusIndicators.Success symbol
	t.SuccessStyle = lipgloss.NewStyle().
		Foreground(SuccessHighContrast).
		Bold(true)

	// ErrorStyle - High contrast red with bold for colorblind accessibility
	// ACCESSIBILITY: Use with StatusIndicators.Error symbol
	t.ErrorStyle = lipgloss.NewStyle().
		Foreground(ErrorHighContrast).
		Bold(true)

	// WarningStyle - High contrast amber with bold for colorblind accessibility
	// ACCESSIBILITY: Use with StatusIndicators.Warning symbol
	t.WarningStyle = lipgloss.NewStyle().
		Foreground(WarningHighContrast).
		Bold(true)

	// InfoStyle - High contrast blue with bold for colorblind accessibility
	// ACCESSIBILITY: Use with StatusIndicators.Info symbol
	t.InfoStyle = lipgloss.NewStyle().
		Foreground(InfoHighContrast).
		Bold(true)

	// LinkStyle - Blue with underline for visual distinction beyond color
	// ACCESSIBILITY: Underline provides non-color visual cue for links
	t.LinkStyle = lipgloss.NewStyle().
		Foreground(LinkColor).
		Underline(true)
}

// SetSize updates the theme dimensions for responsive layouts.
func (t *Theme) SetSize(width, height int) {
	t.Width = width
	t.Height = height
}

// GetLayoutMode returns the current layout mode based on width.
func (t *Theme) GetLayoutMode() LayoutMode {
	if t.Width < 60 {
		return LayoutNarrow
	}
	if t.Width < 100 {
		return LayoutMedium
	}
	return LayoutWide
}

// LayoutMode represents the current responsive layout mode.
type LayoutMode int

const (
	LayoutNarrow LayoutMode = iota // < 60 columns
	LayoutMedium                   // 60-100 columns
	LayoutWide                     // > 100 columns
)
