// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package chat provides the chat view component for the TUI.
//
// This file implements Vim-style modal editing for the chat interface.
package chat

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// VIM MODE TYPES
// =============================================================================

// VimMode represents the current vim editing mode
type VimMode int

const (
	VimModeNormal  VimMode = iota // Normal mode: navigation and commands
	VimModeInsert                 // Insert mode: text editing
	VimModeVisual                 // Visual mode: text selection
	VimModeCommand                // Command mode: : commands
)

// String returns the display string for the vim mode
func (v VimMode) String() string {
	switch v {
	case VimModeNormal:
		return "NORMAL"
	case VimModeInsert:
		return "INSERT"
	case VimModeVisual:
		return "VISUAL"
	case VimModeCommand:
		return "COMMAND"
	default:
		return "UNKNOWN"
	}
}

// =============================================================================
// VIM HANDLER
// =============================================================================

// VimHandler handles vim-style navigation and editing
type VimHandler struct {
	mode          VimMode
	enabled       bool
	commandBuffer string // For : commands
	searchBuffer  string // For / search
	visualStart   int    // Start position for visual selection
	visualEnd     int    // End position for visual selection
	count         int    // Numeric prefix (e.g., 5j for 5 lines down)
	lastG         bool   // Track if g was just pressed (for gg)
}

// NewVimHandler creates a vim handler
func NewVimHandler(enabled bool) *VimHandler {
	return &VimHandler{
		mode:    VimModeNormal,
		enabled: enabled,
	}
}

// Enabled returns whether vim mode is active
func (vh *VimHandler) Enabled() bool {
	return vh.enabled
}

// SetEnabled sets whether vim mode is active
func (vh *VimHandler) SetEnabled(enabled bool) {
	vh.enabled = enabled
	if enabled {
		vh.mode = VimModeNormal
	} else {
		vh.mode = VimModeInsert // When disabled, act like always in insert mode
	}
}

// Mode returns current mode
func (vh *VimHandler) Mode() VimMode {
	return vh.mode
}

// ModeString returns mode as display string
func (vh *VimHandler) ModeString() string {
	if !vh.enabled {
		return "" // Don't show mode indicator when vim mode is disabled
	}
	return vh.mode.String()
}

// HandleKey processes a key in the current mode
// Returns: consumed (bool), command (tea.Cmd)
func (vh *VimHandler) HandleKey(key tea.KeyMsg, vp *viewport.Model, input *textinput.Model) (bool, tea.Cmd) {
	if !vh.enabled {
		return false, nil // Not enabled, don't consume any keys
	}

	keyStr := key.String()

	switch vh.mode {
	case VimModeNormal:
		return vh.handleNormalMode(keyStr, vp, input)
	case VimModeInsert:
		return vh.handleInsertMode(keyStr, vp, input)
	case VimModeVisual:
		return vh.handleVisualMode(keyStr, vp, input)
	case VimModeCommand:
		return vh.handleCommandMode(keyStr, vp, input)
	default:
		return false, nil
	}
}

// =============================================================================
// NORMAL MODE HANDLERS
// =============================================================================

func (vh *VimHandler) handleNormalMode(keyStr string, vp *viewport.Model, input *textinput.Model) (bool, tea.Cmd) {
	// Handle numeric prefix for count (e.g., 5j)
	if keyStr >= "1" && keyStr <= "9" {
		vh.count = vh.count*10 + int(keyStr[0]-'0')
		return true, nil
	}

	// Get count (default 1)
	count := vh.count
	if count == 0 {
		count = 1
	}

	var consumed bool
	var cmd tea.Cmd

	switch keyStr {
	// Navigation
	case "j":
		vp.LineDown(count)
		consumed = true
	case "k":
		vp.LineUp(count)
		consumed = true
	case "h":
		// Move cursor left (in input when focused)
		consumed = true
	case "l":
		// Move cursor right (in input when focused)
		consumed = true

	// Scroll
	case "ctrl+d":
		vp.HalfViewDown()
		consumed = true
	case "ctrl+u":
		vp.HalfViewUp()
		consumed = true
	case "ctrl+f":
		vp.ViewDown()
		consumed = true
	case "ctrl+b":
		vp.ViewUp()
		consumed = true

	// Go to
	case "g":
		if vh.lastG {
			// gg - go to top
			vp.GotoTop()
			vh.lastG = false
			consumed = true
		} else {
			// First g, wait for second g
			vh.lastG = true
			consumed = true
		}
	case "G":
		vp.GotoBottom()
		consumed = true

	// Enter insert mode
	case "i":
		vh.enterInsertMode(input)
		consumed = true
		cmd = textinput.Blink
	case "a":
		vh.enterInsertMode(input)
		input.CursorEnd()
		consumed = true
		cmd = textinput.Blink
	case "I":
		vh.enterInsertMode(input)
		input.SetCursor(0)
		consumed = true
		cmd = textinput.Blink
	case "A":
		vh.enterInsertMode(input)
		input.CursorEnd()
		consumed = true
		cmd = textinput.Blink
	case "o":
		// Open new line below (enter insert mode)
		vh.enterInsertMode(input)
		consumed = true
		cmd = textinput.Blink
	case "O":
		// Open new line above (enter insert mode)
		vh.enterInsertMode(input)
		consumed = true
		cmd = textinput.Blink

	// Enter visual mode
	case "v":
		vh.enterVisualMode(vp)
		consumed = true

	// Enter command mode
	case ":":
		vh.enterCommandMode(input)
		consumed = true
		cmd = textinput.Blink

	// Search
	case "/":
		vh.enterSearchMode(input)
		consumed = true
		cmd = textinput.Blink

	default:
		// Not a vim normal mode key
		consumed = false
	}

	// Reset count after command
	if consumed && keyStr != "g" {
		vh.count = 0
		vh.lastG = false
	}

	return consumed, cmd
}

// =============================================================================
// INSERT MODE HANDLERS
// =============================================================================

func (vh *VimHandler) handleInsertMode(keyStr string, vp *viewport.Model, input *textinput.Model) (bool, tea.Cmd) {
	switch keyStr {
	case "esc":
		vh.exitInsertMode(input)
		return true, nil
	default:
		// Let all other keys pass through to input
		return false, nil
	}
}

// =============================================================================
// VISUAL MODE HANDLERS
// =============================================================================

func (vh *VimHandler) handleVisualMode(keyStr string, vp *viewport.Model, input *textinput.Model) (bool, tea.Cmd) {
	switch keyStr {
	case "j":
		vp.LineDown(1)
		vh.visualEnd = vp.YOffset
		return true, nil
	case "k":
		vp.LineUp(1)
		vh.visualEnd = vp.YOffset
		return true, nil
	case "y":
		// Yank (copy) selection - would need clipboard integration
		vh.mode = VimModeNormal
		return true, nil
	case "esc":
		vh.mode = VimModeNormal
		return true, nil
	default:
		return false, nil
	}
}

// =============================================================================
// COMMAND MODE HANDLERS
// =============================================================================

func (vh *VimHandler) handleCommandMode(keyStr string, vp *viewport.Model, input *textinput.Model) (bool, tea.Cmd) {
	switch keyStr {
	case "esc":
		vh.commandBuffer = ""
		vh.mode = VimModeNormal
		input.Blur()
		return true, nil
	case "enter":
		// Execute command
		cmd := vh.executeCommand(vh.commandBuffer)
		vh.commandBuffer = ""
		vh.mode = VimModeNormal
		input.Blur()
		return true, cmd
	case "backspace":
		if len(vh.commandBuffer) > 0 {
			vh.commandBuffer = vh.commandBuffer[:len(vh.commandBuffer)-1]
		} else {
			// Empty buffer, exit command mode
			vh.mode = VimModeNormal
			input.Blur()
		}
		return true, nil
	default:
		// Add character to command buffer
		if len(keyStr) == 1 {
			vh.commandBuffer += keyStr
		}
		return true, nil
	}
}

// =============================================================================
// MODE TRANSITIONS
// =============================================================================

func (vh *VimHandler) enterInsertMode(input *textinput.Model) {
	vh.mode = VimModeInsert
	input.Focus()
}

func (vh *VimHandler) exitInsertMode(input *textinput.Model) {
	vh.mode = VimModeNormal
	input.Blur()
}

func (vh *VimHandler) enterVisualMode(vp *viewport.Model) {
	vh.mode = VimModeVisual
	vh.visualStart = vp.YOffset
	vh.visualEnd = vp.YOffset
}

func (vh *VimHandler) enterCommandMode(input *textinput.Model) {
	vh.mode = VimModeCommand
	vh.commandBuffer = ""
	input.Focus()
}

func (vh *VimHandler) enterSearchMode(input *textinput.Model) {
	vh.mode = VimModeCommand
	vh.commandBuffer = "/"
	input.Focus()
}

// =============================================================================
// COMMAND EXECUTION
// =============================================================================

func (vh *VimHandler) executeCommand(cmd string) tea.Cmd {
	// Parse command
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}

	// Handle different commands
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil
	}

	switch parts[0] {
	case "w", "save":
		// Save conversation
		return func() tea.Msg {
			return VimCommandMsg{Command: "save"}
		}
	case "q", "quit":
		// Quit
		return tea.Quit
	case "wq":
		// Save and quit
		return func() tea.Msg {
			return VimCommandMsg{Command: "wq"}
		}
	case "help":
		// Show help
		return func() tea.Msg {
			return VimCommandMsg{Command: "help"}
		}
	case "set":
		// Handle set commands
		if len(parts) > 1 {
			return vh.executeSetCommand(parts[1])
		}
	}

	return nil
}

func (vh *VimHandler) executeSetCommand(arg string) tea.Cmd {
	switch arg {
	case "vim":
		vh.enabled = true
		return func() tea.Msg {
			return VimCommandMsg{Command: "set-vim", Value: true}
		}
	case "novim":
		vh.enabled = false
		vh.mode = VimModeInsert
		return func() tea.Msg {
			return VimCommandMsg{Command: "set-vim", Value: false}
		}
	}
	return nil
}

// GetCommandBuffer returns the current command buffer (for display)
func (vh *VimHandler) GetCommandBuffer() string {
	if vh.mode == VimModeCommand {
		return ":" + vh.commandBuffer
	}
	return ""
}

// =============================================================================
// VIM COMMAND MESSAGE
// =============================================================================

// VimCommandMsg represents a vim command execution request
type VimCommandMsg struct {
	Command string
	Value   interface{}
}
