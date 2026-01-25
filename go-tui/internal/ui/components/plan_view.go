// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides reusable UI components.
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jeranaias/rigrun-tui/internal/plan"
)

// =============================================================================
// PLAN VIEW COMPONENT
// =============================================================================

// PlanView renders a plan for display in the TUI.
type PlanView struct {
	width  int
	height int
}

// NewPlanView creates a new plan view component.
func NewPlanView(width, height int) *PlanView {
	return &PlanView{
		width:  width,
		height: height,
	}
}

// SetSize updates the dimensions of the plan view.
func (pv *PlanView) SetSize(width, height int) {
	pv.width = width
	pv.height = height
}

// Render renders the plan view.
func (pv *PlanView) Render(p *plan.Plan) string {
	if p == nil {
		return "No plan to display"
	}

	var sb strings.Builder

	// Header
	sb.WriteString(pv.renderHeader(p))
	sb.WriteString("\n\n")

	// Steps
	sb.WriteString(pv.renderSteps(p))
	sb.WriteString("\n")

	// Footer
	sb.WriteString(pv.renderFooter(p))

	return sb.String()
}

// renderHeader renders the plan header.
func (pv *PlanView) renderHeader(p *plan.Plan) string {
	var sb strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#89B4FA")). // Blue
		MarginBottom(1)

	sb.WriteString(titleStyle.Render("Execution Plan"))
	sb.WriteString("\n")

	// Description
	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#CDD6F4")) // Text

	sb.WriteString(descStyle.Render(p.Description))
	sb.WriteString("\n")

	// Status
	statusStyle := lipgloss.NewStyle().
		Foreground(pv.statusColor(p.Status)).
		Bold(true)

	sb.WriteString(fmt.Sprintf("Status: %s", statusStyle.Render(p.Status.String())))

	// Progress
	if p.Status == plan.StatusRunning || p.Status == plan.StatusPaused {
		progressStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A6ADC8")) // Subtext

		sb.WriteString(fmt.Sprintf(" | Progress: %s",
			progressStyle.Render(p.Progress())))
	}

	return sb.String()
}

// renderSteps renders the plan steps.
func (pv *PlanView) renderSteps(p *plan.Plan) string {
	var sb strings.Builder

	stepsStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F5C2E7")). // Pink
		MarginBottom(1)

	sb.WriteString(stepsStyle.Render("Steps:"))
	sb.WriteString("\n")

	for i := range p.Steps {
		sb.WriteString(pv.renderStep(i+1, &p.Steps[i], i == p.CurrentStep))
		if i < len(p.Steps)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderStep renders a single step.
func (pv *PlanView) renderStep(num int, step *plan.PlanStep, isCurrent bool) string {
	var sb strings.Builder

	// Step number and status icon
	icon := pv.statusIcon(step.Status)
	iconColor := pv.stepStatusColor(step.Status)

	iconStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(iconColor))

	numStyle := lipgloss.NewStyle().
		Bold(isCurrent).
		Foreground(lipgloss.Color("#89B4FA")) // Blue

	sb.WriteString(fmt.Sprintf("  %s %s. ",
		iconStyle.Render(icon),
		numStyle.Render(fmt.Sprintf("%d", num))))

	// Description
	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#CDD6F4")). // Text
		Bold(isCurrent)

	if isCurrent && step.Status == plan.StepRunning {
		descStyle = descStyle.Foreground(lipgloss.Color("#F9E2AF")) // Yellow
	}

	sb.WriteString(descStyle.Render(step.Description))

	// Duration (if completed)
	if step.Status == plan.StepComplete || step.Status == plan.StepFailed {
		durationStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A6ADC8")) // Subtext

		sb.WriteString(fmt.Sprintf(" %s",
			durationStyle.Render(fmt.Sprintf("(%.1fs)", step.Duration().Seconds()))))
	}

	// Tool calls (if editable/pending)
	if step.Editable && len(step.ToolCalls) > 0 {
		sb.WriteString("\n")
		for _, tc := range step.ToolCalls {
			sb.WriteString(fmt.Sprintf("     -> %s: %s\n",
				tc.Name,
				tc.Description))
		}
	}

	// Error (if failed)
	if step.Status == plan.StepFailed && step.Error != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F38BA8")). // Red
			Italic(true)

		sb.WriteString(fmt.Sprintf("\n     Error: %s",
			errorStyle.Render(step.Error.Error())))
	}

	// Result (if completed and has result)
	if step.Status == plan.StepComplete && step.Result != "" {
		resultStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A6ADC8")). // Subtext
			Italic(true)

		// Truncate long results
		result := step.Result
		if len(result) > 100 {
			result = result[:97] + "..."
		}
		sb.WriteString(fmt.Sprintf("\n     %s",
			resultStyle.Render(result)))
	}

	return sb.String()
}

// renderFooter renders the plan footer with actions.
func (pv *PlanView) renderFooter(p *plan.Plan) string {
	var actions []string

	switch p.Status {
	case plan.StatusDraft:
		actions = append(actions, "[a]pprove", "[e]dit", "[c]ancel")
	case plan.StatusApproved:
		actions = append(actions, "[s]tart", "[e]dit", "[c]ancel")
	case plan.StatusRunning:
		actions = append(actions, "[p]ause", "[c]ancel")
	case plan.StatusPaused:
		actions = append(actions, "[r]esume", "[e]dit", "[c]ancel")
	case plan.StatusComplete:
		actions = append(actions, "[n]ew plan", "[c]lose")
	case plan.StatusFailed:
		actions = append(actions, "[r]etry", "[e]dit", "[c]lose")
	}

	if len(actions) == 0 {
		return ""
	}

	actionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A6ADC8")). // Subtext
		Italic(true)

	return actionStyle.Render("Actions: " + strings.Join(actions, " | "))
}

// statusIcon returns the icon for a step status (ASCII-compatible).
func (pv *PlanView) statusIcon(status plan.StepStatus) string {
	switch status {
	case plan.StepPending:
		return "[ ]" // Empty
	case plan.StepRunning:
		return "[>]" // In progress
	case plan.StepComplete:
		return "[x]" // Filled
	case plan.StepFailed:
		return "[X]" // Failed
	case plan.StepSkipped:
		return "[-]" // Skipped
	default:
		return "[?]"
	}
}

// statusColor returns the color for a plan status.
func (pv *PlanView) statusColor(status plan.PlanStatus) lipgloss.Color {
	switch status {
	case plan.StatusDraft:
		return lipgloss.Color("#A6ADC8") // Subtext
	case plan.StatusApproved:
		return lipgloss.Color("#89B4FA") // Blue
	case plan.StatusRunning:
		return lipgloss.Color("#F9E2AF") // Yellow
	case plan.StatusPaused:
		return lipgloss.Color("#FAB387") // Peach
	case plan.StatusComplete:
		return lipgloss.Color("#A6E3A1") // Green
	case plan.StatusFailed:
		return lipgloss.Color("#F38BA8") // Red
	case plan.StatusCancelled:
		return lipgloss.Color("#6C7086") // Overlay0
	default:
		return lipgloss.Color("#CDD6F4") // Text
	}
}

// stepStatusColor returns the color for a step status.
func (pv *PlanView) stepStatusColor(status plan.StepStatus) string {
	switch status {
	case plan.StepPending:
		return "#6C7086" // Overlay0
	case plan.StepRunning:
		return "#F9E2AF" // Yellow
	case plan.StepComplete:
		return "#A6E3A1" // Green
	case plan.StepFailed:
		return "#F38BA8" // Red
	case plan.StepSkipped:
		return "#A6ADC8" // Subtext
	default:
		return "#CDD6F4" // Text
	}
}

// =============================================================================
// COMPACT PLAN PROGRESS
// =============================================================================

// RenderCompactProgress renders a compact one-line progress indicator.
func RenderCompactProgress(p *plan.Plan) string {
	if p == nil {
		return ""
	}

	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9E2AF")). // Yellow
		Bold(true)

	return style.Render(fmt.Sprintf("Plan: Step %s - %s",
		p.Progress(),
		p.CurrentStepDescription()))
}
