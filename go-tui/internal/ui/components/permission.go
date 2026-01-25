// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/tools"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// PERMISSION PROMPT
// =============================================================================

// PermissionPrompt displays a modal dialog for tool permission requests.
type PermissionPrompt struct {
	// Tool information
	tool   *tools.Tool
	call   *tools.ToolCall

	// UI state
	visible  bool
	selected int // 0=Allow, 1=Allow Always, 2=Deny
	width    int
	height   int

	// Styles
	theme *styles.Theme
}

// Button options
const (
	ButtonAllow       = 0
	ButtonAlwaysAllow = 1
	ButtonDeny        = 2
	ButtonCount       = 3
)

// NewPermissionPrompt creates a new permission prompt.
func NewPermissionPrompt(theme *styles.Theme) *PermissionPrompt {
	return &PermissionPrompt{
		theme:    theme,
		selected: ButtonAllow,
	}
}

// =============================================================================
// PERMISSION PROMPT METHODS
// =============================================================================

// Show displays the permission prompt for a tool call.
func (p *PermissionPrompt) Show(tool *tools.Tool, call *tools.ToolCall) {
	p.tool = tool
	p.call = call
	p.visible = true
	p.selected = ButtonAllow
}

// Hide hides the permission prompt.
func (p *PermissionPrompt) Hide() {
	p.visible = false
	p.tool = nil
	p.call = nil
}

// IsVisible returns whether the prompt is visible.
func (p *PermissionPrompt) IsVisible() bool {
	return p.visible
}

// SetSize updates the prompt dimensions.
func (p *PermissionPrompt) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// =============================================================================
// BUBBLE TEA METHODS
// =============================================================================

// Update handles key events for the permission prompt.
func (p *PermissionPrompt) Update(msg tea.Msg) (tea.Cmd, bool) {
	if !p.visible {
		return nil, false
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			p.selected = (p.selected - 1 + ButtonCount) % ButtonCount
			return nil, true

		case "right", "l":
			p.selected = (p.selected + 1) % ButtonCount
			return nil, true

		case "tab":
			p.selected = (p.selected + 1) % ButtonCount
			return nil, true

		case "shift+tab":
			p.selected = (p.selected - 1 + ButtonCount) % ButtonCount
			return nil, true

		case "enter", " ":
			return p.handleSelect(), true

		case "escape", "n":
			// Deny and close
			call := p.call // Capture before Hide clears it
			p.Hide()
			return func() tea.Msg {
				return tools.ToolPermissionResponseMsg{
					Call:        call,
					Allowed:     false,
					AlwaysAllow: false,
				}
			}, true

		case "y":
			// Quick allow
			p.selected = ButtonAllow
			return p.handleSelect(), true

		case "a":
			// Quick always allow
			p.selected = ButtonAlwaysAllow
			return p.handleSelect(), true
		}
	}

	return nil, false
}

// handleSelect processes the current selection.
func (p *PermissionPrompt) handleSelect() tea.Cmd {
	call := p.call
	allowed := p.selected != ButtonDeny
	alwaysAllow := p.selected == ButtonAlwaysAllow

	p.Hide()

	return func() tea.Msg {
		return tools.ToolPermissionResponseMsg{
			Call:       call,
			Allowed:    allowed,
			AlwaysAllow: alwaysAllow,
		}
	}
}

// =============================================================================
// VIEW RENDERING
// =============================================================================

// View renders the permission prompt.
func (p *PermissionPrompt) View() string {
	if !p.visible || p.tool == nil || p.call == nil {
		return ""
	}

	// Calculate dimensions
	boxWidth := 60
	if p.width > 0 && p.width < 80 {
		boxWidth = p.width - 10
	}
	if boxWidth < 40 {
		boxWidth = 40
	}

	// Build content
	var content strings.Builder

	// Title with risk level color
	riskColor := p.tool.RiskLevel.Color()
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(riskColor)).
		Bold(true)

	content.WriteString(titleStyle.Render("Tool Request"))
	content.WriteString("\n\n")

	// Tool name and description
	toolNameStyle := lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true)

	content.WriteString(toolNameStyle.Render(p.tool.Name))
	content.WriteString(" wants to execute:\n\n")

	// Parameters box
	paramsContent := p.renderParameters()
	paramsBox := lipgloss.NewStyle().
		Background(styles.SurfaceDim).
		Foreground(styles.TextPrimary).
		Padding(0, 1).
		Width(boxWidth - 6).
		Render(paramsContent)

	content.WriteString(paramsBox)
	content.WriteString("\n\n")

	// Risk level indicator
	riskStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(riskColor))

	content.WriteString("Risk Level: ")
	content.WriteString(riskStyle.Render(p.tool.RiskLevel.String()))
	content.WriteString("\n\n")

	// Buttons
	content.WriteString(p.renderButtons())

	// Keyboard hints
	hintStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Italic(true)

	content.WriteString("\n\n")
	content.WriteString(hintStyle.Render("y=Allow  a=Always  n=Deny  Tab=Navigate"))

	// Main box
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(riskColor)).
		Background(styles.Surface).
		Padding(1, 2).
		Width(boxWidth)

	box := boxStyle.Render(content.String())

	// Center in terminal
	if p.width > 0 && p.height > 0 {
		return lipgloss.Place(
			p.width, p.height,
			lipgloss.Center, lipgloss.Center,
			box,
		)
	}

	return box
}

// renderParameters renders the parameter display.
func (p *PermissionPrompt) renderParameters() string {
	if p.tool == nil || p.tool.Schema.Parameters == nil {
		return ""
	}

	var builder strings.Builder

	paramStyle := lipgloss.NewStyle().
		Foreground(styles.TextSecondary)

	valueStyle := lipgloss.NewStyle().
		Foreground(styles.TextPrimary)

	for _, param := range p.tool.Schema.Parameters {
		if val, ok := p.call.Params[param.Name]; ok {
			builder.WriteString(paramStyle.Render(param.Name + ": "))

			// Format value
			valStr := formatParamValue(val)
			// Truncate long values using rune-based truncation for Unicode safety
			valRunes := []rune(valStr)
			if len(valRunes) > 100 {
				valStr = string(valRunes[:97]) + "..."
			}

			builder.WriteString(valueStyle.Render(valStr))
			builder.WriteString("\n")
		}
	}

	return strings.TrimSuffix(builder.String(), "\n")
}

// renderButtons renders the button row.
func (p *PermissionPrompt) renderButtons() string {
	buttonStyle := lipgloss.NewStyle().
		Foreground(styles.TextPrimary).
		Background(styles.Overlay).
		Padding(0, 2).
		MarginRight(1)

	activeStyle := lipgloss.NewStyle().
		Foreground(styles.TextInverse).
		Background(styles.Purple).
		Bold(true).
		Padding(0, 2).
		MarginRight(1)

	var buttons []string

	// Allow button
	if p.selected == ButtonAllow {
		buttons = append(buttons, activeStyle.Render("Allow"))
	} else {
		buttons = append(buttons, buttonStyle.Render("Allow"))
	}

	// Always Allow button
	if p.selected == ButtonAlwaysAllow {
		buttons = append(buttons, activeStyle.Render("Always Allow"))
	} else {
		buttons = append(buttons, buttonStyle.Render("Always Allow"))
	}

	// Deny button
	denyButtonStyle := buttonStyle
	denyActiveStyle := activeStyle.Background(styles.Rose)

	if p.selected == ButtonDeny {
		buttons = append(buttons, denyActiveStyle.Render("Deny"))
	} else {
		buttons = append(buttons, denyButtonStyle.Render("Deny"))
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, buttons...)
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// formatParamValue formats a parameter value for display.
func formatParamValue(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return util.IntToString(v)
	case int64:
		return util.IntToString(int(v))
	case float64:
		return util.FloatToString(v)
	default:
		return "<complex value>"
	}
}
