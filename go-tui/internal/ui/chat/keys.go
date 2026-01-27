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

// HelpContext represents the UI context for filtering help items.
// Follows lazygit's pattern of context-aware keybinding display.
type HelpContext string

const (
	// ContextNormal is the default state - ready for navigation
	ContextNormal HelpContext = "normal"
	// ContextInput is when the user is typing in the input field
	ContextInput HelpContext = "input"
	// ContextStreaming is when receiving a streaming response
	ContextStreaming HelpContext = "streaming"
	// ContextError is when an error is displayed
	ContextError HelpContext = "error"
	// ContextSearch is when search mode is active
	ContextSearch HelpContext = "search"
	// ContextHelp is when help overlay is visible
	ContextHelp HelpContext = "help"
	// ContextPalette is when command palette is open
	ContextPalette HelpContext = "palette"
)

// HelpCategory represents action type grouping for help display.
type HelpCategory string

const (
	CategoryNavigation HelpCategory = "Navigation"
	CategoryCommands   HelpCategory = "Commands"
	CategoryModes      HelpCategory = "Modes"
	CategoryActions    HelpCategory = "Actions"
	CategorySearch     HelpCategory = "Search"
	CategoryVim        HelpCategory = "Vim"
)

// HelpItem represents a single help entry with key, description, and context.
// Context-aware help follows lazygit's pattern where only relevant keybindings
// are shown based on the current UI state.
type HelpItem struct {
	Key      string        // Key binding(s) displayed (e.g., "up/k", "C-c")
	Desc     string        // Human-readable description
	Contexts []HelpContext // Contexts where this binding is active
	Category HelpCategory  // Action type grouping for display
}

// GetHelpItems returns all help items for display in the help overlay.
// This provides a comprehensive list of all available keyboard shortcuts
// with context and category information for context-sensitive filtering.
func GetHelpItems() []HelpItem {
	// Define common context sets for reuse
	all := []HelpContext{ContextNormal, ContextInput, ContextStreaming, ContextError, ContextSearch}
	navContexts := []HelpContext{ContextNormal, ContextStreaming}
	normalOnly := []HelpContext{ContextNormal}
	inputOnly := []HelpContext{ContextInput}
	streamingOnly := []HelpContext{ContextStreaming}
	errorOnly := []HelpContext{ContextError}
	searchOnly := []HelpContext{ContextSearch}
	normalAndInput := []HelpContext{ContextNormal, ContextInput}

	return []HelpItem{
		// Navigation - available in normal mode and during streaming
		{"up/k", "Scroll up", navContexts, CategoryNavigation},
		{"down/j", "Scroll down", navContexts, CategoryNavigation},
		{"PgUp/C-u", "Page up", navContexts, CategoryNavigation},
		{"PgDn/C-d", "Page down", navContexts, CategoryNavigation},
		{"Home/g", "Go to top", navContexts, CategoryNavigation},
		{"End/G", "Go to bottom", navContexts, CategoryNavigation},

		// Actions - context-specific
		{"Enter", "Send message", inputOnly, CategoryActions},
		{"C-c", "Cancel streaming", streamingOnly, CategoryActions},
		{"Esc", "Cancel / exit mode", []HelpContext{ContextInput, ContextStreaming, ContextSearch, ContextError}, CategoryActions},
		{"C-y", "Copy last response", normalAndInput, CategoryActions},

		// Commands - mostly available in normal/input modes
		{"C-p", "Command palette", normalAndInput, CategoryCommands},
		{"C-l", "Clear screen", normalAndInput, CategoryCommands},
		{"C-f or /", "Search", normalOnly, CategoryCommands},
		{"C-r", "Cycle routing mode", normalAndInput, CategoryCommands},
		{"/command", "Run slash command", inputOnly, CategoryCommands},

		// Mode switching
		{"?", "Toggle help", normalOnly, CategoryModes},
		{"C-k", "Multi-line mode", inputOnly, CategoryModes},
		{"i", "Enter input mode", normalOnly, CategoryModes},

		// Quit
		{"q", "Quit", normalOnly, CategoryActions},
		{"C-q", "Quit (emergency)", all, CategoryActions},

		// Search mode specific
		{"n/Enter", "Next match", searchOnly, CategorySearch},
		{"N", "Previous match", searchOnly, CategorySearch},
		{"Esc", "Exit search", searchOnly, CategorySearch},

		// Error mode specific
		{"Esc/Enter", "Dismiss error", errorOnly, CategoryActions},
		{"r", "Retry last action", errorOnly, CategoryActions},
	}
}

// GetHelpItemsForContext returns help items filtered for the given context.
// This is the primary method for context-sensitive help display, following
// lazygit's pattern where only currently-active keybindings are shown.
func GetHelpItemsForContext(ctx HelpContext) []HelpItem {
	all := GetHelpItems()
	var filtered []HelpItem

	for _, item := range all {
		for _, itemCtx := range item.Contexts {
			if itemCtx == ctx {
				filtered = append(filtered, item)
				break
			}
		}
	}

	return filtered
}

// GetHelpItemsByCategory returns help items grouped by category for the given context.
// Returns a map of category -> items for organized display.
func GetHelpItemsByCategory(ctx HelpContext) map[HelpCategory][]HelpItem {
	items := GetHelpItemsForContext(ctx)
	grouped := make(map[HelpCategory][]HelpItem)

	for _, item := range items {
		grouped[item.Category] = append(grouped[item.Category], item)
	}

	return grouped
}

// GetCategoryOrder returns the preferred display order for categories.
func GetCategoryOrder() []HelpCategory {
	return []HelpCategory{
		CategoryNavigation,
		CategoryActions,
		CategoryCommands,
		CategoryModes,
		CategorySearch,
		CategoryVim,
	}
}

// GetVimHelpItems returns vim-specific navigation help items.
// These are shown in the vim section when vim mode is enabled.
func GetVimHelpItems() []HelpItem {
	normalOnly := []HelpContext{ContextNormal}
	searchOnly := []HelpContext{ContextSearch}

	return []HelpItem{
		{"i", "Enter input mode", normalOnly, CategoryVim},
		{"a", "Append mode (cursor at end)", normalOnly, CategoryVim},
		{"Esc", "Exit input mode", []HelpContext{ContextInput}, CategoryVim},
		{"j/k", "Scroll down/up", normalOnly, CategoryVim},
		{"g/G", "Go to top/bottom", normalOnly, CategoryVim},
		{"C-u/C-d", "Half-page up/down", normalOnly, CategoryVim},
		{"n/N", "Next/prev search match", searchOnly, CategoryVim},
		{":w", "Save conversation", normalOnly, CategoryVim},
		{":q", "Quit", normalOnly, CategoryVim},
		{":wq", "Save and quit", normalOnly, CategoryVim},
	}
}

// GetContextDisplayName returns a human-readable name for a context.
func GetContextDisplayName(ctx HelpContext) string {
	switch ctx {
	case ContextNormal:
		return "Normal Mode"
	case ContextInput:
		return "Input Mode"
	case ContextStreaming:
		return "Streaming"
	case ContextError:
		return "Error"
	case ContextSearch:
		return "Search Mode"
	case ContextHelp:
		return "Help"
	case ContextPalette:
		return "Command Palette"
	default:
		return string(ctx)
	}
}

