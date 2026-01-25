// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides the visual UI components for rigrun TUI.
package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// INPUT AREA COMPONENT - Beautiful text input with character counter
// =============================================================================

// InputArea represents the styled text input component
type InputArea struct {
	input       textinput.Model
	placeholder string
	maxChars    int
	width       int
	height      int
	focused     bool
	theme       *styles.Theme
}

// NewInputArea creates a new InputArea component
func NewInputArea(theme *styles.Theme) *InputArea {
	ti := textinput.New()
	ti.Placeholder = "Type a message... (@file: for context, / for commands)"
	ti.CharLimit = 4096
	ti.Width = 70
	ti.Prompt = "> "

	// Style the text input
	ti.PromptStyle = lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true)

	ti.TextStyle = lipgloss.NewStyle().
		Foreground(styles.TextPrimary)

	ti.PlaceholderStyle = lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Italic(true)

	ti.Cursor.Style = lipgloss.NewStyle().
		Foreground(styles.Cyan)

	return &InputArea{
		input:       ti,
		placeholder: ti.Placeholder,
		maxChars:    4096,
		width:       80,
		height:      3,
		focused:     false,
		theme:       theme,
	}
}

// Focus focuses the input
func (i *InputArea) Focus() tea.Cmd {
	i.focused = true
	return i.input.Focus()
}

// Blur removes focus from the input
func (i *InputArea) Blur() {
	i.focused = false
	i.input.Blur()
}

// Focused returns whether the input is focused
func (i *InputArea) Focused() bool {
	return i.focused
}

// SetWidth sets the input area width
func (i *InputArea) SetWidth(width int) {
	i.width = width
	// Account for prompt and padding
	inputWidth := width - 10
	if inputWidth < 20 {
		inputWidth = 20
	}
	i.input.Width = inputWidth
}

// SetPlaceholder sets the placeholder text
func (i *InputArea) SetPlaceholder(placeholder string) {
	i.placeholder = placeholder
	i.input.Placeholder = placeholder
}

// SetMaxChars sets the maximum character limit
func (i *InputArea) SetMaxChars(max int) {
	i.maxChars = max
	i.input.CharLimit = max
}

// Value returns the current input value
func (i *InputArea) Value() string {
	return i.input.Value()
}

// SetValue sets the input value
func (i *InputArea) SetValue(value string) {
	i.input.SetValue(value)
}

// Reset clears the input
func (i *InputArea) Reset() {
	i.input.Reset()
}

// CursorPosition returns the cursor position
func (i *InputArea) CursorPosition() int {
	return i.input.Position()
}

// SetCursorPosition sets the cursor position
func (i *InputArea) SetCursorPosition(pos int) {
	i.input.SetCursor(pos)
}

// Update handles input updates
func (i *InputArea) Update(msg tea.Msg) (*InputArea, tea.Cmd) {
	var cmd tea.Cmd
	i.input, cmd = i.input.Update(msg)
	return i, cmd
}

// View renders the input area
func (i *InputArea) View() string {
	// Calculate character count display
	charCount := len([]rune(i.input.Value()))
	charCountDisplay := i.renderCharCounter(charCount)

	// Get the text input view
	inputView := i.input.View()

	// Determine border style based on focus state
	var borderColor lipgloss.AdaptiveColor
	if i.focused {
		borderColor = styles.Cyan // Glowing effect when focused
	} else {
		borderColor = styles.Overlay
	}

	// Build the container
	containerStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(i.width - 2)

	// If focused, add a subtle glow effect by using a brighter border
	if i.focused {
		containerStyle = containerStyle.
			BorderForeground(styles.FocusRing)
	}

	// Top decoration line
	topLine := i.renderTopDecoration()

	// Build the complete input area
	inputSection := containerStyle.Render(inputView)

	// Character counter aligned to the right
	counterWidth := i.width - 4
	counterStyle := lipgloss.NewStyle().
		Width(counterWidth).
		Align(lipgloss.Right)

	charCountLine := counterStyle.Render(charCountDisplay)

	// Combine everything
	return lipgloss.JoinVertical(
		lipgloss.Left,
		topLine,
		inputSection,
		charCountLine,
	)
}

// ViewCompact renders a compact single-line input
func (i *InputArea) ViewCompact() string {
	inputView := i.input.View()
	charCount := len([]rune(i.input.Value()))

	// Compact char counter
	counterStyle := i.getCharCountStyle(charCount)
	counter := counterStyle.Render("(" + util.IntToString(charCount) + ")")

	return inputView + " " + counter
}

// ==========================================================================
// HELPER RENDER METHODS
// ==========================================================================

// renderTopDecoration renders a decorative line above the input
func (i *InputArea) renderTopDecoration() string {
	lineStyle := lipgloss.NewStyle().Foreground(styles.Overlay)

	// Simple decoration
	lineLen := i.width - 4
	if lineLen < 10 {
		lineLen = 10
	}

	return lineStyle.Render(strings.Repeat("-", lineLen))
}

// renderCharCounter renders the character counter with color coding
func (i *InputArea) renderCharCounter(count int) string {
	percent := 0.0
	if i.maxChars > 0 {
		percent = float64(count) / float64(i.maxChars) * 100
	}

	// Format: "1,234 / 4,096 chars"
	countStr := formatNumber(count)
	maxStr := formatNumber(i.maxChars)

	counterText := countStr + " / " + maxStr + " chars"

	// Color code based on usage
	style := i.getCharCountStyle(count)

	// Add visual indicator for high usage
	indicator := ""
	if percent >= 90 {
		indicator = " [!]"
	} else if percent >= 75 {
		indicator = " [~]"
	}

	return style.Render(counterText + indicator)
}

// getCharCountStyle returns the appropriate style for the character count
func (i *InputArea) getCharCountStyle(count int) lipgloss.Style {
	percent := 0.0
	if i.maxChars > 0 {
		percent = float64(count) / float64(i.maxChars) * 100
	}

	if percent >= 90 {
		return lipgloss.NewStyle().
			Foreground(styles.Rose).
			Bold(true)
	}
	if percent >= 75 {
		return lipgloss.NewStyle().
			Foreground(styles.Amber)
	}
	if percent >= 50 {
		return lipgloss.NewStyle().
			Foreground(styles.TextSecondary)
	}
	return lipgloss.NewStyle().
		Foreground(styles.TextMuted)
}

// =============================================================================
// MULTILINE INPUT AREA - For longer messages
// =============================================================================

// MultilineInputArea represents a multiline text input
type MultilineInputArea struct {
	lines       []string
	cursorRow   int
	cursorCol   int
	maxChars    int
	width       int
	height      int
	focused     bool
	placeholder string
	theme       *styles.Theme
}

// NewMultilineInputArea creates a new multiline input area
func NewMultilineInputArea(theme *styles.Theme) *MultilineInputArea {
	return &MultilineInputArea{
		lines:       []string{""},
		cursorRow:   0,
		cursorCol:   0,
		maxChars:    8192,
		width:       80,
		height:      5,
		focused:     false,
		placeholder: "Type your message... (Shift+Enter for new line)",
		theme:       theme,
	}
}

// Value returns the full text content
func (m *MultilineInputArea) Value() string {
	return strings.Join(m.lines, "\n")
}

// SetValue sets the text content
func (m *MultilineInputArea) SetValue(value string) {
	m.lines = strings.Split(value, "\n")
	if len(m.lines) == 0 {
		m.lines = []string{""}
	}
	m.cursorRow = len(m.lines) - 1
	m.cursorCol = len([]rune(m.lines[m.cursorRow]))
}

// Reset clears the input
func (m *MultilineInputArea) Reset() {
	m.lines = []string{""}
	m.cursorRow = 0
	m.cursorCol = 0
}

// Focus focuses the input
func (m *MultilineInputArea) Focus() {
	m.focused = true
}

// Blur removes focus
func (m *MultilineInputArea) Blur() {
	m.focused = false
}

// InsertChar inserts a character at the cursor position
func (m *MultilineInputArea) InsertChar(char rune) {
	totalChars := len([]rune(m.Value()))
	if totalChars >= m.maxChars {
		return
	}

	line := m.lines[m.cursorRow]
	runes := []rune(line)
	if m.cursorCol > len(runes) {
		m.cursorCol = len(runes)
	}
	newLine := string(runes[:m.cursorCol]) + string(char) + string(runes[m.cursorCol:])
	m.lines[m.cursorRow] = newLine
	m.cursorCol++
}

// InsertNewLine inserts a new line at the cursor position
func (m *MultilineInputArea) InsertNewLine() {
	line := m.lines[m.cursorRow]
	runes := []rune(line)
	if m.cursorCol > len(runes) {
		m.cursorCol = len(runes)
	}
	before := string(runes[:m.cursorCol])
	after := string(runes[m.cursorCol:])

	// Update current line and insert new line
	m.lines[m.cursorRow] = before
	newLines := append(m.lines[:m.cursorRow+1], append([]string{after}, m.lines[m.cursorRow+1:]...)...)
	m.lines = newLines

	m.cursorRow++
	m.cursorCol = 0
}

// Backspace removes the character before the cursor
func (m *MultilineInputArea) Backspace() {
	line := m.lines[m.cursorRow]
	runes := []rune(line)
	if m.cursorCol > len(runes) {
		m.cursorCol = len(runes)
	}
	if m.cursorCol > 0 {
		m.lines[m.cursorRow] = string(runes[:m.cursorCol-1]) + string(runes[m.cursorCol:])
		m.cursorCol--
	} else if m.cursorRow > 0 {
		// Merge with previous line
		prevLine := m.lines[m.cursorRow-1]
		currLine := m.lines[m.cursorRow]

		m.lines[m.cursorRow-1] = prevLine + currLine
		m.lines = append(m.lines[:m.cursorRow], m.lines[m.cursorRow+1:]...)

		m.cursorRow--
		m.cursorCol = len([]rune(prevLine))
	}
}

// View renders the multiline input
func (m *MultilineInputArea) View() string {
	var content strings.Builder

	// Render each line
	for row, line := range m.lines {
		if row > 0 {
			content.WriteString("\n")
		}

		// Show cursor on current row
		if m.focused && row == m.cursorRow {
			runes := []rune(line)
			// Insert cursor character
			if m.cursorCol >= len(runes) {
				content.WriteString(line)
				cursorStyle := lipgloss.NewStyle().
					Background(styles.Cyan).
					Foreground(styles.Surface)
				content.WriteString(cursorStyle.Render(" "))
			} else {
				content.WriteString(string(runes[:m.cursorCol]))
				cursorStyle := lipgloss.NewStyle().
					Background(styles.Cyan).
					Foreground(styles.Surface)
				content.WriteString(cursorStyle.Render(string(runes[m.cursorCol])))
				content.WriteString(string(runes[m.cursorCol+1:]))
			}
		} else {
			content.WriteString(line)
		}
	}

	// Show placeholder if empty
	displayContent := content.String()
	if displayContent == "" || (len(m.lines) == 1 && m.lines[0] == "") {
		if !m.focused {
			placeholderStyle := lipgloss.NewStyle().
				Foreground(styles.TextMuted).
				Italic(true)
			displayContent = placeholderStyle.Render(m.placeholder)
		}
	}

	// Character counter
	totalChars := len([]rune(m.Value()))
	charCounter := m.renderCharCounter(totalChars)

	// Border style based on focus
	borderColor := styles.Overlay
	if m.focused {
		borderColor = styles.FocusRing
	}

	// Container
	containerStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(m.width - 2).
		Height(m.height)

	inputBox := containerStyle.Render(displayContent)

	// Counter aligned right
	counterStyle := lipgloss.NewStyle().
		Width(m.width - 4).
		Align(lipgloss.Right)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		inputBox,
		counterStyle.Render(charCounter),
	)
}

// renderCharCounter renders the character counter for multiline input
func (m *MultilineInputArea) renderCharCounter(count int) string {
	percent := 0.0
	if m.maxChars > 0 {
		percent = float64(count) / float64(m.maxChars) * 100
	}

	countStr := formatNumber(count)
	maxStr := formatNumber(m.maxChars)
	lineCount := len(m.lines)

	counterText := countStr + " / " + maxStr + " chars | " + util.IntToString(lineCount) + " lines"

	// Color code based on usage
	var style lipgloss.Style
	if percent >= 90 {
		style = lipgloss.NewStyle().Foreground(styles.Rose).Bold(true)
	} else if percent >= 75 {
		style = lipgloss.NewStyle().Foreground(styles.Amber)
	} else {
		style = lipgloss.NewStyle().Foreground(styles.TextMuted)
	}

	return style.Render(counterText)
}
