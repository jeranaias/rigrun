// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/tools"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// TOOL RESULT VIEW
// =============================================================================

// ToolResultView displays the result of a tool execution.
type ToolResultView struct {
	// Tool information
	toolName string
	result   tools.Result

	// UI state
	expanded      bool
	maxCollapsed  int // Max lines when collapsed (default: 3)
	maxExpanded   int // Max lines when expanded (default: 50)
	width         int

	// Styles
	theme *styles.Theme
}

// NewToolResultView creates a new tool result view.
func NewToolResultView(theme *styles.Theme) *ToolResultView {
	return &ToolResultView{
		theme:        theme,
		maxCollapsed: 3,
		maxExpanded:  50,
	}
}

// =============================================================================
// TOOL RESULT VIEW METHODS
// =============================================================================

// SetResult sets the tool result to display.
func (v *ToolResultView) SetResult(toolName string, result tools.Result) {
	v.toolName = toolName
	v.result = result
	v.expanded = false
}

// SetWidth sets the display width.
func (v *ToolResultView) SetWidth(width int) {
	v.width = width
}

// Toggle expands or collapses the result.
func (v *ToolResultView) Toggle() {
	v.expanded = !v.expanded
}

// IsExpanded returns whether the result is expanded.
func (v *ToolResultView) IsExpanded() bool {
	return v.expanded
}

// SetExpanded sets the expanded state.
func (v *ToolResultView) SetExpanded(expanded bool) {
	v.expanded = expanded
}

// =============================================================================
// VIEW RENDERING
// =============================================================================

// View renders the tool result.
func (v *ToolResultView) View() string {
	if v.toolName == "" {
		return ""
	}

	if v.expanded {
		return v.renderExpanded()
	}
	return v.renderCollapsed()
}

// renderCollapsed renders the collapsed view.
func (v *ToolResultView) renderCollapsed() string {
	var builder strings.Builder

	// ACCESSIBILITY: Status icon with shape indicator for colorblind users
	var icon string
	var iconStyle lipgloss.Style

	if v.result.Success {
		// ACCESSIBILITY: Checkmark symbol + high contrast green
		icon = styles.StatusIndicators.Success
		iconStyle = lipgloss.NewStyle().Foreground(styles.SuccessHighContrast).Bold(true)
	} else {
		// ACCESSIBILITY: X mark symbol + high contrast red
		icon = styles.StatusIndicators.Error
		iconStyle = lipgloss.NewStyle().Foreground(styles.ErrorHighContrast).Bold(true)
	}

	builder.WriteString(iconStyle.Render(icon))
	builder.WriteString(" ")

	// Tool name
	nameStyle := lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true)

	builder.WriteString(nameStyle.Render(v.toolName))

	// Summary info
	infoStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted)

	summary := v.buildSummary()
	if summary != "" {
		builder.WriteString(infoStyle.Render(" (" + summary + ")"))
	}

	// Expand indicator if there's content
	if v.hasContent() {
		expandStyle := lipgloss.NewStyle().
			Foreground(styles.TextMuted)
		builder.WriteString(expandStyle.Render(" [+]"))
	}

	// Show first few lines of output/error
	content := v.getContentPreview()
	if content != "" {
		builder.WriteString("\n")

		contentStyle := lipgloss.NewStyle().
			Foreground(styles.TextSecondary).
			PaddingLeft(2)

		builder.WriteString(contentStyle.Render(content))
	}

	// ACCESSIBILITY: Apply border style with high contrast colors
	borderColor := styles.SuccessHighContrast
	if !v.result.Success {
		borderColor = styles.ErrorHighContrast
	}

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		BorderLeft(true).
		PaddingLeft(1)

	return boxStyle.Render(builder.String())
}

// renderExpanded renders the expanded view.
func (v *ToolResultView) renderExpanded() string {
	var builder strings.Builder

	// ACCESSIBILITY: Header with tool name and status indicator for colorblind users
	var headerIcon string
	var borderColor lipgloss.AdaptiveColor

	if v.result.Success {
		// ACCESSIBILITY: Checkmark symbol for success
		headerIcon = styles.StatusIndicators.Success
		borderColor = styles.SuccessHighContrast
	} else {
		// ACCESSIBILITY: X mark symbol for error
		headerIcon = styles.StatusIndicators.Error
		borderColor = styles.ErrorHighContrast
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true)

	// ACCESSIBILITY: High contrast colors with bold for colorblind users
	iconStyle := lipgloss.NewStyle().Bold(true)
	if v.result.Success {
		iconStyle = iconStyle.Foreground(styles.SuccessHighContrast)
	} else {
		iconStyle = iconStyle.Foreground(styles.ErrorHighContrast)
	}

	builder.WriteString(iconStyle.Render(headerIcon))
	builder.WriteString(" ")
	builder.WriteString(headerStyle.Render(v.toolName))

	// Collapse indicator
	collapseStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted)
	builder.WriteString(collapseStyle.Render(" [-]"))

	builder.WriteString("\n")

	// Separator
	sepStyle := lipgloss.NewStyle().
		Foreground(styles.Overlay)

	width := v.width - 6
	if width < 20 {
		width = 60
	}

	builder.WriteString(sepStyle.Render(strings.Repeat("-", width)))
	builder.WriteString("\n")

	// Content
	if v.result.Success {
		content := v.result.Output
		lines := strings.Split(content, "\n")

		// Limit lines
		if len(lines) > v.maxExpanded {
			lines = lines[:v.maxExpanded]
			lines = append(lines, "... ("+util.IntToString(len(strings.Split(content, "\n"))-v.maxExpanded)+" more lines)")
		}

		contentStyle := lipgloss.NewStyle().
			Foreground(styles.TextPrimary)

		builder.WriteString(contentStyle.Render(strings.Join(lines, "\n")))
	} else {
		// ACCESSIBILITY: Error message with high contrast red for colorblind users
		errorStyle := lipgloss.NewStyle().
			Foreground(styles.ErrorHighContrast).
			Bold(true)

		builder.WriteString(errorStyle.Render(styles.StatusIndicators.Error + " Error: " + v.result.Error))

		// Show any output even on error
		if v.result.Output != "" {
			builder.WriteString("\n\n")
			builder.WriteString(lipgloss.NewStyle().Foreground(styles.TextSecondary).Render("Output:"))
			builder.WriteString("\n")
			builder.WriteString(lipgloss.NewStyle().Foreground(styles.TextMuted).Render(v.result.Output))
		}
	}

	builder.WriteString("\n")

	// Footer with timing
	if v.result.Duration > 0 {
		builder.WriteString(sepStyle.Render(strings.Repeat("-", width)))
		builder.WriteString("\n")

		footerStyle := lipgloss.NewStyle().
			Foreground(styles.TextMuted).
			Align(lipgloss.Right)

		builder.WriteString(footerStyle.Render(formatToolDuration(v.result.Duration)))
	}

	// Box with colored border
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	if v.width > 0 {
		boxStyle = boxStyle.Width(v.width - 2)
	}

	return boxStyle.Render(builder.String())
}

// =============================================================================
// HELPER METHODS
// =============================================================================

// buildSummary creates a summary string for the result.
func (v *ToolResultView) buildSummary() string {
	var parts []string

	// Lines count
	if v.result.LinesCount > 0 {
		parts = append(parts, util.IntToString(v.result.LinesCount)+" lines")
	}

	// Files matched
	if v.result.FilesMatched > 0 {
		parts = append(parts, util.IntToString(v.result.FilesMatched)+" files")
	}

	// Match count
	if v.result.MatchCount > 0 {
		parts = append(parts, util.IntToString(v.result.MatchCount)+" matches")
	}

	// Bytes read/written
	if v.result.BytesRead > 0 {
		parts = append(parts, formatBytes(v.result.BytesRead))
	} else if v.result.BytesWritten > 0 {
		parts = append(parts, formatBytes(v.result.BytesWritten))
	}

	// Duration
	if v.result.Duration > 0 {
		parts = append(parts, formatToolDuration(v.result.Duration))
	}

	// Truncated indicator
	if v.result.Truncated {
		parts = append(parts, "truncated")
	}

	return strings.Join(parts, ", ")
}

// hasContent returns true if there's content to show.
func (v *ToolResultView) hasContent() bool {
	return v.result.Output != "" || v.result.Error != ""
}

// getContentPreview returns a preview of the content.
func (v *ToolResultView) getContentPreview() string {
	content := v.result.Output
	if !v.result.Success && v.result.Error != "" {
		content = v.result.Error
	}

	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	if len(lines) > v.maxCollapsed {
		lines = lines[:v.maxCollapsed]
		remaining := len(strings.Split(content, "\n")) - v.maxCollapsed
		lines = append(lines, "... ("+util.IntToString(remaining)+" more lines)")
	}

	return strings.Join(lines, "\n")
}

// formatDuration formats a duration for display.
func formatToolDuration(d interface{}) string {
	// Handle time.Duration
	switch v := d.(type) {
	case int64:
		// Nanoseconds
		ms := v / 1000000
		if ms < 1000 {
			return util.IntToString(int(ms)) + "ms"
		}
		return util.IntToString(int(ms/1000)) + "." + util.IntToString(int(ms%1000/100)) + "s"
	}

	return ""
}

// formatBytes formats bytes for display.
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
	)

	if bytes >= MB {
		return util.IntToString(int(bytes/MB)) + "MB"
	}
	if bytes >= KB {
		return util.IntToString(int(bytes/KB)) + "KB"
	}
	return util.IntToString(int(bytes)) + "B"
}

// =============================================================================
// TOOL RESULT LIST
// =============================================================================

// ToolResultList manages a list of tool results.
type ToolResultList struct {
	results []*ToolResultView
	theme   *styles.Theme
	width   int
}

// NewToolResultList creates a new tool result list.
func NewToolResultList(theme *styles.Theme) *ToolResultList {
	return &ToolResultList{
		theme:   theme,
		results: make([]*ToolResultView, 0),
	}
}

// AddResult adds a tool result to the list.
func (l *ToolResultList) AddResult(toolName string, result tools.Result) {
	view := NewToolResultView(l.theme)
	view.SetResult(toolName, result)
	view.SetWidth(l.width)
	l.results = append(l.results, view)
}

// SetWidth sets the width for all results.
func (l *ToolResultList) SetWidth(width int) {
	l.width = width
	for _, r := range l.results {
		r.SetWidth(width)
	}
}

// Clear removes all results.
func (l *ToolResultList) Clear() {
	l.results = make([]*ToolResultView, 0)
}

// Count returns the number of results.
func (l *ToolResultList) Count() int {
	return len(l.results)
}

// ToggleAt toggles the result at the given index.
func (l *ToolResultList) ToggleAt(index int) {
	if index >= 0 && index < len(l.results) {
		l.results[index].Toggle()
	}
}

// View renders all results.
func (l *ToolResultList) View() string {
	if len(l.results) == 0 {
		return ""
	}

	var views []string
	for _, r := range l.results {
		views = append(views, r.View())
	}

	return strings.Join(views, "\n")
}
