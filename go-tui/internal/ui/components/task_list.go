// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides reusable UI components for the TUI.
package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jeranaias/rigrun-tui/internal/tasks"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// TASK LIST COMPONENT
// =============================================================================

// TaskList renders a list of background tasks.
type TaskList struct {
	queue  *tasks.Queue
	theme  *styles.Theme
	width  int
	height int

	// Filter options
	showCompleted bool
	showFailed    bool
	showCanceled  bool
}

// NewTaskList creates a new task list component.
func NewTaskList(queue *tasks.Queue, theme *styles.Theme) *TaskList {
	return &TaskList{
		queue:         queue,
		theme:         theme,
		showCompleted: true,
		showFailed:    true,
		showCanceled:  false,
	}
}

// =============================================================================
// CONFIGURATION
// =============================================================================

// SetSize sets the component dimensions.
func (tl *TaskList) SetSize(width, height int) {
	tl.width = width
	tl.height = height
}

// SetShowCompleted sets whether to show completed tasks.
func (tl *TaskList) SetShowCompleted(show bool) {
	tl.showCompleted = show
}

// SetShowFailed sets whether to show failed tasks.
func (tl *TaskList) SetShowFailed(show bool) {
	tl.showFailed = show
}

// SetShowCanceled sets whether to show canceled tasks.
func (tl *TaskList) SetShowCanceled(show bool) {
	tl.showCanceled = show
}

// =============================================================================
// RENDERING
// =============================================================================

// View renders the task list.
func (tl *TaskList) View() string {
	if tl.queue == nil {
		return tl.renderEmpty()
	}

	// Get tasks based on filter
	allTasks := tl.queue.All()
	if len(allTasks) == 0 {
		return tl.renderEmpty()
	}

	var filtered []*tasks.Task
	for _, task := range allTasks {
		if tl.shouldShow(task) {
			filtered = append(filtered, task)
		}
	}

	if len(filtered) == 0 {
		return tl.renderNoMatching()
	}

	return tl.renderTasks(filtered)
}

// shouldShow returns true if the task should be displayed based on filters.
func (tl *TaskList) shouldShow(task *tasks.Task) bool {
	status := task.GetStatus()
	switch status {
	case tasks.TaskStatusComplete:
		return tl.showCompleted
	case tasks.TaskStatusFailed:
		return tl.showFailed
	case tasks.TaskStatusCanceled:
		return tl.showCanceled
	default:
		return true // Always show running and queued
	}
}

// renderEmpty renders the empty state.
func (tl *TaskList) renderEmpty() string {
	emptyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true).
		Padding(2).
		Width(tl.width).
		Align(lipgloss.Center)

	return emptyStyle.Render("No background tasks")
}

// renderNoMatching renders when no tasks match the filter.
func (tl *TaskList) renderNoMatching() string {
	emptyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true).
		Padding(2).
		Width(tl.width).
		Align(lipgloss.Center)

	return emptyStyle.Render("No tasks match current filter")
}

// renderTasks renders the list of tasks.
func (tl *TaskList) renderTasks(taskList []*tasks.Task) string {
	var b strings.Builder

	// Header
	header := tl.renderHeader()
	b.WriteString(header)
	b.WriteString("\n\n")

	// Task rows
	for i, task := range taskList {
		row := tl.renderTask(task)
		b.WriteString(row)
		if i < len(taskList)-1 {
			b.WriteString("\n")
		}
	}

	// Footer with summary
	b.WriteString("\n\n")
	b.WriteString(tl.renderFooter())

	return b.String()
}

// renderHeader renders the task list header.
func (tl *TaskList) renderHeader() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("14")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("240")).
		Width(tl.width).
		Padding(0, 1)

	return headerStyle.Render("Background Tasks")
}

// renderTask renders a single task row.
func (tl *TaskList) renderTask(task *tasks.Task) string {
	// Task ID (short)
	id := task.ID
	if len(id) > 8 {
		id = id[:8]
	}

	// Status icon and color
	icon, color := tl.statusIcon(task.GetStatus())

	// Duration
	duration := formatTaskDuration(task.Duration())

	// Progress (for running tasks)
	progress := ""
	if task.IsRunning() {
		progress = fmt.Sprintf("[%d%%]", task.GetProgress())
	}

	// Build row
	row := fmt.Sprintf("%s %s  %s  %s %s",
		lipgloss.NewStyle().Foreground(color).Render(icon),
		lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(id),
		task.Description,
		progress,
		lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(duration),
	)

	// Wrap in styled container
	rowStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Width(tl.width)

	return rowStyle.Render(row)
}

// statusIcon returns the icon and color for a task status (ASCII-compatible).
func (tl *TaskList) statusIcon(status tasks.TaskStatus) (string, lipgloss.Color) {
	switch status {
	case tasks.TaskStatusQueued:
		return "[ ]", lipgloss.Color("11") // Yellow
	case tasks.TaskStatusRunning:
		return "[>]", lipgloss.Color("14") // Cyan
	case tasks.TaskStatusComplete:
		return "[OK]", lipgloss.Color("10") // Green
	case tasks.TaskStatusFailed:
		return "[X]", lipgloss.Color("9") // Red
	case tasks.TaskStatusCanceled:
		return "[--]", lipgloss.Color("240") // Gray
	default:
		return "[?]", lipgloss.Color("240")
	}
}

// renderFooter renders the footer with queue summary.
func (tl *TaskList) renderFooter() string {
	summary := tl.queue.Summary()

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(lipgloss.Color("240")).
		Width(tl.width).
		Padding(0, 1)

	return footerStyle.Render(summary)
}

// =============================================================================
// TASK DETAIL VIEW
// =============================================================================

// ViewDetail renders detailed information about a specific task.
func (tl *TaskList) ViewDetail(taskID string) string {
	task := tl.queue.Get(taskID)
	if task == nil {
		return tl.renderTaskNotFound(taskID)
	}

	var b strings.Builder

	// Header
	b.WriteString(tl.renderDetailHeader(task))
	b.WriteString("\n\n")

	// Task info
	b.WriteString(tl.renderTaskInfo(task))
	b.WriteString("\n\n")

	// Output
	if task.GetOutput() != "" {
		b.WriteString(tl.renderOutput(task))
		b.WriteString("\n\n")
	}

	// Error
	if task.GetError() != "" {
		b.WriteString(tl.renderError(task))
	}

	return b.String()
}

// renderTaskNotFound renders when a task is not found.
func (tl *TaskList) renderTaskNotFound(taskID string) string {
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")).
		Bold(true).
		Padding(2).
		Width(tl.width).
		Align(lipgloss.Center)

	return errorStyle.Render(fmt.Sprintf("Task not found: %s", taskID))
}

// renderDetailHeader renders the detail view header.
func (tl *TaskList) renderDetailHeader(task *tasks.Task) string {
	icon, color := tl.statusIcon(task.GetStatus())

	header := fmt.Sprintf("%s  %s",
		lipgloss.NewStyle().Foreground(color).Render(icon),
		task.Description,
	)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("14")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("240")).
		Width(tl.width).
		Padding(0, 1)

	return headerStyle.Render(header)
}

// renderTaskInfo renders task metadata.
func (tl *TaskList) renderTaskInfo(task *tasks.Task) string {
	var b strings.Builder

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Bold(true)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15"))

	// ID
	b.WriteString(labelStyle.Render("ID: "))
	b.WriteString(valueStyle.Render(task.ID))
	b.WriteString("\n")

	// Status
	b.WriteString(labelStyle.Render("Status: "))
	b.WriteString(valueStyle.Render(string(task.GetStatus())))
	b.WriteString("\n")

	// Command
	b.WriteString(labelStyle.Render("Command: "))
	cmdStr := task.Command
	if len(task.Args) > 0 {
		cmdStr += " " + strings.Join(task.Args, " ")
	}
	b.WriteString(valueStyle.Render(cmdStr))
	b.WriteString("\n")

	// Duration
	if task.Duration() > 0 {
		b.WriteString(labelStyle.Render("Duration: "))
		b.WriteString(valueStyle.Render(formatTaskDuration(task.Duration())))
		b.WriteString("\n")
	}

	// Progress
	if task.IsRunning() {
		b.WriteString(labelStyle.Render("Progress: "))
		b.WriteString(valueStyle.Render(fmt.Sprintf("%d%%", task.GetProgress())))
		b.WriteString("\n")
	}

	return b.String()
}

// renderOutput renders task output.
func (tl *TaskList) renderOutput(task *tasks.Task) string {
	output := task.GetOutput()

	// Limit output length for display
	maxLines := 20
	lines := strings.Split(output, "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
		output = "... (truncated)\n" + strings.Join(lines, "\n")
	}

	outputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1).
		Width(tl.width - 4)

	return outputStyle.Render(output)
}

// renderError renders task error.
func (tl *TaskList) renderError(task *tasks.Task) string {
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")).
		Bold(true).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("9")).
		Padding(1).
		Width(tl.width - 4)

	return errorStyle.Render("Error: " + task.GetError())
}

// =============================================================================
// UTILITY FUNCTIONS
// =============================================================================

// formatTaskDuration formats a duration for display in task list.
func formatTaskDuration(d time.Duration) string {
	if d == 0 {
		return "-"
	}

	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}

	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}

	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", mins, secs)
	}

	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", hours, mins)
}
