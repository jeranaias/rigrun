// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides the visual UI components for rigrun TUI.
package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jeranaias/rigrun-tui/internal/commands"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// COMPLETION POPUP COMPONENT
// =============================================================================

// CompletionPopup displays a popup with completion suggestions.
type CompletionPopup struct {
	completions []commands.Completion
	selected    int
	maxVisible  int
	showPreview bool
	width       int
	theme       *styles.Theme
}

// NewCompletionPopup creates a new completion popup.
func NewCompletionPopup(theme *styles.Theme) *CompletionPopup {
	return &CompletionPopup{
		completions: nil,
		selected:    0,
		maxVisible:  8, // Show up to 8 completions at once
		showPreview: true,
		width:       50,
		theme:       theme,
	}
}

// SetCompletions sets the completions to display.
func (c *CompletionPopup) SetCompletions(completions []commands.Completion) {
	c.completions = completions
	c.selected = 0
}

// GetCompletions returns the current completions.
func (c *CompletionPopup) GetCompletions() []commands.Completion {
	return c.completions
}

// SetSelected sets the selected index.
func (c *CompletionPopup) SetSelected(index int) {
	if index < 0 || index >= len(c.completions) {
		return
	}
	c.selected = index
}

// GetSelected returns the selected index.
func (c *CompletionPopup) GetSelected() int {
	return c.selected
}

// Next selects the next completion.
func (c *CompletionPopup) Next() {
	if len(c.completions) == 0 {
		return
	}
	c.selected = (c.selected + 1) % len(c.completions)
}

// Prev selects the previous completion.
func (c *CompletionPopup) Prev() {
	if len(c.completions) == 0 {
		return
	}
	c.selected--
	if c.selected < 0 {
		c.selected = len(c.completions) - 1
	}
}

// GetSelectedCompletion returns the currently selected completion, or nil.
func (c *CompletionPopup) GetSelectedCompletion() *commands.Completion {
	if c.selected < 0 || c.selected >= len(c.completions) {
		return nil
	}
	return &c.completions[c.selected]
}

// HasCompletions returns true if there are completions to show.
func (c *CompletionPopup) HasCompletions() bool {
	return len(c.completions) > 0
}

// Clear clears all completions.
func (c *CompletionPopup) Clear() {
	c.completions = nil
	c.selected = 0
}

// SetWidth sets the popup width.
func (c *CompletionPopup) SetWidth(width int) {
	c.width = width
}

// SetMaxVisible sets the maximum number of visible completions.
func (c *CompletionPopup) SetMaxVisible(max int) {
	c.maxVisible = max
}

// SetShowPreview sets whether to show preview/usage information.
func (c *CompletionPopup) SetShowPreview(show bool) {
	c.showPreview = show
}

// View renders the completion popup.
func (c *CompletionPopup) View() string {
	if len(c.completions) == 0 {
		return ""
	}

	// Calculate visible range (scrolling window)
	start := 0
	end := len(c.completions)

	if len(c.completions) > c.maxVisible {
		// Center the selected item in the window
		start = c.selected - c.maxVisible/2
		if start < 0 {
			start = 0
		}
		end = start + c.maxVisible
		if end > len(c.completions) {
			end = len(c.completions)
			start = end - c.maxVisible
			if start < 0 {
				start = 0
			}
		}
	}

	// Build completion items
	var items []string
	for i := start; i < end; i++ {
		items = append(items, c.renderCompletionItem(c.completions[i], i == c.selected))
	}

	// Build the popup box
	content := strings.Join(items, "\n")

	// Add preview/usage if enabled and there's a selected item
	if c.showPreview && c.selected >= 0 && c.selected < len(c.completions) {
		preview := c.renderPreview(c.completions[c.selected])
		if preview != "" {
			content += "\n" + c.renderDivider() + "\n" + preview
		}
	}

	// Box style
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.Cyan).
		Padding(0, 1).
		Width(c.width).
		MaxWidth(c.width)

	return boxStyle.Render(content)
}

// renderCompletionItem renders a single completion item.
func (c *CompletionPopup) renderCompletionItem(comp commands.Completion, isSelected bool) string {
	// Value (left aligned)
	valueStyle := lipgloss.NewStyle().
		Width(20).
		Foreground(styles.TextPrimary)

	// Description (right aligned)
	descStyle := lipgloss.NewStyle().
		Width(c.width - 24). // Account for padding and value width
		Foreground(styles.TextSecondary)

	if isSelected {
		// Highlight selected item
		valueStyle = valueStyle.
			Background(styles.Cyan).
			Foreground(styles.Surface).
			Bold(true)
		descStyle = descStyle.
			Foreground(styles.TextPrimary)
	}

	value := comp.Display
	if value == "" {
		value = comp.Value
	}

	// Truncate if needed
	valueRunes := []rune(value)
	if len(valueRunes) > 20 {
		value = string(valueRunes[:17]) + "..."
	}

	// Truncate description
	desc := comp.Description
	descRunes := []rune(desc)
	maxDescLen := c.width - 24
	if len(descRunes) > maxDescLen {
		desc = string(descRunes[:maxDescLen-3]) + "..."
	}

	// Indicator for selected item (ASCII)
	indicator := " "
	if isSelected {
		indicator = ">"
	}

	indicatorStyle := lipgloss.NewStyle().
		Width(2).
		Foreground(styles.Cyan)

	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		indicatorStyle.Render(indicator),
		valueStyle.Render(value),
		descStyle.Render(desc),
	)
}

// renderPreview renders preview/usage information for a completion.
func (c *CompletionPopup) renderPreview(comp commands.Completion) string {
	// For now, return empty - can be extended to show usage examples
	// based on the completion type
	return ""
}

// renderDivider renders a divider line.
func (c *CompletionPopup) renderDivider() string {
	dividerStyle := lipgloss.NewStyle().
		Foreground(styles.Overlay)

	return dividerStyle.Render(strings.Repeat("-", c.width-2))
}

// ViewCompact renders a compact single-line completion indicator.
// Shows "Tab: N completions" or "Tab: complete X" for single completion.
func (c *CompletionPopup) ViewCompact() string {
	if len(c.completions) == 0 {
		return ""
	}

	style := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Italic(true)

	if len(c.completions) == 1 {
		value := c.completions[0].Display
		if value == "" {
			value = c.completions[0].Value
		}
		return style.Render("Tab: complete \"" + value + "\"")
	}

	return style.Render("Tab: " + formatInt(len(c.completions)) + " completions")
}

// ViewInline renders completions inline (suitable for bottom of input).
func (c *CompletionPopup) ViewInline() string {
	if len(c.completions) == 0 {
		return ""
	}

	// Show first few completions inline
	maxInline := 3
	if len(c.completions) < maxInline {
		maxInline = len(c.completions)
	}

	var parts []string
	for i := 0; i < maxInline; i++ {
		comp := c.completions[i]
		value := comp.Display
		if value == "" {
			value = comp.Value
		}

		style := lipgloss.NewStyle().
			Foreground(styles.TextSecondary)

		if i == c.selected {
			style = style.
				Foreground(styles.Cyan).
				Bold(true)
		}

		parts = append(parts, style.Render(value))
	}

	if len(c.completions) > maxInline {
		moreStyle := lipgloss.NewStyle().
			Foreground(styles.TextMuted)
		parts = append(parts, moreStyle.Render("..."+formatInt(len(c.completions)-maxInline)+" more"))
	}

	return strings.Join(parts, " | ")
}

// formatInt converts an integer to string without fmt package.
// Handles all edge cases including MinInt64.
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}

	var digits []byte
	negative := n < 0
	if negative {
		// Handle overflow for minimum int value
		if n == -n {
			// This is the minimum int value (e.g., -9223372036854775808 for int64)
			// We can't negate it directly, so we handle it specially
			n = n / 10
			remainder := -(n*10 - (-n))
			digits = append([]byte{byte('0' + remainder)}, digits...)
		}
		n = -n
	}

	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}
