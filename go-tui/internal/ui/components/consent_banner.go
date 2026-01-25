// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
//
// consent_banner.go implements the DoD System Use Notification consent banner
// for IL5 compliance with NIST 800-53 AC-8 (System Use Notification).
package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/cli"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// CONSENT BANNER COMPONENT (IL5 AC-8 Compliance)
// =============================================================================

// ConsentBanner is the DoD System Use Notification consent banner component.
// This component displays the standard USG warning text and requires explicit
// acknowledgment before proceeding to the TUI.
type ConsentBanner struct {
	// Dimensions
	width  int
	height int

	// Viewport for scrollable content when banner is larger than screen
	viewport viewport.Model
	ready    bool

	// Track if content needs scrolling and cache rendered content
	needsScrolling  bool
	renderedContent string
	contentDirty    bool

	// State
	acknowledged bool
}

// NewConsentBanner creates a new consent banner component.
func NewConsentBanner() ConsentBanner {
	vp := viewport.New(80, 24)
	vp.Style = lipgloss.NewStyle()

	return ConsentBanner{
		acknowledged: false,
		viewport:     vp,
		ready:        false,
		contentDirty: true,
	}
}

// SetSize updates the dimensions and recalculates the viewport.
func (c *ConsentBanner) SetSize(width, height int) {
	// Only mark dirty if size actually changed
	if c.width != width || c.height != height {
		c.width = width
		c.height = height
		c.contentDirty = true
	}

	// Update viewport dimensions
	c.viewport.Width = width
	c.viewport.Height = height
	c.ready = true
}

// IsAcknowledged returns whether the user has acknowledged the consent banner.
func (c *ConsentBanner) IsAcknowledged() bool {
	return c.acknowledged
}

// Acknowledge marks the consent as acknowledged.
func (c *ConsentBanner) Acknowledge() {
	c.acknowledged = true
}

// =============================================================================
// BUBBLE TEA MESSAGES
// =============================================================================

// ConsentAcknowledgedMsg signals that the user acknowledged the consent banner.
type ConsentAcknowledgedMsg struct{}

// ConsentDeclinedMsg signals that the user declined consent (exited).
type ConsentDeclinedMsg struct{}

// =============================================================================
// BUBBLE TEA INTERFACE
// =============================================================================

// Init initializes the consent banner.
func (c ConsentBanner) Init() tea.Cmd {
	return nil
}

// Update handles messages including window resize and scroll keys.
func (c ConsentBanner) Update(msg tea.Msg) (ConsentBanner, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Mark dirty and update size
		if c.width != msg.Width || c.height != msg.Height {
			c.contentDirty = true
		}
		c.width = msg.Width
		c.height = msg.Height
		c.viewport.Width = msg.Width
		c.viewport.Height = msg.Height
		c.ready = true

		// Recalculate if scrolling is needed based on new dimensions
		c.needsScrolling = c.calculateNeedsScrolling()

	case tea.KeyMsg:
		// Always handle scrolling keys - they're harmless when not needed
		switch msg.String() {
		case "up", "k":
			c.viewport.LineUp(1)
		case "down", "j":
			c.viewport.LineDown(1)
		case "pgup":
			c.viewport.ViewUp()
		case "pgdown", "pgdn":
			c.viewport.ViewDown()
		case "home":
			c.viewport.GotoTop()
		case "end":
			c.viewport.GotoBottom()
		}

	case tea.MouseMsg:
		// Handle mouse wheel scrolling
		switch msg.Type {
		case tea.MouseWheelUp:
			c.viewport.LineUp(3)
		case tea.MouseWheelDown:
			c.viewport.LineDown(3)
		}
	}

	// Let viewport handle any other messages
	c.viewport, cmd = c.viewport.Update(msg)

	return c, cmd
}

// calculateNeedsScrolling estimates if the content will need scrolling
// based on current dimensions. This is an approximation used to enable
// scrolling controls.
func (c *ConsentBanner) calculateNeedsScrolling() bool {
	// The banner content is approximately 35-40 lines of text when wrapped
	// to standard width. Add some buffer for the box border, title, etc.
	estimatedContentHeight := 45 // Conservative estimate for full banner

	return estimatedContentHeight > c.height
}

// View renders the consent banner as a full-screen amber/gold display.
// The banner adapts to window size and supports scrolling when content
// exceeds the available height.
func (c ConsentBanner) View() string {
	width := c.width
	if width == 0 {
		width = 80
	}
	height := c.height
	if height == 0 {
		height = 24
	}

	// ==========================================================================
	// STYLING: Amber/Gold background for DoD consent banner
	// ==========================================================================

	// Amber/Gold color scheme for consent banner
	amberBg := lipgloss.Color("#1A1500")   // Dark amber background
	amberFg := lipgloss.Color("#FFB000")   // Bright amber/gold text
	redBorder := lipgloss.Color("#FF4444") // Red border for attention

	// ==========================================================================
	// CALCULATE RESPONSIVE DIMENSIONS
	// ==========================================================================

	// Calculate content box dimensions with proper responsive sizing
	// Minimum: 40 chars, Maximum: 76 chars, with 4 char margin on each side
	horizontalMargin := 4
	if width < 50 {
		horizontalMargin = 2 // Reduce margin on very small screens
	}
	if width < 30 {
		horizontalMargin = 1 // Minimal margin on tiny screens
	}

	maxContentWidth := width - (horizontalMargin * 2)
	if maxContentWidth > 76 {
		maxContentWidth = 76 // Cap at reasonable maximum
	}
	if maxContentWidth < 30 {
		maxContentWidth = 30 // Minimum readable width
	}

	// Inner content width (accounting for box padding and border)
	innerWidth := maxContentWidth - 6 // 2 for border + 4 for padding (2 each side)
	if innerWidth < 20 {
		innerWidth = 20
	}

	// ==========================================================================
	// BUILD CONSENT BANNER TEXT
	// ==========================================================================

	// Get the DoD consent banner text from the CLI package
	bannerText := cli.DoDConsentBanner

	// Clean up the banner text - remove the outer frame since we'll add our own
	lines := strings.Split(bannerText, "\n")
	var contentLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip the frame lines (all = or empty)
		if trimmed == "" || strings.HasPrefix(trimmed, "=") {
			continue
		}
		contentLines = append(contentLines, line)
	}

	// Wrap text to fit content width
	wrappedContent := wrapTextLines(contentLines, innerWidth)

	// ==========================================================================
	// RENDER BANNER CONTENT
	// ==========================================================================

	// Title style
	titleStyle := lipgloss.NewStyle().
		Foreground(redBorder).
		Background(amberBg).
		Bold(true).
		Align(lipgloss.Center).
		Width(innerWidth)

	// Content text style
	contentStyle := lipgloss.NewStyle().
		Foreground(amberFg).
		Background(amberBg).
		Align(lipgloss.Left).
		Width(innerWidth)

	// Acknowledgment prompt style
	promptStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(amberBg).
		Bold(true).
		Align(lipgloss.Center).
		Width(innerWidth)

	// Hint style for ESC instruction
	hintStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Background(amberBg).
		Italic(true).
		Align(lipgloss.Center).
		Width(innerWidth)

	// Scroll hint style
	scrollHintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Background(amberBg).
		Italic(true).
		Align(lipgloss.Center).
		Width(innerWidth)

	// Build content sections
	var parts []string

	// Title
	parts = append(parts, "")
	parts = append(parts, titleStyle.Render("U.S. GOVERNMENT INFORMATION SYSTEM"))
	parts = append(parts, titleStyle.Render("DoD SYSTEM USE NOTIFICATION (AC-8)"))
	parts = append(parts, "")

	// Separator - use responsive width
	separatorWidth := innerWidth - 2
	if separatorWidth < 10 {
		separatorWidth = 10
	}
	separator := strings.Repeat("-", separatorWidth)
	separatorStyle := lipgloss.NewStyle().
		Foreground(amberFg).
		Background(amberBg).
		Align(lipgloss.Center).
		Width(innerWidth)
	parts = append(parts, separatorStyle.Render(separator))
	parts = append(parts, "")

	// Main content
	parts = append(parts, contentStyle.Render(wrappedContent))
	parts = append(parts, "")

	// Separator
	parts = append(parts, separatorStyle.Render(separator))
	parts = append(parts, "")

	// Acknowledgment prompt
	parts = append(parts, promptStyle.Render("Press ENTER or Y to acknowledge and continue"))
	parts = append(parts, "")
	parts = append(parts, hintStyle.Render("Press ESC to exit without acknowledging"))
	parts = append(parts, "")

	content := lipgloss.JoinVertical(lipgloss.Center, parts...)

	// ==========================================================================
	// CREATE FRAMED BOX
	// ==========================================================================

	// Box style with red border on amber background
	// Use responsive padding based on available width
	horizontalPadding := 2
	if width < 50 {
		horizontalPadding = 1
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(redBorder).
		Background(amberBg).
		Padding(1, horizontalPadding).
		Width(maxContentWidth).
		Align(lipgloss.Center)

	box := boxStyle.Render(content)

	// ==========================================================================
	// HANDLE SCROLLING FOR SMALL WINDOWS
	// ==========================================================================

	// Count lines in the rendered box
	boxLines := strings.Split(box, "\n")
	boxHeight := len(boxLines)

	// Check if content needs scrolling (with some margin for safety)
	needsScrolling := boxHeight > (height - 2)

	// Create amber background for the entire screen
	bgStyle := lipgloss.NewStyle().
		Background(amberBg).
		Width(width).
		Height(height)

	if needsScrolling {
		// Add scroll indicator at the bottom
		scrollIndicator := scrollHintStyle.Render("[Use Up/Down or PgUp/PgDn to scroll]")
		parts = append(parts, scrollIndicator)
		content = lipgloss.JoinVertical(lipgloss.Center, parts...)
		box = boxStyle.Render(content)

		// Center the box horizontally within the viewport content
		centeredBox := lipgloss.Place(
			width, boxHeight+2, // Add some extra height for the scroll indicator
			lipgloss.Center, lipgloss.Top,
			box,
			lipgloss.WithWhitespaceBackground(amberBg),
		)

		// Set viewport content - the viewport handles scrolling internally
		c.viewport.SetContent(centeredBox)

		return bgStyle.Render(c.viewport.View())
	}

	// ==========================================================================
	// CENTER ON SCREEN (when content fits)
	// ==========================================================================

	// Center the box vertically and horizontally
	centered := lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		box,
		lipgloss.WithWhitespaceBackground(amberBg),
	)

	return bgStyle.Render(centered)
}

// updateContent rebuilds the rendered content and updates viewport if needed.
// This should be called when dimensions change.
func (c *ConsentBanner) updateContent() {
	// This method is called to update the internal state
	// The actual rendering happens in View()
	c.contentDirty = false
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// wrapTextLines wraps a slice of text lines to fit within maxWidth.
func wrapTextLines(lines []string, maxWidth int) string {
	var result []string

	for _, line := range lines {
		// If line fits, add it directly
		if len(line) <= maxWidth {
			result = append(result, line)
			continue
		}

		// Wrap long lines
		wrapped := wrapText(line, maxWidth)
		result = append(result, wrapped...)
	}

	return strings.Join(result, "\n")
}

// wrapText wraps a single line of text to fit within maxWidth.
func wrapText(text string, maxWidth int) []string {
	if len(text) <= maxWidth {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	var currentLine strings.Builder

	for _, word := range words {
		if currentLine.Len() == 0 {
			currentLine.WriteString(word)
		} else if currentLine.Len()+1+len(word) <= maxWidth {
			currentLine.WriteString(" ")
			currentLine.WriteString(word)
		} else {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		}
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}

// =============================================================================
// CONSENT CHECK HELPER (for use in main.go)
// =============================================================================

// CheckConsentStatus checks if consent is required and valid.
// Returns (requiresConsent bool, message string).
// If requiresConsent is true, the consent banner should be shown.
func CheckConsentStatus() (bool, string) {
	canProceed, msg := cli.CheckConsentRequired()
	return !canProceed, msg
}
