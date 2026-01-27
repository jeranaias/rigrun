// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
//
// consent_banner.go implements the DoD System Use Notification consent banner
// for IL5 compliance with NIST 800-53 AC-8 (System Use Notification).
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/cli"
)

// =============================================================================
// CONSENT BANNER COMPONENT (IL5 AC-8 Compliance)
// =============================================================================

// Minimum terminal size constants
const (
	minTerminalWidth  = 40
	minTerminalHeight = 12
	footerHeight      = 4 // Lines reserved for the fixed footer
)

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

		// Update viewport dimensions
		c.viewport.Width = width
		c.viewport.Height = height

		// Rebuild viewport content for new size
		c.rebuildViewportContent()
	}

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
			c.width = msg.Width
			c.height = msg.Height
			// Rebuild viewport content for new size and reset to top
			c.rebuildViewportContent()
		}
		c.ready = true

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

// isTooSmall checks if the terminal is too small to display the consent banner.
func (c ConsentBanner) isTooSmall() bool {
	return c.width < minTerminalWidth || c.height < minTerminalHeight
}

// renderTooSmallMessage renders a message asking the user to resize the terminal.
func (c ConsentBanner) renderTooSmallMessage() string {
	amberBg := lipgloss.Color("#1A1500")
	amberFg := lipgloss.Color("#FFB000")
	redFg := lipgloss.Color("#FF4444")

	warningStyle := lipgloss.NewStyle().
		Foreground(redFg).
		Background(amberBg).
		Bold(true).
		Align(lipgloss.Center)

	messageStyle := lipgloss.NewStyle().
		Foreground(amberFg).
		Background(amberBg).
		Align(lipgloss.Center)

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Background(amberBg).
		Italic(true).
		Align(lipgloss.Center)

	var parts []string
	parts = append(parts, warningStyle.Render("TERMINAL TOO SMALL"))
	parts = append(parts, "")
	parts = append(parts, messageStyle.Render("Please resize your"))
	parts = append(parts, messageStyle.Render("terminal window."))
	parts = append(parts, "")
	parts = append(parts, hintStyle.Render(fmt.Sprintf("Min: %dx%d", minTerminalWidth, minTerminalHeight)))
	parts = append(parts, hintStyle.Render(fmt.Sprintf("Now: %dx%d", c.width, c.height)))

	content := lipgloss.JoinVertical(lipgloss.Center, parts...)

	return lipgloss.Place(
		c.width, c.height,
		lipgloss.Center, lipgloss.Center,
		content,
		lipgloss.WithWhitespaceBackground(amberBg),
	)
}

// renderFooter renders the fixed footer with controls.
func (c ConsentBanner) renderFooter() string {
	amberBg := lipgloss.Color("#1A1500")
	amberFg := lipgloss.Color("#FFB000")

	separatorStyle := lipgloss.NewStyle().
		Foreground(amberFg).
		Background(amberBg)

	controlsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(amberBg).
		Bold(true).
		Align(lipgloss.Center).
		Width(c.width)

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Background(amberBg).
		Align(lipgloss.Center).
		Width(c.width)

	// Build separator line
	separatorWidth := c.width - 4
	if separatorWidth < 10 {
		separatorWidth = 10
	}
	separator := strings.Repeat("─", separatorWidth)

	var parts []string
	parts = append(parts, separatorStyle.Render(separator))

	// Show scroll indicator if scrolling is enabled
	if c.needsScrolling {
		scrollPercent := int(c.viewport.ScrollPercent() * 100)
		scrollInfo := fmt.Sprintf("↑/↓ scroll [%d%%]  •  ENTER/Y accept  •  ESC quit", scrollPercent)
		parts = append(parts, controlsStyle.Render(scrollInfo))
	} else {
		parts = append(parts, controlsStyle.Render("ENTER/Y to accept  •  ESC to quit"))
	}
	parts = append(parts, hintStyle.Render("U.S. Government System - Authorized Use Only"))

	return lipgloss.JoinVertical(lipgloss.Center, parts...)
}

// View renders the consent banner as a full-screen amber/gold display.
// The banner adapts to window size and supports scrolling when content
// exceeds the available height. Controls are displayed in a fixed footer
// that is always visible at the bottom of the screen.
func (c ConsentBanner) View() string {
	width := c.width
	if width == 0 {
		width = 80
	}
	height := c.height
	if height == 0 {
		height = 24
	}

	// Check for minimum terminal size
	if c.isTooSmall() {
		return c.renderTooSmallMessage()
	}

	// Amber background for entire screen
	amberBg := lipgloss.Color("#1A1500")

	// Render the fixed footer
	footer := c.renderFooter()
	footerLines := strings.Count(footer, "\n") + 1

	// Calculate available height for content (excluding footer)
	contentHeight := height - footerLines

	// Use viewport for scrolling - it contains the full content
	if c.ready && c.needsScrolling {
		// Viewport handles the scrolling
		viewportContent := c.viewport.View()

		// Pad viewport content to fill the content area
		viewportLines := strings.Count(viewportContent, "\n") + 1
		if viewportLines < contentHeight {
			padding := strings.Repeat("\n", contentHeight-viewportLines)
			viewportContent = viewportContent + padding
		}

		return lipgloss.JoinVertical(lipgloss.Left,
			viewportContent,
			footer,
		)
	}

	// Content fits - center it in the content area (above footer)
	content := c.renderedContent
	if content == "" {
		content = c.buildBannerContent()
	}

	centered := lipgloss.Place(
		width, contentHeight,
		lipgloss.Center, lipgloss.Center,
		content,
		lipgloss.WithWhitespaceBackground(amberBg),
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		centered,
		footer,
	)
}

// updateContent rebuilds the rendered content and updates viewport if needed.
// This should be called when dimensions change.
func (c *ConsentBanner) updateContent() {
	// This method is called to update the internal state
	// The actual rendering happens in View()
	c.contentDirty = false
}

// rebuildViewportContent rebuilds and sets the viewport content for scrolling.
// This must be called from SetSize (pointer receiver) to persist viewport state.
func (c *ConsentBanner) rebuildViewportContent() {
	if c.width == 0 || c.height == 0 {
		return
	}

	// Build the full consent banner content (styled)
	styledContent := c.buildBannerContent()
	c.renderedContent = styledContent

	// Check if scrolling is needed (reserve footerHeight lines for fixed footer)
	contentLines := strings.Count(styledContent, "\n") + 1
	availableHeight := c.height - footerHeight
	c.needsScrolling = contentLines > availableHeight

	if c.needsScrolling {
		// For scrolling, use simple plain text content (no box styling)
		// The viewport doesn't handle complex ANSI styling well
		simpleContent := c.buildSimpleContent()

		// Set viewport dimensions (leave room for fixed footer)
		c.viewport.Width = c.width
		c.viewport.Height = availableHeight

		// Set viewport content with simple styling
		// Note: SetContent() internally resets scroll position to top
		c.viewport.SetContent(simpleContent)
	}

	c.contentDirty = false
}

// buildSimpleContent builds a simple scrollable version of the consent banner.
// Note: Control instructions are displayed in the fixed footer, not here.
func (c *ConsentBanner) buildSimpleContent() string {
	amberFg := lipgloss.Color("#FFB000")
	redFg := lipgloss.Color("#FF4444")

	titleStyle := lipgloss.NewStyle().Foreground(redFg).Bold(true)
	textStyle := lipgloss.NewStyle().Foreground(amberFg)

	var sb strings.Builder

	sb.WriteString(titleStyle.Render("═══════════════════════════════════════════════════════════════"))
	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render("           U.S. GOVERNMENT INFORMATION SYSTEM"))
	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render("        DoD SYSTEM USE NOTIFICATION (AC-8)"))
	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render("═══════════════════════════════════════════════════════════════"))
	sb.WriteString("\n\n")

	// Get the banner text and wrap it
	bannerText := cli.DoDConsentBanner
	lines := strings.Split(bannerText, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			sb.WriteString("\n")
			continue
		}
		if strings.HasPrefix(trimmed, "=") {
			continue // Skip separator lines
		}
		sb.WriteString(textStyle.Render(trimmed))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render("═══════════════════════════════════════════════════════════════"))
	sb.WriteString("\n")

	return sb.String()
}

// buildBannerContent builds the consent banner content string.
func (c *ConsentBanner) buildBannerContent() string {
	width := c.width
	if width == 0 {
		width = 80
	}
	height := c.height
	if height == 0 {
		height = 24
	}

	// Amber/Gold color scheme for consent banner
	amberBg := lipgloss.Color("#1A1500")
	amberFg := lipgloss.Color("#FFB000")
	redBorder := lipgloss.Color("#FF4444")

	// Calculate responsive dimensions
	horizontalMargin := 4
	if width < 50 {
		horizontalMargin = 2
	}
	if width < 30 {
		horizontalMargin = 1
	}

	maxContentWidth := width - (horizontalMargin * 2)
	if maxContentWidth > 76 {
		maxContentWidth = 76
	}
	if maxContentWidth < 30 {
		maxContentWidth = 30
	}

	innerWidth := maxContentWidth - 6
	if innerWidth < 20 {
		innerWidth = 20
	}

	// Get the DoD consent banner text
	bannerText := cli.DoDConsentBanner

	// Clean up the banner text
	lines := strings.Split(bannerText, "\n")
	var contentLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "=") {
			continue
		}
		contentLines = append(contentLines, line)
	}

	wrappedContent := wrapTextLines(contentLines, innerWidth)

	// Styles
	// Note: Control instructions are displayed in the fixed footer, not in this content
	titleStyle := lipgloss.NewStyle().
		Foreground(redBorder).
		Background(amberBg).
		Bold(true).
		Align(lipgloss.Center).
		Width(innerWidth)

	contentStyle := lipgloss.NewStyle().
		Foreground(amberFg).
		Background(amberBg).
		Align(lipgloss.Left).
		Width(innerWidth)

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

	// Build content (controls are in fixed footer)
	var parts []string
	parts = append(parts, "")
	parts = append(parts, titleStyle.Render("U.S. GOVERNMENT INFORMATION SYSTEM"))
	parts = append(parts, titleStyle.Render("DoD SYSTEM USE NOTIFICATION (AC-8)"))
	parts = append(parts, "")
	parts = append(parts, separatorStyle.Render(separator))
	parts = append(parts, "")
	parts = append(parts, contentStyle.Render(wrappedContent))
	parts = append(parts, "")
	parts = append(parts, separatorStyle.Render(separator))
	parts = append(parts, "")

	content := lipgloss.JoinVertical(lipgloss.Center, parts...)

	// Create box
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

	// Center horizontally
	centered := lipgloss.Place(
		width, 0,
		lipgloss.Center, lipgloss.Top,
		box,
		lipgloss.WithWhitespaceBackground(amberBg),
	)

	return centered
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
