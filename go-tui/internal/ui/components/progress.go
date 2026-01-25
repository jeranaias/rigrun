// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides the visual UI components for rigrun TUI.
package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// PROGRESS INDICATOR COMPONENT
// =============================================================================

// ProgressStatus represents the state of a progress indicator
type ProgressStatus string

const (
	ProgressStatusRunning  ProgressStatus = "Running"
	ProgressStatusPaused   ProgressStatus = "Paused"
	ProgressStatusComplete ProgressStatus = "Complete"
	ProgressStatusCanceled ProgressStatus = "Canceled"
	ProgressStatusError    ProgressStatus = "Error"
)

// ProgressIndicator represents a progress tracker for multi-step operations.
// Displays current step, overall progress, tool being executed, and elapsed time.
type ProgressIndicator struct {
	// Step tracking
	CurrentStep int
	TotalSteps  int
	StepTitle   string // e.g., "Running tests"

	// Tool tracking
	CurrentTool     string // e.g., "Bash"
	CurrentToolArgs string // e.g., "pytest tests/"

	// Time tracking
	StepStartTime time.Time // When current step started
	OverallStart  time.Time // When overall operation started

	// State
	Status ProgressStatus

	// Display settings
	Width          int
	ShowCancelHint bool // Whether to show "Press Ctrl+C to cancel"
	Compact        bool // Use compact single-line mode
}

// NewProgressIndicator creates a new progress indicator
func NewProgressIndicator(totalSteps int) *ProgressIndicator {
	if totalSteps < 1 {
		totalSteps = 1
	}
	now := time.Now()
	return &ProgressIndicator{
		CurrentStep:    0,
		TotalSteps:     totalSteps,
		StepTitle:      "",
		CurrentTool:    "",
		CurrentToolArgs: "",
		StepStartTime:  now,
		OverallStart:   now,
		Status:         ProgressStatusRunning,
		Width:          80,
		ShowCancelHint: true,
		Compact:        false,
	}
}

// StartStep marks the beginning of a new step
func (p *ProgressIndicator) StartStep(step int, title string) {
	if step < 1 {
		step = 1
	}
	if step > p.TotalSteps {
		step = p.TotalSteps
	}
	p.CurrentStep = step
	p.StepTitle = title
	p.StepStartTime = time.Now()
	p.Status = ProgressStatusRunning
}

// SetTool updates the current tool being executed
func (p *ProgressIndicator) SetTool(toolName string, args string) {
	p.CurrentTool = toolName
	p.CurrentToolArgs = args
}

// Complete marks the progress as complete
func (p *ProgressIndicator) Complete() {
	p.Status = ProgressStatusComplete
	p.CurrentStep = p.TotalSteps
}

// Cancel marks the progress as canceled
func (p *ProgressIndicator) Cancel() {
	p.Status = ProgressStatusCanceled
}

// Pause pauses the progress
func (p *ProgressIndicator) Pause() {
	p.Status = ProgressStatusPaused
}

// Resume resumes the progress
func (p *ProgressIndicator) Resume() {
	p.Status = ProgressStatusRunning
}

// Error marks the progress as errored
func (p *ProgressIndicator) Error() {
	p.Status = ProgressStatusError
}

// GetStepElapsed returns the elapsed time for the current step
func (p *ProgressIndicator) GetStepElapsed() time.Duration {
	if p.StepStartTime.IsZero() {
		return 0
	}
	return time.Since(p.StepStartTime)
}

// GetOverallElapsed returns the total elapsed time
func (p *ProgressIndicator) GetOverallElapsed() time.Duration {
	if p.OverallStart.IsZero() {
		return 0
	}
	return time.Since(p.OverallStart)
}

// GetPercent returns the progress percentage (0-100)
func (p *ProgressIndicator) GetPercent() float64 {
	if p.TotalSteps <= 0 {
		return 0
	}
	return float64(p.CurrentStep) / float64(p.TotalSteps) * 100
}

// IsActive returns true if the progress is running or paused
func (p *ProgressIndicator) IsActive() bool {
	return p.Status == ProgressStatusRunning || p.Status == ProgressStatusPaused
}

// =============================================================================
// RENDERING
// =============================================================================

// Render renders the progress indicator
func (p *ProgressIndicator) Render() string {
	if p.Compact {
		return p.renderCompact()
	}
	return p.renderFull()
}

// renderFull renders the full boxed progress indicator
func (p *ProgressIndicator) renderFull() string {
	// Calculate dimensions
	width := p.Width
	if width <= 0 {
		width = 80
	}
	contentWidth := width - 4 // Account for borders and padding

	if contentWidth < 30 {
		// Too narrow for full display - use compact mode
		return p.renderCompact()
	}

	// Build content lines
	var lines []string

	// Line 1: Step indicator
	stepLine := p.renderStepLine()
	lines = append(lines, stepLine)

	// Line 2: Progress bar
	progressBar := p.renderProgressBar(contentWidth)
	lines = append(lines, progressBar)

	// Line 3: Tool info (if available)
	if p.CurrentTool != "" {
		toolLine := p.renderToolLine()
		lines = append(lines, toolLine)
	}

	// Line 4: Time info
	timeLine := p.renderTimeLine()
	lines = append(lines, timeLine)

	// Line 5: Cancel hint (if enabled)
	if p.ShowCancelHint && p.Status == ProgressStatusRunning {
		cancelHint := p.renderCancelHint()
		lines = append(lines, cancelHint)
	}

	// Join lines
	content := strings.Join(lines, "\n")

	// Determine border color based on status
	borderColor := p.getBorderColor()

	// Create box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(contentWidth)

	// Add title based on status
	title := p.getTitle()
	if title != "" {
		// Title style
		titleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(borderColor)

		titleText := titleStyle.Render(title)

		// Create bordered box with title
		box := boxStyle.Render(content)

		// Manually add title to border (simple approach)
		return titleText + "\n" + box
	}

	return boxStyle.Render(content)
}

// renderCompact renders a single-line compact progress indicator
func (p *ProgressIndicator) renderCompact() string {
	// Format: [2/5] Running tests | Bash (pytest) | 12s | 40% [####------]
	var parts []string

	// Step counter
	stepCounter := fmt.Sprintf("[%d/%d]", p.CurrentStep, p.TotalSteps)
	stepStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Purple)
	parts = append(parts, stepStyle.Render(stepCounter))

	// Step title
	if p.StepTitle != "" {
		titleStyle := lipgloss.NewStyle().Foreground(styles.TextPrimary)
		parts = append(parts, titleStyle.Render(p.StepTitle))
	}

	// Tool info
	if p.CurrentTool != "" {
		toolInfo := p.CurrentTool
		if p.CurrentToolArgs != "" {
			// Truncate args if too long (use rune-based truncation for Unicode safety)
			args := p.CurrentToolArgs
			if len(args) > 20 {
				runes := []rune(args)
				if len(runes) > 20 {
					args = string(runes[:17]) + "..."
				}
			}
			toolInfo = fmt.Sprintf("%s (%s)", p.CurrentTool, args)
		}
		toolStyle := lipgloss.NewStyle().Foreground(styles.Cyan)
		parts = append(parts, toolStyle.Render(toolInfo))
	}

	// Elapsed time
	elapsed := p.GetStepElapsed()
	timeStr := formatProgressDuration(elapsed)
	timeStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	parts = append(parts, timeStyle.Render(timeStr))

	// Progress bar (compact)
	percent := p.GetPercent()
	barWidth := 10
	progressBar := styles.RenderProgressBar(barWidth, percent)
	percentStr := fmt.Sprintf("%.0f%%", percent)
	progressStyle := lipgloss.NewStyle().Foreground(p.getProgressColor())
	parts = append(parts, progressStyle.Render(percentStr+" "+progressBar))

	// Join with separator
	sep := lipgloss.NewStyle().Foreground(styles.Overlay).Render(" | ")
	return strings.Join(parts, sep)
}

// renderStepLine renders the step indicator line
func (p *ProgressIndicator) renderStepLine() string {
	stepNum := fmt.Sprintf("Step %d of %d", p.CurrentStep, p.TotalSteps)

	stepStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Purple)

	titleStyle := lipgloss.NewStyle().
		Foreground(styles.TextPrimary)

	if p.StepTitle != "" {
		return stepStyle.Render(stepNum) + ": " + titleStyle.Render(p.StepTitle)
	}

	return stepStyle.Render(stepNum)
}

// renderProgressBar renders the progress bar line
func (p *ProgressIndicator) renderProgressBar(width int) string {
	// Reserve space for percentage
	barWidth := width - 10 // "100% " + some padding
	if barWidth < 10 {
		barWidth = 10
	}

	percent := p.GetPercent()
	bar := styles.RenderProgressBar(barWidth, percent)

	percentStr := fmt.Sprintf("%.0f%%", percent)
	percentStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.getProgressColor())

	barStyle := lipgloss.NewStyle().
		Foreground(p.getProgressColor())

	return barStyle.Render(bar) + " " + percentStyle.Render(percentStr)
}

// renderToolLine renders the tool execution line
func (p *ProgressIndicator) renderToolLine() string {
	labelStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted)

	toolStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Cyan)

	argsStyle := lipgloss.NewStyle().
		Foreground(styles.TextSecondary)

	result := labelStyle.Render("Tool: ") + toolStyle.Render(p.CurrentTool)

	if p.CurrentToolArgs != "" {
		result += " " + argsStyle.Render(fmt.Sprintf("(%s)", p.CurrentToolArgs))
	}

	return result
}

// renderTimeLine renders the time elapsed line
func (p *ProgressIndicator) renderTimeLine() string {
	stepElapsed := p.GetStepElapsed()
	overallElapsed := p.GetOverallElapsed()

	labelStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted)

	timeStyle := lipgloss.NewStyle().
		Foreground(styles.TextSecondary)

	stepStr := formatProgressDuration(stepElapsed)
	totalStr := formatProgressDuration(overallElapsed)

	return labelStyle.Render("Elapsed: ") +
		timeStyle.Render(stepStr) +
		labelStyle.Render(" | Total: ") +
		timeStyle.Render(totalStr)
}

// renderCancelHint renders the cancellation hint
func (p *ProgressIndicator) renderCancelHint() string {
	hintStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Italic(true)

	return hintStyle.Render("Press Ctrl+C to cancel")
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// getBorderColor returns the border color based on status
func (p *ProgressIndicator) getBorderColor() lipgloss.AdaptiveColor {
	switch p.Status {
	case ProgressStatusRunning:
		return styles.Purple
	case ProgressStatusPaused:
		return styles.Amber
	case ProgressStatusComplete:
		return styles.Emerald
	case ProgressStatusCanceled:
		return styles.TextMuted
	case ProgressStatusError:
		return styles.Rose
	default:
		return styles.Purple
	}
}

// getProgressColor returns the progress bar color based on status
func (p *ProgressIndicator) getProgressColor() lipgloss.AdaptiveColor {
	switch p.Status {
	case ProgressStatusRunning:
		return styles.Purple
	case ProgressStatusPaused:
		return styles.Amber
	case ProgressStatusComplete:
		return styles.Emerald
	case ProgressStatusCanceled:
		return styles.TextMuted
	case ProgressStatusError:
		return styles.Rose
	default:
		return styles.Purple
	}
}

// getTitle returns the title text based on status
func (p *ProgressIndicator) getTitle() string {
	switch p.Status {
	case ProgressStatusRunning:
		return "- Executing Plan -"
	case ProgressStatusPaused:
		return "- Paused -"
	case ProgressStatusComplete:
		return "- Complete -"
	case ProgressStatusCanceled:
		return "- Canceled -"
	case ProgressStatusError:
		return "- Error -"
	default:
		return "- Progress -"
	}
}

// formatProgressDuration formats a duration for display
func formatProgressDuration(d time.Duration) string {
	seconds := int(d.Seconds())

	if seconds < 1 {
		// Show milliseconds for very short durations
		ms := int(d.Milliseconds())
		return fmt.Sprintf("%dms", ms)
	}

	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}

	minutes := seconds / 60
	secs := seconds % 60

	if minutes < 60 {
		return fmt.Sprintf("%dm %ds", minutes, secs)
	}

	hours := minutes / 60
	mins := minutes % 60

	return fmt.Sprintf("%dh %dm", hours, mins)
}

// =============================================================================
// MULTI-STEP PROGRESS TRACKER
// =============================================================================

// StepInfo represents information about a single step
type StepInfo struct {
	Title       string
	Description string
	Status      ProgressStatus
	StartTime   time.Time
	EndTime     time.Time
	Tool        string
	ToolArgs    string
}

// MultiStepProgress tracks progress across multiple steps
type MultiStepProgress struct {
	Steps           []StepInfo
	CurrentStepIdx  int
	OverallStart    time.Time
	Indicator       *ProgressIndicator
	OnStepComplete  func(step int, info StepInfo) // Callback when step completes
	OnAllComplete   func()                        // Callback when all steps complete
	OnError         func(step int, err error)     // Callback on error
}

// NewMultiStepProgress creates a new multi-step progress tracker
func NewMultiStepProgress(stepTitles []string) *MultiStepProgress {
	steps := make([]StepInfo, len(stepTitles))
	for i, title := range stepTitles {
		steps[i] = StepInfo{
			Title:  title,
			Status: ProgressStatusRunning,
		}
	}

	return &MultiStepProgress{
		Steps:          steps,
		CurrentStepIdx: 0,
		OverallStart:   time.Now(),
		Indicator:      NewProgressIndicator(len(stepTitles)),
	}
}

// NextStep advances to the next step
func (m *MultiStepProgress) NextStep() {
	// Mark current step as complete
	if m.CurrentStepIdx < len(m.Steps) {
		m.Steps[m.CurrentStepIdx].Status = ProgressStatusComplete
		m.Steps[m.CurrentStepIdx].EndTime = time.Now()

		// Call completion callback
		if m.OnStepComplete != nil {
			m.OnStepComplete(m.CurrentStepIdx, m.Steps[m.CurrentStepIdx])
		}
	}

	// Advance to next step
	m.CurrentStepIdx++

	// Update indicator
	if m.CurrentStepIdx < len(m.Steps) {
		step := &m.Steps[m.CurrentStepIdx]
		step.StartTime = time.Now()
		step.Status = ProgressStatusRunning

		m.Indicator.StartStep(m.CurrentStepIdx+1, step.Title)
		m.Indicator.SetTool(step.Tool, step.ToolArgs)
	} else {
		// All steps complete
		m.Indicator.Complete()
		if m.OnAllComplete != nil {
			m.OnAllComplete()
		}
	}
}

// SetToolForCurrentStep sets the tool being executed for the current step
func (m *MultiStepProgress) SetToolForCurrentStep(tool, args string) {
	if m.CurrentStepIdx < len(m.Steps) {
		m.Steps[m.CurrentStepIdx].Tool = tool
		m.Steps[m.CurrentStepIdx].ToolArgs = args
		m.Indicator.SetTool(tool, args)
	}
}

// MarkError marks the current step as errored
func (m *MultiStepProgress) MarkError(err error) {
	if m.CurrentStepIdx < len(m.Steps) {
		m.Steps[m.CurrentStepIdx].Status = ProgressStatusError
		m.Steps[m.CurrentStepIdx].EndTime = time.Now()
	}

	m.Indicator.Error()

	if m.OnError != nil {
		m.OnError(m.CurrentStepIdx, err)
	}
}

// Cancel cancels the progress
func (m *MultiStepProgress) Cancel() {
	if m.CurrentStepIdx < len(m.Steps) {
		m.Steps[m.CurrentStepIdx].Status = ProgressStatusCanceled
		m.Steps[m.CurrentStepIdx].EndTime = time.Now()
	}

	m.Indicator.Cancel()
}

// Render renders the current progress state
func (m *MultiStepProgress) Render() string {
	return m.Indicator.Render()
}

// GetSummary returns a summary of all steps
func (m *MultiStepProgress) GetSummary() string {
	var lines []string

	for i, step := range m.Steps {
		statusIcon := getStatusIcon(step.Status)

		var timing string
		if !step.StartTime.IsZero() && !step.EndTime.IsZero() {
			duration := step.EndTime.Sub(step.StartTime)
			timing = fmt.Sprintf(" (%s)", formatProgressDuration(duration))
		}

		line := fmt.Sprintf("%s %s%s", statusIcon, step.Title, timing)

		if i == m.CurrentStepIdx && step.Status == ProgressStatusRunning {
			// Highlight current step
			line = lipgloss.NewStyle().Bold(true).Foreground(styles.Purple).Render(line)
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// getStatusIcon returns an icon for the status
func getStatusIcon(status ProgressStatus) string {
	switch status {
	case ProgressStatusComplete:
		return styles.StatusIndicators.Success
	case ProgressStatusRunning:
		return styles.StatusIndicators.Pending
	case ProgressStatusError:
		return styles.StatusIndicators.Error
	case ProgressStatusCanceled:
		return "-"
	case ProgressStatusPaused:
		return "||"
	default:
		return "?"
	}
}
