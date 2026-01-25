// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// TUTORIAL STATE
// =============================================================================

// TutorialStep represents a step in the interactive tutorial.
type TutorialStep struct {
	Title       string // Step title (e.g., "Welcome")
	Message     string // Step content/instructions
	Action      string // Expected action (e.g., "Type a message and press Enter")
	Icon        string // ASCII icon for visual appeal (no emojis)
	CompletedBy string // What action completes this step: "message", "command", "file", etc.
}

// Tutorial steps configuration
var tutorialSteps = []TutorialStep{
	{
		Title:       "Welcome to rigrun!",
		Icon:        "[1]",
		Message:     "rigrun is your DoD-compliant AI coding assistant.\n\nYou can chat, use commands, and include files for context.",
		Action:      "Type a message and press Enter to continue",
		CompletedBy: "message",
	},
	{
		Title:       "Slash Commands",
		Icon:        "[2]",
		Message:     "Commands start with / and give you powerful shortcuts.\n\nTry /help to see all available commands.",
		Action:      "Type /help and press Enter",
		CompletedBy: "help",
	},
	{
		Title:       "Model Selection",
		Icon:        "[3]",
		Message:     "Switch between AI models with /model.\n\nUse /models to see what's available locally.",
		Action:      "Type /models to see available models",
		CompletedBy: "models",
	},
	{
		Title:       "File Context",
		Icon:        "[4]",
		Message:     "Include files with @file:path for context-aware answers.\n\nPress Tab after typing @file: to autocomplete paths.",
		Action:      "Try typing @file: and press Tab to explore",
		CompletedBy: "mention",
	},
	{
		Title:       "You're All Set!",
		Icon:        "[*]",
		Message:     "You're ready to use rigrun!\n\nType /help anytime for command reference.\nPress Ctrl+F to search conversations.\nPress ? to toggle keyboard shortcuts.",
		Action:      "Press Enter or Esc to start chatting",
		CompletedBy: "complete",
	},
}

// =============================================================================
// TUTORIAL MODEL
// =============================================================================

// TutorialOverlay is an interactive tutorial overlay component.
type TutorialOverlay struct {
	// State
	visible       bool
	currentStep   int
	totalSteps    int
	skipped       bool
	dismissible   bool
	autoAdvance   bool   // Auto-advance when action is detected
	lastAction    string // Track last action to detect completion
	startTime     time.Time
	stepStartTime time.Time

	// Dimensions
	width  int
	height int

	// Callback when tutorial completes or is skipped
	onComplete func(completed bool, currentStep int)
}

// NewTutorialOverlay creates a new tutorial overlay.
func NewTutorialOverlay() TutorialOverlay {
	return TutorialOverlay{
		visible:       false,
		currentStep:   0,
		totalSteps:    len(tutorialSteps),
		dismissible:   true,
		autoAdvance:   true,
		startTime:     time.Now(),
		stepStartTime: time.Now(),
	}
}

// =============================================================================
// CONFIGURATION
// =============================================================================

// Show displays the tutorial overlay.
func (t *TutorialOverlay) Show() {
	t.visible = true
	t.currentStep = 0
	t.skipped = false
	t.startTime = time.Now()
	t.stepStartTime = time.Now()
}

// Hide hides the tutorial overlay.
func (t *TutorialOverlay) Hide() {
	t.visible = false
}

// IsVisible returns whether the tutorial is visible.
func (t *TutorialOverlay) IsVisible() bool {
	return t.visible
}

// SetSize sets the display dimensions.
func (t *TutorialOverlay) SetSize(width, height int) {
	t.width = width
	t.height = height
}

// SetOnComplete sets the callback for when tutorial completes or is skipped.
func (t *TutorialOverlay) SetOnComplete(fn func(completed bool, currentStep int)) {
	t.onComplete = fn
}

// GetCurrentStep returns the current step index.
func (t *TutorialOverlay) GetCurrentStep() int {
	return t.currentStep
}

// =============================================================================
// TUTORIAL ACTIONS
// =============================================================================

// TutorialCompleteMsg signals that the tutorial was completed or skipped.
type TutorialCompleteMsg struct {
	Completed   bool // True if fully completed, false if skipped
	CurrentStep int  // Last step reached
}

// NextStep advances to the next tutorial step.
// Returns a tea.Cmd if the tutorial is complete.
func (t *TutorialOverlay) NextStep() tea.Cmd {
	if t.currentStep < t.totalSteps-1 {
		t.currentStep++
		t.stepStartTime = time.Now()
		return nil
	}
	// Tutorial complete
	return t.completeTutorial(true)
}

// PrevStep goes back to the previous tutorial step.
func (t *TutorialOverlay) PrevStep() {
	if t.currentStep > 0 {
		t.currentStep--
		t.stepStartTime = time.Now()
	}
}

// SkipTutorial skips the tutorial.
// Returns a tea.Cmd to send the completion message.
func (t *TutorialOverlay) SkipTutorial() tea.Cmd {
	t.skipped = true
	return t.completeTutorial(false)
}

// completeTutorial marks the tutorial as complete and hides it.
// Returns a tea.Cmd to send the TutorialCompleteMsg.
func (t *TutorialOverlay) completeTutorial(completed bool) tea.Cmd {
	t.visible = false
	if t.onComplete != nil {
		t.onComplete(completed, t.currentStep)
	}
	// Return a command to send TutorialCompleteMsg
	currentStep := t.currentStep
	return func() tea.Msg {
		return TutorialCompleteMsg{
			Completed:   completed,
			CurrentStep: currentStep,
		}
	}
}

// TutorialAdvanceMsg signals that the tutorial should advance after a delay.
type TutorialAdvanceMsg struct{}

// RecordAction records a user action and auto-advances if it matches the step requirement.
// Returns a tea.Cmd to schedule advancement after a delay instead of blocking.
func (t *TutorialOverlay) RecordAction(action string) tea.Cmd {
	if !t.visible || !t.autoAdvance {
		return nil
	}

	t.lastAction = action

	// Check if this action completes the current step
	step := tutorialSteps[t.currentStep]
	completed := false

	switch step.CompletedBy {
	case "message":
		completed = action == "message"
	case "help":
		completed = action == "help"
	case "models":
		completed = action == "models"
	case "mention":
		completed = action == "mention" || action == "file"
	case "complete":
		completed = true
	}

	if completed {
		// Instead of blocking sleep, return a command to advance after delay
		return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return TutorialAdvanceMsg{}
		})
	}

	return nil
}

// =============================================================================
// BUBBLE TEA INTERFACE
// =============================================================================

// Init initializes the tutorial overlay.
func (t TutorialOverlay) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (t TutorialOverlay) Update(msg tea.Msg) (TutorialOverlay, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height

	case TutorialAdvanceMsg:
		// Handle auto-advance after delay
		if t.visible {
			cmd := t.NextStep()
			return t, cmd
		}
		return t, nil

	case tea.KeyMsg:
		if !t.visible {
			return t, nil
		}

		switch msg.String() {
		case "esc":
			// Skip tutorial
			cmd := t.SkipTutorial()
			return t, cmd

		case "enter", " ":
			// Advance to next step
			cmd := t.NextStep()
			return t, cmd

		case "left", "h":
			// Go to previous step
			t.PrevStep()
			return t, nil

		case "right", "l":
			// Go to next step
			cmd := t.NextStep()
			return t, cmd
		}
	}

	return t, nil
}

// View renders the tutorial overlay.
func (t TutorialOverlay) View() string {
	if !t.visible {
		return ""
	}

	return t.renderOverlay()
}

// =============================================================================
// RENDER METHODS
// =============================================================================

// renderOverlay renders the tutorial as a centered overlay box.
func (t TutorialOverlay) renderOverlay() string {
	width := t.width
	height := t.height
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}

	// Calculate box dimensions
	boxWidth := 60
	if boxWidth > width-8 {
		boxWidth = width - 8
	}
	if boxWidth < 40 {
		boxWidth = 40
	}

	// Get current step
	step := tutorialSteps[t.currentStep]

	// Build content sections
	var sections []string

	// Title with icon
	titleStyle := lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true).
		Width(boxWidth - 4)
	sections = append(sections, titleStyle.Render(step.Icon+"  "+step.Title))

	// Spacing
	sections = append(sections, "")

	// Message
	messageStyle := lipgloss.NewStyle().
		Foreground(styles.TextPrimary).
		Width(boxWidth - 4)
	sections = append(sections, messageStyle.Render(step.Message))

	// Spacing
	sections = append(sections, "")

	// Action hint
	actionStyle := lipgloss.NewStyle().
		Foreground(styles.Emerald).
		Italic(true).
		Width(boxWidth - 4)
	sections = append(sections, actionStyle.Render("-> "+step.Action))

	// Spacing
	sections = append(sections, "")

	// Separator
	separatorStyle := lipgloss.NewStyle().
		Foreground(styles.OverlayDim)
	sections = append(sections, separatorStyle.Render(strings.Repeat("-", boxWidth-4)))

	// Controls hint
	controlsStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Width(boxWidth - 4)
	sections = append(sections, controlsStyle.Render("[Enter] Continue    [Esc] Skip tutorial"))

	// Progress dots
	sections = append(sections, t.renderProgressDots())

	// Join all sections
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Create box with border using ASCII-safe normal border
	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(styles.Cyan).
		Padding(1, 2).
		Width(boxWidth).
		Render(content)

	// Add title to box border
	titleText := fmt.Sprintf(" Tutorial (%d/%d) ", t.currentStep+1, t.totalSteps)
	titleStyle = lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true)
	title := titleStyle.Render(titleText)

	// Insert title into first line of box (after "+-")
	lines := strings.Split(box, "\n")
	if len(lines) > 0 {
		firstLine := lines[0]

		// Ensure we have enough space in the first line for the title
		if len(firstLine) > 2 && len(titleText) > 0 && len(firstLine) >= 2+len(titleText) {
			lines[0] = firstLine[:2] + title + firstLine[2+len(titleText):]
		}
		box = strings.Join(lines, "\n")
	}

	// Center the box
	if height > 0 {
		return lipgloss.Place(
			width, height,
			lipgloss.Center, lipgloss.Center,
			box,
		)
	}

	return box
}

// renderProgressDots renders progress indicator dots using ASCII characters.
func (t TutorialOverlay) renderProgressDots() string {
	var dots []string

	for i := 0; i < t.totalSteps; i++ {
		var dot string
		var style lipgloss.Style

		if i < t.currentStep {
			// Completed step
			dot = "[x]"
			style = lipgloss.NewStyle().Foreground(styles.Emerald)
		} else if i == t.currentStep {
			// Current step
			dot = "[>]"
			style = lipgloss.NewStyle().Foreground(styles.Cyan).Bold(true)
		} else {
			// Upcoming step
			dot = "[ ]"
			style = lipgloss.NewStyle().Foreground(styles.TextMuted)
		}

		dots = append(dots, style.Render(dot))
	}

	return strings.Join(dots, " ")
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// ShouldShowTutorial determines if the tutorial should be shown on startup.
func ShouldShowTutorial(tutorialCompleted bool) bool {
	return !tutorialCompleted
}
