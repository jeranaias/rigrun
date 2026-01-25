// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package chat provides the chat view component for the TUI.
package chat

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeranaias/rigrun-tui/internal/commands"
)

// =============================================================================
// TAB COMPLETION HANDLERS
// =============================================================================

// handleTabCompletion handles Tab key press for completion.
// First Tab: Show completions
// Second Tab (quickly): Cycle through completions
func (m Model) handleTabCompletion() (tea.Model, tea.Cmd) {
	// Get current input
	input := m.input.Value()
	cursorPos := m.input.Position()

	// If completions are already showing, cycle to next
	if m.showCompletions && m.completionState != nil && m.completionState.Visible {
		m.completionCycleCount++

		// Cycle through completions
		m.completionState.Next()

		// Apply the selected completion
		return m.applyCompletion()
	}

	// First Tab - get completions
	if m.completer == nil {
		return m, nil
	}

	completions := m.completer.Complete(input, cursorPos)
	if len(completions) == 0 {
		return m, nil
	}

	// Update completion state
	if m.completionState != nil {
		m.completionState.Update(input, completions)
		m.showCompletions = true
		m.completionCycleCount = 1

		// If only one completion, apply it immediately
		if len(completions) == 1 {
			return m.applyCompletion()
		}
	}

	return m, nil
}

// applyCompletion applies the currently selected completion to the input.
func (m Model) applyCompletion() (tea.Model, tea.Cmd) {
	if m.completionState == nil || !m.completionState.Visible {
		return m, nil
	}

	selected := m.completionState.GetSelected()
	if selected == nil {
		return m, nil
	}

	// Get the completion value
	value := selected.Value

	// Determine what to replace
	input := m.input.Value()
	cursorPos := m.input.Position()

	// Find the start of the current word/mention/command
	start := m.findCompletionStart(input, cursorPos)

	// Build the new input
	newInput := input[:start] + value

	// If completing a command and there are arguments, add a space
	if strings.HasPrefix(value, "/") && !strings.HasSuffix(value, " ") {
		// Check if the command takes arguments
		cmd := m.completer.GetCommand(value)
		if cmd != nil && len(cmd.Args) > 0 {
			newInput += " "
		}
	}

	// For file completions, add a space at the end if it's a file (not a directory)
	if strings.HasPrefix(value, "@file:") && !strings.HasSuffix(value, "/") && !strings.HasSuffix(value, "\\") {
		newInput += " "
	}

	// Update input
	m.input.SetValue(newInput)
	m.input.CursorEnd()

	// If we've cycled through all completions, hide the popup
	if m.completionCycleCount > len(m.completionState.Completions) {
		m.showCompletions = false
		m.completionCycleCount = 0
	}

	// Record tutorial action for @file: mentions
	var tutorialCmd tea.Cmd
	if m.tutorial != nil && m.tutorial.IsVisible() {
		if strings.HasPrefix(value, "@file:") {
			tutorialCmd = m.tutorial.RecordAction("mention")
		}
	}

	// Batch commands if tutorial is active
	if tutorialCmd != nil {
		return m, tea.Batch(textinput.Blink, tutorialCmd)
	}
	return m, textinput.Blink
}

// findCompletionStart finds the start position of the current word being completed.
func (m Model) findCompletionStart(input string, cursorPos int) int {
	if cursorPos > len(input) {
		cursorPos = len(input)
	}

	// Handle commands (/) - only if input starts with /
	trimmedInput := strings.TrimSpace(input[:cursorPos])
	if strings.HasPrefix(trimmedInput, "/") {
		// Find the last /
		for i := cursorPos - 1; i >= 0; i-- {
			if input[i] == '/' {
				return i
			}
			if input[i] == ' ' {
				// Found a space before the /, so we're completing an argument
				break
			}
		}
	}

	// Handle mentions (@)
	if strings.Contains(input[:cursorPos], "@") {
		// Find the last @
		for i := cursorPos - 1; i >= 0; i-- {
			if input[i] == '@' {
				return i
			}
			if input[i] == ' ' {
				// Found a space before the @, so start from space
				break
			}
		}
	}

	// Default: find last space or start of string
	for i := cursorPos - 1; i >= 0; i-- {
		if input[i] == ' ' {
			return i + 1
		}
	}

	return 0
}

// clearCompletions clears the completion state.
func (m *Model) clearCompletions() {
	m.showCompletions = false
	m.completionCycleCount = 0
	if m.completionState != nil {
		m.completionState.Clear()
	}
}

// SetCompleter sets the completer for the chat model.
func (m *Model) SetCompleter(completer *commands.Completer) {
	m.completer = completer
}

// GetCompleter returns the completer.
func (m *Model) GetCompleter() *commands.Completer {
	return m.completer
}

// SetupCompleterCallbacks sets up the dynamic completion callbacks.
func (m *Model) SetupCompleterCallbacks() {
	if m.completer == nil {
		return
	}

	// Model completion
	m.completer.ModelsFn = func() []string {
		if m.ollama == nil {
			return nil
		}
		// Get models from ollama - this would need to be async in production
		// For now, return a default list
		return []string{
			"qwen2.5-coder:14b",
			"qwen2.5-coder:7b",
			"codestral:22b",
			"llama3.1:8b",
			"deepseek-coder:6.7b",
		}
	}

	// Session completion - would integrate with session storage
	m.completer.SessionsFn = func() []commands.SessionInfo {
		// TODO: Integrate with session storage
		return nil
	}

	// Tool completion
	m.completer.ToolsFn = func() []string {
		if m.toolRegistry == nil {
			return nil
		}
		allTools := m.toolRegistry.All()
		names := make([]string, len(allTools))
		for i, t := range allTools {
			names[i] = t.Name
		}
		return names
	}

	// Config completion
	m.completer.ConfigFn = func() []string {
		return []string{
			"model", "mode", "temperature", "max_tokens",
			"timeout", "autosave", "theme", "routing.default_mode",
		}
	}

	// File completion - uses the default file completer
	m.completer.FilesFn = nil
}
