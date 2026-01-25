// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package chat provides the chat view component for the TUI.
package chat

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/ui/components"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// COMPLETION POPUP RENDERING
// =============================================================================

// renderCompletionPopup renders the tab completion popup above the input.
func (m Model) renderCompletionPopup() string {
	if !m.showCompletions || m.completionState == nil || !m.completionState.Visible {
		return ""
	}

	completions := m.completionState.Completions
	if len(completions) == 0 {
		return ""
	}

	// Create completion popup component
	popup := components.NewCompletionPopup(m.theme)
	popup.SetWidth(minInt(60, m.width-4))
	popup.SetMaxVisible(8)
	popup.SetCompletions(completions)
	popup.SetSelected(m.completionState.Selected)

	// Render the popup
	popupView := popup.View()

	// Position the popup (it appears above the input)
	return popupView
}

// renderCompletionHint renders a subtle hint about available completions.
// This appears when there are completions but the popup is not shown.
func (m Model) renderCompletionHint() string {
	if m.showCompletions || m.completionState == nil {
		return ""
	}

	// Don't show hint if no completions
	if !m.completionState.Visible || len(m.completionState.Completions) == 0 {
		return ""
	}

	hintStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Italic(true)

	count := len(m.completionState.Completions)
	if count == 1 {
		return hintStyle.Render("Press Tab to complete")
	}

	return hintStyle.Render("Press Tab for " + formatInt(count) + " completions")
}

// renderInputWithCompletion renders the input area with completion popup overlay.
func (m Model) renderInputWithCompletion() string {
	width := m.width
	if width <= 0 {
		width = 80
	}

	// Render the base input area
	baseInput := m.renderInput()

	// If no completions to show, return base input
	if !m.showCompletions || m.completionState == nil || !m.completionState.Visible {
		return baseInput
	}

	// Render completion popup
	popup := m.renderCompletionPopup()
	if popup == "" {
		return baseInput
	}

	// Position popup above the input
	// We'll join them vertically with the popup on top
	// The popup should appear to "float" above the input
	combined := lipgloss.JoinVertical(
		lipgloss.Left,
		popup,
		baseInput,
	)

	return combined
}

// minInt returns the minimum of two integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
