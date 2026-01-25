// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/commands"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// COMMAND PALETTE
// =============================================================================

// CommandPalette is an overlay component for searching and executing commands.
type CommandPalette struct {
	// Input field for filtering
	input textinput.Model

	// Registry of commands
	registry *commands.Registry

	// Filtered commands with scores
	filtered []scoredCommand

	// Selected index
	selected int

	// Dimensions
	width  int
	height int

	// Visibility
	visible bool

	// Theme
	theme *styles.Theme

	// Maximum items to show
	maxItems int

	// Recent commands (most recent first)
	recentCommands []string

	// Max recent commands to track
	maxRecent int
}

// scoredCommand holds a command with its fuzzy match score.
type scoredCommand struct {
	command *commands.Command
	score   int
}

// NewCommandPalette creates a new command palette.
func NewCommandPalette(registry *commands.Registry, theme *styles.Theme) *CommandPalette {
	ti := textinput.New()
	ti.Placeholder = "Type a command..."
	ti.Prompt = "> "
	ti.CharLimit = 100
	ti.Width = 50
	ti.PromptStyle = lipgloss.NewStyle().Foreground(styles.Cyan).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(styles.TextPrimary)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(styles.TextMuted).Italic(true)

	cp := &CommandPalette{
		input:          ti,
		registry:       registry,
		theme:          theme,
		maxItems:       10,
		selected:       0,
		recentCommands: make([]string, 0, 10),
		maxRecent:      10,
	}

	cp.updateFiltered()

	return cp
}

// =============================================================================
// BUBBLE TEA INTERFACE
// =============================================================================

// Init initializes the command palette.
func (cp *CommandPalette) Init() tea.Cmd {
	return nil
}

// Update handles messages for the command palette.
func (cp *CommandPalette) Update(msg tea.Msg) (*CommandPalette, tea.Cmd) {
	if !cp.visible {
		return cp, nil
	}

	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			cp.Hide()
			return cp, nil

		case "enter":
			if cp.selected >= 0 && cp.selected < len(cp.filtered) {
				selectedCmd := cp.filtered[cp.selected].command
				cp.recordRecentCommand(selectedCmd.Name)
				cp.Hide()
				return cp, cp.executeCommand(selectedCmd)
			}
			return cp, nil

		case "up":
			if len(cp.filtered) == 0 {
				return cp, nil
			}
			cp.selected--
			if cp.selected < 0 {
				cp.selected = len(cp.filtered) - 1
			}
			return cp, nil

		case "down", "ctrl+n":
			if len(cp.filtered) == 0 {
				return cp, nil
			}
			cp.selected++
			if cp.selected >= len(cp.filtered) {
				cp.selected = 0
			}
			return cp, nil

		case "tab":
			// Tab also selects next item
			if len(cp.filtered) == 0 {
				return cp, nil
			}
			cp.selected++
			if cp.selected >= len(cp.filtered) {
				cp.selected = 0
			}
			return cp, nil
		}
	}

	// Update the input field
	previousValue := cp.input.Value()
	cp.input, cmd = cp.input.Update(msg)

	// If input changed, update filtered list
	if cp.input.Value() != previousValue {
		cp.updateFiltered()
		cp.selected = 0
	}

	return cp, cmd
}

// View renders the command palette.
func (cp *CommandPalette) View() string {
	if !cp.visible {
		return ""
	}

	// Box dimensions
	boxWidth := 60
	if cp.width > 0 && cp.width < boxWidth+10 {
		boxWidth = cp.width - 10
	}
	if boxWidth < 40 {
		boxWidth = 40
	}

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(styles.Purple).
		Bold(true).
		Padding(0, 1)
	header := headerStyle.Render("Commands")

	// Separator
	sepStyle := lipgloss.NewStyle().
		Foreground(styles.Overlay)
	separator := sepStyle.Render(strings.Repeat("-", boxWidth-4))

	// Input
	cp.input.Width = boxWidth - 6
	inputView := cp.input.View()

	// Command list
	var listItems []string
	for i, sc := range cp.filtered {
		if i >= cp.maxItems {
			// Show "... X more" indicator
			remaining := len(cp.filtered) - cp.maxItems
			if remaining > 0 {
				moreStyle := lipgloss.NewStyle().
					Foreground(styles.TextMuted).
					Italic(true)
				listItems = append(listItems, moreStyle.Render("  ... "+formatInt(remaining)+" more"))
			}
			break
		}

		item := cp.renderItem(sc.command, i == cp.selected, boxWidth-6)
		listItems = append(listItems, item)
	}

	list := strings.Join(listItems, "\n")

	// If no commands match
	if len(cp.filtered) == 0 {
		noMatchStyle := lipgloss.NewStyle().
			Foreground(styles.TextMuted).
			Italic(true).
			Padding(1, 0)
		list = noMatchStyle.Render("No matching commands")
	}

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Padding(1, 0, 0, 0)
	help := helpStyle.Render("Up/Down navigate | Enter select | Esc close")

	// Combine all parts
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		separator,
		inputView,
		separator,
		list,
		help,
	)

	// Box style
	boxStyle := lipgloss.NewStyle().
		Background(styles.Surface).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.Purple).
		Padding(1, 2).
		Width(boxWidth)

	box := boxStyle.Render(content)

	// Center the box
	if cp.width > 0 && cp.height > 0 {
		return lipgloss.Place(
			cp.width, cp.height,
			lipgloss.Center, lipgloss.Center,
			box,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("#000000")),
		)
	}

	return box
}

// =============================================================================
// INTERNAL METHODS
// =============================================================================

// renderItem renders a single command item.
func (cp *CommandPalette) renderItem(cmd *commands.Command, selected bool, width int) string {
	// Check if this is a recently used command
	isRecent := cp.isRecentCommand(cmd.Name)

	// Selection indicator (ASCII)
	indicator := "  "
	if selected {
		indicator = "> "
	}

	// Command name style
	cmdStyle := lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true)

	// Description style
	descStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted)

	// Recent indicator (ASCII)
	recentIndicator := ""
	if isRecent {
		recentIndicator = lipgloss.NewStyle().
			Foreground(styles.Emerald).
			Render(" *")
	}

	// Build the item with indicator
	name := cmdStyle.Render(cmd.Name)

	// Calculate remaining width for description
	usedWidth := lipgloss.Width(indicator) + lipgloss.Width(name) + lipgloss.Width(recentIndicator) + 2
	descWidth := width - usedWidth
	if descWidth < 10 {
		descWidth = 10
	}

	desc := descStyle.Render(truncateString(cmd.Description, descWidth))

	item := indicator + name + recentIndicator + "  " + desc

	if selected {
		selectedStyle := lipgloss.NewStyle().
			Background(styles.Purple).
			Foreground(styles.TextInverse).
			Width(width).
			Padding(0, 1)
		return selectedStyle.Render(item)
	}

	return item
}

// updateFiltered updates the filtered command list based on input using fuzzy matching.
func (cp *CommandPalette) updateFiltered() {
	if cp.registry == nil {
		cp.filtered = nil
		return
	}

	filter := strings.TrimSpace(cp.input.Value())
	filter = strings.TrimPrefix(filter, "/")

	if filter == "" {
		// Show all commands, with recent commands at the top
		cp.filtered = cp.getAllCommandsWithRecent()
		return
	}

	// Fuzzy match against all commands
	var scored []scoredCommand
	for _, cmd := range cp.registry.All() {
		if cmd.Hidden {
			continue
		}

		// Try fuzzy matching against name (without leading /)
		name := strings.TrimPrefix(cmd.Name, "/")
		nameScore, nameMatched := FuzzyMatch(filter, name)

		// Try fuzzy matching against description
		descScore, descMatched := FuzzyMatch(filter, cmd.Description)

		// Try fuzzy matching against aliases
		aliasScore := 0
		aliasMatched := false
		for _, alias := range cmd.Aliases {
			aliasClean := strings.TrimPrefix(alias, "/")
			score, matched := FuzzyMatch(filter, aliasClean)
			if matched && score > aliasScore {
				aliasScore = score
				aliasMatched = true
			}
		}

		// Take the best match
		bestScore := 0
		matched := false
		if nameMatched && nameScore > bestScore {
			bestScore = nameScore
			matched = true
		}
		if descMatched && descScore > bestScore {
			bestScore = descScore / 2 // Description matches get lower priority
			matched = true
		}
		if aliasMatched && aliasScore > bestScore {
			bestScore = aliasScore
			matched = true
		}

		if matched {
			// Boost score for recent commands
			if cp.isRecentCommand(cmd.Name) {
				bestScore += 100
			}

			scored = append(scored, scoredCommand{
				command: cmd,
				score:   bestScore,
			})
		}
	}

	// Sort by score (highest first)
	cp.sortScoredCommands(scored)

	cp.filtered = scored
}

// sortScoredCommands sorts scored commands by score in descending order.
func (cp *CommandPalette) sortScoredCommands(scored []scoredCommand) {
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
}

// getAllCommandsWithRecent returns all non-hidden commands, with recent ones first.
func (cp *CommandPalette) getAllCommandsWithRecent() []scoredCommand {
	if cp.registry == nil {
		return nil
	}

	var scored []scoredCommand

	// Add all non-hidden commands
	for _, cmd := range cp.registry.All() {
		if !cmd.Hidden {
			score := 0
			// Recent commands get higher scores
			if cp.isRecentCommand(cmd.Name) {
				score = 1000 - cp.getRecentIndex(cmd.Name)
			}
			scored = append(scored, scoredCommand{
				command: cmd,
				score:   score,
			})
		}
	}

	// Sort by score (recent first)
	cp.sortScoredCommands(scored)

	return scored
}

// isRecentCommand returns true if the command is in the recent list.
func (cp *CommandPalette) isRecentCommand(cmdName string) bool {
	for _, recent := range cp.recentCommands {
		if recent == cmdName {
			return true
		}
	}
	return false
}

// getRecentIndex returns the index of a command in the recent list.
// Returns -1 if not found.
func (cp *CommandPalette) getRecentIndex(cmdName string) int {
	for i, recent := range cp.recentCommands {
		if recent == cmdName {
			return i
		}
	}
	return -1
}

// recordRecentCommand adds a command to the recent list.
func (cp *CommandPalette) recordRecentCommand(cmdName string) {
	// Remove if already exists
	for i, recent := range cp.recentCommands {
		if recent == cmdName {
			cp.recentCommands = append(cp.recentCommands[:i], cp.recentCommands[i+1:]...)
			break
		}
	}

	// Add to front
	cp.recentCommands = append([]string{cmdName}, cp.recentCommands...)

	// Trim to max size
	if len(cp.recentCommands) > cp.maxRecent {
		cp.recentCommands = cp.recentCommands[:cp.maxRecent]
	}
}

// executeCommand returns a command to execute the selected command.
func (cp *CommandPalette) executeCommand(cmd *commands.Command) tea.Cmd {
	return func() tea.Msg {
		return ExecuteCommandMsg{
			Command: cmd,
			Args:    nil,
		}
	}
}

// =============================================================================
// PUBLIC METHODS
// =============================================================================

// Show shows the command palette.
func (cp *CommandPalette) Show() {
	cp.visible = true
	cp.input.Reset()
	cp.input.Focus()
	cp.updateFiltered()
	cp.selected = 0
}

// Hide hides the command palette.
func (cp *CommandPalette) Hide() {
	cp.visible = false
	cp.input.Blur()
}

// Toggle toggles the visibility of the command palette.
func (cp *CommandPalette) Toggle() {
	if cp.visible {
		cp.Hide()
	} else {
		cp.Show()
	}
}

// IsVisible returns true if the command palette is visible.
func (cp *CommandPalette) IsVisible() bool {
	return cp.visible
}

// SetSize sets the dimensions for centering the palette.
func (cp *CommandPalette) SetSize(width, height int) {
	cp.width = width
	cp.height = height
}

// Focus focuses the input field.
func (cp *CommandPalette) Focus() tea.Cmd {
	return cp.input.Focus()
}

// =============================================================================
// MESSAGES
// =============================================================================

// ExecuteCommandMsg is sent when a command is selected from the palette.
type ExecuteCommandMsg struct {
	Command *commands.Command
	Args    []string
}

// ShowPaletteMsg is sent to show the command palette.
type ShowPaletteMsg struct{}

// HidePaletteMsg is sent to hide the command palette.
type HidePaletteMsg struct{}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// truncateString truncates a string to maxLen characters.
// Uses rune-based truncation to handle Unicode correctly.
func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}
