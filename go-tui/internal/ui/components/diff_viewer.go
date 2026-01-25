// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/diff"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// DIFF VIEWER
// =============================================================================

// DiffViewer displays file diffs with syntax highlighting.
type DiffViewer struct {
	diff       *diff.Diff
	width      int
	height     int
	scrollPos  int
	approved   bool
	rejected   bool
	showHelp   bool
}

// NewDiffViewer creates a new diff viewer.
func NewDiffViewer(d *diff.Diff) *DiffViewer {
	return &DiffViewer{
		diff:     d,
		width:    80,
		height:   24,
		showHelp: true,
	}
}

// SetSize sets the viewer dimensions.
func (dv *DiffViewer) SetSize(width, height int) {
	dv.width = width
	dv.height = height
}

// Approve marks the diff as approved.
func (dv *DiffViewer) Approve() {
	dv.approved = true
	dv.rejected = false
}

// Reject marks the diff as rejected.
func (dv *DiffViewer) Reject() {
	dv.approved = false
	dv.rejected = true
}

// IsApproved returns whether the diff was approved.
func (dv *DiffViewer) IsApproved() bool {
	return dv.approved
}

// IsRejected returns whether the diff was rejected.
func (dv *DiffViewer) IsRejected() bool {
	return dv.rejected
}

// ScrollUp scrolls the view up.
func (dv *DiffViewer) ScrollUp(lines int) {
	dv.scrollPos -= lines
	if dv.scrollPos < 0 {
		dv.scrollPos = 0
	}
}

// ScrollDown scrolls the view down.
func (dv *DiffViewer) ScrollDown(lines int) {
	dv.scrollPos += lines
	// Max scroll will be limited in View()
}

// =============================================================================
// RENDERING
// =============================================================================

// View renders the diff viewer.
func (dv *DiffViewer) View() string {
	if dv.diff == nil {
		return "No diff available"
	}

	var content strings.Builder

	// Header
	content.WriteString(dv.renderHeader())
	content.WriteString("\n\n")

	// Diff stats
	content.WriteString(dv.renderStats())
	content.WriteString("\n\n")

	// File path
	filePathStyle := lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true)
	content.WriteString(filePathStyle.Render(dv.diff.FilePath))
	content.WriteString("\n")

	// Separator
	separatorStyle := lipgloss.NewStyle().
		Foreground(styles.Overlay)
	content.WriteString(separatorStyle.Render(strings.Repeat("-", minDiffInt(dv.width-4, 80))))
	content.WriteString("\n\n")

	// Diff hunks
	content.WriteString(dv.renderHunks())

	// Footer with help
	if dv.showHelp {
		content.WriteString("\n\n")
		content.WriteString(dv.renderHelp())
	}

	// Wrap in container
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Purple).
		Padding(1, 2).
		Width(minDiffInt(dv.width-4, 100))

	return containerStyle.Render(content.String())
}

// renderHeader renders the diff viewer header.
func (dv *DiffViewer) renderHeader() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(styles.Purple).
		Bold(true).
		Underline(true)

	return titleStyle.Render("File Diff Preview")
}

// renderStats renders diff statistics.
func (dv *DiffViewer) renderStats() string {
	stats := dv.diff.Stats

	var parts []string

	// File mode
	modeStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Italic(true)

	switch stats.FileMode {
	case "new":
		parts = append(parts, modeStyle.Render("New file"))
	case "deleted":
		parts = append(parts, modeStyle.Render("Deleted file"))
	case "modified":
		parts = append(parts, modeStyle.Render("Modified"))
	}

	// Additions
	if stats.Additions > 0 {
		addStyle := lipgloss.NewStyle().
			Foreground(styles.Emerald).
			Bold(true)
		parts = append(parts, addStyle.Render(fmt.Sprintf("+%d", stats.Additions)))
	}

	// Deletions
	if stats.Deletions > 0 {
		delStyle := lipgloss.NewStyle().
			Foreground(styles.Rose).
			Bold(true)
		parts = append(parts, delStyle.Render(fmt.Sprintf("-%d", stats.Deletions)))
	}

	// Lines
	if stats.Additions > 0 || stats.Deletions > 0 {
		lineText := "line"
		if stats.Additions+stats.Deletions != 1 {
			lineText = "lines"
		}
		parts = append(parts, modeStyle.Render(lineText))
	}

	return strings.Join(parts, " ")
}

// renderHunks renders all diff hunks.
func (dv *DiffViewer) renderHunks() string {
	if len(dv.diff.Hunks) == 0 {
		noChangesStyle := lipgloss.NewStyle().
			Foreground(styles.TextMuted).
			Italic(true)
		return noChangesStyle.Render("No changes")
	}

	var content strings.Builder

	for i, hunk := range dv.diff.Hunks {
		if i > 0 {
			content.WriteString("\n")
		}
		content.WriteString(dv.renderHunk(hunk))
	}

	return content.String()
}

// renderHunk renders a single diff hunk.
func (dv *DiffViewer) renderHunk(hunk diff.DiffHunk) string {
	var content strings.Builder

	// Hunk header
	hunkHeaderStyle := lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Background(styles.SurfaceDim).
		Bold(true).
		Padding(0, 1)

	header := fmt.Sprintf("@@ -%d,%d +%d,%d @@",
		hunk.OldStart, hunk.OldCount,
		hunk.NewStart, hunk.NewCount)
	content.WriteString(hunkHeaderStyle.Render(header))
	content.WriteString("\n")

	// Lines
	for _, line := range hunk.Lines {
		content.WriteString(dv.renderLine(line))
		content.WriteString("\n")
	}

	return content.String()
}

// renderLine renders a single diff line.
func (dv *DiffViewer) renderLine(line diff.DiffLine) string {
	var lineNumStr string
	var prefix string
	var lineStyle lipgloss.Style

	switch line.Type {
	case diff.DiffLineAdded:
		// Added line - green
		lineStyle = lipgloss.NewStyle().
			Foreground(styles.Emerald).
			Background(lipgloss.Color("#003300")) // Dark green background
		prefix = "+"
		lineNumStr = fmt.Sprintf("    %4d", line.NewLine)

	case diff.DiffLineRemoved:
		// Removed line - red
		lineStyle = lipgloss.NewStyle().
			Foreground(styles.Rose).
			Background(lipgloss.Color("#330000")) // Dark red background
		prefix = "-"
		lineNumStr = fmt.Sprintf("%4d    ", line.OldLine)

	case diff.DiffLineContext:
		// Context line - gray
		lineStyle = lipgloss.NewStyle().
			Foreground(styles.TextMuted)
		prefix = " "
		lineNumStr = fmt.Sprintf("%4d %4d", line.OldLine, line.NewLine)
	}

	// Line number style
	lineNumStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Width(9) // Fixed width for alignment

	// Render the line
	formattedLineNum := lineNumStyle.Render(lineNumStr)
	formattedContent := lineStyle.Render(prefix + line.Content)

	return formattedLineNum + " " + formattedContent
}

// renderHelp renders the help/action prompt.
func (dv *DiffViewer) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Italic(true)

	if dv.approved {
		approvedStyle := lipgloss.NewStyle().
			Foreground(styles.Emerald).
			Bold(true)
		return approvedStyle.Render("[OK] Changes approved - will be applied")
	}

	if dv.rejected {
		rejectedStyle := lipgloss.NewStyle().
			Foreground(styles.Rose).
			Bold(true)
		return rejectedStyle.Render("[X] Changes rejected - will not be applied")
	}

	var lines []string
	lines = append(lines, helpStyle.Render("Review the changes above:"))
	lines = append(lines, "")

	approveStyle := lipgloss.NewStyle().
		Foreground(styles.Emerald).
		Bold(true)
	rejectStyle := lipgloss.NewStyle().
		Foreground(styles.Rose).
		Bold(true)

	lines = append(lines, approveStyle.Render("  y / Enter")+"  - Approve and apply changes")
	lines = append(lines, rejectStyle.Render("  n / Esc")+"   - Reject changes")

	return strings.Join(lines, "\n")
}

// =============================================================================
// HELPERS
// =============================================================================

// minDiffInt returns the minimum of two integers (diff viewer specific).
func minDiffInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
