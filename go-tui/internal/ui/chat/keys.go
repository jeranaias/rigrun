// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package chat provides the chat view component for the TUI.
//
// This file defines keyboard bindings and shortcuts for the chat interface.
// It provides a comprehensive KeyMap with vim-like navigation and standard
// terminal shortcuts, along with help text generation for user reference.
package chat

import (
	"github.com/charmbracelet/bubbles/key"
)

// =============================================================================
// KEY MAP DEFINITION
// =============================================================================

// KeyMap defines all keyboard bindings for the chat interface.
// Each binding supports multiple keys and includes help text for documentation.
type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	PageUp    key.Binding
	PageDown  key.Binding
	Home      key.Binding
	End       key.Binding
	Submit    key.Binding
	Cancel    key.Binding
	Help      key.Binding
	Quit      key.Binding
	Search    key.Binding
	Clear     key.Binding
	MultiLine key.Binding
	CycleMode key.Binding
}

// DefaultKeyMap returns the default key bindings for the chat interface.
// These bindings support both standard terminal navigation and vim-like shortcuts.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("up/k", "scroll up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("down/j", "scroll down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("PgUp/C-u", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("PgDn/C-d", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("Home/g", "go to top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("End/G", "go to bottom"),
		),
		Submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("Enter", "submit"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc", "ctrl+c"),
			key.WithHelp("Esc/C-c", "cancel streaming"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+q", "q"),
			key.WithHelp("q/C-q", "quit"),
		),
		Search: key.NewBinding(
			key.WithKeys("ctrl+f", "/"),
			key.WithHelp("C-f or /", "search"),
		),
		Clear: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("C-l", "clear screen"),
		),
		MultiLine: key.NewBinding(
			key.WithKeys("ctrl+k"),
			key.WithHelp("C-k", "multi-line mode"),
		),
		CycleMode: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("C-r", "cycle routing"),
		),
	}
}

// =============================================================================
// KEY BINDING HELPERS
// =============================================================================

// ShortHelp returns a slice of key bindings to show in the short help view.
// These are the most commonly used shortcuts.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Submit, k.Cancel, k.Help, k.Quit}
}

// FullHelp returns a slice of key bindings to show in the full help view.
// This is organized into groups for better readability.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Navigation
		{k.Up, k.Down, k.PageUp, k.PageDown},
		// Go to
		{k.Home, k.End},
		// Actions
		{k.Submit, k.Cancel, k.Search, k.Clear},
		// Modes
		{k.MultiLine, k.CycleMode, k.Help, k.Quit},
	}
}

// =============================================================================
// HELP TEXT DATA
// =============================================================================

// HelpItem represents a single help entry with key and description.
type HelpItem struct {
	Key  string
	Desc string
}

// GetHelpItems returns all help items for display in the help overlay.
// This provides a comprehensive list of all available keyboard shortcuts.
func GetHelpItems() []HelpItem {
	return []HelpItem{
		{"up/k, down/j", "Scroll up/down"},
		{"PgUp/C-u, PgDn/C-d", "Page up/down"},
		{"Home/g, End/G", "Top/bottom"},
		{"Enter", "Send message"},
		{"C-c", "Cancel streaming"},
		{"C-y", "Copy last response"},
		{"C-p", "Command palette"},
		{"C-q", "Quit (emergency exit)"},
		{"C-l", "Clear screen"},
		{"C-k", "Multi-line mode"},
		{"C-f or /", "Search"},
		{"C-r", "Cycle routing mode"},
		{"/command", "Run command"},
		{"?", "Toggle help"},
		{"q", "Quit (normal mode)"},
	}
}

// GetVimHelpItems returns vim-specific navigation help items.
func GetVimHelpItems() []HelpItem {
	return []HelpItem{
		{"i", "Enter input mode"},
		{"Esc", "Exit input mode"},
		{"j/k", "Scroll down/up"},
		{"g/G", "Go to top/bottom"},
		{"C-u/C-d", "Half-page up/down"},
		{"n/N", "Next/prev search match"},
	}
}

